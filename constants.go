package teleport

import (
	"strings"
	"time"
)

// WebAPIVersion is a current webapi version
const WebAPIVersion = "v1"

// ForeverTTL means that object TTL will not expire unless deleted
const ForeverTTL time.Duration = 0

const (
	// SSHAuthSock is the environment variable pointing to the
	// Unix socket the SSH agent is running on.
	SSHAuthSock = "SSH_AUTH_SOCK"
	// SSHAgentPID is the environment variable pointing to the agent
	// process ID
	SSHAgentPID = "SSH_AGENT_PID"

	// SSHTeleportUser is the current Teleport user that is logged in.
	SSHTeleportUser = "SSH_TELEPORT_USER"

	// SSHSessionWebproxyAddr is the address the web proxy.
	SSHSessionWebproxyAddr = "SSH_SESSION_WEBPROXY_ADDR"

	// SSHTeleportClusterName is the name of the cluster this node belongs to.
	SSHTeleportClusterName = "SSH_TELEPORT_CLUSTER_NAME"

	// SSHTeleportHostUUID is the UUID of the host.
	SSHTeleportHostUUID = "SSH_TELEPORT_HOST_UUID"

	// SSHSessionID is the UUID of the current session.
	SSHSessionID = "SSH_SESSION_ID"
)

const (
	// HTTPSProxy is an environment variable pointing to a HTTPS proxy.
	HTTPSProxy = "HTTPS_PROXY"

	// HTTPProxy is an environment variable pointing to a HTTP proxy.
	HTTPProxy = "HTTP_PROXY"
)

const (
	// TOTPValidityPeriod is the number of seconds a TOTP token is valid.
	TOTPValidityPeriod uint = 30

	// TOTPSkew adds that many periods before and after to the validity window.
	TOTPSkew uint = 1
)

