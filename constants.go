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

package teleport

import (
	"strings"
	"time"

	"github.com/gravitational/trace"
)

const (
	// SSHAuthSock is the environment variable pointing to the
	// Unix socket the SSH agent is running on.
	SSHAuthSock = "SSH_AUTH_SOCK"
	// SSHAgentPID is the environment variable pointing to the agent
	// process ID
	SSHAgentPID = "SSH_AGENT_PID"

	// SSHTeleportUser is the current Teleport user that is logged in.
	SSHTeleportUser = "SSH_TELEPORT_USER"

	// SSHSessionWebProxyAddr is the address the web proxy.
	SSHSessionWebProxyAddr = "SSH_SESSION_WEBPROXY_ADDR"

	// SSHTeleportClusterName is the name of the cluster this node belongs to.
	SSHTeleportClusterName = "SSH_TELEPORT_CLUSTER_NAME"

	// SSHTeleportHostUUID is the UUID of the host.
	SSHTeleportHostUUID = "SSH_TELEPORT_HOST_UUID"

	// SSHSessionID is the UUID of the current session.
	SSHSessionID = "SSH_SESSION_ID"
)

const (
	// HTTPNextProtoTLS is the NPN/ALPN protocol negotiated during
	// HTTP/1.1.'s TLS setup.
	// https://www.iana.org/assignments/tls-extensiontype-values/tls-extensiontype-values.xhtml#alpn-protocol-ids
	HTTPNextProtoTLS = "http/1.1"
)

const (
	// TOTPValidityPeriod is the number of seconds a TOTP token is valid.
	TOTPValidityPeriod uint = 30

	// TOTPSkew adds that many periods before and after to the validity window.
	TOTPSkew uint = 1
)

