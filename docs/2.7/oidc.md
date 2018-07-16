# OAuth2 / OpenID Connect (OIDC) Authentication for SSH

This guide will cover how to configure an SSO provider using [OpenID Connect](http://openid.net/connect/) 
(also known as OIDC) to issue SSH credentials to a specific groups of users.
When used in combination with role based access control (RBAC) it allows SSH
administrators to define policies like:

* Only members of "DBA" group can SSH into machines running PostgreSQL.
* Developers must never SSH into production servers.
* ... and many others.

!!! warning "Version Warning":
    This guide requires a commercial edition of Teleport. The open source
    edition of Teleport only supports [Github](admin-guide/#github-oauth-20) as
    an SSO provider.

## Enable OIDC Authentication

First, configure Teleport auth server to use OIDC authentication instead of the local
user database. Update `/etc/teleport.yaml` as show below and restart the
teleport daemon.

```bash
auth_service:
    authentication:
        type: oidc
```

## Identity Providers

Register Teleport with the external identity provider you will be using and
obtain your `client_id` and `client_secret`. This information should be
documented on the identity providers website. Here are a few links:

   * [Auth0 Client Configuration](https://auth0.com/docs/clients)
   * [Google Identity Platform](https://developers.google.com/identity/protocols/OpenIDConnect)
   * [Keycloak Client Registration](http://www.keycloak.org/docs/2.0/securing_apps_guide/topics/client-registration.html)

Add your OIDC connector information to `teleport.yaml`. A few examples are
provided below.

## OIDC Redirect URL

OIDC relies on HTTP re-directs to return control back to Teleport after
authentication is complete. The redirect URL must be selected by a Teleport
administrator in advance.

If the Teleport web proxy is running on `proxy.example.com` host, the redirect URL 
should be `https://proxy.example.com:3080/v1/webapi/oidc/callback`

## OIDC connector configuration

The next step is to add an OIDC connector to Teleport. The connectors are manipulated
via `tctl` [resource commands](admin-guide#resources). To create a new connector,
create a connector resource file in YAML format, for example `oidc-connector.yaml`.

The file contents are shown below. This connector requests the scope `group`
from the identity provider then mapping the value to either to `admin` role or
the `user` role depending on the value returned for `group` within the claims.

```bash
# oidc-connector.yaml
kind: oidc
version: v2
metadata:
  name: "example-oidc-connector"
spec:
  # display allows to set the caption of the "login" button
  # in the Web interface
  display: "Login with Example"
  issuer_url: "https://oidc.example.com"
  client_id: "xxxxxxxx.example.com"
  client_secret: "zzzzzzzzzzzzzzzzzzzzzzzz"
  redirect_url: "https://teleport-proxy.example.com:3080/v1/webapi/oidc/callback"
  # scope instructs Teleport to query for 'group' scope to retreive
  # user's group membership
  scope: ["group"]
  # once Teleport retreives the user's groups, this section configures
  # the mapping from groups to Teleport roles
  claims_to_roles:
     - claim: "group"
       value: "admin"
       roles: ["admin"]
     - claim: "group"
       value: "user"
       roles: ["user"]
```

Create the connector:

```bash
$ tctl create oidc-connector.yaml
```

## Create Teleport Roles

The next step is to define Teleport roles. They are created using the same 
`tctl` [resource commands](admin-guide#resources) as we used for the auth
connector.

Below are two example roles that are mentioned above, the first is an admin
with full access to the system while the second is a developer with limited
access.

```bash
# role-admin.yaml
kind: "role"
version: "v3"
metadata:
  name: "admin"
spec:
  options:
    max_session_ttl: "90h0m0s"
  allow:
    logins: [root]
    node_labels:
      "*": "*"
    rules:
      - resources: ["*"]
        verbs: ["*"]
```

Users are only allowed to login to nodes labelled with `access: relaxed`
teleport label. Developers can log in as either `ubuntu` to a username that
arrives in their assertions. Developers also do not have any rules needed to
obtain admin access.

```bash
# role-dev.yaml
kind: "role"
version: "v3"
metadata:
  name: "dev"
spec:
  options:
    max_session_ttl: "90h0m0s"
  allow:
    logins: [ "{{external.username}}", ubuntu ]
    node_labels:
      access: relaxed
```

Create both roles:

```bash
$ tctl create role-admin.yaml
$ tctl create role-dev.yaml
```

### Optional: ACR Values

Teleport supports sending Authentication Context Class Reference (ACR) values
when obtaining an authorization code from an OIDC provider. By default ACR
values are not set. However, if the `acr_values` field is set, Teleport expects
to receive the same value in the `acr` claim, otherwise it will consider the
callback invalid.

In addition, Teleport supports OIDC provider specific ACR value processing
which can be enabled by setting the `provider` field in OIDC configuration. At
the moment, the only build-in support is for NetIQ.

A example of using ACR values and provider specific processing is below:

```bash
# example connector which uses ACR values
kind: oidc
version: v2
metadata:
  name: "oidc-connector"
spec:
  issuer_url: "https://oidc.example.com"
  client_id: "xxxxxxxxxxxxxxxxxxxxxxx.example.com"
  client_secret: "zzzzzzzzzzzzzzzzzzzzzzzz"
  redirect_url: "https://localhost:3080/v1/webapi/oidc/callback"
  display: "Login with Example"
  acr_values: "foo/bar"
  provider: netiq
  scope: [ "group" ]
  claims_to_roles:
     - claim: "group"
       value: "admin"
       roles: [ "admin" ]
     - claim: "group"
       value: "user"
       roles: [ "user" ]
```

## Testing

For the Web UI, if the above configuration were real, you would see a button
that says `Login with Example`. Simply click on that and you will be
re-directed to a login page for your identity provider and if successful,
redirected back to Teleport.

For console login, you simple type `tsh --proxy <proxy-addr> ssh <server-addr>`
and a browser window should automatically open taking you to the login page for
your identity provider. `tsh` will also output a link the login page of the
identity provider if you are not automatically redirected.

## Troubleshooting

If you get "access denied errors" the number one place to check is the audit
log on the Teleport auth server. It is located in `/var/lib/teleport/log` by
default and it will contain the detailed reason why a user's login was denied.

Some errors (like filesystem permissions or misconfigured network) can be
diagnosed using Teleport's `stderr` log, which is usually available via:

```bash
$ sudo journalctl -fu teleport
```

If you wish to increase the verbocity of Teleport's syslog, you can pass
`--debug` flag to `teleport start` command.