const (
	// ComponentAuthority is a TLS and an SSH certificate authority
	ComponentAuthority = "ca"

	// ComponentProcess is a main control process
	ComponentProcess = "proc"

	// ComponentReverseTunnelServer is reverse tunnel server
	// that together with agent establish a bi-directional SSH revers tunnel
	// to bypass firewall restrictions
	ComponentReverseTunnelServer = "proxy:server"

	// ComponentReverseTunnel is reverse tunnel agent
	// that together with server establish a bi-directional SSH revers tunnel
	// to bypass firewall restrictions
	ComponentReverseTunnelAgent = "proxy:agent"

	// ComponentKube is a kubernetes proxy
	ComponentKube = "proxy:kube"

	// ComponentAuth is the cluster CA node (auth server API)
	ComponentAuth = "auth"

	// ComponentNode is SSH node (SSH server serving requests)
	ComponentNode = "node"

	// ComponentNode is SSH node (SSH server serving requests)
	ComponentForwardingNode = "node:forward"

	// ComponentProxy is SSH proxy (SSH server forwarding connections)
	ComponentProxy = "proxy"

	// ComponentDiagnostic is a diagnostic service
	ComponentDiagnostic = "diag"

	// ComponentClient is a client
	ComponentClient = "client"

	// ComponentTunClient is a tunnel client
	ComponentTunClient = "client:tunnel"

	// ComponentCachingClient is a caching auth client
	ComponentCachingClient = "client:cache"

	// ComponentSubsystemProxy is the proxy subsystem.
	ComponentSubsystemProxy = "subsystem:proxy"

	// ComponentLocalTerm is a terminal on a regular SSH node.
	ComponentLocalTerm = "term:local"

	// ComponentRemoteTerm is a terminal on a forwarding SSH node.
	ComponentRemoteTerm = "term:remote"

	// ComponentRemoteSubsystem is subsystem on a forwarding SSH node.
	ComponentRemoteSubsystem = "subsystem:remote"

	// ComponentAuditLog is audit log component
	ComponentAuditLog = "audit"

	// ComponentKeyAgent is an agent that has loaded the sessions keys and
	// certificates for a user connected to a proxy.
	ComponentKeyAgent = "keyagent"

	// ComponentKeyStore is all sessions keys and certificates a user has on disk
	// for all proxies.
	ComponentKeyStore = "keystore"

	// ComponentConnectProxy is the HTTP CONNECT proxy used to tunnel connection.
	ComponentConnectProxy = "http:proxy"

	// ComponentKeyGen is the public/private keypair generator.
	ComponentKeyGen = "keygen"

	// ComponentSession is an active session.
	ComponentSession = "session"

	// ComponentDynamoDB represents dynamodb clients
	ComponentDynamoDB = "dynamodb"

	// Component pluggable authentication module (PAM)
	ComponentPAM = "pam"

	// ComponentUpload is a session recording upload server
	ComponentUpload = "upload"

	// ComponentWebsocket is websocket server that the web client connects to.
	ComponentWebsocket = "websocket"

	// ComponentRBAC is role-based access control.
	ComponentRBAC = "rbac"

	// DebugEnvVar tells tests to use verbose debug output
	DebugEnvVar = "DEBUG"

	// VerboseLogEnvVar forces all logs to be verbose (down to DEBUG level)
	VerboseLogsEnvVar = "TELEPORT_DEBUG"

	// DefaultTerminalWidth defines the default width of a server-side allocated
	// pseudo TTY
	DefaultTerminalWidth = 80

	// DefaultTerminalHeight defines the default height of a server-side allocated
	// pseudo TTY
	DefaultTerminalHeight = 25

	// SafeTerminalType is the fall-back TTY type to fall back to (when $TERM
	// is not defined)
	SafeTerminalType = "xterm"

	// ConnectorOIDC means connector type OIDC
	ConnectorOIDC = "oidc"

	// ConnectorSAML means connector type SAML
	ConnectorSAML = "saml"

	// ConnectorGithub means connector type Github
	ConnectorGithub = "github"

	// DataDirParameterName is the name of the data dir configuration parameter passed
	// to all backends during initialization
	DataDirParameterName = "data_dir"

	// SSH request type to keep the connection alive. A client and a server keep
	// pining each other with it:
	KeepAliveReqType = "keepalive@openssh.com"

	// RecordingProxyReqType is the name of a global request which returns if
	// the proxy is recording sessions or not.
	RecordingProxyReqType = "recording-proxy@teleport.com"

	// OTP means One-time Password Algorithm for Two-Factor Authentication.
	OTP = "otp"

	// TOTP means Time-based One-time Password Algorithm. for Two-Factor Authentication.
	TOTP = "totp"

	// HOTP means HMAC-based One-time Password Algorithm.for Two-Factor Authentication.
	HOTP = "hotp"

	// U2F means Universal 2nd Factor.for Two-Factor Authentication.
	U2F = "u2f"

	// OFF means no second factor.for Two-Factor Authentication.
	OFF = "off"

	// Local means authentication will happen locally within the Teleport cluster.
	Local = "local"

	// OIDC means authentication will happen remotely using an OIDC connector.
	OIDC = ConnectorOIDC

	// SAML means authentication will happen remotely using a SAML connector.
	SAML = ConnectorSAML

	// Github means authentication will happen remotely using a Github connector.
	Github = ConnectorGithub

	// JSON means JSON serialization format
	JSON = "json"

	// LinuxAdminGID is the ID of the standard adm group on linux
	LinuxAdminGID = 4

	// LinuxOS is the GOOS constant used for Linux.
	LinuxOS = "linux"

	// WindowsOS is the GOOS constant used for Microsoft Windows.
	WindowsOS = "windows"

	// DarwinOS is the GOOS constant for Apple macOS/darwin.
	DarwinOS = "darwin"

	// DirMaskSharedGroup is the mask for a directory accessible
	// by the owner and group
	DirMaskSharedGroup = 0770

	// FileMaskOwnerOnly is the file mask that allows read write access
	// to owers only
	FileMaskOwnerOnly = 0600

	// On means mode is on
	On = "on"

	// Off means mode is off
	Off = "off"

	// SchemeS3 is S3 file scheme, means upload or download to S3 like object
	// storage
	SchemeS3 = "s3"

	// SchemeFile is a local disk file storage
	SchemeFile = "file"

	// LogsDir is a log subdirectory for events and logs
	LogsDir = "log"

	// Syslog is a mode for syslog logging
	Syslog = "syslog"

	// HumanDateFormat is a human readable date formatting
	HumanDateFormat = "Jan _2 15:04 UTC"

	// HumanDateFormatSeconds is a human readable date formatting with seconds
	HumanDateFormatSeconds = "Jan _2 15:04:05 UTC"

	// HumanDateFormatMilli is a human readable date formatting with milliseconds
	HumanDateFormatMilli = "Jan _2 15:04:05.000 UTC"
)

// Component generates "component:subcomponent1:subcomponent2" strings used
// in debugging
func Component(components ...string) string {
	return strings.Join(components, ":")
}

const (
	// AuthorizedKeys are public keys that check against User CAs.
	AuthorizedKeys = "authorized_keys"
	// KnownHosts are public keys that check against Host CAs.
	KnownHosts = "known_hosts"
)

const (
	// CertExtensionPermitAgentForwarding allows agent forwarding for certificate
	CertExtensionPermitAgentForwarding = "permit-agent-forwarding"
	// CertExtensionPermitPTY allows user to request PTY
	CertExtensionPermitPTY = "permit-pty"
	// CertExtensionPermitPortForwarding allows user to request port forwarding
	CertExtensionPermitPortForwarding = "permit-port-forwarding"
	// CertExtensionTeleportRoles is used to propagate teleport roles
	CertExtensionTeleportRoles = "teleport-roles"
)

const (
	// NetIQ is an identity provider.
	NetIQ = "netiq"
	// ADFS is Microsoft Active Directory Federation Services
	ADFS = "adfs"
)

