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

// Package defaults contains default constants set in various parts of
// teleport codebase
package defaults

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
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

	// ProxyPeeringListenPort is the default port proxies will listen on when
	// proxy peering is enabled.
	ProxyPeeringListenPort = 3021

	// RDPListenPort is the standard port for RDP servers.
	RDPListenPort = 3389

	// BackendDir is a default backend subdirectory
	BackendDir = "backend"

	// BackendPath is a default backend path parameter
	BackendPath = "path"

	// By default SSH server (and SSH proxy) will bind to this IP
	BindIP = "0.0.0.0"

	// GRPCMaxConcurrentStreams is the max GRPC streams that can be active at a time.  Once the limit is reached new
	// RPC calls will queue until capacity is available.
	GRPCMaxConcurrentStreams = 1000

	// HTTPMaxIdleConns is the max idle connections across all hosts.
	HTTPMaxIdleConns = 2000

	// HTTPMaxIdleConnsPerHost is the max idle connections per-host.
	HTTPMaxIdleConnsPerHost = 1000

	// HTTPMaxConnsPerHost is the maximum number of connections per-host.
	HTTPMaxConnsPerHost = 250

	// HTTPIdleTimeout is a default timeout for idle HTTP connections
	HTTPIdleTimeout = 30 * time.Second

	// HTTPRequestTimeout is a default timeout for HTTP requests
	HTTPRequestTimeout = 30 * time.Second

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
	ReadHeadersTimeout = 10 * time.Second

	// DatabaseConnectTimeout is a timeout for connecting to a database via
	// database access.
	DatabaseConnectTimeout = time.Minute

	// HandshakeReadDeadline is the default time to wait for the client during
	// the TLS handshake.
	HandshakeReadDeadline = 15 * time.Second

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

	// BearerTokenTTL specifies standard bearer token to exist before
	// it has to be renewed by the client
	BearerTokenTTL = 10 * time.Minute

	// TokenLenBytes is len in bytes of the invite token
	TokenLenBytes = 16

	// RecoveryTokenLenBytes is len in bytes of a user token for recovery.
	RecoveryTokenLenBytes = 32

	// SessionTokenBytes is the number of bytes of a web or application session.
	SessionTokenBytes = 32

	// ProvisioningTokenTTL is a the default TTL for server provisioning
	// tokens. When a user generates a token without an explicit TTL, this
	// value is used.
	ProvisioningTokenTTL = 30 * time.Minute

	// MinPasswordLength is minimum password length.
	// PCI DSS v4.0 control 8.3.6 requires a minimum password length of 12 characters.
	// NIST SP 800-63B section 5.1.1.1 requires a minimum password length of 8 characters.
	MinPasswordLength = 12

	// MaxPasswordLength is maximum password length (for sanity)
	MaxPasswordLength = 128

	// MaxIterationLimit is max iteration limit
	MaxIterationLimit = 1000

	// EventsIterationLimit is a default limit if it's not set for events
	EventsIterationLimit = 500

	// EventsIterationLimit is max iteration limit for events
	EventsMaxIterationLimit = 10000

	// ActiveSessionTTL is a TTL when session is marked as inactive
	ActiveSessionTTL = 30 * time.Second

	// OIDCAuthRequestTTL is TTL of internally stored auth request created by client
	OIDCAuthRequestTTL = 10 * 60 * time.Second

	// SAMLAuthRequestTTL is TTL of internally stored auth request created by client
	SAMLAuthRequestTTL = 10 * 60 * time.Second

	// GithubAuthRequestTTL is TTL of internally stored Github auth request
	GithubAuthRequestTTL = 10 * 60 * time.Second

	// LogRotationPeriod defines how frequently to rotate the audit log file
	LogRotationPeriod = time.Hour * 24

	// UploaderScanPeriod is a default uploader scan period
	UploaderScanPeriod = 5 * time.Second

	// UploaderConcurrentUploads is a default number of concurrent
	UploaderConcurrentUploads = 10

	// MaxLoginAttempts sets the max. number of allowed failed login attempts
	// before a user account is locked for AccountLockInterval
	MaxLoginAttempts int = 5

	// AccountLockInterval defines a time interval during which a user account
	// is locked after MaxLoginAttempts.
	// PCI DSS v4.0 control 8.3.4 requires a minimum lockout duration of 30 minutes.
	AccountLockInterval = 30 * time.Minute

	// AttemptTTL is TTL for login attempt
	AttemptTTL = time.Minute * 30

	// AuditLogTimeFormat is the format for the timestamp on audit log files.
	AuditLogTimeFormat = "2006-01-02.15:04:05"

	// PlaybackRecycleTTL is the TTL for unpacked session playback files
	PlaybackRecycleTTL = 3 * time.Hour

	// WaitCopyTimeout is how long Teleport will wait for a session to finish
	// copying data from the PTY after "exit-status" has been received.
	WaitCopyTimeout = 5 * time.Second

	// ClientCacheSize is the size of the RPC clients expiring cache
	ClientCacheSize = 1024

	// Localhost is the address of localhost. Used for the default binding
	// address for port forwarding.
	Localhost = "127.0.0.1"

	// AnyAddress is used to refer to the non-routable meta-address used to
	// refer to all addresses on the machine.
	AnyAddress = "0.0.0.0"

	// SSOCallbackTimeout is how long to wait for a response from SSO provider
	// before timeout.
	SSOCallbackTimeout = 180 * time.Second

	// HeadlessLoginTimeout is how long to wait for user to approve/reject headless login request.
	HeadlessLoginTimeout = SSOCallbackTimeout

	// NodeJoinTokenTTL is when a token for nodes expires.
	NodeJoinTokenTTL = 4 * time.Hour

	// LockMaxStaleness is the maximum staleness for cached lock resources
	// to be deemed acceptable for strict locking mode.
	LockMaxStaleness = 5 * time.Minute

	// DefaultRedisUsername is a default username used by Redis when
	// no name is provided at connection time.
	DefaultRedisUsername = "default"

	// ProxyPingInterval is the interval ping messages are going to be sent.
	// This is only applicable for TLS routing protocols that support ping
	// wrapping.
	ProxyPingInterval = 30 * time.Second
)

