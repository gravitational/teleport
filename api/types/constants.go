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

package types

const (
	// DefaultAPIGroup is a default group of permissions API,
	// lets us to add different permission types
	DefaultAPIGroup = "gravitational.io/teleport"

	// DefaultReleaseServerAddr is the default release service URL
	DefaultReleaseServerAddr = "rlz.teleport.sh"

	// ReleaseServerEnvVar is the environment variable used to overwrite
	// the default release server address
	ReleaseServerEnvVar = "RELEASE_SERVER_HOSTPORT"

	// EnterpriseReleaseEndpoint is the endpoint of Teleport Enterprise
	// releases on the release server
	EnterpriseReleaseEndpoint = "teleport-ent"

	// ActionRead grants read access (get, list)
	ActionRead = "read"

	// ActionWrite allows to write (create, update, delete)
	ActionWrite = "write"

	// Wildcard is a special wildcard character matching everything
	Wildcard = "*"

	// True holds "true" string value
	True = "true"

	// HomeEnvVar specifies the home location for tsh configuration
	// and data
	HomeEnvVar = "TELEPORT_HOME"

	// KindNamespace is a namespace
	KindNamespace = "namespace"

	// KindUser is a user resource
	KindUser = "user"

	// KindHostCert is a host certificate
	KindHostCert = "host_cert"

	// KindJWT is a JWT token signer.
	KindJWT = "jwt"

	// KindLicense is a license resource
	KindLicense = "license"

	// KindRole is a role resource
	KindRole = "role"

	// KindAccessRequest is an AccessRequest resource
	KindAccessRequest = "access_request"

	// KindPluginData is a PluginData resource
	KindPluginData = "plugin_data"

	// KindAccessPluginData is a resource directive that applies
	// only to plugin data associated with access requests.
	KindAccessPluginData = "access_plugin_data"

	// KindOIDC is OIDC connector resource
	KindOIDC = "oidc"

	// KindSAML is SAML connector resource
	KindSAML = "saml"

	// KindGithub is Github connector resource
	KindGithub = "github"

	// KindOIDCRequest is OIDC auth request resource
	KindOIDCRequest = "oidc_request"

	// KindSAMLRequest is SAML auth request resource
	KindSAMLRequest = "saml_request"

	// KindGithubRequest is Github auth request resource
	KindGithubRequest = "github_request"

	// KindSession is a recorded SSH session.
	KindSession = "session"

	// KindSSHSession is an active SSH session.
	KindSSHSession = "ssh_session"

	// KindWebSession is a web session resource
	KindWebSession = "web_session"

	// KindWebToken is a web token resource
	KindWebToken = "web_token"

	// KindAppSession represents an application specific web session.
	KindAppSession = "app_session"

	// KindSnowflakeSession represents a Snowflake specific web session.
	KindSnowflakeSession = "snowflake_session"

	// KindSAMLIdPSession represents a SAML IdP session.
	KindSAMLIdPSession = "saml_idp_session"

	// KindEvent is structured audit logging event
	KindEvent = "event"

	// KindAuthServer is auth server resource
	KindAuthServer = "auth_server"

	// KindProxy is proxy resource
	KindProxy = "proxy"

	// KindNode is node resource. It can be either a Teleport node or
	// a registered OpenSSH (agentless) node.
	KindNode = "node"

	// SubKindTeleportNode is a Teleport node.
	SubKindTeleportNode = "teleport"

	// SubKindOpenSSHNode is a registered OpenSSH (agentless) node.
	SubKindOpenSSHNode = "openssh"

	// KindAppServer is an application server resource.
	KindAppServer = "app_server"

	// KindApp is a web app resource.
	KindApp = "app"

	// KindDatabaseServer is a database proxy server resource.
	KindDatabaseServer = "db_server"

	// KindDatabaseService is a database service resource.
	KindDatabaseService = "db_service"

	// KindDatabase is a database resource.
	KindDatabase = "db"

	// KindKubeServer is an kubernetes server resource.
	KindKubeServer = "kube_server"

	// KindKubernetesCluster is a Kubernetes cluster.
	KindKubernetesCluster = "kube_cluster"

	// KindKubePod is an Kubernetes Pod resource type.
	KindKubePod = "pod"

	// KindToken is a provisioning token resource
	KindToken = "token"

	// KindCertAuthority is a certificate authority resource
	KindCertAuthority = "cert_authority"

	// KindReverseTunnel is a reverse tunnel connection
	KindReverseTunnel = "tunnel"

	// KindOIDCConnector is a OIDC connector resource
	KindOIDCConnector = "oidc"

	// KindSAMLConnector is a SAML connector resource
	KindSAMLConnector = "saml"

	// KindGithubConnector is Github OAuth2 connector resource
	KindGithubConnector = "github"

	// KindConnectors is a shortcut for all authentication connector
	KindConnectors = "connectors"

	// KindClusterAuthPreference is the type of authentication for this cluster.
	KindClusterAuthPreference = "cluster_auth_preference"

	// MetaNameClusterAuthPreference is the type of authentication for this cluster.
	MetaNameClusterAuthPreference = "cluster-auth-preference"

	// KindSessionRecordingConfig is the resource for session recording configuration.
	KindSessionRecordingConfig = "session_recording_config"

	// MetaNameSessionRecordingConfig is the exact name of the singleton resource for
	// session recording configuration.
	MetaNameSessionRecordingConfig = "session-recording-config"

	// KindClusterConfig is the resource that holds cluster level configuration.
	// Deprecated: This does not correspond to an actual resource anymore but is
	// still used when checking access to the new configuration resources, as an
	// alternative to their individual resource kinds.
	KindClusterConfig = "cluster_config"

	// KindClusterAuditConfig is the resource that holds cluster audit configuration.
	KindClusterAuditConfig = "cluster_audit_config"

	// MetaNameClusterAuditConfig is the exact name of the singleton resource holding
	// cluster audit configuration.
	MetaNameClusterAuditConfig = "cluster-audit-config"

	// MetaNameUIConfig is the exact name of the singleton resource holding
	// proxy service UI configuration.
	MetaNameUIConfig = "ui-config"

	// KindClusterNetworkingConfig is the resource that holds cluster networking configuration.
	KindClusterNetworkingConfig = "cluster_networking_config"

	// MetaNameClusterNetworkingConfig is the exact name of the singleton resource holding
	// cluster networking configuration.
	MetaNameClusterNetworkingConfig = "cluster-networking-config"

	// KindSemaphore is the resource that provides distributed semaphore functionality
	KindSemaphore = "semaphore"

	// KindClusterName is a type of configuration resource that contains the cluster name.
	KindClusterName = "cluster_name"

	// MetaNameClusterName is the name of a configuration resource for cluster name.
	MetaNameClusterName = "cluster-name"

	// MetaNameWatchStatus is the name of a watch status resource.
	MetaNameWatchStatus = "watch-status"

	// KindStaticTokens is a type of configuration resource that contains static tokens.
	KindStaticTokens = "static_tokens"

	// MetaNameStaticTokens is the name of a configuration resource for static tokens.
	MetaNameStaticTokens = "static-tokens"

	// MetaNameSessionTracker is the prefix of resources used to track live sessions.
	MetaNameSessionTracker = "session-tracker"

	// KindTrustedCluster is a resource that contains trusted cluster configuration.
	KindTrustedCluster = "trusted_cluster"

	// KindAuthConnector allows access to OIDC and SAML connectors.
	KindAuthConnector = "auth_connector"

	// KindTunnelConnection specifies connection of a reverse tunnel to proxy
	KindTunnelConnection = "tunnel_connection"

	// KindRemoteCluster represents remote cluster connected via reverse tunnel
	// to proxy
	KindRemoteCluster = "remote_cluster"

	// KindUserToken is a user token used for various user related actions.
	KindUserToken = "user_token"

	// KindUserTokenSecrets is user token secrets.
	KindUserTokenSecrets = "user_token_secrets"

	// KindIdentity is local on disk identity resource
	KindIdentity = "identity"

	// KindState is local on disk process state
	KindState = "state"

	// KindMFADevice is an MFA device for a user.
	KindMFADevice = "mfa_device"

	// KindBilling represents access to cloud billing features
	KindBilling = "billing"

	// KindLock is a lock resource.
	KindLock = "lock"

	// KindNetworkRestrictions are restrictions for SSH sessions
	KindNetworkRestrictions = "network_restrictions"

	// MetaNameNetworkRestrictions is the exact name of the singleton resource for
	// network restrictions
	MetaNameNetworkRestrictions = "network-restrictions"

	// KindWindowsDesktopService is a Windows desktop service resource.
	KindWindowsDesktopService = "windows_desktop_service"

	// KindWindowsDesktop is a Windows desktop host.
	KindWindowsDesktop = "windows_desktop"

	// KindRecoveryCodes is a resource that holds users recovery codes.
	KindRecoveryCodes = "recovery_codes"

	// KindSessionTracker is a resource that tracks a live session.
	KindSessionTracker = "session_tracker"

	// KindConnectionDiagnostic is a resource that tracks the result of testing a connection
	KindConnectionDiagnostic = "connection_diagnostic"

	// KindDatabaseCertificate is a resource to control Database Certificates generation
	KindDatabaseCertificate = "database_certificate"

	// KindInstaller is a resource that holds a node installer script
	// used to install teleport on discovered nodes
	KindInstaller = "installer"

	// KindUIConfig is a resource that holds configuration for the UI
	// served by the proxy service
	KindUIConfig = "ui_config"

	// KindClusterAlert is a resource that conveys a cluster-level alert message.
	KindClusterAlert = "cluster_alert"

	// KindDevice represents a registered or trusted device.
	KindDevice = "device"

	// KindDownload represents Teleport binaries downloads.
	KindDownload = "download"

	// KindUsageEvent is an external cluster usage event. Similar to
	// KindHostCert, this kind is not backed by a real resource.
	KindUsageEvent = "usage_event"

	// KindInstance represents a teleport instance independent of any specific service.
	KindInstance = "instance"

	// KindLoginRule is a login rule resource.
	KindLoginRule = "login_rule"

	// KindPlugin represents a plugin instance
	KindPlugin = "plugin"

	// KindPluginStaticCredentials represents plugin static credentials.
	KindPluginStaticCredentials = "plugin_static_credentials"

	// KindSAMLIdPServiceProvider is a SAML service provider for the built in Teleport IdP.
	KindSAMLIdPServiceProvider = "saml_idp_service_provider"

	// KindUserGroup is an externally sourced user group.
	KindUserGroup = "user_group"

	// KindOktaImportRule is a rule for importing Okta objects.
	KindOktaImportRule = "okta_import_rule"

	// KindOktaAssignment is a set of actions to apply to Okta.
	KindOktaAssignment = "okta_assignment"

	// KindHeadlessAuthentication is a headless authentication resource.
	KindHeadlessAuthentication = "headless_authentication"

	// KindAssistant is used to program RBAC for
	// Teleport Assist resources.
	KindAssistant = "assistant"

	// KindIntegration is a connection to a 3rd party system API.
	KindIntegration = "integration"

	// KindClusterMaintenanceConfig determines maintenance times for the cluster.
	KindClusterMaintenanceConfig = "cluster_maintenance_config"

	// KindServerInfo contains info that should be applied to joining Nodes.
	KindServerInfo = "server_info"

	// MetaNameClusterMaintenanceConfig is the only allowed metadata.name value for the maintenance
	// window singleton resource.
	MetaNameClusterMaintenanceConfig = "cluster-maintenance-config"

	// KindWatchStatus is a kind for WatchStatus resource which contains information about a successful Watch request.
	KindWatchStatus = "watch_status"

	// V7 is the seventh version of resources.
	V7 = "v7"

	// V6 is the sixth version of resources.
	V6 = "v6"

	// V5 is the fifth version of resources.
	V5 = "v5"

	// V4 is the fourth version of resources.
	V4 = "v4"

	// V3 is the third version of resources.
	V3 = "v3"

	// V2 is the second version of resources.
	V2 = "v2"

	// V1 is the first version of resources. Note: The first version was
	// not explicitly versioned.
	V1 = "v1"
)

