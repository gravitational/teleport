# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [auth.proto](#auth.proto)
    - [AccessRequests](#auth.AccessRequests)
    - [AuditStreamRequest](#auth.AuditStreamRequest)
    - [AuditStreamStatus](#auth.AuditStreamStatus)
    - [Certs](#auth.Certs)
    - [CompleteStream](#auth.CompleteStream)
    - [CreateAppSessionRequest](#auth.CreateAppSessionRequest)
    - [CreateAppSessionResponse](#auth.CreateAppSessionResponse)
    - [CreateResetPasswordTokenRequest](#auth.CreateResetPasswordTokenRequest)
    - [CreateStream](#auth.CreateStream)
    - [DeleteAllAppServersRequest](#auth.DeleteAllAppServersRequest)
    - [DeleteAllKubeServicesRequest](#auth.DeleteAllKubeServicesRequest)
    - [DeleteAppServerRequest](#auth.DeleteAppServerRequest)
    - [DeleteAppSessionRequest](#auth.DeleteAppSessionRequest)
    - [DeleteKubeServiceRequest](#auth.DeleteKubeServiceRequest)
    - [DeleteUserRequest](#auth.DeleteUserRequest)
    - [Event](#auth.Event)
    - [FlushAndCloseStream](#auth.FlushAndCloseStream)
    - [GenerateAppTokenRequest](#auth.GenerateAppTokenRequest)
    - [GenerateAppTokenResponse](#auth.GenerateAppTokenResponse)
    - [GetAppServersRequest](#auth.GetAppServersRequest)
    - [GetAppServersResponse](#auth.GetAppServersResponse)
    - [GetAppSessionRequest](#auth.GetAppSessionRequest)
    - [GetAppSessionResponse](#auth.GetAppSessionResponse)
    - [GetAppSessionsResponse](#auth.GetAppSessionsResponse)
    - [GetKubeServicesRequest](#auth.GetKubeServicesRequest)
    - [GetKubeServicesResponse](#auth.GetKubeServicesResponse)
    - [GetResetPasswordTokenRequest](#auth.GetResetPasswordTokenRequest)
    - [GetUserRequest](#auth.GetUserRequest)
    - [GetUsersRequest](#auth.GetUsersRequest)
    - [PingRequest](#auth.PingRequest)
    - [PingResponse](#auth.PingResponse)
    - [PluginDataSeq](#auth.PluginDataSeq)
    - [RequestID](#auth.RequestID)
    - [RequestStateSetter](#auth.RequestStateSetter)
    - [ResumeStream](#auth.ResumeStream)
    - [RotateResetPasswordTokenSecretsRequest](#auth.RotateResetPasswordTokenSecretsRequest)
    - [Semaphores](#auth.Semaphores)
    - [UpsertAppServerRequest](#auth.UpsertAppServerRequest)
    - [UpsertKubeServiceRequest](#auth.UpsertKubeServiceRequest)
    - [UserCertsRequest](#auth.UserCertsRequest)
    - [Watch](#auth.Watch)
    - [WatchKind](#auth.WatchKind)
    - [WatchKind.FilterEntry](#auth.WatchKind.FilterEntry)
  
    - [Operation](#auth.Operation)
  
    - [AuthService](#auth.AuthService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="auth.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## auth.proto



<a name="auth.AccessRequests"></a>

### AccessRequests
AccessRequests is a collection of AccessRequest values.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| AccessRequests | [types.AccessRequestV3](#types.AccessRequestV3) | repeated |  |






<a name="auth.AuditStreamRequest"></a>

### AuditStreamRequest
AuditStreamRequest contains stream request - event or stream control request


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| CreateStream | [CreateStream](#auth.CreateStream) |  | CreateStream creates the stream for session ID should be the first message sent to the stream |
| ResumeStream | [ResumeStream](#auth.ResumeStream) |  | ResumeStream resumes existing stream, should be the first message sent to the stream |
| CompleteStream | [CompleteStream](#auth.CompleteStream) |  | CompleteStream completes the stream |
| FlushAndCloseStream | [FlushAndCloseStream](#auth.FlushAndCloseStream) |  | FlushAndClose flushes and closes the stream |
| Event | [events.OneOf](#events.OneOf) |  | Event contains the stream event |






<a name="auth.AuditStreamStatus"></a>

### AuditStreamStatus
AuditStreamStatus returns audit stream status
with corresponding upload ID


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| UploadID | [string](#string) |  | UploadID is upload ID associated with the stream, can be used to resume the stream |






<a name="auth.Certs"></a>

### Certs
Set of certificates corresponding to a single public key.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SSH | [bytes](#bytes) |  | SSH X509 cert (PEM-encoded). |
| TLS | [bytes](#bytes) |  | TLS X509 cert (PEM-encoded). |






<a name="auth.CompleteStream"></a>

### CompleteStream
CompleteStream completes the stream
and uploads it to the session server






<a name="auth.CreateAppSessionRequest"></a>

### CreateAppSessionRequest
CreateAppSessionRequest contains the parameters to request a application web session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Username | [string](#string) |  | Username is the name of the user requesting the session. |
| ParentSession | [string](#string) |  | ParentSession is the session ID of the parent session. |
| PublicAddr | [string](#string) |  | PublicAddr is the public address the application. |
| ClusterName | [string](#string) |  | ClusterName is cluster within which the application is running. |






<a name="auth.CreateAppSessionResponse"></a>

### CreateAppSessionResponse
CreateAppSessionResponse contains the requested application web session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Session | [types.WebSessionV2](#types.WebSessionV2) |  | Session is the application web session. |






<a name="auth.CreateResetPasswordTokenRequest"></a>

### CreateResetPasswordTokenRequest
CreateResetPasswordTokenRequest is a request to create an instance of
ResetPasswordToken


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is the user name. |
| Type | [string](#string) |  | Type is a token type. |
| TTL | [int64](#int64) |  | TTL specifies how long the generated token is valid for. |






<a name="auth.CreateStream"></a>

### CreateStream
CreateStream creates stream for a new session ID


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SessionID | [string](#string) |  |  |






<a name="auth.DeleteAllAppServersRequest"></a>

### DeleteAllAppServersRequest
DeleteAllAppServersRequest are the parameters used to remove all applications.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Namespace | [string](#string) |  | Namespace is the namespace for application. |






<a name="auth.DeleteAllKubeServicesRequest"></a>

### DeleteAllKubeServicesRequest
DeleteAllKubeServicesRequest are the parameters used to remove all kubernetes types.






<a name="auth.DeleteAppServerRequest"></a>

### DeleteAppServerRequest
DeleteAppServerRequest are the parameters used to remove an application.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Namespace | [string](#string) |  | Namespace is the namespace for application. |
| Name | [string](#string) |  | Name is the name of the application to delete. |






<a name="auth.DeleteAppSessionRequest"></a>

### DeleteAppSessionRequest
DeleteAppSessionRequest contains the parameters used to remove an application web session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SessionID | [string](#string) |  |  |






<a name="auth.DeleteKubeServiceRequest"></a>

### DeleteKubeServiceRequest
DeleteKubeServiceRequest are the parameters used to remove a kubernetes service.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is the name of the kubernetes service to delete. |






<a name="auth.DeleteUserRequest"></a>

### DeleteUserRequest
DeleteUserRequest is the input value for the DeleteUser method.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is the user name to delete. |






<a name="auth.Event"></a>

### Event
Event returns cluster event


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Type | [Operation](#auth.Operation) |  | Operation identifies operation |
| ResourceHeader | [types.ResourceHeader](#types.ResourceHeader) |  | ResourceHeader is specified in delete events, the full object is not available, so resource header is used to provide information about object type |
| CertAuthority | [types.CertAuthorityV2](#types.CertAuthorityV2) |  | CertAuthority is filled in certificate-authority related events |
| StaticTokens | [types.StaticTokensV2](#types.StaticTokensV2) |  | StaticTokens is filled in static-tokens related events |
| ProvisionToken | [types.ProvisionTokenV2](#types.ProvisionTokenV2) |  | ProvisionToken is filled in provision-token related events |
| ClusterName | [types.ClusterNameV2](#types.ClusterNameV2) |  | ClusterNameV2 is a cluster name resource |
| ClusterConfig | [types.ClusterConfigV3](#types.ClusterConfigV3) |  | ClusterConfig is a cluster configuration resource |
| User | [types.UserV2](#types.UserV2) |  | User is a user resource |
| Role | [types.RoleV3](#types.RoleV3) |  | Role is a role resource |
| Namespace | [types.Namespace](#types.Namespace) |  | Namespace is a namespace resource |
| Server | [types.ServerV2](#types.ServerV2) |  | Server is a node or proxy resource |
| ReverseTunnel | [types.ReverseTunnelV2](#types.ReverseTunnelV2) |  | ReverseTunnel is a resource with reverse tunnel |
| TunnelConnection | [types.TunnelConnectionV2](#types.TunnelConnectionV2) |  | TunnelConnection is a resource for tunnel connnections |
| AccessRequest | [types.AccessRequestV3](#types.AccessRequestV3) |  | AccessRequest is a resource for access requests |
| AppSession | [types.WebSessionV2](#types.WebSessionV2) |  | AppSession is an application web session. |
| RemoteCluster | [types.RemoteClusterV3](#types.RemoteClusterV3) |  | RemoteCluster is a resource for remote clusters |






<a name="auth.FlushAndCloseStream"></a>

### FlushAndCloseStream
FlushAndCloseStream flushes the stream data and closes the stream






<a name="auth.GenerateAppTokenRequest"></a>

### GenerateAppTokenRequest
GenerateAppTokenRequest are the parameters used to request an application
token.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Username | [string](#string) |  | Username is the Teleport username. |
| Roles | [string](#string) | repeated | Roles is a list of Teleport roles assigned to the user. |
| URI | [string](#string) |  | URI is the URI of the application this token is targeting. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is the time this token expires. |






<a name="auth.GenerateAppTokenResponse"></a>

### GenerateAppTokenResponse
GenerateAppTokenResponse contains a signed application token.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Token | [string](#string) |  |  |






<a name="auth.GetAppServersRequest"></a>

### GetAppServersRequest
GetAppServersRequest are the parameters used to request application servers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Namespace | [string](#string) |  | Namespace is the namespace for application. |
| SkipValidation | [bool](#bool) |  | SkipValidation is used to skip JSON schema validation. |






<a name="auth.GetAppServersResponse"></a>

### GetAppServersResponse
GetAppServersResponse contains all requested application servers.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Servers | [types.ServerV2](#types.ServerV2) | repeated | Servers is a slice of types.Server that represent applications. |






<a name="auth.GetAppSessionRequest"></a>

### GetAppSessionRequest
GetAppSessionRequest are the parameters used to request an application web session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SessionID | [string](#string) |  | SessionID is the ID of the session being requested. |






<a name="auth.GetAppSessionResponse"></a>

### GetAppSessionResponse
GetAppSessionResponse contains the requested application web session.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Session | [types.WebSessionV2](#types.WebSessionV2) |  | Session is the application web session. |






<a name="auth.GetAppSessionsResponse"></a>

### GetAppSessionsResponse
GetAppSessionsResponse contains all the requested application web sessions.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Sessions | [types.WebSessionV2](#types.WebSessionV2) | repeated | Sessions is a list of application web sessions. |






<a name="auth.GetKubeServicesRequest"></a>

### GetKubeServicesRequest
GetKubeServicesRequest are the parameters used to request kubernetes types.






<a name="auth.GetKubeServicesResponse"></a>

### GetKubeServicesResponse
GetKubeServicesResponse contains all requested kubernetes types.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Servers | [types.ServerV2](#types.ServerV2) | repeated | Servers is a slice of types.Server that represent kubernetes types. |






<a name="auth.GetResetPasswordTokenRequest"></a>

### GetResetPasswordTokenRequest
GetResetPasswordTokenRequest is a request to get a reset password token.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| TokenID | [string](#string) |  |  |






<a name="auth.GetUserRequest"></a>

### GetUserRequest
GetUserRequest specifies parameters for the GetUser method.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Name | [string](#string) |  | Name is the name of the desired user. |
| WithSecrets | [bool](#bool) |  | WithSecrets specifies whether to load associated secrets. |






<a name="auth.GetUsersRequest"></a>

### GetUsersRequest
GetUsersRequest specifies parameters for the GetUsers method.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| WithSecrets | [bool](#bool) |  | WithSecrets specifies whether to load associated secrets. |






<a name="auth.PingRequest"></a>

### PingRequest
PingRequest is the input value for the Ping method.

Ping method currently takes no parameters






<a name="auth.PingResponse"></a>

### PingResponse
PingResponse contains data about the teleport auth server.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ClusterName | [string](#string) |  | ClusterName is the name of the teleport cluster. |
| ServerVersion | [string](#string) |  | ServerVersion is the version of the auth server. |






<a name="auth.PluginDataSeq"></a>

### PluginDataSeq
PluginDataSeq is a sequence of plugin data.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| PluginData | [types.PluginDataV3](#types.PluginDataV3) | repeated |  |






<a name="auth.RequestID"></a>

### RequestID
RequestID is the unique identifier of an access request.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ID | [string](#string) |  |  |






<a name="auth.RequestStateSetter"></a>

### RequestStateSetter
RequestStateSetter encodes the paramters necessary to update the
state of a privilege escalation request.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ID | [string](#string) |  | ID is the request ID being targeted |
| State | [types.RequestState](#types.RequestState) |  | State is the desired state to be set |
| Delegator | [string](#string) |  | Delegator is an optional indicator of who delegated this state update (used by plugins to indicate which user approved or denied the request). |
| Reason | [string](#string) |  | Reason is an optional message indicating the reason for the resolution (approval, denail , etc...). |
| Annotations | [wrappers.LabelValues](#wrappers.LabelValues) |  | Annotations are key/value pairs received from plugins during request resolution. They are currently only used to provide additional logging information. |
| Roles | [string](#string) | repeated | Roles, if present, overrides the existing set of roles associated with the access request. |






<a name="auth.ResumeStream"></a>

### ResumeStream
ResumeStream resumes stream that was previously created


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| SessionID | [string](#string) |  | SessionID is a session ID of the stream |
| UploadID | [string](#string) |  | UploadID is upload ID to resume |






<a name="auth.RotateResetPasswordTokenSecretsRequest"></a>

### RotateResetPasswordTokenSecretsRequest
RotateResetPasswordTokenSecretsRequest is a request to rotate token secrets.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| TokenID | [string](#string) |  |  |






<a name="auth.Semaphores"></a>

### Semaphores
Semaphores is a sequence of Semaphore resources.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Semaphores | [types.SemaphoreV3](#types.SemaphoreV3) | repeated |  |






<a name="auth.UpsertAppServerRequest"></a>

### UpsertAppServerRequest
UpsertAppServerRequest are the parameters used to add an application.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Server | [types.ServerV2](#types.ServerV2) |  |  |






<a name="auth.UpsertKubeServiceRequest"></a>

### UpsertKubeServiceRequest
UpsertKubeServiceRequest are the parameters used to add or update a
kubernetes service.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Server | [types.ServerV2](#types.ServerV2) |  |  |






<a name="auth.UserCertsRequest"></a>

### UserCertsRequest
UserCertRequest specifies certificate-generation parameters
for a user.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| PublicKey | [bytes](#bytes) |  | PublicKey is a public key to be signed. |
| Username | [string](#string) |  | Username of key owner. |
| Expires | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | Expires is a desired time of the expiry of the certificate, could be adjusted based on the permissions |
| Format | [string](#string) |  | Format encodes the desired SSH Certificate format (either old ssh compatibility format to remove some metadata causing trouble with old SSH servers) or standard SSH cert format with custom extensions |
| RouteToCluster | [string](#string) |  | RouteToCluster is an optional cluster name to add to the certificate, so that requests originating with this certificate will be redirected to this cluster |
| AccessRequests | [string](#string) | repeated | AccessRequests is an optional list of request IDs indicating requests whose escalated privileges should be added to the certificate. |
| KubernetesCluster | [string](#string) |  | KubernetesCluster specifies the target kubernetes cluster for TLS identities. This can be empty on older Teleport clients. |






<a name="auth.Watch"></a>

### Watch
Watch specifies watch parameters


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kinds | [WatchKind](#auth.WatchKind) | repeated | Kinds specifies object kinds to watch |






<a name="auth.WatchKind"></a>

### WatchKind
WatchKind specifies resource kind to watch


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| Kind | [string](#string) |  | Kind is a resource kind to watch |
| LoadSecrets | [bool](#bool) |  | LoadSecrets specifies whether to load secrets |
| Name | [string](#string) |  | Name is an optional specific resource type to watch, if specified only the events with a specific resource name will be sent |
| Filter | [WatchKind.FilterEntry](#auth.WatchKind.FilterEntry) | repeated | Filter is an optional mapping of custom filter parameters. Valid values vary by resource kind. |






<a name="auth.WatchKind.FilterEntry"></a>

### WatchKind.FilterEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |





 


<a name="auth.Operation"></a>

### Operation
Operation identifies type of operation

| Name | Number | Description |
| ---- | ------ | ----------- |
| INIT | 0 | INIT is sent as a first sentinel event on the watch channel |
| PUT | 1 | PUT identifies created or updated object |
| DELETE | 2 | DELETE identifies deleted object |


 

 


<a name="auth.AuthService"></a>

### AuthService
AuthService is authentication/authorization service implementation

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| SendKeepAlives | [.types.KeepAlive](#types.KeepAlive) stream | [.google.protobuf.Empty](#google.protobuf.Empty) | SendKeepAlives allows node to send a stream of keep alive requests |
| WatchEvents | [Watch](#auth.Watch) | [Event](#auth.Event) stream | WatchEvents returns a new stream of cluster events |
| UpsertNode | [.types.ServerV2](#types.ServerV2) | [.types.KeepAlive](#types.KeepAlive) | UpsertNode upserts node |
| GenerateUserCerts | [UserCertsRequest](#auth.UserCertsRequest) | [Certs](#auth.Certs) | GenerateUserCerts generates a set of user certificates for use by `tctl auth sign`. |
| GetUser | [GetUserRequest](#auth.GetUserRequest) | [.types.UserV2](#types.UserV2) | GetUser gets a user resource by name. |
| GetUsers | [GetUsersRequest](#auth.GetUsersRequest) | [.types.UserV2](#types.UserV2) stream | GetUsers gets all current user resources. |
| GetAccessRequests | [.types.AccessRequestFilter](#types.AccessRequestFilter) | [AccessRequests](#auth.AccessRequests) | GetAccessRequests gets all pending access requests. |
| CreateAccessRequest | [.types.AccessRequestV3](#types.AccessRequestV3) | [.google.protobuf.Empty](#google.protobuf.Empty) | CreateAccessRequest creates a new access request. |
| DeleteAccessRequest | [RequestID](#auth.RequestID) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteAccessRequest deletes an access request. |
| SetAccessRequestState | [RequestStateSetter](#auth.RequestStateSetter) | [.google.protobuf.Empty](#google.protobuf.Empty) | SetAccessRequestState sets the state of an access request. |
| GetPluginData | [.types.PluginDataFilter](#types.PluginDataFilter) | [PluginDataSeq](#auth.PluginDataSeq) | GetPluginData gets all plugin data matching the supplied filter. |
| UpdatePluginData | [.types.PluginDataUpdateParams](#types.PluginDataUpdateParams) | [.google.protobuf.Empty](#google.protobuf.Empty) | UpdatePluginData updates a plugin&#39;s resource-specific datastore. |
| Ping | [PingRequest](#auth.PingRequest) | [PingResponse](#auth.PingResponse) | Ping gets basic info about the auth server. This method is intended to mimic the behavior of the proxy&#39;s Ping method, and may be used by clients for verification or configuration on startup. |
| RotateResetPasswordTokenSecrets | [RotateResetPasswordTokenSecretsRequest](#auth.RotateResetPasswordTokenSecretsRequest) | [.types.ResetPasswordTokenSecretsV3](#types.ResetPasswordTokenSecretsV3) | RotateResetPasswordTokenSecrets rotates token secrets for a given tokenID. |
| GetResetPasswordToken | [GetResetPasswordTokenRequest](#auth.GetResetPasswordTokenRequest) | [.types.ResetPasswordTokenV3](#types.ResetPasswordTokenV3) | GetResetPasswordToken returns a token. |
| CreateResetPasswordToken | [CreateResetPasswordTokenRequest](#auth.CreateResetPasswordTokenRequest) | [.types.ResetPasswordTokenV3](#types.ResetPasswordTokenV3) | CreateResetPasswordToken creates ResetPasswordToken. |
| CreateUser | [.types.UserV2](#types.UserV2) | [.google.protobuf.Empty](#google.protobuf.Empty) | CreateUser inserts a new user entry to a backend. |
| UpdateUser | [.types.UserV2](#types.UserV2) | [.google.protobuf.Empty](#google.protobuf.Empty) | UpdateUser updates an existing user in a backend. |
| DeleteUser | [DeleteUserRequest](#auth.DeleteUserRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteUser deletes an existing user in a backend by username. |
| AcquireSemaphore | [.types.AcquireSemaphoreRequest](#types.AcquireSemaphoreRequest) | [.types.SemaphoreLease](#types.SemaphoreLease) | AcquireSemaphore acquires lease with requested resources from semaphore. |
| KeepAliveSemaphoreLease | [.types.SemaphoreLease](#types.SemaphoreLease) | [.google.protobuf.Empty](#google.protobuf.Empty) | KeepAliveSemaphoreLease updates semaphore lease. |
| CancelSemaphoreLease | [.types.SemaphoreLease](#types.SemaphoreLease) | [.google.protobuf.Empty](#google.protobuf.Empty) | CancelSemaphoreLease cancels semaphore lease early. |
| GetSemaphores | [.types.SemaphoreFilter](#types.SemaphoreFilter) | [Semaphores](#auth.Semaphores) | GetSemaphores returns a list of all semaphores matching the supplied filter. |
| DeleteSemaphore | [.types.SemaphoreFilter](#types.SemaphoreFilter) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteSemaphore deletes a semaphore matching the supplied filter. |
| EmitAuditEvent | [.events.OneOf](#events.OneOf) | [.google.protobuf.Empty](#google.protobuf.Empty) | EmitAuditEvent emits audit event |
| CreateAuditStream | [AuditStreamRequest](#auth.AuditStreamRequest) stream | [.events.StreamStatus](#events.StreamStatus) stream | CreateAuditStream creates or resumes audit events streams |
| GetAppServers | [GetAppServersRequest](#auth.GetAppServersRequest) | [GetAppServersResponse](#auth.GetAppServersResponse) | GetAppServers gets all application servers. |
| UpsertAppServer | [UpsertAppServerRequest](#auth.UpsertAppServerRequest) | [.types.KeepAlive](#types.KeepAlive) | UpsertAppServer adds an application server. |
| DeleteAppServer | [DeleteAppServerRequest](#auth.DeleteAppServerRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteAppServer removes an application server. |
| DeleteAllAppServers | [DeleteAllAppServersRequest](#auth.DeleteAllAppServersRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteAllAppServers removes all application servers. |
| GenerateAppToken | [GenerateAppTokenRequest](#auth.GenerateAppTokenRequest) | [GenerateAppTokenResponse](#auth.GenerateAppTokenResponse) | GenerateAppToken will generate a JWT token for application access. |
| GetAppSession | [GetAppSessionRequest](#auth.GetAppSessionRequest) | [GetAppSessionResponse](#auth.GetAppSessionResponse) | GetAppSession gets an application web session. |
| GetAppSessions | [.google.protobuf.Empty](#google.protobuf.Empty) | [GetAppSessionsResponse](#auth.GetAppSessionsResponse) | GetAppSessions gets all application web sessions. |
| CreateAppSession | [CreateAppSessionRequest](#auth.CreateAppSessionRequest) | [CreateAppSessionResponse](#auth.CreateAppSessionResponse) | CreateAppSession creates an application web session. Application web sessions represent a browser session the client holds. |
| DeleteAppSession | [DeleteAppSessionRequest](#auth.DeleteAppSessionRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteAppSession removes an application web session. |
| DeleteAllAppSessions | [.google.protobuf.Empty](#google.protobuf.Empty) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteAllAppSessions removes all application web sessions. |
| UpdateRemoteCluster | [.types.RemoteClusterV3](#types.RemoteClusterV3) | [.google.protobuf.Empty](#google.protobuf.Empty) | UpdateRemoteCluster updates remote cluster |
| GetKubeServices | [GetKubeServicesRequest](#auth.GetKubeServicesRequest) | [GetKubeServicesResponse](#auth.GetKubeServicesResponse) | GetKubeServices gets all kubernetes types. |
| UpsertKubeService | [UpsertKubeServiceRequest](#auth.UpsertKubeServiceRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) | UpsertKubeService adds or updates a kubernetes service. |
| DeleteKubeService | [DeleteKubeServiceRequest](#auth.DeleteKubeServiceRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteKubeService removes a kubernetes service. |
| DeleteAllKubeServices | [DeleteAllKubeServicesRequest](#auth.DeleteAllKubeServicesRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) | DeleteAllKubeServices removes all kubernetes types. |

 



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

