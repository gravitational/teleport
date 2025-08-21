/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package config

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/services/application"
	"github.com/gravitational/teleport/lib/tbot/services/awsra"
	"github.com/gravitational/teleport/lib/tbot/services/database"
	"github.com/gravitational/teleport/lib/tbot/services/example"
	"github.com/gravitational/teleport/lib/tbot/services/identity"
	"github.com/gravitational/teleport/lib/tbot/services/k8s"
	"github.com/gravitational/teleport/lib/tbot/services/ssh"
	"github.com/gravitational/teleport/lib/tbot/services/workloadidentity"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	DefaultCertificateTTL = 60 * time.Minute
	DefaultRenewInterval  = 20 * time.Minute
)

// ReservedServiceNames are the service names reserved for internal use.
var ReservedServiceNames = []string{
	"ca-rotation",
	"crl-cache",
	"heartbeat",
	"identity",
	"spiffe-trust-bundle-cache",
}

var reservedServiceNamesMap = func() map[string]struct{} {
	m := make(map[string]struct{}, len(ReservedServiceNames))
	for _, k := range ReservedServiceNames {
		m[k] = struct{}{}
	}
	return m
}()

var serviceNameRegex = regexp.MustCompile(`\A[a-z\d_\-+]+\z`)

func validateServiceName(name string) error {
	if name == "" {
		return nil
	}
	if _, ok := reservedServiceNamesMap[name]; ok {
		return trace.BadParameter("service name %q is reserved for internal use", name)
	}
	if !serviceNameRegex.MatchString(name) {
		return trace.BadParameter("invalid service name: %q, may only contain lowercase letters, numbers, hyphens, underscores, or plus symbols", name)
	}
	return nil
}

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

