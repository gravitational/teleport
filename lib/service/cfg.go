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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/oauth2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http/httpguts"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/kube/proxy"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// Rate describes a rate ratio, i.e. the number of "events" that happen over
// some unit time period
type Rate struct {
	Amount int
	Time   time.Duration
}

// RoleAndIdentityEvent is a role and its corresponding identity event.
type RoleAndIdentityEvent struct {
	// Role is a system role.
	Role types.SystemRole

	// IdentityEvent is the identity event associated with the above role.
	IdentityEvent string
}

// JoinParams is a set of extra parameters for joining the auth server.
type JoinParams struct {
	Azure AzureJoinParams
}

// AzureJoinParams is the parameters specific to the azure join method.
type AzureJoinParams struct {
	ClientID string
}

// Config structure is used to initialize _all_ services Teleport can run.
// Some settings are global (like DataDir) while others are grouped into
// sections, like AuthConfig
type Config struct {
	// Teleport configuration version.
	Version string
	// DataDir is the directory where teleport stores its permanent state
	// (in case of auth server backed by BoltDB) or local state, e.g. keys
	DataDir string

	// Hostname is a node host name
	Hostname string

	// JoinMethod is the method the instance will use to join the auth server
	JoinMethod types.JoinMethod

	// JoinParams is a set of extra parameters for joining the auth server.
	JoinParams JoinParams

	// ProxyServer is the address of the proxy
	ProxyServer utils.NetAddr

	// Identities is an optional list of pre-generated key pairs
	// for teleport roles, this is helpful when server is preconfigured
	Identities []*auth.Identity

	// AdvertiseIP is used to "publish" an alternative IP address or hostname this node
	// can be reached on, if running behind NAT
	AdvertiseIP string

	// CachePolicy sets caching policy for nodes and proxies
	// in case if they lose connection to auth servers
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

	// Metrics defines the metrics service configuration.
	Metrics MetricsConfig

	// WindowsDesktop defines the Windows desktop service configuration.
	WindowsDesktop WindowsDesktopConfig

	// Discovery defines the discovery service configuration.
	Discovery DiscoveryConfig

	// Okta defines the okta service configuration.
	Okta OktaConfig

	// Tracing defines the tracing service configuration.
	Tracing TracingConfig

	// Keygen points to a key generator implementation
	Keygen sshca.Authority

	// HostUUID is a unique UUID of this host (it will be known via this UUID within
	// a teleport cluster). It's automatically generated on 1st start
	HostUUID string

	// Console writer to speak to a user
	Console io.Writer

	// ReverseTunnels is a list of reverse tunnels to create on the
	// first cluster start
	ReverseTunnels []types.ReverseTunnel

	// OIDCConnectors is a list of trusted OpenID Connect identity providers
	OIDCConnectors []types.OIDCConnector

	// PidFile is a full path of the PID file for teleport daemon
	PIDFile string

	// Trust is a service that manages users and credentials
	Trust services.Trust

	// Presence service is a discovery and heartbeat tracker
	Presence services.PresenceInternal

	// Events is events service
	Events types.Events

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Trust is a service that manages users and credentials
	Identity services.Identity

	// Access is a service that controls access
	Access services.Access

	// UsageReporter is a service that reports usage events.
	UsageReporter usagereporter.UsageReporter

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

	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string

	// Clock is used to control time in tests.
	Clock clockwork.Clock

	// TeleportVersion is used to control the Teleport version in tests.
	TeleportVersion string

	// FIPS means FedRAMP/FIPS 140-2 compliant configuration was requested.
	FIPS bool

	// SkipVersionCheck means the version checking between server and client
	// will be skipped.
	SkipVersionCheck bool

	// BPFConfig holds configuration for the BPF service.
	BPFConfig *bpf.Config

	// Kube is a Kubernetes API gateway using Teleport client identities.
	Kube KubeConfig

	// Log optionally specifies the logger
	Log utils.Logger

	// PluginRegistry allows adding enterprise logic to Teleport services
	PluginRegistry plugin.Registry

	// RotationConnectionInterval is the interval between connection
	// attempts as used by the rotation state service
	RotationConnectionInterval time.Duration

	// MaxRetryPeriod is the maximum period between reconnection attempts to auth
	MaxRetryPeriod time.Duration

	// ConnectFailureC is a channel to notify of failures to connect to auth (used in tests).
	ConnectFailureC chan time.Duration

	// TeleportHome is the path to tsh configuration and data, used
	// for loading profiles when TELEPORT_HOME is set
	TeleportHome string

	// CircuitBreakerConfig configures the auth client circuit breaker.
	CircuitBreakerConfig breaker.Config

	// AdditionalExpectedRoles are additional roles to attach to the Teleport instances.
	AdditionalExpectedRoles []RoleAndIdentityEvent

	// AdditionalReadyEvents are additional events to watch for to consider the Teleport instance ready.
	AdditionalReadyEvents []string

	// InstanceMetadataClient specifies the instance metadata client.
	InstanceMetadataClient cloud.InstanceMetadata

	// token is either the token needed to join the auth server, or a path pointing to a file
	// that contains the token
	//
	// This is private to avoid external packages reading the value - the value should be obtained
	// using Token()
	token string

	// v1, v2 -
	// AuthServers is a list of auth servers, proxies and peer auth servers to
	// connect to. Yes, this is not just auth servers, the field name is
	// misleading.
	// v3 -
	// AuthServers contains a single address that is set by `auth_server` in the config
	// A proxy address would be specified separately, so this no longer contains both
	// auth servers and proxies.
	//
	// In order to keep backwards compatibility between v3 and v2/v1, this is now private
	// and the value is retrieved via AuthServerAddresses() and set via SetAuthServerAddresses()
	// as we still need to keep multiple addresses and return them for older config versions.
	authServers []utils.NetAddr
}

