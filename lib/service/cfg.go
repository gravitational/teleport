/*
Copyright 2015-2021 Gravitational, Inc.

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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/local"
	"github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
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
	Identities []*server.Identity

	// AdvertiseIP is used to "publish" an alternative IP address or hostname this node
	// can be reached on, if running behind NAT
	AdvertiseIP string

	// CachePolicy sets caching policy for nodes and proxies
	// in case if they loose connection to auth servers
	CachePolicy CachePolicy

	// Auth service configuration. Manages cluster state and configuration.
	Auth AuthConfig

	// Proxy service configuration. Manages incoming and outbound
	// connections to the cluster.
	Proxy ProxyConfig

	// SSH service configuration. Manages SSH servers running within the cluster.
	SSH SSHConfig

	// App service configuration. Manages applications running within the cluster.
	Apps AppsConfig

	// Databases defines database proxy service configuration.
	Databases DatabasesConfig

	// Keygen points to a key generator implementation
	Keygen sshca.Authority

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
	Trust local.Trust

	// Presence service is a discovery and hearbeat tracker
	Presence local.Presence

	// Events is events service
	Events services.Events

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner local.Provisioner

	// Trust is a service that manages users and credentials
	Identity local.Identity

	// Access is a service that controls access
	Access local.Access

	// ClusterConfiguration is a service that provides cluster configuration
	ClusterConfiguration local.ClusterConfiguration

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
	UploadEventsC chan events.UploadEvent `json:"-"`

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

	// Kube is a Kubernetes API gateway using Teleport client identities.
	Kube KubeConfig

	// Log optionally specifies the logger
	Log utils.Logger

	// PluginRegistry allows adding enterprise logic to Teleport services
	PluginRegistry plugin.Registry
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

	// DisableDatabaseProxy disables database access proxy listener
	DisableDatabaseProxy bool

	// ReverseTunnelListenAddr is address where reverse tunnel dialers connect to
	ReverseTunnelListenAddr utils.NetAddr

	// EnableProxyProtocol enables proxy protocol support
	EnableProxyProtocol bool

	// WebAddr is address for web portal of the proxy
	WebAddr utils.NetAddr

	// SSHAddr is address of ssh proxy
	SSHAddr utils.NetAddr

	// MySQLAddr is address of MySQL proxy.
	MySQLAddr utils.NetAddr

	Limiter limiter.Config

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

	// KeyPairs are the key and certificate pairs that the proxy will load.
	KeyPairs []KeyPairPath

	// ACME is ACME protocol support config
	ACME ACME
}

// ACME configures ACME automatic certificate renewal
type ACME struct {
	// Enabled enables or disables ACME support
	Enabled bool
	// Email receives notifications from ACME server
	Email string
	// URI is ACME server URI
	URI string
}

// KeyPairPath are paths to a key and certificate file.
type KeyPairPath struct {
	// PrivateKey is the path to a PEM encoded private key.
	PrivateKey string
	// Certificate is the path to a PEM encoded certificate.
	Certificate string
}

// KubeAddr returns the address for the Kubernetes endpoint on this proxy that
// can be reached by clients.
func (c ProxyConfig) KubeAddr() (string, error) {
	if !c.Kube.Enabled {
		return "", trace.NotFound("kubernetes support not enabled on this proxy")
	}
	if len(c.Kube.PublicAddrs) > 0 {
		return fmt.Sprintf("https://%s", c.Kube.PublicAddrs[0].Addr), nil
	}
	host := "<proxyhost>"
	// Try to guess the hostname from the HTTP public_addr.
	if len(c.PublicAddrs) > 0 {
		host = c.PublicAddrs[0].Host()
	}
	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, strconv.Itoa(c.Kube.ListenAddr.Port(defaults.KubeListenPort))),
	}
	return u.String(), nil
}

// KubeProxyConfig specifies configuration for proxy service
type KubeProxyConfig struct {
	// Enabled turns kubernetes proxy role on or off for this process
	Enabled bool

	// ListenAddr is the address to listen on for incoming kubernetes requests.
	ListenAddr utils.NetAddr

	// ClusterOverride causes all traffic to go to a specific remote
	// cluster, used only in tests
	ClusterOverride string

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

	Limiter limiter.Config

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
	Limiter               limiter.Config
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

// KubeConfig specifies configuration for kubernetes service
type KubeConfig struct {
	// Enabled turns kubernetes service role on or off for this process
	Enabled bool

	// ListenAddr is the address to listen on for incoming kubernetes requests.
	// Optional.
	ListenAddr *utils.NetAddr

	// PublicAddrs is a list of the public addresses the Teleport kubernetes
	// service can be reached by the proxy service.
	PublicAddrs []utils.NetAddr

	// KubeClusterName is the name of a kubernetes cluster this proxy is running
	// in. If empty, defaults to the Teleport cluster name.
	KubeClusterName string

	// KubeconfigPath is a path to kubeconfig
	KubeconfigPath string

	// Labels are used for RBAC on clusters.
	StaticLabels  map[string]string
	DynamicLabels services.CommandLabels

	// Limiter limits the connection and request rates.
	Limiter limiter.Config
}

// DatabasesConfig configures the database proxy service.
type DatabasesConfig struct {
	// Enabled enables the database proxy service.
	Enabled bool
	// Databases is a list of databases proxied by this service.
	Databases []Database
}

// Database represents a single database that's being proxied.
type Database struct {
	// Name is the database name, used to refer to in CLI.
	Name string
	// Description is a free-form database description.
	Description string
	// Protocol is the database type, e.g. postgres or mysql.
	Protocol string
	// URI is the database endpoint to connect to.
	URI string
	// StaticLabels is a map of database static labels.
	StaticLabels map[string]string
	// DynamicLabels is a list of database dynamic labels.
	DynamicLabels services.CommandLabels
	// CACert is an optional database CA certificate.
	CACert []byte
	// AWS contains AWS specific settings for RDS/Aurora.
	AWS DatabaseAWS
	// GCP contains GCP specific settings for Cloud SQL.
	GCP DatabaseGCP
}

// DatabaseAWS contains AWS specific settings for RDS/Aurora databases.
type DatabaseAWS struct {
	// Region is the cloud region database is running in when using AWS RDS.
	Region string
}

// DatabaseGCP contains GCP specific settings for Cloud SQL databases.
type DatabaseGCP struct {
	// ProjectID is the GCP project ID where the database is deployed.
	ProjectID string
	// InstanceID is the Cloud SQL instance ID.
	InstanceID string
}

// Check validates the database proxy configuration.
func (d *Database) Check() error {
	if d.Name == "" {
		return trace.BadParameter("empty database name")
	}
	// Unlike application access proxy, database proxy name doesn't necessarily
	// need to be a valid subdomain but use the same validation logic for the
	// simplicity and consistency.
	if errs := validation.IsDNS1035Label(d.Name); len(errs) > 0 {
		return trace.BadParameter("invalid database %q name: %v", d.Name, errs)
	}
	if !utils.SliceContainsStr(defaults.DatabaseProtocols, d.Protocol) {
		return trace.BadParameter("unsupported database %q protocol %q, supported are: %v",
			d.Name, d.Protocol, defaults.DatabaseProtocols)
	}
	if _, _, err := net.SplitHostPort(d.URI); err != nil {
		return trace.BadParameter("invalid database %q address %q: %v",
			d.Name, d.URI, err)
	}
	if len(d.CACert) != 0 {
		if _, err := tlsca.ParseCertificatePEM(d.CACert); err != nil {
			return trace.BadParameter("provided database %q CA doesn't appear to be a valid x509 certificate: %v",
				d.Name, err)
		}
	}
	// Validate Cloud SQL specific configuration.
	switch {
	case d.GCP.ProjectID != "" && d.GCP.InstanceID == "":
		return trace.BadParameter("missing Cloud SQL instance ID for database %q", d.Name)
	case d.GCP.ProjectID == "" && d.GCP.InstanceID != "":
		return trace.BadParameter("missing Cloud SQL project ID for database %q", d.Name)
	case d.GCP.ProjectID != "" && d.GCP.InstanceID != "":
		// Only Postgres Cloud SQL instances currently support IAM authentication.
		// It's a relatively new feature so we'll be able to enable it once it
		// expands to MySQL as well:
		//   https://cloud.google.com/sql/docs/postgres/authentication
		if d.Protocol != defaults.ProtocolPostgres {
			return trace.BadParameter("Cloud SQL IAM authentication is currently supported only for PostgreSQL databases, can't use database %q with protocol %q", d.Name, d.Protocol)
		}
		// TODO(r0mant): See if we can download it automatically similar to RDS:
		// https://cloud.google.com/sql/docs/postgres/instance-info#rest-v1beta4
		if len(d.CACert) == 0 {
			return trace.BadParameter("missing Cloud SQL instance root certificate for database %q", d.Name)
		}
	}
	return nil
}

// AppsConfig configures application proxy service.
type AppsConfig struct {
	// Enabled enables application proxying service.
	Enabled bool

	// DebugApp enabled a header dumping debugging application.
	DebugApp bool

	// Apps is the list of applications that are being proxied.
	Apps []App
}

// App is the specific application that will be proxied by the application
// service. This needs to exist because if the "config" package tries to
// directly create a services.App it will get into circular imports.
type App struct {
	// Name of the application.
	Name string

	// Description is the app description.
	Description string

	// URI is the internal address of the application.
	URI string

	// Public address of the application. This is the address users will access
	// the application at.
	PublicAddr string

	// StaticLabels is a map of static labels to apply to this application.
	StaticLabels map[string]string

	// DynamicLabels is a list of dynamic labels to apply to this application.
	DynamicLabels services.CommandLabels

	// InsecureSkipVerify is used to skip validating the server's certificate.
	InsecureSkipVerify bool

	// Rewrite defines a block that is used to rewrite requests and responses.
	Rewrite *Rewrite
}

// Check validates an application.
func (a App) Check() error {
	if a.Name == "" {
		return trace.BadParameter("missing application name")
	}
	if a.URI == "" {
		return trace.BadParameter("missing application URI")
	}
	// Check if the application name is a valid subdomain. Don't allow names that
	// are invalid subdomains because for trusted clusters the name is used to
	// construct the domain that the application will be available at.
	if errs := validation.IsDNS1035Label(a.Name); len(errs) > 0 {
		return trace.BadParameter("application name %q must be a valid DNS subdomain: https://goteleport.com/teleport/docs/application-access/#application-name", a.Name)
	}
	// Parse and validate URL.
	if _, err := url.Parse(a.URI); err != nil {
		return trace.BadParameter("application URI invalid: %v", err)
	}
	// If a port was specified or an IP address was provided for the public
	// address, return an error.
	if a.PublicAddr != "" {
		if _, _, err := net.SplitHostPort(a.PublicAddr); err == nil {
			return trace.BadParameter("public_addr %q can not contain a port, applications will be available on the same port as the web proxy", a.PublicAddr)
		}
		if net.ParseIP(a.PublicAddr) != nil {
			return trace.BadParameter("public_addr %q can not be an IP address, Teleport Application Access uses DNS names for routing", a.PublicAddr)
		}
	}

	return nil
}

// Rewrite is a list of rewriting rules to apply to requests and responses.
type Rewrite struct {
	// Redirect is a list of hosts that should be rewritten to the public address.
	Redirect []string
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

	if cfg.Log == nil {
		cfg.Log = utils.NewLogger()
	}

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
		cfg.Log.Errorf("Failed to determine hostname: %v.", err)
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
	cfg.Auth.StaticTokens = auth.DefaultStaticTokens()
	cfg.Auth.ClusterConfig = auth.DefaultClusterConfig()
	cfg.Auth.Preference = services.DefaultAuthPreference()
	defaults.ConfigureLimiter(&cfg.Auth.Limiter)
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

	// Kubernetes service defaults.
	cfg.Kube.Enabled = false
	defaults.ConfigureLimiter(&cfg.Kube.Limiter)

	// Apps service defaults. It's disabled by default.
	cfg.Apps.Enabled = false

	// Databases proxy service is disabled by default.
	cfg.Databases.Enabled = false
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