const (
	// TerminalResizePeriod is how long tsh waits before updating the size of the
	// terminal window.
	TerminalResizePeriod = 2 * time.Second

	// SessionIdlePeriod is the period of inactivity after which the
	// session will be considered idle
	SessionIdlePeriod = 20 * time.Second

	// HighResPollingPeriod is a default high resolution polling period
	HighResPollingPeriod = 10 * time.Second

	// LowResPollingPeriod is a default low resolution polling period
	LowResPollingPeriod = 600 * time.Second

	// SessionControlTimeout is the maximum amount of time a controlled session
	// may persist after contact with the auth server is lost (sessctl semaphore
	// leases are refreshed at a rate of ~1/2 this duration).
	SessionControlTimeout = time.Minute * 2

	// PrometheusScrapeInterval is the default time interval for prometheus scrapes. Used for metric update periods.
	PrometheusScrapeInterval = 15 * time.Second

	// MaxWatcherBackoff is the maximum retry time a watcher should use in
	// the event of connection issues
	MaxWatcherBackoff = 90 * time.Second

	// MaxLongWatcherBackoff is the maximum backoff used for watchers that incur high cluster-level
	// load (non-control-plane caches being the primary example).
	MaxLongWatcherBackoff = 256 * time.Second
)

const (
	// AuthQueueSize is auth service queue size
	AuthQueueSize = 8192

	// ProxyQueueSize is proxy service queue size
	ProxyQueueSize = 8192

	// UnifiedResourcesQueueSize is the unified resource watcher queue size
	UnifiedResourcesQueueSize = 8192

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

	// DiscoveryQueueSize is discovery service queue size.
	DiscoveryQueueSize = 128
)

