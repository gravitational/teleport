---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD XXX - SSO as MFA Method

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

Note: the examples below are for `tsh`, but the same flow should be created for
Teleport Connect and the WebUI.

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

**List MFA devices**

When the user lists their MFA methods with `tsh mfa ls`, they will see their
registered MFA devices, as well as any SSO auth connectors that can be used
for MFA checks.

```console
> tsh mfa ls
Name     Type     Added at Last used                     
-------- -------- -------- ---------
yubi     WebAuthn ...      ...
okta-mfa SSO      ...      ...
```

Note: The SSO MFA device will correspond to an actual SSO MFA device created
for the user. This will be important for technical reasons explained later.
It also means a could remove the MFA method temporarily, or maintain other
states such as "preferred MFA method", "disabled MFA method", etc., if such
states are ever added.

Note to self: we should automatically delete the MFA method if we ever find
the connector is defunct.

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

### Configuration

#### Enable SSO connector as MFA method.

IdP connectors will gain a new `allow_as_mfa_method` field with possible values
`no`, `yes`, and `only`. 

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
registered connectors and return the first one with `allow_as_mfa_method` set
to `yes` or`only`.

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

A logged in user can start a separate SSO login flow that returns a privileged
token instead of a login session. This will be done through
`rpc Create[connector-type]AuthRequest` instead of the unauthenticated webapi
endpoints `/webapi/[connector-type]/login/console`.

Privileged tokens are used as transient MFA verification for some operations in
Teleport today, like account resets. This token can only be received after an
MFA challenge has been completed, or now if an SSO-MFA challenge has been
completed, so it is a safe substitute.

Note: As is the case with normal SSO login, the login response is encrypted
using a secret key owned by the client.

TODO: expand on details.

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

// U2F defines settings for U2F device.
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
+  // MFAConnectorChallenge is an auth connector MFA challenge. If set, the client
+  // will attempt to create an auth request with this connector to acquire a
+  // privileged token as a substitute for local MFA.
+  MFAConnectorChallenge MFAConnectorChallenge = 5;
}

+// MFAConnectorChallenge contains auth connector details for for completing an
+// auth connector MFA challenge.
+message MFAConnectorChallenge {
+  // Type is the auth connector type.
+  string Type = 1;
+  // ID is the auth connector ID.
+  string ID = 2;
+}

message MFAAuthenticateResponse {
  oneof Response {
    ...
+    // TokenID is a privileged token ID used as a substitute for local MFA.
+    string TokenID = 4;
  }
}
```

### Backward Compatibility

N/A

### Audit Events

TODO

### Observability

TODO

### Product Usage

TODO

### Test Plan

TODO
