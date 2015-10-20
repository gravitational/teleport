package service

import (
	"io/ioutil"
	"strings"

	"github.com/gravitational/configure"
	outils "github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/orbit/lib/utils"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/utils"
)

func ParseYAMLFile(path string, cfg interface{}) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return trace.Wrap(err)
	}
	rendered, err := renderTemplate(bytes)
	if err != nil {
		return trace.Wrap(err)
	}
	return configure.ParseYAML(rendered, cfg)
}

func ParseEnv(cfg interface{}) error {
	return configure.ParseEnv(cfg)
}

type Config struct {
	Log LogConfig `yaml:"log"`

	DataDir string `yaml:"data_dir" env:"TELEPORT_DATA_DIR"`
	FQDN    string `yaml:"fqdn" env:"TELEPORT_FQDN"`

	AuthServers NetAddrSlice `yaml:"auth_servers,flow" env:"TELEPORT_AUTH_SERVERS"`

	SSH  SSHConfig  `yaml:"ssh"`
	Auth AuthConfig `yaml:"auth"`
	Tun  TunConfig  `yaml:"tun"`
}

type LogConfig struct {
	Output   string `yaml:"output" env:"TELEPORT_LOG_OUTPUT"`
	Severity string `yaml:"severity" env:"TELEPORT_LOG_SEVERITY"`
}

type AuthConfig struct {
	// Enabled turns auth role on or off on this machine
	Enabled bool `yaml:"enabled" env:"TELEPORT_AUTH_ENABLED"`
	// HTTPAddr is the listening address for HTTP service API
	HTTPAddr utils.NetAddr `yaml:"http_addr" env:"TELEPORT_AUTH_HTTP_ADDR"`
	// SSHAddr is the listening address of SSH tunnel to HTTP service
	SSHAddr utils.NetAddr `yaml:"ssh_addr" env:"TELEPORT_AUTH_SSH_ADDR"`
	// Domain is Host Certificate Authority domain name
	Domain string `yaml:"domain" env:"TELEPORT_AUTH_DOMAIN"`
	// Token is a provisioning token for new auth server joining the cluster
	Token string `yaml:"token" env:"TELEPORT_AUTH_TOKEN"`
	// SecretKey is an encryption key for secret service, will be used
	// to initialize secret service if set
	SecretKey string `yaml:"secret_key" env:"TELEPORT_AUTH_SECRET_KEY"`

	// AllowedTokens is a set of tokens that will be added as trusted
	AllowedTokens KeyVal `yaml:"allowed_tokens" env:"TELEPORT_AUTH_ALLOWED_TOKENS"`

	// TrustedUserCertAuthorities is a set of trusted user certificate authorities
	TrustedUserAuthorities KeyVal `yaml:"trusted_user_authorities" env:"TELEPORT_AUTH_TRUSTED_USER_AUTHORITIES"`

	// KeysBackend configures backend that stores encryption keys
	KeysBackend struct {
		// Type is a backend type - etcd or boltdb
		Type string `yaml:"type" env:"TELEPORT_AUTH_KEYS_BACKEND_TYPE"`
		// Params is map with backend specific parameters
		Params KeyVal `yaml:"params,flow" env:"TELEPORT_AUTH_KEYS_BACKEND_PARAMS"`
		// AdditionalKey is a additional signing GPG key
		AdditionalKey string `yaml:"additional_key" env:"TELEPORT_AUTH_KEYS_BACKEND_ADDITIONAL_KEY"`
	} `yaml:"keys_backend"`

	// EventsBackend configures backend that stores cluster events (login attempts, etc)
	EventsBackend struct {
		// Type is a backend type, etcd or bolt
		Type string `yaml:"type" env:"TELEPORT_AUTH_EVENTS_BACKEND_TYPE"`
		// Params is map with backend specific parameters
		Params KeyVal `yaml:"params,flow" env:"TELEPORT_AUTH_EVENTS_BACKEND_PARAMS"`
	} `yaml:"events_backend"`

	// RecordsBackend configures backend that stores live SSH sessions recordings
	RecordsBackend struct {
		// Type is a backend type, currently only bolt
		Type string `yaml:"type" env:"TELEPORT_AUTH_RECORDS_BACKEND_TYPE"`
		// Params is map with backend specific parameters
		Params KeyVal `yaml:"params,flow" env:"TELEPORT_AUTH_RECORDS_BACKEND_PARAMS"`
	} `yaml:"records_backend"`
}

// SSHConfig configures SSH server node role
type SSHConfig struct {
	Enabled bool          `yaml:"enabled" env:"TELEPORT_SSH_ENABLED"`
	Token   string        `yaml:"token" env:"TELEPORT_SSH_TOKEN"`
	Addr    utils.NetAddr `yaml:"addr" env:"TELEPORT_SSH_ADDR"`
	Shell   string        `yaml:"shell" env:"TELEPORT_SSH_SHELL"`
}

// TunConfig configures reverse tunnel role
type TunConfig struct {
	Enabled    bool          `yaml:"enabled" env:"TELEPORT_TUN_ENABLED"`
	Token      string        `yaml:"token" env:"TELEPORT_TUN_TOKEN"`
	ServerAddr utils.NetAddr `yaml:"server_addr" env:"TELEPORT_TUN_SERVER_ADDR"`
}

type NetAddrSlice []utils.NetAddr

func (s *NetAddrSlice) Set(val string) error {
	values := outils.SplitComma(val)
	out := make([]utils.NetAddr, len(values))
	for i, v := range values {
		a, err := utils.ParseAddr(v)
		if err != nil {
			return trace.Wrap(err)
		}
		out[i] = *a
	}
	*s = out
	return nil
}

type KeyVal map[string]string

// Set accepts string with arguments in the form "key:val,key2:val2"
func (kv *KeyVal) Set(v string) error {
	if len(*kv) == 0 {
		*kv = make(map[string]string)
	}
	for _, i := range outils.SplitComma(v) {
		vals := strings.SplitN(i, ":", 2)
		if len(vals) != 2 {
			return trace.Errorf("extra options should be defined like KEY:VAL")
		}
		(*kv)[vals[0]] = vals[1]
	}
	return nil
}

func setDefaults(cfg *Config) {
	if cfg.Log.Output == "" {
		cfg.Log.Output = "console"
	}
	if cfg.Log.Severity == "" {
		cfg.Log.Severity = "INFO"
	}
	if cfg.SSH.Addr.IsEmpty() {
		cfg.SSH.Addr = utils.NetAddr{
			Network: "tcp",
			Addr:    "127.0.0.1:33001",
		}
	}
	if cfg.SSH.Shell == "" {
		cfg.SSH.Shell = "/bin/bash"
	}

	if cfg.Auth.HTTPAddr.IsEmpty() {
		cfg.Auth.HTTPAddr = utils.NetAddr{
			Network: "unix",
			Addr:    "/tmp/teleport.auth.sock",
		}
	}
	if cfg.Auth.SSHAddr.IsEmpty() {
		cfg.Auth.SSHAddr = utils.NetAddr{
			Network: "tcp",
			Addr:    "127.0.0.1:33000",
		}
	}
}
