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

import (
	"encoding/json"
	"time"

	"github.com/gravitational/trace"
)

const (
	// DefaultImplicitRole is implicit role that gets added to all service.RoleSet
	// objects.
	DefaultImplicitRole = "default-implicit-role"

	// APIDomain is a default domain name for Auth server API. It is often
	// used as an SNI to pass TLS handshakes regardless of the server address
	// since we register "teleport.cluster.local" as a DNS in Certificates.
	APIDomain = "teleport.cluster.local"

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

	// HeadlessConnector is the authentication connector for headless logins.
	HeadlessConnector = "headless"

	// Local means authentication will happen locally within the Teleport cluster.
	Local = "local"

	// OIDC means authentication will happen remotely using an OIDC connector.
	OIDC = "oidc"

	// SAML means authentication will happen remotely using a SAML connector.
	SAML = "saml"

	// Github means authentication will happen remotely using a Github connector.
	Github = "github"

	// HumanDateFormatSeconds is a human readable date formatting with seconds
	HumanDateFormatSeconds = "Jan 2 2006 15:04:05 UTC"

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

	// KeepAliveDatabaseService is the keep alive type for database service.
	KeepAliveDatabaseService = "db_service"

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
	// AWSUSGovConsoleURL is the URL of AWS management console for AWS GovCloud
	// (US) Partition.
	AWSUSGovConsoleURL = "https://console.amazonaws-us-gov.com"
	// AWSCNConsoleURL is the URL of AWS management console for AWS China
	// Partition.
	AWSCNConsoleURL = "https://console.amazonaws.cn"

	// AWSAccountIDLabel is the key of the label containing AWS account ID.
	AWSAccountIDLabel = "aws_account_id"

	// RSAKeySize is the size of the RSA key.
	RSAKeySize = 2048

	// NoLoginPrefix is the prefix used for nologin certificate principals.
	NoLoginPrefix = "-teleport-nologin-"

	// SSHRSAType is the string which specifies an "ssh-rsa" formatted keypair
	SSHRSAType = "ssh-rsa"

	// OktaAssignmentStatusPending is represents a pending status for an Okta assignment.
	OktaAssignmentStatusPending = "pending"

	// OktaAssignmentStatusProcessing is represents an Okta assignment which is currently being acted on.
	OktaAssignmentStatusProcessing = "processing"

	// OktaAssignmentStatusSuccessful is represents a successfully applied Okta assignment.
	OktaAssignmentStatusSuccessful = "successful"

	// OktaAssignmentStatusFailed is represents an Okta assignment which failed to apply. It will be retried.
	OktaAssignmentStatusFailed = "failed"

	// OktaAssignmentStatusPending is represents a unknown status for an Okta assignment.
	OktaAssignmentStatusUnknown = "unknown"

	// OktaAssignmentTargetApplication is an application target of an Okta assignment.
	OktaAssignmentTargetApplication = "application"

	// OktaAssignmentActionTargetGroup is a group target of an Okta assignment.
	OktaAssignmentTargetGroup = "group"

	// OktaAssignmentTargetUnknown is an unknown target of an Okta assignment.
	OktaAssignmentTargetUnknown = "unknown"
)

// LocalConnectors are the system connectors that use local auth.
var LocalConnectors = []string{
	LocalConnector,
	PasswordlessConnector,
}

// SystemConnectors lists the names of the system-reserved connectors.
var SystemConnectors = []string{
	LocalConnector,
	PasswordlessConnector,
	HeadlessConnector,
}

// OIDCPKCEMode represents the mode of PKCE (Proof Key for Code Exchange).
type OIDCPKCEMode string

const (
	// OIDCPKCEModeUnknown indicates an unknown or uninitialized state of the PKCE mode.
	OIDCPKCEModeUnknown OIDCPKCEMode = ""
	// OIDCPKCEModeEnabled indicates that PKCE is enabled for the OIDC flow.
	OIDCPKCEModeEnabled OIDCPKCEMode = "enabled"
	// OIDCPKCEModeDisabled indicates that PKCE is disabled for the OIDC flow.
	OIDCPKCEModeDisabled OIDCPKCEMode = "disabled"
)

// SecondFactorType is the type of 2FA authentication.
type SecondFactorType string