// BotConfig is the bot's root config object.
// This is currently at version "v2".
type BotConfig struct {
	Version    Version           `yaml:"version"`
	Onboarding onboarding.Config `yaml:"onboarding,omitempty"`
	Storage    *StorageConfig    `yaml:"storage,omitempty"`
	// Deprecated: Use Services
	Outputs  ServiceConfigs `yaml:"outputs,omitempty"`
	Services ServiceConfigs `yaml:"services,omitempty"`

	Debug       bool   `yaml:"debug"`
	AuthServer  string `yaml:"auth_server,omitempty"`
	ProxyServer string `yaml:"proxy_server,omitempty"`

	// AuthServerAddressMode controls whether it's permissible to provide a
	// proxy server address as an auth server address. This is unsupported in
	// the tbot binary as of v19, but we maintain support for cases where tbot
	// is embedded in a binary which does not differentiate between address types
	// such as tctl or the Kubernetes operator.
	AuthServerAddressMode connection.AuthServerAddressMode `yaml:"-"`

	// JoinURI is a joining URI, used to supply connection and authentication
	// parameters in a single bundle. If set, the value is parsed and merged on
	// top of the existing configuration during `CheckAndSetDefaults()`.
	JoinURI string `yaml:"join_uri,omitempty"`

	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`
	Oneshot            bool                   `yaml:"oneshot"`
	// FIPS instructs `tbot` to run in a mode designed to comply with FIPS
	// regulations. This means the bot should:
	// - Refuse to run if not compiled with boringcrypto
	// - Use FIPS relevant endpoints for cloud providers (e.g AWS)
	// - Restrict TLS / SSH cipher suites and TLS version
	// - RSA2048 or ECDSA with NIST-P256 curve should be used for private key generation
	FIPS bool `yaml:"fips"`
	// DiagAddr is the address the diagnostics http service should listen on.
	// If not set, no diagnostics listener is created.
	DiagAddr string `yaml:"diag_addr,omitempty"`

	// ReloadCh allows a channel to be injected into the bot to trigger a
	// renewal.
	ReloadCh <-chan struct{} `yaml:"-"`

	// Insecure configures the bot to trust the certificates from the Auth Server or Proxy on first connect without verification.
	// Do not use in production.
	Insecure bool `yaml:"insecure,omitempty"`
}

// ConnectionConfig creates a connection.Config from the user's configuration.
func (conf *BotConfig) ConnectionConfig() connection.Config {
	cc := connection.Config{
		Insecure:              conf.Insecure,
		AuthServerAddressMode: conf.AuthServerAddressMode,
		StaticProxyAddress:    shouldUseProxyAddr(),
	}

	switch {
	case conf.ProxyServer != "":
		cc.Address = conf.ProxyServer
		cc.AddressKind = connection.AddressKindProxy
	case conf.AuthServer != "":
		cc.Address = conf.AuthServer
		cc.AddressKind = connection.AddressKindAuth
	}

	return cc
}

// useProxyAddrEnv is an environment variable which can be set to
// force `tbot` to prefer using the proxy address explicitly provided by the
// user over the one fetched from the proxy ping. This is only intended to work
// in cases where TLS routing is enabled, and is intended to support cases where
// the Proxy is accessible from multiple addresses, and the one included in the
// ProxyPing is incorrect.
const useProxyAddrEnv = "TBOT_USE_PROXY_ADDR"

// shouldUseProxyAddr returns true if the TBOT_USE_PROXY_ADDR environment
// variable is set to "yes". More generally, this indicates that the user wishes
// for tbot to prefer using the proxy address that has been explicitly provided
// by the user rather than the one fetched via a discovery process (e.g ping).
func shouldUseProxyAddr() bool {
	return os.Getenv(useProxyAddrEnv) == "yes"
}

func (conf *BotConfig) UnmarshalYAML(node *yaml.Node) error {
	// Wrap conf in an anonymous struct to avoid having the deprecated field on
	// the BotConfig or CredentialLifetime structs, and keep it purely a config
	// file parsing concern.
	//
	// The type alias prevents infinite recursion by obscuring UnmarshalYAML.
	type alias BotConfig
	output := struct {
		*alias                   `yaml:",inline"`
		DeprecatedCertificateTTL *time.Duration `yaml:"certificate_ttl"`
	}{alias: (*alias)(conf)}
	if err := node.Decode(&output); err != nil {
		return err
	}

	if output.DeprecatedCertificateTTL != nil {
		log.WarnContext(context.TODO(), "Config option certificate_ttl is deprecated and will be removed in a future release. Please use credential_ttl instead.")

		if conf.CredentialLifetime.TTL == 0 {
			conf.CredentialLifetime.TTL = *output.DeprecatedCertificateTTL
		} else {
			log.WarnContext(context.TODO(), "Both certificate_ttl and credential_ttl config options were given, credential_ttl will be used.")
		}
	}

	return nil
}

func (conf *BotConfig) CheckAndSetDefaults() error {
	if conf.Version == "" {
		conf.Version = V2
	}

	if conf.Storage == nil {
		conf.Storage = &StorageConfig{}
	}

	if err := conf.Storage.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// We've migrated Outputs to Services, so copy all Outputs to Services.
	conf.Services = append(conf.Services, conf.Outputs...)
	uniqueNames := make(map[string]struct{}, len(conf.Services))
	for i, service := range conf.Services {
		if err := service.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating service[%d]", i)
		}
		if err := service.GetCredentialLifetime().Validate(conf.Oneshot); err != nil {
			return trace.Wrap(err, "validating service[%d]", i)
		}
		if name := service.GetName(); name != "" {
			if err := validateServiceName(name); err != nil {
				return trace.Wrap(err, "validating service[%d]", i)
			}
			if _, seen := uniqueNames[name]; seen {
				return trace.BadParameter("validating service[%d]: duplicate name: %q", i, name)
			}
			uniqueNames[name] = struct{}{}
		}
	}

	destinationPaths := map[string]int{}
	addDestinationToKnownPaths := func(d destination.Destination) {
		switch d := d.(type) {
		case *destination.Directory:
			destinationPaths[fmt.Sprintf("file://%s", d.Path)]++
		case *k8s.SecretDestination:
			destinationPaths[fmt.Sprintf("kubernetes-secret://%s", d.Name)]++
		}
	}
	for _, svc := range conf.Services {
		v, ok := svc.(interface {
			GetDestination() destination.Destination
		})
		if ok {
			addDestinationToKnownPaths(v.GetDestination())
		}
	}

	// Check for identical destinations being used. This is a deeply
	// uncharted/unknown behavior area. For now we'll emit a heavy warning,
	// in 15+ this will be an explicit area as outputs writing over one another
	// is too complex to support.
	addDestinationToKnownPaths(conf.Storage.Destination)
	for path, count := range destinationPaths {
		if count > 1 {
			log.ErrorContext(
				context.TODO(),
				"Identical destinations used within config. This can produce unusable results. In Teleport 15.0, this will be a fatal error",
				"path", path,
			)
		}
	}

	if conf.CredentialLifetime.TTL == 0 {
		conf.CredentialLifetime.TTL = DefaultCertificateTTL
	}

	if conf.CredentialLifetime.RenewalInterval == 0 {
		conf.CredentialLifetime.RenewalInterval = DefaultRenewInterval
	}

	// We require the join method for `configure` and `start` but not for `init`
	// Therefore, we need to check its valid here, but enforce its presence
	// elsewhere.
	if conf.Onboarding.JoinMethod != types.JoinMethodUnspecified {
		if !slices.Contains(onboarding.SupportedJoinMethods, string(conf.Onboarding.JoinMethod)) {
			return trace.BadParameter("unrecognized join method: %q", conf.Onboarding.JoinMethod)
		}
	}

	// Validate Insecure and CA Settings
	if conf.Insecure {
		if len(conf.Onboarding.CAPins) > 0 {
			return trace.BadParameter("the option ca-pin is mutually exclusive with --insecure")
		}

		if conf.Onboarding.CAPath != "" {
			return trace.BadParameter("the option ca-path is mutually exclusive with --insecure")
		}
	} else {
		if len(conf.Onboarding.CAPins) > 0 && conf.Onboarding.CAPath != "" {
			return trace.BadParameter("the options ca-pin and ca-path are mutually exclusive")
		}
	}

	// Validate CertificateTTL and RenewalInterval options
	var ttlErr bot.SuboptimalCredentialTTLError
	err := conf.CredentialLifetime.Validate(conf.Oneshot)
	switch {
	case errors.As(err, &ttlErr):
		// Note: we log this as a warning for backward-compatibility, but should
		// just reject the configuration in a future release.
		//
		//nolint:sloglint // msg cannot be constant
		log.WarnContext(context.TODO(), ttlErr.Message(), ttlErr.LogLabels()...)
	case err != nil:
		return err
	}

	return nil
}

// ServiceConfig is an interface over the various service configurations.
type ServiceConfig interface {
	Type() string
	CheckAndSetDefaults() error

	// GetCredentialLifetime returns the service's custom certificate TTL and
	// RenewalInterval. It's used for validation purposes; services that do not
	// support these options should return the zero value.
	GetCredentialLifetime() bot.CredentialLifetime

	// GetName returns the user-given name of the service, used for validation
	// purposes.
	GetName() string
}

// ServiceConfigs assists polymorphic unmarshaling of a slice of ServiceConfigs.
type ServiceConfigs []ServiceConfig

func (o *ServiceConfigs) UnmarshalYAML(node *yaml.Node) error {
	var out []ServiceConfig
	var unmarshalContext unmarshalConfigContext
	for _, node := range node.Content {
		header := struct {
			Type string `yaml:"type"`
		}{}
		if err := node.Decode(&header); err != nil {
			return trace.Wrap(err)
		}

		switch header.Type {
		case example.ServiceType:
			v := &example.Config{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case database.TunnelServiceType:
			v := &database.TunnelConfig{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case ssh.MultiplexerServiceType:
			v := &ssh.MultiplexerConfig{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case k8s.OutputV1ServiceType:
			v := &k8s.OutputV1Config{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case k8s.OutputV2ServiceType:
			v := &k8s.OutputV2Config{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case k8s.ArgoCDOutputServiceType:
			v := &k8s.ArgoCDOutputConfig{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case ssh.HostOutputServiceType:
			v := &ssh.HostOutputConfig{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case application.OutputServiceType:
			v := &application.OutputConfig{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case database.OutputServiceType:
			v := &database.OutputConfig{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case identity.OutputServiceType:
			v := &identity.OutputConfig{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case application.TunnelServiceType:
			v := &application.TunnelConfig{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case workloadidentity.X509OutputServiceType:
			v := &workloadidentity.X509OutputConfig{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case workloadidentity.WorkloadAPIServiceType:
			v := &workloadidentity.WorkloadAPIConfig{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case workloadidentity.JWTOutputServiceType:
			v := &workloadidentity.JWTOutputConfig{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case awsra.ServiceType:
			v := &awsra.Config{}
			if err := v.UnmarshalConfig(unmarshalContext, node); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		default:
			return trace.BadParameter("unrecognized service type (%s)", header.Type)
		}
	}

	*o = out
	return nil
}

type unmarshalConfigContext struct {
	internal.DefaultDestinationUnmarshaler
}

func (ctx unmarshalConfigContext) UnmarshalDestination(node *yaml.Node) (destination.Destination, error) {
	header := struct {
		Type string `yaml:"type"`
	}{}
	if err := node.Decode(&header); err != nil {
		return nil, trace.Wrap(err)
	}

	switch header.Type {
	case k8s.SecretDestinationType:
		v := &k8s.SecretDestination{}
		if err := node.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	default:
		return ctx.DefaultDestinationUnmarshaler.UnmarshalDestination(node)
	}
}

// Initable represents any ServiceConfig which is compatible with
// `tbot init`.
type Initable interface {
	GetDestination() destination.Destination
	Init(ctx context.Context) error
	Describe() []bot.FileDescription
}

func (conf *BotConfig) GetInitables() []Initable {
	var out []Initable
	for _, service := range conf.Services {
		if v, ok := service.(Initable); ok {
			out = append(out, v)
		}
	}
	return out
}

// DestinationFromURI parses a URI from the input string and returns a matching
// destination.Destination implementation, if possible.
func DestinationFromURI(uriString string) (destination.Destination, error) {
	uri, err := url.Parse(uriString)
	if err != nil {
		return nil, trace.Wrap(err, "parsing --data-dir")
	}
	switch uri.Scheme {
	case "", "file":
		if uri.Host != "" {
			return nil, trace.BadParameter(
				"file-backed data storage must be on the local host",
			)
		}
		// TODO(strideynet): eventually we can allow for URI query parameters
		// to be used to configure symlinks/acl protection.
		return &destination.Directory{
			Path: uri.Path,
		}, nil
	case "memory":
		if uri.Host != "" || uri.Path != "" {
			return nil, trace.BadParameter(
				"memory-backed data storage should not have host or path specified",
			)
		}
		return &destination.Memory{}, nil
	case "kubernetes-secret":
		if uri.Path == "" {
			return nil, trace.BadParameter(
				"kubernetes-secret scheme should have a path specified",
			)
		}
		// kubernetes-secret:///my-secret

		// Path will be prefixed with '/' so we'll strip it off.
		secretName := strings.TrimPrefix(uri.Path, "/")

		return &k8s.SecretDestination{
			Name:      secretName,
			Namespace: uri.Host,
		}, nil
	default:
		return nil, trace.BadParameter(
			"unrecognized data storage scheme",
		)
	}
}

// ReadConfigFromFile reads and parses a YAML config from a file.
func ReadConfigFromFile(filePath string, manualMigration bool) (*BotConfig, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(filePath)
	if err != nil {
		return nil, trace.Wrap(err, fmt.Sprintf("failed to open file: %v", filePath))
	}

	defer f.Close()
	return ReadConfig(f, manualMigration)
}

// ReadConfigFromBase64String reads and parses a YAML config from a base64 encoded string.
func ReadConfigFromBase64String(b64Str string, manualMigration bool) (*BotConfig, error) {
	data, err := base64.StdEncoding.DecodeString(b64Str)
	if err != nil {
		return nil, trace.Wrap(err, "failed to decode base64 encoded config")
	}
	r := bytes.NewReader(data)
	return ReadConfig(r, manualMigration)
}

type Version string

var (
	V1 Version = "v1"
	V2 Version = "v2"
)

// ReadConfig parses a YAML config file from a Reader.
func ReadConfig(reader io.ReadSeeker, manualMigration bool) (*BotConfig, error) {
	var version struct {
		Version Version `yaml:"version"`
	}
	decoder := yaml.NewDecoder(reader)
	if err := decoder.Decode(&version); err != nil {
		return nil, trace.BadParameter("failed parsing config file version: %s", strings.ReplaceAll(err.Error(), "\n", ""))
	}

	// Reset reader and decoder
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, trace.Wrap(err)
	}
	decoder = yaml.NewDecoder(reader)

	switch version.Version {
	case V1, "":
		if !manualMigration {
			log.WarnContext(
				context.TODO(), "Deprecated config version (V1) detected. Attempting to perform an on-the-fly in-memory migration to latest version. Please persist the config migration by following the guidance at https://goteleport.com/docs/reference/machine-id/v14-upgrade-guide/")
		}
		config := &configV1{}
		if err := decoder.Decode(config); err != nil {
			return nil, trace.BadParameter("failed parsing config file: %s", strings.ReplaceAll(err.Error(), "\n", ""))
		}
		latestConfig, err := config.migrate()
		if err != nil {
			return nil, trace.WithUserMessage(
				trace.Wrap(err, "migrating v1 config"),
				"Failed to migrate. See https://goteleport.com/docs/reference/machine-id/v14-upgrade-guide/",
			)
		}
		return latestConfig, nil
	case V2:
		if manualMigration {
			return nil, trace.BadParameter("configuration already the latest version. nothing to migrate.")
		}
		decoder.KnownFields(true)
		config := &BotConfig{}
		if err := decoder.Decode(config); err != nil {
			return nil, trace.BadParameter("failed parsing config file: %s", strings.ReplaceAll(err.Error(), "\n", ""))
		}
		return config, nil
	default:
		return nil, trace.BadParameter("unrecognized config version %q", version.Version)
	}
}
