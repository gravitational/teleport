<h1>Single Sign-On (SSO) for SSH</h1>

## Introduction

The commercial edition of Teleport allows users to login and retrieve their SSH
credentials through a [Single Sign-On](https://en.wikipedia.org/wiki/Single_sign-on)
(SSO) system used by the rest of the organization.

Examples of supported SSO systems include commercial solutions like [Okta](https://www.okta.com),
[Auth0](https://auth0.com/), [SailPoint](https://www.sailpoint.com/),
[OneLogin](https://www.onelogin.com/) or [Active Directory](https://en.wikipedia.org/wiki/Active_Directory_Federation_Services), as
well as open source products like [Keycloak](http://www.keycloak.org).
Other identity management systems are supported as long as they provide an
SSO mechanism based on either [SAML](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language)
or [OAuth2/OpenID Connect](https://en.wikipedia.org/wiki/OpenID_Connect).


## How does SSO work with SSH?

From the user's perspective they need to execute the following command login:

```bsh
# this command will automatically open the default web browser and take a user
# through the login process with an SSO provider:
$ tsh login --proxy=proxy.example.com

# output:
If browser window does not open automatically, open it by clicking on the link:
http://127.0.0.1:45235/055a310a-1099-43ea-8cf6-ffc41d88ad1f
```

Teleport will wait for up to 3 minutes for a user to authenticate. If authentication
succeeds, Teleport will retrieve an SSH certificate and will store it in
`~/.tsh/keys/proxy.example.com` directory and also will add it to an
[SSH agent](https://en.wikipedia.org/wiki/Ssh-agent) if there's one running.

## Configuring SSO

Teleport works with SSO providers by relying on a concept called
_"authentication connector"_. An auth connector is a plugin which controls how
a user logs in and which group he or she belongs to.

The following connectors are supported:

* `local` connector type uses the built-in user database. This database can be
  manipulated by the `tctl users` command.
* `saml` connector type uses the [SAML protocol](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language)
  to authenticate users and query their group membership.
* `oidc` connector type uses the [OpenID Connect protocol](https://en.wikipedia.org/wiki/OpenID_Connect)
  to authenticate users and query their group membership.

To configure [SSO](https://en.wikipedia.org/wiki/Single_sign-on), a Teleport administrator must:

* Update `/etc/teleport.yaml` on the auth server to set the default
  authentication connector.
* Define the connector [resource](admin-guide/#resources) and save it into
  a YAML file (like `connector.yaml`)
* Create the connector using `tctl create connector.yaml`.

```yaml
# snippet from /etc/teleport.yaml on the auth server:
auth_service:
    # defines the default authentication connector type:
    authentication:
        type: saml
```

An example of a connector:

```yaml
# connector.yaml
kind: saml
version: v2
metadata:
  name: corporate
spec:
  # display allows to set the caption of the "login" button
  # in the Web interface
  display: "Login with Okta SSO"

  acs: https://teleport-proxy.example.com:3080/v1/webapi/saml/acs
  attributes_to_roles:
    - {name: "groups", value: "okta-admin", roles: ["admin"]}
    - {name: "groups", value: "okta-dev", roles: ["dev"]}

     # note that wildcards can also be used. the next line instructs Teleport
     # to assign "admin" role to any user who has the SAML attribute that begins with "admin":
     - { name: "group", value: "admin*", roles: ["admin"] }
     # regular expressions with capture are also supported. the next line instructs Teleport
     # to assign users to roles `admin-1` if his SAML "group" attribute equals 'ssh_admin_1':
     - { name: "group", value: "^ssh_admin_(.*)$", roles: ["admin-$1"] }

  entity_descriptor: |
    <paste SAML XML contents here>
```

* See [examples/resources](https://github.com/gravitational/teleport/tree/master/examples/resources)
  directory in the Teleport Github repository for examples of possible connectors.

### User Logins

Often it is required to restrict SSO users to their unique UNIX logins when they
connect to Teleport nodes. To support this:

* Use the SSO provider to create a field called _"unix_login"_ (you can use another name).
* Make sure it's exposed as a claim via SAML/OIDC.
* Update a Teleport SSH role to include `{{external.unix_login}}` variable into the list of allowed logins:

```yaml
kind: role
version: v3
metadata:
  name: sso_user
spec:
  allow:
    logins:
    - '{{external.unix_login}}'
    node_labels:
      '*': '*'
```

## Multiple SSO Providers

Teleport can also support multiple connectors, i.e. a Teleport administrator
can define and create multiple connector resources using `tctl create` as shown above.

To see all configured connectors, execute this on the auth server:

```bash
$ tctl get connectors
```

To delete/update connectors, use the usual `tctl rm` and `tctl create` commands
as described in the [Resources section](admin-guide/#resources) in the Admin Manual.

If multiple authentication connectors exist, the clients must supply a
connector name to `tsh login` via `--auth` argument:

```bsh
# use "okta" SAML connector:
$ tsh --proxy=proxy.example.com login --auth=okta

# use local Teleport user DB:
$ tsh --proxy=proxy.example.com login --auth=local --user=admin
```


Refer to the following guides to configure authentication connectors of both
SAML and OIDC types:

* [SSH Authentication with Okta](ssh_okta)
* [SSH Authentication with OneLogin](ssh_one_login)
* [SSH Authentication with ADFS](ssh_adfs)
* [SSH Authentication with OAuth2 / OpenID Connect](oidc)

## Troubleshooting

Troubleshooting SSO configuration can be challenging. Usually a Teleport administrator
must be able to:

* Ensure that HTTP/TLS certificates are configured properly for both Teleport
  proxy and the SSO provider.
* Be able to see what SAML/OIDC claims and values are getting exported and passed
  by the SSO provider to Teleport.
* Be able to see how Teleport maps the received claims to role mappings as defined
  in the connector.

If something is not working, we recommend to:

* Double-check the host names, tokens and TCP ports in a connector definition.
* Look into Teleport's audit log for claim mapping problems. It is usually stored on the
  auth server in the `/var/lib/teleport/log` directory.


