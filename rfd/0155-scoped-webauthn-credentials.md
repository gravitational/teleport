---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 155 - Scoped Webauthn Credentials

## Required Approvers

- Engineering: @rosstimothy && @codingllama
- Product: @klizhentas || @xinding33
- Security: @jentfoo

## What

Add and enforce scope for webauthn credentials.

## Why

Today, when a client performs a webauthn ceremony, the resulting webauthn
credentials can be used to perform actions other than what the client initially
intended.

For example, webauthn credentials intended for per-session MFA could be used
instead for login. This means that if an attacker gets a hold of a user's
webauthn credentials before the client consumes them, an attacker with the
user's password could login as the user. Adding scope to the webauthn
credentials would prevent the attacker from using the stolen webauthn
credentials freely.

Adding scope also enables us to allow webauthn credentials to be reused within
specific scopes. For example, for admin actions it may make sense to allow the
client to reuse a single webauthn credential for a string of requests that are
functionally a single action, such as creating a new user and their corresponding
invite token.

## Details

### Security

Enforcing scope for webauthn credentials strictly improves security, as it is a
purely restrictive change that prevents certain attack vectors, as described
in [Why](#why).

On the other hand, allowing clients to reuse webauthn credentials could have
negative security implications if not handled carefully. Specifically, the
client and server must clearly define:

1. The webauthn credential's scope is provided by the client
2. Reuse is requested by the client
3. Reuse is permitted for the action - server enforced
4. The expiration of the credentials - server enforced (5 minutes)

Note: Sensitive operations like "login" and "recovery" must never allow reuse.

### Proto

```diff
// webauthn.proto

message SessionData {
  bytes challenge = 1 [(gogoproto.jsontag) = "challenge,omitempty"];
  bytes user_id = 2 [(gogoproto.jsontag) = "userId,omitempty"];
  repeated bytes allow_credentials = 3 [(gogoproto.jsontag) = "allowCredentials,omitempty"];
  bool resident_key = 4 [(gogoproto.jsontag) = "residentKey,omitempty"];
  string user_verification = 5 [(gogoproto.jsontag) = "userVerification,omitempty"];

+  // Scope authorized by this webauthn session.
+  ChallengeScope scope = 6 [(gogoproto.jsontag) = "scope,omitempty"];
}

+// Scope is a scope authorized by a webauthn challenge resolution.
+enum ChallengeScope {
+  SCOPE_UNSPECIFIED = 0;
+  // Standard webauthn login.
+  SCOPE_LOGIN = 1;
+  // Passwordless webauthn login.
+  SCOPE_PASSWORDLESS_LOGIN = 2;
+  // MFA device management.
+  SCOPE_MANAGE_DEVICES = 3;
+  // Account recovery.
+  SCOPE_RECOVERY = 4;
+  // Used for per-session MFA and moderated session presence checks.
+  SCOPE_SESSION = 5;
+  // Headless login approval.
+  SCOPE_HEADLESS = 6;
+  // Used for various administrative actions, such as adding, updating, or
+  // deleting administrative resources (users, roles, etc.).
+  //
+  // Note: this scope should not be used for new MFA capabilities that have
+  // more precise scope. Instead, new scopes should be added. This scope may
+  // also be split into multiple smaller scopes in the future.
+  SCOPE_ADMIN_ACTION = 7;
+  // Webauthn credentials resolved from a challenge with this scope can be
+  // reused for a short span of time before the challenge expires.
+  //
+  // This scope for a select few admin actions and will be rejected by other
+  // admin actions. See the server implementation for details.
+  SCOPE_ADMIN_ACTION_WITH_REUSE = 8;
+}

// authservice.proto

message CreateAuthenticateChallengeRequest {
  oneof Request {
    UserCredentials UserCredentials = 1 [(gogoproto.jsontag) = "user_credentials,omitempty"];
    string RecoveryStartTokenID = 2 [(gogoproto.jsontag) = "recovery_start_token_id,omitempty"];
    ContextUser ContextUser = 3 [(gogoproto.jsontag) = "context_user,omitempty"];
    Passwordless Passwordless = 4 [(gogoproto.jsontag) = "passwordless,omitempty"];
  }
  IsMFARequiredRequest MFARequiredCheck = 5 [(gogoproto.jsontag) = "mfa_required_check,omitempty"];

+  // Scope is a authorization scope for this MFA challenge.
+  // Required. Only applies to webauthn challenges.
+  webauthn.ChallengeScope Scope = 6 [(gogoproto.jsontag) = "scope,omitempty"];
}
```

### Client changes

Clients will be expected to provide a `Scope` when requesting an MFA challenge
through `rpc CreateAuthenticateChallenge`. For specific login and device
management endpoints, the scope will be automatically set on the server side.

For admin actions, clients can provide either `SCOPE_ADMIN_ACTION` or
`SCOPE_ADMIN_ACTION_WITH_REUSE` to indicate whether reuse is needed. However,
the reuse scope is only permitted for a limited number of admin actions.
See [the server implemenation details](#reuse)

### Server changes

#### Scope

Each MFA action performed by the Auth Server will have an assigned scope. When
verifying a webauthn credential for the action, the server will check that the
user's stored webauthn challenge:

1. Matches the webauthn credential, as normal
2. Matches the scope associated with the action

For example, if the Auth server is verifying a webauthn credential for scope
"login", but webauthn challenge stored for the user has scope "headless", the
verification will fail.

Note: some scopes may be inherited, such that one scope can validate another.
For example, `SCOPE_ADMIN_ACTION` will validate `SCOPE_ADMIN_ACTION_WITH_REUSE`

#### Reuse

After verifying a webauthn credential, the Auth server will check the
`Scope` field of the stored challenge. If the scope is
`SCOPE_ADMIN_ACTION_WITH_REUSE`, the challenge won't be deleted as normal.

As mentioned in [Security](#security), reuse will only be permitted for specific
RPCs:

- `rpc CreateUser`
- `rpc UpdateUser`
- `rpc UpsertUser`
- `rpc CreateRole`
- `rpc UpdateRole`
- `rpc UpsertRoleV2`
- `rpc UpsertRole`
- `rpc CreateUserGroup`
- `rpc UpdateUserGroup`
- `rpc CreateResetPasswordToken`
- `rpc SetClusterNetworkingConfig`
- `rpc SetSessionRecordingConfig`
- `rpc SetAuthPreference`
- `rpc SetNetworkRestrictions`
- `rpc UpsertOIDCConnector`
- `rpc CreateOIDCAuthRequest`
- `rpc UpsertSAMLConnector`
- `rpc CreateSAMLAuthRequest`
- `rpc UpsertGithubConnector`
- `rpc CreateGithubAuthRequest`
- `rpc CreateSAMLIdPServiceProvider`
- `rpc UpdateSAMLIdPServiceProvider`
- `rpc UpsertAccessList`
- `rpc UpsertAccessListWithMembers`
- `rpc UpsertTokenV2`
- `rpc CreateTokenV2`
- `rpc GenerateToken`

These RPCs are currently used for "bulk" requests such as:

- `tctl create -f multiple-resources.yaml` which can perform several
updates and creates at once.
- `tctl users add` which creates a user and a reset password token for the user.

The server will refuse to validate a reusable webauthn credential for RPCs not
in the list above.

This list should be kept to a minimum by opting to create new endpoints which
contain the full string of actions when feasible.

Reuse should not be extended to sensitive operations, including all of the non
admin action scopes laid out above and the following admin action endpoints:

- Account recovery management
  - `rpc ChangeUserAuthentication`
  - `rpc StartAccountRecovery`
  - `rpc VerifyAccountRecovery`
  - `rpc CompleteAccountRecovery`
  - `rpc CreateAccountRecoveryCodes`
- Dynamic access
  - `rpc CreateAccessRequest`
  - `rpc SetAccessRequestState`
  - `rpc SubmitAccessReview`
  - `AccessRequestPromote`
  - `CreateAccessListReview`
- CA management
  - `http rotateCertAuthority`
  - `http rotateExternalCertAuthority`
  - `http upsertCertAuthority`
  - `http deleteCertAuthority`
- Certificate generation
  - `rpc GenerateHostCerts`
  - `rpc GenerateUserCerts`
  - `http createWebSession`
  - `http deleteWebSession`

#### Expiration

Webauthn challenges are always set to expire after 5 minutes. However, as we've
seen with Dynamo DB, these expirations are not always strictly respected.
Therefore, the Auth server will start checking the expiration of stored
webauthn challenges. If the challenge is past it's expiration, the Auth server
will delete it from the backend explicitly.

Note: This change should be backported since this is an existing issue for
unconsumed webauthn challenges.

### UX

There should be no changes to UX, other than reducing the number of MFA taps
required for certain admin actions from several down to one.

### Backward Compatibility

In order to maintain backwards compatibility with old clients, scope will only
be enforced opportunistically when provided by the client until the next major
version after this feature is released.

However, any new features tied to scope, such as webauthn credential reuse,
will only be permitted to clients that do provide scope.

### Audit Events

Currently, MFA audit details are quite limited. We only emit the MFA device used
for some MFA actions, such as login. We will add 2 new audit events for better
coverage.

```proto
// events.proto
message CreateMFAAuthChallenge {
  Metadata Metadata = 1;
  UserMetadata User = 2;
  webauthn.Scope Scope = 3;
  bool AllowReuse = 4;
}

message ValidateMFAAuthResponse {
  Metadata Metadata = 1;
  UserMetadata User = 2;
  MFADeviceMetadata Device = 3;
  webauthn.Scope Scope = 4;
  bool AllowReuse = 5;
}
```

### Product Usage

```proto
// MFAVerificationEvent is emitted when an MFA verification event is completed.
message MFAVerificationEvent {
  // anonymized
  string user_name = 1;
  // the mfa device type used for verification. e.g. Webauthn, U2F, or TOTP.
  // Matches api/types.MFADevice.MFAType.
  string mfa_device_type = 2;
  string scope = 3;
}
```
