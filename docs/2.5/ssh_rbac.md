# RBAC for SSH

## Introduction

Role Based Access Control (RBAC) allows Teleport administrators to more
granular access control. An example of an RBAC policy can be:  _"admins can do
anything, developers must never touch production servers and interns can only
SSH into staging servers as guests"_

RBAC is almost always used in conjunction with
[SSO](https://en.wikipedia.org/wiki/Single_sign-on) but it also works with
users stored in Teleport's internal database. 

### How does it work?

Lets assume a company is using [Okta](https://www.okta.com/) to authenticate users and place
them into groups. A typical enterprise deployment of Teleport in this scenario
would look like this:

1. Configure Teleport to use existing user identities stored in Okta.
2. Okta would have users placed in certain grops, perhaps "developers", "admins", "contractors", etc.
3. Define _Teleport roles_, for simplicity sake let them be "developers" and "admins".
4. Define mappings from the Okta groups (SAML assertions) to the Teleport roles. 
   Every Teleport user will be assigned a _Teleport role_ based on their Okta
   group membership.

## Roles

Every user in Teleport is **always** assigned a set of roles. One can think of
them as "SSH Roles". The open source edition of Teleport automatically assigns
every user to the built-in "admin" role, but the Teleport Enterprise allows
administrators to define their own roles with far greater control over the
actions users have authorization to take.

Some of the permissions a role defines include:

* Which SSH nodes a user can or cannot access. Teleport uses [node
  labels](admin-guide/#labeling-nodes) to do this, i.e. some nodes can be
  labeled "production" while others can be labeled "staging".
* Is this user allowed to replay recorded sessions?
* Is this user allowed to update cluster configuration?
* Which UNIX logins this user is allowed to use when logging into servers?

A _Teleport role_ works by having two lists of rules: _"allow rules"_ and _"deny" rules_. 
When declaring access rules, keep in mind the following:

* Everything is denied by default.
* Deny rules get evaluated first and take priority.

A rule consists of two parts: the resources and verbs. Here's an example of an
"allow" rule describing "list" verb applied to SSH sessions.  It means "allow
users of this role to see a list of active SSH sessions".

```bash
allow:
    - resources: [session]
      verbs: [list]
```

If this rule was declared in `deny` section of a role definition, it effectively
prohibits users from getting a list of trusted clusters and sessions.

To manage cluster roles, a Teleport administrator can use the Web UI or the command
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
`{{ internal.logins }}` | Substituted with "allowed logins" parameter used in `tctl users add [user] <allowed logins>` command. This applies only to users stored in Teleport's own local database.
`{{ external.xyz }}`    | Substituted with a value from an external [SSO provider](https://en.wikipedia.org/wiki/Single_sign-on). If using SAML, this will be expanded with "xyz" assertion value. For OIDC, this will be expanded a value of "xyz" claim.

Both variables above are there to deliver the same benefit: it allows Teleport
administrators to define allowed OS logins via the user database, be it the
local DB, or an identity manager behind a SAML or OIDC endpoint.


## RBAC for Hosts

A Teleport role can also define which hosts (nodes) a user can have access to.
This works by [labeling nodes](admin-guide/#labeling-nodes) and listing 
allow/deny labels in a role definition. 

Consider the following use case:

The infrastructure is split into staging/production environments using labels
like `environment=production` and `environment=staging`. You can create roles
that only have access to one environment. Let's say you create an intern role
with allow rule for label `environment=staging`.

### Example

The role below allows access to all nodes labeled "env=stage" except those that
also have "worload=database" (these will always be denied).

Access to any other nodes will be denied:

```bash
kind: role
version: v3
metadata:
  name: example-role
spec:
  allow:
    node_labels:
      'env': 'stage'

  deny:
    node_labels:
      'workload': 'database'
```

!!! tip "Dynamic RBAC":
    Remember that node labels can be dynamic, i.e. determined at runtime by an output
    of an executable. In this case you can implement "permissions follow workload"
    policy, i.e. any server where PostgreSQL is running becomes _automatically_
    accessible only by the members of the "DBA" group and nobody else.


## Q&A

**Q:** But what if a node has multiple labels?

**A:** In this case the access will be granted only if **all of the labels**
defined in the role are present. This effectively means Teleport uses an "AND"
operator when evaluating node-level access using labels. 

**Q:** How can I use node-level RBAC with OpenSSH servers?

**A:** No. OpenSSH servers running `sshd` do not have the ability to label
themselves. This is one of the reasons to run Teleport SSH service instead.
