---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 180 - SSO as MFA Method

## Required Approvers
* Engineering: @rosstimothy || @codingllama
* Product: @xinding33 || @klizhentas

## What

Provide the ability to satisfy Teleport MFA checks via a 3rd party identity
provider with SSO.

Per-session MFA must be supported for all Teleport clients, while additional
features will be included as a stretch goal, such as MFA for admin actions and
moderated sessions. Ideally we will support SSO as a first class MFA method.

## Why

Delegating MFA checks to a registered IdP has several benefits:
- Administrators gain the ability to configure and monitor all authentication
directly through an IdP.
- Teleport can integrate with custom MFA mechanisms and devices through an IdP.
- Improves UX for SSO users without an MFA device registered.
- Allows SSO users to add their first MFA device securely with sso-as-mfa.

## Details

### UX and User stories

This feature will improve the UX of performing MFA checks for SSO users by
removing the requirement to add an MFA device. The primary UX concerns for
this feature are:
- Adding too many options without clarity (OTP, Webauthn, SSO).
- Locking users into SSO MFA in cases where WebAuthn may be preferred.
- Automatically opening browser windows when WebAuthn may be preferred.

#### First time SSO user

> I am a new Teleport user logging in to the cluster for the first time
> I want to connect to a resource protected by per-session MFA
> I have not registered through Teleport, my company uses an IdP provider for login


**Old behavior**

When the user logs in with SSO for the first time, their Teleport user is created
without any MFA devices registered. In order to access resources protected by
per-session MFA, they would need to add an MFA device through `tsh mfa add` or
the settings tab in the WebUI.

Attempting to connect to the cluster without an MFA device currently results in
an error telling the user to add their first MFA device.

```console
> tsh ssh server01
ERROR: MFA is required to access this resource but user has no MFA devices; use 'tsh mfa add' to register MFA devices
```

**New MFA prompts**

With SSO as an MFA method, this first time user would instead be prompted to
re-authenticate through their SSO provider for a more seamless experience.

```console
> tsh ssh server01
MFA is required to access Node "server01"
Complete an auth check in your local web browser:
If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:60433/f5858c78-75e1-4f3f-b2c5-69e8e76c0ff9
```

If SSO is not the preferred MFA method in the cluster, the user will also be
notified of how to add an MFA device for future MFA checks. The SSO login
browser will not be opened automatically in order to draw attention to the
output. The link can still be clicked for easy UX.

```console
> tsh ssh server01
MFA is required to access Node "server01"
Complete an SSO auth check in your local web browser. Open it by clicking on the link:
 http://127.0.0.1:60433/f5858c78-75e1-4f3f-b2c5-69e8e76c0ff9
Or register an MFA device with `tsh mfa add` to complete MFA checks with a tap.
```

Note: the MFA prompt is moved to the end to help it stick out over the SSO link.

Stretch: Rather than having the user add an MFA device with additional steps,
they should be guided through MFA registration and allowed to complete their
request afterwards. This would also apply without the SSO MFA option.

```console
> tsh ssh server01
MFA is required to access Node "server01"
Complete an SSO auth check in your local web browser. Open it by clicking on the link:
 http://127.0.0.1:60433/f5858c78-75e1-4f3f-b2c5-69e8e76c0ff9
Or register an MFA device to complete MFA checks with a tap.
Choose device type [WEBAUTHN]:

### User can type to continue the registration process or proceed with SSO re-auth.
```

#### Existing SSO user with registered MFA method

> I have logged into this cluster before with SSO
> I have registered one or more MFA devices
> I want to connect to a resource protected by per-session MFA

**New MFA prompts**

The user will be given the option to pass MFA checks with a registered device
or with SSO.

```console
> tsh ssh server01
MFA is required to access Node "server01"
Complete an auth check in your local web browser:
If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:60433/f5858c78-75e1-4f3f-b2c5-69e8e76c0ff9
Or tap any security key
```

