# Teleport Enterprise Features

This chapter covers Teleport features that are only available in the commercial
edition of Teleport, called "Teleport Enterprise". The table below gives the quick 
overview of the Enterprise Edition features.

|Teleport Enterprise Feature|Description
---------|--------------
|[Role Based Access Control (RBAC)](#rbac)|Allows Teleport administrators to define User Roles and restrict each role to specific actions. RBAC also allows to partition cluster nodes into groups with different access permissions.
|[External User Identity Integration](#external-identities)| Allows Teleport to integrate with existing enterprise identity systems. Examples include Active Directory, Github, Google Apps and numerous identity middleware solutions like Auth0, Okta, and so on. Teleport supports LDAP, SAML and OAuth/OpenID Connect protocols to interact with them.
|[Integration with Kubernetes](#integration-with-kubernetes)| Teleport embedded into Kubernetes is available as a separate offering called [Telekube](http://gravitational.com/telekube/). Telekube allows users to define and enfroce company-wide policies like "Developers Must Never Have Access to Production Data" and Telekube will enforce these rules on both the SSH and Kubernetes API level. Another use case of Telekube is to run Kubernetes clusters on multi-region infrastructure, even in behind-firewalls environments. 
|Commercial Support | In addition to these features, Teleport Enterprise also comes with a premium support SLA with guaranteed response times. 

!!! tip "Contact Information":
    If you are interested in Teleport Enterprise or Telekube, please reach out to
    `sales@gravitational.com` for more information.

## External Identities

Teleport Enterprise can authenticate users against a corporate identity
management system and map user group membership to Teleport SSH roles. This
allows to integrate the SSH access management into the same single sign on
(SSO) system used by the rest of the organization.

Examples of such systems include commercial solutions like [Okta](https://www.okta.com),
[Auth0](https://auth0.com/), [SailPoint](https://www.sailpoint.com/), 
[OneLogin](https://www.onelogin.com/) or [Active Directory](https://en.wikipedia.org/wiki/Active_Directory_Federation_Services), as 
well as open source products like [Keycloak](http://www.keycloak.org).
Other identity management systems are supported as long as they provide an
SSO mechanism based on either [SAML](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language) 
or [OAuth2/OpenID Connect](https://en.wikipedia.org/wiki/OpenID_Connect).


### How does SSO work with SSH?

From the user's perspective, nothing changes: they need to execute:

```bash
$ tsh login
```

... once a day to retreive their SSH certificate, assuming that Teleport is
configured with a certificate TTL of 8 hours.

`tsh login` will print a URL into the console, which will open an SSO login
prompt, along with the 2FA as enforced by the SSO provider. If user supplies
valid credentials into the SSO logon proess, Teleport will issue an SSH
certificate.

### Configuring SSO

Teleport works with SSO providers by relying on a concept called
_"authentication connector"_. An auth connector is a plugin which controls how
a user logs in and which group he or she belongs to. 

The following connectors are supported:

* `local` connector type uses the built-in user database. This database can be
  manipulated by `tctl users` command.
* `saml` connector type uses [SAML protocol](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language)
  to authenticate users and query their group membership.
* `oidc` connector type uses [OpenID Connect protocol](https://en.wikipedia.org/wiki/OpenID_Connect) 
  to authenticate users and query their group membership.

To configure [SSO](https://en.wikipedia.org/wiki/Single_sign-on), a Teleport administrator must:

* Update `/etc/teleport.yaml` on the auth server to set the default
  authentication connector.
* Define the connector [resource](admin-guide/#resources) and save it into 
  a YAML file (like `connector.yaml`) 
* Create the connector using `tctl create connector.yaml`.

```bash
# snippet from /etc/teleport.yaml on the auth server:
auth_service:
    # defines the default authentication connector type:
    authentication:
        type: saml 
```

An example of a connector:

```
# connector.yaml
kind: saml
version: v2
metadata:
  name: corporate
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


Teleport can also support multiple connectors. This works via supplying
a connector name to `tsh login` via `--auth` argument:

```bash
$ tsh --proxy=proxy.example.com login --auth=corporate
```

Refer to the following chapters to configure authentication connectors of both
SAML and OIDC types:

* [SAML](saml.md) chapter includes examples for Okta, One Login and Active Directory.
* [OpenID Connect (OIDC)](oidc.md) chapter covers generic OIDC providers.

## RBAC

RBAC stands for `Role Based Access Control`, quoting
[Wikipedia](https://en.wikipedia.org/wiki/Role-based_access_control):

> In computer systems security, role-based access control (RBAC) is an
> approach to restricting system access to authorized users. It is used by the
> majority of enterprises with more than 500 employees, and can implement
> mandatory access control (MAC) or discretionary access control (DAC). RBAC is
> sometimes referred to as role-based security.

... in other words, RBAC allows Teleport administrators to more granular access
control. An example of an RBAC policy can be:  _"admins can do anything, 
developers must never touch production servers and interns can only SSH into
staging servers as guests"_

### How does it work?

Every user in Teleport is **always** assigned a set of roles. The open source
edition of Teleport automatically assigns every user to the built-in "admin"
role, but the Teleport Enterprise allows administrators to define their own
roles with far greater control over the actions users have authorization to
take.

Lets assume a company is using Active Directory to authenticate users and place
them into groups. A typical enterprise deployment of Teleport in this scenario
would look like this:

1. Teleport will be configured to use existing user identities stored in Active
   Directory.
2. Active Directory would have users placed in certain grops, perhaps "interns",
   "developers", "admins", "contractors", etc.
3. The Teleport administrator will have to define Teleport Roles, for
   simplicity sake let them be "users", "developers" and "admins".
4. The last step will be to define mappings from the Active Directory groups (claims) 
   to the Teleport Roles. So every Teleport user will be assigned a role based 
   on the group membership.

### Roles

To manage cluster roles, a Teleport administrator can use the Web UI or the command
line using [tctl resource commands](admin-guide#resources). To see the list of
roles in a Teleport cluster, an administrator can execute:

```bash
$ tctl get roles
```

Some of the permissions a role defines include:

* Which SSH nodes a user can or cannot access. Teleport uses [node
  labels](admin-guide/#labeling-nodes) to do this, i.e. some nodes can be
  labeled "production" while others can be labeled "staging".
* Is this user allowed to replay recorded sessions?
* Is this user allowed to update cluster configuration?
* Which UNIX logins this user is allowed to use when logging into servers?

To learn more, take a look at [RBAC for SSH](ssh_rbac.md) chapter.

## Integration With Kubernetes

Gravitational maintains a [Kubernetes](https://kubernetes.io/) distribution
with Teleport Enterprise integrated, called [Telekube](http://gravitational.com/telekube/). 
Telekube's aim is to dramatically lower the cost of Kubernetes management in a
multi-region / multi-site environment. 

Its highlights:

* Quickly create Kubernetes clusters on any infrastructure using a pre-defined
  "snapshot" of a known cluster state. Each replica of the original cluster
  will contain the same set of binaries, pre-installed components and
  applications as the original cluster.
* Every cluster includes an SSH bastion and can be managed remotely even if behind a firewall.
* Every Kubernetes cluster becomes a Teleport cluster, with all Teleport
  capabilities like session recording, audit, etc.
* Every cluster is dependency free and automomous, i.e. highly available (HA) 
  and includes a built-in caching Docker registry.
* Automated remote cluster upgrades.

Typical users of Telekube are:

* Software companies who want to deploy their Kubernetes applications into
  the infrastructure owned by their customers, i.e. "on-premise".
* Managed Service Providers (MSPs) who manage Kubernetes clusters for their
  clients.
* Enterprises who run many Kubernetes clusters in multiple geographically 
  distributed regions / clouds.

!!! tip "Contact Information":
    For more information about Telekube please reach out us to `sales@gravitational.com` or fill out the contact for on our [website](http://gravitational.com/)