const (
	// SecondFactorOff means no second factor.
	SecondFactorOff = SecondFactorType("off") // todo(lxea): DELETE IN 17
	// SecondFactorOTP means that only OTP is supported for 2FA and 2FA is
	// required for all users.
	SecondFactorOTP = SecondFactorType("otp")
	// SecondFactorU2F means that only Webauthn is supported for 2FA and 2FA
	// is required for all users.
	// Deprecated: "u2f" is aliased to "webauthn". Prefer using
	// SecondFactorWebauthn instead.
	SecondFactorU2F = SecondFactorType("u2f")
	// SecondFactorWebauthn means that only Webauthn is supported for 2FA and 2FA
	// is required for all users.
	SecondFactorWebauthn = SecondFactorType("webauthn")
	// SecondFactorOn means that all 2FA protocols are supported and 2FA is
	// required for all users.
	SecondFactorOn = SecondFactorType("on")
	// SecondFactorOptional means that all 2FA protocols are supported and 2FA
	// is required only for users that have MFA devices registered.
	SecondFactorOptional = SecondFactorType("optional") // todo(lxea): DELETE IN 17
)

// UnmarshalYAML supports parsing off|on into string on SecondFactorType.
func (sft *SecondFactorType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmp interface{}
	if err := unmarshal(&tmp); err != nil {
		return err
	}
	switch v := tmp.(type) {
	case string:
		*sft = SecondFactorType(v)
	case bool:
		if v {
			*sft = SecondFactorOn
		} else {
			*sft = SecondFactorOff
		}
	default:
		return trace.BadParameter("SecondFactorType invalid type %T", v)
	}
	return nil
}

