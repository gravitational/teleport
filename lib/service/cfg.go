/*
Copyright 2015 Gravitational, Inc.

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
package service

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
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

func (cfg *Config) RoleConfig() RoleConfig {
	return RoleConfig{
		DataDir:     cfg.DataDir,
		Hostname:    cfg.Hostname,
		AuthServers: cfg.AuthServers,
		Auth:        cfg.Auth,
	}
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

	// SSHAddr is address of ssh proxy
	SSHAddr utils.NetAddr `yaml:"ssh_addr" env:"TELEPORT_PROXY_SSH_ADDR"`

	// AssetsDir is a directory with proxy website assets
	AssetsDir string `yaml:"assets_dir" env:"TELEPORT_PROXY_ASSETS_DIR"`

	// TLSKey is a base64 encoded private key used by web portal
	TLSKey string `yaml:"tls_key" env:"TELEPORT_PROXY_TLS_KEY"`

	// TLSCert is a base64 encoded certificate used by web portal
	TLSCert string `yaml:"tlscert" env:"TELEPORT_PROXY_TLS_CERT"`

	Limiter limiter.LimiterConfig `yaml:"limiter" env:"TELEPORT_PROXY_LIMITER"`
}

type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool `yaml:"enabled" env:"TELEPORT_AUTH_ENABLED"`

	// SSHAddr is the listening address of SSH tunnel to HTTP service
	SSHAddr utils.NetAddr `yaml:"ssh_addr" env:"TELEPORT_AUTH_SSH_ADDR"`

	// HostAuthorityDomain is Host Certificate Authority domain name
	HostAuthorityDomain string `yaml:"host_authority_domain" env:"TELEPORT_AUTH_HOST_AUTHORITY_DOMAIN"`

	// Token is a provisioning token for an additonal auth server joining the cluster
	Token string `yaml:"token" env:"TELEPORT_AUTH_TOKEN"`

	// SecretKey is an encryption key for secret service, will be used
	// to initialize secret service if set
	SecretKey string `yaml:"secret_key" env:"TELEPORT_AUTH_SECRET_KEY"`

	// AllowedTokens is a set of tokens that will be added as trusted
	AllowedTokens KeyVal `yaml:"allowed_tokens" env:"TELEPORT_AUTH_ALLOWED_TOKENS"`

	// TrustedAuthorities is a set of trusted user certificate authorities
	TrustedAuthorities CertificateAuthorities `yaml:"trusted_authorities" env:"TELEPORT_AUTH_TRUSTED_AUTHORITIES"`

	// UserCA allows to pass preconfigured user certificate authority keypair
	// to auth server so it will use it on the first start instead of generating
	// a new keypair
	UserCA LocalCertificateAuthority `yaml:"user_ca_keypair" env:"TELEPORT_AUTH_USER_CA_KEYPAIR"`

	// HostCA allows to pass preconfigured host certificate authority keypair
	// to auth server so it will use it on the first start instead of generating
	// a new keypair
	HostCA LocalCertificateAuthority `yaml:"host_ca_keypair" env:"TELEPORT_AUTH_HOST_CA_KEYPAIR"`

	// KeysBackend configures backend that stores auth keys, certificates, tokens ...
	KeysBackend struct {
		// Type is a backend type - etcd or boltdb
		Type string `yaml:"type" env:"TELEPORT_AUTH_KEYS_BACKEND_TYPE"`
		// Params is map with backend specific parameters
		Params string `yaml:"params,flow" env:"TELEPORT_AUTH_KEYS_BACKEND_PARAMS"`
		// AdditionalKey is a additional signing GPG key
		EncryptionKeys StringArray `yaml:"encryption_keys" env:"TELEPORT_AUTH_KEYS_BACKEND_ENCRYPTION_KEYS"`
	} `yaml:"keys_backend"`

	// EventsBackend configures backend that stores cluster events (login attempts, etc)
	EventsBackend struct {
		// Type is a backend type, etcd or bolt
		Type string `yaml:"type" env:"TELEPORT_AUTH_EVENTS_BACKEND_TYPE"`
		// Params is map with backend specific parameters
		Params string `yaml:"params,flow" env:"TELEPORT_AUTH_EVENTS_BACKEND_PARAMS"`
	} `yaml:"events_backend"`

	// RecordsBackend configures backend that stores live SSH sessions recordings
	RecordsBackend struct {
		// Type is a backend type, currently only bolt
		Type string `yaml:"type" env:"TELEPORT_AUTH_RECORDS_BACKEND_TYPE"`
		// Params is map with backend specific parameters
		Params string `yaml:"params,flow" env:"TELEPORT_AUTH_RECORDS_BACKEND_PARAMS"`
	} `yaml:"records_backend"`

	Limiter limiter.LimiterConfig `yaml:"limiter" env:"TELEPORT_AUTH_LIMITER"`
}

// SSHConfig configures SSH server node role
type SSHConfig struct {
	Enabled   bool                   `yaml:"enabled" env:"TELEPORT_SSH_ENABLED"`
	Token     string                 `yaml:"token" env:"TELEPORT_SSH_TOKEN"`
	Addr      utils.NetAddr          `yaml:"addr" env:"TELEPORT_SSH_ADDR"`
	Shell     string                 `yaml:"shell" env:"TELEPORT_SSH_SHELL"`
	Limiter   limiter.LimiterConfig  `yaml:"limiter" env:"TELEPORT_SSH_LIMITER"`
	Labels    map[string]string      `yaml:"labels" env:"TELEPORT_SSH_LABELS"`
	CmdLabels services.CommandLabels `yaml:"label-commands" env:"TELEPORT_SSH_LABEL_COMMANDS"`
}

// ReverseTunnelConfig configures reverse tunnel role
type ReverseTunnelConfig struct {
	Enabled  bool                  `yaml:"enabled" env:"TELEPORT_REVERSE_TUNNEL_ENABLED"`
	Token    string                `yaml:"token" env:"TELEPORT_REVERSE_TUNNEL_TOKEN"`
	DialAddr utils.NetAddr         `yaml:"dial_addr" env:"TELEPORT_REVERSE_TUNNEL_DIAL_ADDR"`
	Limiter  limiter.LimiterConfig `yaml:"limiter" env:"TELEPORT_REVERSE_TUNNEL_LIMITER"`
}

type NetAddrSlice []utils.NetAddr

func (s *NetAddrSlice) Set(val string) error {
	values := make([]string, 0)
	err := json.Unmarshal([]byte(val), &values)
	if err != nil {
		return trace.Wrap(err)
	}

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

type StringArray []string

func (sa *StringArray) Set(v string) error {
	if len(*sa) == 0 {
		*sa = make([]string, 0)
	}
	err := json.Unmarshal([]byte(v), sa)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type KeyVal map[string]string

// Set accepts string with arguments in the form "key:val,key2:val2"
func (kv *KeyVal) Set(v string) error {
	if len(*kv) == 0 {
		*kv = make(map[string]string)
	}
	err := json.Unmarshal([]byte(v), kv)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type CertificateAuthority struct {
	Type       string `json:"type" yaml:"type"`
	ID         string `json:"id" yaml:"id"`
	DomainName string `json:"domain_name" yaml:"domain_name"`
	PublicKey  string `json:"public_key" yaml:"public_key"`
}

type CertificateAuthorities []CertificateAuthority

func (c *CertificateAuthorities) SetEnv(v string) error {
	var certs []CertificateAuthority
	if err := json.Unmarshal([]byte(v), &certs); err != nil {
		return trace.Wrap(err, "expected JSON encoded remote certificate")
	}
	*c = certs
	return nil
}

func (a CertificateAuthorities) Authorities() ([]services.CertificateAuthority, error) {
	outCerts := make([]services.CertificateAuthority, len(a))
	for i, v := range a {
		outCerts[i] = services.CertificateAuthority{
			Type:       v.Type,
			ID:         v.ID,
			DomainName: v.DomainName,
			PublicKey:  []byte(v.PublicKey),
		}
	}
	return outCerts, nil
}

type LocalCertificateAuthority struct {
	CertificateAuthority `json:"public" yaml:"public"`
	PrivateKey           string `json:"private_key" yaml:"private_key"`
}

func (c *LocalCertificateAuthority) SetEnv(v string) error {
	var ca *LocalCertificateAuthority
	if err := json.Unmarshal([]byte(v), &ca); err != nil {
		return trace.Wrap(err, "expected JSON encoded certificate authority")
	}
	*c = *ca
	return nil
}

func (c *LocalCertificateAuthority) CA() (*services.LocalCertificateAuthority, error) {
	privateKey, err := base64.StdEncoding.DecodeString(c.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := services.LocalCertificateAuthority{
		PrivateKey: privateKey,
		CertificateAuthority: services.CertificateAuthority{
			Type:       c.Type,
			ID:         c.ID,
			DomainName: c.DomainName,
			PublicKey:  []byte(c.PublicKey),
		},
	}
	return &out, nil
}
