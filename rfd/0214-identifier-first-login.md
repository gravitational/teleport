---
authors: Yassine Bounekhla (yassine@goteleport.com)
---

# RFD 0214 - Identifier-first Login

## Required Approvals

- Engineering: @zmb3
- Product: @roraback

## What

Identifier-first login allowing support for mapping SSO providers to users.

## Why

Currently, every user of a cluster sees a list of all SSO identity providers set up with Teleport on the
login page. This results in unnecessary clutter and potential confusion, as it is likely that
they can/will only ever use one. Allowing cluster admins to map SSO providers to particular users or groups of users mitigates this by ensuring that a user only sees the identity providers relevant to them.

For example, if contractors need access to my cluster and they use their own identity provider
(ie. Okta), their login flow can be simplified by ensuring that they only ever have the option to log
in using it, as opposed to also seeing the identity providers that my regular users use but that are
unusable to them, and vice versa.

Additionally, users will be remembered so that the next time they visit the login page, they'll
automatically be shown the auth connector(s) relevant to them.

### UX

<img src="./assets/0214-identifier-first-flow.png" width="800" />

#### First-time user visiting a cluster with one or more auth connectors with an explicitly defined `user_matchers` field

_A. Matches only one auth connector_

1. Upon visiting the login page for the first time, the user will be prompted to enter a username (1a).
2. After the user enters their username, they will be taken directly to the identity provider they matched to log in. Their
   username input will be stored in `localStorage`, so the next time they visit the login page, they won't need to enter their username again.

_B. Matches 2 or more auth connectors_

1. Upon visiting the login page for the first time, the user will be prompted to enter a username (1a).
2. After a user enters their username, their username will be matched to the auth connectors relevant to them based on their `user_matchers`
   field, and those connectors will be displayed to them. Their username input will be stored in `localStorage`, so the next time they visit the login page,
   they won't need to enter their username again.
3. Once they click on a connector, they will be taken to that identity provider to log in.

_C. Matches 0 auth connectors_

1. Upon visiting the login page for the first time, the user will be prompted to enter a username (1a).
2. After a user enters their username, an error message will be displayed.

#### Returning user visiting a cluster with one or more auth connectors with an explicitly defined `user_matchers` field

_A. Correct user_

1. Upon visiting the login page, the returning user will see a welcome view with their auth connector(s) displayed to them.
2. Once they click on a connector, they will be taken to that identity provider to log in.

_B. Different user on same device_

1. Upon visiting the login page, the user will see the welcome page for the last person to log in on that device.
2. The user can click "not you?" to reset the login page to default and clear their username from memory. This puts them back into the use case of a first-time user.

_C. Remembered user with no auth connectors_

This can happen if auth connector configs were changed to exclude that user. In this case, upon visiting the login page, they will be directed back to the default initial login screen and their remembered username will be cleared from memory.

#### Any user visiting a cluster with no explicitly defined `user_matchers` fields in any auth connector

If there are no auth connectors with an explicitly defined `user_matchers` field, the login page will
be the same as it is now prior to the implementation of this feature (with all auth connectors listed),
this is because it means that all auth connectors match all users, and thus prompting the user for
their username is redundant and a pointless extra step.

## Details

Identifier-first login will be configured in an auth connector's resource yaml. Any given auth
connector can be configured with a `user_matchers` field containing a set of glob pattern(s) that
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
user_matchers: ['*@foo.com']
```

#### Match `joe@foo.com` and all usernames that end in `@goteleport.com`

```yaml
user_matchers: ['joe@foo.com', '*@goteleport.com']
```

#### Example auth connector configuration:

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
  user_matchers: ['joe@foo.com', '*@goteleport.com']
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
including the Teleport edition, login methods and auth connectors (stored in `window['GRV_CONFIG']`). This
config will now also include whether identifier-first login is enabled, which is determined by whether there
are one or more auth connectors with `user_matchers` explicitly defined.

If identifier-first login is enabled, the login page on the client will prompt for the user to enter their username,
after which a `POST` request will be made to a new endpoint `/v1/webapi/authconnectors` which will return the list
of connectors that match the username, if any. If successful, the connectors will be displayed to the client, and
their email will be remembered in `localStorage`. If there is already a username remembered, the request to get
matching auth connectors will be done automatically on page load using that username.

##### `POST /v1/webapi/authconnectors`

Purpose: Given a username, return a list of all matching auth connectors based on their `user_matchers` field.

###### Example Request Body

```json
{
  "username": "foo@bar.com"
}
```

###### Example Response

```json
{
  "connectors": [
    {
      "name": "okta-connector",
      "displayName": "Okta",
      "type": "saml",
      "url": "/v1/webapi/saml/sso?connector_id=:providerName\u0026redirect_url=:redirect"
    },
    {
      "name": "auth0-connector",
      "displayName": "Auth0",
      "type": "oidc",
      "url": "/v1/webapi/oidc/sso?connector_id=:providerName\u0026redirect_url=:redirect"
    }
  ]
}
```

##### Proto changes

`SAMLConnectorSpecV2`, `OIDCConnectorSpecV3`, and `GithubConnectorSpecV3` will need to be updated to include the `user_matchers` field.

```proto
// SAMLConnectorSpecV2 is a SAML connector specification.
message SAMLConnectorSpecV2 {
  ...
  // UserMatchers is a set of glob patterns to narrow down which username(s) this auth connector should
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
  // UserMatchers is a set of glob patterns to narrow down which username(s) this auth connector should
  // match for identifier-first login.
  repeated string UserMatchers = 21 [(gogoproto.jsontag) = "user_matchers,omitempty"]
}
```

```proto
// GithubConnectorSpecV3 is a Github connector specification.
message GithubConnectorSpecV3 {
   ...
  // UserMatchers is a set of glob patterns to narrow down which username(s) this auth connector should
  // match for identifier-first login.
  repeated string UserMatchers = 10 [(gogoproto.jsontag) = "user_matchers,omitempty"]
}
```

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