// WebSessionSubKinds lists subkinds of web session resources
var WebSessionSubKinds = []string{KindAppSession, KindWebSession, KindSnowflakeSession, KindSAMLIdPSession}

const (
	// VerbList is used to list all objects. Does not imply the ability to read a single object.
	VerbList = "list"

	// VerbCreate is used to create an object.
	VerbCreate = "create"

	// VerbRead is used to read a single object.
	VerbRead = "read"

	// VerbReadNoSecrets is used to read a single object without secrets.
	VerbReadNoSecrets = "readnosecrets"

	// VerbUpdate is used to update an object.
	VerbUpdate = "update"

	// VerbDelete is used to remove an object.
	VerbDelete = "delete"

	// VerbRotate is used to rotate certificate authorities
	// used only internally
	VerbRotate = "rotate"

	// VerbCreateEnrollToken allows the creation of device enrollment tokens.
	// Device Trust is a Teleport Enterprise feature.
	VerbCreateEnrollToken = "create_enroll_token"

	// VerbEnroll allows enrollment of trusted devices.
	// Device Trust is a Teleport Enterprise feature.
	VerbEnroll = "enroll"

	// VerbUse allows the usage of an Integration.
	// Roles with this verb can issue API calls using the integration.
	VerbUse = "use"
)