// AuthServerAddresses returns the value of authServers for config versions v1 and v2 and
// will return just the first (as only one should be set) address for config versions v3
// onwards.
func (cfg *Config) AuthServerAddresses() []utils.NetAddr {
	return cfg.authServers
}

// SetAuthServerAddresses sets the value of authServers
// If the config version is v1 or v2, it will set the value to all the given addresses (as
// multiple can be specified).
// If the config version is v3 or onwards, it'll error if more than one address is given.
func (cfg *Config) SetAuthServerAddresses(addrs []utils.NetAddr) error {
	// from config v3 onwards, we will error if more than one address is given
	if cfg.Version != defaults.TeleportConfigVersionV1 && cfg.Version != defaults.TeleportConfigVersionV2 {
		if len(addrs) > 1 {
			return trace.BadParameter("only one auth server address should be set from config v3 onwards")
		}
	}

	cfg.authServers = addrs

	return nil
}

// SetAuthServerAddress sets the value of authServers to a single value
func (cfg *Config) SetAuthServerAddress(addr utils.NetAddr) {
	cfg.authServers = []utils.NetAddr{addr}
}

// Token returns token needed to join the auth server
//
// If the value stored points to a file, it will attempt to read the token value from the file
// and return an error if it wasn't successful
// If the value stored doesn't point to a file, it'll return the value stored
// If the token hasn't been set, an empty string will be returned
func (cfg *Config) Token() (string, error) {
	token, err := utils.TryReadValueAsFile(cfg.token)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}

// SetToken stores the value for --token or auth_token in the config
//
// This can be either the token or an absolute path to a file containing the token.
func (cfg *Config) SetToken(token string) {
	cfg.token = token
}

// HasToken gives the ability to check if there has been a token value stored
// in the config
func (cfg *Config) HasToken() bool {
	return cfg.token != ""
}

// ApplyCAPins assigns the given CA pin(s), filtering out empty pins.
// If a pin is specified as a path to a file, that file must not be empty.
func (cfg *Config) ApplyCAPins(caPins []string) error {
	var filteredPins []string
	for _, pinOrPath := range caPins {
		if pinOrPath == "" {
			continue
		}
		pins, err := utils.TryReadValueAsFile(pinOrPath)
		if err != nil {
			return trace.Wrap(err)
		}
		// an empty pin file is less obvious than a blank ca_pin in the config yaml.
		if pins == "" {
			return trace.BadParameter("empty ca_pin file: %v", pinOrPath)
		}
		filteredPins = append(filteredPins, strings.Split(pins, "\n")...)
	}
	if len(filteredPins) > 0 {
		cfg.CAPins = filteredPins
	}
	return nil
}

// RoleConfig is a config for particular Teleport role
func (cfg *Config) RoleConfig() RoleConfig {
	return RoleConfig{
		DataDir:     cfg.DataDir,
		HostUUID:    cfg.HostUUID,
		HostName:    cfg.Hostname,
		AuthServers: cfg.AuthServerAddresses(),
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
	// MaxRetryPeriod is maximum period cache waits before retrying on failure.
	// Not exposed through the config file, used in tests.
	MaxRetryPeriod time.Duration
}

// CheckAndSetDefaults checks and sets default values
func (c *CachePolicy) CheckAndSetDefaults() error {
	return nil
}

// String returns human-friendly representation of the policy
func (c CachePolicy) String() string {
	if !c.Enabled {
		return "no cache"
	}
	return "in-memory cache"
}

// ProxyConfig specifies configuration for proxy service
type ProxyConfig struct {
	// Enabled turns proxy role on or off for this process
	Enabled bool

	// DisableTLS is enabled if we don't want self-signed certs
	DisableTLS bool

	// DisableWebInterface allows turning off serving the Web UI interface
	DisableWebInterface bool

	// DisableWebService turns off serving web service completely, including web UI
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

	// PostgresAddr is address of Postgres proxy.
	PostgresAddr utils.NetAddr

	// MongoAddr is address of Mongo proxy.
	MongoAddr utils.NetAddr

	// PeerAddr is the proxy peering address.
	PeerAddr utils.NetAddr

	// PeerPublicAddr is the public address the proxy advertises for proxy
	// peering clients.
	PeerPublicAddr utils.NetAddr

	Limiter limiter.Config

	// PublicAddrs is a list of the public addresses the proxy advertises
	// for the HTTP endpoint. The hosts in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	PublicAddrs []utils.NetAddr

	// SSHPublicAddrs is a list of the public addresses the proxy advertises
	// for the SSH endpoint. The hosts in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	SSHPublicAddrs []utils.NetAddr

	// TunnelPublicAddrs is a list of the public addresses the proxy advertises
	// for the tunnel endpoint. The hosts in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	TunnelPublicAddrs []utils.NetAddr

	// PostgresPublicAddrs is a list of the public addresses the proxy
	// advertises for Postgres clients.
	PostgresPublicAddrs []utils.NetAddr

	// MySQLPublicAddrs is a list of the public addresses the proxy
	// advertises for MySQL clients.
	MySQLPublicAddrs []utils.NetAddr

	// MongoPublicAddrs is a list of the public addresses the proxy
	// advertises for Mongo clients.
	MongoPublicAddrs []utils.NetAddr

	// Kube specifies kubernetes proxy configuration
	Kube KubeProxyConfig

	// KeyPairs are the key and certificate pairs that the proxy will load.
	KeyPairs []KeyPairPath

	// KeyPairsReloadInterval is the interval between attempts to reload
	// x509 key pairs. If set to 0, then periodic reloading is disabled.
	KeyPairsReloadInterval time.Duration

	// ACME is ACME protocol support config
	ACME ACME

	// IdP is the identity provider config
	//
	//nolint:revive // Because we want this to be IdP.
	IdP IdP

	// DisableALPNSNIListener allows turning off the ALPN Proxy listener. Used in tests.
	DisableALPNSNIListener bool

	// UI provides config options for the web UI
	UI webclient.UIConfig

	// AssistAPIKey is the OpenAI API key.
	// TODO: This key will be moved to a plugin once support for plugins is implemented.
	AssistAPIKey string
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

// IdP configures identity providers.
//
//nolint:revive // Because we want this to be IdP.
type IdP struct {
	// SAMLIdP is configuration options for the SAML identity provider.
	SAMLIdP SAMLIdP
}

// SAMLIdP configures SAML identity providers
type SAMLIdP struct {
	// Enabled enables or disables the identity provider.
	Enabled bool
	// BaseURL is the base URL for the identity provider.
	BaseURL string
}

// KeyPairPath are paths to a key and certificate file.
type KeyPairPath struct {
	// PrivateKey is the path to a PEM encoded private key.
	PrivateKey string
	// Certificate is the path to a PEM encoded certificate.
	Certificate string
}

// WebPublicAddr returns the address for the web endpoint on this proxy that
// can be reached by clients.
func (c ProxyConfig) WebPublicAddr() (string, error) {
	// Use the port from the first public address if possible.
	if len(c.PublicAddrs) > 0 {
		publicAddr := c.PublicAddrs[0]
		u := url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(publicAddr.Host(), strconv.Itoa(publicAddr.Port(defaults.HTTPListenPort))),
		}
		return u.String(), nil
	}

	port := c.WebAddr.Port(defaults.HTTPListenPort)
	return c.getDefaultAddr(port), nil
}

func (c ProxyConfig) getDefaultAddr(port int) string {
	host := "<proxyhost>"
	// Try to guess the hostname from the HTTP public_addr.
	if len(c.PublicAddrs) > 0 {
		host = c.PublicAddrs[0].Host()
	}

	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}
	return u.String()
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

	return c.getDefaultAddr(c.Kube.ListenAddr.Port(defaults.KubeListenPort)), nil
}

