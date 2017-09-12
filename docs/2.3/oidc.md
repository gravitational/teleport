# OpenID Connect (OIDC)

Teleport supports [OpenID Connect](http://openid.net/connect/) (also known as
`OIDC`) to provide external authentication using commercial OpenID providers
like [Auth0](https://auth0.com) as well as open source identity managers like
[Keycloak](http://www.keycloak.org).

## Configuration

OIDC relies on re-directs to return control back to Teleport after
authentication is complete. Decide on the redirect URL you will be using and
know it in advance before you register Teleport with an external identity
provider.

### Development mode

For development purposes we recommend the following `redirect_url`:
`https://localhost:3080/v1/webapi/oidc/callback`.

### Identity Providers

Register Teleport with the external identity provider you will be using and
obtain your `client_id` and `client_secret`. This information should be
documented on the identity providers website. Here are a few links:

   * [Auth0 Client Configuration](https://auth0.com/docs/clients)
   * [Google Identity Platform](https://developers.google.com/identity/protocols/OpenIDConnect)
   * [Keycloak Client Registration](http://www.keycloak.org/docs/2.0/securing_apps_guide/topics/client-registration.html)

Add your OIDC connector information to `teleport.yaml`. A few examples are
provided below.

### Enable OIDC Authentication

First, configure Teleport auth server to use OID authentication instead of the local
user database. Update `/etc/teleport.yaml` as show below and restart the
teleport daemon.

```yaml
auth_service:
    # Turns 'auth' role on. Default is 'yes'
    enabled: yes

    # defines the types and second factors the auth server supports
    authentication:
        type: oidc
```

#### OIDC connector configuration

In the configuration below, we are requesting the scope `group` from the
identity provider then mapping the value to either to `admin` role or the `user`
role depending on the value returned for `group` within the claims.

```yaml
kind: oidc
version: v2
metadata:
  name: "google"
spec:
  issuer_url: "https://oidc.example.com"
  client_id: "000000000000-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com"
  client_secret: "AAAAAAAAAAAAAAAAAAAAAAAA"
  redirect_url: "https://localhost:3080/v1/webapi/oidc/callback"
  display: "Login with Example"
  scope: [ "group" ]
  claims_to_roles:
     - claim: "group"
       value: "admin"
       roles: [ "admin" ]
     - claim: "group"
       value: "user"
       roles: [ "user" ]
```

Below are two example roles that are mentioned above, the first is an admin
with full access to the system while the second is a developer with limited
access.

```yaml
kind: "role"
version: "v3"
metadata:
  name: "admin"
spec:
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
kind: "role"
version: "v3"
metadata:
  name: "dev"
spec:
  max_session_ttl: "90h0m0s"
  allow:
    logins: [ "{{external.username}}", ubuntu ]
    node_labels:
      access: relaxed
```

#### ACR Values

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
kind: oidc
version: v2
metadata:
  name: "google"
spec:
  issuer_url: "https://oidc.example.com"
  client_id: "000000000000-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com"
  client_secret: "AAAAAAAAAAAAAAAAAAAAAAAA"
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

#### Login

For the Web UI, if the above configuration were real, you would see a button
that says `Login with Example`. Simply click on that and you will be
re-directed to a login page for your identity provider and if successful,
redirected back to Teleport.

For console login, you simple type `tsh --proxy <proxy-addr> ssh <server-addr>`
and a browser window should automatically open taking you to the login page for
your identity provider. `tsh` will also output a link the login page of the
identity provider if you are not automatically redirected.
