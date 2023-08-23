---
authors: Joel Wejdenstal <jwejdenstal@goteleport.com>
state: implemented (v10.1.9)
---

# RFD 73 - IdP-initiated Login Flows for SAML

## Required Approvers

* Engineering @zmb3
* Security @reedloden
* Product: (@xinding33 || @klizhentas)

## Terminology

- SP: Service Provider (Teleport)

- IdP: Identity Provider (Okta, Google, etc)

- SAML: Security Assertion Markup Language 2.0

- OIDC: OpenID Connect

## What

This RFD was spawned by https://github.com/gravitational/teleport/issues/2967. Teleport should support IdP-initiated SAML login. That is, an authentication payload sent via the client to Teleport without Teleport asking for it.

## Why

This is a useful feature to reduce friction and make it easier for users to access Teleport via WebUI. Currently, one has to navigate to the Teleport WebUI and click the "Login" button (which in turn directs the user to their SSO provider). This isn't always optimal and we can reduce friction by allowing users to access Teleport WebUI directly from their SSO providers' landing page, such as the Okta dashboard.

## Details

### SAML and OIDC

SAML and OIDC are two different protocols that we support for SSO which have different authentication mechanisms. Modern versions of SAML have mechanisms for both SP-initiated and IdP-initiated login. OIDC on the other hand is based on OAuth2 and while it does have mechanisms for SP-initiated login, it lacks a mechanism for IdP-initiated login.

OIDC does have a feature called Third Party Initiated Login (OpenID Connect Core, Section 4) that allows an initiating third-party application to bounce to the SP which sends you to the IdP and back. This can technically be employed to support IdP-initiated login, but I don't believe that OIDC is a good candidate for this feature. This is because to make IdP-initiated login useful, you need a base platform such as a dashboard (often run by the IdP) that can be used to send the user to the SP and initiate the login. Since SAML has a commonly used mechanism for IdP-initiated login, most IdP's have a feature like this for SAML.

OIDC does not have a commonly used mechanism for IdP-initiated login. This has resulted in IdP support for this feature being scarce and limited. Furthermore, the Third Party Initiated Login feature of OIDC is not well specified in the standard as it is generally used to create a middleman IdP that sits between between a true SP and IdP using a completely custom flow between an SP and an IdP which you both control. Therefore OIDC lacks the tooling and product support from IdP's to make this feature useful.

For this reason, the rest of the RFD focuses purely on SAML and assumes that is the only connector we will support IdP-initiated login for.

### Configuration

Add a new configurable field to SAML connectors called `allow_idp_initiated` which defaults to false. If set to true, Teleport will accept the SAML assertion sent by the IdP even if it is not requested by Teleport. This allows you to configure the SAML callback in an IdP provider as usual and log in to Teleport directly from the IdP dashboard.

```
kind: saml
version: v2
metadata:
  name: new_saml_connector
spec:
  display: MyIdP
+ allow_idp_initiated: true
  # other fields
```

### Security Considerations

Allowing IdP-initiated login flows comes with a set of security tradeoffs inherent to the use of SAML and the flow itself. In this configuration, Teleport becomes more prone to replay and CSRF attacks since Teleport cannot verify that the user started the login flow intentionally. This opens up attack vectors where an attacker can trick a legitimate user into unknowingly logging into the application with the identity of the attacker.

Due to the security risks below, this feature should be opt-in using the configuration option above and not enabled by default. Since this can reduce usage friction, we should still offer the feature to those that want it.

#### Specific risks and Employed Mitigations

- Replay Attacks: Replay attacks are possible by capturing SAML assertions and possibly reusing them. We can attempt to mitigate this in Teleport by storing a cache of SAML assertion IDs that are valid for as long the SAML assertion is itself valid. This may not always be feasible depending on the data we receive from the IdP, but it is an useful mitigation we will employ.

- SAML interception: Since no sort of nonce or request/response system can be employed with randomly generated CSRF relay state tokens or other mechanism, IdP-initiated is prone to being chained with other attacks using assertion injection, interception and stealing and reuse by an attacker. We can attempt to mitigate this by reducing the trust length (the time we consider a response to be valid for login from it's issuance) but this is ultimately just a mitigation and not a perfect solution.

- Flow-change misuse: If a response from an SP-initiated login is somehow captured or stolen, an attacker could use it without a valid request by sending it to the IdP-initiated login callback. To prevent this, we will check the `InResponseTo` field to ensure any stolen responses aren't reused with the IdP-initiated flow.

## References and Resources

[Auth0: Configure SAML Identity Provider-Initiated Single Sign-On](https://auth0.com/docs/authenticate/protocols/saml/saml-sso-integrations/identity-provider-initiated-single-sign-on)

[OWASP: Unsolicited Response (ie. IdP Initiated SSO) Considerations for Service Providers](https://cheatsheetseries.owasp.org/cheatsheets/SAML_Security_Cheat_Sheet.html#unsolicited-response-ie-idp-initiated-sso-considerations-for-service-providers)
