/*
Copyright 2020-2021 Gravitational, Inc.

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

// Package constants defines Teleport-specific constants
package constants

const (
	// DefaultImplicitRole is implicit role that gets added to all service.RoleSet
	// objects.
	DefaultImplicitRole = "default-implicit-role"

	// APIDomain is a default domain name for Auth server API. It is often
	// used as an SNI to pass TLS handshakes regardless of the server address
	// since we register "teleport.cluster.local" as a DNS in Certificates.
	APIDomain = "teleport.cluster.local"

	// EnhancedRecordingMinKernel is the minimum kernel version for the enhanced
	// recording feature.
	EnhancedRecordingMinKernel = "5.8.0"

	// EnhancedRecordingCommand is a role option that implies command events are
	// captured.
	EnhancedRecordingCommand = "command"

	// EnhancedRecordingDisk is a role option that implies disk events are captured.
	EnhancedRecordingDisk = "disk"

	// EnhancedRecordingNetwork is a role option that implies network events
	// are captured.
	EnhancedRecordingNetwork = "network"

	// LocalConnector is the authenticator connector for local logins.
	LocalConnector = "local"

	// PasswordlessConnector is the authenticator connector for
	// local/passwordless logins.
	PasswordlessConnector = "passwordless"

	// Local means authentication will happen locally within the Teleport cluster.
	Local = "local"

	// OIDC means authentication will happen remotely using an OIDC connector.
	OIDC = "oidc"

	// SAML means authentication will happen remotely using a SAML connector.
	SAML = "saml"

	// Github means authentication will happen remotely using a Github connector.
	Github = "github"

	// Idemeum means authentcation will happen remotely using the Idemeum and it passes the identity using session jwt
	Idemeum = "idemeum"

	// HumanDateFormatSeconds is a human readable date formatting with seconds
	HumanDateFormatSeconds = "Jan _2 15:04:05 UTC"

	// MaxLeases serves as an identifying error string indicating that the
	// semaphore system is rejecting an acquisition attempt due to max
	// leases having already been reached.
	MaxLeases = "err-max-leases"

	// CertificateFormatStandard is used for normal Teleport operation without any
	// compatibility modes.
	CertificateFormatStandard = "standard"

	// DurationNever is human friendly shortcut that is interpreted as a Duration of 0
	DurationNever = "never"

	// OIDCPromptSelectAccount instructs the Authorization Server to
	// prompt the End-User to select a user account.
	OIDCPromptSelectAccount = "select_account"

	// OIDCPromptNone instructs the Authorization Server to skip the prompt.
	OIDCPromptNone = "none"

	// KeepAliveNode is the keep alive type for SSH servers.
	KeepAliveNode = "node"

	// KeepAliveApp is the keep alive type for application server.
	KeepAliveApp = "app"

	// KeepAliveDatabase is the keep alive type for database server.
	KeepAliveDatabase = "db"

	// KeepAliveWindowsDesktopService is the keep alive type for a Windows
	// desktop service.
	KeepAliveWindowsDesktopService = "windows_desktop_service"

	// KeepAliveKube is the keep alive type for Kubernetes server
	KeepAliveKube = "kube"

	// WindowsOS is the GOOS constant used for Microsoft Windows.
	WindowsOS = "windows"

	// LinuxOS is the GOOS constant used for Linux.
	LinuxOS = "linux"

	// DarwinOS is the GOOS constant for Apple macOS/darwin.
	DarwinOS = "darwin"

	// UseOfClosedNetworkConnection is a special string some parts of
	// go standard lib are using that is the only way to identify some errors
	//
	// TODO(r0mant): See if we can use net.ErrClosed and errors.Is() instead.
	UseOfClosedNetworkConnection = "use of closed network connection"

	// FailedToSendCloseNotify is an error message from Go net package
	// indicating that the connection was closed by the server.
	FailedToSendCloseNotify = "tls: failed to send closeNotify alert (but connection was closed anyway)"

	// AWSConsoleURL is the URL of AWS management console.
	AWSConsoleURL = "https://console.aws.amazon.com"
	// AWSAccountIDLabel is the key of the label containing AWS account ID.
	AWSAccountIDLabel = "aws_account_id"

	// RSAKeySize is the size of the RSA key.
	RSAKeySize = 2048

	// NoLoginPrefix is the prefix used for nologin certificate principals.
	NoLoginPrefix = "-teleport-nologin-"

	// DatabaseCAMinVersion is the minimum Teleport version that supports Database Certificate Authority.
	DatabaseCAMinVersion = "10.0.0"

	// SSHRSAType is the string which specifies an "ssh-rsa" formatted keypair
	SSHRSAType = "ssh-rsa"
)

// SystemConnectors lists the names of the system-reserved connectors.
var SystemConnectors = []string{
	LocalConnector,
	PasswordlessConnector,
}

// SecondFactorType is the type of 2FA authentication.
type SecondFactorType string

const (
	// SecondFactorOff means no second factor.
	SecondFactorOff = SecondFactorType("off")
	// SecondFactorOTP means that only OTP is supported for 2FA and 2FA is
	// required for all users.
	SecondFactorOTP = SecondFactorType("otp")
	// SecondFactorU2F means that only U2F is supported for 2FA and 2FA is
	// required for all users.
	// U2F is marked for removal. It currently works as an alias for "webauthn".
	SecondFactorU2F = SecondFactorType("u2f")
	// SecondFactorWebauthn means that only Webauthn is supported for 2FA and 2FA
	// is required for all users.
	SecondFactorWebauthn = SecondFactorType("webauthn")
	// SecondFactorOn means that all 2FA protocols are supported and 2FA is
	// required for all users.
	SecondFactorOn = SecondFactorType("on")
	// SecondFactorOptional means that all 2FA protocols are supported and 2FA
	// is required only for users that have MFA devices registered.
	SecondFactorOptional = SecondFactorType("optional")
)

// LockingMode determines how a (possibly stale) set of locks should be applied
// to an interaction.
type LockingMode string

const (
	// LockingModeStrict causes all interactions to be terminated when the
	// available lock view becomes unreliable.
	LockingModeStrict = LockingMode("strict")

	// LockingModeBestEffort applies the most recently known locks under all
	// circumstances.
	LockingModeBestEffort = LockingMode("best_effort")
)

const (
	// ChanTransport is a channel type that can be used to open a net.Conn
	// through the reverse tunnel server. Used for trusted clusters and dial back
	// nodes.
	ChanTransport = "teleport-transport"

	// ChanTransportDialReq is the first (and only) request sent on a
	// chanTransport channel. It's payload is the address of the host a
	// connection should be established to.
	ChanTransportDialReq = "teleport-transport-dial"

	// RemoteAuthServer is a special non-resolvable address that indicates client
	// requests a connection to the remote auth server.
	RemoteAuthServer = "@remote-auth-server"

	// ALPNSNIAuthProtocol allows dialing local/remote auth service based on SNI cluster name value.
	ALPNSNIAuthProtocol = "teleport-auth@"
	// ALPNSNIProtocolReverseTunnel is TLS ALPN protocol value used to indicate Proxy reversetunnel protocol.
	ALPNSNIProtocolReverseTunnel = "teleport-reversetunnel"
)

const (
	// KubeSNIPrefix is a SNI Kubernetes prefix used for distinguishing the Kubernetes HTTP traffic.
	// DELETE IN 11.0. Deprecated, use only KubeTeleportProxyALPNPrefix.
	KubeSNIPrefix = "kube."
	// KubeTeleportProxyALPNPrefix is a SNI Kubernetes prefix used for distinguishing the Kubernetes HTTP traffic.
	KubeTeleportProxyALPNPrefix = "kube-teleport-proxy-alpn."
)

const (
	// HTTPSProxy is an environment variable pointing to a HTTPS proxy.
	HTTPSProxy = "HTTPS_PROXY"

	// HTTPProxy is an environment variable pointing to a HTTP proxy.
	HTTPProxy = "HTTP_PROXY"

	// NoProxy is an environment variable matching the cases
	// when HTTPS_PROXY or HTTP_PROXY is ignored
	NoProxy = "NO_PROXY"
)

// SessionRecordingService is used to differentiate session recording services.
type SessionRecordingService int

const (
	// SessionRecordingServiceSSH represents the SSH service session.
	SessionRecordingServiceSSH SessionRecordingService = iota
)

// SessionRecordingMode determines how session recording will behave in failure
// scenarios.
type SessionRecordingMode string

const (
	// SessionRecordingModeStrict causes any failure session recording to
	// terminate the session or prevent a new session from starting.
	SessionRecordingModeStrict = SessionRecordingMode("strict")

	// SessionRecordingModeBestEffort allows the session to keep going even when
	// session recording fails.
	SessionRecordingModeBestEffort = SessionRecordingMode("best_effort")
)

// Constants for Traits
const (
	// TraitLogins is the name of the role variable used to store
	// allowed logins.
	TraitLogins = "logins"

	// TraitWindowsLogins is the name of the role variable used
	// to store allowed Windows logins.
	TraitWindowsLogins = "windows_logins"

	// TraitKubeGroups is the name the role variable used to store
	// allowed kubernetes groups
	TraitKubeGroups = "kubernetes_groups"

	// TraitKubeUsers is the name the role variable used to store
	// allowed kubernetes users
	TraitKubeUsers = "kubernetes_users"

	// TraitDBNames is the name of the role variable used to store
	// allowed database names.
	TraitDBNames = "db_names"

	// TraitDBUsers is the name of the role variable used to store
	// allowed database users.
	TraitDBUsers = "db_users"

	// TraitAWSRoleARNs is the name of the role variable used to store
	// allowed AWS role ARNs.
	TraitAWSRoleARNs = "aws_role_arns"
)
