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
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
)

// Config structure is used to initialize _all_ services Teleporot can run.
// Some settings are globl (like DataDir) while others are grouped into
// sections, like AuthConfig
type Config struct {
	// DataDir provides directory where teleport stores it's permanent state
	// (in case of auth server backed by BoltDB) or local state, e.g. keys
	DataDir string

	// Hostname is a node host name
	Hostname string

	// Token is used to register this Teleport instance with the auth server
	Token string

	// AuthServers is a list of auth servers nodes, proxies and peer auth servers
	// connect to
	AuthServers []utils.NetAddr

	// Identities is an optional list of pre-generated key pairs
	// for teleport roles, this is helpful when server is preconfigured
	Identities []*auth.Identity

	// AdvertiseIP is used to "publish" an alternative IP address this node
	// can be reached on, if running behind NAT
	AdvertiseIP net.IP

	// CachePolicy sets caching policy for nodes and proxies
	// in case if they loose connection to auth servers
	CachePolicy CachePolicy

	// SSH role an SSH endpoint server
	SSH SSHConfig

	// Auth server authentication and authorizatin server config
	Auth AuthConfig

	// Keygen points to a key generator implementation
	Keygen auth.Authority

	// Proxy is SSH proxy that manages incoming and outbound connections
	// via multiple reverse tunnels
	Proxy ProxyConfig

	// HostUUID is a unique UUID of this host (it will be known via this UUID within
	// a teleport cluster). It's automatically generated on 1st start
	HostUUID string

	// Console writer to speak to a user
	Console io.Writer

	// ReverseTunnels is a list of reverse tunnels to create on the
	// first cluster start
	ReverseTunnels []services.ReverseTunnel

	// OIDCConnectors is a list of trusted OpenID Connect identity providers
	OIDCConnectors []services.OIDCConnector

	// PidFile is a full path of the PID file for teleport daemon
	PIDFile string

	// Trust is a service that manages users and credentials
	Trust services.Trust

	// Presence service is a discovery and hearbeat tracker
	Presence services.Presence

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Trust is a service that manages users and credentials
	Identity services.Identity

	// Access is a service that controls access
	Access services.Access

	// Ciphers is a list of ciphers that the server supports. If omitted,
	// the defaults will be used.
	Ciphers []string

	// KEXAlgorithms is a list of key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	KEXAlgorithms []string

	// MACAlgorithms is a list of message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	MACAlgorithms []string
}

// ApplyToken assigns a given token to all internal services but only if token
// is not an empty string.
//
// Returns 'true' if token was modified
func (cfg *Config) ApplyToken(token string) bool {
	if token != "" {
		cfg.Token = token
		return true
	}
	return false
}

// RoleConfig is a config for particular Teleport role
func (cfg *Config) RoleConfig() RoleConfig {
	return RoleConfig{
		DataDir:     cfg.DataDir,
		HostUUID:    cfg.HostUUID,
		HostName:    cfg.Hostname,
		AuthServers: cfg.AuthServers,
		Auth:        cfg.Auth,
		Console:     cfg.Console,
	}
}

// DebugDumpToYAML is useful for debugging: it dumps the Config structure into
// a string
func (cfg *Config) DebugDumpToYAML() string {
	shallow := *cfg
	// do not copy sensitive data to stdout
	shallow.Identities = nil
	shallow.Auth.Authorities = nil
	out, err := yaml.Marshal(shallow)
	if err != nil {
		return err.Error()
	}
	return string(out)
}

// CachePolicy sets caching policy for proxies and nodes
type CachePolicy struct {
	// Enabled enables or disables caching
	Enabled bool
	// TTL sets maximum TTL for the cached values
	// without explicit TTL set
	TTL time.Duration
	// NeverExpires means that cache values without TTL
	// set by the auth server won't expire
	NeverExpires bool
}

// String returns human-friendly representation of the policy
func (c CachePolicy) String() string {
	if !c.Enabled {
		return "no cache policy"
	}
	if c.NeverExpires {
		return "never expiring cache policy"
	}
	if c.TTL == 0 {
		return fmt.Sprintf("cache policy with %v TTL", defaults.CacheTTL)
	}
	return fmt.Sprintf("cache policy with %v TTL", c.TTL)
}