var (
	// ResyncInterval is how often tunnels are resynced.
	ResyncInterval = 5 * time.Second

	// HeartbeatCheckPeriod is a period between heartbeat status checks
	HeartbeatCheckPeriod = 5 * time.Second
)

// Default connection limits, they can be applied separately on any of the Teleport
// services (SSH, auth, proxy)
const (
	// LimiterMaxConnections Number of max. simultaneous connections to a service
	LimiterMaxConnections = 15000

	// LimiterMaxConcurrentSignatures limits maximum number of concurrently
	// generated signatures by the auth server
	LimiterMaxConcurrentSignatures = 10
)

// Default rate limits for unauthenticated endpoints.
const (
	// LimiterPeriod is the default period for unauthenticated limiters.
	LimiterPeriod = 1 * time.Minute
	// LimiterAverage is the default average for unauthenticated limiters.
	LimiterAverage = 20
	// LimiterBurst is the default burst for unauthenticated limiters.
	LimiterBurst = 40
)

// Default high rate limits for unauthenticated endpoints that are CPU constrained.
const (
	// LimiterHighPeriod is the default period for high rate unauthenticated limiters.
	LimiterHighPeriod = 1 * time.Minute
	// LimiterHighAverage is the default average for high rate unauthenticated limiters.
	LimiterHighAverage = 120
	// LimiterHighBurst is the default burst for high rate unauthenticated limiters.
	LimiterHighBurst = 480
)

const (
	// HostCertCacheSize is the number of host certificates to cache at any moment.
	HostCertCacheSize = 4000

	// HostCertCacheTime is how long a certificate stays in the cache.
	HostCertCacheTime = 24 * time.Hour
)

const (
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
	// RoleDiscovery is a discovery service
	RoleDiscovery = "discovery"
)

const (
	// ProtocolPostgres is the PostgreSQL database protocol.
	ProtocolPostgres = "postgres"
	// ProtocolMySQL is the MySQL/MariaDB database protocol.
	ProtocolMySQL = "mysql"
	// ProtocolMongoDB is the MongoDB database protocol.
	ProtocolMongoDB = "mongodb"
	// ProtocolOracle is the Oracle database protocol.
	ProtocolOracle = "oracle"
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
	// ProtocolSnowflake is the Snowflake REST database protocol.
	ProtocolSnowflake = "snowflake"
	// ProtocolCassandra is the Cassandra database protocol.
	ProtocolCassandra = "cassandra"
	// ProtocolElasticsearch is the Elasticsearch database protocol.
	ProtocolElasticsearch = "elasticsearch"
	// ProtocolOpenSearch is the OpenSearch database protocol.
	ProtocolOpenSearch = "opensearch"
	// ProtocolDynamoDB is the DynamoDB database protocol.
	ProtocolDynamoDB = "dynamodb"
	// ProtocolClickHouse is the ClickHouse database native write protocol.
	// (https://clickhouse.com/docs/en/interfaces/tcp)
	ProtocolClickHouse = "clickhouse"
	// ProtocolClickHouseHTTP is the ClickHouse database HTTP protocol.
	ProtocolClickHouseHTTP = "clickhouse-http"
	// ProtocolSpanner is the GCP Spanner database protocol.
	ProtocolSpanner = "spanner"
)

// DatabaseProtocols is a list of all supported database protocols.
var DatabaseProtocols = []string{
	ProtocolPostgres,
	ProtocolMySQL,
	ProtocolMongoDB,
	ProtocolOracle,
	ProtocolCockroachDB,
	ProtocolRedis,
	ProtocolSnowflake,
	ProtocolSQLServer,
	ProtocolCassandra,
	ProtocolElasticsearch,
	ProtocolOpenSearch,
	ProtocolDynamoDB,
	ProtocolClickHouse,
	ProtocolClickHouseHTTP,
	ProtocolSpanner,
}