const (
	// TeleportNamespace is used as the namespace prefix for labels defined by Teleport which can
	// carry metadata such as cloud AWS account or instance. Those labels can be used for RBAC.
	//
	// If a label with this prefix is used in a config file, the associated feature must take into
	// account that the label might be removed, modified or could have been set by the user.
	//
	// See also TeleportInternalLabelPrefix and TeleportHiddenLabelPrefix.
	TeleportNamespace = "teleport.dev"

	// OriginLabel is a resource metadata label name used to identify a source
	// that the resource originates from.
	OriginLabel = TeleportNamespace + "/origin"

	// ADLabel is a resource metadata label name used to identify if resource is part of Active Directory
	ADLabel = TeleportNamespace + "/ad"

	// OriginDefaults is an origin value indicating that the resource was
	// constructed as a default value.
	OriginDefaults = "defaults"

	// OriginConfigFile is an origin value indicating that the resource is
	// derived from static configuration.
	OriginConfigFile = "config-file"

	// OriginDynamic is an origin value indicating that the resource was
	// committed as dynamic configuration.
	OriginDynamic = "dynamic"

	// OriginCloud is an origin value indicating that the resource was
	// imported from a cloud provider.
	OriginCloud = "cloud"

	// OriginKubernetes is an origin value indicating that the resource was
	// created from the Kubernetes Operator.
	OriginKubernetes = "kubernetes"

	// OriginOkta is an origin value indicating that the resource was
	// created from the Okta service.
	OriginOkta = "okta"

	// AWSAccountIDLabel is used to identify nodes by AWS account ID
	// found via automatic discovery, to avoid re-running installation
	// commands on the node.
	AWSAccountIDLabel = TeleportNamespace + "/account-id"
	// AWSInstanceIDLabel is used to identify nodes by EC2 instance ID
	// found via automatic discovery, to avoid re-running installation
	// commands on the node.
	AWSInstanceIDLabel = TeleportNamespace + "/instance-id"
	// AWSInstanceRegion is used to identify the region an EC2
	// instance is running in
	AWSInstanceRegion = TeleportNamespace + "/aws-region"
	// SubscriptionIDLabel is used to identify virtual machines by Azure
	// subscription ID found via automatic discovery, to avoid re-running
	// installation commands on the node.
	SubscriptionIDLabel = TeleportNamespace + "/subscription-id"
	// VMIDLabel is used to identify virtual machines by ID found
	// via automatic discovery, to avoid re-running installation commands
	// on the node.
	VMIDLabel = TeleportNamespace + "/vm-id"

	// CloudLabel is used to identify the cloud where the resource was discovered.
	CloudLabel = TeleportNamespace + "/cloud"

	// DatabaseAdminLabel is used to identify database admin user for auto-
	// discovered databases.
	DatabaseAdminLabel = TeleportNamespace + "/db-admin"

	// ReqAnnotationSchedulesLabel is the request annotation key at which schedules are stored for access plugins.
	ReqAnnotationSchedulesLabel = "/schedules"

	// CloudAWS identifies that a resource was discovered in AWS.
	CloudAWS = "AWS"
	// CloudAzure identifies that a resource was discovered in Azure.
	CloudAzure = "Azure"
	// CloudGCP identifies that a resource was discovered in GCP.
	CloudGCP = "GCP"

	// TeleportAzureMSIEndpoint is a special URL intercepted by TSH local proxy, serving Azure credentials.
	TeleportAzureMSIEndpoint = "azure-msi." + TeleportNamespace
)