// publicPeerAddr attempts to returns the public address the proxy advertises
// for proxy peering clients if available. It falls back to PeerAddr othewise.
func (c ProxyConfig) publicPeerAddr() (*utils.NetAddr, error) {
	addr := &c.PeerPublicAddr
	if addr.IsEmpty() || addr.IsHostUnspecified() {
		return c.peerAddr()
	}
	return addr, nil
}

// peerAddr returns the address the proxy advertises for proxy peering clients.
func (c ProxyConfig) peerAddr() (*utils.NetAddr, error) {
	addr := &c.PeerAddr
	if addr.IsEmpty() {
		addr = defaults.ProxyPeeringListenAddr()
	}
	if !addr.IsHostUnspecified() {
		return addr, nil
	}

	ip, err := utils.GuessHostIP()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	port := addr.Port(defaults.ProxyPeeringListenPort)
	addr, err = utils.ParseAddr(fmt.Sprintf("%s:%d", ip.String(), port))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return addr, nil
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

	// LegacyKubeProxy specifies that this proxy was configured using the
	// legacy kubernetes section.
	LegacyKubeProxy bool
}

// AuthConfig is a configuration of the auth server
type AuthConfig struct {
	// Enabled turns auth role on or off for this process
	Enabled bool

	// EnableProxyProtocol enables proxy protocol support
	EnableProxyProtocol bool

	// ListenAddr is the listening address of the auth service
	ListenAddr utils.NetAddr

	// Authorities is a set of trusted certificate authorities
	// that will be added by this auth server on the first start
	Authorities []types.CertAuthority

	// BootstrapResources is a set of previously backed up resources
	// used to bootstrap backend state on the first start.
	BootstrapResources []types.Resource

	// ApplyOnStartupResources is a set of resources that should be applied
	// on each Teleport start.
	ApplyOnStartupResources []types.Resource

	// Roles is a set of roles to pre-provision for this cluster
	Roles []types.Role

	// ClusterName is a name that identifies this authority and all
	// host nodes in the cluster that will share this authority domain name
	// as a base name, e.g. if authority domain name is example.com,
	// all nodes in the cluster will have UUIDs in the form: <uuid>.example.com
	ClusterName types.ClusterName

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	StaticTokens types.StaticTokens

	// StorageConfig contains configuration settings for the storage backend.
	StorageConfig backend.Config

	Limiter limiter.Config

	// NoAudit, when set to true, disables session recording and event audit
	NoAudit bool

	// Preference defines the authentication preference (type and second factor) for
	// the auth server.
	Preference types.AuthPreference

	// AuditConfig stores cluster audit configuration.
	AuditConfig types.ClusterAuditConfig

	// NetworkingConfig stores cluster networking configuration.
	NetworkingConfig types.ClusterNetworkingConfig

	// SessionRecordingConfig stores session recording configuration.
	SessionRecordingConfig types.SessionRecordingConfig

	// LicenseFile is a full path to the license file
	LicenseFile string

	// PublicAddrs affects the SSH host principals and DNS names added to the SSH and TLS certs.
	PublicAddrs []utils.NetAddr

	// KeyStore configuration. Handles CA private keys which may be held in a HSM.
	KeyStore keystore.Config

	// LoadAllCAs sends the host CAs of all clusters to SSH clients logging in when enabled,
	// instead of just the host CA for the current cluster.
	LoadAllCAs bool

	// HostedPlugins configures the Enterprise hosted plugin runtime
	HostedPlugins HostedPluginsConfig
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

	// RestrictedSession holds kernel objects restrictions for Teleport.
	RestrictedSession *bpf.RestrictedSessionConfig

	// AllowTCPForwarding indicates that TCP port forwarding is allowed on this node
	AllowTCPForwarding bool

	// IdleTimeoutMessage is sent to the client when a session expires due to
	// the inactivity timeout expiring. The empty string indicates that no
	// timeout message will be sent.
	IdleTimeoutMessage string

	// X11 holds x11 forwarding configuration for Teleport.
	X11 *x11.ServerConfig

	// AllowFileCopying indicates whether this node is allowed to handle
	// remote file operations via SCP or SFTP.
	AllowFileCopying bool

	// DisableCreateHostUser disables automatic user provisioning on this
	// SSH node.
	DisableCreateHostUser bool
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

	// CheckImpersonationPermissions is an optional override to the default
	// impersonation permissions check, for use in testing.
	CheckImpersonationPermissions proxy.ImpersonationPermissionsChecker

	// ResourceMatchers match dynamic kube_cluster resources.
	ResourceMatchers []services.ResourceMatcher
}