const (
	// ComponentKey is a field that represents a component - e.g. service or
	// function
	ComponentKey = "trace.component"
	// ComponentFields is a fields component
	ComponentFields = "trace.fields"

	// ComponentMemory is a memory backend
	ComponentMemory = "memory"

	// ComponentAuthority is a TLS and an SSH certificate authority
	ComponentAuthority = "ca"

	// ComponentProcess is a main control process
	ComponentProcess = "proc"

	// ComponentServer is a server subcomponent of some services
	ComponentServer = "server"

	// ComponentACME is ACME protocol controller
	ComponentACME = "acme"

	// ComponentReverseTunnelServer is reverse tunnel server
	// that together with agent establish a bi-directional SSH revers tunnel
	// to bypass firewall restrictions
	ComponentReverseTunnelServer = "proxy:server"

	// ComponentReverseTunnelAgent is reverse tunnel agent
	// that together with server establish a bi-directional SSH revers tunnel
	// to bypass firewall restrictions
	ComponentReverseTunnelAgent = "proxy:agent"

	// ComponentLabel is a component label name used in reporting
	ComponentLabel = "component"

	// ComponentProxyKube is a kubernetes proxy
	ComponentProxyKube = "proxy:kube"

	// ComponentAuth is the cluster CA node (auth server API)
	ComponentAuth = "auth"

	// ComponentGRPC is gRPC server
	ComponentGRPC = "grpc"

	// ComponentMigrate is responsible for data migrations
	ComponentMigrate = "migrate"

	// ComponentNode is SSH node (SSH server serving requests)
	ComponentNode = "node"

	// ComponentForwardingNode is SSH node (SSH server serving requests)
	ComponentForwardingNode = "node:forward"

	// ComponentProxy is SSH proxy (SSH server forwarding connections)
	ComponentProxy = "proxy"

	// ComponentProxyPeer is the proxy peering component of the proxy service
	ComponentProxyPeer = "proxy:peer"

	// ComponentApp is the application proxy service.
	ComponentApp = "app:service"

	// ComponentDatabase is the database proxy service.
	ComponentDatabase = "db:service"

	// ComponentDiscovery is the Discovery service.
	ComponentDiscovery = "discovery:service"

	// ComponentAppProxy is the application handler within the web proxy service.
	ComponentAppProxy = "app:web"

	// ComponentWebProxy is the web handler within the web proxy service.
	ComponentWebProxy = "web"

	// ComponentDiagnostic is a diagnostic service
	ComponentDiagnostic = "diag"

	// ComponentDiagnosticHealth is the health monitor used by the diagnostic
	// and debug services.
	ComponentDiagnosticHealth = "diag:health"

	// ComponentDebug is the debug service, which exposes debugging
	// configuration over a Unix socket.
	ComponentDebug = "debug"

	// ComponentClient is a client
	ComponentClient = "client"

	// ComponentTunClient is a tunnel client
	ComponentTunClient = "client:tunnel"

	// ComponentCache is a cache component
	ComponentCache = "cache"

	// ComponentBackend is a backend component
	ComponentBackend = "backend"

	// ComponentSubsystemProxy is the proxy subsystem.
	ComponentSubsystemProxy = "subsystem:proxy"

	// ComponentSubsystemSFTP is the SFTP subsystem.
	ComponentSubsystemSFTP = "subsystem:sftp"

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

	// ComponentSOCKS is a SOCKS5 proxy.
	ComponentSOCKS = "socks"

	// ComponentKeyGen is the public/private keypair generator.
	ComponentKeyGen = "keygen"

	// ComponentFirestore represents firestore clients
	ComponentFirestore = "firestore"

	// ComponentSession is an active session.
	ComponentSession = "session"

	// ComponentHostUsers represents host user management.
	ComponentHostUsers = "hostusers"

	// ComponentDynamoDB represents dynamodb clients
	ComponentDynamoDB = "dynamodb"

	// Component pluggable authentication module (PAM)
	ComponentPAM = "pam"

	// ComponentUpload is a session recording upload server
	ComponentUpload = "upload"

	// ComponentWeb is a web server
	ComponentWeb = "web"

	// ComponentUnifiedResource is a cache of resources meant to be listed and displayed
	// together in the web UI
	ComponentUnifiedResource = "unified_resource"

	// ComponentWebsocket is websocket server that the web client connects to.
	ComponentWebsocket = "websocket"

	// ComponentRBAC is role-based access control.
	ComponentRBAC = "rbac"

	// ComponentKeepAlive is keep-alive messages sent from clients to servers
	// and vice versa.
	ComponentKeepAlive = "keepalive"

	// ComponentTeleport is the "teleport" binary.
	ComponentTeleport = "teleport"

	// ComponentTSH is the "tsh" binary.
	ComponentTSH = "tsh"

	// ComponentTCTL is the "tctl" binary.
	ComponentTCTL = "tctl"

	// ComponentTBot is the "tbot" binary
	ComponentTBot = "tbot"

	// ComponentKubeClient is the Kubernetes client.
	ComponentKubeClient = "client:kube"

	// ComponentBuffer is in-memory event circular buffer
	// used to broadcast events to subscribers.
	ComponentBuffer = "buffer"

	// ComponentBPF is the eBPF packagae.
	ComponentBPF = "bpf"

	// ComponentCgroup is the cgroup package.
	ComponentCgroup = "cgroups"

	// ComponentKube is an Kubernetes API gateway.
	ComponentKube = "kubernetes"

	// ComponentSAML is a SAML service provider.
	ComponentSAML = "saml"

	// ComponentMetrics is a metrics server
	ComponentMetrics = "metrics"

	// ComponentWindowsDesktop is a Windows desktop access server.
	ComponentWindowsDesktop = "windows_desktop"

	// ComponentTracing is a tracing exporter
	ComponentTracing = "tracing"

	// ComponentInstance is an abstract component common to all services.
	ComponentInstance = "instance"

	// ComponentVersionControl is the component common to all version control operations.
	ComponentVersionControl = "version-control"

	// ComponentUsageReporting is the component responsible for reporting usage metrics.
	ComponentUsageReporting = "usage-reporting"

	// ComponentAthena represents athena clients.
	ComponentAthena = "athena"

	// ComponentProxySecureGRPC represents a secure gRPC server running on Proxy (used for Kube).
	ComponentProxySecureGRPC = "proxy:secure-grpc"

	// ComponentUpdater represents the teleport-update binary.
	ComponentUpdater = "updater"

	// ComponentRolloutController represents the autoupdate_agent_rollout controller.
	ComponentRolloutController = "rollout-controller"

	// ComponentGit represents git proxy related services.
	ComponentGit = "git"

	// ComponentForwardingGit represents the SSH proxy that forwards Git commands.
	ComponentForwardingGit = "git:forward"

	// VerboseLogsEnvVar forces all logs to be verbose (down to DEBUG level)
	VerboseLogsEnvVar = "TELEPORT_DEBUG"

	// IterationsEnvVar sets tests iterations to run
	IterationsEnvVar = "ITERATIONS"

	// DefaultTerminalWidth defines the default width of a server-side allocated
	// pseudo TTY
	DefaultTerminalWidth = 80

	// DefaultTerminalHeight defines the default height of a server-side allocated
	// pseudo TTY
	DefaultTerminalHeight = 25

	// SafeTerminalType is the fall-back TTY type to fall back to (when $TERM
	// is not defined)
	SafeTerminalType = "xterm"

	// DataDirParameterName is the name of the data dir configuration parameter passed
	// to all backends during initialization
	DataDirParameterName = "data_dir"

	// KeepAliveReqType is a SSH request type to keep the connection alive. A client and
	// a server keep pinging each other with it.
	KeepAliveReqType = "keepalive@openssh.com"

	// ClusterDetailsReqType is the name of a global request which returns cluster details like
	// if the proxy is recording sessions or not and if FIPS is enabled.
	ClusterDetailsReqType = "cluster-details@goteleport.com"

	// JSON means JSON serialization format
	JSON = "json"

	// YAML means YAML serialization format
	YAML = "yaml"

	// Text means text serialization format
	Text = "text"

	// PTY is a raw PTY session capture format
	PTY = "pty"

	// Names is for formatting node names in plain text
	Names = "names"

	// LinuxAdminGID is the ID of the standard adm group on linux
	LinuxAdminGID = 4

	// DirMaskSharedGroup is the mask for a directory accessible
	// by the owner and group
	DirMaskSharedGroup = 0o770

	// FileMaskOwnerOnly is the file mask that allows read write access
	// to owers only
	FileMaskOwnerOnly = 0o600

	// On means mode is on
	On = "on"

	// Off means mode is off
	Off = "off"

	// GCSTestURI turns on GCS tests
	GCSTestURI = "TEST_GCS_URI"

	// AZBlobTestURI specifies the storage account URL to use for Azure Blob
	// Storage tests.
	AZBlobTestURI = "TEST_AZBLOB_URI"

	// AWSRunTests turns on tests executed against AWS directly
	AWSRunTests = "TEST_AWS"

	// AWSRunDBTests turns on tests executed against AWS databases directly.
	AWSRunDBTests = "TEST_AWS_DB"

	// Region is AWS region parameter
	Region = "region"

	// Endpoint is an optional Host for non-AWS S3
	Endpoint = "endpoint"

	// Insecure is an optional switch to use HTTP instead of HTTPS
	Insecure = "insecure"

	// DisableServerSideEncryption is an optional switch to opt out of SSE in case the provider does not support it
	DisableServerSideEncryption = "disablesse"

	// ACL is the canned ACL to send to S3
	ACL = "acl"

	// SSEKMSKey is an optional switch to use an KMS CMK key for S3 SSE.
	SSEKMSKey = "sse_kms_key"

	// SchemeFile configures local disk-based file storage for audit events
	SchemeFile = "file"

	// SchemeStdout outputs audit log entries to stdout
	SchemeStdout = "stdout"

	// SchemeS3 is used for S3-like object storage
	SchemeS3 = "s3"

	// SchemeGCS is used for Google Cloud Storage
	SchemeGCS = "gs"

	// SchemeAZBlob is the Azure Blob Storage scheme, used as the scheme in the
	// session storage URI to identify a storage account accessed over https.
	SchemeAZBlob = "azblob"

	// SchemeAZBlobHTTP is the Azure Blob Storage scheme, used as the scheme in the
	// session storage URI to identify a storage account accessed over http.
	SchemeAZBlobHTTP = "azblob-http"

	// LogsDir is a log subdirectory for events and logs
	LogsDir = "log"

	// Syslog is a mode for syslog logging
	Syslog = "syslog"

	// DebugLevel is a debug logging level name
	DebugLevel = "debug"

	// MinimumEtcdVersion is the minimum version of etcd supported by Teleport
	MinimumEtcdVersion = "3.3.0"

	// EnvVarAllowNoSecondFactor is used to allow disabling second factor auth
	// todo(tross): DELETE WHEN ABLE TO
	EnvVarAllowNoSecondFactor = "TELEPORT_ALLOW_NO_SECOND_FACTOR"
)

