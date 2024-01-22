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

Enforce MFA verification for administrative actions. For example:

- Adding, removing, or editing Nodes and other services.
- Adding, removing, or editing users, roles, auth connectors, and other
administrative resources.
- Approving access requests.
- Impersonating users.

This will apply to administrative actions performed from any Teleport client,
including `tctl`, `tsh`, the Teleport Web UI, and Teleport Connect.

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
action.

## Details

### Administrative Actions

An administrative action will be defined as an action which creates, updates,
or deletes an administrative teleport resource. For example:

- `tctl rm/create/edit`
- `tctl users add/rm/reset`
- Modifying a user, role, or auth connector from the WebUI

There are hundreds of actions which could qualify as administrative actions,
but to restrict the scope of this feature, we will start with a "short" list of
actions which modify the following administrative resources.

- User management
  - Users
    - `CreateUser`, `UpdateUser`, `UpsertUser`, `DeleteUser`
  - Roles
    - `CreateRole`, `UpdateRole`, `UpsertRoleV2`, `UpsertRole`, `DeleteRole`
  - Trusted devices
    - `CreateDevice`, `UpdateDevice`, `UpsertDevice`, `DeleteDevice`, `BulkCreateDevices`, `CreateDeviceEnrollToken`, `SyncInventory`
  - User groups
    - `CreateUserGroup`, `UpdateUserGroup`, `DeleteUserGroup`, `DeleteAllUserGroups`
  - Account recovery
    - `ChangeUserAuthentication`, `StartAccountRecovery`, `VerifyAccountRecovery`, `CompleteAccountRecovery`, `CreateAccountRecoveryCodes`
- Cluster configuration
  - Networking
    - `SetClusterNetworkingConfig`, `ResetClusterNetworkingConfig`
  - Session recording
    - `SetSessionRecordingConfig`, `ResetSessionRecordingConfig`
  - Auth preference
    - `SetAuthPreference`, `ResetAuthPreference`
  - Network restrictions
    - `SetNetworkRestrictions`, `DeleteNetworkRestrictions`
- Access management
  - Auth connectors
    - `UpsertOIDCConnector`, `DeleteOIDCConnector`, `CreateOIDCAuthRequest`
    - `UpsertSAMLConnector`, `DeleteSAMLConnector`, `CreateSAMLAuthRequest`
    - `UpsertGithubConnector`, `DeleteGithubConnector`, `CreateGithubAuthRequest`
    - `CreateSAMLIdPServiceProvider`, `UpdateSAMLIdPServiceProvider`, `DeleteSAMLIdPServiceProvider`, `DeleteAllSAMLIdPServiceProviders`
  - Access requests
    - `CreateAccessRequest`, `DeleteAccessRequest`, `SetAccessRequestState`, `SubmitAccessReview`
  - Access lists
    - `UpsertAccessList`, `UpsertAccessListWithMembers`, `DeleteAccessList`, `AccessRequestPromote`
    - `UpsertAccessListMember`, `DeleteAccessListMember`, `DeleteAllAccessListMembersForAccessList`
    - `CreateAccessListReview`, `DeleteAccessListReview`
    - `GetSuggestedAccessLists`
  - Join Tokens
    - `UpsertTokenV2`, `CreateTokenV2`, `GenerateToken`, `DeleteToken`
    - http: `deleteStaticTokens`, `setStaticTokens`
  - Login Rules
    - `UpsertLoginRule`, `CreateLoginRule`, `DeleteLoginRule`
- Certificates
  - CA management
    - `UpsertCertAuthority`, `DeleteCertAuthority`
    - http: `rotateCertAuthority`, `rotateExternalCertAuthority`
  - Host certs
    - `GenerateHostCerts`, `GenerateHostCert`
  - User certs
    - `GenerateUserCerts`
  - Web sessions
    - http: `createWebSession`, `deleteWebSession`
  - Bots
    - `CreateBot`, `DeleteBot`

