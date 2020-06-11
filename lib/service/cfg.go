/*
Copyright 2015-2020 Gravitational, Inc.

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
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Config structure is used to initialize _all_ services Teleport can run.
// Some settings are global (like DataDir) while others are grouped into
// sections, like AuthConfig
type Config struct {
	// DataDir provides directory where teleport stores it's permanent state
	// (in case of auth server backed by BoltDB) or local state, e.g. keys
	DataDir string

	// Hostname is a node host name
	Hostname string

	// Token is used to register this Teleport instance with the auth server
	Token string

	// AuthServers is a list of auth servers, proxies and peer auth servers to
	// connect to. Yes, this is not just auth servers, the field name is
	// misleading.
	AuthServers []utils.NetAddr

	// Identities is an optional list of pre-generated key pairs
	// for teleport roles, this is helpful when server is preconfigured
	Identities []*auth.Identity

	// AdvertiseIP is used to "publish" an alternative IP address or hostname this node
	// can be reached on, if running behind NAT
	AdvertiseIP string

	// CachePolicy sets caching policy for nodes and proxies
	// in case if they loose connection to auth servers
	CachePolicy CachePolicy

	// SSH role an SSH endpoint server
	SSH SSHConfig

	// Auth server authentication and authorization server config
	Auth AuthConfig

	// Keygen points to a key generator implementation
	Keygen sshca.Authority

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

	// Events is events service
	Events services.Events

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Trust is a service that manages users and credentials
	Identity services.Identity

	// Access is a service that controls access
	Access services.Access

	// ClusterConfiguration is a service that provides cluster configuration
	ClusterConfiguration services.ClusterConfiguration

	// CipherSuites is a list of TLS ciphersuites that Teleport supports. If
	// omitted, a Teleport selected list of defaults will be used.
	CipherSuites []uint16

	// Ciphers is a list of SSH ciphers that the server supports. If omitted,
	// the defaults will be used.
	Ciphers []string

	// KEXAlgorithms is a list of SSH key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	KEXAlgorithms []string

	// MACAlgorithms is a list of SSH message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	MACAlgorithms []string

	// CASignatureAlgorithm is an SSH Certificate Authority (CA) signature
	// algorithm that the server uses for signing user and host certificates.
	// If omitted, the default will be used.
	CASignatureAlgorithm *string

	// DiagnosticAddr is an address for diagnostic and healthz endpoint service
	DiagnosticAddr utils.NetAddr

	// Debug sets debugging mode, results in diagnostic address
	// endpoint extended with additional /debug handlers
	Debug bool

	// UploadEventsC is a channel for upload events
	// used in tests
	UploadEventsC chan *events.UploadEvent `json:"-"`

	// FileDescriptors is an optional list of file descriptors for the process
	// to inherit and use for listeners, used for in-process updates.
	FileDescriptors []FileDescriptor

	// PollingPeriod is set to override default internal polling periods
	// of sync agents, used to speed up integration tests.
	PollingPeriod time.Duration

	// ClientTimeout is set to override default client timeouts
	// used by internal clients, used to speed up integration tests.
	ClientTimeout time.Duration

	// ShutdownTimeout is set to override default shutdown timeout.
	ShutdownTimeout time.Duration

	// CAPin is the SKPI hash of the CA used to verify the Auth Server.
	CAPin string

	// Clock is used to control time in tests.
	Clock clockwork.Clock

	// FIPS means FedRAMP/FIPS 140-2 compliant configuration was requested.
	FIPS bool

	// BPFConfig holds configuration for the BPF service.
	BPFConfig *bpf.Config
}

// ApplyToken assigns a given token to all internal services but only if token
// is not an empty string.
//
// returns:
// true, nil if the token has been modified
// false, nil if the token has not been modified
// false, err if there was an error
func (cfg *Config) ApplyToken(token string) (bool, error) {
	if token != "" {
		var err error
		cfg.Token, err = utils.ReadToken(token)
		if err != nil {
			return false, trace.Wrap(err)
		}
		return true, nil
	}
	return false, nil
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
	// Type sets the cache type
	Type string
	// Enabled enables or disables caching
	Enabled bool
	// TTL sets maximum TTL for the cached values
	// without explicit TTL set
	TTL time.Duration
	// NeverExpires means that cache values without TTL
	// set by the auth server won't expire
	NeverExpires bool
	// RecentTTL is the recently accessed items cache TTL
	RecentTTL *time.Duration
}

// GetRecentTTL either returns TTL that was set,
// or default recent TTL value
func (c *CachePolicy) GetRecentTTL() time.Duration {
	if c.RecentTTL == nil {
		return defaults.RecentCacheTTL
	}
	return *c.RecentTTL
}

// CheckAndSetDefaults checks and sets default values
func (c *CachePolicy) CheckAndSetDefaults() error {
	switch c.Type {
	case "", lite.GetName():
		c.Type = lite.GetName()
	case memory.GetName():
	default:
		return trace.BadParameter("unsupported cache type %q, supported values are %q and %q",
			c.Type, lite.GetName(), memory.GetName())
	}
	return nil
}

// String returns human-friendly representation of the policy
func (c CachePolicy) String() string {
	if !c.Enabled {
		return "no cache policy"
	}
	recentCachePolicy := ""
	if c.GetRecentTTL() == 0 {
		recentCachePolicy = "will not cache frequently accessed items"
	} else {
		recentCachePolicy = fmt.Sprintf("will cache frequently accessed items for %v", c.GetRecentTTL())
	}
	if c.NeverExpires {
		return fmt.Sprintf("%v cache that will not expire in case if connection to database is lost, %v", c.Type, recentCachePolicy)
	}
	if c.TTL == 0 {
		return fmt.Sprintf("%v cache that will expire after connection to database is lost after %v, %v", c.Type, defaults.CacheTTL, recentCachePolicy)
	}
	return fmt.Sprintf("%v cache that will expire after connection to database is lost after %v, %v", c.Type, c.TTL, recentCachePolicy)
}

// ProxyConfig specifies configuration for proxy service
type ProxyConfig struct {
	// Enabled turns proxy role on or off for this process
	Enabled bool

	//DisableTLS is enabled if we don't want self signed certs
	DisableTLS bool

	// DisableWebInterface allows to turn off serving the Web UI interface
	DisableWebInterface bool

	// DisableWebService turnes off serving web service completely, including web UI
	DisableWebService bool

	// DisableReverseTunnel disables reverse tunnel on the proxy
	DisableReverseTunnel bool

	// ReverseTunnelListenAddr is address where reverse tunnel dialers connect to
	ReverseTunnelListenAddr utils.NetAddr

	// EnableProxyProtocol enables proxy protocol support
	EnableProxyProtocol bool

	// WebAddr is address for web portal of the proxy
	WebAddr utils.NetAddr

	// SSHAddr is address of ssh proxy
	SSHAddr utils.NetAddr

	// TLSKey is a base64 encoded private key used by web portal
	TLSKey string

	// TLSCert is a base64 encoded certificate used by web portal
	TLSCert string

	Limiter limiter.LimiterConfig

	// PublicAddrs is a list of the public addresses the proxy advertises
	// for the HTTP endpoint. The hosts in in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	PublicAddrs []utils.NetAddr

	// SSHPublicAddrs is a list of the public addresses the proxy advertises
	// for the SSH endpoint. The hosts in in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	SSHPublicAddrs []utils.NetAddr

	// TunnelPublicAddrs is a list of the public addresses the proxy advertises
	// for the tunnel endpoint. The hosts in in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	TunnelPublicAddrs []utils.NetAddr

	// Kube specifies kubernetes proxy configuration
	Kube KubeProxyConfig
}

// KubeProxyConfig specifies configuration for proxy service
type KubeProxyConfig struct {
	// Enabled turns kubernetes proxy role on or off for this process
	Enabled bool

	// ListenAddr is address where reverse tunnel dialers connect to
	ListenAddr utils.NetAddr

	// KubeAPIAddr is address of kubernetes API server
	APIAddr utils.NetAddr

	// ClusterOverride causes all traffic to go to a specific remote
	// cluster, used only in tests
	ClusterOverride string

	// CACert is a PEM encoded kubernetes CA certificate
	CACert []byte

	// PublicAddrs is a list of the public addresses the Teleport Kube proxy can be accessed by,
	// it also affects the host principals and routing logic
	PublicAddrs []utils.NetAddr

	// KubeconfigPath is a path to kubeconfig
	KubeconfigPath string
}

// AuthConfig is a configuration of the auth server
type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool

	// EnableProxyProtocol enables proxy protocol support
	EnableProxyProtocol bool

	// SSHAddr is the listening address of SSH tunnel to HTTP service
	SSHAddr utils.NetAddr

	// Authorities is a set of trusted certificate authorities
	// that will be added by this auth server on the first start
	Authorities []services.CertAuthority

	// Resources is a set of previously backed up resources
	// used to bootstrap backend state on the first start.
	Resources []services.Resource

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

	// ClusterConfig stores cluster level configuration.
	ClusterConfig services.ClusterConfig

	// LicenseFile is a full path to the license file
	LicenseFile string

	// PublicAddrs affects the SSH host principals and DNS names added to the SSH and TLS certs.
	PublicAddrs []utils.NetAddr
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

	// PAM holds PAM configuration for Teleport.
	PAM *pam.Config

	// PublicAddrs affects the SSH host principals and DNS names added to the SSH and TLS certs.
	PublicAddrs []utils.NetAddr

	// BPF holds BPF configuration for Teleport.
	BPF *bpf.Config
}

// MakeDefaultConfig creates a new Config structure and populates it with defaults
func MakeDefaultConfig() (config *Config) {
	config = &Config{}
	ApplyDefaults(config)
	return config
}

// ApplyDefaults applies default values to the existing config structure
func ApplyDefaults(cfg *Config) {
	// Get defaults for Cipher, Kex algorithms, and MAC algorithms from
	// golang.org/x/crypto/ssh default config.
	var sc ssh.Config
	sc.SetDefaults()

	// Remove insecure and (borderline insecure) cryptographic primitives from
	// default configuration. These can still be added back in file configuration by
	// users, but not supported by default by Teleport. See #1856 for more
	// details.
	kex := utils.RemoveFromSlice(sc.KeyExchanges,
		defaults.DiffieHellmanGroup1SHA1,
		defaults.DiffieHellmanGroup14SHA1)
	macs := utils.RemoveFromSlice(sc.MACs,
		defaults.HMACSHA1,
		defaults.HMACSHA196)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
		log.Errorf("Failed to determine hostname: %v.", err)
	}

	// Global defaults.
	cfg.Hostname = hostname
	cfg.DataDir = defaults.DataDir
	cfg.Console = os.Stdout
	cfg.CipherSuites = utils.DefaultCipherSuites()
	cfg.Ciphers = sc.Ciphers
	cfg.KEXAlgorithms = kex
	cfg.MACAlgorithms = macs

	// Auth service defaults.
	cfg.Auth.Enabled = true
	cfg.Auth.SSHAddr = *defaults.AuthListenAddr()
	cfg.Auth.StorageConfig.Type = lite.GetName()
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	cfg.Auth.StaticTokens = services.DefaultStaticTokens()
	cfg.Auth.ClusterConfig = services.DefaultClusterConfig()
	defaults.ConfigureLimiter(&cfg.Auth.Limiter)
	// set new style default auth preferences
	ap := &services.AuthPreferenceV2{}
	ap.CheckAndSetDefaults()
	cfg.Auth.Preference = ap
	cfg.Auth.LicenseFile = filepath.Join(cfg.DataDir, defaults.LicenseFile)

	// Proxy service defaults.
	cfg.Proxy.Enabled = true
	cfg.Proxy.SSHAddr = *defaults.ProxyListenAddr()
	cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnelListenAddr()
	defaults.ConfigureLimiter(&cfg.Proxy.Limiter)

	// Kubernetes proxy service defaults.
	cfg.Proxy.Kube.Enabled = false
	cfg.Proxy.Kube.ListenAddr = *defaults.KubeProxyListenAddr()

	// SSH service defaults.
	cfg.SSH.Enabled = true
	cfg.SSH.Shell = defaults.DefaultShell
	defaults.ConfigureLimiter(&cfg.SSH.Limiter)
	cfg.SSH.PAM = &pam.Config{Enabled: false}
	cfg.SSH.BPF = &bpf.Config{Enabled: false}
}

// ApplyFIPSDefaults updates default configuration to be FedRAMP/FIPS 140-2
// compliant.
func ApplyFIPSDefaults(cfg *Config) {
	cfg.FIPS = true

	// Update TLS and SSH cryptographic primitives.
	cfg.CipherSuites = defaults.FIPSCipherSuites
	cfg.Ciphers = defaults.FIPSCiphers
	cfg.KEXAlgorithms = defaults.FIPSKEXAlgorithms
	cfg.MACAlgorithms = defaults.FIPSMACAlgorithms

	// Only SSO based authentication is supported in FIPS mode. The SSO
	// provider is where any FedRAMP/FIPS 140-2 compliance (like password
	// complexity) should be enforced.
	cfg.Auth.ClusterConfig.SetLocalAuth(false)

	// Update cluster configuration to record sessions at node, this way the
	// entire cluster is FedRAMP/FIPS 140-2 compliant.
	cfg.Auth.ClusterConfig.SetSessionRecording(services.RecordAtNode)
}
