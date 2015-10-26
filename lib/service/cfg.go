package service

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/configure"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
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

	DataDir  string `yaml:"data_dir" env:"TELEPORT_DATA_DIR"`
	Hostname string `yaml:"hostname" env:"TELEPORT_HOSTNAME"`

	AuthServers NetAddrSlice `yaml:"auth_servers,flow" env:"TELEPORT_AUTH_SERVERS"`

	// SSH role an SSH endpoint server
	SSH SSHConfig `yaml:"ssh"`

	// Auth server authentication and authorizatin server config
	Auth AuthConfig `yaml:"auth"`

	// ReverseTunnnel role creates and mantains outbound SSH reverse tunnel to the proxy
	ReverseTunnel ReverseTunnelConfig `yaml:"reverse_tunnel"`

	// Proxy is SSH proxy that manages incoming and outbound connections
	// via multiple reverse tunnels
	Proxy ProxyConfig `yaml:"proxy"`
}

type LogConfig struct {
	Output   string `yaml:"output" env:"TELEPORT_LOG_OUTPUT"`
	Severity string `yaml:"severity" env:"TELEPORT_LOG_SEVERITY"`
}

type ProxyConfig struct {
	// Enabled turns proxy role on or off for this process
	Enabled bool `yaml:"enabled" env:"TELEPORT_PROXY_ENABLED"`

	// Token is a provisioning token for new proxy server registering with auth
	Token string `yaml:"token" env:"TELEPORT_PROXY_TOKEN"`

	// ReverseTunnelListenAddr is address where reverse tunnel dialers connect to
	ReverseTunnelListenAddr utils.NetAddr `yaml:"reverse_tunnel_listen_addr" env:"TELEPORT_PROXY_REVERSE_TUNNEL_LISTEN_ADDR"`

	// WebAddr is address for web portal of the proxy
	WebAddr utils.NetAddr `yaml:"web_addr" env:"TELEPORT_PROXY_WEB_ADDR"`

	// AssetsDir is a directory with proxy website assets
	AssetsDir string `yaml:"assets_dir" env:"TELEPORT_PROXY_ASSETS_DIR"`

	// TLSKey is a base64 encoded private key used by web portal
	TLSKey string `yaml:"tls_key" env:"TELEPORT_PROXY_TLS_KEY"`

	// TLSCert is a base64 encoded certificate used by web portal
	TLSCert string `yaml:"tlscert" env:"TELEPORT_PROXY_TLS_CERT"`
}

type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool `yaml:"enabled" env:"TELEPORT_AUTH_ENABLED"`

	// HTTPAddr is the listening address for HTTP service API
	HTTPAddr utils.NetAddr `yaml:"http_addr" env:"TELEPORT_AUTH_HTTP_ADDR"`

	// SSHAddr is the listening address of SSH tunnel to HTTP service
	SSHAddr utils.NetAddr `yaml:"ssh_addr" env:"TELEPORT_AUTH_SSH_ADDR"`

	// HostAuthorityDomain is Host Certificate Authority domain name
	HostAuthorityDomain string `yaml:"host_authority_domain" env:"TELEPORT_AUTH_HOST_AUTHORITY_DOMAIN"`

	// Token is a provisioning token for new auth server joining the cluster
	Token string `yaml:"token" env:"TELEPORT_AUTH_TOKEN"`

	// SecretKey is an encryption key for secret service, will be used
	// to initialize secret service if set
	SecretKey string `yaml:"secret_key" env:"TELEPORT_AUTH_SECRET_KEY"`

	// AllowedTokens is a set of tokens that will be added as trusted
	AllowedTokens KeyVal `yaml:"allowed_tokens" env:"TELEPORT_AUTH_ALLOWED_TOKENS"`

	// TrustedAuthorities is a set of trusted user certificate authorities
	TrustedAuthorities RemoteCerts `yaml:"trusted_authorities" env:"TELEPORT_AUTH_TRUSTED_AUTHORITIES"`

	// UserCA allows to pass preconfigured user certificate authority keypair
	// to auth server so it will use it on the first start instead of generating
	// a new keypair
	UserCA CertificateAuthority `yaml:"user_ca_keypair" env:"TELEPORT_AUTH_USER_CA_KEYPAIR"`

	// HostCA allows to pass preconfigured host certificate authority keypair
	// to auth server so it will use it on the first start instead of generating
	// a new keypair
	HostCA CertificateAuthority `yaml:"host_ca_keypair" env:"TELEPORT_AUTH_HOST_CA_KEYPAIR"`

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