const (
	// These values are from https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest

	// OIDCPromptSelectAccount instructs the Authorization Server to
	// prompt the End-User to select a user account.
	OIDCPromptSelectAccount = "select_account"

	// OIDCAccessTypeOnline indicates that OIDC flow should be performed
	// with Authorization server and user connected online
	OIDCAccessTypeOnline = "online"
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
	// CertExtensionPermitX11Forwarding allows X11 forwarding for certificate
	CertExtensionPermitX11Forwarding = "permit-X11-forwarding"
	// CertExtensionPermitAgentForwarding allows agent forwarding for certificate
	CertExtensionPermitAgentForwarding = "permit-agent-forwarding"
	// CertExtensionPermitPTY allows user to request PTY
	CertExtensionPermitPTY = "permit-pty"
	// CertExtensionPermitPortForwarding allows user to request port forwarding
	CertExtensionPermitPortForwarding = "permit-port-forwarding"
	// CertExtensionTeleportRoles is used to propagate teleport roles
	CertExtensionTeleportRoles = "teleport-roles"
	// CertExtensionTeleportRouteToCluster is used to encode
	// the target cluster to route to in the certificate
	CertExtensionTeleportRouteToCluster = "teleport-route-to-cluster"
	// CertExtensionTeleportTraits is used to propagate traits about the user.
	CertExtensionTeleportTraits = "teleport-traits"
	// CertExtensionTeleportActiveRequests is used to track which privilege
	// escalation requests were used to construct the certificate.
	CertExtensionTeleportActiveRequests = "teleport-active-requests"
	// CertExtensionMFAVerified is used to mark certificates issued after an MFA
	// check.
	CertExtensionMFAVerified = "mfa-verified"
	// CertExtensionPreviousIdentityExpires is the extension that stores an RFC3339
	// timestamp representing the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	CertExtensionPreviousIdentityExpires = "prev-identity-expires"
	// CertExtensionLoginIP is used to embed the IP of the client that created
	// the certificate.
	CertExtensionLoginIP = "login-ip"
	// CertExtensionImpersonator is set when one user has requested certificates
	// for another user
	CertExtensionImpersonator = "impersonator"
	// CertExtensionDisallowReissue is set when a certificate should not be allowed
	// to request future certificates.
	CertExtensionDisallowReissue = "disallow-reissue"
	// CertExtensionRenewable is a flag to indicate the certificate may be
	// renewed.
	CertExtensionRenewable = "renewable"
	// CertExtensionGeneration counts the number of times a certificate has
	// been renewed.
	CertExtensionGeneration = "generation"
	// CertExtensionAllowedResources lists the resources which this certificate
	// should be allowed to access
	CertExtensionAllowedResources = "teleport-allowed-resources"
	// CertExtensionConnectionDiagnosticID contains the ID of the ConnectionDiagnostic.
	// The Node/Agent will append connection traces to this diagnostic instance.
	CertExtensionConnectionDiagnosticID = "teleport-connection-diagnostic-id"
	// CertExtensionPrivateKeyPolicy is used to mark certificates with their supported
	// private key policy.
	CertExtensionPrivateKeyPolicy = "private-key-policy"
	// CertExtensionDeviceID is the trusted device identifier.
	CertExtensionDeviceID = "teleport-device-id"
	// CertExtensionDeviceAssetTag is the device inventory identifier.
	CertExtensionDeviceAssetTag = "teleport-device-asset-tag"
	// CertExtensionDeviceCredentialID is the identifier for the credential used
	// by the device to authenticate itself.
	CertExtensionDeviceCredentialID = "teleport-device-credential-id"
	// CertExtensionBotName indicates the name of the Machine ID bot this
	// certificate was issued to, if any.
	CertExtensionBotName = "bot-name@goteleport.com"
	// CertExtensionBotInstanceID indicates the unique identifier of this
	// Machine ID bot instance, if any. This identifier is persisted through
	// certificate renewals.
	CertExtensionBotInstanceID = "bot-instance-id@goteleport.com"

	// CertCriticalOptionSourceAddress is a critical option that defines IP addresses (in CIDR notation)
	// from which this certificate is accepted for authentication.
	// See: https://cvsweb.openbsd.org/src/usr.bin/ssh/PROTOCOL.certkeys?annotate=HEAD.
	CertCriticalOptionSourceAddress = "source-address"
	// CertExtensionGitHubUserID indicates the GitHub user ID identified by the
	// GitHub connector.
	CertExtensionGitHubUserID = "github-id@goteleport.com"
	// CertExtensionGitHubUsername indicates the GitHub username identified by
	// the GitHub connector.
	CertExtensionGitHubUsername = "github-login@goteleport.com"
)

