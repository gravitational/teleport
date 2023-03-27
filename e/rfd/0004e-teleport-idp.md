---
authors: Michael Wilson (mike@goteleport.com)
state: implemented (12.1)
---

# RFD 4E - Teleport as a SAML IdP

### Required Approvers

* Engineering: @r0mant
* Security: @reed
* Product: (@xinding33 || @klizhentas)

## What

Allow Teleport to be used as a SAML identity provider for any users known to Teleport,
including both Teleport local users and users known to Teleport through a connector like SAML,
OIDC, or Github.

Note: This is an enterprise only feature.

## Why

With Teleport functioning as a identity provider, Teleport will be able to authenticate
to external services, allowing Teleport to expand its assertion over connectivity and access
to SAML enabled web applications.

As an example, say you have an organizational Slack that you would like Teleport to provide
authentication and authorization for. You would be able configure Slack to point to Teleport's
SAML identity provider to provide access to Slack. Additionally, this will work regardless of
whether Teleport's users are sourced locally or from an external IdP. A user could login to
Teleport using Okta, and then login to Slack using the IdP from Teleport.

### Very basic SAML primer

These are my definitions and explanations, not necessarily an authoritative source. There are
likely more thorough explanations elsewhere.

#### Identity provider

The **identity provider** is the authoritative source for authentication assertions. It
functions as the source of truth for authentication of users to a system. Identity providers
expect **service providers** to consume the assertions issued by the identity provider.
Identity providers need to establish some sort of trust to service providers to avoid
man-in-the-middle attacks.

#### Service provider

A **service provider** consumes assertions from an identity provider. Service providers need
to be configured to point to the identity provider, which is often done by consuming the
metadata produced by the identity provider. The metadata produced by the identity provider
allows the service provider to verify the identity provider as well. The metadata for the IdP
is typically produced via a metadata endpoint.

Web applications will serve as service providers to Teleport as an identity provider.

#### Assertions

Assertions are made by an identity provider. These serve as authoritative statements that a
user has been successfully authenticated.

#### IdP-initiated SSO

IdP-initiated SSO means that the identity provider itself initiates the login sequence for
a service provider.

#### SP-initiated SSO

SP-initiated SSO means that the service provider initiates the login sequence.

## Details

### UX

When starting an enterprise Teleport instance, the SAML IdP will be enabled by default. To
disable, add a new section to the `proxy_service` configuration YAML, `idp`, and set the
key `enabled` under the `saml` key to `no`.

```yaml
proxy_service:
  enabled: "yes"
  ...
  # Disabling the IdP
  idp:
    saml:
      enabled: yes # defaults to on
  ...
```

For exposing these configuration options for cloud, we should adjust the
`ClusterAuthPreference` object to include a new `idp` section that will contain a
`saml_enabled` key with a boolean value. This will default to `true`:

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: auth_preference
spec:
  ...
  idp:
    saml: yes
```

#### Adding service providers

In order to establish service providers of the Teleport IdP, service provider XML will need to be
added to Teleport. To do this, a new `saml_service_provider` object will be created which can
be added to Teleport at runtime using `tctl`.

An example YAML:

```yaml
#
# Example resource for a SAML Service Provider
#
kind: saml_service_provider
version: v1
metadata:
  # the name of the service provider
  name: sp-example
spec:
  entity_descriptor: |
    <?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor entityID=...
```

It'll be up to the user to generate this XML, which can usually be generated via the target
application or service.

```bash
$ tctl create -f saml-service-provider.yaml
```

Users will also be able to update, list, and delete this providers as well:

```bash
# Update (if the service provider already exists)
$ tctl create -f saml-service-provider.yaml

# List
$ tctl get saml_service_provider

# Get
$ tctl get saml_service_provider/sp-example

