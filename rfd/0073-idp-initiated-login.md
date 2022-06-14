---
authors: Joel Wejdenstal <jwejdenstal@goteleport.com>
state: draft
---

# RFD 73 - IdP Initated SAML Login

## What

This RFD was spawned by https://github.com/gravitational/teleport/issues/2967. Teleport should support IdP-initiated SAML login. That is, an authentication payload sent via the client to Teleport without Teleport asking for it.

## Why

This is a useful feature to reduce friction and make it easier for users to access Teleport via WebUI. Currently, one has to navigate to the Teleport WebUI and click the "Login" button. This isn't always optimal and we can reduce friction by allowing users to access Teleport WebUI directly from their SSO providers' landing page, such as the Okta dashboard.

## Details

Add a new configurable field to SAML connectors called `idp_initiated` which defaults to false. If set to true, Teleport will accept the SAML assertion sent by the IdP even if it is not requested by Teleport. This allows you to configure the SAML callback in an IdP provider as usual and log in to Teleport directly from the IdP dashboard.

```
kind: saml
version: v2
metadata:
 name: new_saml_connector
spec:
 display: MyIdP
+ idp_initiated: true
 # other fields
```

### Security Considerations

Allowing IdP-initiated login flows comes with a set of security tradeoffs inherent to the use of SAML and the flow itself. In this configuration, Teleport becomes more prone to replay and CSRF attacks since Teleport cannot verify that the user started the login flow intentionally. This opens up attack vectors where an attacker can trick a legitimate user into unknowingly logging into the application with the identity of the attacker.

Due to the security risks above, this feature should be opt-in using the configuration option above and not enabled by default. Since this can reduce usage friction, we should still offer the feature to those that want it.

## References and Resources

[Auth0: Configure SAML Identity Provider-Initiated Single Sign-On](https://auth0.com/docs/authenticate/protocols/saml/saml-sso-integrations/identity-provider-initiated-single-sign-on)
