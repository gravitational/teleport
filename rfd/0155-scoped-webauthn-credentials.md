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
+  // AllowReuse indicates that this session can be used multiple times for
+  // authentication, until the session expires.
+  bool AllowReuse = 7 [(gogoproto.jsontag) = "allow_reuse,omitempty"];
}



+// Scope is a scope authorized by a webauthn challenge resolution.
+//
+// Note: New scopes can be added to cover new use cases, or to split existing
+// scopes into more granular scopes. For example, passwordless and headless login
+// could be included under the "login" scope, but splitting them into their own
+// scopes improves the security of each.
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
+  // AllowReuse means webauthn credentials resolved from this challenge can be
+  // reused for a short span of time before the challenge expires.
+  //
+  // Reuse is only permitted for specific actions by the discretion of the server.
+  // See the server implementation for details.
+  bool AllowReuse = 7 [(gogoproto.jsontag) = "allow_reuse,omitempty"];
}
```

### Client changes

Clients will be expected to provide a `Scope` when requesting an MFA challenge
through `rpc CreateAuthenticateChallenge`. For specific login and device
management endpoints, the scope will be automatically set on the server side.

Clients can optionally provide `AllowReuse=true` if the client wants to reuse
the resulting webauthn credentials for multiple requests. However, the server
will only permit reuse for a select number of actions. See
[the server implementation details](#reuse).

### Server changes

#### Scope

When verifying a webauthn credential against the user's stored webauthn
challenge, the Auth Server will check that the stored scope matches the scope
that the Auth server is verifying for. For example, if the Auth server is
verifying a webauthn credential for scope "login", but webauthn challenge
stored for the user has scope "headless", the verification will fail.

#### Reuse

If an MFA action does not allow reuse, the Auth Server will validate that the
webauthn challenge has `AllowReuse=false`.

Additionally, challenges with `AllowReuse=true` will not be deleted immediately,
instead letting them expire in the backend after 5 minutes.

Initially, reuse will only be permitted for the following admin action RPCs:

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
- `rpc CreateOIDCConnector`
- `rpc UpdateOIDCConnector`
- `rpc UpsertOIDCConnector`
- `rpc UpsertSAMLConnector`
- `rpc CreateSAMLConnector`
- `rpc UpdateSAMLConnector`
- `rpc UpsertSAMLConnector`
- `rpc CreateGithubConnector`
- `rpc UpdateGithubConnector`
- `rpc UpsertGithubConnector`
- `rpc CreateSAMLIdPServiceProvider`
- `rpc UpdateSAMLIdPServiceProvider`
- `rpc UpsertAccessList`
- `rpc UpsertAccessListWithMembers`
- `rpc UpsertTokenV2`
- `rpc CreateTokenV2`
- `rpc GenerateToken`
- `rpc CreateBot`
- `rpc UpdateBot`
- `rpc UpsertBot`
- `rpc DeleteBot`

These RPCs are currently used for "bulk" requests such as:

- `tctl create -f multiple-resources.yaml` which can perform several
updates and creates at once.
- `tctl users add` which creates a user and a reset password token for the user.

##### When to extend reuse

The list above should be kept to a minimum, but can be extended on a case by
case basis.

In many cases, it would be best to create new endpoints which contain the full
string of actions when feasible. For example, we could replace our current add
user flow, which calls `CreateUser` and `CreatePasswordToken`, to call a single
`AddUser` endpoint which does both operations.

Other times, it may be overly cumbersome or complicated to move an operation
into a single endpoint. For example, it does not currently seem worth it to
create a single `UpsertResources` endpoint to handle all cases of
`tctl create -f multiple-resources.yaml`. Instead we call individual CRUD
endpoints for each resource contained in the file, reusing the same Webauthn
challenge along the way.

Warning: Reuse should not be extended to sensitive operations, including all
of the non admin action scopes laid out above and the following admin action
endpoints:

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
  - `rpc AccessRequestPromote`
  - `rpc CreateAccessListReview`
- CA management
  - `http rotateCertAuthority`
  - `http rotateExternalCertAuthority`
  - `rpc UpsertCertAuthority`
  - `rpc DeleteCertAuthority`
  - `rpc DeleteCertAuthority`
- Certificate generation
  - `rpc GenerateHostCert`
  - `rpc GenerateHostCerts`
  - `rpc GenerateUserCerts`
  - `http createWebSession`
  - `http deleteWebSession`

Update: Minimizing which admin actions allow reuse has caused several issues
in new bulk admin actions, most notably the new Discover flows. Other than the
critical admin action endpoints listed above, most now allow reuse. It is
instead left up to the client to be reasonable, only requesting a reusable MFA
challenge in preparation for a bulk admin action.

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

## Additional

### TOTP

This RFD does not cover adding scope for TOTP. However, if necessary, we could
extend this RFD to TOTP by creating some TOTP session data in the backend to
match the webauthn session data flow. This TOTP session would just hold the
scope and whether reuse is allowed. This TOTP session would replace the
existing "Used TOTP token" we store in the backend.