// DatabasesConfig configures the database proxy service.
type DatabasesConfig struct {
	// Enabled enables the database proxy service.
	Enabled bool
	// Databases is a list of databases proxied by this service.
	Databases []Database
	// ResourceMatchers match cluster database resources.
	ResourceMatchers []services.ResourceMatcher
	// AWSMatchers match AWS hosted databases.
	AWSMatchers []services.AWSMatcher
	// AzureMatchers match Azure hosted databases.
	AzureMatchers []services.AzureMatcher
	// Limiter limits the connection and request rates.
	Limiter limiter.Config
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
	// MySQL are additional MySQL database options.
	MySQL MySQLOptions
	// DynamicLabels is a list of database dynamic labels.
	DynamicLabels services.CommandLabels
	// TLS keeps database connection TLS configuration.
	TLS DatabaseTLS
	// AWS contains AWS specific settings for RDS/Aurora/Redshift databases.
	AWS DatabaseAWS
	// GCP contains GCP specific settings for Cloud SQL databases.
	GCP DatabaseGCP
	// AD contains Active Directory configuration for database.
	AD DatabaseAD
	// Azure contains Azure database configuration.
	Azure DatabaseAzure
}

// TLSMode defines all possible database verification modes.
type TLSMode string

const (
	// VerifyFull is the strictest. Verifies certificate and server name.
	VerifyFull TLSMode = "verify-full"
	// VerifyCA checks the certificate, but skips the server name verification.
	VerifyCA TLSMode = "verify-ca"
	// Insecure accepts any certificate.
	Insecure TLSMode = "insecure"
)

// AllTLSModes keeps all possible database TLS modes for easy access.
var AllTLSModes = []TLSMode{VerifyFull, VerifyCA, Insecure}

// CheckAndSetDefaults check if TLSMode holds a correct value. If the value is not set
// VerifyFull is set as a default. BadParameter error is returned if value set is incorrect.
func (m *TLSMode) CheckAndSetDefaults() error {
	switch *m {
	case "": // Use VerifyFull if not set.
		*m = VerifyFull
	case VerifyFull, VerifyCA, Insecure:
		// Correct value, do nothing.
	default:
		return trace.BadParameter("provided incorrect TLSMode value. Correct values are: %v", AllTLSModes)
	}

	return nil
}

// ToProto returns a matching protobuf type or VerifyFull for empty value.
func (m TLSMode) ToProto() types.DatabaseTLSMode {
	switch m {
	case VerifyCA:
		return types.DatabaseTLSMode_VERIFY_CA
	case Insecure:
		return types.DatabaseTLSMode_INSECURE
	default: // VerifyFull
		return types.DatabaseTLSMode_VERIFY_FULL
	}
}

// MySQLOptions are additional MySQL options.
type MySQLOptions struct {
	// ServerVersion is the version reported by Teleport DB Proxy on initial handshake.
	ServerVersion string
}

// DatabaseTLS keeps TLS settings used when connecting to database.
type DatabaseTLS struct {
	// Mode is the TLS connection mode. See TLSMode for more details.
	Mode TLSMode
	// ServerName allows providing custom server name.
	// This name will override DNS name when validating certificate presented by the database.
	ServerName string
	// CACert is an optional database CA certificate.
	CACert []byte
}

// DatabaseAWS contains AWS specific settings for RDS/Aurora databases.
type DatabaseAWS struct {
	// Region is the cloud region database is running in when using AWS RDS.
	Region string
	// Redshift contains Redshift specific settings.
	Redshift DatabaseAWSRedshift
	// RDS contains RDS specific settings.
	RDS DatabaseAWSRDS
	// ElastiCache contains ElastiCache specific settings.
	ElastiCache DatabaseAWSElastiCache
	// MemoryDB contains MemoryDB specific settings.
	MemoryDB DatabaseAWSMemoryDB
	// SecretStore contains settings for managing secrets.
	SecretStore DatabaseAWSSecretStore
	// AccountID is the AWS account ID.
	AccountID string
	// ExternalID is an optional AWS external ID used to enable assuming an AWS role across accounts.
	ExternalID string
	// RedshiftServerless contains AWS Redshift Serverless specific settings.
	RedshiftServerless DatabaseAWSRedshiftServerless
}

// DatabaseAWSRedshift contains AWS Redshift specific settings.
type DatabaseAWSRedshift struct {
	// ClusterID is the Redshift cluster identifier.
	ClusterID string
}