If the user re-authenticates with MFA, the web browser should be closed
automatically.

As in the case above, if SSO is not the preferred MFA method for the cluster,
the browser will not be opened automatically, but the user can still click the
link to proceed with SSO.

```console
> tsh ssh server01
Re-authentication is required to access this node.
Complete an auth check in your local web browser. Open it by clicking on the link:
 http://127.0.0.1:60433/f5858c78-75e1-4f3f-b2c5-69e8e76c0ff9
Or tap any security key
```

#### Teleport Connect

The Teleport connect flow will be identical to `tsh`.

#### WebUI

In the WebUI, opening a new tab for MFA automatically would be awkward UX.
Instead, we should redirect the user in the current tab to complete
authentication with the IdP. Once complete, the user will be redirected back
to the original page requiring MFA.

We will update the current MFA modal to have 3 buttons; `Webauthn`, `SSO`, and
`Cancel`. Either Webauthn or SSO will be highlighted depending on the cluster 
configuration.

### Configuration

#### Enable SSO connector as MFA method.

IdP connectors will gain a new `allow_as_mfa_method` field with possible values
`no`, `yes`, and `only`. The default value is `no`.

When set to `no`, the connector cannot be used for MFA checks, and vice versa
for `yes`.

When set to `only`, the connector can be used for MFA checks but cannot be used
for login. This option is useful in cases where administrators want to set up
a second IdP which performs a subset of the login checks. e.g. just Webauthn
without password.

Note: a split Login/MFA IdP setup requires that both IdPs are set up for the
same list of users. If a user tries to authenticate through the MFA IdP, and
the resulting login's username doesn't match, it will result in an error. The
user will be prompted to add an MFA device to give them a path forward while
administrators sort out the misconfiguration.

```yaml
kind: saml
version: v2
metadata:
  name: okta
spec:
  allow_as_mfa_method: no | yes | only
```

#### Make SSO the only MFA method

Using SSO as an MFA method enables Administrators to maintain tighter control
over what MFA devices can be used for MFA. In some cases, it may make sense to
disable non-SSO MFA methods to prevent users from going around registered SSO
MFA connectors.

For this use case, we will add `second_factor: sso`, which will prevent users
from registering/using MFA devices registered through Teleport.

#### Default MFA connector

Cluster auth preference will also gain the `mfa_connector_name` field to set
a preferred IdP connector for MFA checks. As detailed in the UX section, setting
a preferred IdP connector has some beneficial UX implications.

If `mfa_connector_name` is not set, but `connector_name` is set, that connector
will be used as an MFA method if `allow_as_mfa_method` is set to `yes`.

If neither of the fields above are set, Teleport will look through all
registered connectors in lexical order and return the first one with 
`allow_as_mfa_method` set to `yes` or`only`.

#### Bad configuration

The following two fail states should be prevented:
- `mfa_connector_name` points to a connector with `allow_as_mfa_method = no`
- `connector_name` points to a connector with `allow_as_mfa_method = only`

These fail states will be checked on both connector update and auth preference
update.

### Security

Teleport uses MFA checks for some of its most security focused features, including
per-session MFA, moderated sessions, and MFA for admin actions. Using SSO as an
MFA method opens up the possibility of poorly configured clusters being
vulnerable to attacks ranging from internal users avoiding safe MFA checks to
attackers with a compromised IdP gaining keys to the castle.

#### Opt-in

SSO as an MFA method will be opt-in. Administrators will be instructed through
the docs to only enable an IdP connector as an MFA method if the IdP provider
has strict checks itself (e.g. Administered Webauthn devices, Trusted devices).

Teleport has no way to confirm whether a registered IdP connector follows the
guidelines, but it will display a warning to admins who attempt to enable it.

