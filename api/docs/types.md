# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [types.proto](#types.proto)
    - [AccessRequestClaimMapping](#services.AccessRequestClaimMapping)
    - [AccessRequestConditions](#services.AccessRequestConditions)
    - [AccessRequestFilter](#services.AccessRequestFilter)
    - [AccessRequestSpecV3](#services.AccessRequestSpecV3)
    - [AccessRequestV3](#services.AccessRequestV3)
    - [AcquireSemaphoreRequest](#services.AcquireSemaphoreRequest)
    - [App](#services.App)
    - [App.DynamicLabelsEntry](#services.App.DynamicLabelsEntry)
    - [App.StaticLabelsEntry](#services.App.StaticLabelsEntry)
    - [AuditConfig](#services.AuditConfig)
    - [BoolValue](#services.BoolValue)
    - [CertAuthoritySpecV2](#services.CertAuthoritySpecV2)
    - [CertAuthorityV2](#services.CertAuthorityV2)
    - [ClusterConfigSpecV3](#services.ClusterConfigSpecV3)
    - [ClusterConfigV3](#services.ClusterConfigV3)
    - [ClusterNameSpecV2](#services.ClusterNameSpecV2)
    - [ClusterNameV2](#services.ClusterNameV2)
    - [CommandLabelV2](#services.CommandLabelV2)
    - [ConnectorRef](#services.ConnectorRef)
    - [CreatedBy](#services.CreatedBy)
    - [ExternalIdentity](#services.ExternalIdentity)
    - [JWTKeyPair](#services.JWTKeyPair)
    - [KeepAlive](#services.KeepAlive)
    - [KubernetesCluster](#services.KubernetesCluster)
    - [KubernetesCluster.DynamicLabelsEntry](#services.KubernetesCluster.DynamicLabelsEntry)
    - [KubernetesCluster.StaticLabelsEntry](#services.KubernetesCluster.StaticLabelsEntry)
    - [LocalAuthSecrets](#services.LocalAuthSecrets)
    - [LoginStatus](#services.LoginStatus)
    - [Metadata](#services.Metadata)
    - [Metadata.LabelsEntry](#services.Metadata.LabelsEntry)
    - [Namespace](#services.Namespace)
    - [NamespaceSpec](#services.NamespaceSpec)
    - [PluginDataEntry](#services.PluginDataEntry)
    - [PluginDataEntry.DataEntry](#services.PluginDataEntry.DataEntry)
    - [PluginDataFilter](#services.PluginDataFilter)
    - [PluginDataSpecV3](#services.PluginDataSpecV3)
    - [PluginDataSpecV3.EntriesEntry](#services.PluginDataSpecV3.EntriesEntry)
    - [PluginDataUpdateParams](#services.PluginDataUpdateParams)
    - [PluginDataUpdateParams.ExpectEntry](#services.PluginDataUpdateParams.ExpectEntry)
    - [PluginDataUpdateParams.SetEntry](#services.PluginDataUpdateParams.SetEntry)
    - [PluginDataV3](#services.PluginDataV3)
    - [ProvisionTokenSpecV2](#services.ProvisionTokenSpecV2)
    - [ProvisionTokenV1](#services.ProvisionTokenV1)
    - [ProvisionTokenV2](#services.ProvisionTokenV2)
    - [RemoteClusterStatusV3](#services.RemoteClusterStatusV3)
    - [RemoteClusterV3](#services.RemoteClusterV3)
    - [ResetPasswordTokenSecretsSpecV3](#services.ResetPasswordTokenSecretsSpecV3)
    - [ResetPasswordTokenSecretsV3](#services.ResetPasswordTokenSecretsV3)
    - [ResetPasswordTokenSpecV3](#services.ResetPasswordTokenSpecV3)
    - [ResetPasswordTokenV3](#services.ResetPasswordTokenV3)
    - [ResourceHeader](#services.ResourceHeader)
    - [ReverseTunnelSpecV2](#services.ReverseTunnelSpecV2)
    - [ReverseTunnelV2](#services.ReverseTunnelV2)
    - [Rewrite](#services.Rewrite)
    - [RoleConditions](#services.RoleConditions)
    - [RoleMapping](#services.RoleMapping)
    - [RoleOptions](#services.RoleOptions)
    - [RoleSpecV3](#services.RoleSpecV3)
    - [RoleV3](#services.RoleV3)
    - [Rotation](#services.Rotation)
    - [RotationSchedule](#services.RotationSchedule)
    - [Rule](#services.Rule)
    - [SemaphoreFilter](#services.SemaphoreFilter)
    - [SemaphoreLease](#services.SemaphoreLease)
    - [SemaphoreLeaseRef](#services.SemaphoreLeaseRef)
    - [SemaphoreSpecV3](#services.SemaphoreSpecV3)
    - [SemaphoreV3](#services.SemaphoreV3)
    - [ServerSpecV2](#services.ServerSpecV2)
    - [ServerSpecV2.CmdLabelsEntry](#services.ServerSpecV2.CmdLabelsEntry)
    - [ServerV2](#services.ServerV2)
    - [StaticTokensSpecV2](#services.StaticTokensSpecV2)
    - [StaticTokensV2](#services.StaticTokensV2)
    - [TLSKeyPair](#services.TLSKeyPair)
    - [TunnelConnectionSpecV2](#services.TunnelConnectionSpecV2)
    - [TunnelConnectionV2](#services.TunnelConnectionV2)
    - [U2FRegistrationData](#services.U2FRegistrationData)
    - [UserRef](#services.UserRef)
    - [UserSpecV2](#services.UserSpecV2)
    - [UserV2](#services.UserV2)
    - [WebSessionSpecV2](#services.WebSessionSpecV2)
    - [WebSessionV2](#services.WebSessionV2)
  
    - [CertAuthoritySpecV2.SigningAlgType](#services.CertAuthoritySpecV2.SigningAlgType)
    - [KeepAlive.KeepAliveType](#services.KeepAlive.KeepAliveType)
    - [RequestState](#services.RequestState)
  
- [Scalar Value Types](#scalar-value-types)



<a name="types.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## types.proto



<a name="services.AccessRequestClaimMapping"></a>

### AccessRequestClaimMapping
AccessRequestClaimMapping is a variant of the trait mapping pattern,
used to propagate requestable roles from external identity providers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Claim | [string](#string) |  | Claim is the name of the trait to be matched against. |
| Value | [string](#string) |  | Value is matches a trait value. |
| Roles | [string](#string) | repeated | Roles are the roles being mapped to. |






<a name="services.AccessRequestConditions"></a>

### AccessRequestConditions
AccessRequestConditions is a matcher for allow/deny restrictions on
access-requests.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Roles | [string](#string) | repeated | Roles is the name of roles which will match the request rule. |
| ClaimsToRoles | [AccessRequestClaimMapping](#services.AccessRequestClaimMapping) | repeated | ClaimsToRoles specifies a mapping from claims (traits) to teleport roles. |
| Annotations | [wrappers.LabelValues](#wrappers.LabelValues) |  | Annotations is a collection of annotations to be programmatically appended to pending access requests at the time of their creation. These annotations serve as a mechanism to propagate extra information to plugins. Since these annotations support variable interpolation syntax, they also offer a mechanism for forwarding claims from an external identity provider, to a plugin via `{{external.trait_name}}` style substitutions. |






<a name="services.AccessRequestFilter"></a>

### AccessRequestFilter
AccessRequestFilter encodes filter params for access requests.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ID | [string](#string) |  | ID specifies a request ID if set. |
| User | [string](#string) |  | User specifies a username if set. |
| State | [RequestState](#services.RequestState) |  | RequestState filters for requests in a specific state. |






<a name="services.AccessRequestSpecV3"></a>

### AccessRequestSpecV3
AccessRequestSpec is the specification for AccessRequest


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| User | [string](#string) |  | User is the name of the user to whom the roles will be applied. |
| Roles | [string](#string) | repeated | Roles is the name of the roles being requested. |
| State | [RequestState](#services.RequestState) |  | State is the current state of this access request. |
| Created | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Created encodes the time at which the request was registered with the auth server. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires constrains the maximum lifetime of any login session for which this request is active. |
| RequestReason | [string](#string) |  | RequestReason is an optional message explaining the reason for the request. |
| ResolveReason | [string](#string) |  | ResolveReason is an optional message explaining the reason for the resolution of the request (approval, denail, etc...). |
| ResolveAnnotations | [wrappers.LabelValues](#wrappers.LabelValues) |  | ResolveAnnotations is a set of arbitrary values received from plugins or other resolving parties during approval/denial. Importantly, these annotations are included in the access_request.update event, allowing plugins to propagate arbitrary structured data to the audit log. |
| SystemAnnotations | [wrappers.LabelValues](#wrappers.LabelValues) |  | SystemAnnotations is a set of programmatically generated annotations attached to pending access requests by teleport. These annotations are generated by applying variable interpolation to the RoleConditions.Request.Annotations block of a user&#39;s role(s). These annotations serve as a mechanism for administrators to pass extra information to plugins when they process pending access requests. |






<a name="services.AccessRequestV3"></a>

### AccessRequestV3
AccessRequest represents an access request resource specification


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is AccessRequest metadata |
| Spec | [AccessRequestSpecV3](#services.AccessRequestSpecV3) |  | Spec is an AccessReqeust specification |






<a name="services.AcquireSemaphoreRequest"></a>

### AcquireSemaphoreRequest
AcquireSemaphoreRequest holds semaphore lease acquisition parameters.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SemaphoreKind | [string](#string) |  | SemaphoreKind is the kind of the semaphore. |
| SemaphoreName | [string](#string) |  | SemaphoreName is the name of the semaphore. |
| MaxLeases | [int64](#int64) |  | MaxLeases is the maximum number of concurrent leases. If acquisition would cause more than MaxLeases to exist, acquisition must fail. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is the time at which this lease expires. |
| Holder | [string](#string) |  | Holder identifies the entitiy holding the lease. |






<a name="services.App"></a>

### App
App is a specific application that a server proxies.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is the name of the application. |
| URI | [string](#string) |  | URI is the internal address the application is available at. |
| PublicAddr | [string](#string) |  | PublicAddr is the public address the application is accessible at. |
| StaticLabels | [App.StaticLabelsEntry](#services.App.StaticLabelsEntry) | repeated | StaticLabels is map of static labels associated with an application. Used for RBAC. |
| DynamicLabels | [App.DynamicLabelsEntry](#services.App.DynamicLabelsEntry) | repeated | DynamicLabels is map of dynamic labels associated with an application. Used for RBAC. |
| InsecureSkipVerify | [bool](#bool) |  |  |
| Rewrite | [Rewrite](#services.Rewrite) |  | Rewrite is a list of rewriting rules to apply to requests and responses. |






<a name="services.App.DynamicLabelsEntry"></a>

### App.DynamicLabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [CommandLabelV2](#services.CommandLabelV2) |  |  |






<a name="services.App.StaticLabelsEntry"></a>

### App.StaticLabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="services.AuditConfig"></a>

### AuditConfig
AuditConfig represents audit log settings in the cluster


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Type | [string](#string) |  | Type is audit backend type |
| Region | [string](#string) |  | Region is a region setting for audit sessions used by cloud providers |
| AuditSessionsURI | [string](#string) |  | AuditSessionsURI is a parameter where to upload sessions |
| AuditEventsURI | [wrappers.StringValues](#wrappers.StringValues) |  | AuditEventsURI is a parameter with all supported outputs for audit events |
| AuditTableName | [string](#string) |  | AuditTableName is a DB table name used for audits Deprecated in favor of AuditEventsURI DELETE IN (3.1.0) |
| EnableContinuousBackups | [bool](#bool) |  | EnableContinuousBackups is used to enable (or disable) PITR (Point-In-Time Recovery). |
| EnableAutoScaling | [bool](#bool) |  | EnableAutoScaling is used to enable (or disable) auto scaling policy. |
| ReadMaxCapacity | [int64](#int64) |  | ReadMaxCapacity is the maximum provisioned read capacity. |
| ReadMinCapacity | [int64](#int64) |  | ReadMinCapacity is the minimum provisioned read capacity. |
| ReadTargetValue | [double](#double) |  | ReadTargetValue is the ratio of consumed read to provisioned capacity. |
| WriteMaxCapacity | [int64](#int64) |  | WriteMaxCapacity is the maximum provisioned write capacity. |
| WriteMinCapacity | [int64](#int64) |  | WriteMinCapacity is the minimum provisioned write capacity. |
| WriteTargetValue | [double](#double) |  | WriteTargetValue is the ratio of consumed write to provisioned capacity. |






<a name="services.BoolValue"></a>

### BoolValue
BoolValue is a wrapper around bool, used in cases
whenever bool value can have different default value when missing


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Value | [bool](#bool) |  |  |






<a name="services.CertAuthoritySpecV2"></a>

### CertAuthoritySpecV2
CertAuthoritySpecV2 is a host or user certificate authority that
can check and if it has private key stored as well, sign it too


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Type | [string](#string) |  | Type is either user or host certificate authority |
| ClusterName | [string](#string) |  | DELETE IN(2.7.0) this field is deprecated, as resource name matches cluster name after migrations. and this property is enforced by the auth server code. ClusterName identifies cluster name this authority serves, for host authorities that means base hostname of all servers, for user authorities that means organization name |
| CheckingKeys | [bytes](#bytes) | repeated | Checkers is a list of SSH public keys that can be used to check certificate signatures |
| SigningKeys | [bytes](#bytes) | repeated | SigningKeys is a list of private keys used for signing |
| Roles | [string](#string) | repeated | Roles is a list of roles assumed by users signed by this CA |
| RoleMap | [RoleMapping](#services.RoleMapping) | repeated | RoleMap specifies role mappings to remote roles |
| TLSKeyPairs | [TLSKeyPair](#services.TLSKeyPair) | repeated | TLS is a list of TLS key pairs |
| Rotation | [Rotation](#services.Rotation) |  | Rotation is a status of the certificate authority rotation |
| SigningAlg | [CertAuthoritySpecV2.SigningAlgType](#services.CertAuthoritySpecV2.SigningAlgType) |  |  |
| JWTKeyPairs | [JWTKeyPair](#services.JWTKeyPair) | repeated | JWTKeyPair is a list of JWT key pairs. |






<a name="services.CertAuthorityV2"></a>

### CertAuthorityV2
CertAuthorityV2 is version 2 resource spec for Cert Authority


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is connector metadata |
| Spec | [CertAuthoritySpecV2](#services.CertAuthoritySpecV2) |  | Spec contains cert authority specification |






<a name="services.ClusterConfigSpecV3"></a>

### ClusterConfigSpecV3
ClusterConfigSpecV3 is the actual data we care about for ClusterConfig.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SessionRecording | [string](#string) |  | SessionRecording controls where (or if) the session is recorded. |
| ClusterID | [string](#string) |  | ClusterID is the unique cluster ID that is set once during the first auth server startup. |
| ProxyChecksHostKeys | [string](#string) |  | ProxyChecksHostKeys is used to control if the proxy will check host keys when in recording mode. |
| Audit | [AuditConfig](#services.AuditConfig) |  | Audit is a section with audit config |
| ClientIdleTimeout | [int64](#int64) |  | ClientIdleTimeout sets global cluster default setting for client idle timeouts |
| DisconnectExpiredCert | [bool](#bool) |  | DisconnectExpiredCert provides disconnect expired certificate setting - if true, connections with expired client certificates will get disconnected |
| KeepAliveInterval | [int64](#int64) |  | KeepAliveInterval is the interval the server sends keep-alive messsages to the client at. |
| KeepAliveCountMax | [int64](#int64) |  | KeepAliveCountMax is the number of keep-alive messages that can be missed before the server disconnects the connection to the client. |
| LocalAuth | [bool](#bool) |  | LocalAuth is true if local authentication is enabled. |
| SessionControlTimeout | [int64](#int64) |  | SessionControlTimeout is the session control lease expiry and defines the upper limit of how long a node may be out of contact with the auth server before it begins terminating controlled sessions. |






<a name="services.ClusterConfigV3"></a>

### ClusterConfigV3
ClusterConfigV3 implements the ClusterConfig interface.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [ClusterConfigSpecV3](#services.ClusterConfigSpecV3) |  | Spec is a cluster config V3 spec |






<a name="services.ClusterNameSpecV2"></a>

### ClusterNameSpecV2
ClusterNameSpecV2 is the actual data we care about for ClusterName.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ClusterName | [string](#string) |  | ClusterName is the name of the cluster. Changing this value once the cluster is setup can and will cause catastrophic problems. |






<a name="services.ClusterNameV2"></a>

### ClusterNameV2
ClusterNameV2 implements the ClusterName interface.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [ClusterNameSpecV2](#services.ClusterNameSpecV2) |  | Spec is a cluster name V2 spec |






<a name="services.CommandLabelV2"></a>

### CommandLabelV2
CommandLabelV2 is a label that has a value as a result of the
output generated by running command, e.g. hostname


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Period | [int64](#int64) |  | Period is a time between command runs |
| Command | [string](#string) | repeated | Command is a command to run |
| Result | [string](#string) |  | Result captures standard output |






<a name="services.ConnectorRef"></a>

### ConnectorRef
ConnectorRef holds information about OIDC connector


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Type | [string](#string) |  | Type is connector type |
| ID | [string](#string) |  | ID is connector ID |
| Identity | [string](#string) |  | Identity is external identity of the user |






<a name="services.CreatedBy"></a>

### CreatedBy
CreatedBy holds information about the person or agent who created the user


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Connector | [ConnectorRef](#services.ConnectorRef) |  | Identity if present means that user was automatically created by identity |
| Time | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Time specifies when user was created |
| User | [UserRef](#services.UserRef) |  | User holds information about user |






<a name="services.ExternalIdentity"></a>

### ExternalIdentity
ExternalIdentity is OpenID Connect/SAML or Github identity that is linked
to particular user and connector and lets user to log in using external
credentials, e.g. google


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ConnectorID | [string](#string) |  | ConnectorID is id of registered OIDC connector, e.g. &#39;google-example.com&#39; |
| Username | [string](#string) |  | Username is username supplied by external identity provider |






<a name="services.JWTKeyPair"></a>

### JWTKeyPair
JWTKeyPair is a PEM encoded keypair used for signing JWT tokens.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| PublicKey | [bytes](#bytes) |  | PublicKey is a PEM encoded public key. |
| PrivateKey | [bytes](#bytes) |  | PrivateKey is a PEM encoded private key. |






<a name="services.KeepAlive"></a>

### KeepAlive



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name of the resource to keep alive. |
| Namespace | [string](#string) |  | Namespace is the namespace of the resource. |
| LeaseID | [int64](#int64) |  | LeaseID is ID of the lease. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is set to update expiry time of the resource. |
| Type | [KeepAlive.KeepAliveType](#services.KeepAlive.KeepAliveType) |  |  |






<a name="services.KubernetesCluster"></a>

### KubernetesCluster
KubernetesCluster is a named kubernetes API endpoint handled by a Server.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is the name of this kubernetes cluster. |
| StaticLabels | [KubernetesCluster.StaticLabelsEntry](#services.KubernetesCluster.StaticLabelsEntry) | repeated | StaticLabels is map of static labels associated with this cluster. Used for RBAC. |
| DynamicLabels | [KubernetesCluster.DynamicLabelsEntry](#services.KubernetesCluster.DynamicLabelsEntry) | repeated | DynamicLabels is map of dynamic labels associated with this cluster. Used for RBAC. |






<a name="services.KubernetesCluster.DynamicLabelsEntry"></a>

### KubernetesCluster.DynamicLabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [CommandLabelV2](#services.CommandLabelV2) |  |  |






<a name="services.KubernetesCluster.StaticLabelsEntry"></a>

### KubernetesCluster.StaticLabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="services.LocalAuthSecrets"></a>

### LocalAuthSecrets
LocalAuthSecrets holds sensitive data used to authenticate a local user.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| PasswordHash | [bytes](#bytes) |  | PasswordHash encodes a combined salt &amp; hash for password verification. |
| TOTPKey | [string](#string) |  | TOTPKey is the key used for Time-based One Time Password varification. |
| U2FRegistration | [U2FRegistrationData](#services.U2FRegistrationData) |  | U2FRegistration holds Universal Second Factor registration info. |
| U2FCounter | [uint32](#uint32) |  | U2FCounter holds the highest seen Universal Second Factor registration count. |






<a name="services.LoginStatus"></a>

### LoginStatus
LoginStatus is a login status of the user


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| IsLocked | [bool](#bool) |  | IsLocked tells us if user is locked |
| LockedMessage | [string](#string) |  | LockedMessage contains the message in case if user is locked |
| LockedTime | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | LockedTime contains time when user was locked |
| LockExpires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | LockExpires contains time when this lock will expire |






<a name="services.Metadata"></a>

### Metadata
Metadata is resource metadata


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is an object name |
| Namespace | [string](#string) |  | Namespace is object namespace. The field should be called &#34;namespace&#34; when it returns in Teleport 2.4. |
| Description | [string](#string) |  | Description is object description |
| Labels | [Metadata.LabelsEntry](#services.Metadata.LabelsEntry) | repeated | Labels is a set of labels |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is a global expiry time header can be set on any resource in the system. |
| ID | [int64](#int64) |  | ID is a record ID |






<a name="services.Metadata.LabelsEntry"></a>

### Metadata.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="services.Namespace"></a>

### Namespace
Namespace represents namespace resource specification


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [NamespaceSpec](#services.NamespaceSpec) |  | Spec is a namespace spec |






<a name="services.NamespaceSpec"></a>

### NamespaceSpec
NamespaceSpec is a namespace specificateion






<a name="services.PluginDataEntry"></a>

### PluginDataEntry
PluginDataEntry wraps a mapping of arbitrary string values used by
plugins to store per-resource information.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Data | [PluginDataEntry.DataEntry](#services.PluginDataEntry.DataEntry) | repeated | Data is a mapping of arbitrary string values. |






<a name="services.PluginDataEntry.DataEntry"></a>

### PluginDataEntry.DataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="services.PluginDataFilter"></a>

### PluginDataFilter
PluginDataFilter encodes filter params for plugin data.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is the kind of resource that the target plugin data is associated with. |
| Resource | [string](#string) |  | Resource matches a specific resource name if set. |
| Plugin | [string](#string) |  | Plugin matches a specific plugin name if set. |






<a name="services.PluginDataSpecV3"></a>

### PluginDataSpecV3
PluginData stores a collection of values associated with a specific resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Entries | [PluginDataSpecV3.EntriesEntry](#services.PluginDataSpecV3.EntriesEntry) | repeated | Entries is a collection of PluginData values organized by plugin name. |






<a name="services.PluginDataSpecV3.EntriesEntry"></a>

### PluginDataSpecV3.EntriesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [PluginDataEntry](#services.PluginDataEntry) |  |  |






<a name="services.PluginDataUpdateParams"></a>

### PluginDataUpdateParams
PluginDataUpdateParams encodes paramers for updating a PluginData field.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is the kind of resource that the target plugin data is associated with. |
| Resource | [string](#string) |  | Resource indicates the name of the target resource. |
| Plugin | [string](#string) |  | Plugin is the name of the plugin that owns the data. |
| Set | [PluginDataUpdateParams.SetEntry](#services.PluginDataUpdateParams.SetEntry) | repeated | Set indicates the fields which should be set by this operation. |
| Expect | [PluginDataUpdateParams.ExpectEntry](#services.PluginDataUpdateParams.ExpectEntry) | repeated | Expect optionally indicates the expected state of fields prior to this update. |






<a name="services.PluginDataUpdateParams.ExpectEntry"></a>

### PluginDataUpdateParams.ExpectEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="services.PluginDataUpdateParams.SetEntry"></a>

### PluginDataUpdateParams.SetEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="services.PluginDataV3"></a>

### PluginDataV3
PluginData stores a collection of values associated with a specific resource.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is PluginData metadata |
| Spec | [PluginDataSpecV3](#services.PluginDataSpecV3) |  | Spec is a PluginData specification |






<a name="services.ProvisionTokenSpecV2"></a>

### ProvisionTokenSpecV2
ProvisionTokenSpecV2 is a specification for V2 token


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Roles | [string](#string) | repeated | Roles is a list of roles associated with the token, that will be converted to metadata in the SSH and X509 certificates issued to the user of the token |






<a name="services.ProvisionTokenV1"></a>

### ProvisionTokenV1
ProvisionTokenV1 is a provisioning token V1


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Roles | [string](#string) | repeated | Roles is a list of roles associated with the token, that will be converted to metadata in the SSH and X509 certificates issued to the user of the token |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is a global expiry time header can be set on any resource in the system. |
| Token | [string](#string) |  | Token is a token name |






<a name="services.ProvisionTokenV2"></a>

### ProvisionTokenV2
ProvisionTokenV2 specifies provisioning token


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [ProvisionTokenSpecV2](#services.ProvisionTokenSpecV2) |  | Spec is a provisioning token V2 spec |






<a name="services.RemoteClusterStatusV3"></a>

### RemoteClusterStatusV3
RemoteClusterStatusV3 represents status of the remote cluster


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Connection | [string](#string) |  | Connection represents connection status, online or offline |
| LastHeartbeat | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | LastHeartbeat records last heartbeat of the cluster |






<a name="services.RemoteClusterV3"></a>

### RemoteClusterV3
RemoteClusterV3 represents remote cluster resource specification


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is resource API version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Status | [RemoteClusterStatusV3](#services.RemoteClusterStatusV3) |  | Status is a remote cluster status |






<a name="services.ResetPasswordTokenSecretsSpecV3"></a>

### ResetPasswordTokenSecretsSpecV3



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| OTPKey | [string](#string) |  | OTPKey is is a secret value of one time password secret generator |
| QRCode | [string](#string) |  | OTPKey is is a secret value of one time password secret generator |
| Created | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Created holds information about when the token was created |






<a name="services.ResetPasswordTokenSecretsV3"></a>

### ResetPasswordTokenSecretsV3



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is ResetPasswordTokenSecrets metadata |
| Spec | [ResetPasswordTokenSecretsSpecV3](#services.ResetPasswordTokenSecretsSpecV3) |  | Spec is an ResetPasswordTokenSecrets specification |






<a name="services.ResetPasswordTokenSpecV3"></a>

### ResetPasswordTokenSpecV3



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| User | [string](#string) |  | User is user name associated with this token |
| URL | [string](#string) |  | URL is this token URL |
| Created | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Created holds information about when the token was created |






<a name="services.ResetPasswordTokenV3"></a>

### ResetPasswordTokenV3



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is ResetPasswordToken metadata |
| Spec | [ResetPasswordTokenSpecV3](#services.ResetPasswordTokenSpecV3) |  | Spec is an ResetPasswordToken specification |






<a name="services.ResourceHeader"></a>

### ResourceHeader
ResorceHeader is a shared resource header
used in cases when only type and name is known


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |






<a name="services.ReverseTunnelSpecV2"></a>

### ReverseTunnelSpecV2
ReverseTunnelSpecV2 is a specification for V2 reverse tunnel


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ClusterName | [string](#string) |  | ClusterName is a domain name of remote cluster we are connecting to |
| DialAddrs | [string](#string) | repeated | DialAddrs is a list of remote address to establish a connection to it&#39;s always SSH over TCP |
| Type | [string](#string) |  | Type is the type of reverse tunnel, either proxy or node. |






<a name="services.ReverseTunnelV2"></a>

### ReverseTunnelV2
ReverseTunnelV2 is version 2 of the resource spec of the reverse tunnel


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is a resource metadata |
| Spec | [ReverseTunnelSpecV2](#services.ReverseTunnelSpecV2) |  | Spec is a reverse tunnel specification |






<a name="services.Rewrite"></a>

### Rewrite
Rewrite is a list of rewriting rules to apply to requests and responses.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Redirect | [string](#string) | repeated | Redirect defines a list of hosts which will be rewritten to the public address of the application if they occur in the &#34;Location&#34; header. |






<a name="services.RoleConditions"></a>

### RoleConditions
RoleConditions is a set of conditions that must all match to be allowed or
denied access.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Logins | [string](#string) | repeated | Logins is a list of *nix system logins. |
| Namespaces | [string](#string) | repeated | Namespaces is a list of namespaces (used to partition a cluster). The field should be called &#34;namespaces&#34; when it returns in Teleport 2.4. |
| NodeLabels | [wrappers.LabelValues](#wrappers.LabelValues) |  | NodeLabels is a map of node labels (used to dynamically grant access to nodes). |
| Rules | [Rule](#services.Rule) | repeated | Rules is a list of rules and their access levels. Rules are a high level construct used for access control. |
| KubeGroups | [string](#string) | repeated | KubeGroups is a list of kubernetes groups |
| Request | [AccessRequestConditions](#services.AccessRequestConditions) |  |  |
| KubeUsers | [string](#string) | repeated | KubeUsers is an optional kubernetes users to impersonate |
| AppLabels | [wrappers.LabelValues](#wrappers.LabelValues) |  | AppLabels is a map of labels used as part of the RBAC system. |
| ClusterLabels | [wrappers.LabelValues](#wrappers.LabelValues) |  | ClusterLabels is a map of node labels (used to dynamically grant access to clusters). |
| KubernetesLabels | [wrappers.LabelValues](#wrappers.LabelValues) |  | KubernetesLabels is a map of kubernetes cluster labels used for RBAC. |






<a name="services.RoleMapping"></a>

### RoleMapping
RoleMappping provides mapping of remote roles to local roles
for trusted clusters


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Remote | [string](#string) |  | Remote specifies remote role name to map from |
| Local | [string](#string) | repeated | Local specifies local roles to map to |






<a name="services.RoleOptions"></a>

### RoleOptions
RoleOptions is a set of role options


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ForwardAgent | [bool](#bool) |  | ForwardAgent is SSH agent forwarding. |
| MaxSessionTTL | [int64](#int64) |  | MaxSessionTTL defines how long a SSH session can last for. |
| PortForwarding | [BoolValue](#services.BoolValue) |  | PortForwarding defines if the certificate will have &#34;permit-port-forwarding&#34; in the certificate. PortForwarding is &#34;yes&#34; if not set, that&#39;s why this is a pointer |
| CertificateFormat | [string](#string) |  | CertificateFormat defines the format of the user certificate to allow compatibility with older versions of OpenSSH. |
| ClientIdleTimeout | [int64](#int64) |  | ClientIdleTimeout sets disconnect clients on idle timeout behavior, if set to 0 means do not disconnect, otherwise is set to the idle duration. |
| DisconnectExpiredCert | [bool](#bool) |  | DisconnectExpiredCert sets disconnect clients on expired certificates. |
| BPF | [string](#string) | repeated | BPF defines what events to record for the BPF-based session recorder. |
| PermitX11Forwarding | [bool](#bool) |  | PermitX11Forwarding authorizes use of X11 forwarding. |
| MaxConnections | [int64](#int64) |  | MaxConnections defines the maximum number of concurrent connections a user may hold. |
| MaxSessions | [int64](#int64) |  | MaxSessions defines the maximum number of concurrent sessions per connection. |
| RequestAccess | [string](#string) |  | RequestAccess defines the access request stategy (optional|note|always) where optional is the default. |
| RequestPrompt | [string](#string) |  | RequestPrompt is an optional message which tells users what they aught to |






<a name="services.RoleSpecV3"></a>

### RoleSpecV3
RoleSpecV3 is role specification for RoleV3.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Options | [RoleOptions](#services.RoleOptions) |  | Options is for OpenSSH options like agent forwarding. |
| Allow | [RoleConditions](#services.RoleConditions) |  | Allow is the set of conditions evaluated to grant access. |
| Deny | [RoleConditions](#services.RoleConditions) |  | Deny is the set of conditions evaluated to deny access. Deny takes priority over allow. |






<a name="services.RoleV3"></a>

### RoleV3
RoleV3 represents role resource specification


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [RoleSpecV3](#services.RoleSpecV3) |  | Spec is a role specification |






<a name="services.Rotation"></a>

### Rotation
Rotation is a status of the rotation of the certificate authority


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| State | [string](#string) |  | State could be one of &#34;init&#34; or &#34;in_progress&#34;. |
| Phase | [string](#string) |  | Phase is the current rotation phase. |
| Mode | [string](#string) |  | Mode sets manual or automatic rotation mode. |
| CurrentID | [string](#string) |  | CurrentID is the ID of the rotation operation to differentiate between rotation attempts. |
| Started | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Started is set to the time when rotation has been started in case if the state of the rotation is &#34;in_progress&#34;. |
| GracePeriod | [int64](#int64) |  | GracePeriod is a period during which old and new CA are valid for checking purposes, but only new CA is issuing certificates. |
| LastRotated | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | LastRotated specifies the last time of the completed rotation. |
| Schedule | [RotationSchedule](#services.RotationSchedule) |  | Schedule is a rotation schedule - used in automatic mode to switch beetween phases. |






<a name="services.RotationSchedule"></a>

### RotationSchedule
RotationSchedule is a rotation schedule setting time switches
for different phases.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| UpdateClients | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | UpdateClients specifies time to switch to the &#34;Update clients&#34; phase |
| UpdateServers | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | UpdateServers specifies time to switch to the &#34;Update servers&#34; phase. |
| Standby | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Standby specifies time to switch to the &#34;Standby&#34; phase. |






<a name="services.Rule"></a>

### Rule
Rule represents allow or deny rule that is executed to check
if user or service have access to resource


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Resources | [string](#string) | repeated | Resources is a list of resources |
| Verbs | [string](#string) | repeated | Verbs is a list of verbs |
| Where | [string](#string) |  | Where specifies optional advanced matcher |
| Actions | [string](#string) | repeated | Actions specifies optional actions taken when this rule matches |






<a name="services.SemaphoreFilter"></a>

### SemaphoreFilter
SemaphoreFilter encodes semaphore filtering params.
A semaphore filter matches a semaphore if all nonzero fields
match the corresponding semaphore fileds (e.g. a filter which
specifies only `kind=foo` would match all semaphores of
kind `foo`).


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SemaphoreKind | [string](#string) |  | SemaphoreKind is the kind of the semaphore. |
| SemaphoreName | [string](#string) |  | SemaphoreName is the name of the semaphore. |






<a name="services.SemaphoreLease"></a>

### SemaphoreLease
SemaphoreLease represents lease acquired for semaphore


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SemaphoreKind | [string](#string) |  | SemaphoreKind is the kind of the semaphore. |
| SemaphoreName | [string](#string) |  | SemaphoreName is the name of the semaphore. |
| LeaseID | [string](#string) |  | LeaseID uniquely identifies this lease. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is the time at which this lease expires. |






<a name="services.SemaphoreLeaseRef"></a>

### SemaphoreLeaseRef
SemaphoreLeaseRef identifies an existent lease.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| LeaseID | [string](#string) |  | LeaseID is the unique ID of the lease. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is the time at which the lease expires. |
| Holder | [string](#string) |  | Holder identifies the lease holder. |






<a name="services.SemaphoreSpecV3"></a>

### SemaphoreSpecV3
SemaphoreSpecV3 contains the data about lease


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Leases | [SemaphoreLeaseRef](#services.SemaphoreLeaseRef) | repeated | Leases is a list of all currently acquired leases. |






<a name="services.SemaphoreV3"></a>

### SemaphoreV3
SemaphoreV3 implements Semaphore interface


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is Semaphore metadata |
| Spec | [SemaphoreSpecV3](#services.SemaphoreSpecV3) |  | Spec is a lease V3 spec |






<a name="services.ServerSpecV2"></a>

### ServerSpecV2
ServerSpecV2 is a specification for V2 Server


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Addr | [string](#string) |  | Addr is server host:port address |
| PublicAddr | [string](#string) |  | PublicAddr is the public address this cluster can be reached at. |
| Hostname | [string](#string) |  | Hostname is server hostname |
| CmdLabels | [ServerSpecV2.CmdLabelsEntry](#services.ServerSpecV2.CmdLabelsEntry) | repeated | CmdLabels is server dynamic labels |
| Rotation | [Rotation](#services.Rotation) |  | Rotation specifies server rotation |
| UseTunnel | [bool](#bool) |  | UseTunnel indicates that connections to this server should occur over a reverse tunnel. |
| Version | [string](#string) |  | TeleportVersion is the teleport version that the server is running on |
| Apps | [App](#services.App) | repeated | Apps is a list of applications this server is proxying. |
| KubernetesClusters | [KubernetesCluster](#services.KubernetesCluster) | repeated | KubernetesClusters is a list of kubernetes clusters provided by this Proxy or KubeService server.

Important: jsontag must not be &#34;kubernetes_clusters&#34;, because a different field with that jsontag existed in 4.4: https://github.com/gravitational/teleport/issues/4862 |






<a name="services.ServerSpecV2.CmdLabelsEntry"></a>

### ServerSpecV2.CmdLabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [CommandLabelV2](#services.CommandLabelV2) |  |  |






<a name="services.ServerV2"></a>

### ServerV2
ServerV2 represents a Node, App, Proxy or Auth server in a Teleport cluster.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [ServerSpecV2](#services.ServerSpecV2) |  | Spec is a server spec |






<a name="services.StaticTokensSpecV2"></a>

### StaticTokensSpecV2
StaticTokensSpecV2 is the actual data we care about for StaticTokensSpecV2.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| StaticTokens | [ProvisionTokenV1](#services.ProvisionTokenV1) | repeated | StaticTokens is a list of tokens that can be used to add nodes to the cluster. |






<a name="services.StaticTokensV2"></a>

### StaticTokensV2
StaticTokensV2 implements the StaticTokens interface.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [StaticTokensSpecV2](#services.StaticTokensSpecV2) |  | Spec is a provisioning token V2 spec |






<a name="services.TLSKeyPair"></a>

### TLSKeyPair
TLSKeyPair is a TLS key pair


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Cert | [bytes](#bytes) |  | Cert is a PEM encoded TLS cert |
| Key | [bytes](#bytes) |  | Key is a PEM encoded TLS key |






<a name="services.TunnelConnectionSpecV2"></a>

### TunnelConnectionSpecV2
TunnelConnectionSpecV2 is a specification for V2 tunnel connection


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ClusterName | [string](#string) |  | ClusterName is a name of the cluster |
| ProxyName | [string](#string) |  | ProxyName is the name of the proxy server |
| LastHeartbeat | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | LastHeartbeat is a time of the last heartbeat |
| Type | [string](#string) |  | Type is the type of reverse tunnel, either proxy or node. |






<a name="services.TunnelConnectionV2"></a>

### TunnelConnectionV2
TunnelConnectionV2 is version 2 of the resource spec of the tunnel connection


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is a resource metadata |
| Spec | [TunnelConnectionSpecV2](#services.TunnelConnectionSpecV2) |  | Spec is a tunnel specification |






<a name="services.U2FRegistrationData"></a>

### U2FRegistrationData
U2FRegistrationData encodes the universal second factor registration payload.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Raw | [bytes](#bytes) |  | Raw is the serialized registration data as received from the token |
| KeyHandle | [bytes](#bytes) |  | KeyHandle uniquely identifies a key on a device |
| PubKey | [bytes](#bytes) |  | PubKey is an DER encoded ecdsa public key |






<a name="services.UserRef"></a>

### UserRef
UserRef holds references to user


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is name of the user |






<a name="services.UserSpecV2"></a>

### UserSpecV2
UserSpecV2 is a specification for V2 user


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| OIDCIdentities | [ExternalIdentity](#services.ExternalIdentity) | repeated | OIDCIdentities lists associated OpenID Connect identities that let user log in using externally verified identity |
| SAMLIdentities | [ExternalIdentity](#services.ExternalIdentity) | repeated | SAMLIdentities lists associated SAML identities that let user log in using externally verified identity |
| GithubIdentities | [ExternalIdentity](#services.ExternalIdentity) | repeated | GithubIdentities list associated Github OAuth2 identities that let user log in using externally verified identity |
| Roles | [string](#string) | repeated | Roles is a list of roles assigned to user |
| Traits | [wrappers.LabelValues](#wrappers.LabelValues) |  | Traits are key/value pairs received from an identity provider (through OIDC claims or SAML assertions) or from a system administrator for local accounts. Traits are used to populate role variables. |
| Status | [LoginStatus](#services.LoginStatus) |  | Status is a login status of the user |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires if set sets TTL on the user |
| CreatedBy | [CreatedBy](#services.CreatedBy) |  | CreatedBy holds information about agent or person created this user |
| LocalAuth | [LocalAuthSecrets](#services.LocalAuthSecrets) |  | LocalAuths hold sensitive data necessary for performing local authentication |






<a name="services.UserV2"></a>

### UserV2
UserV2 is version 2 resource spec of the user


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources |
| Version | [string](#string) |  | Version is version |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is User metadata |
| Spec | [UserSpecV2](#services.UserSpecV2) |  | Spec is a user specification |






<a name="services.WebSessionSpecV2"></a>

### WebSessionSpecV2
WebSessionSpecV2 is a specification for web session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| User | [string](#string) |  | User is the identity of the user to which the web session belongs. |
| Pub | [bytes](#bytes) |  | Pub is the SSH certificate for the user. |
| Priv | [bytes](#bytes) |  | Priv is the SSH private key for the user. |
| TLSCert | [bytes](#bytes) |  | TLSCert is the TLS certificate for the user. |
| BearerToken | [string](#string) |  | BearerToken is a token that is paired with the session cookie for authentication. It is periodically rotated so a stolen cookie itself is not enough to steal a session. In addition it is used for CSRF mitigation. |
| BearerTokenExpires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | BearerTokenExpires is the absolute time when the token expires. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is the absolute time when the session expires. |






<a name="services.WebSessionV2"></a>

### WebSessionV2
WebSessionV2 represents an application or UI web session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind. |
| SubKind | [string](#string) |  | SubKind is an optional resource sub kind, used in some resources. |
| Version | [string](#string) |  | Version is version. |
| Metadata | [Metadata](#services.Metadata) |  | Metadata is a resource metadata. |
| Spec | [WebSessionSpecV2](#services.WebSessionSpecV2) |  | Spec is a tunnel specification. |





 


<a name="services.CertAuthoritySpecV2.SigningAlgType"></a>

### CertAuthoritySpecV2.SigningAlgType
SigningAlg is the algorithm used for signing new SSH certificates using
SigningKeys.

| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| RSA_SHA1 | 1 |  |
| RSA_SHA2_256 | 2 |  |
| RSA_SHA2_512 | 3 |  |



<a name="services.KeepAlive.KeepAliveType"></a>

### KeepAlive.KeepAliveType
Type is the type of keep alive, used by servers. At the moment only
&#34;node&#34; and &#34;app&#34; are supported.

| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| NODE | 1 |  |
| APP | 2 |  |



<a name="services.RequestState"></a>

### RequestState
RequestState represents the state of a request for escalated privilege.

| Name | Number | Description |
| ---- | ------ | ----------- |
| NONE | 0 | NONE variant exists to allow RequestState to be explicitly omitted in certain circumstances (e.g. in an AccessRequestFilter). |
| PENDING | 1 | PENDING variant is the default for newly created requests. |
| APPROVED | 2 | APPROVED variant indicates that a request has been accepted by an administrating party. |
| DENIED | 3 | DENIED variant indicates that a request has been rejected by an administrating party. |


 

 

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

