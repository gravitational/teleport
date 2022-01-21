package config

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	STORAGE_DEFAULT_PATH = "/var/lib/teleport/bot"

	CONFIG_KIND_SSH = "ssh"
	CONFIG_KIND_TLS = "tls"

	CONFIG_TEMPLATE_SSH_CLIENT = "ssh_client"

	CONFIG_DEFAULT_CERTIFICATE_TTL = 60 * time.Minute
	CONFIG_DEFAULT_RENEW_INTERVAL  = 20 * time.Minute
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

// AllKinds lists all valid config kinds, intended for validation purposes.
var AllKinds = [...]string{CONFIG_KIND_SSH, CONFIG_KIND_TLS}

// AllConfigTemplates lists all valid config templates, intended for help
// messages
var AllConfigTemplates = [...]string{CONFIG_TEMPLATE_SSH_CLIENT}

// CLIConf is configuration from the CLI.
type CLIConf struct {
	ConfigPath string

	Debug      bool
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

	// RenewInterval is the interval at which certificates are renewed, as a
	// time.ParseDuration() string. It must be less than the certificate TTL.
	RenewInterval time.Duration

	// CertificateTTL is the requested TTL of certificates. It should be some
	// multiple of the renewal interval to allow for failed renewals.
	CertificateTTL time.Duration
}

// StorageConfig contains config parameters for the bot's internal certificate
// storage.
type StorageConfig struct {
	DestinationMixin `yaml:",inline"`
}

// storageDefaults applies default destinations for the bot's internal storage
// section.
func storageDefaults(dm *DestinationMixin) error {
	dm.Directory = &DestinationDirectory{
		Path: STORAGE_DEFAULT_PATH,
	}

	return nil
}

