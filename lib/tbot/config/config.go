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
	"fmt"
	"io"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	DefaultCertificateTTL = 60 * time.Minute
	DefaultRenewInterval  = 20 * time.Minute
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/config")

var SupportedJoinMethods = []string{
	string(types.JoinMethodAzure),
	string(types.JoinMethodCircleCI),
	string(types.JoinMethodGCP),
	string(types.JoinMethodGitHub),
	string(types.JoinMethodGitLab),
	string(types.JoinMethodIAM),
	string(types.JoinMethodKubernetes),
	string(types.JoinMethodSpacelift),
	string(types.JoinMethodToken),
}

var log = logrus.WithFields(logrus.Fields{
	teleport.ComponentKey: teleport.ComponentTBot,
})

// RemainingArgsList is a custom kingpin parser that consumes all remaining
// arguments.
type RemainingArgsList []string

func (r *RemainingArgsList) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func (r *RemainingArgsList) String() string {
	return strings.Join([]string(*r), " ")
}

func (r *RemainingArgsList) IsCumulative() bool {
	return true
}

// RemainingArgs returns a list of remaining arguments for the given command.
func RemainingArgs(s kingpin.Settings) (target *[]string) {
	target = new([]string)
	s.SetValue((*RemainingArgsList)(target))
	return
}

// CLIConf is configuration from the CLI.
type CLIConf struct {
	ConfigPath string

	Debug bool

	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string

	// AuthServer is a Teleport auth server address. It may either point
	// directly to an auth server, or to a Teleport proxy server in which case
	// a tunneled auth connection will be established.
	// Prefer using Address() to pick an address.
	AuthServer string

	// DataDir stores the bot's internal data.
	DataDir string

	// DestinationDir stores the generated end-user certificates.
	DestinationDir string

	// CAPins is a list of pinned SKPI hashes of trusted auth server CAs, used
	// only on first connect.
	CAPins []string

	// Token is a bot join token.
	Token string

	// RenewalInterval is the interval at which certificates are renewed, as a
	// time.ParseDuration() string. It must be less than the certificate TTL.
	RenewalInterval time.Duration

	// CertificateTTL is the requested TTL of certificates. It should be some
	// multiple of the renewal interval to allow for failed renewals.
	CertificateTTL time.Duration

	// JoinMethod is the method the bot should use to exchange a token for the
	// initial certificate
	JoinMethod string

	// Oneshot controls whether the bot quits after a single renewal.
	Oneshot bool

	// InitDir specifies which Destination to initialize if multiple are
	// configured.
	InitDir string

	// BotUser is a Unix username that should be given permission to write
	BotUser string

	// ReaderUser is the Unix username that will be reading the files
	ReaderUser string

	// Owner is the user:group that will own the Destination files. Due to SSH
	// restrictions on key permissions, it cannot be the same as the reader
	// user. If ACL support is unused or unavailable, the reader user will own
	// files directly.
	Owner string

	// Clean is a flag that, if set, instructs `tbot init` to remove existing
	// unexpected files.
	Clean bool

	// ConfigureOutput provides a path that the generated configuration file
	// should be written to
	ConfigureOutput string

	// ProxyServer is the teleport proxy address. Unlike `AuthServer` this must
	// explicitly point to a Teleport proxy.
	// Example: "example.teleport.sh:443"
	ProxyServer string

	// Cluster is the name of the Teleport cluster on which resources should
	// be accessed.
	Cluster string

	// RemainingArgs is the remaining string arguments for commands that
	// require them.
	RemainingArgs []string

	// FIPS instructs `tbot` to run in a mode designed to comply with FIPS
	// regulations. This means the bot should:
	// - Refuse to run if not compiled with boringcrypto
	// - Use FIPS relevant endpoints for cloud providers (e.g AWS)
	// - Restrict TLS / SSH cipher suites and TLS version
	// - RSA2048 should be used for private key generation
	FIPS bool

	// DiagAddr is the address the diagnostics http service should listen on.
	// If not set, no diagnostics listener is created.
	DiagAddr string

	// Insecure instructs `tbot` to trust the Auth Server without verifying the CA.
	Insecure bool

	// Trace indicates whether tracing should be enabled.
	Trace bool

	// TraceExporter is a manually provided URI to send traces to instead of
	// forwarding them to the Auth service.
	TraceExporter string
}

// AzureOnboardingConfig holds configuration relevant to the "azure" join method.
type AzureOnboardingConfig struct {
	// ClientID of the managed identity to use. Required if the VM has more
	// than one assigned identity.
	ClientID string `yaml:"client_id,omitempty"`
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
}

// HasToken gives the ability to check if there has been a token value stored
// in the config
func (conf *OnboardingConfig) HasToken() bool {
	return conf.TokenValue != ""
}

