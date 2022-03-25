/*
Copyright 2016-2020 Gravitational, Inc.

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

// Package defaults contains default constants set in various parts of
// teleport codebase
package defaults

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/square/go-jose.v2"

	"golang.org/x/crypto/ssh"
)

// Default port numbers used by all teleport tools
const (
	// Web UI over HTTP(s)
	HTTPListenPort = 3080

	// When running in "SSH Server" mode behind a proxy, this
	// listening port will be used to connect users to:
	SSHServerListenPort = 3022

	// When running in "SSH Proxy" role this port will be used to
	// accept incoming client connections and proxy them to SSHServerListenPort of
	// one of many SSH nodes
	SSHProxyListenPort = 3023

	SSHProxyTunnelListenPort = defaults.SSHProxyTunnelListenPort

	// KubeListenPort is a default port for kubernetes proxies
	KubeListenPort = 3026

	// When running as a "SSH Proxy" this port will be used to
	// serve auth requests.
	AuthListenPort = 3025

	// MySQLListenPort is the default listen port for MySQL proxy.
	MySQLListenPort = 3036

	// PostgresListenPort is the default listen port for PostgreSQL proxy.
	PostgresListenPort = 5432

	// MongoListenPort is the default listen port for Mongo proxy.
	MongoListenPort = 27017

	// RedisListenPort is the default listen port for Redis proxy.
	RedisListenPort = 6379

	// MetricsListenPort is the default listen port for the metrics service.
	MetricsListenPort = 3081

	// WindowsDesktopListenPort is the default listed port for
	// windows_desktop_service.
	//
	// TODO(awly): update to match HTTPListenPort once SNI routing is
	// implemented.
	WindowsDesktopListenPort = 3028

	// RDPListenPort is the standard port for RDP servers.
	RDPListenPort = 3389

	// Default DB to use for persisting state. Another options is "etcd"
	BackendType = "bolt"

	// BackendDir is a default backend subdirectory
	BackendDir = "backend"

	// BackendPath is a default backend path parameter
	BackendPath = "path"

	// Name of events bolt database file stored in DataDir
	EventsBoltFile = "events.db"

	// By default SSH server (and SSH proxy) will bind to this IP
	BindIP = "0.0.0.0"

	// By default all users use /bin/bash
	DefaultShell = "/bin/bash"

	// CacheTTL is a default cache TTL for persistent node cache
	CacheTTL = 20 * time.Hour

	// RecentCacheTTL is a default cache TTL for recently accessed items
	RecentCacheTTL = 2 * time.Second

	// InviteTokenTTL sets the lifespan of tokens used for adding nodes and users
	// to a cluster
	InviteTokenTTL = 15 * time.Minute

	// HTTPMaxIdleConns is the max idle connections across all hosts.
	HTTPMaxIdleConns = 2000

	// HTTPMaxIdleConnsPerHost is the max idle connections per-host.
	HTTPMaxIdleConnsPerHost = 1000

	// HTTPMaxConnsPerHost is the maximum number of connections per-host.
	HTTPMaxConnsPerHost = 250

	// HTTPIdleTimeout is a default timeout for idle HTTP connections
	HTTPIdleTimeout = 30 * time.Second

	// DefaultThrottleTimeout is a timemout used to throttle failed auth servers
	DefaultThrottleTimeout = 10 * time.Second

	// WebHeadersTimeout is a timeout that is set for web requests
	// before browsers raise "Timeout waiting web headers" error in
	// the browser
	WebHeadersTimeout = 10 * time.Second

	// DefaultIdleConnectionDuration indicates for how long Teleport will hold
	// the SSH connection open if there are no reads/writes happening over it.
	// 15 minutes default is compliant with PCI DSS standards
	DefaultIdleConnectionDuration = 15 * time.Minute

	// DefaultGracefulShutdownTimeout is a default timeout for
	// graceful shutdown waiting for connections to drain off
	// before cutting the connections forcefully.
	DefaultGracefulShutdownTimeout = 5 * time.Minute

	// ShutdownPollPeriod is a polling period for graceful shutdowns of SSH servers
	ShutdownPollPeriod = 500 * time.Millisecond

	// ReadHeadersTimeout is a default TCP timeout when we wait
	// for the response headers to arrive
	ReadHeadersTimeout = time.Second

	// DatabaseConnectTimeout is a timeout for connecting to a database via
	// database access.
	DatabaseConnectTimeout = time.Minute

	// HandshakeReadDeadline is the default time to wait for the client during
	// the TLS handshake.
	HandshakeReadDeadline = 5 * time.Second

	// SignupTokenTTL is a default TTL for a web signup one time token
	SignupTokenTTL = time.Hour

	// MaxSignupTokenTTL is a maximum TTL for a web signup one time token
	// clients can reduce this time, not increase it
	MaxSignupTokenTTL = 48 * time.Hour

	// MaxChangePasswordTokenTTL is a maximum TTL for password change token
	MaxChangePasswordTokenTTL = 24 * time.Hour

	// ChangePasswordTokenTTL is a default password change token expiry time
	ChangePasswordTokenTTL = 8 * time.Hour

	// DefaultRenewableCertTTL is the default TTL for a renewable user certificate.
	DefaultRenewableCertTTL = 1 * time.Hour

	// MaxRenewableCertTTL is the maximum TTL that a certificate renewal bot
	// can request for a renewable user certificate.
	MaxRenewableCertTTL = 24 * time.Hour

	// DefaultBotJoinTTL is the default TTL for bot join tokens.
	DefaultBotJoinTTL = 1 * time.Hour

	// RecoveryStartTokenTTL is a default expiry time for a recovery start token.
	RecoveryStartTokenTTL = 3 * time.Hour

	// RecoveryApprovedTokenTTL is a default expiry time for a recovery approved token.
	RecoveryApprovedTokenTTL = 15 * time.Minute

	// PrivilegeTokenTTL is a default expiry time for a privilege token.
	PrivilegeTokenTTL = 5 * time.Minute

	// ResetPasswordLength is the length of the reset user password
	ResetPasswordLength = 16

	// ProvisioningTokenTTL is a the default TTL for server provisioning
	// tokens. When a user generates a token without an explicit TTL, this
	// value is used.
	ProvisioningTokenTTL = 30 * time.Minute

	// HOTPFirstTokensRange is amount of lookahead tokens we remember
	// for sync purposes
	HOTPFirstTokensRange = 4

	// HOTPTokenDigits is the number of digits in each token
	HOTPTokenDigits = 6

	// MinPasswordLength is minimum password length
	MinPasswordLength = 6

	// MaxPasswordLength is maximum password length (for sanity)
	MaxPasswordLength = 128

	// IterationLimit is a default limit if it's not set
	IterationLimit = 100

	// MaxIterationLimit is max iteration limit
	MaxIterationLimit = 1000

	// EventsIterationLimit is a default limit if it's not set for events
	EventsIterationLimit = 500

	// EventsIterationLimit is max iteration limit for events
	EventsMaxIterationLimit = 10000

	// ActiveSessionTTL is a TTL when session is marked as inactive
	ActiveSessionTTL = 30 * time.Second

	// ActivePartyTTL is a TTL when party is marked as inactive
	ActivePartyTTL = 30 * time.Second

	// OIDCAuthRequestTTL is TTL of internally stored auth request created by client
	OIDCAuthRequestTTL = 10 * 60 * time.Second

	// SAMLAuthRequestTTL is TTL of internally stored auth request created by client
	SAMLAuthRequestTTL = 10 * 60 * time.Second

	// GithubAuthRequestTTL is TTL of internally stored Github auth request
	GithubAuthRequestTTL = 10 * 60 * time.Second

	// OAuth2TTL is the default TTL for objects created during OAuth 2.0 flow
	// such as web sessions, certificates or dynamically created users
	OAuth2TTL = 60 * 60 * time.Second // 1 hour

	// LogRotationPeriod defines how frequently to rotate the audit log file
	LogRotationPeriod = (time.Hour * 24)

	// UploaderScanPeriod is a default uploader scan period
	UploaderScanPeriod = 5 * time.Second

	// UploaderConcurrentUploads is a default number of concurrent
	UploaderConcurrentUploads = 10

	// MaxLoginAttempts sets the max. number of allowed failed login attempts
	// before a user account is locked for AccountLockInterval
	MaxLoginAttempts int = 5

	// MaxAccountRecoveryAttempts sets the max number of allowed failed recovery attempts
	// before a user is locked from login and further recovery attempts for AccountLockInterval.
	MaxAccountRecoveryAttempts = 3

	// AccountLockInterval defines a time interval during which a user account
	// is locked after MaxLoginAttempts
	AccountLockInterval = 20 * time.Minute

	// AttemptTTL is TTL for login attempt
	AttemptTTL = time.Minute * 30

	// AuditLogSessions is the default expected amount of concurrent sessions
	// supported by Audit logger, this number limits the possible
	// amount of simultaneously processes concurrent sessions by the
	// Audit log server, and 16K is OK for now
	AuditLogSessions = 16384

	// AccessPointCachedValues is the default maximum amount of cached values
	// in access point
	AccessPointCachedValues = 16384

	// AuditLogTimeFormat is the format for the timestamp on audit log files.
	AuditLogTimeFormat = "2006-01-02.15:04:05"

	// PlaybackRecycleTTL is the TTL for unpacked session playback files
	PlaybackRecycleTTL = 3 * time.Hour

	// WaitCopyTimeout is how long Teleport will wait for a session to finish
	// copying data from the PTY after "exit-status" has been received.
	WaitCopyTimeout = 5 * time.Second

	// ClientCacheSize is the size of the RPC clients expiring cache
	ClientCacheSize = 1024

	// CSRSignTimeout is a default timeout for CSR request to be processed by K8s
	CSRSignTimeout = 30 * time.Second

	// Localhost is the address of localhost. Used for the default binding
	// address for port forwarding.
	Localhost = "127.0.0.1"

	// AnyAddress is used to refer to the non-routable meta-address used to
	// refer to all addresses on the machine.
	AnyAddress = "0.0.0.0"

	// CallbackTimeout is how long to wait for a response from SSO provider
	// before timeout.
	CallbackTimeout = 180 * time.Second

	// ConcurrentUploadsPerStream limits the amount of concurrent uploads
	// per stream
	ConcurrentUploadsPerStream = 1

	// InactivityFlushPeriod is a period of inactivity
	// that triggers upload of the data - flush.
	InactivityFlushPeriod = 5 * time.Minute

	// NodeJoinTokenTTL is when a token for nodes expires.
	NodeJoinTokenTTL = 4 * time.Hour

	// LockMaxStaleness is the maximum staleness for cached lock resources
	// to be deemed acceptable for strict locking mode.
	LockMaxStaleness = 5 * time.Minute

	// DefaultRedisUsername is a default username used by Redis when
	// no name is provided at connection time.
	DefaultRedisUsername = "default"

	// SessionTrackerTTL defines the default base ttl of a session tracker,
	// which may be extended with session activity.
	SessionTrackerTTL = time.Hour

	// AbandonedUploadPollingRate defines how often to check for
	// abandoned uploads which need to be completed.
	AbandonedUploadPollingRate = SessionTrackerTTL / 6
)

var (
	// ResyncInterval is how often tunnels are resynced.
	ResyncInterval = 5 * time.Second

	// AuthServersRefreshPeriod is a period for clients to refresh their
	// their stored list of auth servers
	AuthServersRefreshPeriod = 30 * time.Second

	// TerminalResizePeriod is how long tsh waits before updating the size of the
	// terminal window.
	TerminalResizePeriod = 2 * time.Second

	// SessionRefreshPeriod is how often session data is updated on the backend.
	// The web client polls this information about session to update the UI.
	//
	// TODO(klizhentas): All polling periods should go away once backend supports
	// events.
	SessionRefreshPeriod = 2 * time.Second

	// SessionIdlePeriod is the period of inactivity after which the
	// session will be considered idle
	SessionIdlePeriod = SessionRefreshPeriod * 10

	// NetworkBackoffDuration is a standard backoff on network requests
	// usually is slow, e.g. once in 30 seconds
	NetworkBackoffDuration = time.Second * 30

	// AuditBackoffTimeout is a time out before audit logger will
	// start losing events
	AuditBackoffTimeout = 5 * time.Second

	// NetworkRetryDuration is a standard retry on network requests
	// to retry quickly, e.g. once in one second
	NetworkRetryDuration = time.Second

	// FastAttempts is the initial amount of fast retry attempts
	// before switching to slow mode
	FastAttempts = 10

	// ReportingPeriod is a period for reports in logs
	ReportingPeriod = 5 * time.Minute

	// HighResPollingPeriod is a default high resolution polling period
	HighResPollingPeriod = 10 * time.Second

	// HeartbeatCheckPeriod is a period between heartbeat status checks
	HeartbeatCheckPeriod = 5 * time.Second

	// LowResPollingPeriod is a default low resolution polling period
	LowResPollingPeriod = 600 * time.Second

	// HighResReportingPeriod is a high resolution polling reporting
	// period used in services
	HighResReportingPeriod = 10 * time.Second

	// DiskAlertThreshold is the disk space alerting threshold.
	DiskAlertThreshold = 90

	// DiskAlertInterval is disk space check interval.
	DiskAlertInterval = 5 * time.Minute

	// TopRequestsCapacity sets up default top requests capacity
	TopRequestsCapacity = 128

	// AuthQueueSize is auth service queue size
	AuthQueueSize = 8192

	// ProxyQueueSize is proxy service queue size
	ProxyQueueSize = 8192

	// NodeQueueSize is node service queue size
	NodeQueueSize = 128

	// KubernetesQueueSize is kubernetes service watch queue size
	KubernetesQueueSize = 128

	// AppsQueueSize is apps service queue size.
	AppsQueueSize = 128

	// DatabasesQueueSize is db service queue size.
	DatabasesQueueSize = 128

	// WindowsDesktopQueueSize is windows_desktop service watch queue size.
	WindowsDesktopQueueSize = 128

	// CASignatureAlgorithm is the default signing algorithm to use when
	// creating new SSH CAs.
	CASignatureAlgorithm = ssh.SigAlgoRSASHA2512

	// SessionControlTimeout is the maximum amount of time a controlled session
	// may persist after contact with the auth server is lost (sessctl semaphore
	// leases are refreshed at a rate of ~1/2 this duration).
	SessionControlTimeout = time.Minute * 2

	// SPDYPingPeriod is the period for sending out SPDY ping frames on inbound
	// and outbound connections. SPDY is used for interactive Kubernetes
	// connections. These pings are needed to avoid timeouts on load balancers
	// that don't respect TCP keep-alives.
	SPDYPingPeriod = 30 * time.Second

	// AsyncBufferSize is a default buffer size for async emitters
	AsyncBufferSize = 1024

	// ConnectionErrorMeasurementPeriod is the maximum age of a connection error
	// to be considered when deciding to restart the process. The process will
	// restart if there has been more than `MaxConnectionErrorsBeforeRestart`
	// errors in the preceding `ConnectionErrorMeasurementPeriod`
	ConnectionErrorMeasurementPeriod = 2 * time.Minute

	// MaxConnectionErrorsBeforeRestart is the number or allowable network errors
	// in the previous `ConnectionErrorMeasurementPeriod`. The process will
	// restart if there has been more than `MaxConnectionErrorsBeforeRestart`
	// errors in the preceding `ConnectionErrorMeasurementPeriod`
	MaxConnectionErrorsBeforeRestart = 5

	// MaxWatcherBackoff is the maximum retry time a watcher should use in
	// the event of connection issues
	MaxWatcherBackoff = time.Minute

	// PrometheusScrapeInterval is the default time interval for prometheus scrapes. Used for metric update periods.
	PrometheusScrapeInterval = 15 * time.Second
)

// Default connection limits, they can be applied separately on any of the Teleport
// services (SSH, auth, proxy)
const (
	// LimiterMaxConnections Number of max. simultaneous connections to a service
	LimiterMaxConnections = 15000

	// LimiterMaxConcurrentUsers Number of max. simultaneous connected users/logins
	LimiterMaxConcurrentUsers = 250

	// LimiterMaxConcurrentSignatures limits maximum number of concurrently
	// generated signatures by the auth server
	LimiterMaxConcurrentSignatures = 10
)

// Default rate limits for unauthenticated passwordless endpoints.
const (
	// LimiterPasswordlessPeriod is the default period for passwordless limiters.
	LimiterPasswordlessPeriod = 1 * time.Minute
	// LimiterPasswordlessAverage is the default average for passwordless
	// limiters.
	LimiterPasswordlessAverage = 10
	// LimiterPasswordlessBurst is the default burst for passwordless limiters.
	LimiterPasswordlessBurst = 20
)

const (
	// HostCertCacheSize is the number of host certificates to cache at any moment.
	HostCertCacheSize = 4000

	// HostCertCacheTime is how long a certificate stays in the cache.
	HostCertCacheTime = 24 * time.Hour
)

const (
	// MinCertDuration specifies minimum duration of validity of issued certificate
	MinCertDuration = time.Minute

	// RotationGracePeriod is a default rotation period for graceful
	// certificate rotations, by default to set to maximum allowed user
	// cert duration
	RotationGracePeriod = defaults.MaxCertDuration

	// PendingAccessDuration defines the expiry of a pending access request.
	PendingAccessDuration = time.Hour

	// MaxAccessDuration defines the maximum time for which an access request
	// can be active.
	MaxAccessDuration = defaults.MaxCertDuration
)

// list of roles teleport service can run as:
const (
	// RoleNode is SSH stateless node
	RoleNode = "node"
	// RoleProxy is a stateless SSH access proxy (bastion)
	RoleProxy = "proxy"
	// RoleAuthService is authentication and authorization service,
	// the only stateful role in the system
	RoleAuthService = "auth"
	// RoleApp is an application proxy.
	RoleApp = "app"
	// RoleDatabase is a database proxy role.
	RoleDatabase = "db"
	// RoleWindowsDesktop is a Windows desktop service.
	RoleWindowsDesktop = "windowsdesktop"
)

const (
	// ProtocolPostgres is the PostgreSQL database protocol.
	ProtocolPostgres = "postgres"
	// ProtocolMySQL is the MySQL/MariaDB database protocol.
	ProtocolMySQL = "mysql"
	// ProtocolMongoDB is the MongoDB database protocol.
	ProtocolMongoDB = "mongodb"
	// ProtocolRedis is the Redis database protocol.
	ProtocolRedis = "redis"
	// ProtocolCockroachDB is the CockroachDB database protocol.
	//
	// Technically it's the same as the Postgres protocol, but it's used to
	// differentiate between Cockroach and Postgres databases e.g. when
	// selecting a CLI client to use.
	ProtocolCockroachDB = "cockroachdb"
	// ProtocolSQLServer is the Microsoft SQL Server database protocol.
	ProtocolSQLServer = "sqlserver"
)

// DatabaseProtocols is a list of all supported database protocols.
var DatabaseProtocols = []string{
	ProtocolPostgres,
	ProtocolMySQL,
	ProtocolMongoDB,
	ProtocolCockroachDB,
	ProtocolRedis,
	ProtocolSQLServer,
}

const (
	// PerfBufferPageCount is the size of the perf ring buffer in number of pages.
	// Must be power of 2.
	PerfBufferPageCount = 8

	// OpenPerfBufferPageCount is the page count for the perf buffer. Open
	// events generate many events so this buffer needs to be extra large.
	// Must be power of 2.
	OpenPerfBufferPageCount = 128

	// CgroupPath is where the cgroupv2 hierarchy will be mounted.
	CgroupPath = "/cgroup2"

	// ArgsCacheSize is the number of args events to store before dropping args
	// events.
	ArgsCacheSize = 1024
)

var (
	// ConfigFilePath is default path to teleport config file
	ConfigFilePath = "/etc/teleport.yaml"

	// DataDir is where all mutable data is stored (user keys, recorded sessions,
	// registered SSH servers, etc):
	DataDir = "/var/lib/teleport"

	// StartRoles is default roles teleport assumes when started via 'start' command
	StartRoles = []string{RoleProxy, RoleNode, RoleAuthService, RoleApp, RoleDatabase}

	// ETCDPrefix is default key in ETCD clustered configurations
	ETCDPrefix = "/teleport"

	// ConfigEnvar is a name of teleport's configuration environment variable
	ConfigEnvar = "TELEPORT_CONFIG"

	// ConfigFileEnvar is the name of the environment variable used to specify a path to
	// the Teleport configuration file that tctl reads on use
	ConfigFileEnvar = "TELEPORT_CONFIG_FILE"

	// LicenseFile is the default name of the license file
	LicenseFile = "license.pem"

	// CACertFile is the default name of the certificate authority file to watch
	CACertFile = "ca.cert"

	// Krb5FilePath is the default location of Kerberos configuration file.
	Krb5FilePath = "/etc/krb5.conf"
)

const (
	// ServiceName is the default PAM policy to use if one is not passed in
	// configuration.
	ServiceName = "sshd"
)

const (
	initError = "failure initializing default values"
)

const (
	// WebauthnChallengeTimeout is the timeout for ongoing Webauthn authentication
	// or registration challenges.
	WebauthnChallengeTimeout = 5 * time.Minute
	// WebauthnGlobalChallengeTimeout is the timeout for global authentication
	// challenges.
	// Stricter than WebauthnChallengeTimeout because global challenges are
	// anonymous.
	WebauthnGlobalChallengeTimeout = 1 * time.Minute
)

const (
	// LookaheadBufSize is a reasonable buffer size for decoders that need
	// to buffer for the purposes of lookahead (e.g. `YAMLOrJSONDecoder`).
	LookaheadBufSize = 32 * 1024
)

// TLS constants for Web Proxy HTTPS connection
const (
	// path to a self-signed TLS PRIVATE key file for HTTPS connection for the web proxy
	SelfSignedKeyPath = "webproxy_key.pem"
	// path to a self-signed TLS PUBLIC key file for HTTPS connection for the web proxy
	SelfSignedPubPath = "webproxy_pub.pem"
	// path to a self-signed TLS cert file for HTTPS connection for the web proxy
	SelfSignedCertPath = "webproxy_cert.pem"
)

// ConfigureLimiter assigns the default parameters to a connection throttler (AKA limiter)
func ConfigureLimiter(lc *limiter.Config) {
	lc.MaxConnections = LimiterMaxConnections
	lc.MaxNumberOfUsers = LimiterMaxConcurrentUsers
}

// AuthListenAddr returns the default listening address for the Auth service
func AuthListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, AuthListenPort)
}

// AuthConnectAddr returns the default address to search for auth. service on
func AuthConnectAddr() *utils.NetAddr {
	return makeAddr("127.0.0.1", AuthListenPort)
}

// ProxyListenAddr returns the default listening address for the SSH Proxy service
func ProxyListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, SSHProxyListenPort)
}

// KubeProxyListenAddr returns the default listening address for the Kubernetes Proxy service
func KubeProxyListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, KubeListenPort)
}

// ProxyWebListenAddr returns the default listening address for the Web-based SSH Proxy service
func ProxyWebListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, HTTPListenPort)
}

// SSHServerListenAddr returns the default listening address for the Web-based SSH Proxy service
func SSHServerListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, SSHServerListenPort)
}

// ReverseTunnelListenAddr returns the default listening address for the SSH Proxy service used
// by the SSH nodes to establish proxy<->ssh_node connection from behind a firewall which
// blocks inbound connecions to ssh_nodes
func ReverseTunnelListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, SSHProxyTunnelListenPort)
}

// MetricsServiceListenAddr returns the default listening address for the metrics service
func MetricsServiceListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, MetricsListenPort)
}

func makeAddr(host string, port int16) *utils.NetAddr {
	addrSpec := fmt.Sprintf("tcp://%s:%d", host, port)
	retval, err := utils.ParseAddr(addrSpec)
	if err != nil {
		panic(fmt.Sprintf("%s: error parsing '%v'", initError, addrSpec))
	}
	return retval
}

// CATTL is a default lifetime of a CA certificate
const CATTL = time.Hour * 24 * 365 * 10

const (
	// WebsocketVersion is the version of the protocol.
	WebsocketVersion = "1"

	// WebsocketClose is sent when the SSH session is over without any errors.
	WebsocketClose = "c"

	// WebsocketAudit is sending a audit event over the websocket to the web client.
	WebsocketAudit = "a"

	// WebsocketRaw is sending raw terminal bytes over the websocket to the web
	// client.
	WebsocketRaw = "r"

	// WebsocketResize is receiving a resize request.
	WebsocketResize = "w"

	// WebsocketWebauthnChallenge is sending a webauthn challenge.
	WebsocketWebauthnChallenge = "n"
)

// The following are cryptographic primitives Teleport does not support in
// it's default configuration.
const (
	DiffieHellmanGroup14SHA1 = "diffie-hellman-group14-sha1"
	DiffieHellmanGroup1SHA1  = "diffie-hellman-group1-sha1"
	HMACSHA1                 = "hmac-sha1"
	HMACSHA196               = "hmac-sha1-96"
)

const (
	// ApplicationTokenKeyType is the type of asymmetric key used to sign tokens.
	// See https://tools.ietf.org/html/rfc7518#section-6.1 for possible values.
	ApplicationTokenKeyType = "RSA"
	// ApplicationTokenAlgorithm is the default algorithm used to sign
	// application access tokens.
	ApplicationTokenAlgorithm = jose.RS256
)

// WindowsOpenSSHNamedPipe is the address of the named pipe that the
// OpenSSH agent is on.
const WindowsOpenSSHNamedPipe = `\\.\pipe\openssh-ssh-agent`

var (
	// FIPSCipherSuites is a list of supported FIPS compliant TLS cipher suites.
	FIPSCipherSuites = []uint16{
		//
		// These two ciper suites:
		//
		// tls.TLS_RSA_WITH_AES_128_GCM_SHA256
		// tls.TLS_RSA_WITH_AES_256_GCM_SHA384
		//
		// although supported by FIPS, are blacklisted in http2 spec:
		//
		// https://tools.ietf.org/html/rfc7540#appendix-A
		//
		// therefore we do not include them in this list.
		//
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	}

	// FIPSCiphers is a list of supported FIPS compliant SSH ciphers.
	FIPSCiphers = []string{
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
		"aes128-gcm@openssh.com",
	}

	// FIPSKEXAlgorithms is a list of supported FIPS compliant SSH kex algorithms.
	FIPSKEXAlgorithms = []string{
		"ecdh-sha2-nistp256",
		"ecdh-sha2-nistp384",
		"echd-sha2-nistp521",
	}

	// FIPSMACAlgorithms is a list of supported FIPS compliant SSH mac algorithms.
	FIPSMACAlgorithms = []string{
		"hmac-sha2-256-etm@openssh.com",
		"hmac-sha2-256",
	}
)

// CheckPasswordLimiter creates a rate limit that can be used to slow down
// requests that come to the check password endpoint.
func CheckPasswordLimiter() *limiter.Limiter {
	limiter, err := limiter.NewLimiter(limiter.Config{
		MaxConnections:   LimiterMaxConnections,
		MaxNumberOfUsers: LimiterMaxConcurrentUsers,
		Rates: []limiter.Rate{
			limiter.Rate{
				Period:  1 * time.Second,
				Average: 10,
				Burst:   10,
			},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create limiter: %v.", err))
	}
	return limiter
}

// Transport returns a new http.RoundTripper with sensible defaults.
func Transport() (*http.Transport, error) {
	// Clone the default transport to pick up sensible defaults.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, trace.BadParameter("invalid transport type %T", http.DefaultTransport)
	}
	tr := defaultTransport.Clone()

	// Increase the size of the transport's connection pool. This substantially
	// improves the performance of Teleport under load as it reduces the number
	// of TLS handshakes performed.
	tr.MaxIdleConns = HTTPMaxIdleConns
	tr.MaxIdleConnsPerHost = HTTPMaxIdleConnsPerHost

	// Set IdleConnTimeout on the transport. This defines the maximum amount of
	// time before idle connections are closed. Leaving this unset will lead to
	// connections open forever and will cause memory leaks in a long running
	// process.
	tr.IdleConnTimeout = HTTPIdleTimeout

	return tr, nil
}

const (
	// TeleportConfigVersionV1 is the teleport proxy configuration v1 version.
	TeleportConfigVersionV1 string = "v1"
	// TeleportConfigVersionV2 is the teleport proxy configuration v2 version.
	TeleportConfigVersionV2 string = "v2"
)