Note: http endpoints in the list above are noted separately due to the [additional
work](#http-endpoints) associated with these endpoints.

Here are some notable exclusions from the list which may be future admin action
candidates:

- Service management and registration
- Session management
- Trusted cluster management
- Lock management
- Cluster alerts
- Cloud billing and other cloud specific endpoints

Here is a list of actions which are exempt from being admin actions now or in
the future:

- User actions that are authorized based on the user owning the resource.
These requests will only require MFA when the authorization comes from the
user's role permissions rather than the user's ownership. For example:
  - `CreateAccessRequest`, `DeleteAccessRequest`
  - `CreateAuthenticateChallenge`
  - `ChangePassword`, `CreateResetPasswordToken`
  - `AddMFADeviceSync`, `DeleteMFADeviceSync`
- Actions which only require `DefaultImplicitRole`:
  - `SubmitUsageEvent`
- Actions which are limited to [built-in service roles](#built-in-roles).

#### Caveat

This list is too long to thoroughly dig into all of the details and exceptions
in this RFD alone. Instead, we will add MFA to the listed (and unlisted) admin
actions one at a time, grouping similar endpoints into separate Github PRs.

Within each PR, we can ensure that every usage of that endpoint is updated in a
backwards compatible way across all Teleport clients (`tsh`, `tctl`, Teleport
Connect, Teleport Web UI, Plugins and plugin guides).

### Server configuration

MFA for admin actions will required on any cluster where Webauthn is the
preferred MFA method. This includes clusters with `cluster_auth_preference.second_factor`
set to `on`, `optional`, or `webauthn` where `cluster_auth_preference.webauthn`
is also set.

#### Built-in Roles

Built-in roles will not require MFA to complete admin actions. For example:

- `Auth`, `Proxy`, `Node`, and other service roles.
- `Admin` role used by `tctl` directly on an Auth server.
- `Bot` role used by MachineID to generate certificates.

#### Automated Use Cases

In order to support automated use cases such as the
[Teleport Terraform Provider](https://goteleport.com/docs/management/guides/terraform-provider/)
or [Access Request Plugins](https://goteleport.com/docs/access-controls/access-request-plugins/),
we need to provide a way for non-interactive identities to bypass MFA for admin actions.

First, we will narrow the scope of what constitutes a non-interactive identity.
A non-interactive identity is a set of certificates generated with impersonation
by either the `Bot` or `Admin` built in role, which is done with MachineID or
`tctl auth sign` on the Auth server respectively.

If an API client's certificates have the `impersonator` extension set to a
user with the `Bot` or `Admin` role, then MFA will not be attempted nor
required for admin actions.

Impersonated certificates generated by a user will still require MFA for admin
actions. We currently use this type of impersonation for our API and plugin
guides, so each of these guides will need to be updated to use MachineID.

### MFA enforcement

MFA verification will be required to carry out various administrative actions.
This will be enforced by the Auth API Server for the specific endpoints which
are considered [administrative actions](#administrative-actions).

Each Admin Action API request will follow this flow:

1. Create an Auth API client using the user's existing valid certificates.
1. Use the client to retrieve an MFA challenge for the user with
   the existing `rpc CreateAuthenticateChallenge`.
1. Prompt the user to solve the MFA challenge with their MFA device.
1. Send the resulting `MFAAuthenticateResponse` to the Auth Server as part of the
   API request.
1. Validate and consume the `MFAAuthenticateResponse` in the Auth Server to
   authorize the request (in addition to normal identity-based authorization).

Steps 1-3 are already possible with Teleport currently and used for various MFA
features. Steps 4 and 5 will require some changes to the Auth API client and
server respectively.

#### Server changes

For admin actions, the Auth server will validate MFA for each request using the
`MFAAuthenticateResponse` passed in the request metadata.

If the request fails MFA verification, an access denied error will be returned
to the client.

```go
// ErrAPIMFARequired is returned by AccessChecker when an API request
// requires an MFA check.
var ErrAdminActionMFARequired = trace.AccessDenied("administrative action requires MFA")
```

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
  for external users of the API.
- The MFA verification only applies to one API request, preventing MFA-replay-like
  attacks.

Cons:

- Requires custom changes to both the client and server implementations for
  each affected endpoint, both to pass the MFA challenge response from the
  client and for the Auth server to verify it for the request.
- Old clients have no way to pass MFA until the `MFAAuthenticateResponse`
  message is included in the proto specification. In order to maintain backwards
  compatibility, each request that we change to an admin action will take a full
  major version before we can actually require MFA for that action.

2. Allow clients to pass `MFAAuthenticateResponse` as client request metadata.

This approach is similar to passing a bearer-token in HTTP requests. Note that it
will augment the normal certificate auth flow, not replace it. Additionally, the
MFA Challenge Response will be passed to the server within the context of the TLS
handshake, so it is secure.

The client can uniformly pass the `MFAAuthenticateResponse` with the gRPC client call
option `PerRPCCredentials`. The Auth server can then grab the `MFAAuthenticateResponse`
from the request metadata, consume/verify the challenge response, and treat the request
as MFA verified. [POC](https://github.com/gravitational/teleport/commit/aa0a8102eccd91cff2851053fba3e1c271bdaa65).

Pros:

- Client can apply common logic for MFA verification, rather than endpoint
  specific logic.
  - Ex: First request without MFA, receive access denied error, re-request with MFA.
  - MFA verification can be extended to additional Auth endpoints without any client changes.
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

##### Option choice: 2

Option 1 and 2 both guarantee that a user's MFA verification applies to just one
admin action in the fewest number of API requests, solidifying it as a preferable
solution over option 3 for both security and implementation complexity.

Between option 1 and 2, option 1 is more explicit and simple in how and where MFA
will be required. This would make the feature easier to use for both internal and
external developers. However, it requires significant API changes and has some
backwards compatibility drawbacks.

We will implement option 2 for its simplicity and limited drawbacks. This will
allow us to quickly implement the feature and apply it to all relevant admin
actions in Teleport 14.

In the future, we can consider coming back to implement option 1, as they are
not mutually exclusive.

##### MFA prompt for non-`tsh` clients

Currently `tsh` is the only client with universal MFA prompt logic. We will
need to implement similar logic in `tctl`, the Web UI, and Teleport Connect.

`tctl` can reuse the same logic used in `tsh`, making it the easiest change.
However, we will also need to start creating a signed `tctl.app` in order to
support Touch ID in `tctl`.

For the Web UI and Teleport Connect, MFA prompt logic is handled on a case by
case basis. We will develop reusable logic and modals for these Apps to prompt
for MFA when necessary.

##### Infer MFA Requirement

Before making an admin action request, Teleport clients can check the server's
`cluster_auth_preference.second_factor` settings with a ping request. If MFA is
required for the set `second_factor`, the client will first make a
`CreateAuthenticateChallenge` request, solve the returned challenge, and attach
it to the admin action request.

If a user has no MFA device registered, `CreateAuthenticateChallenge` will fail.
In this case, the client will make the request without the MFA challenge response,
in case we are handling a special case (e.g. Built-in role, `Bot` or `Admin`
impersonation).

Additionally, if the client doesn't know whether a request requires MFA or not,
it will first attempt the request without it. If the server responds with
`ErrAdminActionMFARequired`, the client will attempt to retry the request with
MFA.

### Proto Specification

Since the MFA verification will be passed through the gRPC metadata, there are
no proto changes required.

#### HTTP Endpoints

Some requests which we would like to make admin actions are still used through
the HTTP API rather than the gRPC API, such as `POST UpsertUser`. Part of this
work will require converting these requests to gRPC, which will also move us
closer to [fully deprecating the HTTP API](https://github.com/gravitational/teleport/issues/6394).

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

### Security

The bulk of this RFD is focused on security and will result in a net positive
to our security outlook.

Here are a few key points to review:

- [Server Configuration](#server-configuration):
  - MFA will be required for admin actions if a cluster's second factor setting
  allows it (`optional`, `on`, `webauthn`, `otp`).
  - [Built-in roles](#built-in-roles) will not require MFA for admin actions, including the
  `Admin` role used when executing `tctl` directly on an Auth Server.
  - [MachineID](#automated-use-cases) certificates generated with MachineID or
  Admin impersonation will not require MFA. Users with access to the Auth server
  directly or with `create` permissions on the `role`, `user`, and `token` could
  use their privileges to generate impersonated certificates that do not
  require MFA for admin actions.

### Backward Compatibility

In order to maintain backwards compatibility with old clients (`tsh`, `tctl`,
`Teleport Connect`), MFA will not be strictly required for admin actions until
Teleport 15.

|        | Teleport 13 | Teleport 14 | Teleport 15 |
|--------|-------------|-------------|-------------|
| Server | Does not require MFA | Does not require MFA | Requires MFA |
| Client | Does not provide MFA | Provides MFA | Provides MFA |

Additionally, Teleport 14+ clients will be able to provide MFA for admin actions
whether or not the client has prior knowledge of the request requiring MFA. This
means that additional endpoints can be changed into admin actions without any
consequences.

#### WebUI

The WebUI will not support providing MFA for admin actions until Teleport 15.
This means that administrators will need to upgrade their Proxy services to
Teleport 15 alongside their Auth services to avoid getting admin actions
denied. This must not cause any upgrading issues.

#### HTTP endpoints

Clients will not attempt to provide MFA for [HTTP endpoints](#http-endpoints).
This means that http endpoints converted to gRPC admin action endpoints should
not be made to require MFA until a full major version cycle has passed.

### Audit Events

#### Admin Action Events

Audit events will be added to each admin action. Many admin actions already
have their own audit events, such as `role.created` event. Other admin actions
are missing audit events, like `node.created`. Any admin action missing an
audit event will have one added.

These events will only be emitted when the action is taken by a user. This
prevents audit spam from built in services performing their normal duties.
For example, a node upserting itself with a heartbeat will not be recorded,
but if a user edits a node directly, it will be.

#### Admin Action MFA Event

In addition to the individual audit events emitted, we will emit an admin
action mfa authentication event. This event will also hold the metadata of its
corresponding admin action event to tie the two events together.

```proto
// UserCreate is emitted when the user is created or updated (upsert).
message AdminActionMFAAuthentication {
  Metadata metadata = 1;
  UserMetadata user = 2;
  // request_metadata of the corresponding admin action event
  Metadata request_metadata = 3;
  Status status = 4;
  MFADeviceMetadata mfa_device = 4;
}
```

If MFA verification fails for an admin action, then this event will be emitted
with an error in its status, while the corresponding admin action event, in most
cases, will not be emitted.

### Product Usage

We will add a new admin action mfa PostHog event. This event will be derived
from the audit event above.

```proto
message AdminActionMFAAuthenticationEvent {
  // The name of the admin action, derived from the audit event's request_metadata.type field.
  string admin_action_request_name = 1;
  // anonymized
  string user_name = 2;
  bool success = 3;
}
```

### Test Plan

The implementation of this feature will include automated unit tests to ensure
that MFA is required for admin actions across applicable server configurations.

### Other considerations

#### Hardware Key Support

Hardware Key Support can also be used for this use case. When setting
`require_session_mfa: hardware_key_touch` for a role or cluster, YubiKey tap is
required for _every_ private key operation, which applies to essentially every
action you can take in Teleport.

However, Hardware Key support is still somewhat limited due to the limitations
of PIV. Most notably, Hardware Key support is not supported on the Web UI, which
is essential for this feature.

#### TOTP

The WebUI and Teleport Connect do not currently have a universal TOTP MFA
prompt, making it difficult to universally support TOTP for admin actions.

If in the future TOTP prompts are added for these applications, we can consider
widening this feature to also cover all clusters where `cluster_auth_preference.second_factor`
is not set to `off`, rather than the more limited scope described in
[Server configuration](#server-configuration). However, this most likely won't
be a priority as we are actually looking to [deprecate TOTP](https://github.com/gravitational/teleport/issues/20725).