// RenewableJoinMethod indicates that certificate renewal should be used with
// this join method rather than rejoining each time.
func (conf *OnboardingConfig) RenewableJoinMethod() bool {
	return conf.JoinMethod == types.JoinMethodToken
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
	Outputs    Outputs          `yaml:"outputs,omitempty"`
	Services   ServiceConfigs   `yaml:"services,omitempty"`

	Debug      bool   `yaml:"debug"`
	AuthServer string `yaml:"auth_server,omitempty"`
	// ProxyServer is the teleport proxy address. Unlike `AuthServer` this must
	// explicitly point to a Teleport proxy.
	// Example: "example.teleport.sh:443"
	ProxyServer     string        `yaml:"proxy_server,omitempty"`
	CertificateTTL  time.Duration `yaml:"certificate_ttl"`
	RenewalInterval time.Duration `yaml:"renewal_interval"`
	Oneshot         bool          `yaml:"oneshot"`
	// FIPS instructs `tbot` to run in a mode designed to comply with FIPS
	// regulations. This means the bot should:
	// - Refuse to run if not compiled with boringcrypto
	// - Use FIPS relevant endpoints for cloud providers (e.g AWS)
	// - Restrict TLS / SSH cipher suites and TLS version
	// - RSA2048 should be used for private key generation
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

	destinationPaths := map[string]int{}
	addDestinationToKnownPaths := func(d bot.Destination) {
		switch d := d.(type) {
		case *DestinationDirectory:
			destinationPaths[fmt.Sprintf("file://%s", d.Path)]++
		case *DestinationKubernetesSecret:
			destinationPaths[fmt.Sprintf("kubernetes-secret://%s", d.Name)]++
		}
	}
	for _, output := range conf.Outputs {
		if err := output.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		addDestinationToKnownPaths(output.GetDestination())
	}

	// Check for identical destinations being used. This is a deeply
	// uncharted/unknown behavior area. For now we'll emit a heavy warning,
	// in 15+ this will be an explicit area as outputs writing over one another
	// is too complex to support.
	addDestinationToKnownPaths(conf.Storage.Destination)
	for path, count := range destinationPaths {
		if count > 1 {
			log.WithField("path", path).Error(
				"Identical destinations used within config. This can produce unusable results. In Teleport 15.0, this will be a fatal error.",
			)
		}
	}

	// Validate configured services
	for i, service := range conf.Services {
		if err := service.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating service[%d]", i)
		}
	}

	if conf.CertificateTTL == 0 {
		conf.CertificateTTL = DefaultCertificateTTL
	}

	if conf.RenewalInterval == 0 {
		conf.RenewalInterval = DefaultRenewInterval
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

	// Warn about config where renewals will fail due to weird TTL vs Interval
	if !conf.Oneshot && conf.RenewalInterval > conf.CertificateTTL {
		log.Warnf(
			"Certificate TTL (%s) is shorter than the renewal interval (%s). This is likely an invalid configuration. Increase the certificate TTL or decrease the renewal interval.",
			conf.CertificateTTL,
			conf.RenewalInterval,
		)
	}

	return nil
}

// ServiceConfig is an interface over the various service configurations.
type ServiceConfig interface {
	Type() string
	CheckAndSetDefaults() error
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
		default:
			return trace.BadParameter("unrecognized service type (%s)", header.Type)
		}
	}

	*o = out
	return nil
}

// Outputs assists polymorphic unmarshaling of a slice of Outputs
type Outputs []Output