```console
> tctl edit connector/okta
### sets `allow_as_mfa_method: yes`
Warning: Allowing this IdP provider to be used as an MFA method may reduce the
security of enforced MFA checks for critical security features. This option
should not be enabled unless the IdP provider has strict MFA and/or Device trust
enforcement itself. Continue? (y/N):
```

#### IdP Compromise

In the case of a full-scale IdP compromise, an attacker may have the ability
to auto-provision users with arbitrary permissions.

When device trust is required, newly auto-provisioned SSO users are required
to add their first MFA device from a trusted device. When combined with MFA
security features, such as MFA for Admin actions and per-session MFA, the blast
radius of an IdP compromise is largely contained. The attacker would be
prevented from accessing any critical infrastructure or making any changes
to the cluster's security configuration.

Allowing SSO as an MFA method would bypass the device trust check, opening the
cluster back up to attacks in the case of an IdP compromise. To maintain this
invariant, device trust must be enforced within the SSO MFA check.

### Implementation

#### Privilege Tokens

Privileged tokens are used as transient MFA verification for some operations in
Teleport today, like account resets. These tokens can be generated by passing a
completed MFA challenge to `rpc CreatePrivilegeToken`.

Currently, privilege tokens can only be used in select endpoints as a replacement
for MFA. Privilege Tokens will be supported as a normal MFA response to support
other use cases:

```proto
message MFAAuthenticateResponse {
  oneof Response {
    TOTPResponse TOTP = 2;
    webauthn.CredentialAssertionResponse Webauthn = 3;
    PrivilegeTokenResponse Token = 4;
  }
}
```

Note: Privilege tokens used in this way must be consumed upon verification to
ensure that exchanging a webauthn or totp token for a privilege token through
`rpc CreatePrivilegeToken` has no significant security implications.

#### SSO MFA flow

We will introduce a new SSO login flow that allows clients to get a new privilege
token instead of the standard login credentials. Authenticated clients can create
an sso auth request with `rpc CreateSAMLAuthRequest`, `rpc CreateOIDCAuthRequest`
or `rpc CreateGithubAuthRequest` and set `CreatePrivilegeToken=true` to use this
new flow. Once the client authenticates through the IdP, the client will be given
a privilege token generated for the user.

Note: As is the case with normal SSO login, the login response is encrypted
using a secret key owned by the client so that the token can not be intercepted
in a man in the middle attack.

#### OIDC ACR Values

ACR values can be provided to an OIDC provider in an auth request to specify a
specific type of authentication to perform. This can be useful for Teleport to
specify to the IdP that MFA authentication is required.

However, there are no common ACR values supported by all OIDC providers. Each
provider will support its own arbitrary list of ACR values, if any at all.

For example, Okta supports a phishing resistant (phr) acr value that would
require Fido2/WebAuthn authentication to satisfy the requirement.

Since this will vary between providers and configurations, Teleport will not
use and ACR values by default, though we will document how to set `acr_values`
in the OIDC connector in cases where it is useful.

#### SAML RequestedAuthnContext

A SAML client can provide `RequestedAuthnContext` to request a specific type of
authentication. Similar to ACR values, the supported values vary between
providers and configurations, so Teleport cannot make direct use of them.

Once we find a SAML provider with a supported `RequestedAuthnContext` similar to
Okta's phr ACR value, we may add a `requested_authn_context` field to the SAML
connector resource to support it in certain configurations.

Note: When using ADFS or JumpCloud as a SAML IdP, Teleport requires password
auth by setting `PasswordProtectedTransport` as a minimum `RequestedAuthnContext`.
This minimum will be skipped when the SAML connector is enabled for MFA only.

#### Forced Re-authentication

By default, Teleport will require the user to re-login through their IdP to
pass an MFA check. This will prevent long lasting login sessions from being
used as MFA verification, which could easily be exploited by an attacker with
remote access to the user's machine.

For SAML, this can be achieved by setting `ForceAuthn=true` in the authn
request. For OIDC and Github, setting `max_age=0` in the server redirect URL's
query parameter will force re-auth.

