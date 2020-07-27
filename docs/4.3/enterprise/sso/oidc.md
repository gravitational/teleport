# OAuth2 / OpenID Connect (OIDC) Authentication for SSH

This guide will cover how to configure an SSO provider using [OpenID Connect](http://openid.net/connect/)
(also known as OIDC) to issue SSH credentials to a specific groups of users.
When used in combination with role based access control (RBAC) it allows SSH
administrators to define policies like:

* Only members of "DBA" group can SSH into machines running PostgreSQL.
* Developers must never SSH into production servers.
* ... and many others.

!!! warning "Version Warning"

    This guide requires an Enterprise edition of Teleport. The Community
    edition of Teleport only supports [Github](../../admin-guide.md#github-oauth-20) as
    an SSO provider.

## Enable OIDC Authentication

First, configure Teleport auth server to use OIDC authentication instead of the local
user database. Update `/etc/teleport.yaml` as show below and restart the
teleport daemon.

```yaml
auth_service:
    authentication:
        type: oidc
```

## Identity Providers

Register Teleport with the external identity provider you will be using and
obtain your `client_id` and `client_secret`. This information should be
documented on the identity providers website. Here are a few links:

   * [Auth0 Client Configuration](https://auth0.com/docs/applications)
   * [Google Identity Platform](https://developers.google.com/identity/protocols/OpenIDConnect)
   * [Keycloak Client Registration](https://www.keycloak.org/docs/latest/securing_apps/index.html#_client_registration)

Add your OIDC connector information to `teleport.yaml`. A few examples are
provided below.

!!! tip
    For Google / G Suite please follow our dedicated [Guide](ssh_gsuite.md)

## OIDC Redirect URL

OIDC relies on HTTP re-directs to return control back to Teleport after
authentication is complete. The redirect URL must be selected by a Teleport
administrator in advance.

If the Teleport web proxy is running on `proxy.example.com` host, the redirect URL
should be `https://proxy.example.com:3080/v1/webapi/oidc/callback`

## OIDC connector configuration

The next step is to add an OIDC connector to Teleport. The connectors are manipulated
via `tctl` [resource commands](../../admin-guide.md#resources). To create a new connector,
create a connector resource file in YAML format, for example `oidc-connector.yaml`.

The file contents are shown below. This connector requests the scope `group`
from the identity provider then mapping the value to either to `admin` role or
the `user` role depending on the value returned for `group` within the claims.

```yaml
{!examples/resources/oidc-connector.yaml!}
```

Create the connector:

```bsh
$ tctl create oidc-connector.yaml
```

## Create Teleport Roles

The next step is to define Teleport roles. They are created using the same
`tctl` [resource commands](../../admin-guide.md#resources) as we used for the auth
connector.

Below are two example roles that are mentioned above, the first is an admin
with full access to the system while the second is a developer with limited
access.

```yaml
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

```yaml
# role-dev.yaml
kind: "role"
version: "v3"
metadata:
  name: "dev"
spec:
  options:
    max_session_ttl: "90h0m0s"
  allow:
    logins: [ "{% raw %}{{external.username}}{% endraw %}", ubuntu ]
    node_labels:
      access: relaxed
```

Create both roles:

```bsh
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

```yaml
# example connector which uses ACR values
kind: oidc
version: v2
metadata:
  name: "oidc-connector"
spec:
  issuer_url: "https://oidc.example.com"
  client_id: "xxxxxxxxxxxxxxxxxxxxxxx.example.com"
  client_secret: "zzzzzzzzzzzzzzzzzzzzzzzz"
  redirect_url: "https://<cluster-url>:3080/v1/webapi/oidc/callback"
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

### Optional: Redirect URL and Timeout

The redirect URL must be accessible by all user, optional redirect timeout.

```yaml
# Extra parts of OIDC yaml have been removed.
spec:
  redirect_url: https://<cluster-url>.example.com:3080/v1/webapi/oidc/callback
  # Optional Redirect Timeout.
  # redirect_timeout: 90s
```

### Optional: Prompt

By default, Teleport will prompt end users to select an account each time they log in
even if the user only has one account.

Teleport 4.2 now lets Teleport Admins configure this option. Since `prompt` is optional,
by setting the variable to an empty string Teleport will override the default `select_account`.

```yaml
kind: oidc
version: v2
metadata:
  name: connector
spec:
  prompt: ''
```

The below example will prompt the end-user for reauthentication and will require consent
from the client.

```yaml
kind: oidc
version: v2
metadata:
  name: connector
spec:
  prompt: 'login consent'
```

A list of available optional prompt parameters are available from the
[OpenID website](https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest).

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

```bsh
$ sudo journalctl -fu teleport
```

If you wish to increase the verbosity of Teleport's syslog, you can pass
`--debug` flag to `teleport start` command.