// DatabaseAWSRedshiftServerless contains AWS Redshift Serverless specific settings.
type DatabaseAWSRedshiftServerless struct {
	// WorkgroupName is the Redshift Serverless workgroup name.
	WorkgroupName string
	// EndpointName is the Redshift Serverless VPC endpoint name.
	EndpointName string
}

// DatabaseAWSRDS contains AWS RDS specific settings.
type DatabaseAWSRDS struct {
	// InstanceID is the RDS instance identifier.
	InstanceID string
	// ClusterID is the RDS cluster (Aurora) identifier.
	ClusterID string
}

// DatabaseAWSElastiCache contains settings for ElastiCache databases.
type DatabaseAWSElastiCache struct {
	// ReplicationGroupID is the ElastiCache replication group ID.
	ReplicationGroupID string
}

// DatabaseAWSMemoryDB contains settings for MemoryDB databases.
type DatabaseAWSMemoryDB struct {
	// ClusterName is the MemoryDB cluster name.
	ClusterName string
}

// DatabaseAWSSecretStore contains secret store configurations.
type DatabaseAWSSecretStore struct {
	// KeyPrefix specifies the secret key prefix.
	KeyPrefix string
	// KMSKeyID specifies the AWS KMS key for encryption.
	KMSKeyID string
}

// DatabaseGCP contains GCP specific settings for Cloud SQL databases.
type DatabaseGCP struct {
	// ProjectID is the GCP project ID where the database is deployed.
	ProjectID string
	// InstanceID is the Cloud SQL instance ID.
	InstanceID string
}

// DatabaseAD contains database Active Directory configuration.
type DatabaseAD struct {
	// KeytabFile is the path to the Kerberos keytab file.
	KeytabFile string
	// Krb5File is the path to the Kerberos configuration file. Defaults to /etc/krb5.conf.
	Krb5File string
	// Domain is the Active Directory domain the database resides in.
	Domain string
	// SPN is the service principal name for the database.
	SPN string
	// LDAPCert is the Active Directory LDAP Certificate.
	LDAPCert string
	// KDCHostName is the Key Distribution Center Hostname for x509 authentication
	KDCHostName string
}

// IsEmpty returns true if the database AD configuration is empty.
func (d *DatabaseAD) IsEmpty() bool {
	return d.KeytabFile == "" && d.Krb5File == "" && d.Domain == "" && d.SPN == ""
}

// DatabaseAzure contains Azure database configuration.
type DatabaseAzure struct {
	// ResourceID is the Azure fully qualified ID for the resource.
	ResourceID string
	// IsFlexiServer is true if the database is an Azure Flexible server.
	IsFlexiServer bool
}

// CheckAndSetDefaults validates database Active Directory configuration.
func (d *DatabaseAD) CheckAndSetDefaults(name string) error {
	if d.KeytabFile == "" && d.KDCHostName == "" {
		return trace.BadParameter("missing keytab file path or kdc_host_name for database %q", name)
	}
	if d.Krb5File == "" {
		d.Krb5File = defaults.Krb5FilePath
	}
	if d.Domain == "" {
		return trace.BadParameter("missing Active Directory domain for database %q", name)
	}
	if d.SPN == "" {
		return trace.BadParameter("missing service principal name for database %q", name)
	}

	if d.KDCHostName != "" {
		if d.LDAPCert == "" {
			return trace.BadParameter("missing LDAP certificate for x509 authentication for database %q", name)
		}
	}

	return nil
}

// CheckAndSetDefaults validates the database proxy configuration.
func (d *Database) CheckAndSetDefaults() error {
	if d.Name == "" {
		return trace.BadParameter("empty database name")
	}

	// Mark the database as coming from the static configuration.
	if d.StaticLabels == nil {
		d.StaticLabels = make(map[string]string)
	}
	d.StaticLabels[types.OriginLabel] = types.OriginConfigFile

	if err := d.TLS.Mode.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// We support Azure AD authentication and Kerberos auth with AD for SQL
	// Server. The first method doesn't require additional configuration since
	// it assumes the environment’s Azure credentials
	// (https://learn.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication).
	// The second method requires additional information, validated by
	// DatabaseAD.
	if d.Protocol == defaults.ProtocolSQLServer && (d.AD.Domain != "" || !strings.Contains(d.URI, azureutils.MSSQLEndpointSuffix)) {
		if err := d.AD.CheckAndSetDefaults(d.Name); err != nil {
			return trace.Wrap(err)
		}
	}

	// Do a test run with extra validations.
	db, err := d.ToDatabase()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(services.ValidateDatabase(db))
}