const (
	// TeleportInternalLabelPrefix is the prefix used by all Teleport internal labels. Those labels
	// are automatically populated by Teleport and are expected to be used by Teleport internal
	// components and not for RBAC.
	//
	// See also TeleportNamespace and TeleportHiddenLabelPrefix.
	TeleportInternalLabelPrefix = "teleport.internal/"

	// TeleportHiddenLabelPrefix is the prefix used by all user specified hidden labels.
	//
	// See also TeleportNamespace and TeleportInternalLabelPrefix.
	TeleportHiddenLabelPrefix = "teleport.hidden/"

	// BotLabel is a label used to identify a resource used by a certificate renewal bot.
	BotLabel = TeleportInternalLabelPrefix + "bot"

	// BotGenerationLabel is a label used to record the certificate generation counter.
	BotGenerationLabel = TeleportInternalLabelPrefix + "bot-generation"

	// InternalResourceIDLabel is a label used to store an ID to correlate between two resources
	// A pratical example of this is to create a correlation between a Node Provision Token and
	// the Node that used that token to join the cluster
	InternalResourceIDLabel = TeleportInternalLabelPrefix + "resource-id"

	// AlertOnLogin is an internal label that indicates an alert should be displayed to users on login
	AlertOnLogin = TeleportInternalLabelPrefix + "alert-on-login"

	// AlertPermitAll is an internal label that indicates that an alert is suitable for display
	// to all users.
	AlertPermitAll = TeleportInternalLabelPrefix + "alert-permit-all"

	// AlertLink is an internal label that indicates that an alert is a link.
	AlertLink = TeleportInternalLabelPrefix + "link"

	// AlertVerbPermit is an internal label that permits a user to view the alert if they
	// hold a specific resource permission verb (e.g. 'node:list'). Note that this label is
	// a coarser control than it might initially appear and has the potential for accidental
	// misuse. Because this permitting strategy doesn't take into account constraints such as
	// label selectors or where clauses, it can't reliably protect information related to a
	// specific resource. This label should be used only for permitting of alerts that are
	// of concern to holders of a given <resource>:<verb> capability in the most general case.
	AlertVerbPermit = TeleportInternalLabelPrefix + "alert-verb-permit"

	// AlertSupersedes is an internal label used to indicate when one alert supersedes
	// another. Teleport may choose to hide the superseded alert if the superseding alert
	// is also visible to the user and of higher or equivalent severity. This intended as
	// a mechanism for reducing noise/redundancy, and is not a form of access control. Use
	// one of the "permit" labels if you need to restrict viewership of an alert.
	AlertSupersedes = TeleportInternalLabelPrefix + "alert-supersedes"

	// AlertLicenseExpired is an internal label that indicates that the license has expired.
	AlertLicenseExpired = TeleportInternalLabelPrefix + "license-expired-warning"

	// TeleportInternalDiscoveryGroupName is the label used to store the name of the discovery group
	// that the discovered resource is owned by. It is used to differentiate resources
	// that belong to different discovery services that operate on different sets of resources.
	TeleportInternalDiscoveryGroupName = TeleportInternalLabelPrefix + "discovery-group-name"

	// TeleportDowngradedLabel identifies resources that have been automatically
	// downgraded before being returned to clients on older versions that do not
	// support one or more features enabled in that resource.
	TeleportDowngradedLabel = TeleportInternalLabelPrefix + "downgraded"

	// TeleportInternalResourceType indicates the type of internal Teleport resource a resource is.
	// Valid values are:
	// - system: These resources will be automatically created and overwritten on startup. Users should
	//           not change these resources.
	// - preset: These resources will be created if they don't exist. Updates may be applied to them,
	//           but user changes to these resources will be preserved.
	TeleportInternalResourceType = TeleportInternalLabelPrefix + "resource-type"

	// SystemResource are resources that will be automatically created and overwritten on startup. Users
	// should not change these resources.
	SystemResource = "system"

	// PresetResource are resources resources will be created if they don't exist. Updates may be applied
	// to them, but user changes to these resources will be preserved.
	PresetResource = "preset"
)

