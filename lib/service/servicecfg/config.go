/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package servicecfg contains the runtime configuration for Teleport services
package servicecfg

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Config contains the configuration for all services that Teleport can run.
// Some settings are global, while others are grouped into separate sections.
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
	Identities []*state.Identity

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

	// DebugService defines the debug service configuration.
	DebugService DebugConfig

	// WindowsDesktop defines the Windows desktop service configuration.
	WindowsDesktop WindowsDesktopConfig

	// Discovery defines the discovery service configuration.
	Discovery DiscoveryConfig

	// OpenSSH defines the configuration for an openssh node
	OpenSSH OpenSSHConfig

	// Okta defines the okta service configuration.
	Okta OktaConfig

	// Jamf defines the Jamf MDM service configuration.
	Jamf JamfConfig

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

	// Trust is a service that manages certificate authorities
	Trust services.TrustInternal

	// Presence service is a discovery and heartbeat tracker
	Presence services.PresenceInternal

	// Events is events service
	Events types.Events

	// Provisioner is a service that keeps track of provisioning tokens
	Provisioner services.Provisioner

	// Identity is a service that manages users and credentials
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

	// FileDescriptors is an optional list of file descriptors for the process
	// to inherit and use for listeners, used for in-process updates.
	FileDescriptors []*FileDescriptor

	// PollingPeriod is set to override default internal polling periods
	// of sync agents, used to speed up integration tests.
	PollingPeriod time.Duration

	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string

	// Clock is used to control time in tests.
	Clock clockwork.Clock

	// FIPS means FedRAMP/FIPS 140-2 compliant configuration was requested.
	FIPS bool

	// SkipVersionCheck means the version checking between server and client
	// will be skipped.
	SkipVersionCheck bool

	// BPFConfig holds configuration for the BPF service.
	BPFConfig *BPFConfig

	// Kube is a Kubernetes API gateway using Teleport client identities.
	Kube KubeConfig

	// Log optionally specifies the logger.
	// Deprecated: use Logger instead.
	Log utils.Logger
	// Logger outputs messages using slog. The underlying handler respects
	// the user supplied logging config.
	Logger *slog.Logger
	// LoggerLevel defines the Logger log level.
	LoggerLevel *slog.LevelVar

	// PluginRegistry allows adding enterprise logic to Teleport services
	PluginRegistry plugin.Registry

	// RotationConnectionInterval is the interval between connection
	// attempts as used by the rotation state service
	RotationConnectionInterval time.Duration

	// MaxRetryPeriod is the maximum period between reconnection attempts to auth
	MaxRetryPeriod time.Duration

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
	InstanceMetadataClient imds.Client

	// Testing is a group of properties that are used in tests.
	Testing ConfigTesting

	// AccessGraph represents AccessGraph server config
	AccessGraph AccessGraphConfig

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

type ConfigTesting struct {
	// ConnectFailureC is a channel to notify of failures to connect to auth (used in tests).
	ConnectFailureC chan time.Duration

	// UploadEventsC is a channel for upload events used in tests
	UploadEventsC chan events.UploadEvent `json:"-"`

	// ClientTimeout is set to override default client timeouts
	// used by internal clients, used to speed up integration tests.
	ClientTimeout time.Duration

	// ShutdownTimeout is set to override default shutdown timeout.
	ShutdownTimeout time.Duration

	// TeleportVersion is used to control the Teleport version in tests.
	TeleportVersion string

	// KubeMultiplexerIgnoreSelfConnections signals that Proxy TLS server's listener should
	// require PROXY header if 'proxyProtocolMode: true' even from self connections. Used in tests as all connections are self
	// connections there.
	KubeMultiplexerIgnoreSelfConnections bool

	// OpenAIConfig contains the optional OpenAI client configuration used by
	// auth and proxy. When it's not set (the default, we don't offer a way to
	// set it when executing the regular Teleport binary) we use the default
	// configuration with auth tokens passed from Auth.AssistAPIKey or
	// Proxy.AssistAPIKey. We set this only when testing to avoid calls to reach
	// the real OpenAI API.
	// Note: When set, this overrides Auth and Proxy's AssistAPIKey settings.
	OpenAIConfig *openai.ClientConfig
}

// AccessGraphConfig represents TAG server config
type AccessGraphConfig struct {
	// Enabled Access Graph reporting enabled
	Enabled bool

	// Addr of the Access Graph service addr
	Addr string

	// CA is the path to the CA certificate file
	CA string

	// Insecure is true if the connection to the Access Graph service should be insecure
	Insecure bool
}

// RoleAndIdentityEvent is a role and its corresponding identity event.
type RoleAndIdentityEvent struct {
	// Role is a system role.
	Role types.SystemRole

	// IdentityEvent is the identity event associated with the above role.
	IdentityEvent string
}

// DisableLongRunningServices disables all services but OpenSSH
func DisableLongRunningServices(cfg *Config) {
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false
	cfg.Kube.Enabled = false
	cfg.Apps.Enabled = false
	cfg.WindowsDesktop.Enabled = false
	cfg.Databases.Enabled = false
	cfg.Okta.Enabled = false
}

// JoinParams is a set of extra parameters for joining the auth server.
type JoinParams struct {
	Azure AzureJoinParams
}

// AzureJoinParams is the parameters specific to the azure join method.
type AzureJoinParams struct {
	ClientID string
}

