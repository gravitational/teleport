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
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	DefaultCertificateTTL = 60 * time.Minute
	DefaultRenewInterval  = 20 * time.Minute
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/config")

var SupportedJoinMethods = []string{
	string(types.JoinMethodAzure),
	string(types.JoinMethodAzureDevops),
	string(types.JoinMethodBitbucket),
	string(types.JoinMethodCircleCI),
	string(types.JoinMethodGCP),
	string(types.JoinMethodGitHub),
	string(types.JoinMethodGitLab),
	string(types.JoinMethodIAM),
	string(types.JoinMethodKubernetes),
	string(types.JoinMethodSpacelift),
	string(types.JoinMethodToken),
	string(types.JoinMethodTPM),
	string(types.JoinMethodTerraformCloud),
	string(types.JoinMethodBoundKeypair),
}

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)

// AzureOnboardingConfig holds configuration relevant to the "azure" join method.
type AzureOnboardingConfig struct {
	// ClientID of the managed identity to use. Required if the VM has more
	// than one assigned identity.
	ClientID string `yaml:"client_id,omitempty"`
}

// TerraformOnboardingConfig contains parameters for the "terraform" join method
type TerraformOnboardingConfig struct {
	// TokenTag is the name of the tag configured via the environment variable
	// `TERRAFORM_WORKLOAD_IDENTITY_AUDIENCE(_$TAG)`. If unset, the untagged
	// variant is used.
	AudienceTag string `yaml:"audience_tag,omitempty"`
}

// GitlabOnboardingConfig holds configuration relevant to the "gitlab" join method.
type GitlabOnboardingConfig struct {
	// TokenEnvVarName is the name of the environment variable that contains the
	// GitLab ID token. This can be useful to override in cases where a single
	// gitlab job needs to authenticate to multiple Teleport clusters.
	TokenEnvVarName string `yaml:"token_env_var_name,omitempty"`
}

// BoundKeypairOnboardingConfig contains parameters for the `bound_keypair` join
// method
type BoundKeypairOnboardingConfig struct {
	// InitialJoinSecret is the name of the initial joining secret, if any. If
	// not specified, a keypair must be created using `tbot keypair create` and
	// registered with Teleport in advance.
	InitialJoinSecret string
}

// OnboardingConfig contains values relevant to how the bot authenticates with
// the Teleport cluster.
type OnboardingConfig struct {
	// TokenValue is either the token needed to join the auth server, or a path pointing to a file
	// that contains the token
	//
	// You should use Token() instead - this has to be an exported field for YAML unmarshaling
	// to work correctly, but this could be a path instead of a token
	TokenValue string `yaml:"token,omitempty"`

	// CAPath is an optional path to a CA certificate.
	CAPath string `yaml:"ca_path,omitempty"`

	// CAPins is a list of certificate authority pins, used to validate the
	// connection to the Teleport auth server.
	CAPins []string `yaml:"ca_pins,omitempty"`

	// JoinMethod is the method the bot should use to exchange a token for the
	// initial certificate
	JoinMethod types.JoinMethod `yaml:"join_method"`

	// Azure holds configuration relevant to the azure joining method.
	Azure AzureOnboardingConfig `yaml:"azure,omitempty"`

	// Terraform holds configuration relevant to the `terraform` join method.
	Terraform TerraformOnboardingConfig `yaml:"terraform,omitempty"`

	// Gitlab holds configuration relevant to the `gitlab` join method.
	Gitlab GitlabOnboardingConfig `yaml:"gitlab,omitempty"`

	// BoundKeypair holds configuration relevant to the `bound_keypair` join method
	BoundKeypair BoundKeypairOnboardingConfig `yaml:"bound_keypair,omitempty"`
}

// HasToken gives the ability to check if there has been a token value stored
// in the config
func (conf *OnboardingConfig) HasToken() bool {
	return conf.TokenValue != ""
}