// Note: when adding new providers to this list, consider updating the help message for --provider flag
// for `tctl sso configure oidc` and `tctl sso configure saml` commands
// as well as docs at https://goteleport.com/docs/enterprise/sso/#provider-specific-workarounds
const (
	// NetIQ is an identity provider.
	NetIQ = "netiq"
	// ADFS is Microsoft Active Directory Federation Services
	ADFS = "adfs"
	// Ping is the common backend for all Ping Identity-branded identity
	// providers (including PingOne, PingFederate, etc).
	Ping = "ping"
	// Okta should be used for Okta OIDC providers.
	Okta = "okta"
	// JumpCloud is an identity provider.
	JumpCloud = "jumpcloud"
)

const (
	// RemoteCommandSuccess is returned when a command has successfully executed.
	RemoteCommandSuccess = 0
	// RemoteCommandFailure is returned when a command has failed to execute and
	// we don't have another status code for it.
	RemoteCommandFailure = 255
	// HomeDirNotFound is returned when a the "teleport checkhomedir" command cannot
	// find the user's home directory.
	HomeDirNotFound = 254
	// HomeDirNotAccessible is returned when a the "teleport checkhomedir" command has
	// found the user's home directory, but the user does NOT have permissions to
	// access it.
	HomeDirNotAccessible = 253
)