func (sc *StorageConfig) CheckAndSetDefaults() error {
	if err := sc.DestinationMixin.CheckAndSetDefaults(storageDefaults); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ConfigTemplateSSHClient contains parameters for the ssh_config config
// template
type ConfigTemplateSSHClient struct {
}

// ConfigTemplate contains all possible config template variants. Exactly one
// variant must be set to be considered valid.
type ConfigTemplate struct {
	SSHClient *ConfigTemplateSSHClient `yaml:"ssh_client,omitempty"`
}

func (c *ConfigTemplate) UnmarshalYAML(node *yaml.Node) error {
	var simpleTemplate string
	if err := node.Decode(&simpleTemplate); err == nil {
		switch simpleTemplate {
		case CONFIG_TEMPLATE_SSH_CLIENT:
			c.SSHClient = &ConfigTemplateSSHClient{}
		default:
			return trace.BadParameter(
				"invalid config template '%s' on line %d, expected one of: %s",
				simpleTemplate, node.Line, strings.Join(AllConfigTemplates[:], ", "),
			)
		}
		return nil
	}

	type rawTemplate ConfigTemplate
	if err := node.Decode((*rawTemplate)(c)); err != nil {
		return err
	}

	return nil
}

func (c *ConfigTemplate) CheckAndSetDefaults() error {
	notNilCount := 0

	if c.SSHClient != nil {
		notNilCount += 1
	}

	if notNilCount == 0 {
		return trace.BadParameter("config template must not be empty")
	} else if notNilCount > 1 {
		return trace.BadParameter("config template must have exactly one configuration")
	}

	return nil
}

// DestinationConfig configures a user certificate destination.
type DestinationConfig struct {
	DestinationMixin `yaml:",inline"`

	Roles   []string         `yaml:"roles,omitempty"`
	Kinds   []string         `yaml:"kinds,omitempty"`
	Configs []ConfigTemplate `yaml:"configs,omitempty"`
}

// destinationDefaults applies defaults for an output sink's destination. Since
// these have no sane defaults, in practice it just returns an error if no
// config is provided.
func destinationDefaults(dm *DestinationMixin) error {
	return trace.BadParameter("destinations require some valid output sink")
}

func (dc *DestinationConfig) CheckAndSetDefaults() error {
	if err := dc.DestinationMixin.CheckAndSetDefaults(destinationDefaults); err != nil {
		return trace.Wrap(err)
	}

	// Note: empty roles is allowed; interpreted to mean "all" at generation
	// time

	if len(dc.Kinds) == 0 && len(dc.Configs) == 0 {
		dc.Kinds = []string{CONFIG_KIND_SSH}
		dc.Configs = []ConfigTemplate{{
			SSHClient: &ConfigTemplateSSHClient{},
		}}
	} else {
		for _, cfg := range dc.Configs {
			if err := cfg.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// OnboardingConfig contains values only required on first connect.
type OnboardingConfig struct {
	// Token is a bot join token.
	Token string `yaml:"token"`

	// CAPath is an optional path to a CA certificate.
	CAPath string

	// CAPins is a list of certificate authority pins, used to validate the
	// connection to the Teleport auth server.
	CAPins []string `yaml:"ca_pins"`
}

// BotConfig is the bot's root config object.
type BotConfig struct {
	Onboarding   *OnboardingConfig   `yaml:"onboarding,omitempty"`
	Storage      StorageConfig       `yaml:"storage,omitempty"`
	Destinations []DestinationConfig `yaml:"destinations,omitempty"`

	Debug          bool          `yaml:"debug"`
	AuthServer     string        `yaml:"auth_server"`
	CertificateTTL time.Duration `yaml:"certificate_ttl"`
	RenewInterval  time.Duration `yaml:"renew_interval"`
}

func (conf *BotConfig) CheckAndSetDefaults() error {
	if err := conf.Storage.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	for _, dest := range conf.Destinations {
		if err := dest.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	if conf.AuthServer == "" {
		return trace.BadParameter("an auth server address must be configured")
	}

	if conf.CertificateTTL == 0 {
		conf.CertificateTTL = CONFIG_DEFAULT_CERTIFICATE_TTL
	}

	if conf.RenewInterval == 0 {
		conf.RenewInterval = CONFIG_DEFAULT_RENEW_INTERVAL
	}

	return nil
}

// NewDefaultConfig creates a new minimal bot configuration from defaults.
// CheckAndSetDefaults() will be called.
func NewDefaultConfig(authServer string) (*BotConfig, error) {
	// Note: we need authServer for CheckAndSetDefaults to succeed.
	cfg := BotConfig{
		AuthServer: authServer,
	}
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cfg, nil
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
			return nil, trace.WrapWithMessage(err, "loading bot config from path %s", cf.ConfigPath)
		}
	}

	if cf.Debug {
		config.Debug = true
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

	if cf.RenewInterval != 0 {
		if config.RenewInterval != 0 {
			log.Warnf("CLI parameters are overriding renewal interval configured in %s", cf.ConfigPath)
		}
		config.RenewInterval = cf.RenewInterval
	}

	// DataDir overrides any previously-configured storage config
	if cf.DataDir != "" {
		if _, err := config.Storage.GetDestination(); err != nil {
			log.Warnf("CLI parameters are overriding storage location from %s", cf.ConfigPath)
		}

		config.Storage = StorageConfig{
			DestinationMixin: DestinationMixin{
				Directory: &DestinationDirectory{
					Path: cf.DataDir,
				},
			},
		}
	}

	// If any onboarding flags are set, override the whole section.
	// (CAPath, CAPins, etc follow different codepaths so we don't want a
	// situation where different fields become set weirdly due to struct
	// merging)
	if cf.Token != "" || len(cf.CAPins) > 0 {
		if config.Onboarding.Token != "" || config.Onboarding.CAPath != "" || len(config.Onboarding.CAPins) > 0 {
			// To be safe, warn about possible confusion.
			log.Warn("CLI parameters are overriding onboarding config from %s", cf.ConfigPath)
		}

		config.Onboarding = &OnboardingConfig{
			Token:  cf.Token,
			CAPins: cf.CAPins,
		}
	}

	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.WrapWithMessage(err, "validing merged bot config")
	}

	return config, nil
}

// ReadFromFile reads and parses a YAML config from a file.
func ReadConfigFromFile(filePath string) (*BotConfig, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, trace.Wrap(err, fmt.Sprintf("failed to open file: %v", filePath))
	}

	defer f.Close()
	return ReadConfig(f)
}

// ReadConfig parses a YAML config file from a Reader.
func ReadConfig(reader io.Reader) (*BotConfig, error) {
	var config BotConfig

	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return nil, trace.BadParameter("failed parsing config file: %s", strings.Replace(err.Error(), "\n", "", -1))
	}

	return &config, nil
}
