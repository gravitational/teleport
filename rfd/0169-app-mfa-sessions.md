---
author: Brian Joerger (bjoerger@goteleport.com)
state: draft
---
 
# RFD 169 - Application MFA Sessions

## Required Approvers

- Engineering: @rosstimothy || @zmb3
- Security: @reedloden || @jentfoo

## What

Implement per-session MFA for Application Access.

## Why

As described in the initial [Per-session MFA RFD](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md)
for Server access, Per-session MFA hardens Teleport Access solutions against
compromised user credentials by requiring an additional MFA check to start new
sessions.

This feature has been implemented for all other services except Application
Access and is needed for a strong security posture across all Teleport access
solutions.

## Details

### Existing Per-session MFA Implementations

The majority of this implementation will follow that of the existing Per-session
MFA implementations outlined in previous RFDs:

- SSH Access - [RFD 14](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md)
- DB Access - [RFD 90](https://github.com/gravitational/teleport/blob/master/rfd/0090-db-mfa-sessions.md)
- Kube Access - [RFD 121](https://github.com/gravitational/teleport/blob/master/rfd/0121-kube-mfa-sessions.md)

In this RFD I will primarily outline the changes necessary to implement
Per-session MFA for Application Access rather than cover all of the previously
discussed design principles for Per-session MFA in general.

### App Sessions

An App session is a Web Session targeted at a specific application known by the
Teleport cluster. App Sessions have a UUID, a cookie, a private key, and a TLS
certificate.

In order to pass the Per-session MFA check, the App Session TLS certificate
must be marked as MFA verified.

Teleport clients have 2 ways to interface with an App Session they've created:

Web clients use the App Session's bearer token and session ID in an HTTP header
to connect to the Proxy. The Proxy gets the matching App Session, compares the
client cookie to the expected cookie, and proxies the client to the App service
using the App Session's credentials.

Direct clients request an additional x509 certificate from the Auth server with
the `AppSessionIDASN1ExtensionOID` set to match the App Session. This
certificate is then used to connect to the Proxy. The Proxy checks verifies the
client certificate, parses the session ID, and retrieves the associated App
Session before proxying the client to the App service using the App Session's
credentials.

Note: While it is not completely necessary for the additional x509 certificate
of direct clients to be MFA verified, they should be in order to signify that
the associated App Session is MFA verified. In other words, the Auth Server
should not sign non-MFA verified certs for MFA verified App Sessions.

#### `rpc CreateAppSession`

Clients will be able to pass an MFA response into `rpc CreateAppSession` in
order to generate an MFA verified App Session.

```diff
// CreateAppSessionRequest contains the parameters to request a application web session.
message CreateAppSessionRequest {
  reserved 2;
  // Username is the name of the user requesting the session.
  string Username = 1 [(gogoproto.jsontag) = "username"];
  // PublicAddr is the public address the application.
  string PublicAddr = 3 [(gogoproto.jsontag) = "public_addr"];
  // ClusterName is cluster within which the application is running.
  string ClusterName = 4 [(gogoproto.jsontag) = "cluster_name"];
  // AWSRoleARN is AWS role the user wants to assume.
  string AWSRoleARN = 5 [(gogoproto.jsontag) = "aws_role_arn"];
  // AzureIdentity is Azure identity the user wants to assume.
  string AzureIdentity = 6 [(gogoproto.jsontag) = "azure_identity"];
  // GCPServiceAccount is the GCP service account the user wants to assume.
  string GCPServiceAccount = 7 [(gogoproto.jsontag) = "gcp_service_account"];
+ // MFAResponse is a response to a challenge from a user's MFA device.
+ // An optional field, that when provided, the response will be validated
+ // and the ID of the validated MFA device will be stored in the certificate.
+ MFAAuthenticateResponse MFAResponse = 8 [(gogoproto.jsontag) = "mfa_response,omitempty"];
}
```

The TTL of app sessions created with `CreateAppSession` will default to the TTL
of the user's current certificates. This is the existing behaviour.

#### `rpc GenerateUserCerts`

Currently, `rpc CreateAppSession` is used by direct clients to create an App
Session, retrieve it's session ID, and request local app certs linked to the
session ID. In order for direct clients to mark both the App Session cert and
local app cert as MFA verified, the user would need to perform two MFA checks
and two roundtrips.

This should be avoided by allowing clients to call `rpc GenerateUserCerts` to
generate both the local app cert and App Session with a single MFA response.
The client only needs the local app cert after all.

The TTL of both the local app cert and the App Session cert will be set to 1
minute by default. However, the client can set `UserCertsRequest.Requester` to
`TSH_APP_LOCAL_PROXY` to signal that the client is going to set up a local app
proxy (`tsh proxy app`, or Teleport Connect Proxy). In this case, the TTL will
be set to the TTL of the user's current credentials.

```diff
message UserCertsRequest {
  ...
  // Requester is the name of the service that sent the request.
  enum Requester {
    // UNSPECIFIED is set when the requester in unknown.
    UNSPECIFIED = 0;
    // TSH_DB_LOCAL_PROXY_TUNNEL is set when the request was sent by a tsh db local proxy tunnel.
    TSH_DB_LOCAL_PROXY_TUNNEL = 1;
    // TSH_KUBE_LOCAL_PROXY is set when the request was sent by a tsh kube local proxy.
    TSH_KUBE_LOCAL_PROXY = 2;
    // TSH_KUBE_LOCAL_PROXY_HEADLESS is set when the request was sent by a tsh kube local proxy in headless mode.
    TSH_KUBE_LOCAL_PROXY_HEADLESS = 3;
+   // TSH_APP_LOCAL_PROXY is set when the request was sent by a tsh app local proxy.
+   TSH_APP_LOCAL_PROXY = 4;
  }
  // RequesterName identifies who sent the request.
  Requester RequesterName = 17 [(gogoproto.jsontag) = "requester_name"];
  ...
}
```

Note: Clients should not store these longer-lived proxy local app certs on disk.
Instead, they should be held in memory and should be lost once the local proxy
is closed by the user.

### UX

#### `tsh app login`

Users can login with `tsh app login` to generate an MFA verified App session
and local certs with a 1-minute TTL. The corresponding App certs will be stored
in `~/.tsh` to preserve existing behavior.

```console
> tsh app login grafana
MFA is required to access Application "grafana"
Tap any security key
Detected security key tap
Logged into app grafana. Example curl command:

curl \
  --cert /home/bjoerger/.tsh/keys/root.example.com/dev-app/root.example.com/grafana-x509.pem \
  --key /home/bjoerger/.tsh/keys/root.example.com/dev \
  https://grafana.root.example.com:3080

> curl \
  --cert /home/bjoerger/.tsh/keys/root.example.com/dev-app/root.example.com/grafana-x509.pem \
  --key /home/bjoerger/.tsh/keys/root.example.com/dev \
  https://grafana.root.example.com:3080
<a href="/login">Found</a>.
```

#### `tsh proxy app`

Users can use `tsh proxy app` to create a local proxy for the app. This command
will generate an MFA verified App session and local app certificate with a TTL
equal to the user's current credentials.

The local app certificate will not be saved to disk to prevent an attacker from
hijacking a long-lived MFA verified App session.

```console
> tsh proxy app --port 8080 grafana
MFA is required to access Application "grafana"
Tap any security key
Detected security key tap
Proxying connections to grafana on 127.0.0.1:8080

### Switch tabs
> curl 127.0.0.1:8080 
<a href="/login">Found</a>.
```

When per-session MFA is required, `tsh proxy app` will use existing local app
certificates acquired with `tsh app login`. Since these certs may expire in a
minute or less, `tsh proxy app` should also handle reissuing of certs after the
initial certs expire.

#### WebUI and Teleport Connect

App Sessions started through the WebUI or Teleport Connect will have a TTL equal
to the TTL of the user's existing credentials.

The WebUI and Teleport Connect will handle MFA prompts to start App sessions in
the same way that they do for SSH sessions. This involves opening an MFA prompt
modal and waiting for the user to complete or cancel the MFA check.

Note: This modal should steal focus to ensure the user knows to tap their MFA
key. This is primarily important in the Teleport Connect local-proxy use case.

### Security

For the most part, this RFD does not introduce any security implications that
have not been accepted in the previous Per-session MFA RFDs linked above.

The primary difference is that in Application access, we have two sets of
secrets to manage:

- Client Secret - one of the following:
  - An App-only TLS certificate targeting the App Session UUID
  - The App session bearer token
- Proxy Secret - the App Session private key and TLS certificate

#### Client secret protection - TTL control

The client requests access to the application through the Proxy. The Proxy then
verifies the client secret and attempts to connect to the Application using its
own secret. If successful, the client gets proxied to the application.

For App sessions started by a local client, the App-only TLS certificate will
be protected by either having a short TTL (1 minute) or being kept strictly in
memory for local proxies.

#### Client secret vulnerability - bearer token

On the other hand, for App sessions started through the WebUI, the bearer token
(browser cookie) could potentially be stolen. This bearer token is all an
attacker would need to hijack a user's existing app session, MFA verified or
not. There is currently no plan to mitigate this attack vector, but if needed
we could consider enforcing a shorter TTL for bearer tokens (15 minutes?) to
require the user to perform additional periodic MFA checks.

#### Proxy secret protection - App Session ownership

The bearer token *or* Proxy secret could also be read by a Teleport User with
ownership of the App Session. This means that an attacker with access to a
user's normal Teleport credentials could read the user's current MFA-verified
App session secrets and hijack a supposedly MFA verified session, either using
the bearer token to connect through the proxy or the Proxy secret to try to
connect directly.

In order to protect against this attack vector, users will no longer be allowed
to read their own App Session secrets. As far as I can tell, we do not support
clients connecting directly to applications - App Sessions are always proxied
through the Proxy Service.

Note: this change is also crucial to Hardware Key Support for App Access, since
these App Session secrets will be attested as `web_session`.

## Additional considerations

### TOTP

As we continue to sunset TOTP, it is not currently feasible to implement TOTP
support for Application Session MFA in the WebUI or Teleport Connect. As is
the case with desktop access, only WebAuthn will be supported.

TOTP will be supported through `tsh` until it is ultimately deprecated in favor
of WebAuthn.

### Extending the `/x-teleport-auth` login endpoint to support MFA directly

[RFD 14](https://github.com/gravitational/teleport/blob/master/rfd/0014-session-2FA.md#web-apps)
lays out a way to support MFA prompts directly in the browser. However, this
solution adds additional layers of complexity to Application access and
deviates from the standard MFA flow used by other Teleport services.

As far as I can tell, there are no UX or security benefits to that approach.