// MaxEnvironmentFileLines is the maximum number of lines in a environment file.
const MaxEnvironmentFileLines = 1000

// MaxResourceSize is the maximum size (in bytes) of a serialized resource.  This limit is
// typically only enforced against resources that are likely to arbitrarily grow (e.g. PluginData).
const MaxResourceSize = 1000000

// MaxHTTPRequestSize is the maximum accepted size (in bytes) of the body of
// a received HTTP request.  This limit is meant to be used with utils.ReadAtMost
// to prevent resource exhaustion attacks.
const MaxHTTPRequestSize = 10 * 1024 * 1024

// MaxHTTPResponseSize is the maximum accepted size (in bytes) of the body of
// a received HTTP response.  This limit is meant to be used with utils.ReadAtMost
// to prevent resource exhaustion attacks.
const MaxHTTPResponseSize = 10 * 1024 * 1024

const (
	// CertificateFormatOldSSH is used to make Teleport interoperate with older
	// versions of OpenSSH.
	CertificateFormatOldSSH = "oldssh"

	// CertificateFormatUnspecified is used to check if the format was specified
	// or not.
	CertificateFormatUnspecified = ""
)

const (
	// TraitInternalPrefix is the role variable prefix that indicates it's for
	// local accounts.
	TraitInternalPrefix = "internal"

	// TraitExternalPrefix is the role variable prefix that indicates the data comes from an external identity provider.
	TraitExternalPrefix = "external"

	// TraitTeams is the name of the role variable use to store team
	// membership information
	TraitTeams = "github_teams"

	// TraitInternalLoginsVariable is the variable used to store allowed
	// logins for local accounts.
	TraitInternalLoginsVariable = "{{internal.logins}}"

	// TraitInternalWindowsLoginsVariable is the variable used to store
	// allowed Windows Desktop logins for local accounts.
	TraitInternalWindowsLoginsVariable = "{{internal.windows_logins}}"

	// TraitInternalKubeGroupsVariable is the variable used to store allowed
	// kubernetes groups for local accounts.
	TraitInternalKubeGroupsVariable = "{{internal.kubernetes_groups}}"

	// TraitInternalKubeUsersVariable is the variable used to store allowed
	// kubernetes users for local accounts.
	TraitInternalKubeUsersVariable = "{{internal.kubernetes_users}}"

	// TraitInternalDBNamesVariable is the variable used to store allowed
	// database names for local accounts.
	TraitInternalDBNamesVariable = "{{internal.db_names}}"

	// TraitInternalDBUsersVariable is the variable used to store allowed
	// database users for local accounts.
	TraitInternalDBUsersVariable = "{{internal.db_users}}"

	// TraitInternalDBRolesVariable is the variable used to store allowed
	// database roles for automatic database user provisioning.
	TraitInternalDBRolesVariable = "{{internal.db_roles}}"

	// TraitInternalAWSRoleARNs is the variable used to store allowed AWS
	// role ARNs for local accounts.
	TraitInternalAWSRoleARNs = "{{internal.aws_role_arns}}"

	// TraitInternalAzureIdentities is the variable used to store allowed
	// Azure identities for local accounts.
	TraitInternalAzureIdentities = "{{internal.azure_identities}}"

	// TraitInternalGCPServiceAccounts is the variable used to store allowed
	// GCP service accounts for local accounts.
	TraitInternalGCPServiceAccounts = "{{internal.gcp_service_accounts}}"

	// TraitInternalJWTVariable is the variable used to store JWT token for
	// app sessions.
	TraitInternalJWTVariable = "{{internal.jwt}}"

	// TraitInternalGitHubOrgs is the variable used to store allowed GitHub
	// organizations for GitHub integrations.
	TraitInternalGitHubOrgs = "{{internal.github_orgs}}"
)