// ProxyConfig configures proy service
type ProxyConfig struct {
	// Enabled turns proxy role on or off for this process
	Enabled bool

	// DisableWebInterface allows to turn off serving the Web UI interface
	DisableWebInterface bool

	// DisableWebService turnes off serving web service completely, including web UI
	DisableWebService bool

	// DisableReverseTunnel disables reverse tunnel on the proxy
	DisableReverseTunnel bool

	// ReverseTunnelListenAddr is address where reverse tunnel dialers connect to
	ReverseTunnelListenAddr utils.NetAddr

	// WebAddr is address for web portal of the proxy
	WebAddr utils.NetAddr

	// SSHAddr is address of ssh proxy
	SSHAddr utils.NetAddr

	// TLSKey is a base64 encoded private key used by web portal
	TLSKey string

	// TLSCert is a base64 encoded certificate used by web portal
	TLSCert string

	Limiter limiter.LimiterConfig

	// PublicAddr is the public address the Teleport UI can be accessed at.
	PublicAddr utils.NetAddr
}

// AuthConfig is a configuration of the auth server
type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool

	// SSHAddr is the listening address of SSH tunnel to HTTP service
	SSHAddr utils.NetAddr

	// Authorities is a set of trusted certificate authorities
	// that will be added by this auth server on the first start
	Authorities []services.CertAuthority

	// Roles is a set of roles to pre-provision for this cluster
	Roles []services.Role

	// ClusterName is a name that identifies this authority and all
	// host nodes in the cluster that will share this authority domain name
	// as a base name, e.g. if authority domain name is example.com,
	// all nodes in the cluster will have UUIDs in the form: <uuid>.example.com
	ClusterName services.ClusterName

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens services.StaticTokens

	// StorageConfig contains configuration settings for the storage backend.
	StorageConfig backend.Config

	Limiter limiter.LimiterConfig

	// NoAudit, when set to true, disables session recording and event audit
	NoAudit bool

	// Preference defines the authentication preference (type and second factor) for
	// the auth server.
	Preference services.AuthPreference
}

// SSHConfig configures SSH server node role
type SSHConfig struct {
	Enabled               bool
	Addr                  utils.NetAddr
	Namespace             string
	Shell                 string
	Limiter               limiter.LimiterConfig
	Labels                map[string]string
	CmdLabels             services.CommandLabels
	PermitUserEnvironment bool
}

// MakeDefaultConfig creates a new Config structure and populates it with defaults
func MakeDefaultConfig() (config *Config) {
	config = &Config{}
	ApplyDefaults(config)
	return config
}

// ApplyDefaults applies default values to the existing config structure
func ApplyDefaults(cfg *Config) {
	// get defaults for cipher, kex algorithms, and mac algorithms from
	// golang.org/x/crypto/ssh default config.
	var sc ssh.Config
	sc.SetDefaults()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
		log.Errorf("Failed to determine hostname: %v", err)
	}

	// global defaults
	cfg.Hostname = hostname
	cfg.DataDir = defaults.DataDir
	cfg.Console = os.Stdout
	cfg.Ciphers = sc.Ciphers
	cfg.KEXAlgorithms = sc.KeyExchanges
	cfg.MACAlgorithms = sc.MACs

	// defaults for the auth service:
	cfg.Auth.Enabled = true
	cfg.Auth.SSHAddr = *defaults.AuthListenAddr()
	cfg.Auth.StorageConfig.Type = boltbk.GetName()
	cfg.Auth.StorageConfig.Params = backend.Params{"path": cfg.DataDir}
	cfg.Auth.StaticTokens = services.DefaultStaticTokens()
	defaults.ConfigureLimiter(&cfg.Auth.Limiter)
	// set new style default auth preferences
	ap := &services.AuthPreferenceV2{}
	ap.CheckAndSetDefaults()
	cfg.Auth.Preference = ap

	// defaults for the SSH proxy service:
	cfg.Proxy.Enabled = true
	cfg.Proxy.SSHAddr = *defaults.ProxyListenAddr()
	cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnellListenAddr()
	defaults.ConfigureLimiter(&cfg.Proxy.Limiter)

	// defaults for the SSH service:
	cfg.SSH.Enabled = true
	cfg.SSH.Addr = *defaults.SSHServerListenAddr()
	cfg.SSH.Shell = defaults.DefaultShell
	defaults.ConfigureLimiter(&cfg.SSH.Limiter)
}
