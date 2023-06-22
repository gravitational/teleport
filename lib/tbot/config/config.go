/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
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

var SupportedJoinMethods = []string{
	string(types.JoinMethodToken),
	string(types.JoinMethodAzure),
	string(types.JoinMethodCircleCI),
	string(types.JoinMethodGitHub),
	string(types.JoinMethodGitLab),
	string(types.JoinMethodIAM),
}

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

// RemainingArgs is a custom kingpin parser that consumes all remaining
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

	// AuthServer is a Teleport auth server address. It may either point
	// directly to an auth server, or to a Teleport proxy server in which case
	// a tunneled auth connection will be established.
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

	// Proxy is the teleport proxy address. Unlike `AuthServer` this must
	// explicitly point to a Teleport proxy.
	Proxy string

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
type BotConfig struct {
	Version    Version          `yaml:"version"`
	Onboarding OnboardingConfig `yaml:"onboarding,omitempty"`
	Storage    *StorageConfig   `yaml:"storage,omitempty"`
	Outputs    Outputs          `yaml:"outputs,omitempty"`

	Debug           bool          `yaml:"debug"`
	AuthServer      string        `yaml:"auth_server"`
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
	for _, output := range conf.Outputs {
		if err := output.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		// This check currently only handles directory destinations, but we'll
		// need to create a more polymorphic way of doing this when we introduce
		// more destination types.
		directoryDestination, ok := output.GetDestination().(*DestinationDirectory)
		if ok {
			destinationPaths[directoryDestination.Path]++
		}
	}
	// Check for outputs reusing the same destination. This is a deeply
	// uncharted/unknown behaviour area. For now we'll emit a heavy warning,
	// in 15+ this will be an explicit area as outputs writing over one another
	// is too complex to support.
	for path, count := range destinationPaths {
		if count > 1 {
			log.WithField("path", path).Error(
				"Multiple outputs reusing the same destination path. This can produce unusable results. In Teleport 15.0, this will be a fatal error.",
			)
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
		config, err = ReadConfigFromFile(cf.ConfigPath)

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

	return config, nil
}

// ReadConfigFromFile reads and parses a YAML config from a file.
func ReadConfigFromFile(filePath string) (*BotConfig, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, trace.Wrap(err, fmt.Sprintf("failed to open file: %v", filePath))
	}

	defer f.Close()
	return ReadConfig(f)
}

type Version string

var (
	V1 Version = "v1"
	V2 Version = "v2"
)

// ReadConfig parses a YAML config file from a Reader.
func ReadConfig(reader io.ReadSeeker) (*BotConfig, error) {
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
	decoder.KnownFields(true)

	switch version.Version {
	case V1, "":
		panic("migration code will be inserted here in follow up PR")
	case V2:
		config := &BotConfig{}
		if err := decoder.Decode(config); err != nil {
			return nil, trace.BadParameter("failed parsing config file: %s", strings.Replace(err.Error(), "\n", "", -1))
		}
		return config, nil
	default:
		return nil, trace.BadParameter("unrecognized config version %q", version.Version)
	}
}