// SCP is Secure Copy.
const SCP = "scp"

// AdminRoleName is the name of the default admin role for all local users if
// another role is not explicitly assigned
const AdminRoleName = "admin"

const (
	// PresetEditorRoleName is a name of a preset role that allows
	// editing cluster configuration.
	PresetEditorRoleName = "editor"

	// PresetAccessRoleName is a name of a preset role that allows
	// accessing cluster resources.
	PresetAccessRoleName = "access"

	// PresetAuditorRoleName is a name of a preset role that allows
	// reading cluster events and playing back session records.
	PresetAuditorRoleName = "auditor"

	// PresetReviewerRoleName is a name of a preset role that allows
	// for reviewing access requests.
	PresetReviewerRoleName = "reviewer"

	// PresetRequesterRoleName is a name of a preset role that allows
	// for requesting access to resources.
	PresetRequesterRoleName = "requester"

	// PresetGroupAccessRoleName is a name of a preset role that allows
	// access to all user groups.
	PresetGroupAccessRoleName = "group-access"

	// PresetDeviceAdminRoleName is the name of the "device-admin" role.
	// The role is used to administer trusted devices.
	PresetDeviceAdminRoleName = "device-admin"

	// PresetDeviceEnrollRoleName is the name of the "device-enroll" role.
	// The role is used to grant device enrollment powers to users.
	PresetDeviceEnrollRoleName = "device-enroll"

	// PresetRequireTrustedDeviceRoleName is the name of the
	// "require-trusted-device" role.
	// The role is used as a basis for requiring trusted device access to
	// resources.
	PresetRequireTrustedDeviceRoleName = "require-trusted-device"

	// PresetTerraformProviderRoleName is a name of a default role that allows the Terraform provider
	// to configure all its supported Teleport resources.
	PresetTerraformProviderRoleName = "terraform-provider"

	// SystemAutomaticAccessApprovalRoleName names a preset role that may
	// automatically approve any Role Access Request
	SystemAutomaticAccessApprovalRoleName = "@teleport-access-approver"

	// ConnectMyComputerRoleNamePrefix is the prefix used for roles prepared for individual users
	// during the setup of Connect My Computer. The prefix is followed by the name of the cluster
	// user. See teleterm.connectmycomputer.RoleSetup.
	ConnectMyComputerRoleNamePrefix = "connect-my-computer-"

	// SystemOktaRequesterRoleName is a name of a system role that allows
	// for requesting access to Okta resources. This differs from the requester role
	// in that it allows for requesting longer lived access.
	SystemOktaRequesterRoleName = "okta-requester"

	// SystemOktaAccessRoleName is the name of the system role that allows
	// access to Okta resources. This will be used by the Okta requester role to
	// search for Okta resources.
	SystemOktaAccessRoleName = "okta-access"

	// SystemIdentityCenterAccessRoleName specifies the name of a system role
	// that grants a user access to AWS Identity Center resources via
	// Access Requests.
	SystemIdentityCenterAccessRoleName = "aws-ic-access"

	// PresetWildcardWorkloadIdentityIssuerRoleName is a name of a preset role
	// that includes the permissions necessary to issue workload identity
	// credentials using any workload_identity resource. This exists to simplify
	// Day 0 UX experience with workload identity.
	PresetWildcardWorkloadIdentityIssuerRoleName = "wildcard-workload-identity-issuer"
)

var PresetRoles = []string{PresetEditorRoleName, PresetAccessRoleName, PresetAuditorRoleName}

const (
	// SystemAccessApproverUserName names a Teleport user that acts as
	// an Access Request approver for access plugins
	SystemAccessApproverUserName = "@teleport-access-approval-bot"
)

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
	SharedDirMode = 0o750

	// PrivateDirMode is a mode for private directories
	PrivateDirMode = 0o700
)