// CloudHostnameTag is the name of the tag in a cloud instance used to override a node's hostname.
const CloudHostnameTag = "TeleportHostname"

// InstanceMetadataType is the type of cloud instance metadata client.
type InstanceMetadataType string

const (
	InstanceMetadataTypeDisabled InstanceMetadataType = "disabled"
	InstanceMetadataTypeEC2      InstanceMetadataType = "EC2"
	InstanceMetadataTypeAzure    InstanceMetadataType = "Azure"
)

// OriginValues lists all possible origin values.
var OriginValues = []string{
	OriginDefaults,
	OriginConfigFile,
	OriginDynamic,
	OriginCloud,
	OriginKubernetes,
	OriginOkta,
}

const (
	// RecordAtNode is the default. Sessions are recorded at Teleport nodes.
	RecordAtNode = "node"

	// RecordAtProxy enables the recording proxy which intercepts and records
	// all sessions.
	RecordAtProxy = "proxy"

	// RecordOff is used to disable session recording completely.
	RecordOff = "off"

	// RecordAtNodeSync enables the nodes to stream sessions in sync mode
	// to the auth server
	RecordAtNodeSync = "node-sync"

	// RecordAtProxySync enables the recording proxy which intercepts and records
	// all sessions, streams the records synchronously
	RecordAtProxySync = "proxy-sync"
)