// ToDatabase converts Database to types.Database.
func (d *Database) ToDatabase() (types.Database, error) {
	return types.NewDatabaseV3(types.Metadata{
		Name:        d.Name,
		Description: d.Description,
		Labels:      d.StaticLabels,
	}, types.DatabaseSpecV3{
		Protocol: d.Protocol,
		URI:      d.URI,
		CACert:   string(d.TLS.CACert),
		TLS: types.DatabaseTLS{
			CACert:     string(d.TLS.CACert),
			ServerName: d.TLS.ServerName,
			Mode:       d.TLS.Mode.ToProto(),
		},
		MySQL: types.MySQLOptions{
			ServerVersion: d.MySQL.ServerVersion,
		},
		AWS: types.AWS{
			AccountID:  d.AWS.AccountID,
			ExternalID: d.AWS.ExternalID,
			Region:     d.AWS.Region,
			Redshift: types.Redshift{
				ClusterID: d.AWS.Redshift.ClusterID,
			},
			RedshiftServerless: types.RedshiftServerless{
				WorkgroupName: d.AWS.RedshiftServerless.WorkgroupName,
				EndpointName:  d.AWS.RedshiftServerless.EndpointName,
			},
			RDS: types.RDS{
				InstanceID: d.AWS.RDS.InstanceID,
				ClusterID:  d.AWS.RDS.ClusterID,
			},
			ElastiCache: types.ElastiCache{
				ReplicationGroupID: d.AWS.ElastiCache.ReplicationGroupID,
			},
			MemoryDB: types.MemoryDB{
				ClusterName: d.AWS.MemoryDB.ClusterName,
			},
			SecretStore: types.SecretStore{
				KeyPrefix: d.AWS.SecretStore.KeyPrefix,
				KMSKeyID:  d.AWS.SecretStore.KMSKeyID,
			},
		},
		GCP: types.GCPCloudSQL{
			ProjectID:  d.GCP.ProjectID,
			InstanceID: d.GCP.InstanceID,
		},
		DynamicLabels: types.LabelsToV2(d.DynamicLabels),
		AD: types.AD{
			KeytabFile:  d.AD.KeytabFile,
			Krb5File:    d.AD.Krb5File,
			Domain:      d.AD.Domain,
			SPN:         d.AD.SPN,
			LDAPCert:    d.AD.LDAPCert,
			KDCHostName: d.AD.KDCHostName,
		},
		Azure: types.Azure{
			ResourceID:    d.Azure.ResourceID,
			IsFlexiServer: d.Azure.IsFlexiServer,
		},
	})
}

// AppsConfig configures application proxy service.
type AppsConfig struct {
	// Enabled enables application proxying service.
	Enabled bool

	// DebugApp enabled a header dumping debugging application.
	DebugApp bool

	// Apps is the list of applications that are being proxied.
	Apps []App

	// ResourceMatchers match cluster database resources.
	ResourceMatchers []services.ResourceMatcher

	// MonitorCloseChannel will be signaled when a monitor closes a connection.
	// Used only for testing. Optional.
	MonitorCloseChannel chan struct{}
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

	// AWS contains additional options for AWS applications.
	AWS *AppAWS

	// Cloud identifies the cloud instance the app represents.
	Cloud string
}

// CheckAndSetDefaults validates an application.
func (a *App) CheckAndSetDefaults() error {
	if a.Name == "" {
		return trace.BadParameter("missing application name")
	}
	if a.URI == "" {
		if a.Cloud != "" {
			a.URI = fmt.Sprintf("cloud://%v", a.Cloud)
		} else {
			return trace.BadParameter("missing application %q URI", a.Name)
		}
	}
	// Check if the application name is a valid subdomain. Don't allow names that
	// are invalid subdomains because for trusted clusters the name is used to
	// construct the domain that the application will be available at.
	if errs := validation.IsDNS1035Label(a.Name); len(errs) > 0 {
		return trace.BadParameter("application name %q must be a valid DNS subdomain: https://goteleport.com/docs/application-access/guides/connecting-apps/#application-name", a.Name)
	}
	// Parse and validate URL.
	if _, err := url.Parse(a.URI); err != nil {
		return trace.BadParameter("application %q URI invalid: %v", a.Name, err)
	}
	// If a port was specified or an IP address was provided for the public
	// address, return an error.
	if a.PublicAddr != "" {
		if _, _, err := net.SplitHostPort(a.PublicAddr); err == nil {
			return trace.BadParameter("application %q public_addr %q can not contain a port, applications will be available on the same port as the web proxy", a.Name, a.PublicAddr)
		}
		if net.ParseIP(a.PublicAddr) != nil {
			return trace.BadParameter("application %q public_addr %q can not be an IP address, Teleport Application Access uses DNS names for routing", a.Name, a.PublicAddr)
		}
	}
	// Mark the app as coming from the static configuration.
	if a.StaticLabels == nil {
		a.StaticLabels = make(map[string]string)
	}
	a.StaticLabels[types.OriginLabel] = types.OriginConfigFile
	// Make sure there are no reserved headers in the rewrite configuration.
	// They wouldn't be rewritten even if we allowed them here but catch it
	// early and let the user know.
	if a.Rewrite != nil {
		for _, h := range a.Rewrite.Headers {
			if common.IsReservedHeader(h.Name) {
				return trace.BadParameter("invalid application %q header rewrite configuration: header %q is reserved and can't be rewritten",
					a.Name, http.CanonicalHeaderKey(h.Name))
			}
		}
	}
	return nil
}

// MetricsConfig specifies configuration for the metrics service
type MetricsConfig struct {
	// Enabled turns the metrics service role on or off for this process
	Enabled bool

	// ListenAddr is the address to listen on for incoming metrics requests.
	// Optional.
	ListenAddr *utils.NetAddr

	// MTLS turns mTLS on the metrics service on or off
	MTLS bool

	// KeyPairs are the key and certificate pairs that the metrics service will
	// use for mTLS.
	// Used in conjunction with MTLS = true
	KeyPairs []KeyPairPath

	// CACerts are prometheus ca certs
	// use for mTLS.
	// Used in conjunction with MTLS = true
	CACerts []string

	// GRPCServerLatency enables histogram metrics for each grpc endpoint on the auth server
	GRPCServerLatency bool

	// GRPCServerLatency enables histogram metrics for each grpc endpoint on the auth server
	GRPCClientLatency bool
}

// TracingConfig specifies the configuration for the tracing service
type TracingConfig struct {
	// Enabled turns the tracing service role on or off for this process.
	Enabled bool

	// ExporterURL is the OTLP exporter URL to send spans to.
	ExporterURL string

	// KeyPairs are the paths for key and certificate pairs that the tracing
	// service will use for outbound TLS connections.
	KeyPairs []KeyPairPath

	// CACerts are the paths to the CA certs used to validate the collector.
	CACerts []string

	// SamplingRate is the sampling rate for the exporter.
	// 1.0 will record and export all spans and 0.0 won't record any spans.
	SamplingRate float64
}