// ReverseTunnelConfig configures reverse tunnel role
type ReverseTunnelConfig struct {
	Enabled  bool          `yaml:"enabled" env:"TELEPORT_REVERSE_TUNNEL_ENABLED"`
	Token    string        `yaml:"token" env:"TELEPORT_REVERSE_TUNNEL_TOKEN"`
	DialAddr utils.NetAddr `yaml:"dial_addr" env:"TELEPORT_REVERSE_TUNNEL_DIAL_ADDR"`
}

type NetAddrSlice []utils.NetAddr

func (s *NetAddrSlice) Set(val string) error {
	values := configure.SplitComma(val)
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
	for _, i := range configure.SplitComma(v) {
		vals := strings.SplitN(i, ":", 2)
		if len(vals) != 2 {
			return trace.Errorf("extra options should be defined like KEY:VAL")
		}
		(*kv)[vals[0]] = vals[1]
	}
	return nil
}

type RemoteCert struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	FQDN  string `json:"fqdn"`
	Value string `json:"value"`
}

type RemoteCerts []RemoteCert

func (c *RemoteCerts) SetEnv(v string) error {
	var certs []RemoteCert
	if err := json.Unmarshal([]byte(v), &certs); err != nil {
		return trace.Wrap(err, "expected JSON encoded remote certificate")
	}
	*c = certs
	return nil
}

type CertificateAuthority struct {
	PublicKey  string `json:"public" yaml:"public"`
	PrivateKey string `json:"private" yaml:"private"`
}

func (c *CertificateAuthority) SetEnv(v string) error {
	var ca CertificateAuthority
	if err := json.Unmarshal([]byte(v), &ca); err != nil {
		return trace.Wrap(err, "expected JSON encoded certificate authority")
	}
	key, err := base64.StdEncoding.DecodeString(ca.PrivateKey)
	if err != nil {
		return trace.Wrap(err, "private key should be base64 encoded")
	}
	c.PublicKey = ca.PublicKey
	c.PrivateKey = string(key)
	if c.PrivateKey == "" || c.PublicKey == "" {
		return trace.Errorf("both public key and private key should be setup")
	}
	return nil
}

func (c *CertificateAuthority) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var ca struct {
		PublicKey  string `json:"public" yaml:"public"`
		PrivateKey string `json:"private" yaml:"private"`
	}
	if err := unmarshal(&ca); err != nil {
		return trace.Wrap(err)
	}
	key, err := base64.StdEncoding.DecodeString(ca.PrivateKey)
	if err != nil {
		return trace.Wrap(err, "private key should be base64 encoded")
	}
	c.PublicKey = ca.PublicKey
	c.PrivateKey = string(key)
	if c.PrivateKey == "" || c.PublicKey == "" {
		return trace.Errorf("both public key and private key should be setup")
	}
	return nil
}

func (c *CertificateAuthority) ToCA() *services.CA {
	return &services.CA{
		Pub:  []byte(c.PublicKey),
		Priv: []byte(c.PrivateKey),
	}
}

func convertRemoteCerts(inCerts RemoteCerts) []services.RemoteCert {
	outCerts := make([]services.RemoteCert, len(inCerts))
	for i, v := range inCerts {
		outCerts[i] = services.RemoteCert{
			ID:    v.ID,
			FQDN:  v.FQDN,
			Type:  v.Type,
			Value: []byte(v.Value),
		}
	}
	return outCerts
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
	if cfg.Proxy.ReverseTunnelListenAddr.IsEmpty() {
		cfg.Proxy.ReverseTunnelListenAddr = utils.NetAddr{
			Network: "tcp",
			Addr:    "127.0.0.1:33006",
		}
	}
	if cfg.Proxy.WebAddr.IsEmpty() {
		cfg.Proxy.WebAddr = utils.NetAddr{
			Network: "tcp",
			Addr:    "127.0.0.1:33007",
		}
	}
}