// ReadableDatabaseProtocol returns a more human-readable string of the
// provided database protocol.
func ReadableDatabaseProtocol(p string) string {
	switch p {
	case ProtocolPostgres:
		return "PostgreSQL"
	case ProtocolMySQL:
		return "MySQL"
	case ProtocolMongoDB:
		return "MongoDB"
	case ProtocolOracle:
		return "Oracle"
	case ProtocolCockroachDB:
		return "CockroachDB"
	case ProtocolRedis:
		return "Redis"
	case ProtocolSnowflake:
		return "Snowflake"
	case ProtocolElasticsearch:
		return "Elasticsearch"
	case ProtocolOpenSearch:
		return "OpenSearch"
	case ProtocolSQLServer:
		return "Microsoft SQL Server"
	case ProtocolCassandra:
		return "Cassandra"
	case ProtocolDynamoDB:
		return "DynamoDB"
	case ProtocolClickHouse:
		return "Clickhouse"
	case ProtocolClickHouseHTTP:
		return "Clickhouse (HTTP)"
	case ProtocolSpanner:
		return "GCloud Spanner"
	default:
		// Unknown protocol. Return original string.
		return p
	}
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
)

const (
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

var (
	// ConfigFilePath is default path to teleport config file
	ConfigFilePath = "/etc/teleport.yaml"

	// DataDir is where all mutable data is stored (user keys, recorded sessions,
	// registered SSH servers, etc):
	DataDir = "/var/lib/teleport"

	// StartRoles is default roles teleport assumes when started via 'start' command
	StartRoles = []string{RoleProxy, RoleNode, RoleAuthService, RoleApp, RoleDatabase}
)

const (
	// PAMServiceName is the default PAM policy to use if one is not passed in
	// configuration.
	PAMServiceName = "sshd"
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

const (
	// SnowflakeURL is the Snowflake URL used for address validation.
	SnowflakeURL = "snowflakecomputing.com"
)

// ConfigureLimiter assigns the default parameters to a connection throttler (AKA limiter)
func ConfigureLimiter(lc *limiter.Config) {
	lc.MaxConnections = LimiterMaxConnections
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

func ProxyPeeringListenAddr() *utils.NetAddr {
	return makeAddr(BindIP, ProxyPeeringListenPort)
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

	// WebsocketFileTransferRequest is received when a new file transfer has been requested
	WebsocketFileTransferRequest = "f"

	// WebsocketFileTransferDecision is received when a response (approve/deny) has been
	// made for an existing file transfer request
	WebsocketFileTransferDecision = "t"

	WebsocketReadyToJoin = "j"

	// WebsocketMFAChallenge is sending an MFA challenge. Only supports WebAuthn and SSO MFA.
	WebsocketMFAChallenge = "n"

	// WebsocketSessionMetadata is sending the data for a ssh session.
	WebsocketSessionMetadata = "s"

	WebsocketSessionStatus = "g"

	// WebsocketError is sending an error message.
	WebsocketError = "e"

	// WebsocketLatency provides latency information for a session.
	WebsocketLatency = "l"

	// WebsocketKubeExec provides latency information for a session.
	WebsocketKubeExec = "k"

	// WebsocketDatabaseSessionRequest is received when a new database session
	// is requested.
	WebsocketDatabaseSessionRequest = "d"

	WebSocketChatMessageRequest = "h"
)

// The following are cryptographic primitives Teleport does not support in
// its default configuration.
const (
	DiffieHellmanGroup14SHA1 = "diffie-hellman-group14-sha1"
	DiffieHellmanGroup1SHA1  = "diffie-hellman-group1-sha1"
	HMACSHA1                 = "hmac-sha1"
	HMACSHA196               = "hmac-sha1-96"
)

const (
	// JWTUse is the default usage of the JWT.
	// See https://www.rfc-editor.org/rfc/rfc7517#section-4.2 for more information.
	JWTUse = "sig"
)

var (
	// FIPSCipherSuites is a list of supported FIPS compliant TLS cipher suites (for TLS 1.2 only).
	// Order will dictate the selected cipher, as per RFC 5246 § 7.4.1.2.
	// See https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.2 for more information.
	// This aligns to `crypto/tls`'s `CipherSuites` `supportedOnlyTLS12` list, but
	// just constrained to only FIPS-approved ciphers supported by `crypto/tls`
	// and ordered based on `cipherSuitesPreferenceOrder`.
	FIPSCipherSuites = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	}

	// FIPSCiphers is a list of supported FIPS compliant SSH ciphers.
	// Order will dictate the selected cipher, as per RFC 4253 § 7.1.
	// See `encryption_algorithms` section of https://datatracker.ietf.org/doc/html/rfc4253#section-7.1.
	// This aligns to `x/crypto/ssh`'s `preferredCiphers`, but just constrained to
	// only FIPS-approved ciphers.
	// Can also be compared to OpenSSH's `KEX_SERVER_ENCRYPT` / `KEX_CLIENT_ENCRYPT`.
	FIPSCiphers = []string{
		"aes128-gcm@openssh.com",
		"aes256-gcm@openssh.com",
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
	}

	// FIPSKEXAlgorithms is a list of supported FIPS compliant SSH kex algorithms.
	// Order will dictate the selected algorithm, as per RFC 4253 § 7.1.
	// See `kex_algorithms` section of https://datatracker.ietf.org/doc/html/rfc4253#section-7.1.
	// This aligns to `x/crypto/ssh`'s `preferredKeyAlgos`, but just constrained to
	// only FIPS-approved algorithms.
	// Can also be compared to OpenSSH's `KEX_SERVER_KEX` / `KEX_CLIENT_KEX`.
	FIPSKEXAlgorithms = []string{
		"ecdh-sha2-nistp256",
		"ecdh-sha2-nistp384",
	}

	// FIPSMACAlgorithms is a list of supported FIPS compliant SSH mac algorithms.
	// Order will dictate the selected algorithm, as per RFC 4253 § 7.1.
	// See `mac_algorithms` section of https://datatracker.ietf.org/doc/html/rfc4253#section-7.1.
	// This aligns to `x/crypto/ssh`'s `preferredMACs`, but just constrained to
	// only FIPS-approved algorithms.
	// Can also be compared to OpenSSH's `KEX_SERVER_MAC` / `KEX_CLIENT_MAC`.
	FIPSMACAlgorithms = []string{
		"hmac-sha2-256-etm@openssh.com",
		"hmac-sha2-512-etm@openssh.com",
		"hmac-sha2-256",
		"hmac-sha2-512",
	}

	// FIPSPubKeyAuthAlgorithms is a list of supported FIPS compliant SSH public
	// key authentication algorithms.
	// Order will dictate the selected algorithm, as per RFC 4253 § 7.1.
	// See `server_host_key_algorithms` section of https://datatracker.ietf.org/doc/html/rfc4253#section-7.1.
	// This aligns to `x/crypto/ssh`'s `preferredPubKeyAuthAlgos`, but just
	// constrained to only FIPS-approved algorithms.
	// Can also be compared to OpenSSH's `KEX_DEFAULT_PK_ALG`.
	FIPSPubKeyAuthAlgorithms = []string{
		ssh.KeyAlgoECDSA256,
		ssh.KeyAlgoECDSA384,
		ssh.KeyAlgoRSASHA256,
		ssh.KeyAlgoRSASHA512,
	}
)

// HTTPClient returns a new http.Client with sensible defaults.
func HTTPClient() (*http.Client, error) {
	transport, err := Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &http.Client{
		Transport: transport,
	}, nil
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
	// connections open forever and will cause memory leaks in a long-running
	// process.
	tr.IdleConnTimeout = HTTPIdleTimeout

	return tr, nil
}

const (
	// TeleportConfigVersionV1 is the teleport proxy configuration v1 version.
	TeleportConfigVersionV1 string = "v1"
	// TeleportConfigVersionV2 is the teleport proxy configuration v2 version.
	TeleportConfigVersionV2 string = "v2"
	// TeleportConfigVersionV3 is the teleport proxy configuration v3 version.
	TeleportConfigVersionV3 string = "v3"
)

// TeleportConfigVersions is an exported slice of the allowed versions in the config file,
// for convenience (looping through, etc)
var TeleportConfigVersions = []string{
	TeleportConfigVersionV1,
	TeleportConfigVersionV2,
	TeleportConfigVersionV3,
}

func ValidateConfigVersion(version string) error {
	hasVersion := slices.Contains(TeleportConfigVersions, version)
	if !hasVersion {
		return trace.BadParameter("version must be one of %s", strings.Join(TeleportConfigVersions, ", "))
	}

	return nil
}

// Default values for tsh and tctl commands.
const (
	// Use more human readable format than RFC3339
	TshTctlSessionListTimeFormat = "2006-01-02"
	TshTctlSessionListLimit      = "50"
	TshTctlSessionDayLimit       = 365
)

// DefaultFormats is the default set of formats to use for commands that have the --format flag.
var DefaultFormats = []string{teleport.Text, teleport.JSON, teleport.YAML}

// FormatFlagDescription creates the description for the --format flag.
func FormatFlagDescription(formats ...string) string {
	return fmt.Sprintf("Format output (%s)", strings.Join(formats, ", "))
}

func SearchSessionRange(clock clockwork.Clock, fromUTC, toUTC, recordingsSince string) (from time.Time, to time.Time, err error) {
	if (fromUTC != "" || toUTC != "") && recordingsSince != "" {
		return time.Time{}, time.Time{},
			trace.BadParameter("use of 'since' is mutually exclusive with 'from-utc' and 'to-utc' flags")
	}
	from = clock.Now().Add(time.Hour * -24)
	to = clock.Now()
	if fromUTC != "" {
		from, err = time.Parse(TshTctlSessionListTimeFormat, fromUTC)
		if err != nil {
			return time.Time{}, time.Time{},
				trace.BadParameter("failed to parse session recording listing start time: expected format %s, got %s.", TshTctlSessionListTimeFormat, fromUTC)
		}
	}
	if toUTC != "" {
		to, err = time.Parse(TshTctlSessionListTimeFormat, toUTC)
		if err != nil {
			return time.Time{}, time.Time{},
				trace.BadParameter("failed to parse session recording listing end time: expected format %s, got %s.", TshTctlSessionListTimeFormat, toUTC)
		}
	}
	if recordingsSince != "" {
		since, err := time.ParseDuration(recordingsSince)
		if err != nil {
			return time.Time{}, time.Time{},
				trace.BadParameter("invalid duration provided to 'since': %s: expected format: '5h30m40s'", recordingsSince)
		}
		from = to.Add(-since)
	}

	if to.After(clock.Now()) {
		return time.Time{}, time.Time{},
			trace.BadParameter("invalid '--to-utc': '--to-utc' cannot be in the future")
	}
	if from.After(clock.Now()) {
		return time.Time{}, time.Time{},
			trace.BadParameter("invalid '--from-utc': '--from-utc' cannot be in the future")
	}
	if from.After(to) {
		return time.Time{}, time.Time{},
			trace.BadParameter("invalid '--from-utc' time: 'from' must be before '--to-utc'")
	}
	return from, to, nil
}

const (
	// HostnameLabel is the name of the label added to the sample SSH config generated by the teleport
	// node configure command.
	HostnameLabel = "hostname"
)

const (
	// FilePermissions are safe default permissions to use when
	// creating files.
	FilePermissions = 0o644
	// DirectoryPermissions are safe default permissions to use when
	// creating directories.
	DirectoryPermissions = 0o755
)