const (
	// SessionEvent is sent by servers to clients when an audit event occurs on
	// the session.
	SessionEvent = "x-teleport-event"

	// VersionRequest is sent by clients to server requesting the Teleport
	// version they are running.
	VersionRequest = "x-teleport-version"

	// CurrentSessionIDRequest is sent by servers to inform clients of
	// the session ID that is being used.
	CurrentSessionIDRequest = "current-session-id@goteleport.com"

	// SessionIDQueryRequest is sent by clients to ask servers if they
	// will generate their own session ID when a new session is created.
	SessionIDQueryRequest = "session-id-query@goteleport.com"

	// ForceTerminateRequest is an SSH request to forcefully terminate a session.
	ForceTerminateRequest = "x-teleport-force-terminate"

	// TerminalSizeRequest is a request for the terminal size of the session.
	TerminalSizeRequest = "x-teleport-terminal-size"

	// TCPIPForwardRequest is an SSH request for the server to open a listener
	// for port forwarding.
	TCPIPForwardRequest = "tcpip-forward"

	// CancelTCPIPForwardRequest is an SSHRequest to cancel a previous
	// TCPIPForwardRequest.
	CancelTCPIPForwardRequest = "cancel-tcpip-forward"

	// MFAPresenceRequest is an SSH request to notify clients that MFA presence is required for a session.
	MFAPresenceRequest = "x-teleport-mfa-presence"

	// EnvSSHJoinMode is the SSH environment variable that contains the requested participant mode.
	EnvSSHJoinMode = "TELEPORT_SSH_JOIN_MODE"

	// EnvSSHSessionReason is a reason attached to started sessions meant to describe their intent.
	EnvSSHSessionReason = "TELEPORT_SESSION_REASON"

	// EnvSSHSessionInvited is an environment variable listing people invited to a session.
	EnvSSHSessionInvited = "TELEPORT_SESSION_INVITED_USERS"

	// EnvSSHSessionDisplayParticipantRequirements is set to true or false to indicate if participant
	// requirement information should be printed.
	EnvSSHSessionDisplayParticipantRequirements = "TELEPORT_SESSION_PARTICIPANT_REQUIREMENTS"

	// SSHSessionJoinPrincipal is the SSH principal used when joining sessions.
	// This starts with a hyphen so it isn't a valid unix login.
	SSHSessionJoinPrincipal = "-teleport-internal-join"
)

const (
	// EnvKubeConfig is environment variable for kubeconfig
	EnvKubeConfig = "KUBECONFIG"

	// KubeConfigDir is a default directory where k8s stores its user local config
	KubeConfigDir = ".kube"

	// KubeConfigFile is a default filename where k8s stores its user local config
	KubeConfigFile = "config"

	// KubeRunTests turns on kubernetes tests
	KubeRunTests = "TEST_KUBE"

	// KubeSystemAuthenticated is a builtin group that allows
	// any user to access common API methods, e.g. discovery methods
	// required for initial client usage
	KubeSystemAuthenticated = "system:authenticated"

	// UsageKubeOnly specifies certificate usage metadata
	// that limits certificate to be only used for kubernetes proxying
	UsageKubeOnly = "usage:kube"

	// UsageAppOnly specifies a certificate metadata that only allows it to be
	// used for proxying applications.
	UsageAppsOnly = "usage:apps"

	// UsageDatabaseOnly specifies certificate usage metadata that only allows
	// it to be used for proxying database connections.
	UsageDatabaseOnly = "usage:db"

	// UsageWindowsDesktopOnly specifies certificate usage metadata that limits
	// certificate to be only used for Windows desktop access
	UsageWindowsDesktopOnly = "usage:windows_desktop"
)

// ErrNodeIsAmbiguous serves as an identifying error string indicating that
// the proxy subsystem found multiple nodes matching the specified hostname.
var ErrNodeIsAmbiguous = &trace.NotFoundError{Message: "ambiguous host could match multiple nodes"}

const (
	// NodeIsAmbiguous serves as an identifying error string indicating that
	// the proxy subsystem found multiple nodes matching the specified hostname.
	// TODO(tross) DELETE IN v20.0.0
	// Deprecated: Prefer using ErrNodeIsAmbiguous
	NodeIsAmbiguous = "err-node-is-ambiguous"

	// MaxLeases serves as an identifying error string indicating that the
	// semaphore system is rejecting an acquisition attempt due to max
	// leases having already been reached.
	MaxLeases = "err-max-leases"
)

const (
	// OpenBrowserLinux is the command used to open a web browser on Linux.
	OpenBrowserLinux = "xdg-open"

	// OpenBrowserDarwin is the command used to open a web browser on macOS/Darwin.
	OpenBrowserDarwin = "open"

	// OpenBrowserWindows is the command used to open a web browser on Windows.
	OpenBrowserWindows = "rundll32.exe"

	// BrowserNone is the string used to suppress the opening of a browser in
	// response to 'tsh login' commands.
	BrowserNone = "none"
)