// Config generates a tracing.Config that is populated from the values
// provided to the tracing_service
func (t TracingConfig) Config(attrs ...attribute.KeyValue) (*tracing.Config, error) {
	traceConf := &tracing.Config{
		Service:      teleport.ComponentTeleport,
		Attributes:   attrs,
		ExporterURL:  t.ExporterURL,
		SamplingRate: t.SamplingRate,
	}

	tlsConfig := &tls.Config{}
	// if a custom CA is specified, use a custom cert pool
	if len(t.CACerts) > 0 {
		pool := x509.NewCertPool()
		for _, caCertPath := range t.CACerts {
			caCert, err := os.ReadFile(caCertPath)
			if err != nil {
				return nil, trace.Wrap(err, "failed to read tracing CA certificate %+v", caCertPath)
			}

			if !pool.AppendCertsFromPEM(caCert) {
				return nil, trace.BadParameter("failed to parse tracing CA certificate: %+v", caCertPath)
			}
		}
		tlsConfig.ClientCAs = pool
		tlsConfig.RootCAs = pool
	}

	// add any custom certificates for mTLS
	if len(t.KeyPairs) > 0 {
		for _, pair := range t.KeyPairs {
			certificate, err := tls.LoadX509KeyPair(pair.Certificate, pair.PrivateKey)
			if err != nil {
				return nil, trace.Wrap(err, "failed to read keypair: %+v", err)
			}
			tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
		}
	}

	if len(t.CACerts) > 0 || len(t.KeyPairs) > 0 {
		traceConf.TLSConfig = tlsConfig
	}
	return traceConf, nil
}

// WindowsDesktopConfig specifies the configuration for the Windows Desktop
// Access service.
type WindowsDesktopConfig struct {
	Enabled bool
	// ListenAddr is the address to listed on for incoming desktop connections.
	ListenAddr utils.NetAddr
	// PublicAddrs is a list of advertised public addresses of the service.
	PublicAddrs []utils.NetAddr
	// ShowDesktopWallpaper determines whether desktop sessions will show a
	// user-selected wallpaper vs a system-default, single-color wallpaper.
	ShowDesktopWallpaper bool
	// LDAP is the LDAP connection parameters.
	LDAP LDAPConfig

	// Discovery configures automatic desktop discovery via LDAP.
	Discovery LDAPDiscoveryConfig

	// Hosts is an optional list of static Windows hosts to expose through this
	// service.
	// Hosts is an optional list of static, AD-connected Windows hosts. This gives users
	// a way to specify AD-connected hosts that won't be found by the filters
	// specified in Discovery (or if Discovery is omitted).
	Hosts []utils.NetAddr

	// NonADHosts is an optional list of static Windows hosts to expose through this
	// service. These hosts are not part of Active Directory.
	NonADHosts []utils.NetAddr

	// ConnLimiter limits the connection and request rates.
	ConnLimiter limiter.Config
	// HostLabels specifies rules that are used to apply labels to Windows hosts.
	HostLabels HostLabelRules
	Labels     map[string]string
}

// LDAPDiscoveryConfig is LDAP discovery configuration for windows desktop discovery service.
type LDAPDiscoveryConfig struct {
	// BaseDN is the base DN to search for desktops.
	// Use the value '*' to search from the root of the domain,
	// or leave blank to disable desktop discovery.
	BaseDN string
	// Filters are additional LDAP filters to apply to the search.
	// See: https://ldap.com/ldap-filters/
	Filters []string
	// LabelAttributes are LDAP attributes to apply to hosts discovered
	// via LDAP. Teleport labels hosts by prefixing the attribute with
	// "ldap/" - for example, a value of "location" here would result in
	// discovered desktops having a label with key "ldap/location" and
	// the value being the value of the "location" attribute.
	LabelAttributes []string
}

// HostLabelRules is a collection of rules describing how to apply labels to hosts.
type HostLabelRules struct {
	rules  []HostLabelRule
	labels map[string]map[string]string
}

func NewHostLabelRules(rules ...HostLabelRule) HostLabelRules {
	return HostLabelRules{
		rules: rules,
	}
}

// LabelsForHost returns the set of all labels that should be applied
// to the specified host. If multiple rules match and specify the same
// label keys, the value will be that of the last matching rule.
func (h HostLabelRules) LabelsForHost(host string) map[string]string {
	labels, ok := h.labels[host]
	if ok {
		return labels
	}

	result := make(map[string]string)
	for _, rule := range h.rules {
		if rule.Regexp.MatchString(host) {
			for k, v := range rule.Labels {
				result[k] = v
			}
		}
	}

	if h.labels == nil {
		h.labels = make(map[string]map[string]string)
	}
	h.labels[host] = result

	return result
}

// HostLabelRule specifies a set of labels that should be applied to
// hosts matching the provided regexp.
type HostLabelRule struct {
	Regexp *regexp.Regexp
	Labels map[string]string
}

// LDAPConfig is the LDAP connection parameters.
type LDAPConfig struct {
	// Addr is the address:port of the LDAP server (typically port 389).
	Addr string
	// Domain is the ActiveDirectory domain name.
	Domain string
	// Username for LDAP authentication.
	Username string
	// SID is the SID for the user specified by Username.
	SID string
	// InsecureSkipVerify decides whether whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool
	// ServerName is the name of the LDAP server for TLS.
	ServerName string
	// CA is an optional CA cert to be used for verification if InsecureSkipVerify is set to false.
	CA *x509.Certificate
}

