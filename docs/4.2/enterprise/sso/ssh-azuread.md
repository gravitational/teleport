# SSH Authentication with Azure Active Directory (AD)

This guide will cover how to configure [Microsoft Azure Active Directory](https://azure.microsoft.com/en-us/services/active-directory/) to issue
SSH credentials to specific groups of users with a SAML Authentication Connector. When used in combination with role
based access control (RBAC) it allows SSH administrators to define policies
like:

* Only members of "DBA" Azure AD group can SSH into machines running PostgreSQL.
* Developers must never SSH into production servers.
* ... and many others.

The following steps configure an example SAML authentication connector matching AzureAD groups with security roles.  You can choose to configure other options.

!!! warning "Version Warning"

    This guide requires an Enterprise version of Teleport. The open source
    edition of Teleport only supports [Github](../../admin-guide.md#github-oauth-20) as
    an SSO provider.

## Prerequisites:

Before you get started you’ll need:

- An Enterprise version of Teleport v4.2 or greater, downloaded from [https://dashboard.gravitational.com/](https://dashboard.gravitational.com/web/).
- An Azure AD admin account with access to creating non-gallery applications (P2 License)
- To register one or more users in the directory
- To create at least two security groups in AzureAD and assign one or more users to each group



## Configure Azure AD

1. Select Enterprise Applications from the AzureAD Directory Home
  ![Select Enterprise Applications From Manage](../../img/azuread/azuread-1-home.png)

2. Select New application
  ![Select New Applications From Manage](../../img/azuread/azuread-2-newapp.png)

3. Select a Non-gallery application
   ![Select Non-gallery application](../../img/azuread/azuread-3-selectnongalleryapp.png)

4. Enter the display name (Ex: Teleport)
   ![Enter application name](../../img/azuread/azuread-4-enterappname.png)

5.Select properties under Manage and turn off User assignment required
   ![Turn off user assignment](../../img/azuread/azuread-5-turnoffuserassign.png)

6. Select Single Sign-on under Manage and choose SAML
   ![Select SAML](../../img/azuread/azuread-6-selectsaml.png)

7. Select to edit Basic SAML Configuration
   ![Edit Basic SAML Configuration](../../img/azuread/azuread-7-editbasicsaml.png)

8. Put in the Entity ID and Reply URL the same proxy url https://teleport.example.com:3080/v1/webapi/saml/acs
   ![Put in Entity ID and Reply URL](../../img/azuread/azuread-8-entityandreplyurl.png)

9. Edit User Attributes & Claims

    i. Edit the Claim Name.  Change the name identifier format to Default. Make sure the source attribute is user.userprincipalname.
   ![Confirm Name Identifier](../../img/azuread/azuread-9a-nameidentifier.png)

    ii. Add a group Claim to have user security groups available to the connector
   ![Put in Security group claim](../../img/azuread/azuread-9b-groupclaim.png)

    iii. Add a Claim to pass the username from transforming the AzureAD User name.
   ![Add a transformed username](../../img/azuread/azuread-9c-usernameclaim.png)


10. On the SAML Signing Certificate select to download SAML Download the Federation Metadata XML.
   ![Download Federation Metadata XML](../../img/azuread/azuread-10-fedmeatadataxml.png)

!!! warning "Important"

    This is a important document.  Treat the Federation Metadata XML file as you would a password.

## Create a SAML Connector

Now, create a SAML connector [resource](../../admin-guide.md#resources).  Replace the acs element with your Teleport address, update the group IDs with the actual AzureAD group ID values, and insert the downloaded Federation Metadata XML into the entity_descriptor resource.
Write down this template as `azure-connector.yaml`:

```yaml
kind: saml
version: v2
metadata:
  # the name of the connector
  name: azure-saml
spec:
  display: "Microsoft"
  # acs is the Assertion Consumer Service URL. This should be the address of
  # the Teleport proxy that your identity provider will communicate with.
  acs: https://teleport.example.com:3080/v1/webapi/saml/acs
  attributes_to_roles:
    - {name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", value: "<group id 930210...>", roles: ["admin"]}
    - {name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", value: "<group id 93b110...>", roles: ["dev"]}
  entity_descriptor: |
    <federationmedata.xml contents>
```

Create the connector using `tctl` tool:

```bsh
$ tctl create azure-connector.yaml
```
!!! tip "FYI"

    Teleport will automatically transform the contents of the connector when viewed from the web UI.

 ![Sample Connector Transform](../../img/azuread/azuread-12-sampleconnector.png)

## Create Teleport Roles

We are going to create 2 roles:

-  Privileged role `admin` who is able to login as root and is capable of administrating
the cluster
- Non-privileged role `dev`

```yaml
kind: role
version: v3
metadata:
  name: admin
spec:
  options:
    max_session_ttl: 24h
  allow:
    logins: [root]
    node_labels:
      "*": "*"
    rules:
      - resources: ["*"]
        verbs: ["*"]
```

Devs are only allowed to login to nodes labeled with `access: relaxed`
Teleport label. Developers can log in as either `ubuntu` or a username that
arrives in their assertions. Developers also do not have any rules needed to
obtain admin access to Teleport.

```yaml
kind: role
version: v3
metadata:
  name: dev
spec:
  options:
    max_session_ttl: 24h
  allow:
    logins: [ "{% raw %}{{external.username}}{% endraw %}", ubuntu ]
    node_labels:
      access: relaxed
```

**Notice:** Replace `ubuntu` with linux login available on your servers!

```bsh
$ tctl create admin.yaml
$ tctl create dev.yaml
```

## Testing


Update the Teleport settings to use the SAML settings to make this the default.
```yaml
auth_service:
  authentication:
    type: saml
```
![Login with Microsoft](../../img/azuread/azure-11-loginwithmsft.png)

The Web UI will now contain a new button: "Login with Microsoft". The CLI is
the same as before:

```bsh
$ tsh --proxy=proxy.example.com login
```

This command will print the SSO login URL (and will try to open it
automatically in a browser).

!!! tip "Tip"

    Teleport can use multiple SAML connectors. In this case a connector name
    can be passed via `tsh login --auth=connector_name`


## Troubleshooting

If you get "access denied" errors the number one place to check is the audit
log on the Teleport auth server. It is located in `/var/lib/teleport/log` by
default and it will contain the detailed reason why a user's login was denied.

Example of a user being denied due as the role `clusteradmin` wasn't setup.
```json
{"code":"T1001W","error":"role clusteradmin is not found","event":"user.login","method":"saml","success":false,"time":"2019-06-15T19:38:07Z","uid":"cd9e45d0-b68c-43c3-87cf-73c4e0ec37e9"}
```


Some errors (like filesystem permissions or misconfigured network) can be
diagnosed using Teleport's `stderr` log, which is usually available via:

```bsh
$ sudo journalctl -fu teleport
```

If you wish to increase the verbosity of Teleport's syslog, you can pass the
[`--debug`](../../cli-docs.md#teleport-start) flag to `teleport start` command.
