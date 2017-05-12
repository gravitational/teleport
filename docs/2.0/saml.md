# Security Assertion Markup Language 2.0 (SAML 2.0)

Enterprise supports SAML 2.0 as an external identity provider and has been
tested to work with Okta and Active Directory Federation Services (ADFS)
2016.

## Okta Integration

## ADFS Integration

### ADFS Configuration

You'll need to configure ADFS to export claims about a user (Claims Provider
Trust in ADFS terminology) and you'll need to configure AD FS to trust
Teleport (a Relying Party Trust in ADFS terminology).

For Claims Provider Trust configuration you'll need to specify at least the
following two incoming claims: `Name ID` and `Group`. `Name ID` should contain
the email address that this user is associated with and `Group` should contain
the group the user belongs to. Group membership will be used to map users to
roles (for example to separate normal user and admins).

![Name ID Configuration](https://github.com/gravitational/teleport/tree/master/docs/2.0/img/adfs-1.png)
![Group Configuration](https://github.com/gravitational/teleport/tree/master/docs/2.0/img/adfs-1.png)

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
for it. Like before make sure you send `Name ID` and `Group` claims to the
relying party (Teleport).

### Teleport Configuration

To configure Teleport, you'll need to create a SAML resource. To do this create
a file below and insert it with the command `tctl create -f {file name}`.

```yaml
kind: saml
version: v2
metadata:
  name: "ADFS"
  namespace: "default"
spec:
  provider: "adfs"
  acs: "https://localhost:3080/v1/webapi/saml/acs"
  entity_descriptor_url: "https://adfs.example.com/FederationMetadata/2007-06/FederationMetadata.xml"
  attributes_to_roles:
    - name: "http://schemas.xmlsoap.org/claims/Group"
      value: "teleadmins"
      roles: ["admin"]
    - name: "http://schemas.xmlsoap.org/claims/Group"
      value: "teleusers"
      roles: ["users"]
```

The `acs` field should match the value you set in ADFS earlier and you can
obtain the `entity_descriptor_url` from ADFS under
`AD FS -> Service -> Endpoints -> Metadata`.

The `attributes_to_roles` is used to map attributes to Teleport roles. In our
situation, we are mapping the `Group` attribute whose full name is
`http://schemas.xmlsoap.org/claims/Group` with a value of `teleadmins` to the
`admin` role. Groups with the value `teleusers` is being mapped to the users
role. 

If you have individual system logins using pre-defined roles can be cumbersome
because you need to create a new role every time you add a new member to your
team. In this situation you can use role templates to dynamically create roles
based off information passed in the assertions. In the configuration below, if
the associated have a group with value `teleadmin` we dynamically create a
role with the name extracted from the value of the Name ID and UPN.

```yaml
kind: saml
version: v2
metadata:
  name: "ADFS"
  namespace: "default"
spec:
  provider: "adfs"
  acs: "https://localhost:3080/v1/webapi/saml/acs"
  entity_descriptor_url: "https://adfs.example.com/FederationMetadata/2007-06/FederationMetadata.xml"
  attributes_to_roles:
     - claim: "group"
       value: "admin"
       role_template:
          kind: role
          version: v2
          metadata:
             name: '{{index . "http://schemas.xmlsoap.org/claims/nameidentifier"}}'
             namespace: "default"
          spec:
             namespaces: [ "*" ]
             max_session_ttl: 90h0m0s
             logins: [ '{{index . "http://schemas.xmlsoap.org/claims/upn"}}', root ]
             node_labels:
                "*": "*"
             resources:
                "*": [ "read", "write" ]
```

### Exporting Signing Key

Next you'll need to export the signing key, you can do this with
`tctl saml export --name adfs`. Save the output to a file named `saml.crt`.
Return back to AD FS and open the Relying Party Trust and add this file as one
of the signature verification certificates.

### Login

For the Web UI, if the above configuration were real, you would see a button
that says `Login with Example`. Simply click on that and you will be
re-directed to a login page for your identity provider and if successful,
redirected back to Teleport.

For console login, you simple type `tsh --proxy <proxy-addr> ssh <server-addr>`
and a browser window should automatically open taking you to the login page for
your identity provider. `tsh` will also output a link the login page of the
identity provider if you are not automatically redirected.