# Delete
$ tctl rm saml_service_provider/sp-example
```

Additionally, the CLI should support the `saml_sp` shortcut for these objects.

Note that providing a way to configure the IdP and register service providers via the GUI is out
of initial scope but will be added in the future.

### High level architecture

Teleport will function as the identity provider, serving service providers.

#### Identity Provider

The identity provider will be based on
[crewjam's SAML implementation](https://github.com/crewjam/saml) (BSD-2 licensed), which is a
go native SAML IdP. This will use the list of users currently known to Teleport and only
authenticate these users. If the user has a valid session within Teleport, then the user will
be redirected to the target service provider automatically. Otherwise, they will be prompted to
login before being redirected.

This should be fine regardless of whether user management is being handled within Teleport or
using an SSO connector like Github, SAML, or OIDC.

The IdP will use a new private key and certificate for signing of SAML assertions. This will be
signed by a new CA specifically for the IdP.

Both IdP-initiated and SP-initiated authentication will be supported.

Several of crewjam's SAML IdP endpoints will be exposed under the `/enterprise/saml-idp` path,
but the two most important for service providers are:

* `/enterprise/saml-idp/metadata` will produce the IdP metadata.
* `/enterprise/saml-idp/sso` is where authorization requests are sent to.

More thorough documentation of SAML and its endpoints can be found
[here](http://docs.oasis-open.org/security/saml/v2.0/saml-bindings-2.0-os.pdf).

##### Using Teleport's `AuthenticateRequest` before allowing SAML IdP access

When attempting to access any SAML IdP endpoint, Teleport will attempt to authenticate the
request using the `AuthenticateRequest` funcion in `lib/web/apiserver.go`. This will push the
user through the regular Teleport authentication flow before allowing access. This works
seamlessly for both Teleport native user management and connector enabled authentication within
Teleport.

##### `AuthPreference` dynamic configuration

The service managing the SAML provider should monitor for changes in the `AuthPreference`
object (if it exists) and start or stop the identity provider as specified. Static configs will
taken precedence over settings in `AuthPreference`.

##### Authorization expiration

SAML authorization responses will have an expiration date set to the same time that the
certificates associated with the user's current session are set to expire. In other words, for
a current `sc := web.SessionContext`, the SAML response will be set to expire at
`sc.GetIdentity().Expires()`.

##### Assertion attributes

We will add the following attributes to assertions generated by the IdP:

| Attribute Name | Friendly name | OID Link | Description |
|----------------|---------------|----------|-------------|
| `urn:oid:0.9.2342.19200300.100.1.1` | `uid` | [Link](http://oid-info.com/cgi-bin/display?oid=0.9.2342.19200300.100.1.1&action=display) | The username from Teleport |
| `urn:oid:1.3.6.1.4.1.5923.1.1.1.1` | `eduPersonAffiliation` | [Link](http://oid-info.com/cgi-bin/display?oid=1.3.6.1.4.1.5923.1.1.1.1&a=display) | String array of Teleport roles |

This is a subset of what crewjam offers. Some of crewjam's SAML attributes are mislabeled, so
I suggest we use our own, especially with a light towards later customization if so desired.

##### Certificate Authority behavior and rotation

The certificate authority used for the IdP will be used by the IdP to sign communications from
the IdP. When retrieving the metadata from the IdP, the CA's public certificate will be
used.

A new type of CA `saml-idp` will be rotatable along with the existing types. When the CA is
rotated, it will be necessary to update service providers with the new metadata produced by
the IdP. The recommended rotation period for this CA is 5 years, modeled after Google IdP's
rotation [process](https://support.google.com/a/answer/7394709).

##### Group based IdP access

Administrators will be able to enable or disable a user's ability to authenticate/authorize
against the IdP using a new role option `idp`. This new key will have a `saml` subkey
that will have a `yes/no` value to determine access. This will allow any user belonging to the
role to authenticate/authorize against the SAML IdP:

```yaml
kind: role
metadata:
  name: idp-role
spec:
  ...
  options:
    idp:
      saml: yes
  ... 
version: v3
```

**Note**: users will default to having IdP access enabled unless administrators specify
otherwise.

#### Service providers

With respect to authorization requests received from service providers, the
[SAML v2.0 profiles errata](https://www.oasis-open.org/committees/download.php/56782/sstc-saml-profiles-errata-2.0-wd-07.pdf)
states that:

>  Whether the request is signed or not, the identity provider MUST ensure that any
> `<AssertionConsumerServiceURL>` or `<AssertionConsumerServiceIndex>` elements in the request
> are verified as belonging to the service provider to whom the response will be sent. Failure
> to do so can result in a man-in-the-middle attack.

In order to accomplish this, crewjam's SAML IdP implementation requires that service providers
be registered before authorization using that service provider is allowed. Teleport will expose
that same mechanism.

### Audit events

A number of new audit events will be created as part of this effort:

| Event Name | Description |
|------------|-------------|
| `saml.idp.auth.success` | Emitted when a user has successfully authorized against the SAML IdP |
| `saml.idp.auth.failure` | Emitted when a user has unsuccessfully authorized against the SAML IdP |
| `saml.idp.service.provider.added` | Emitted when a service provider has been added |
| `saml.idp.service.provider.updated` | Emitted when a service provider has been updated |
| `saml.idp.service.provider.deleted` | Emitted when a service provider has been deleted |

Reasons for failures should be added into the metadata of any failure cases.

### Security

* Registering service providers with the SAML IdP will prevent potential man-in-the-middle
  attacks by defining expected key signatures from service provider communication.
* Teleport's built in authentication mechanism will be used to ensure users have valid,
  active sessions in the Teleport UI before allowing SAML IdP access.
* Teleport should be unable to reference its own IdP for for access. This will reduce the
  likelihood of privilege escalation or unauthorized access to Teleport.
* Service providers be required to use https endpoints and not just http endpoints.

### Implementation plan

#### `SAMLIdPServiceProvider` object and APIs

The `SAMLIdPServiceProvider` object will be implemented along with necessary APIs.

#### `SAMLIdPServiceProvider` CLI modifications

Any CLI modifications needed to manage service providers will be implemented.

#### Forwarding/UI modifications

The `webapps` repository will need to be modified to allow `/enterprise/saml-idp/sso` as a valid
forwarding location.

#### SAML IdP CA implementation

The new SAML IdP CA implementation will be added along with the ability to rotate it. This will
require a stub for updating the SAML IdP to update its certificate live.

#### Implementation of the IdP

The identity provider must be established and exposed within the enterprise client.

#### Implementation of an IdP service provider test

We should implement a tester that goes into `examples` that acts as a service provider and
verifies the SAML input/output.