// SessionRecordingModes lists all possible session recording modes.
var SessionRecordingModes = []string{RecordAtNode, RecordAtProxy, RecordOff, RecordAtNodeSync, RecordAtProxySync}

// TunnelType is the type of tunnel.
type TunnelType string

const (
	// NodeTunnel is a tunnel where the node connects to the proxy (dial back).
	NodeTunnel TunnelType = "node"

	// ProxyTunnel is a tunnel where a proxy connects to the proxy (trusted cluster).
	ProxyTunnel TunnelType = "proxy"

	// AppTunnel is a tunnel where the application proxy dials back to the proxy.
	AppTunnel TunnelType = "app"

	// KubeTunnel is a tunnel where the kubernetes service dials back to the proxy.
	KubeTunnel TunnelType = "kube"

	// DatabaseTunnel is a tunnel where a database proxy dials back to the proxy.
	DatabaseTunnel TunnelType = "db"

	// WindowsDesktopTunnel is a tunnel where the Windows desktop service dials back to the proxy.
	WindowsDesktopTunnel TunnelType = "windows_desktop"

	// OktaTunnel is a tunnel where the Okta service dials back to the proxy.
	OktaTunnel TunnelType = "okta"
)

type TunnelStrategyType string

const (
	// AgentMesh requires agents to create a reverse tunnel to
	// every proxy server.
	AgentMesh TunnelStrategyType = "agent_mesh"
	// ProxyPeering requires agents to create a reverse tunnel to a configured
	// number of proxy servers and enables proxy to proxy communication.
	ProxyPeering TunnelStrategyType = "proxy_peering"
)

const (
	// ResourceMetadataName refers to a resource metadata field named "name".
	ResourceMetadataName = "name"

	// ResourceSpecDescription refers to a resource spec field named "description".
	ResourceSpecDescription = "description"

	// ResourceSpecHostname refers to a resource spec field named "hostname".
	ResourceSpecHostname = "hostname"

	// ResourceSpecAddr refers to a resource spec field named "address".
	ResourceSpecAddr = "address"

	// ResourceSpecPublicAddr refers to a resource field named "address".
	ResourceSpecPublicAddr = "publicAddress"

	// ResourceSpecType refers to a resource field named "type".
	ResourceSpecType = "type"
)

// RequestableResourceKinds lists all Teleport resource kinds users can request access to.
var RequestableResourceKinds = []string{
	KindNode,
	KindKubernetesCluster,
	KindDatabase,
	KindApp,
	KindWindowsDesktop,
	KindKubePod,
	KindUserGroup,
}

// KubernetesResourcesKinds lists the supported Kubernetes resource kinds.
var KubernetesResourcesKinds = []string{
	KindKubePod,
}

const (
	// TeleportServiceGroup is a default group that users of the
	// teleport automated user provisioning system get added to so
	// already existing users are not deleted
	TeleportServiceGroup = "teleport-system"
)