In cases where re-auth is not desired behaviour, such as when the IdP is
configured to prompt for MFA for existing sessions, users can set `force_authn`
or `max_age` in the SAML or OIDC connector respectively to override the default
behavior.

### Proto

**MFAConnectorName**
```diff
message AuthPreferenceSpecV2 {
  // Type is the type of authentication.
  string Type = 1 [(gogoproto.jsontag) = "type"];
  ...
  // ConnectorName is the name of the OIDC or SAML connector. If this value is
  // not set the first connector in the backend will be used.
  string ConnectorName = 3 [(gogoproto.jsontag) = "connector_name,omitempty"];
  ...
+  // MFAConnectorName is the name of an auth connector to use for MFA verification.
+  // If this value is not set, the first connector in the backend with AllowAsMFAMethod
+  // set to YES or ONLY will be used, starting with ConnectorName.
+  string MFAConnectorName = 21 [(gogoproto.jsontag) = "mfa_connector_name,omitempty"];
+  // MFAConnectorType is the type of auth connector to use for MFA verification, if any.
+  // Defaults to the auth Type set above.
+  string MFAConnectorType = 22 [(gogoproto.jsontag) = "mfa_connector_type,omitempty"];
}
```

**AllowAsMFAMethod**
```diff
+// AllowAsMFAMethod represents whether an auth connector can be used as an
+// MFA method or not.
+enum AllowAsMFAMethod {
+  ALLOW_AS_MFA_METHOD_UNSPECIFIED = 0;
+  // NO this auth connector cannot be used as an MFA method.
+  ALLOW_AS_MFA_METHOD_NO = 1;
+  // YES this auth connector can be used as an MFA method.
+  ALLOW_AS_MFA_METHOD_YES = 2;
+  // ONLY means this auth connector can only be used as an MFA method, and not
+  // as a primary authentication mechanism. In order for this MFA method to work,
+  // it must be configured for the users from the primary authentication method.
+  ALLOW_AS_MFA_METHOD_ONLY = 3;
+}

message OIDCConnectorSpecV3 {
  ...
+  // AllowAsMFAMethod represents whether this auth connector can be used as an MFA
+  // method or not.
+  AllowAsMFAMethod AllowAsMFAMethod = 19 [(gogoproto.jsontag) = "allow_as_mfa_method,omitempty"];
}

message SAMLConnectorSpecV2 {
  ...
+  // AllowAsMFAMethod represents whether this auth connector can be used as an MFA
+  // method or not.
+  AllowAsMFAMethod AllowAsMFAMethod = 17 [(gogoproto.jsontag) = "allow_as_mfa_method,omitempty"];
}
 
message GithubConnectorSpecV3 {
  ...
+  // AllowAsMFAMethod represents whether this auth connector can be used as an MFA
+  // method or not.
+  AllowAsMFAMethod AllowAsMFAMethod = 10 [(gogoproto.jsontag) = "allow_as_mfa_method,omitempty"];
}
```

**CreatePrivilegedToken**
```diff
message OIDCAuthRequest {
  ...
+  // CreatePrivilegedToken is an option to create a privileged token instead of creating 
+  // a user session. Privileged tokens can be used in place of standard MFA verification for
+  // privileged actions. This action is only allowed if the auth connector is allowed
+  // to be used as an MFA method and if the user is pre-authenticated (not first time login).
+  bool CreatePrivilegedToken = 19 [(gogoproto.jsontag) = "create_privileged_token,omitempty"];
}

message SAMLAuthRequest {
  ...
+  // CreatePrivilegedToken is an option to create a privileged token instead of creating 
+  // a user session. Privileged tokens can be used in place of standard MFA verification for
+  // privileged actions. This action is only allowed if the auth connector is allowed
+  // to be used as an MFA method and if the user is pre-authenticated (not first time login).
+  bool CreatePrivilegedToken = 18 [(gogoproto.jsontag) = "create_token,omitempty"];
}

message GithubAuthRequest {
  ...
+  // CreatePrivilegedToken is an option to create a privileged token instead of creating 
+  // a user session. Privileged tokens can be used in place of standard MFA verification for
+  // privileged actions. This action is only allowed if the auth connector is allowed
+  // to be used as an MFA method and if the user is pre-authenticated (not first time login).
+  bool CreatePrivilegedToken = 18 [(gogoproto.jsontag) = "create_token,omitempty"];
}
```