// Rewrite is a list of rewriting rules to apply to requests and responses.
type Rewrite struct {
	// Redirect is a list of hosts that should be rewritten to the public address.
	Redirect []string
	// Headers is a list of extra headers to inject in the request.
	Headers []Header
}

// Header represents a single http header passed over to the proxied application.
type Header struct {
	// Name is the http header name.
	Name string
	// Value is the http header value.
	Value string
}

type DiscoveryConfig struct {
	Enabled bool
	// DiscoveryGroup is the name of the discovery group that the current
	// discovery service is a part of.
	// It is used to filter out discovered resources that belong to another
	// discovery services. When running in high availability mode and the agents
	// have access to the same cloud resources, this field value must be the same
	// for all discovery services. If different agents are used to discover different
	// sets of cloud resources, this field must be different for each set of agents.
	DiscoveryGroup string
	// AWSMatchers are used to match EC2 instances for auto enrollment.
	AWSMatchers []services.AWSMatcher
	// AzureMatchers are used to match resources for auto enrollment.
	AzureMatchers []services.AzureMatcher
	// GCPMatchers are used to match GCP resources for auto discovery.
	GCPMatchers []services.GCPMatcher
}

// IsEmpty validates if the Discovery Service config has no cloud matchers.
func (d DiscoveryConfig) IsEmpty() bool {
	return len(d.AWSMatchers) == 0 &&
		len(d.AzureMatchers) == 0 && len(d.GCPMatchers) == 0
}

// HostedPluginsConfig configures the hosted plugin runtime.
type HostedPluginsConfig struct {
	Enabled        bool
	OAuthProviders PluginOAuthProviders
}

// PluginOAuthProviders holds application credentials for each
// 3rd party API provider
type PluginOAuthProviders struct {
	Slack *oauth2.ClientCredentials
}

// ParseHeader parses the provided string as a http header.
func ParseHeader(header string) (*Header, error) {
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		return nil, trace.BadParameter("failed to parse %q as http header", header)
	}
	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if !httpguts.ValidHeaderFieldName(name) {
		return nil, trace.BadParameter("invalid http header name: %q", header)
	}
	if !httpguts.ValidHeaderFieldValue(value) {
		return nil, trace.BadParameter("invalid http header value: %q", header)
	}
	return &Header{
		Name:  name,
		Value: value,
	}, nil
}

// ParseHeaders parses the provided list as http headers.
func ParseHeaders(headers []string) (headersOut []Header, err error) {
	for _, header := range headers {
		h, err := ParseHeader(header)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		headersOut = append(headersOut, *h)
	}
	return headersOut, nil
}

// AppAWS contains additional options for AWS applications.
type AppAWS struct {
	// ExternalID is the AWS External ID used when assuming roles in this app.
	ExternalID string
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

	cfg.Version = defaults.TeleportConfigVersionV1

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
	cfg.Auth.ListenAddr = *defaults.AuthListenAddr()
	cfg.Auth.StorageConfig.Type = lite.GetName()
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	cfg.Auth.StaticTokens = types.DefaultStaticTokens()
	cfg.Auth.AuditConfig = types.DefaultClusterAuditConfig()
	cfg.Auth.NetworkingConfig = types.DefaultClusterNetworkingConfig()
	cfg.Auth.SessionRecordingConfig = types.DefaultSessionRecordingConfig()
	cfg.Auth.Preference = types.DefaultAuthPreference()
	defaults.ConfigureLimiter(&cfg.Auth.Limiter)
	cfg.Auth.LicenseFile = filepath.Join(cfg.DataDir, defaults.LicenseFile)

	cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	// Proxy service defaults.
	cfg.Proxy.Enabled = true
	cfg.Proxy.Kube.Enabled = false
	cfg.Proxy.IdP.SAMLIdP.Enabled = true

	defaults.ConfigureLimiter(&cfg.Proxy.Limiter)

	// SSH service defaults.
	cfg.SSH.Enabled = true
	cfg.SSH.Shell = defaults.DefaultShell
	defaults.ConfigureLimiter(&cfg.SSH.Limiter)
	cfg.SSH.PAM = &pam.Config{Enabled: false}
	cfg.SSH.BPF = &bpf.Config{Enabled: false}
	cfg.SSH.RestrictedSession = &bpf.RestrictedSessionConfig{Enabled: false}
	cfg.SSH.AllowTCPForwarding = true
	cfg.SSH.AllowFileCopying = true

	// Kubernetes service defaults.
	cfg.Kube.Enabled = false
	defaults.ConfigureLimiter(&cfg.Kube.Limiter)

	// Apps service defaults. It's disabled by default.
	cfg.Apps.Enabled = false

	// Databases proxy service is disabled by default.
	cfg.Databases.Enabled = false
	defaults.ConfigureLimiter(&cfg.Databases.Limiter)

	// Metrics service defaults.
	cfg.Metrics.Enabled = false

	// Windows desktop service is disabled by default.
	cfg.WindowsDesktop.Enabled = false
	defaults.ConfigureLimiter(&cfg.WindowsDesktop.ConnLimiter)

	cfg.RotationConnectionInterval = defaults.HighResPollingPeriod
	cfg.MaxRetryPeriod = defaults.MaxWatcherBackoff
	cfg.ConnectFailureC = make(chan time.Duration, 1)
	cfg.CircuitBreakerConfig = breaker.DefaultBreakerConfig(cfg.Clock)
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
	cfg.Auth.Preference.SetAllowLocalAuth(false)

	// Update cluster configuration to record sessions at node, this way the
	// entire cluster is FedRAMP/FIPS 140-2 compliant.
	cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtNode)
}