func (o *Outputs) UnmarshalYAML(node *yaml.Node) error {
	var out []Output
	for _, node := range node.Content {
		header := struct {
			Type string `yaml:"type"`
		}{}
		if err := node.Decode(&header); err != nil {
			return trace.Wrap(err)
		}

		switch header.Type {
		case IdentityOutputType:
			v := &IdentityOutput{}
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
		case KubernetesOutputType:
			v := &KubernetesOutput{}
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
		case SSHHostOutputType:
			v := &SSHHostOutput{}
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
		default:
			return trace.BadParameter("unrecognized output type (%s)", header.Type)
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

// GetOutputByPath attempts to fetch a Destination by its filesystem path.
// Only valid for filesystem destinations; returns nil if no matching
// Destination exists.
func (conf *BotConfig) GetOutputByPath(path string) (Output, error) {
	for _, output := range conf.Outputs {
		destImpl := output.GetDestination()

		destDir, ok := destImpl.(*DestinationDirectory)
		if !ok {
			continue
		}

		// Note: this compares only paths as written in the config file. We
		// might want to compare .Abs() if that proves to be confusing (though
		// this may have its own problems)
		if destDir.Path == path {
			return output, nil
		}
	}

	return nil, nil
}

// newTestConfig creates a new minimal bot configuration from defaults for use
// in tests
func newTestConfig(authServer string) (*BotConfig, error) {
	// Note: we need authServer for CheckAndSetDefaults to succeed.
	cfg := BotConfig{
		AuthServer: authServer,
		Onboarding: OnboardingConfig{
			JoinMethod: types.JoinMethodToken,
		},
	}
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cfg, nil
}

func destinationFromURI(uriString string) (bot.Destination, error) {
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
	default:
		return nil, trace.BadParameter(
			"unrecognized data storage scheme",
		)
	}
}

// FromCLIConf loads bot config from CLI parameters, potentially loading and
// merging a configuration file if specified. CheckAndSetDefaults() will
// be called. Note that CLI flags, if specified, will override file values.
func FromCLIConf(cf *CLIConf) (*BotConfig, error) {
	var config *BotConfig
	var err error

	if cf.ConfigPath != "" {
		config, err = ReadConfigFromFile(cf.ConfigPath, false)

		if err != nil {
			return nil, trace.Wrap(err, "loading bot config from path %s", cf.ConfigPath)
		}
	} else {
		config = &BotConfig{}
	}

	if cf.Debug {
		config.Debug = true
	}

	if cf.Oneshot {
		config.Oneshot = true
	}

	if cf.AuthServer != "" {
		if config.AuthServer != "" {
			log.Warnf("CLI parameters are overriding auth server configured in %s", cf.ConfigPath)
		}
		config.AuthServer = cf.AuthServer
	}

	if cf.ProxyServer != "" {
		if config.ProxyServer != "" {
			log.Warnf("CLI parameters are overriding proxy configured in %s", cf.ConfigPath)
		}
		config.ProxyServer = cf.ProxyServer
	}

	if cf.CertificateTTL != 0 {
		if config.CertificateTTL != 0 {
			log.Warnf("CLI parameters are overriding certificate TTL configured in %s", cf.ConfigPath)
		}
		config.CertificateTTL = cf.CertificateTTL
	}

	if cf.RenewalInterval != 0 {
		if config.RenewalInterval != 0 {
			log.Warnf("CLI parameters are overriding renewal interval configured in %s", cf.ConfigPath)
		}
		config.RenewalInterval = cf.RenewalInterval
	}

	// DataDir overrides any previously-configured storage config
	if cf.DataDir != "" {
		if config.Storage != nil && config.Storage.Destination != nil {
			log.Warnf(
				"CLI parameters are overriding storage location from %s",
				cf.ConfigPath,
			)
		}
		dest, err := destinationFromURI(cf.DataDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		config.Storage = &StorageConfig{Destination: dest}
	}

	if cf.DestinationDir != "" {
		// WARNING:
		// See: https://github.com/gravitational/teleport/issues/27206 for
		// potential gotchas that currently exist when dealing with this
		// override behavior.

		// CLI only supports a single filesystem Destination with SSH client config
		// and all roles.
		if len(config.Outputs) > 0 {
			log.Warnf("CLI parameters are overriding destinations from %s", cf.ConfigPath)
		}

		// When using the CLI --Destination-dir we configure an Identity type
		// output for that directory.
		config.Outputs = []Output{
			&IdentityOutput{
				Destination: &DestinationDirectory{
					Path: cf.DestinationDir,
				},
			},
		}
	}

	// If any onboarding flags are set, override the whole section.
	// (CAPath, CAPins, etc follow different codepaths so we don't want a
	// situation where different fields become set weirdly due to struct
	// merging)
	if cf.Token != "" || cf.JoinMethod != "" || len(cf.CAPins) > 0 {
		if !reflect.DeepEqual(config.Onboarding, OnboardingConfig{}) {
			// To be safe, warn about possible confusion.
			log.Warnf("CLI parameters are overriding onboarding config from %s", cf.ConfigPath)
		}

		config.Onboarding = OnboardingConfig{
			CAPins:     cf.CAPins,
			JoinMethod: types.JoinMethod(cf.JoinMethod),
		}
		config.Onboarding.SetToken(cf.Token)
	}

	if cf.FIPS {
		config.FIPS = cf.FIPS
	}

	if cf.DiagAddr != "" {
		if config.DiagAddr != "" {
			log.Warnf("CLI parameters are overriding diagnostics address configured in %s", cf.ConfigPath)
		}
		config.DiagAddr = cf.DiagAddr
	}

	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "validating merged bot config")
	}

	if cf.Insecure {
		config.Insecure = true
	}

	return config, nil
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
		return nil, trace.BadParameter("failed parsing config file version: %s", strings.Replace(err.Error(), "\n", "", -1))
	}

	// Reset reader and decoder
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return nil, trace.Wrap(err)
	}
	decoder = yaml.NewDecoder(reader)

	switch version.Version {
	case V1, "":
		if !manualMigration {
			log.Warn("Deprecated config version (V1) detected. Attempting to perform an on-the-fly in-memory migration to latest version. Please persist the config migration by following the guidance at https://goteleport.com/docs/machine-id/reference/v14-upgrade-guide/")
		}
		config := &configV1{}
		if err := decoder.Decode(config); err != nil {
			return nil, trace.BadParameter("failed parsing config file: %s", strings.Replace(err.Error(), "\n", "", -1))
		}
		latestConfig, err := config.migrate()
		if err != nil {
			return nil, trace.WithUserMessage(
				trace.Wrap(err, "migrating v1 config"),
				"Failed to migrate. See https://goteleport.com/docs/machine-id/reference/v14-upgrade-guide/",
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
			return nil, trace.BadParameter("failed parsing config file: %s", strings.Replace(err.Error(), "\n", "", -1))
		}
		return config, nil
	default:
		return nil, trace.BadParameter("unrecognized config version %q", version.Version)
	}
}