**AuthConnectorChallenge**
```diff
message MFAAuthenticateChallenge {
  ...
+  // ProviderChallenge is an an auth provider MFA challenge. If set, the client
+  // will attempt to create an auth request with this connector to acquire a
+  // privileged token as a substitute for local MFA.
+  ProviderChallenge ProviderChallenge = 5;
}

+// ProviderChallenge contains auth connector details for completing a provider
+// MFA challenge.
+message ProviderChallenge {
+  // Type is the auth connector type.
+  string Type = 1;
+  // ID is the auth connector ID.
+  string ID = 2;
+}

message MFAAuthenticateResponse {
  oneof Response {
    TOTPResponse TOTP = 2;
    webauthn.CredentialAssertionResponse Webauthn = 3;
+   PrivilegeTokenResponse Token = 4;
  }
}
```

### Backward Compatibility

It's possible that an old client or proxy version could result in an attempted
SSO login using an auth connector setup for MFA only. The Auth server will
check auth requests against the configured auth connector to ensure that these
login attempts are prevented. Checks will be added to the following endpoints:

- `rpc CreateSAMLAuthRequest`
- `rpc CreateOIDCAuthRequest`
- `rpc CreateGithubAuthRequest`
- `http /v1/saml/requests/validate`
- `http /v1/oidc/requests/validate`
- `http /v1/github/requests/validate`

### Audit Events

SSO MFA requests will be tracked through the existing audit events that contain
`MFADeviceMetadata`. Since SSO as MFA doesn't correspond to actual user MFA
devices in the backend, this metadata will be picked from the auth connector:

- `DeviceName` - Name of the auth connector
- `DeviceID` - UUID of the auth connector
- `DeviceType` - SSO type (`OIDC`, `SAML`, or `Github`)

```proto
message MFADeviceMetadata {
  // Name is the user-specified name of the MFA device.
  string DeviceName = 1 [(gogoproto.jsontag) = "mfa_device_name"];
  // ID is the UUID of the MFA device generated by Teleport.
  string DeviceID = 2 [(gogoproto.jsontag) = "mfa_device_uuid"];
  // Type is the type of this MFA device.
  string DeviceType = 3 [(gogoproto.jsontag) = "mfa_device_type"];
}
```

#### Privilege Tokens

When a privilege token is created, the context of its creation is lost. This
would make it difficult to tie audit event between the SSO login that creates
the token and the Per-session MFA certificate issuance that consumes the token.

To amend this, we will store the details of the MFA device used to create the
privilege token in the backend token resource. This data can then be included
in the `privilege_token.create` and `mfa_auth_challenge.validate` events.

### Additional considerations

#### Temporary SSO MFA device in backend

There are a few nice to have features that could be implemented if we created
temporary SSO MFA devices for users. For example, users would be able to list
SSO MFA devices available to them.

```console
> tsh mfa ls
Name     Type     Added at Last used                     
-------- -------- -------- ---------
yubi     WebAuthn ...      ...
okta-mfa SAML     ...      ...
```

We could then implement a way for clients to choose an SSO MFA device by
providing new flags such as `--mfa-auth-type` and `--mfa-auth-name`.

However, this idea would require significant engineering to handle
automatic creation, automatic deletion, user interaction (tsh mfa rm),
and other edge cases.

For now, users will always be prompted to use the default MFA SSO connector,
as configured in the cluster auth preference.