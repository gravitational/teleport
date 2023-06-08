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

Starting in Teleport 14, MFA for administrative actions will be turned on by
default for all users with an MFA device registered. This means that when
a cluster's second factor is set to one of `optional`, `on`, `webauthn`, or
`otp`, all normal login sessions will require MFA verification to complete
admin actions.

### Administrative Actions

An administrative action will be primarily defined as an action which modifies a
resource in a Teleport Cluster directly. Essentially, this covers any action which
requires an allow rule with a `create`, `update`, or `delete` verb requirement.

For example:

- `tctl rm/create/edit`
- `tctl users add/rm/reset`
- Adding a node from the WebUI
- Modifying a role from the WebUI

#### Usage Event Creation

The `usage_event` resource can be created by any Teleport User via the
`DefaultImplicitRole`. This will not be considered an admin action.

### Access Request Approval

Submitting an access request approval does not require an `update` allow rule,
but since it can be used to escalate privileges, it will still be considered an
admin action.

Submitting an access request denial will remain a non-admin action.

#### User Actions

Some actions which create, update, or delete user specific resources are
authorized based on the identity of the requesting user rather than their
allow rules. For example:

- Creating, listing, and deleting a user's own access requests.
- Adding or removing MFA devices.
- Changing the user's password.

These actions will continue to be non-administrative actions and will not require
MFA to complete (if they don't already).

### UX

The UX of this feature will be very similar to Per-session MFA for `tsh`, `tctl`,
and the WebUI, and Teleport Connect.

When a user performs an admin action, they will be prompted to tap their MFA key.
Each admin action will require a single unique tap.

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
1. Send the resulting `MFAChallengeResponse` to the Auth Server as part of an
   administrative API request.
1. Validate and consume the `MFAChallengeResponse` in the Auth Server to
   authorize the request (in addition to normal identity-based authorization).

Steps 1-3 are already possible with Teleport currently and used for various MFA
features. Steps 4 and 5 will require some changes to the Auth API client and
server respectively.

#### Client changes

There are a few different ways that the Auth client can pass MFA verification with
the Auth server, each with their own pros and cons:

1. Add the `MFAChallengeResponse` argument to each administrative API endpoint.

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
    a common gRPC message for the `MFAChallengeResponse`.

2. Allow clients to pass `MFAChallengeResponse` as client request metadata.

This approach is similar to passing a bearer-token in HTTP requests. Note that it
will augment the normal certificate auth flow, not replace it. Additionally, the
MFA Challenge Response will be passed to the server within the context of the TLS
handshake, so it is secure.

The client can cleanly pass the challenge response with the gRPC client call
option `PerRPCCredentials`. The Auth server can then grab the `MFAChallengeResponse`
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

##### Option choice: TBD

I am slightly in favor of option 2 due to the flexibility of the system compared
to option 1 and the security benefit over option 3. However, we may favor the
simplicity of defining MFA requirements on the gRPC layer (option 1) or of
enforcing MFA based on the user's certificate alone (option 3).

@Reviewers please offer your opinions and I will update this section once we
decide on an option.

#### Server changes

For admin actions, the Auth server will validate MFA for each request, either
from an `MFAChallengeResponse` passed by the client or an MFA-verified certificate
used for the request.

If the request fails MFA verification, an access denied
error will be returned to the client.

```go
// ErrAPIMFARequired is returned by AccessChecker when an API request
// requires an MFA check.
var ErrAdminActionMFARequired = trace.AccessDenied("administrative action requires MFA")
```

The client will check for this error to determine whether it should retry a
request with MFA verification.

### Other considerations

### Automated use cases

If a user has no MFA device registered, they will not need to pass MFA
verification for admin actions. This means that you can impersonate a user
without an MFA device registered to get certificates for that user which bypass
this MFA requirement. This is essential for automated use cases such as the
[Teleport Terraform Provider](https://goteleport.com/docs/management/guides/terraform-provider/)
or [Access Request Plugins](https://goteleport.com/docs/access-controls/access-request-plugins/).
to continue to work as expected.

#### Hardware Key Support

Hardware Key Support can also be used for this use case. When setting
`require_session_mfa: hardware_key_touch` for a role or cluster, YubiKey tap is
required for _every_ private key operation, which applies to essentially every
action you can take in Teleport.

However, Hardware Key support is still somewhat limited due to the limitations
of PIV. Most notably, Hardware Key support is not supported on the Web UI, which
is essential for this feature.
