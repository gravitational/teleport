# Teleport Enterprise

This section will give an overview of Teleport Enterprise, the commercial product built around
the open source Teleport Community core. For those that want to jump right in, you can play
with the [Quick Start Guide for Teleport Enterprise](quickstart-enterprise.md).

The table below gives a quick overview of the benefits of Teleport Enterprise.

|Teleport Enterprise Feature|Description
---------|--------------
|[Role Based Access Control (RBAC)](#rbac)|Allows Teleport administrators to define User Roles and restrict each role to specific actions. RBAC also allows administrators to partition cluster nodes into groups with different access permissions.
|[Single Sign-On (SSO)](#sso)| Allows Teleport to integrate with existing enterprise identity systems. Examples include Active Directory, Github, Google Apps and numerous identity middleware solutions like Auth0, Okta, and so on. Teleport supports SAML and OAuth/OpenID Connect protocols to interact with them.
|[Approval Plugins](workflow/index.md) | Plugins to approve of Deny escalated RBAC requests.
|[FedRAMP/FIPS](#fedrampfips) | With Teleport 4.0, we have built out the foundation to help Teleport Enterprise customers build and meet the requirements in a FedRAMP System Security Plan (SSP). This includes a FIPS 140-2 friendly build of Teleport Enterprise as well as a variety of improvements to aid in complying with security controls even in FedRAMP High environments.
|Commercial Support | In addition to these features, Teleport Enterprise also comes with a premium support SLA with guaranteed response times.

!!! tip "Contact Information"

    If you are interested in Teleport Enterprise, please reach out to
    `sales@gravitational.com` for more information.

## RBAC

Role Based Access Control ("RBAC") allows Teleport administrators to grant granular access permissions to users. An example of an RBAC policy might be:  _"admins can do anything,
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

With Teleport 4.2 we've introduced the ability for users to request additional roles. The workflow API makes it easy to dynamically approve or deny these requests.

See [Approval Workflows Guide for more information](workflow/index.md)