const (
	// RemoteCommandSuccess is returned when a command has successfully executed.
	RemoteCommandSuccess = 0
	// RemoteCommandFailure is returned when a command has failed to execute and
	// we don't have another status code for it.
	RemoteCommandFailure = 255
)

// MaxEnvironmentFileLines is the maximum number of lines in a environment file.
const MaxEnvironmentFileLines = 1000

const (
	// CertificateFormatOldSSH is used to make Teleport interoperate with older
	// versions of OpenSSH.
	CertificateFormatOldSSH = "oldssh"

	// CertificateFormatStandard is used for normal Teleport operation without any
	// compatibility modes.
	CertificateFormatStandard = "standard"

	// CertificateFormatUnspecified is used to check if the format was specified
	// or not.
	CertificateFormatUnspecified = ""

	// DurationNever is human friendly shortcut that is interpreted as a Duration of 0
	DurationNever = "never"
)

const (
	// TraitInternalPrefix is the role variable prefix that indicates it's for
	// local accounts.
	TraitInternalPrefix = "internal"

	// TraitLogins is the name the role variable used to store
	// allowed logins.
	TraitLogins = "logins"

	// TraitKubeGroups is the name the role variable used to store
	// allowed kubernetes groups
	TraitKubeGroups = "kubernetes_groups"

	// TraitInternalLoginsVariable is the variable used to store allowed
	// logins for local accounts.
	TraitInternalLoginsVariable = "{{internal.logins}}"

	// TraitInternalKubeGroupsVariable is the variable used to store allowed
	// kubernetes groups for local accounts.
	TraitInternalKubeGroupsVariable = "{{internal.kubernetes_groups}}"
)

// SCP is Secure Copy.
const SCP = "scp"

// Root is *nix system administrator account name.
const Root = "root"

// DefaultRole is the name of the default admin role for all local users if
// another role is not explicitly assigned (Enterprise only).
const AdminRoleName = "admin"

// DefaultImplicitRole is implicit role that gets added to all service.RoleSet
// objects.
const DefaultImplicitRole = "default-implicit-role"

// APIDomain is a default domain name for Auth server API
const APIDomain = "teleport.cluster.local"

const (
	// RemoteClusterStatusOffline indicates that cluster is considered as
	// offline, since it has missed a series of heartbeats
	RemoteClusterStatusOffline = "offline"
	// RemoteClusterStatusOnline indicates that cluster is sending heartbeats
	// at expected interval
	RemoteClusterStatusOnline = "online"
)

const (
	// SharedDirMode is a mode for a directory shared with group
	SharedDirMode = 0750

	// PrivateDirMode is a mode for private directories
	PrivateDirMode = 0700
)

const (
	// SessionEvent is sent by servers to clients when an audit event occurs on
	// the session.
	SessionEvent = "x-teleport-event"
)

const (
	// EnvKubeConfig is environment variable for kubeconfig
	EnvKubeConfig = "KUBECONFIG"

	// KubeConfigDir is a default directory where k8s stores its user local config
	KubeConfigDir = ".kube"

	// KubeConfigFile is a default filename where k8s stores its user local config
	KubeConfigFile = "config"

	// EnvHome is home environment variable
	EnvHome = "HOME"

	// EnvUserProfile is the home directory environment variable on Windows.
	EnvUserProfile = "USERPROFILE"

	// KubeServiceAddr is an address for kubernetes endpoint service
	KubeServiceAddr = "kubernetes.default.svc.cluster.local:443"

	// KubeCAPath is a hardcode of mounted CA inside every pod of K8s
	KubeCAPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	// KubeKindCSR is a certificate signing requests
	KubeKindCSR = "CertificateSigningRequest"

	// KubeKindPod is a kubernetes pod
	KubeKindPod = "Pod"

	// KubeMetadataNameSelector is a selector for name metadata in API requests
	KubeMetadataNameSelector = "metadata.name"

	// KubeMetadataLabelSelector is a selector for label
	KubeMetadataLabelSelector = "metadata.label"

	// KubeRunTests turns on kubernetes tests
	KubeRunTests = "TEST_KUBE"

	// KubeSystemMasters is a name of the builtin kubernets group for master nodes
	KubeSystemMasters = "system:masters"

	// UsageKubeOnly specifies certificate usage metadata
	// that limits certificate to be only used for kubernetes proxying
	UsageKubeOnly = "usage:kube"
)

const (
	// UseOfClosedNetworkConnection is a special string some parts of
	// go standard lib are using that is the only way to identify some errors
	UseOfClosedNetworkConnection = "use of closed network connection"
)

const (
	// OpenBrowserLinux is the command used to open a web browser on Linux.
	OpenBrowserLinux = "sensible-browser"

	// OpenBrowserDarwin is the command used to open a web browser on macOS/Darwin.
	OpenBrowserDarwin = "open"

	// OpenBrowserWindows is the command used to open a web browser on Windows.
	OpenBrowserWindows = "rundll32.exe"
)