// SetToken stores the value for --token or auth_token in the config
//
// In the case of the token value pointing to a file, this allows us to
// fetch the value of the token when it's needed (when connecting for the first time)
// instead of trying to read the file every time that teleport is launched.
// This means we can allow temporary token files that are removed after teleport has
// successfully connected the first time.
func (conf *OnboardingConfig) SetToken(token string) {
	conf.TokenValue = token
}

// Token returns token needed to join the auth server
//
// If the value stored points to a file, it will attempt to read the token value from the file
// and return an error if it wasn't successful
// If the value stored doesn't point to a file, it'll return the value stored
func (conf *OnboardingConfig) Token() (string, error) {
	token, err := utils.TryReadValueAsFile(conf.TokenValue)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}

// BotConfig is the bot's root config object.
// This is currently at version "v2".
type BotConfig struct {
	Version    Version          `yaml:"version"`
	Onboarding OnboardingConfig `yaml:"onboarding,omitempty"`
	Storage    *StorageConfig   `yaml:"storage,omitempty"`
	// Deprecated: Use Services
	Outputs  ServiceConfigs `yaml:"outputs,omitempty"`
	Services ServiceConfigs `yaml:"services,omitempty"`

	Debug      bool   `yaml:"debug"`
	AuthServer string `yaml:"auth_server,omitempty"`
	// ProxyServer is the teleport proxy address. Unlike `AuthServer` this must
	// explicitly point to a Teleport proxy.
	// Example: "example.teleport.sh:443"
	ProxyServer        string             `yaml:"proxy_server,omitempty"`
	CredentialLifetime CredentialLifetime `yaml:",inline"`
	Oneshot            bool               `yaml:"oneshot"`
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

type AddressKind string

const (
	AddressKindUnspecified AddressKind = ""
	AddressKindProxy       AddressKind = "proxy"
	AddressKindAuth        AddressKind = "auth"
)

// Address returns the address to the auth server, either directly or via
// a proxy, and the kind of address it is.
func (conf *BotConfig) Address() (string, AddressKind) {
	switch {
	case conf.AuthServer != "" && conf.ProxyServer != "":
		// This is an error case that should be prevented by the validation.
		return "", AddressKindUnspecified
	case conf.ProxyServer != "":
		return conf.ProxyServer, AddressKindProxy
	case conf.AuthServer != "":
		return conf.AuthServer, AddressKindAuth
	default:
		return "", AddressKindUnspecified
	}
}

func (conf *BotConfig) CipherSuites() []uint16 {
	if conf.FIPS {
		return defaults.FIPSCipherSuites
	}
	return utils.DefaultCipherSuites()
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
	for i, service := range conf.Services {
		if err := service.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating service[%d]", i)
		}
		if err := service.GetCredentialLifetime().Validate(conf.Oneshot); err != nil {
			return trace.Wrap(err, "validating service[%d]", i)
		}
	}

	destinationPaths := map[string]int{}
	addDestinationToKnownPaths := func(d bot.Destination) {
		switch d := d.(type) {
		case *DestinationDirectory:
			destinationPaths[fmt.Sprintf("file://%s", d.Path)]++
		case *DestinationKubernetesSecret:
			destinationPaths[fmt.Sprintf("kubernetes-secret://%s", d.Name)]++
		}
	}
	for _, svc := range conf.Services {
		v, ok := svc.(interface{ GetDestination() bot.Destination })
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
		if !slices.Contains(SupportedJoinMethods, string(conf.Onboarding.JoinMethod)) {
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
	var ttlErr SuboptimalCredentialTTLError
	err := conf.CredentialLifetime.Validate(conf.Oneshot)
	switch {
	case errors.As(err, &ttlErr):
		// Note: we log this as a warning for backward-compatibility, but should
		// just reject the configuration in a future release.
		//
		//nolint:sloglint // msg cannot be constant
		log.WarnContext(context.TODO(), ttlErr.msg, ttlErr.LogLabels()...)
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
	GetCredentialLifetime() CredentialLifetime
}

// ServiceConfigs assists polymorphic unmarshaling of a slice of ServiceConfigs.
type ServiceConfigs []ServiceConfig

func (o *ServiceConfigs) UnmarshalYAML(node *yaml.Node) error {
	var out []ServiceConfig
	for _, node := range node.Content {
		header := struct {
			Type string `yaml:"type"`
		}{}
		if err := node.Decode(&header); err != nil {
			return trace.Wrap(err)
		}

		switch header.Type {
		case ExampleServiceType:
			v := &ExampleService{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case SPIFFEWorkloadAPIServiceType:
			v := &SPIFFEWorkloadAPIService{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case DatabaseTunnelServiceType:
			v := &DatabaseTunnelService{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case SSHMultiplexerServiceType:
			v := &SSHMultiplexerService{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case KubernetesOutputType:
			v := &KubernetesOutput{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case KubernetesV2OutputType:
			v := &KubernetesV2Output{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case SPIFFESVIDOutputType:
			v := &SPIFFESVIDOutput{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case SSHHostOutputType:
			v := &SSHHostOutput{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case ApplicationOutputType:
			v := &ApplicationOutput{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case DatabaseOutputType:
			v := &DatabaseOutput{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case IdentityOutputType:
			v := &IdentityOutput{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case ApplicationTunnelServiceType:
			v := &ApplicationTunnelService{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case WorkloadIdentityX509OutputType:
			v := &WorkloadIdentityX509Service{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case WorkloadIdentityAPIServiceType:
			v := &WorkloadIdentityAPIService{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case WorkloadIdentityJWTOutputType:
			v := &WorkloadIdentityJWTService{}
			if err := node.Decode(v); err != nil {
				return trace.Wrap(err)
			}
			out = append(out, v)
		case WorkloadIdentityAWSRAType:
			v := &WorkloadIdentityAWSRAService{}
			if err := node.Decode(v); err != nil {
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

func withTypeHeader[T any](payload T, payloadType string) (interface{}, error) {
	header := struct {
		Type    string `yaml:"type"`
		Payload T      `yaml:",inline"`
	}{
		Type:    payloadType,
		Payload: payload,
	}

	return header, nil
}

// unmarshalDestination takes a *yaml.Node and produces a bot.Destination by
// considering the `type` field.
func unmarshalDestination(node *yaml.Node) (bot.Destination, error) {
	header := struct {
		Type string `yaml:"type"`
	}{}
	if err := node.Decode(&header); err != nil {
		return nil, trace.Wrap(err)
	}

	switch header.Type {
	case DestinationMemoryType:
		v := &DestinationMemory{}
		if err := node.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	case DestinationDirectoryType:
		v := &DestinationDirectory{}
		if err := node.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	case DestinationKubernetesSecretType:
		v := &DestinationKubernetesSecret{}
		if err := node.Decode(v); err != nil {
			return nil, trace.Wrap(err)
		}
		return v, nil
	default:
		return nil, trace.BadParameter("unrecognized destination type (%s)", header.Type)
	}
}

// Initable represents any ServiceConfig which is compatible with
// `tbot init`.
type Initable interface {
	GetDestination() bot.Destination
	Init(ctx context.Context) error
	Describe() []FileDescription
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
// bot.Destination implementation, if possible.
func DestinationFromURI(uriString string) (bot.Destination, error) {
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
		return &DestinationDirectory{
			Path: uri.Path,
		}, nil
	case "memory":
		if uri.Host != "" || uri.Path != "" {
			return nil, trace.BadParameter(
				"memory-backed data storage should not have host or path specified",
			)
		}
		return &DestinationMemory{}, nil
	case "kubernetes-secret":
		if uri.Host != "" {
			return nil, trace.BadParameter(
				"kubernetes-secret scheme should not be specified with host",
			)
		}
		if uri.Path == "" {
			return nil, trace.BadParameter(
				"kubernetes-secret scheme should have a path specified",
			)
		}
		// kubernetes-secret:///my-secret
		// TODO(noah): Eventually we'll support namespace in the host part of
		// the URI. For now, we'll default to the namespace tbot is running in.

		// Path will be prefixed with '/' so we'll strip it off.
		secretName := strings.TrimPrefix(uri.Path, "/")

		return &DestinationKubernetesSecret{
			Name: secretName,
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

// CredentialLifetime contains configuration for how long credentials will
// last (TTL) and the frequency at which they'll be renewed (RenewalInterval).
//
// It's a member on the BotConfig and service/output config structs, marked with
// the `inline` YAML tag so its fields become individual fields in the YAML
// config format.
type CredentialLifetime struct {
	TTL             time.Duration `yaml:"credential_ttl,omitempty"`
	RenewalInterval time.Duration `yaml:"renewal_interval,omitempty"`
	// skipMaxTTLValidation is used by services that do not abide by standard
	// teleport credential lifetime limits to override the check that the
	// user specified TTL is less than the max TTL. For example, X509 SVIDs can
	// be issued with a lifetime of up to 2 weeks.
	skipMaxTTLValidation bool
}

// IsEmpty returns whether none of the fields is set (i.e. it is unconfigured).
func (l CredentialLifetime) IsEmpty() bool {
	// We don't care about this field being set when checking empty state.
	l.skipMaxTTLValidation = false
	return l == CredentialLifetime{}
}

// Validate checks whether the combination of the fields is valid.
func (l CredentialLifetime) Validate(oneShot bool) error {
	if l.IsEmpty() {
		return nil
	}

	if l.TTL == 0 || l.RenewalInterval == 0 {
		return trace.BadParameter("credential_ttl and renewal_interval must both be specified if either is")
	}

	if l.TTL < 0 {
		return trace.BadParameter("credential_ttl must be positive")
	}

	if l.RenewalInterval < 0 {
		return trace.BadParameter("renewal_interval must be positive")
	}

	if l.TTL < l.RenewalInterval && !oneShot {
		return SuboptimalCredentialTTLError{
			msg: "Credential TTL is shorter than the renewal interval. This is likely an invalid configuration. Increase the credential TTL or decrease the renewal interval",
			details: map[string]any{
				"ttl":      l.TTL,
				"interval": l.RenewalInterval,
			},
		}
	}

	if !l.skipMaxTTLValidation && l.TTL > defaults.MaxRenewableCertTTL {
		return SuboptimalCredentialTTLError{
			msg: "Requested certificate TTL exceeds the maximum TTL allowed and will likely be reduced by the Teleport server",
			details: map[string]any{
				"requested_ttl": l.TTL,
				"maximum_ttl":   defaults.MaxRenewableCertTTL,
			},
		}
	}

	return nil
}

// SuboptimalCredentialTTLError is returned from CredentialLifetime.Validate
// when the user has set CredentialTTL to something unusual that we can work
// around (e.g. if they exceed MaxRenewableCertTTL the server will reduce it)
// rather than rejecting their configuration.
//
// In the future, these probably *should* be hard failures - but that would be
// a breaking change.
type SuboptimalCredentialTTLError struct {
	msg     string
	details map[string]any
}

// Error satisfies the error interface.
func (e SuboptimalCredentialTTLError) Error() string {
	if len(e.details) == 0 {
		return e.msg
	}
	parts := make([]string, 0, len(e.details))
	for k, v := range e.details {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return fmt.Sprintf("%s (%s)", e.msg, strings.Join(parts, ", "))
}

// LogLabels returns the error's details as a slice that can be passed as the
// varadic args parameter to log functions.
func (e SuboptimalCredentialTTLError) LogLabels() []any {
	labels := make([]any, 0, len(e.details)*2)
	for k, v := range e.details {
		labels = append(labels, k, v)
	}
	return labels
}
