# Teleport Enterprise

This section will give an overview of Teleport Enterprise, the commercial product built around
the open source Teleport Community core. For those that want to jump right in, you can play
with the [Quick Start Guide for Teleport Enterprise](quickstart-enterprise.md).

The table below gives a quick overview of the benefits of Teleport Enterprise.

|Teleport Enterprise Feature|Description
---------|--------------
|[Role Based Access Control (RBAC)](#rbac)|Allows Teleport administrators to define User Roles and restrict each role to specific actions. RBAC also allows administrators to partition cluster nodes into groups with different access permissions.
|[Single Sign-On (SSO)](#sso)| Allows Teleport to integrate with existing enterprise identity systems. Examples include Active Directory, Github, Google Apps and numerous identity middleware solutions like Auth0, Okta, and so on. Teleport supports SAML and OAuth/OpenID Connect protocols to interact with them.
|[FedRAMP/FIPS](#fedrampfips) | With Teleport 4.0, we have built out the foundation to help Teleport Enterprise customers build and meet the requirements in a FedRAMP System Security Plan (SSP). This includes a FIPS 140-2 friendly build of Teleport Enterprise as well as a variety of improvements to aid in complying with security controls even in FedRAMP High environments.
|Commercial Support | In addition to these features, Teleport Enterprise also comes with a premium support SLA with guaranteed response times.

!!! tip "Contact Information"

    If you are interested in Teleport Enterprise, please reach out to
    `sales@gravitational.com` for more information.

## RBAC

Role Based Access Control ("RBAC") allows Teleport administrators to grant granular access permissions to users. An example of an RBAC policy might be:  "admins can do anything,
developers must never touch production servers, and interns can only SSH into
staging servers as guests".

### How does it work?

Every user in Teleport is **always** assigned a set of roles. The open source
edition of Teleport automatically assigns every user to the built-in "admin"
role, but Teleport Enterprise allows administrators to define their own
roles with far greater control over user permissions.

Let's assume a company is using Active Directory to authenticate users and place
them into groups. A typical enterprise deployment of Teleport in this scenario
would look like this:

1. Teleport will be configured to use existing user identities stored in Active
   Directory.
2. Active Directory would have users placed in certain groups or claims, perhaps "interns",
   "developers", "admins", "contractors", etc.
3. The Teleport administrator will have to define Teleport Roles. For
   example: "users", "developers" and "admins".
4. The last step will be to define mappings from the Active Directory groups (claims)
   to the Teleport Roles so every Teleport user will be assigned a role based
   on the group membership.

See [RBAC for SSH](ssh_rbac.md) chapter to learn more about configuring RBAC with
Teleport.

## SSO

The commercial edition of Teleport allows users to retrieve their SSH
credentials via a [single sign-on](https://en.wikipedia.org/wiki/Single_sign-on)
(SSO) system used by the rest of the organization.

Examples of supported SSO systems include commercial solutions like [Okta](https://www.okta.com),
[Auth0](https://auth0.com/), [SailPoint](https://www.sailpoint.com/),
[OneLogin](https://www.onelogin.com/) or [Active Directory](https://en.wikipedia.org/wiki/Active_Directory_Federation_Services), as
well as open source products like [Keycloak](http://www.keycloak.org).
Other identity management systems are supported as long as they provide an
SSO mechanism based on either [SAML](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language)
or [OAuth2/OpenID Connect](https://en.wikipedia.org/wiki/OpenID_Connect).


### How does SSO work with SSH?

From the user's perspective they need to execute the following command to retrieve their SSH certificate.

```bash
$ tsh login
```

Teleport can be configured with a certificate TTL to determine how often a user needs to log in.

`tsh login` will print a URL into the console, which will open an SSO login
prompt, along with the 2FA, as enforced by the SSO provider. If a user supplies
valid credentials, Teleport will issue an SSH certificate.

Moreover, SSO can be used in combination with role-based access control (RBAC)
to enforce SSH access policies like _"developers must not touch production data"_.
See the [SSO for SSH](ssh_sso.md) chapter for more details.


!!! tip "Contact Information"

    For more information about Teleport Enterprise or Gravity please reach out us to `sales@gravitational.com` or fill out the contact form on our [website](http://gravitational.com/demo).


## FedRAMP/FIPS

With Teleport 4.0 we have built the foundation to meet FedRAMP requirements for
the purposes of accessing infrastructure. This includes support for [FIPS 140-2](https://en.wikipedia.org/wiki/FIPS_140-2),
also known as the Federal Information Processing Standard, which is the US
government approved standard for cryptographic modules.

Enterprise customers can download the custom FIPS package from the [Gravitational Dashboard](https://dashboard.gravitational.com/web/).
Look for `Linux 64-bit (FedRAMP/FIPS)`.

Using `teleport start --fips` Teleport will start in FIPS mode, Teleport will
configure the TLS and SSH servers with FIPS compliant cryptographic algorithms.
In FIPS mode, if non-compliant algorithms are chosen, Teleport will fail to start.
In addition, Teleport checks if the binary was compiled against an approved
cryptographic module (BoringCrypto) and fails to start if it was not.

See our [Enterprise Guide for more information](ssh_fips.md)

## Approval Workflows

!!! warning "Warning: Workflows are currently in Alpha"

    This feature is currently in alpha.
    If you have a question please post to our [community](https://community.gravitational.com/), or file bugs on [Github](https://github.com/gravitational/teleport/issues/new).

With Teleport 4.2 we've introduced the ability for users to request additional roles. The workflow API makes it easy to dynamically approve or deny these requests.

## Setup

**Contractor Role**
This role lets the contractor request the role DBA.

```yaml
kind: role
metadata:
  name: contractor
spec:
  options:
    # ...
  allow:
    request:
      roles: ['dba']
    # ...
  deny:
    # ...
```

**DBA Role**
This role limits the contractor to request the role DBA for 1hr.

```yaml
kind: role
metadata:
  name: dba
spec:
  options:
    # ...
    # Only allows the contractor to use this role for 1hr from time of request.
    options.max_session_ttl: 1hr
  allow:
    # ...
  deny:
    # ...
```

**Admin Role**
This role lets the admin approve the contractor's request.
```yaml
kind: role
metadata:
  name: admin
spec:
  options:
    # ...
  allow:
    # ...
  deny:
    # ...
# list of allow-rules, see
# https://gravitational.com/teleport/docs/enterprise/ssh_rbac/
rules:
    # Access Request is part of Approval Workflows introduced in 4.2
    # `access_request` should only be given to Teleport Admins.
    - resources: [access_request]
      verbs: [list, read, update, delete]
```


```bash
$ tsh login teleport-cluster --request-roles=dba
Seeking request approval... (id: bc8ca931-fec9-4b15-9a6f-20c13c5641a9)
```

As a Teleport Administrator:


```bash
$ tctl request ls
Token                                Requestor Metadata       Created At (UTC)    Status
------------------------------------ --------- -------------- ------------------- -------
bc8ca931-fec9-4b15-9a6f-20c13c5641a9 alice     roles=dba      07 Nov 19 19:38 UTC PENDING
```

```bash
$ tctl request approve bc8ca931-fec9-4b15-9a6f-20c13c5641a9
```

Assuming approval, `tsh` will automatically manage a certificate re-issued with the newly requested roles applied. In this case `contractor` will now have have the permission of the
`dba`.

!!! warning

    Granting a role with administrative abilities could allow a user to **permanently** upgrade their privileges (e.g. if contractor was granted admin for some reason). We recommend only escalating to the next role of least privilege vs jumping directly to "Super Admin" role.

     The `deny.request` block can help mitigate the risk of doing this by accident.

### Other features of Approval Workflows.

 - Users can request multiple roles at one time. e.g `roles: ['dba','netsec','cluster-x']`
 - Approved requests have no affect on Teleport's behavior outside of allowing additional roles on re-issue. This has the nice effect of making requests "compatible" with older versions of Teleport, since only the issuing Auth Server needs any particular knowledge of the feature.
