---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 131 - Administrative Actions MFA

## Required Approvers

- Engineering: @r0mant && @codingllama
- Product: @klizhentas || @xinding33
- Security: @jentfoo

## What

Enforce MFA verification for any administrative actions taken. For example:

- Adding, removing, or editing Nodes and other services.
- Adding, removing, or editing users, roles, auth connectors, and other
administrative resources.
- Approving access requests.

This will apply to administrative actions performed via the WebUI, `tctl`,
and `tsh`, or Teleport Connect.

## Why

Currently, Teleport only prompts for MFA verification in a few circumstances:

1. User logs in with MFA.
2. User updates their password or registered MFA devices.
3. User connects to a service that requires Per-session MFA.

As a result, the vast majority of Teleport actions do not require timely MFA
verification. If an admin's login session is compromised, it can be used to
perform attacks pertaining to privilege escalation, security downgrades, Auth
connector changes, and other forms of cluster sabotage.

Adding an MFA restriction to administrative actions limits this attack vector
by re-verifying the user's identity promptly before performing any administrative
action that could be abused.

## Details

### Server configuration

Requiring MFA for admin actions will be made configurable through
`cluster_auth_preference.admin_action_mfa: off | on`.

When set to on, MFA will be required for admin actions for all users.

Caveat: Built in roles will not require MFA to complete admin actions. This
includes `tctl` operations performed directly on the auth server with the
built in `Admin` role.

If not set directly, this value will default to on or off based on the value of
`clsuter_auth_preference.second_factor`:

- `second_factor = off | optional` => `admin_action_mfa = off`
- `second_factor = on | otp | webauthn` => `admin_action_mfa = on`

#### Automated Use Cases

