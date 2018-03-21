# SSO for SSH with Teleport

## Introduction

The commercial edition of Teleport allows users to retreive their SSH
credentials via a [single sign-on](https://en.wikipedia.org/wiki/Single_sign-on) 
(SSO) system used by the rest of the organization. 

Examples of supported SSO systems include commercial solutions like [Okta](https://www.okta.com),
[Auth0](https://auth0.com/), [SailPoint](https://www.sailpoint.com/), 
[OneLogin](https://www.onelogin.com/) or [Active Directory](https://en.wikipedia.org/wiki/Active_Directory_Federation_Services), as 
well as open source products like [Keycloak](http://www.keycloak.org).
Other identity management systems are supported as long as they provide an
SSO mechanism based on either [SAML](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language) 
or [OAuth2/OpenID Connect](https://en.wikipedia.org/wiki/OpenID_Connect).


## How does SSO work with SSH?

From the user's perspective they need to execute the following command:

```bash
$ tsh login
```

... once a day to retreive their SSH certificate, assuming that Teleport is
configured with a certificate TTL of 8 hours.

`tsh login` will print a URL into the console, which will open an SSO login
prompt, along with the 2FA as enforced by the SSO provider. If user supplies
valid credentials into the SSO logon proess, Teleport will issue an SSH
certificate.

## Configuring SSO

Teleport works with SSO providers by relying on a concept called
_"authentication connector"_. An auth connector is a plugin which controls how
a user logs in and which group he or she belongs to. 

The following connectors are supported:

* `local` connector type uses the built-in user database. This database can be
  manipulated by `tctl users` command.
* `saml` connector type uses [SAML protocol](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language)
  to authenticate users and query their group membership.
* `oidc` connector type uses [OpenID Connect protocol](https://en.wikipedia.org/wiki/OpenID_Connect) 
  to authenticate users and query their group membership.

To configure [SSO](https://en.wikipedia.org/wiki/Single_sign-on), a Teleport administrator must:

* Update `/etc/teleport.yaml` on the auth server to set the default
  authentication connector.
* Define the connector [resource](admin-guide/#resources) and save it into 
  a YAML file (like `connector.yaml`) 
* Create the connector using `tctl create connector.yaml`.

```bash
# snippet from /etc/teleport.yaml on the auth server:
auth_service:
    # defines the default authentication connector type:
    authentication:
        type: saml 
```

An example of a connector:

```
# connector.yaml
kind: saml
version: v2
metadata:
  name: corporate
spec:
  # display allows to set the caption of the "login" button
  # in the Web interface
  display: "Login with Okta SSO"

  acs: https://teleprot-proxy.example.com:3080/v1/webapi/saml/acs
  attributes_to_roles:
    - {name: "groups", value: "okta-admin", roles: ["admin"]}
    - {name: "groups", value: "okta-dev", roles: ["dev"]}
  entity_descriptor: |
    <paste SAML XML contents here>
```

## Multiple SSO Providers

Teleport can also support multiple connectors. This works via supplying
a connector name to `tsh login` via `--auth` argument:

```bash
$ tsh --proxy=proxy.example.com login --auth=corporate
```

Refer to the following guides to configure authentication connectors of both
SAML and OIDC types:

* [SSH Authentication with Okta](ssh_okta)
* [SSH Authentication with OneLogin](ssh_one_login)
* [SSH Authentication with ADFS](ssh_adfs)
* [SSH Authentication with OAuth2 / OpenID Connect](oidc)

