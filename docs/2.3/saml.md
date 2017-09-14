# SAML 2.0 Authentication

Teleport Enterprise supports SAML 2.0 as an external identity provider and has been
tested to work with [Okta](https://www.okta.com/) and 
[Active Directory Federation Services](https://en.wikipedia.org/wiki/Active_Directory_Federation_Services) (ADFS) 2016.
Other identity providers, like [Auth0](https://auth0.com/), are known to work as well.


## Okta

This guide will cover how to configure Teleport to authenticate users via SAML
using [Okta](https://www.okta.com/) as a SAML provider.

### Enable SAML Authentication

First, configure Teleport auth server to use SAML authentication instead of the local
user database. Update `/etc/teleport.yaml` as show below and restart the
teleport daemon.

```bash
...
auth_service:
    # Turns 'auth' role on. Default is 'yes'
    enabled: yes

    # defines the types and second factors the auth server supports
    authentication:
        type: saml
...
```

### Confiugre Okta

First, create a SAML 2.0 Web App in Okta configuration section

![Create APP](img/okta-saml-1.png?raw=true)
![Create APP name](img/okta-saml-2.png?raw=true)

**Create Groups**

We are going to create two groups: "okta-dev" and "okta-admin":

![Create Group Devs](img/okta-saml-2.1.png)

...and the admin:

![Create Group Devs](img/okta-saml-2.2.png)

**Configure the App**

We are going to map the Okta groups we've created above to the SAML Attribute
statements (special signed metadata exposed via a SAML XML response).

![Configure APP](img/okta-saml-3.png)

!!! tip "Important":
    Notice that we have set "NameID" to the email format and mappped the groups with 
    a wildcard regex in the Group Attribute statements. We have also set the "Audience" 
    and SSO URL to the same value.

**Assign Groups**

Assign groups and people to your SAML app:

![Configure APP](img/okta-saml-3.1.png)

Make sure to download the metadata in the form of an XML document. It will be used it to 
configure a Teleport connector:

![Download metadata](img/okta-saml-4.png?raw=true)


### Create a Teleport SAML Connector

Now, create a SAML connector [resource](admin-guide#resources):

```bash
# okta-connector.yaml
kind: saml
version: v2
metadata:
  name: OktaSAML
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

Create the connector using `tctl` tool:

```bash
$ tctl create okta-connector.yaml
```

**Create Teleport Roles**

We are going to create 2 roles, privileged role admin who is able to login as
root and is capable of administrating the cluster and non-privileged dev.

```yaml
kind: "role"
version: "v3"
metadata:
  name: "admin"
spec:
  max_session_ttl: "24h"
  allow:
    logins: [root]
    node_labels:
      "*": "*"
    rules:
      - resources: ["*"]
        verbs: ["*"]
```

Devs are only allowed to login to nodes labelled with `access: relaxed`
teleport label. Developers can log in as either `ubuntu` to a username that
arrives in their assertions. Developers also do not have any rules needed to
obtain admin access.

```yaml
kind: "role"
version: "v3"
metadata:
  name: "dev"
spec:
  max_session_ttl: "24h"
  allow:
    logins: [ "{{external.username}}", ubuntu ]
    node_labels:
      access: relaxed
```
    
**Notice:** Replace `ubuntu` with linux login available on your servers!

```bash
$ tctl create admin.yaml
$ tctl create dev.yaml
```

### Logging In

The Web UI will now contain a new button: "Login with Okta". The CLI is 
the same as before:

```bash
$ tsh --proxy=proxy.example.com login
```

This command will print the SSO login URL (and will try to open it
automatically in a browser).

!!! tip "Tip":
    Teleport can use multiple SAML connectors. In this case a connector name
    can be passed via `tsh login --auth=connector_name`

!!! note "IMPORTANT":
    Teleport only supports sending party initiated flows for SAML 2.0. This
    means you can not initiate login from your identity provider, you have to
    initiate login from either the Teleport Web UI or CLI.

## ADFS

### ADFS Configuration

You'll need to configure ADFS to export claims about a user (Claims Provider
Trust in ADFS terminology) and you'll need to configure AD FS to trust
Teleport (a Relying Party Trust in ADFS terminology).

For Claims Provider Trust configuration you'll need to specify at least the
following two incoming claims: `Name ID` and `Group`. `Name ID` should be a
mapping of the LDAP Attribute `E-Mail-Addresses` to `Name ID`. A group
membership claim should be used to map users to roles (for example to
separate normal users and admins).

![Name ID Configuration](img/adfs-1.png?raw=true)
![Group Configuration](img/adfs-2.png?raw=true)

In addition if you are using dynamic roles (see below), it may be useful to map
the LDAP Attribute `SAM-Account-Name` to `Windows account name` and create
another mapping of `E-Mail-Addresses` to `UPN`.

![WAN Configuration](img/adfs-3.png?raw=true)
![UPN Configuration](img/adfs-4.png?raw=true)

You'll also need to create a Relying Party Trust, use the below information to
help guide you through the Wizard. Note, for development purposes we recommend
using `https://localhost:3080/v1/webapi/saml/acs` as the Assertion Consumer
Service (ACS) URL, but for production you'll want to change this to a domain
that can be accessed by other users as well.

* Create a claims aware trust.
* Enter data about the relying party manually.
* Set the display name to something along the lines of "Teleport".
* Skip the token encryption certificate.
* Select `Enable support for SAML 2.0 Web SSO protocol` and set the URL to `https://localhost:3080/v1/webapi/saml/acs`.
* Set the relying party trust identifier to `https://localhost:3080/v1/webapi/saml/acs` as well.
* For access control policy select `Permit everyone`.

Once the Relying Party Trust has been created, update the Claim Issuance Policy
for it. Like before make sure you send at least `Name ID` and `Group` claims to the
relying party (Teleport). If you are using dynamic roles, it may be useful to
map the LDAP Attribute `SAM-Account-Name` to `Windows account name` and create
another mapping of `E-Mail-Addresses` to `UPN`.

Lastly, ensure the user you create in Active Directory has an email address
associated with it. To check this open Server Manager then
`Tools -> Active Directory Users and Computers` and select the user and right
click and open properties. Make sure the email address field is filled out.

### Teleport Configuration

Lets create two Teleport roles: one for administrators and the other is for
normal users. You can create them using `tctl create {file name}` CLI command
or via the Web UI.

```yaml
# admin-role.yaml
kind: "role"
version: "v3"
metadata:
  name: "admin"
spec:
  max_session_ttl: "90h0m0s"
  allow:
    logins: [ root ]
    node_labels:
      "*": "*"
    rules:
      - resources: ["*"]
        verbs: ["*"]
```

```yaml
# user-role.yaml
kind: "role"
version: "v3"
metadata:
  name: "dev"
spec:
  # regular users can only be guests and their certificates will have a TTL of 1 hour:
  max_session_ttl: 1h
  allow:
    # only allow login as either ubuntu or the username claim
    logins: [ "{{external.username}}", ubuntu ]
```

Next create a SAML connector [resource](admin-guide#resources):

```bash
kind: saml
version: v2
metadata:
  name: "adfs"
spec:
  provider: "adfs"
  acs: "https://localhost:3080/v1/webapi/saml/acs"
  entity_descriptor_url: "https://adfs.example.com/FederationMetadata/2007-06/FederationMetadata.xml"
  attributes_to_roles:
    - name: "http://schemas.xmlsoap.org/claims/Group"
      value: "teleadmins"
      roles: ["admins"]
    - name: "http://schemas.xmlsoap.org/claims/Group"
      value: "teleusers"
      roles: ["users"]
```

The `acs` field should match the value you set in ADFS earlier and you can
obtain the `entity_descriptor_url` from ADFS under
`AD FS -> Service -> Endpoints -> Metadata`.

The `attributes_to_roles` is used to map attributes to the Teleport roles you
just creataed. In our situation, we are mapping the `Group` attribute whose full
name is `http://schemas.xmlsoap.org/claims/Group` with a value of `teleadmins`
to the `admin` role. Groups with the value `teleusers` is being mapped to the
`users` role.

**Exporting Signing Key**

For the last step, you'll need to export the signing key:

```bash
$ tctl saml export adfs
```

Save the output to a file named `saml.crt`, return back to ADFS, open the
"Relying Party Trust" and add this file as one of the signature verification
certificates.