// UnmarshalJSON supports parsing off|on into string on SecondFactorType.
func (sft *SecondFactorType) UnmarshalJSON(data []byte) error {
	var tmp interface{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	switch v := tmp.(type) {
	case string:
		*sft = SecondFactorType(v)
	case bool:
		if v {
			*sft = SecondFactorOn
		} else {
			*sft = SecondFactorOff
		}
	default:
		return trace.BadParameter("SecondFactorType invalid type %T", v)
	}
	return nil
}

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

// DeviceTrustMode is the mode of verification for trusted devices.
// DeviceTrustMode is always "off" for OSS.
// Defaults to "optional" for Enterprise.
type DeviceTrustMode = string

const (
	// DeviceTrustModeOff disables both device authentication and authorization.
	DeviceTrustModeOff DeviceTrustMode = "off"
	// DeviceTrustModeOptional allows both device authentication and
	// authorization, but doesn't enforce the presence of device extensions for
	// sensitive endpoints.
	DeviceTrustModeOptional DeviceTrustMode = "optional"
	// DeviceTrustModeRequired enforces the presence of device extensions for
	// sensitive endpoints.
	DeviceTrustModeRequired DeviceTrustMode = "required"
	// DeviceTrustModeRequiredForHumans enforces the presence of device
	// extensions for sensitive endpoints if the user is human. In this mode,
	// bots are exempt from device trust checks.
	DeviceTrustModeRequiredForHumans DeviceTrustMode = "required-for-humans"
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
	// ALPNSNIProtocolSSH is the TLS ALPN protocol value used to indicate Proxy SSH protocol.
	ALPNSNIProtocolSSH = "teleport-proxy-ssh"
	// ALPNSNIProtocolPingSuffix is TLS ALPN suffix used to wrap connections with Ping.
	ALPNSNIProtocolPingSuffix = "-ping"
)

const (
	// KubeTeleportProxyALPNPrefix is a SNI Kubernetes prefix used for distinguishing the Kubernetes HTTP traffic.
	KubeTeleportProxyALPNPrefix = "kube-teleport-proxy-alpn."
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

// ShowResources determines which resources are shown in the web UI. Default if unset is "requestable"
// which means resources the user has access to and resources they can request will be shown in the
// resources UI. If set to `accessible_only`, only resources the user already has access to will be shown.
type ShowResources string

const (
	// ShowResourcesaccessibleOnly will only show resources the user currently has access to.
	ShowResourcesaccessibleOnly = ShowResources("accessible_only")

	// ShowResourcesRequestable will allow resources that the user can request into resources page.
	ShowResourcesRequestable = ShowResources("requestable")
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

	// TraitDBRoles is the name of the role variable used to store
	// allowed database roles.
	TraitDBRoles = "db_roles"

	// TraitAWSRoleARNs is the name of the role variable used to store
	// allowed AWS role ARNs.
	TraitAWSRoleARNs = "aws_role_arns"

	// TraitAzureIdentities is the name of the role variable used to store
	// allowed Azure identity names.
	TraitAzureIdentities = "azure_identities"

	// TraitGCPServiceAccounts is the name of the role variable used to store
	// allowed GCP service accounts.
	TraitGCPServiceAccounts = "gcp_service_accounts"

	// TraitJWT is the name of the trait containing JWT header for app access.
	TraitJWT = "jwt"

	// TraitHostUserUID is the name of the variable used to specify
	// the UID to create host user account with.
	TraitHostUserUID = "host_user_uid"

	// TraitHostUserGID is the name of the variable used to specify
	// the GID to create host user account with.
	TraitHostUserGID = "host_user_gid"

	// TraitGitHubOrgs is the name of the variable to specify the GitHub
	// organizations for GitHub integration.
	TraitGitHubOrgs = "github_orgs"
	// TraitMCPTools is the name of the variable to specify the MCP tools for
	// MCP servers.
	TraitMCPTools = "mcp_tools"
)

const (
	// TimeoutGetClusterAlerts is the timeout for grabbing cluster alerts from tctl and tsh
	TimeoutGetClusterAlerts = time.Millisecond * 750
)

const (
	// MaxAssumeStartDuration latest duration into the future an access request's assume
	// start time can be
	MaxAssumeStartDuration = time.Hour * 24 * 7
)

const (
	// MaxHealthCheckInterval is the minimum interval between resource health
	// checks.
	MinHealthCheckInterval = 30 * time.Second
	// MaxHealthCheckInterval is the maximum interval between resource health
	// checks. Since timeout must be less than interval, this is effectively the
	// maximum health check timeout as well.
	MaxHealthCheckInterval = 600 * time.Second
	// MinHealthCheckTimeout is the minimum resource health check timeout.
	// There is no corresponding MaxHealthCheckTimeout, because timeout is
	// bounded to be no greater than the interval.
	MinHealthCheckTimeout = time.Second
	// MaxHealthCheckHealthyThreshold is the maximum health check healthy
	// threshold.
	MaxHealthCheckHealthyThreshold = 10
	// MaxHealthCheckUnhealthyThreshold is the maximum health check unhealthy
	// threshold.
	MaxHealthCheckUnhealthyThreshold = MaxHealthCheckHealthyThreshold
)

// Constants for TLS routing connection upgrade. See RFD for more details:
// https://github.com/gravitational/teleport/blob/master/rfd/0123-tls-routing-behind-layer7-lb.md
const (
	// WebAPIConnUpgrade is the HTTP web API to make the connection upgrade
	// call.
	WebAPIConnUpgrade = "/webapi/connectionupgrade"
	// WebAPIConnUpgradeHeader is the header used to indicate the requested
	// connection upgrade types in the connection upgrade API.
	WebAPIConnUpgradeHeader = "Upgrade"
	// WebAPIConnUpgradeTeleportHeader is a Teleport-specific header used to
	// indicate the requested connection upgrade types in the connection
	// upgrade API. This header is sent in addition to "Upgrade" header in case
	// a load balancer/reverse proxy removes "Upgrade".
	WebAPIConnUpgradeTeleportHeader = "X-Teleport-Upgrade"
	// WebAPIConnUpgradeTypeALPN is a connection upgrade type that specifies
	// the upgraded connection should be handled by the ALPN handler.
	WebAPIConnUpgradeTypeALPN = "alpn"
	// WebAPIConnUpgradeTypeALPNPing is a connection upgrade type that
	// specifies the upgraded connection should be handled by the ALPN handler
	// wrapped with the Ping protocol.
	//
	// This should be used when the tunneled TLS Routing protocol cannot keep
	// long-lived connections alive as L7 LB usually ignores TCP keepalives and
	// has very short idle timeouts.
	WebAPIConnUpgradeTypeALPNPing = "alpn-ping"
	// WebAPIConnUpgradeTypeWebSocket is the standard upgrade type for WebSocket.
	WebAPIConnUpgradeTypeWebSocket = "websocket"
	// WebAPIConnUpgradeConnectionHeader is the standard header that controls
	// whether the network connection stays open after the current transaction
	// finishes.
	WebAPIConnUpgradeConnectionHeader = "Connection"
	// WebAPIConnUpgradeConnectionType is the value of the "Connection" header
	// used for connection upgrades.
	WebAPIConnUpgradeConnectionType = "Upgrade"
)

const (
	// InitiateFileTransfer is used when creating a new file transfer request
	InitiateFileTransfer string = "file-transfer@goteleport.com"
	// FileTransferDecision is a request that will approve or deny an active file transfer.
	// Multiple decisions can be sent for the same request if the policy requires it.
	FileTransferDecision string = "file-transfer-decision@goteleport.com"
)

// Terraform provider environment variable names.
// This is mainly used by the Terraform provider and the `tctl terraform` command.
const (
	// EnvVarTerraformAddress is the environment variable configuring the Teleport address the Terraform provider connects to.
	EnvVarTerraformAddress = "TF_TELEPORT_ADDR"
	// EnvVarTerraformCertificates is the environment variable configuring the path the Terraform provider loads its
	// client certificates from. This only works for direct auth joining.
	EnvVarTerraformCertificates = "TF_TELEPORT_CERT"
	// EnvVarTerraformCertificatesBase64 is the environment variable configuring the client certificates used by the
	// Terraform provider. This only works for direct auth joining.
	EnvVarTerraformCertificatesBase64 = "TF_TELEPORT_CERT_BASE64"
	// EnvVarTerraformKey is the environment variable configuring the path the Terraform provider loads its
	// client key from. This only works for direct auth joining.
	EnvVarTerraformKey = "TF_TELEPORT_KEY"
	// EnvVarTerraformKeyBase64 is the environment variable configuring the client key used by the
	// Terraform provider. This only works for direct auth joining.
	EnvVarTerraformKeyBase64 = "TF_TELEPORT_KEY_BASE64"
	// EnvVarTerraformRootCertificates is the environment variable configuring the path the Terraform provider loads its
	// trusted CA certificates from. This only works for direct auth joining.
	EnvVarTerraformRootCertificates = "TF_TELEPORT_ROOT_CA"
	// EnvVarTerraformRootCertificatesBase64 is the environment variable configuring the CA certificates trusted by the
	// Terraform provider. This only works for direct auth joining.
	EnvVarTerraformRootCertificatesBase64 = "TF_TELEPORT_CA_BASE64"
	// EnvVarTerraformProfileName is the environment variable containing name of the profile used by the Terraform provider.
	EnvVarTerraformProfileName = "TF_TELEPORT_PROFILE_NAME"
	// EnvVarTerraformProfilePath is the environment variable containing the profile directory used by the Terraform provider.
	EnvVarTerraformProfilePath = "TF_TELEPORT_PROFILE_PATH"
	// EnvVarTerraformIdentityFilePath is the environment variable containing the path to the identity file used by the provider.
	EnvVarTerraformIdentityFilePath = "TF_TELEPORT_IDENTITY_FILE_PATH"
	// EnvVarTerraformIdentityFile is the environment variable containing the identity file used by the Terraform provider.
	EnvVarTerraformIdentityFile = "TF_TELEPORT_IDENTITY_FILE"
	// EnvVarTerraformIdentityFileBase64 is the environment variable containing the base64-encoded identity file used by the Terraform provider.
	EnvVarTerraformIdentityFileBase64 = "TF_TELEPORT_IDENTITY_FILE_BASE64"
	// EnvVarTerraformInsecure is the environment variable used to control whether the Terraform provider will skip verifying the proxy server's TLS certificate.
	EnvVarTerraformInsecure = "TF_TELEPORT_INSECURE"
	// EnvVarTerraformRetryBaseDuration is the environment variable configuring the base duration between two Terraform provider retries.
	EnvVarTerraformRetryBaseDuration = "TF_TELEPORT_RETRY_BASE_DURATION"
	// EnvVarTerraformRetryCapDuration is the environment variable configuring the maximum duration between two Terraform provider retries.
	EnvVarTerraformRetryCapDuration = "TF_TELEPORT_RETRY_CAP_DURATION"
	// EnvVarTerraformRetryMaxTries is the environment variable configuring the maximum number of Terraform provider retries.
	EnvVarTerraformRetryMaxTries = "TF_TELEPORT_RETRY_MAX_TRIES"
	// EnvVarTerraformDialTimeoutDuration is the environment variable configuring the Terraform provider dial timeout.
	EnvVarTerraformDialTimeoutDuration = "TF_TELEPORT_DIAL_TIMEOUT_DURATION"
	// EnvVarTerraformJoinMethod is the environment variable configuring the Terraform provider native MachineID join method.
	EnvVarTerraformJoinMethod = "TF_TELEPORT_JOIN_METHOD"
	// EnvVarTerraformJoinToken is the environment variable configuring the Terraform provider native MachineID join token.
	EnvVarTerraformJoinToken = "TF_TELEPORT_JOIN_TOKEN"
	// EnvVarTerraformCloudJoinAudienceTag is the environment variable configuring the Terraform provider's native Machine ID
	// joining. The audience tag specifies the optional suffix for the TF_WORKLOAD_IDENTITY_AUDIENCE variable when
	// specifically using the `terraform` join method.
	EnvVarTerraformCloudJoinAudienceTag = "TF_TELEPORT_JOIN_AUDIENCE_TAG"
	// EnvVarGitlabIDTokenEnvVar is the environment variable that specifies the name of the environment variable
	// that contains the GitLab ID token. This can be used to authenticate to multiple Teleport clusters from a single
	// GitLab CI job.
	EnvVarGitlabIDTokenEnvVar = "TF_TELEPORT_GITLAB_ID_TOKEN_ENV_VAR"
)

// MaxPIVPINCacheTTL defines the maximum allowed TTL for PIV PIN client caches.
const MaxPIVPINCacheTTL = time.Hour

// AutoUpdateAgentReportPeriod is the period of the autoupdate agent reporting
// routine running in every auth server. Any report older than this period should
// be considered stale.
const AutoUpdateAgentReportPeriod = time.Minute