const (
	// ExecSubCommand is the sub-command Teleport uses to re-exec itself for
	// command execution (exec and shells).
	ExecSubCommand = "exec"

	// NetworkingSubCommand is the sub-command Teleport uses to re-exec itself
	// for networking operations. e.g. local/remote port forwarding, agent forwarding,
	// or x11 forwarding.
	NetworkingSubCommand = "networking"

	// CheckHomeDirSubCommand is the sub-command Teleport uses to re-exec itself
	// to check if the user's home directory exists.
	CheckHomeDirSubCommand = "checkhomedir"

	// ParkSubCommand is the sub-command Teleport uses to re-exec itself as a
	// specific UID to prevent the matching user from being deleted before
	// spawning the intended child process.
	ParkSubCommand = "park"

	// SFTPSubCommand is the sub-command Teleport uses to re-exec itself to
	// handle SFTP connections.
	SFTPSubCommand = "sftp"

	// WaitSubCommand is the sub-command Teleport uses to wait
	// until a domain name stops resolving. Its main use is to ensure no
	// auth instances are still running the previous major version.
	WaitSubCommand = "wait"

	// VnetAdminSetupSubCommand is the sub-command tsh vnet uses to perform
	// a setup as a privileged user.
	VnetAdminSetupSubCommand = "vnet-admin-setup"
)

const (
	// ChanDirectTCPIP is an SSH channel of type "direct-tcpip".
	ChanDirectTCPIP = "direct-tcpip"

	// ChanForwardedTCPIP is an SSH channel of type "forwarded-tcpip".
	ChanForwardedTCPIP = "forwarded-tcpip"

	// ChanSession is an SSH channel of type "session".
	ChanSession = "session"
)

const (
	// GetHomeDirSubsystem is an SSH subsystem request that Teleport
	// uses to get the home directory of a remote user.
	GetHomeDirSubsystem = "gethomedir"

	// SFTPSubsystem is the SFTP SSH subsystem.
	SFTPSubsystem = "sftp"
)

// A principal name for use in SSH certificates.
type Principal string

const (
	// The localhost domain, for talking to a proxy or node on the same
	// machine.
	PrincipalLocalhost Principal = "localhost"
	// The IPv4 loopback address, for talking to a proxy or node on the same
	// machine.
	PrincipalLoopbackV4 Principal = "127.0.0.1"
	// The IPv6 loopback address, for talking to a proxy or node on the same
	// machine.
	PrincipalLoopbackV6 Principal = "::1"
)

// UserSystem defines a user as system.
const UserSystem = "system"

const (
	// internal application being proxied.
	AppJWTHeader = "teleport-jwt-assertion"

	// HostHeader is the name of the Host header.
	HostHeader = "Host"
)

// UserSingleUseCertTTL is a TTL for per-connection user certificates.
const UserSingleUseCertTTL = time.Minute

// StandardHTTPSPort is the default port used for the https URI scheme,
// cf. RFC 7230 § 2.7.2.
const StandardHTTPSPort = 443

const (
	// KubeSessionDisplayParticipantRequirementsQueryParam is the query parameter used to
	// indicate that the client wants to display the participant requirements
	// for the given session.
	KubeSessionDisplayParticipantRequirementsQueryParam = "displayParticipantRequirements"
	// KubeSessionReasonQueryParam is the query parameter used to indicate the reason
	// for the session request.
	KubeSessionReasonQueryParam = "reason"
	// KubeSessionInvitedQueryParam is the query parameter used to indicate the users
	// to invite to the session.
	KubeSessionInvitedQueryParam = "invite"
)

const (
	// KubeLegacyProxySuffix is the suffix used for legacy proxy services when
	// generating their names Server names.
	KubeLegacyProxySuffix = "-proxy_service"
)

const (
	// DebugServiceSocketName represents the Unix domain socket name of the
	// debug service.
	DebugServiceSocketName = "debug.sock"
)

const (
	// OktaAccessRoleContext is the context used to name Okta Access role created by Okta access list sync
	OktaAccessRoleContext = "access-okta-acl-role"
	// OktaReviewerRoleContext  is the context used to name Okta Reviewer role created by Okta Access List sync
	OktaReviewerRoleContext = "reviewer-okta-acl-role"
)
