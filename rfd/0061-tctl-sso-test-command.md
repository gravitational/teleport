---
authors: Krzysztof Skrzętnicki <krzysztof.skrzetnicki@goteleport.com>
state: draft
---

# RFD 61 - `tctl sso test` command

## What

This RFD proposes new subcommand for the `tctl` tool: `sso test`. The purpose of this command is to perform validation
of auth connector in SSO flow prior to creating the connector resource with `tctl create ...` command.

To accomplish that the definition of auth connector is read from file and attached to the auth request being made. The
Teleport server uses the attached definition to proceed with the flow, instead of using any of the stored auth connector
definitions.

The login flow proceeds as usual, with some exceptions:

- Embedded definition of auth connector will be used whenever applicable.
- The client key is not signed at the end of successful flow, so no actual login will be performed.
- Once the flow is finished (with either success or failure) debugging information is returned to the client.

The following kinds of auth connectors will be supported:

- SAML (Enterprise only)
- OIDC (Enterprise only)
- Github

## Why

Currently, Teleport offers no mechanism for testing the SSO flows prior to creating the connector, at which point the
connector is immediately accessible for everyone. Having a dedicated testing flow using single console terminal for
initiating the test and seeing the results would improve the usability and speed at which the changes to connectors can
be iterated. Decreased SSO configuration time contributes to improved "time-to-first-value" metric of Teleport.

## Details

### UX

_TODO: more examples, better messages._

The user initiates the flow by issuing command such as `tctl --proxy=<proxy.addr> sso test <auth_connector.yaml>`. The
resource is loaded, and it's kind is determined. If the connector kind is supported, the SSO flow is initiated to
appropriate endpoint. In the same manner as `tsh login --auth=<sso_auth>` opens the browser to perform the login, the
user is redirected to the browser as well. Once the flow is finished in any way, the user is notified of that fact along
with any debugging information that has been passed by the server (e.g. claims, mapped roles, ...).

- Example of successful test:

```bash
$ tctl --debug --proxy=teleport.example.com sso test auth_connector.yaml
INFO [AUTH] Connector type: SAML
DEBU [SAML] SSO: https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml services/saml.go:98
DEBU [SAML] Issuer: http://www.okta.com/exk14fxcpjuKMcor30h8 services/saml.go:99
DEBU [SAML] ACS: https://teleport.example.com/v1/webapi/saml/acs services/saml.go:100
INFO [CLIENT]    Waiting for response at: http://127.0.0.1:59150. client/redirect.go:137
If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:59150/07abac10-30bd-4fe9-bdf8-df4eff780bcb
Test succesful!
login: foo@bar.com
claims: ...
roles_to_groups: ...
```

- Example of failure:

```bash
$ tctl --debug --proxy=teleport.example.com sso test auth_connector.yaml
INFO [AUTH] Connector type: SAML
DEBU [SAML] SSO: https://dev-813354.oktapreview.com/app/dev-813354_krzysztofssodev_1/exk14fxcpjuKMcor30h8/sso/saml services/saml.go:98
DEBU [SAML] Issuer: http://www.okta.com/exk14fxcpjuKMcor30h8 services/saml.go:99
DEBU [SAML] ACS: https://teleport.example.com/v1/webapi/saml/acs services/saml.go:100
INFO [CLIENT]    Waiting for response at: http://127.0.0.1:59150. client/redirect.go:137
If browser window does not open automatically, open it by clicking on the link:
 http://127.0.0.1:59150/07abac10-30bd-4fe9-bdf8-df4eff780bcb
Test failed!
Error: no audience in SAML assertion info
```

### Passing details to SSO callback on failed login

Currently, in case of SSO error, the console client is not informed of failure. Unless interrupted (Ctrl-C/SIGINT) the
request will simply time out after few minutes:

```
If browser window does not open automatically, open it by clicking on the link:
http://127.0.0.1:59544/452e0ecc-797d-488e-a9be-ffc23b21fcf8
DEBU [CLIENT]    Timed out waiting for callback after 3m0s. client/weblogin.go:324
```

The user can tell the SSO flow has failed because the browser they were using is redirected to one of two error URLs:

```go
// LoginFailedRedirectURL is the default redirect URL when an SSO error was encountered.
LoginFailedRedirectURL = "/web/msg/error/login"

// LoginFailedBadCallbackRedirectURL is a redirect URL when an SSO error specific to
// auth connector's callback was encountered.
LoginFailedBadCallbackRedirectURL = "/web/msg/error/login/callback"
```

We want to change this behaviour for **ALL** SSO flows (both testing ones and normal ones) so that client callback is
always called. In case of failure the callback will omit the login information (as it did not successfully happen), but
may potentially include debugging information as well as final redirect URL (which will likely point to one of the above
URLs).

This change of flow will allow for:

- prompt closure of failed console logins - no need for timeouts or manual interruption of login
- avenue for reporting of rich information about the cause of failure in console (in addition to Teleport debug logs)
- keeping the existing error messages in the browser.

### Implementation details

There are several conceptual pieces to the implementation:

1. Extending auth requests with embedded connector details. New set of endpoints will be
   created: `"/webapi/{oidc,saml,github}/login/sso_test"`, similar to
   existing `"/webapi/{oidc,saml,github}/login/console"` endpoints. The new endpoints will be authenticated and accept
   the additional parameter for embedded auth connector definition. The embedded definition will be stored with the auth
   request. There are numerous types involved here which will need to be updated, along with conversions between them.
   For example, among others, these types will need to be extended:

- [`services.SAMLAuthRequest`](https://github.com/gravitational/teleport/blob/8c4bf751b211e82b555653a9aee6c6c5bf39411f/lib/services/identity.go#L421)
- [`services.OIDCAuthRequest`](https://github.com/gravitational/teleport/blob/8c4bf751b211e82b555653a9aee6c6c5bf39411f/lib/services/identity.go#L352)
- [`services.GithubAuthRequest`](https://github.com/gravitational/teleport/blob/8c4bf751b211e82b555653a9aee6c6c5bf39411f/lib/services/identity.go#L286)

2. Making the backend aware of the testing flow. Right now there is no concept of "dry run" login flow, this needs to be
   changed as appropriate. For example for SAML this will mean
   updating [Server.ValidateSAMLResponse()](https://github.com/gravitational/teleport/blob/9b8b9d6d0c115d43d31d53c47db3050e27edbc4a/lib/auth/saml.go#L317)
   , [Server.validateSAMLResponse()](https://github.com/gravitational/teleport/blob/9b8b9d6d0c115d43d31d53c47db3050e27edbc4a/lib/auth/saml.go#L361)
   and others to skip creation of web sessions and signing the client public key.

3. Extending the callback with debugging information and calling it on failed SSO login. This means updating the calling
   logic (for SAML this
   is [Handler.samlACS(()](https://github.com/gravitational/teleport/blob/940c83c16133fd9fc506f780e71e7b94edabf9d6/lib/web/saml.go#L89)
   function) and receiving logic (
   i.e. [Redirector.callback()](https://github.com/gravitational/teleport/blob/4db05acbef43847c4d899d54c3b1de2301234cc2/lib/client/redirect.go#L200))
   .

The implementation of this RFD for different kinds of connectors should be largely independent. As such, the first
iteration will implement this functionality for SAML, while the lessons learned will help shape the implementations for
OIDC and Github.

### Security

The new `"/webapi/{oidc,saml,github}/login/sso_test"` endpoints will be authenticated,
unlike `"/webapi/{oidc,saml,github}/login/console"`. The user will be required to have `create` access to appropriate
resource type being tested (e.g. `types.KindSAML`).