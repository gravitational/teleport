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

#### OIDC with pre-defined roles

In the configuration below, we are requesting the scope `group` from the
identity provider then mapping the value to either to `admin` role or the `user`
role depending on the value returned for `group` within the claims.

```yaml
authentication:
   type: oidc
   oidc:
      id: example.com
      redirect_url: https://localhost:3080/v1/webapi/oidc/callback
      redirect_timeout: 90s
      client_id: 000000000000-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com
      client_secret: AAAAAAAAAAAAAAAAAAAAAAAA
      issuer_url: https://oidc.example.com
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

#### OIDC with role templates

If you have individual system logins using pre-defined roles can be cumbersome
because you need to create a new role every time you add a new member to your
team. In this situation you can use role templates to dynamically create roles
based off information passed in the claims. In the configuration below, if the
claims have a `group` with value `admin` we dynamically create a role with the
name extracted from the value of `email` in the claim and login `username`.

```yaml
authentication:
   type: oidc
   oidc:
      id: google
      redirect_url: https://localhost:3080/v1/webapi/oidc/callback
      redirect_timeout: 90s
      client_id: 000000000000-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com
      client_secret: AAAAAAAAAAAAAAAAAAAAAAAA
      issuer_url: https://oidc.example.com
      display: "Login with Example"
      scope: [ "group", "username", "email" ]
      claims_to_roles:
         - claim: "group"
           value: "admin"
           role_template:
              kind: role
              version: v2
              metadata:
                 name: '{{index . "email"}}'
                 namespace: "default"
              spec:
                 namespaces: [ "*" ]
                 max_session_ttl: 90h0m0s
                 logins: [ '{{index . "username"}}', root ]
                 node_labels:
                    "*": "*"
                 resources:
                    "*": [ "read", "write" ]
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
authentication:
   type: oidc
   oidc:
      id: example.com
      redirect_url: https://localhost:3080/v1/webapi/oidc/callback
      redirect_timeout: 90s
      client_id: 000000000000-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com
      client_secret: AAAAAAAAAAAAAAAAAAAAAAAA
      issuer_url: https://oidc.example.com
      acr_values: "foo/bar"
      provider: netiq
      display: "Login with Example"
      scope: [ "group" ]
      claims_to_roles:
         - claim: "group"
           value: "admin"
           roles: [ "admin" ]
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