// CachePolicy sets caching policy for proxies and nodes
type CachePolicy struct {
	// Enabled enables or disables caching
	Enabled bool
	// MaxRetryPeriod is maximum period cache waits before retrying on failure.
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

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.LoggerLevel == nil {
		cfg.LoggerLevel = new(slog.LevelVar)
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
		cfg.Logger.ErrorContext(context.Background(), "Failed to determine hostname", "error", err)
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
	cfg.SSH.PAM = &PAMConfig{Enabled: false}
	cfg.SSH.BPF = &BPFConfig{Enabled: false}
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
	cfg.Testing.ConnectFailureC = make(chan time.Duration, 1)
	cfg.CircuitBreakerConfig = breaker.DefaultBreakerConfig(cfg.Clock)
}

// FileDescriptor is a file descriptor associated
// with a listener
type FileDescriptor struct {
	once sync.Once

	// Type is a listener type, e.g. auth:ssh
	Type string
	// Address is an address of the listener, e.g. 127.0.0.1:3025
	Address string
	// File is a file descriptor associated with the listener
	File *os.File
}

func (fd *FileDescriptor) Close() error {
	var err error
	fd.once.Do(func() {
		err = fd.File.Close()
	})
	return trace.Wrap(err)
}

func (fd *FileDescriptor) ToListener() (net.Listener, error) {
	listener, err := net.FileListener(fd.File)
	if err != nil {
		return nil, err
	}
	if err := fd.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	return listener, nil
}

func ValidateConfig(cfg *Config) error {
	applyDefaults(cfg)

	if err := defaults.ValidateConfigVersion(cfg.Version); err != nil {
		return err
	}

	if err := verifyEnabledService(cfg); err != nil {
		return err
	}

	if err := validateAuthOrProxyServices(cfg); err != nil {
		return err
	}

	if cfg.DataDir == "" {
		return trace.BadParameter("config: please supply data directory")
	}

	for i := range cfg.Auth.Authorities {
		if err := services.ValidateCertAuthority(cfg.Auth.Authorities[i]); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, tun := range cfg.ReverseTunnels {
		if err := services.ValidateReverseTunnel(tun); err != nil {
			return trace.Wrap(err)
		}
	}

	cfg.SSH.Namespace = types.ProcessNamespace(cfg.SSH.Namespace)

	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Version == "" {
		cfg.Version = defaults.TeleportConfigVersionV1
	}

	if cfg.Console == nil {
		cfg.Console = io.Discard
	}

	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.LoggerLevel == nil {
		cfg.LoggerLevel = new(slog.LevelVar)
	}

	if cfg.PollingPeriod == 0 {
		cfg.PollingPeriod = defaults.LowResPollingPeriod
	}
}

func validateAuthOrProxyServices(cfg *Config) error {
	haveAuthServers := len(cfg.authServers) > 0
	haveProxyServer := !cfg.ProxyServer.IsEmpty()

	if cfg.Version == defaults.TeleportConfigVersionV3 {
		if haveAuthServers && haveProxyServer {
			return trace.BadParameter("config: cannot use both auth_server and proxy_server")
		}

		if !haveAuthServers && !haveProxyServer {
			return trace.BadParameter("config: auth_server or proxy_server is required")
		}

		if !cfg.Auth.Enabled {
			if haveAuthServers && cfg.Apps.Enabled {
				return trace.BadParameter("config: when app_service is enabled, proxy_server must be specified instead of auth_server")
			}

			if haveAuthServers && cfg.Databases.Enabled {
				return trace.BadParameter("config: when db_service is enabled, proxy_server must be specified instead of auth_server")
			}
		}

		if haveProxyServer {
			port := cfg.ProxyServer.Port(0)
			if port == defaults.AuthListenPort {
				cfg.Logger.WarnContext(context.Background(), "config: proxy_server is pointing to port 3025, is this the auth server address?")
			}
		}

		if haveAuthServers {
			authServerPort := cfg.authServers[0].Port(0)
			checkPorts := []int{defaults.HTTPListenPort, teleport.StandardHTTPSPort}
			for _, port := range checkPorts {
				if authServerPort == port {
					cfg.Logger.WarnContext(context.Background(), "config: auth_server is pointing to port 3080 or 443, is this the proxy server address?")
				}
			}
		}

		return nil
	}

	if haveProxyServer {
		return trace.BadParameter("config: proxy_server is supported from config version v3 onwards")
	}

	if !haveAuthServers {
		return trace.BadParameter("config: auth_servers is required")
	}

	return nil
}

func verifyEnabledService(cfg *Config) error {
	enabled := []bool{
		cfg.Auth.Enabled,
		cfg.SSH.Enabled,
		cfg.Proxy.Enabled,
		cfg.Kube.Enabled,
		cfg.Apps.Enabled,
		cfg.Databases.Enabled,
		cfg.WindowsDesktop.Enabled,
		cfg.Discovery.Enabled,
		cfg.Okta.Enabled,
		cfg.Jamf.Enabled(),
		cfg.OpenSSH.Enabled,
	}

	for _, item := range enabled {
		if item {
			return nil
		}
	}

	return trace.BadParameter(
		"config: enable at least one of auth_service, ssh_service, proxy_service, app_service, database_service, kubernetes_service, windows_desktop_service, discovery_service, okta_service or jamf_service")
}

// SetLogLevel changes the loggers log level.
//
// If called after `config.ApplyFileConfig` or `config.Configure` it will also
// change the global loggers.
func (c *Config) SetLogLevel(level slog.Level) {
	c.Log.SetLevel(logutils.SlogLevelToLogrusLevel(level))
	c.LoggerLevel.Set(level)
}

// GetLogLevel returns the current log level.
func (c *Config) GetLogLevel() slog.Level {
	return c.LoggerLevel.Level()
}