In order to support automated use casessuch as the
[Teleport Terraform Provider](https://goteleport.com/docs/management/guides/terraform-provider/)
or [Access Request Plugins](https://goteleport.com/docs/access-controls/access-request-plugins/),
we need to provide a way for specific user to bypass MFA for admin actions.

We will do this with a matching role field `admin_action_mfa`. When set, this
field will be prioritized over `cluster_auth_preference.admin_action_mfa`.
We will not recommend setting this via roles outside of necessitating use cases.

#### Backward Compatibility

Once the Auth server begins to require MFA for admin actions, old clients with
no way of providing MFA verification will fail to carry out admin actions.

In order to maintain backwards compatibility between old clients and an upgraded
server, we can't actually default `admin_action_mfa = on` as planned above.

Instead, in the first major version of this feature (likely Teleport 14), it will
always default to `off`. In the next major version (Teleport 15), the defaults
above will be used.

### Administrative Actions

An administrative action will be defined as an action which creates, updates,
or deletes a teleport resource. For example:

- `tctl rm/create/edit`
- `tctl users add/rm/reset`
- Enrolling a new resource from the WebUI (Teleport Discover)
- Modifying a role from the WebUI
- Modifying a cloud upgrade window (at https://<tenant>.teleport.sh/web/support)

To ensure this feature does not have unexpected effects, we will explicitly
define which API requests are admin actions:

gRPC:

- GenerateUserCerts, GenerateHostCerts
- UpsertClusterAlert, CreateAlertAck, ClearAlertAcks
- CreateAccessRequest, DeleteAccessRequest, SetAccessRequestState, SubmitAccessReview
- CreateBot, DeleteBot
- CreateUser, UpdateUser, DeleteUser
- CancelSemaphoreLease, DeleteSemaphore, UpsertDatabaseServer
- DeleteDatabaseServer, DeleteAllDatabaseServers, UpsertDatabaseService, DeleteDatabaseService, DeleteAllDatabaseServices
- GenerateDatabaseCert, GenerateSnowflakeJWT
- UpsertApplicationServer, DeleteApplicationServer, DeleteAllApplicationServers
- DeleteSnowflakeSession, DeleteAllSnowflakeSessions, CreateSnowflakeSession
- CreateAppSession, DeleteAppSession, DeleteAllAppSessions, DeleteUserAppSessions
- CreateSAMLIdPSession, DeleteSAMLIdPSession, DeleteAllSAMLIdPSessions, DeleteUserSAMLIdPSessions
- GenerateAppToken
- DeleteWebSession, DeleteAllWebSessions
- DeleteWebToken, DeleteAllWebTokens
- UpdateRemoteCluster
- UpsertKubernetesServer, DeleteKubernetesServer, DeleteAllKubernetesServers
- UpsertRole, DeleteRole
- GenerateUserSingleUseCerts
- UpsertOIDCConnector, DeleteOIDCConnector, CreateOIDCAuthRequest
- UpsertSAMLConnector, DeleteSAMLConnector, CreateSAMLAuthRequest
- UpsertGithubConnector, DeleteGithubConnector, CreateGithubAuthRequest
- UpsertServerInfo, DeleteServerInfo, DeleteAllServerInfos
- UpsertTrustedCluster, DeleteTrustedCluster
- UpsertToken
- CreateToken, UpsertTokenV2, CreateTokenV2, GenerateToken, DeleteToken
- UpsertNode, DeleteNode, DeleteAllNodes
- SetClusterNetworkingConfig, ResetClusterNetworkingConfig
- SetSessionRecordingConfig, ResetSessionRecordingConfig
- SetAuthPreference, ResetAuthPreference
- SetNetworkRestrictions, DeleteNetworkRestrictions
- UpsertLock, DeleteLock, ReplaceRemoteLocks
- CreateApp, UpdateApp, DeleteApp, DeleteAllApps
- CreateDatabase, UpdateDatabase, DeleteDatabase, DeleteAllDatabases
- UpsertWindowsDesktopService, DeleteWindowsDesktopService, DeleteAllWindowsDesktopServices
- CreateWindowsDesktop, UpdateWindowsDesktop, UpsertWindowsDesktop, DeleteWindowsDesktop, DeleteAllWindowsDesktops
- ChangeUserAuthentication, StartAccountRecovery, VerifyAccountRecovery, CompleteAccountRecovery, CreateAccountRecoveryCodes
- CreatePrivilegeToken, CreateRegisterChallenge
- GenerateCertAuthorityCRL
- CreateConnectionDiagnostic, UpdateConnectionDiagnostic, AppendDiagnosticTrace
- SetInstaller, DeleteInstaller, DeleteAllInstallers
- SetUIConfig, DeleteUIConfig
- CreateKubernetesCluster, UpdateKubernetesCluster, DeleteKubernetesCluster, DeleteAllKubernetesClusters
- CreateSAMLIdPServiceProvider, UpdateSAMLIdPServiceProvider, DeleteSAMLIdPServiceProvider, DeleteAllSAMLIdPServiceProviders
- CreateUserGroup, UpdateUserGroup, DeleteUserGroup, DeleteAllUserGroups
- UpdateHeadlessAuthenticationState
- ExportUpgradeWindows
- UpdateClusterMaintenanceConfig
- UpdatePluginData

HTTP:

- rotateCertAuthority, rotateExternalCertAuthority
- generateHostCert
- createWebSession, deleteWebSession
- upsertAuthServer, upsertProxy
- deleteAllProxies, deleteProxy
- upsertTunnelConnection, deleteTunnelConnection, deleteTunnelConnections, deleteAllTunnelConnections
- createRemoteCluster, deleteRemoteCluster, deleteAllRemoteClusters
- upsertReverseTunnel, deleteReverseTunnel
- validateTrustedCluster, registerUsingToken
- deleteNamespace
- setClusterName
- deleteStaticTokens, setStaticTokens
- validateGithubAuthCallback
- upsertCertAuthority, deleteCertAuthority
- deleteUser

Notable exceptions:

- User actions that are authorized based on the user owning the resource.
These requests will only require MFA when the authorization comes from the
user's role rather than the user itself. For example:
  - CreateAccessRequest, DeleteAccessRequest
  - CreateAuthenticateChallenge
  - ChangePassword, CreateResetPasswordToken
  - AddMFADeviceSync, DeleteMFADeviceSync
- Actions which only require `DefaultImplicitRole`:
  - `SubmitUsageEvent`
- Actions which are limited to internal service roles. For example:
  - CreateSessionTracker, RemoveSessionTracker, UpdateSessionTracker
  - GenerateWindowsDesktopCert, GenerateOpenSSHCert
  - InventoryControlStream
  - SendKeepAlives
  - EmitAuditEvent, CreateAuditStream, ResumeAuditStream
  - ProcessKubeCSR, SignDatabaseCSR

#### Caveat

This list is too long to thoroughly dig into all of the details and exceptions
in this RFD. Instead, we will add admin actions one at a time, grouping similar
endpoints into separate Github PRs.

Within each PR, we can ensure that every usage of that endpoint is updated in a
backwards compatible way across all Teleport clients (`tsh`, `tctl`, Teleport
Connect, Teleport Web UI, Plugins and plugin guides).

#### MVP Admin Actions Subset

Again, this list is too long to implement all changes in one go. This change
will likely take 1-2 major version cycles to complete. As such, the initial
MVP goal will be limited to the following RBAC centric endpoints:

- GenerateUserCerts, GenerateHostCerts
- CreateUser, UpdateUser, DeleteUser
- CreateToken, UpsertTokenV2, CreateTokenV2, GenerateToken, DeleteToken
- UpsertOIDCConnector, DeleteOIDCConnector, CreateOIDCAuthRequest
- UpsertSAMLConnector, DeleteSAMLConnector, CreateSAMLAuthRequest
- UpsertGithubConnector, DeleteGithubConnector, CreateGithubAuthRequest
- SetClusterNetworkingConfig, ResetClusterNetworkingConfig
- SetSessionRecordingConfig, ResetSessionRecordingConfig
- SetAuthPreference, ResetAuthPreference
- SetNetworkRestrictions, DeleteNetworkRestrictions

### UX

The UX of this feature will be very similar to Per-session MFA for `tsh`, `tctl`,
and the WebUI, and Teleport Connect.

When a user performs an admin action, they will be prompted to tap their MFA key.
Each admin action will require a single unique tap or OTP code.

```console
$ tctl rm roles/access
Tap any security key
# success
```

#### Editing resources

With `tctl edit` or the WebUI, it is possible to edit existing resources in a
text editor. In this case, MFA will be prompted before proceeding to the text
editor to prevent data loss from a failed MFA challenge.

```console
$ tctl edit roles/access
Tap any security key
# edit role in Vim
# success
```

### MFA enforcement

MFA verification will be required to carry out various administrative actions.
This will be enforced by the Auth Server API for specific endpoints which are
considered to be [administrative actions](#administrative-actions).

Essentially, this means that an administrative request to the Auth server will
follow this flow:

1. Create an Auth API client using the user's existing valid certificates.
1. Use the client to retrieve an MFA challenge for the user with
   the existing `rpc CreateAuthenticateChallenge`.
1. Prompt the user to solve the MFA challenge with their MFA device.
1. Send the resulting `MFAAuthenticateResponse` to the Auth Server as part of an
   administrative API request.
1. Validate and consume the `MFAAuthenticateResponse` in the Auth Server to
   authorize the request (in addition to normal identity-based authorization).

Steps 1-3 are already possible with Teleport currently and used for various MFA
features. Steps 4 and 5 will require some changes to the Auth API client and
server respectively.

#### Client changes

There are a few different ways that the Auth client can pass MFA verification with
the Auth server, each with their own pros and cons:

1. Add the `MFAAuthenticateResponse` argument to each administrative API endpoint.

The Auth server can consume/verify the challenge response in each request and
treat the request as MFA verified.

Pros:

- It is straightforward and matches existing endpoints which require MFA
  verification (rpc `GenerateUserSingleUseCerts`, login endpoints, etc.).
- It is clear by the API reference which endpoints require MFA verification and
  which do not, making it easier to reason from the client perspective, especially
  for external users of the client.
- The MFA verification only applies to one API request, preventing MFA-replay-like
  attacks.

Cons:

- Requires custom changes to both the client and server implementations for
  each affected endpoint, both to pass the MFA challenge response from the
  client and for the Auth server to verify it for the request.
  - This can potentially be avoided within the implementation by introducing
    a common gRPC message for the `MFAAuthenticateResponse`.

2. Allow clients to pass `MFAAuthenticateResponse` as client request metadata.

This approach is similar to passing a bearer-token in HTTP requests. Note that it
will augment the normal certificate auth flow, not replace it. Additionally, the
MFA Challenge Response will be passed to the server within the context of the TLS
handshake, so it is secure.

The client can cleanly pass the challenge response with the gRPC client call
option `PerRPCCredentials`. The Auth server can then grab the `MFAAuthenticateResponse`
from the request metadata, consume/verify the challenge response, and treat the request
as MFA verified. [POC](https://github.com/gravitational/teleport/commit/aa0a8102eccd91cff2851053fba3e1c271bdaa65).

Pros:

- Client can apply common logic for MFA verification, rather than endpoint
  specific logic.
  - Ex: First request without MFA, receive access denied error, re-request with MFA.
  - MFA verification can be extended to additional Auth endpoint without any client changes.
- Adds a new system of passing user credentials, which can be used in other
  endpoints that always or sometimes need MFA verification.
  - Ex: `GenerateUserSingleUseCerts` could be replaced by simply checking for
    MFA verification in the request metadata of `GenerateUserCerts`.
- The MFA verification only applies to one API request, preventing MFA-replay-like
  attacks.

Cons:

- Adding a new system of passing user credentials makes the client harder to
  reason about, especially for external users of the API client.

3. Reissue short-lived (1 minute TTL) MFA verified TLS certificates.

This approach is similar to Per-session MFA. The client will need to issue a
`GenerateUserSingleUseCerts` request to get MFA verified TLS certificates before
making an administrative request.

Pros:

- Client can apply common logic for MFA verification, rather than endpoint
  specific logic.
  - Ex: First request without MFA, receive access denied error, re-request with MFA.
  - MFA verification can be extended to additional Auth endpoint without any client changes.
- Easily extends to other endpoints that always or sometimes need MFA verification.
- Auth is completely handled by certificates rather than an augmented client request.

Cons:

- MFA verification applies to any requests using the MFA verified certificates.
  This can be used as a MFA-replay-like attack since only one MFA verification is
  required to carry out multiple administrative actions.
  - Like Per-session MFA, this puts the security of the system more in the hands
    of the client to not misuse or expose the certificates within their one
    minute lifetime.
- Requires users to create a new client with the reissued certificates, resulting
  in additional connections to the Auth server.

##### Option choice: 1

Option 1 and 2 both guarantee that a user's MFA verification applies to just one
admin action in the fewest number of API requests, solidifying it as a preferable
solution over option 3 for both security and implementation complexity.

Between option 1 and 2, option 1 is more explicit and simple in how and where MFA
might be required. This will make the feature easier to use and extend for both
internal and external developers.

#### Server changes

For admin actions, the Auth server will validate MFA for each request using the
`MFAAuthenticateResponse` passed in the request.

If the request fails MFA verification, an access denied error will be returned
to the client.

```go
// ErrAPIMFARequired is returned by AccessChecker when an API request
// requires an MFA check.
var ErrAdminActionMFARequired = trace.AccessDenied("administrative action requires MFA")
```

##### Infer MFA Requirement

Before making an admin action request, Teleport clients can check the server's
`cluster_auth_preference` settings with a ping request. When set to on, the
client should first make a `CreateAuthenticateChallenge` request, solve the
returned challenge, and attach it to the admin action request.

If a user has no MFA device registered, `CreateAuthenticateChallenge` will fail.
In this case, the client will make the request without the MFA challenge response
in case we are handling a special case (e.g. Built in `Admin` role, bot user
for automated use cases).

### Proto Specification

Each Admin action API request will need to include the `MFAAuthenticateResponse`
field in its proto specification.

Clients will automatically check for the `MFAAuthenticateResponse` in order to
determine whether it must prompt for MFA. This will be done seamlessly within
a `grpc.UnaryInterceptor`.

Likewise, the Server will automatically check for an `MFAAuthenticateResponse` in
using a `grpc.UnaryInterceptor`. If found, this will be passed down the chain
through the `AuthContext` (like user) where it can be verified by `ServerWithRoles`
if required.

#### HTTP Endpoints

Some requests which we would like to make admin actions are still used through
the HTTP API rather than the gRPC API, such as `POST UpsertUser`. Part of this
work will require converting these requests to gRPC, which will also move us
closer to [fully deprecating the HTTP API](https://github.com/gravitational/teleport/issues/6394).

#### Non-unique gRPC Requests

Many any of our existing gRPC API requests do not use unique request or response
messages, despite it being a best practice. As a result, it is not trivial to add
the `MFAAuthenticateResponse` field to some requests.

For example:

```proto
rpc UpdateUser(types.UserV2) returns (google.protobuf.Empty);
```

For these cases, we will need to create replacement requests:

```proto
rpc UpdateUser(types.UserV2) returns (google.protobuf.Empty);
rpc UpdateUserV2(UpdateUserV2Request) returns (UpdateUserV2Response);
```

For many of these cases above, it will make more sense to add or move the rpc
into a new service. This will allow us to:

- Maintain preferred naming conventions, avoiding `V2` suffixes
- Reduce reliance on gogoproto which has got to go
- Reduce size of legacy proto packages which are incompatible with buf

This will be handled on a case by case basis, but user service looks like a
likely candidate:

```proto
service UserService {
  rpc GetUser(GetUserRequest) returns (types.UserV2);
  rpc GetUsers(GetUsersRequest) returns (stream types.UserV2);
  // We can replace CreateUser and UpdateUser with UpsertUser, as is the latest convention of our API.
  rpc UpsertUser(UpsertUserRequest) returns (UpsertUserResponse);
  rpc DeleteUser(DeleteUserRequest) returns (google.protobuf.Empty);
  ...
}
```

### Test Plan

The implementation of this feature will include automated unit tests to ensure
that MFA is required for admin actions across applicable server configurations.

### Security

The bulk of this RFD is focused on security and will result in a net positive
to our security outlook.

Here are a few key points to review:

- [Server Configuration](#server-configuration):
  - In order to maintain backwards compatibility and non-MFA
  compatible use cases (automated plugins), we give server admins the option to
  turn off MFA for admin actions for the cluster or specific roles. This also
  means that an admin can turn off the requirement for themselves, potentially
  opening them back up to the vulnerability we are trying to close. As a counter
  measure, we will not recommend turning off MFA for admin actions outside of use
  cases which absolutely require it. Our Docs should use warning language whenever
  this option is mentioned.
  - MFA will be required for admin actions by default if a cluster's second
  factor setting allows it (`on`, `webauthn`, `otp`). This default won't apply
  until the second major version of this feature for backwards compatibility.
  - Built in roles will not require MFA for admin actions, including the
  `Admin` role used when executing `tctl` directly on an Auth Server.
- [Limited MVP](#mvp-admin-actions-subset): The changes necessary to convert
all admin actions to requiring MFA is too large to complete in one go. However,
We should take care not to let the priority of the full implementation slip
away once the MVP is complete, since this will reduce the security impact of the
feature.

### Other considerations

#### Hardware Key Support

Hardware Key Support can also be used for this use case. When setting
`require_session_mfa: hardware_key_touch` for a role or cluster, YubiKey tap is
required for _every_ private key operation, which applies to essentially every
action you can take in Teleport.

However, Hardware Key support is still somewhat limited due to the limitations
of PIV. Most notably, Hardware Key support is not supported on the Web UI, which
is essential for this feature.
