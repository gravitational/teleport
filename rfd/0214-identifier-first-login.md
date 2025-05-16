---
authors: Yassine Bounekhla (yassine@goteleport.com)
---

# RFD 0214 - Notifications

## Required Approvals

- Engineering: @zmb3
- Product: @roraback

## What

Identifier-first login allowing support for mapping SSO providers to users.

## Why

Currently, every user of a cluster sees a list of all SSO identity providers setup with Teleport on the
login page. This results in unnecessary clutter and potential confusion, as it is likely that
they can/will only ever use one. Allowing cluster admins to map SSO providers to particular users or groups of users mitigates this by ensuring that a user only sees the identity providers relevant to them.

For example, if contractors need access to my cluster and they use their own identity provider
(ie. Okta), their login flow can be simplified by ensuring that they only ever have the option to log
in using it, as opposed to also seeing the identity providers that my regular users use but that are
unusable to them, and vice versa.

Additionally, users will be remembered so that the next time they visit the login page, they'll
automatically be shown the auth connector(s) relevant to them.

## Details

Identifier-first login will be configured in an auth connector's resource yaml. Any given auth
connector can be configured with a `user_matchers` field containing a set of RegEx pattern(s) that
can match usernames. When a user visits the login page, they will be prompted to provide a username,
after which they'll be shown all the auth connectors configured to match it.

If this field is not set, the auth connector will default to matching all users.

It should be noted that this username does not represent anything beyond being a reference to map to
connectors on the Teleport side, and users can enter any arbitrary string. For example, if a user
enters `joe@foo.com` as their username and are mapped to an Okta connector, but their Okta user is
actually `bob@foo.com`, they will be logged into Teleport as `bob@foo.com`. This detail should be
explicitly mentioned in the docs.

### Example matchers

#### Match all usernames that end in `@foo.com`

```yaml
user_matchers: [".*@foo\.com$"]
```

### Match `joe@foo.com` and all usernames that end in `@goteleport.com`

```yaml
user_matchers: ["^joe@foo.com$", ".*@goteleport\.com$"]
```

### Example auth connector configuration:

```yaml
kind: saml
metadata:
  name: testconnector
  revision: 60443273-15b2-4e5b-a2e2-9338439eac37
spec:
  acs: https://<cluster-url>/v1/webapi/saml/acs/testconnector
  attributes_to_roles:
    - name: groups
      roles:
        - editor
      value: okta-admin
    - name: groups
      roles:
        - auditor
      value: okta-auditor
    - name: groups
      roles:
        - access
      value: okta-dev
  audience: https://<cluster-url>/v1/webapi/saml/acs/testconnector
  user_matchers: ["^joe@foo.com$", ".*@goteleport\.com$"]
  cert: ''
  display: Okta
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://www.example.com/00000000000000000000">
    ...
    </md:EntityDescriptor>
  entity_descriptor_url: ''
  issuer: http://www.example.com/00000000000000000000
  service_provider_issuer: https://teleport.com/v1/webapi/saml/acs/testconnector
  signing_key_pair:
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    private_key: |
      -----BEGIN RSA PRIVATE KEY-----
      ...
      -----END RSA PRIVATE KEY-----
  sso: https://www.example.com/app/00000000000000000000/sso/saml
version: v2
```

### Implementation

Upon loading the Web UI, the server returns a web config which contains some details about the cluster
including the Teleport edition, login methods and auth connectors (stored in `window['GRV_CONFIG']`).
This will now also include the RegEx matchers per auth connector.

All matching and filtering will be done on the client side. This is so that we can avoid having to
introduce any additional calls to the backend once the user has provided a username. This is faster
and more performant, and also reduces complexity since it means we won't need to add and maintain any new
endpoints. Remembered usernames will be kept in `localStorage`.

#### Proto changes

`SAMLConnectorSpecV2`, `OIDCConnectorSpecV3`, and `GithubConnectorSpecV3` will need to be updated to include the `user_matchers` field.

```proto
// SAMLConnectorSpecV2 is a SAML connector specification.
message SAMLConnectorSpecV2 {
  ...
  // UserMatchers is a set of RegEx patterns to narrow down which username(s) this auth connector should
  // match for identifier-first login.
  repeated string UserMatchers = 19 [(gogoproto.jsontag) = "user_matchers,omitempty"]
}
```

```proto
// OIDCConnectorSpecV3 is an OIDC connector specification.
//
// It specifies configuration for Open ID Connect compatible external
// identity provider: https://openid.net/specs/openid-connect-core-1_0.html
message OIDCConnectorSpecV3 {
 ...
  // UserMatchers is a set of RegEx patterns to narrow down which username(s) this auth connector should
  // match for identifier-first login.
  repeated string UserMatchers = 21 [(gogoproto.jsontag) = "user_matchers,omitempty"]
}
```

```proto
// GithubConnectorSpecV3 is a Github connector specification.
message GithubConnectorSpecV3 {
   ...
  // UserMatchers is a set of RegEx patterns to narrow down which username(s) this auth connector should
  // match for identifier-first login.
  repeated string UserMatchers = 10 [(gogoproto.jsontag) = "user_matchers,omitempty"]
}
```

### UX

<img src="./assets/0214-identifier-first-flow.png" width="800" />

Upon visiting the login page for the first time, if there are one or more auth connectors in the cluster
with an explicitly defined `user_matchers` field, they will be prompted to enter a username (1a).
If there are no auth connectors with an explicitly defined `user_matchers` field, the login page will
be the same as it is now prior to the implementation of this feature (with all auth connectors listed),
this is because it means that all auth connectors match all users, and thus prompting the user for
their username is redundant and a pointless extra step.

After a user enters their username, their username will be matched to the auth connector(s) relevant to
them based on their `user_matchers` field, and those connectors will be displayed to them. If they are
only matched to one auth connector, they will be taken directly to that identity provider to log in.
Their username input will be stored in `localStorage`, so the next time they visit the login page, they
won't need to enter their username again. If they are matched to no connectors, an error
message will be displayed.

If a remembered user only has one connector, they will be shown a single "Log in with <connector-name>"
button to log in with that connector. If a remembered user has no connectors, which can happen if auth
connector configs were changed to exclude that user, they will be directed back to the default initial
login screen and their remembered username will be cleared from memory.

### Security

This is a relatively safe feature, as we are not exposing any sensitive information. The only security
concern would be that an attacker could theoretically poke around and get an idea of which usernames
are mapped to auth connectors. For example, if they see that `joe@foo.com` is mapped to auth connectors,
then they'll know that that email is likely real and they may try to target it with a phishing
or social engineering attack.

### Backward Compatibility

As mentioned previously, any auth connector which does not have an explicitly defined `user_matchers`
field will default to matching all users. This means that for existing auth connectors (which don't have
this option), the behavior and UX will be unchanged until they define matchers for at least one
connector.

Given that the filtering will be done in the frontend, API backwards compatibility should not be a
concern.
