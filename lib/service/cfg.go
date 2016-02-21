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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"io"

	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// Config structure is used to initialize _all_ services Teleporot can run.
// Some settings are globl (like DataDir) while others are grouped into
// sections, like AuthConfig
type Config struct {
	DataDir  string
	Hostname string

	AuthServers NetAddrSlice

	// SSH role an SSH endpoint server
	SSH SSHConfig

	// Auth server authentication and authorizatin server config
	Auth AuthConfig

	// ReverseTunnnel role creates and mantains outbound SSH reverse tunnel to the proxy
	ReverseTunnel ReverseTunnelConfig

	// Proxy is SSH proxy that manages incoming and outbound connections
	// via multiple reverse tunnels
	Proxy ProxyConfig

	// Console writer to speak to a user
	Console io.Writer
}

func (cfg *Config) RoleConfig() RoleConfig {
	return RoleConfig{
		DataDir:     cfg.DataDir,
		Hostname:    cfg.Hostname,
		AuthServers: cfg.AuthServers,
		Auth:        cfg.Auth,
		Console:     cfg.Console,
	}
}

// DebugDumpToYAML() is useful for debugging: it dumps the Config structure into
// a string
func (cfg *Config) DebugDumpToYAML() string {
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err.Error()
	}
	return string(out)
}

type ProxyConfig struct {
	// Enabled turns proxy role on or off for this process
	Enabled bool

	// Token is a provisioning token for new proxy server registering with auth
	Token string

	// ReverseTunnelListenAddr is address where reverse tunnel dialers connect to
	ReverseTunnelListenAddr utils.NetAddr

	// WebAddr is address for web portal of the proxy
	WebAddr utils.NetAddr

	// SSHAddr is address of ssh proxy
	SSHAddr utils.NetAddr

	// AssetsDir is a directory with proxy website assets
	AssetsDir string

	// TLSKey is a base64 encoded private key used by web portal
	TLSKey string

	// TLSCert is a base64 encoded certificate used by web portal
	TLSCert string

	Limiter limiter.LimiterConfig
}

type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool

	// SSHAddr is the listening address of SSH tunnel to HTTP service
	SSHAddr utils.NetAddr

	// HostAuthorityDomain is Host Certificate Authority domain name
	HostAuthorityDomain string

	// Token is a provisioning token for an additonal auth server joining the cluster
	Token string

	// SecretKey is an encryption key for secret service, will be used
	// to initialize secret service if set
	SecretKey string

	// AllowedTokens is a set of tokens that will be added as trusted
	AllowedTokens KeyVal

	// TrustedAuthorities is a set of trusted user certificate authorities
	TrustedAuthorities CertificateAuthorities

	// UserCA allows to pass preconfigured user certificate authority keypair
	// to auth server so it will use it on the first start instead of generating
	// a new keypair
	UserCA LocalCertificateAuthority

	// HostCA allows to pass preconfigured host certificate authority keypair
	// to auth server so it will use it on the first start instead of generating
	// a new keypair
	HostCA LocalCertificateAuthority

	// KeysBackend configures backend that stores auth keys, certificates, tokens ...
	KeysBackend struct {
		// Type is a backend type - etcd or boltdb
		Type string
		// Params is map with backend specific parameters
		Params string
		// AdditionalKey is a additional signing GPG key
		EncryptionKeys StringArray
	}

	// EventsBackend configures backend that stores cluster events (login attempts, etc)
	EventsBackend struct {
		// Type is a backend type, etcd or bolt
		Type string
		// Params is map with backend specific parameters
		Params string
	}

	// RecordsBackend configures backend that stores live SSH sessions recordings
	RecordsBackend struct {
		// Type is a backend type, currently only bolt
		Type string
		// Params is map with backend specific parameters
		Params string
	}

	Limiter limiter.LimiterConfig
}

// SSHConfig configures SSH server node role
type SSHConfig struct {
	Enabled   bool
	Token     string
	Addr      utils.NetAddr
	Shell     string
	Limiter   limiter.LimiterConfig
	Labels    map[string]string
	CmdLabels services.CommandLabels
}

// ReverseTunnelConfig configures reverse tunnel role
type ReverseTunnelConfig struct {
	Enabled  bool
	Token    string
	DialAddr utils.NetAddr
	Limiter  limiter.LimiterConfig
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
	Type       string `json:"type"`
	ID         string `json:"id"`
	DomainName string `json:"domain_name"`
	PublicKey  string `json:"public_key"`
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

func (a CertificateAuthorities) Authorities() ([]services.CertAuthority, error) {
	return nil, nil
}

type LocalCertificateAuthority struct {
	CertificateAuthority `json:"public"`
	PrivateKey           string `json:"private_key"`
}

func (c *LocalCertificateAuthority) SetEnv(v string) error {
	var ca *LocalCertificateAuthority
	if err := json.Unmarshal([]byte(v), &ca); err != nil {
		return trace.Wrap(err, "expected JSON encoded certificate authority")
	}
	*c = *ca
	return nil
}

func (c *LocalCertificateAuthority) CA() (*services.CertAuthority, error) {
	return nil, nil
}

// MakeDefaultConfig() creates a new Config structure and populates it with defaults
func MakeDefaultConfig() (config *Config, err error) {
	config = &Config{}
	if err = ApplyDefaults(config); err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// ApplyDefaults applies default values to the existing config structure
func ApplyDefaults(cfg *Config) error {
	hostname, err := os.Hostname()
	if err != nil {
		return trace.Wrap(err)
	}

	// defaults for the auth service:
	cfg.Auth.Enabled = true
	cfg.Auth.HostAuthorityDomain = hostname
	cfg.Auth.SSHAddr = *defaults.AuthListenAddr()
	cfg.Auth.EventsBackend.Type = defaults.BackendType
	cfg.Auth.EventsBackend.Params = boltParams(defaults.DataDir, "events.db")
	cfg.Auth.KeysBackend.Type = defaults.BackendType
	cfg.Auth.KeysBackend.Params = boltParams(defaults.DataDir, "keys.db")
	cfg.Auth.RecordsBackend.Type = defaults.BackendType
	cfg.Auth.RecordsBackend.Params = boltParams(defaults.DataDir, "records.db")
	defaults.ConfigureLimiter(&cfg.Auth.Limiter)

	// defaults for the SSH proxy service:
	cfg.Proxy.Enabled = true
	cfg.Proxy.AssetsDir = defaults.DataDir
	cfg.Proxy.SSHAddr = *defaults.ProxyListenAddr()
	cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	cfg.ReverseTunnel.Enabled = false
	cfg.ReverseTunnel.DialAddr = *defaults.ReverseTunnellConnectAddr()
	cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnellListenAddr()
	defaults.ConfigureLimiter(&cfg.Proxy.Limiter)
	defaults.ConfigureLimiter(&cfg.ReverseTunnel.Limiter)

	// defaults for the SSH service:
	cfg.SSH.Enabled = true
	cfg.SSH.Addr = *defaults.SSHServerListenAddr()
	cfg.SSH.Shell = "/bin/bash"
	defaults.ConfigureLimiter(&cfg.SSH.Limiter)

	// global defaults
	cfg.Hostname = hostname
	cfg.DataDir = defaults.DataDir
	if cfg.Auth.Enabled {
		cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}
	}
	cfg.Console = os.Stdout
	return nil
}

// Generates a string accepted by the BoltDB driver, like this:
// `{"path": "/var/lib/teleport/records.db"}`
func boltParams(storagePath, dbFile string) string {
	return fmt.Sprintf(`{"path": "%s"}`, filepath.Join(storagePath, dbFile))
}
