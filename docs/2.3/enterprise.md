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

## RBAC

RBAC stands for `Role Based Access Control`, quoting
[Wikipedia](https://en.wikipedia.org/wiki/Role-based_access_control):

> In computer systems security, role-based access control (RBAC) is an
> approach to restricting system access to authorized users. It is used by the
> majority of enterprises with more than 500 employees, and can implement
> mandatory access control (MAC) or discretionary access control (DAC). RBAC is
> sometimes referred to as role-based security.

Every user in Teleport is **always** assigned a set of roles. The open source
edition of Teleport automatically assigns every user to the "admin" role, but
the Teleport Enterprise allows administrators to define their own roles with
far greater control over the actions users have authorization to take.

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
   to the Teleport Roles, so every Teleport user will be assigned a role based 
   on the group membership.

### Roles

To manage cluster roles a Teleport administrator can use the Web UI or the command
line using [tctl resource commands](admin-guide#resources). To see the list of
roles in a Teleport cluster, an administrator can execute:

```bash
$ tctl get roles
```

By default there is always one role called "admin" which looks like this:

```bash
kind: role
version: v3
metadata:
  name: admin
spec:
  # SSH options used for user sessions 
  options:
    # max_session_ttl defines the TTL (time to live) of SSH certificates 
    # issued to the users with this role.
    max_session_ttl: 8h

    # forward_agent controls either users are allowed to use SSH agent forwarding
    forward_agent: true

  # allow section declares a list of resource/verb combinations that are
  # allowed for the users of this role. by default nothing is allowed.
  allow:
    # logins array defines the OS logins a user is allowed to use.
    # A few special variables are supported here (see below)
    logins: [root, '{{internal.logins}}']

    # node labels that a user can connect to. The wildcard ('*') means "any node"
    node_labels:
      '*': '*'

    # see below.
    rules:
    - resources: [role]
      verbs: [list, create, read, update, delete]
    - resources: [auth_connector]
      verbs: [connect, list, create, read, update, delete]
    - resources: [session]
      verbs: [list, read]
    - resources: [trusted_cluster]
      verbs: [connect, list, create, read, update, delete]

  # the deny section uses the identical format as the 'allow' section.
  # the deny rules always override allow rules.
  deny: {}
```

The following variables can be used with `logins` field:

Variable                | Description
------------------------|--------------------------
`{{ internal.logins }}` | Substituted with "allowed logins" parameter used in 'tctl users add [login] <allowed logins>' command. This is applicable to the local user DB only.
`{{ external.XYZ }}`    | For SAML-authenticated users this will get substituted with "XYZ" assertion value. For OIDC-authenticated users it will get substituted with "XYZ" claim value.

Both variables above are there to deliver the same benefit: it allows Teleport
administrators to define allowed OS logins via the user database, be it the
local DB, or an identity manager behind a SAML or OIDC endpoint.

**Node Labels**

A user will only be granted access to a node if all of the labels defined in
the role are present on the node. This effectively means we use an AND
operator when evaluating access using labels. Two examples of using labels to
restrict access:

1. If you split your infrastructure at a macro level with the labels
`environment: production` and `environment: staging` then you can create roles
that only have access to one environment. Let's say you create an `intern`
role with allow label `environment: staging` then interns will not have access
to production servers.
1. Like above, suppose you split your infrastructure at a macro level with the
labels `environment: production` and `environment: staging`. In addition,
within each environment you want to split the servers used by the frontend and
backend teams, `team: frontend`, `team: backend`. If you have an intern that
joins the frontend team that should only have access to staging, you would
create a role with the following allow labels
`environment: staging, team: frontend`. That would restrict users with the
`intern` role to only staging servers the frontend team uses.

**Rules**

Each role contains two lists of rules: "allow" rules and "deny" rules. Deny
rules get evaluated first, and a user gets "access denied" error if there's a
deny rule match.

Each rule consists of two lists: the list of resources and verbs. Here's an example of
a rule describing "list" access to sessions and trusted cluters:

```bash
- resources: [session, trusted_cluster]
  verbs: [connect, list, create, read, update, delete]
```

If this rule is declared in `deny` section of a role definition, it effectively
prohibits users from getting a list of trusted clusters and sessions.

## External Identities

Teleport Enterprise allows to authenticate users against a corporate identity management
system and map their group membership to Teleport SSH roles.

Any identity management system can be used with Teleport as long as it implements a 
single sign on (SSO) mechanism via [OpenID Connect](https://en.wikipedia.org/wiki/OpenID_Connect) 
or [SAML](https://en.wikipedia.org/wiki/Security_Assertion_Markup_Language). 
Examples of such systems include commercial solutions like [Okta](https://www.okta.com) or 
[Active Directory](https://en.wikipedia.org/wiki/Active_Directory_Federation_Services), as 
well as open source products like [Keycloak](http://www.keycloak.org).

Refer to the following links for additional additional integration documentation:

* [OpenID Connect (OIDC)](oidc.md)
* [Security Assertion Markup Language 2.0 (SAML 2.0)](saml.md)

## Integration With Kubernetes

Gravitational maintains a [Kubernetes](https://kubernetes.io/) distribution
with Teleport Enterprise integrated, called [Telekube](http://gravitational.com/telekube/). 

Telekube's aim is to dramatically lower the cost of Kubernetes management in a
multi-region / multi-site environment. 

Its highlights:

* Quickly create Kubernetes clusters on any infrastructure.
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
