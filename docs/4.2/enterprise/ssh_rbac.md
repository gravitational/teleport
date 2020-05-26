# Role Based Access Control for SSH

## Introduction

Role Based Access Control (RBAC) gives Teleport administrators more
granular access controls. An example of an RBAC policy could be:  _"admins can do
anything, developers must never touch production servers and interns can only
SSH into staging servers as guests"_

RBAC is almost always used in conjunction with
Single Sign-On ([SSO](https://en.wikipedia.org/wiki/Single_sign-on)) but it also works with
users stored in Teleport's internal database.

### How does it work?

Let's assume a company is using [Okta](https://www.okta.com/) to authenticate users and place
them into groups. A typical deployment of Teleport in this scenario
would look like this:

1. Configure Teleport to use existing user identities stored in Okta.
2. Okta would have users placed in certain groups, perhaps "developers", "admins", "contractors", etc.
3. Teleport would have certain _Teleport roles_ defined. For example: "developers" and "admins".
4. Mappings would connect the Okta groups (SAML assertions) to the Teleport roles.
   Every Teleport user will be assigned a _Teleport role_ based on their Okta
   group membership.

## Roles

Every user in Teleport is **always** assigned a set of roles. One can think of
them as "SSH Roles". The open source edition of Teleport automatically assigns
every user to the built-in `admin` role but the Teleport Enterprise allows
administrators to define their own roles with far greater control over the
user permissions.

Some of the permissions a role could define include:

* Which SSH nodes a user can or cannot access. Teleport uses [node
  labels](../admin-guide/#labeling-nodes) to do this, i.e. some nodes can be
  labeled "production" while others can be labeled "staging".
* Ability to replay recorded sessions.
* Ability to update cluster configuration.
* Which UNIX logins a user is allowed to use when logging into servers.

A Teleport `role` works by having two lists of rules: `allow` rules and `deny` rules.
When declaring access rules, keep in mind the following:

* Everything is denied by default.
* Deny rules get evaluated first and take priority.

A rule consists of two parts: the resources and verbs. Here's an example of an
`allow` rule describing a `list` verb applied to the SSH `sessions` resource.  It means "allow
users of this role to see a list of active SSH sessions".

```yaml
allow:
    - resources: [session]
      verbs: [list]
```

If this rule was declared in `deny` section of a role definition, it effectively
prohibits users from getting a list of trusted clusters and sessions. You can see
all of the available resources and verbs under the `allow` section in the `admin` role configuration
below.

To manage cluster roles, a Teleport administrator can use the Web UI or the command
line using [tctl resource commands](../admin-guide.md#resources). To see the list of
roles in a Teleport cluster, an administrator can execute:

```bsh
$ tctl get roles
```

By default there is always one role called `admin` which looks like this:

```yaml
kind: role
version: v3
metadata:
  name: admin
spec:
  # SSH options used for user sessions with default values:
  options:
    # max_session_ttl defines the TTL (time to live) of SSH certificates
    # issued to the users with this role.
    max_session_ttl: 8h
    # forward_agent controls whether SSH agent forwarding is allowed
    forward_agent: true
    # port_forwarding controls whether TCP port forwarding is allowed
    port_forwarding: true
    # determines if SSH sessions to cluster nodes are forcefully terminated
    # after no activity from a client (idle client). it overrides the global
    # cluster setting. examples: "30m", "1h" or "1h30m"
    client_idle_timeout: never
    # determines if the clients will be forcefully disconnected when their
    # certificates expire in the middle of an active SSH session.
    # it overrides the global cluster setting.
    disconnect_expired_cert: no

  # allow section declares a list of resource/verb combinations that are
  # allowed for the users of this role. by default nothing is allowed.
  allow:
    # logins array defines the OS/UNIX logins a user is allowed to use.
    # a few special variables are supported here (see below)
    logins: [root, '{% raw %}{{internal.logins}}{% endraw %}']
    # if kubernetes integration is enabled, this setting configures which
    # kubernetes groups the users of this role will be assigned to.
    # note that you can refer to a SAML/OIDC trait via the "external" property bag,
    # this allows you to specify Kubernetes group membership in an identity manager:
    kubernetes_groups: ["system:masters", "{% raw %}{{external.trait_name}}{% endraw %}"]]

    # list of node labels a user will be allowed to connect to:
    node_labels:
      # a user can only connect to a node marked with 'test' label:
      'environment': 'test'
      # the wildcard ('*') means "any node"
      '*': '*'
      # labels can be specified as a list:
      'environment': ['test', 'staging']
      # regular expressions are also supported, for example the equivalent
      # of the list example above can be expressed as:
      'environment': '^test|staging$'

    # list of allow-rules. see below for more information.
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
`{% raw %}{{internal.logins}}{% endraw %}` | Substituted with "allowed logins" parameter used in `tctl users add [user] <allowed logins>` command. This applies only to users stored in Teleport's own local database.
`{% raw %}{{external.xyz}}{% endraw %}`    | Substituted with a value from an external [SSO provider](https://en.wikipedia.org/wiki/Single_sign-on). If using SAML, this will be expanded with "xyz" assertion value. For OIDC, this will be expanded a value of "xyz" claim.

Both variables above are there to deliver the same benefit: they allow Teleport
administrators to define allowed OS logins via the user database, be it the
local DB, or an identity manager behind a SAML or OIDC endpoint.

#### An example of a SAML assertion:

Assuming you have the following SAML assertion attribute in your response:

```
<Attribute Name="http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname">
        <AttributeValue>firstname.lastname</AttributeValue>
</Attribute>
```

... you can use the following format in your role:
```
logins:
   - '{% raw %}{{external["http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname"]}}{% endraw %}'
```


### Role Options

As shown above, a role can define certain restrictions on SSH sessions initiated by users.
The table below documents the behavior of each option if multiple roles are assigned to a user.

Option                    | Description                          | Multi-role behavior
--------------------------|--------------------------------------|---------------------
`max_session_ttl`         | Max. time to live (TTL) of a user's SSH certificates | The shortest TTL wins
`forward_agent`           | Allow SSH agent forwarding         | Logical "OR" i.e. if any role allows agent forwarding, it's allowed
`port_forwarding`         | Allow TCP port forwarding          | Logical "OR" i.e. if any role allows port forwarding, it's allowed
`client_idle_timeout`     | Forcefully terminate active SSH sessions after an idle interval | The shortest timeout value wins, i.e. the most restrictive value is selected
`disconnect_expired_cert` | Forcefully terminate active SSH sessions when a client certificate expires | Logical "OR" i.e. evaluates to "yes" if at least one role requires session termination


## RBAC for Hosts

A Teleport role can also define which hosts (nodes) a user can have access to.
This works by [labeling nodes](../admin-guide.md#labeling-nodes) and listing
allow/deny labels in a role definition.

Consider the following use case:

The infrastructure is split into staging/production environments using labels
like `environment=production` and `environment=staging`. You can create roles
that only have access to one environment. Let's say you create an intern role
with allow rule for label `environment=staging`.

### Example

The role below allows access to all nodes labeled "env=stage" except those that
also have "workload=database" (these will always be denied).

Access to any other nodes will be denied:

```yaml
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
      # multiple labels are interpreted as an "or" operation.  in this case
      # Teleport will deny access to any node labeled as 'database' or 'backup'
      'workload': ['database', 'backup']
```

!!! tip "Dynamic RBAC"

    Node labels can be dynamic, i.e. determined at runtime by an output
    of an executable. In this case, you can implement "permissions follow workload"
    policies (eg., any server where PostgreSQL is running becomes _automatically_
    accessible only by the members of the "DBA" group and nobody else).

## RBAC for Sessions

As shown in the role example above, a Teleport administrator can restrict
access to user sessions using the following rule:

```yaml
rules:
  - resources: [session]
    verbs: [list, read]
```

* "list" determines if a user is allowed to see the list of past sessions.
* "read" determines if a user is allowed to replay a session.

It is possible to restrict "list" but to allow "read" (in this case a user will
be able to replay a session using `tsh play` if they know the session ID)



## FAQ

**Q:** What if a node has multiple labels?

**A:** In this case, the access will be granted only if **all of the labels**
defined in the role are present. This effectively means Teleport uses an "AND"
operator when evaluating node-level access using labels.

**Q:** Can I use node-level RBAC with OpenSSH servers?

**A:** No. OpenSSH servers running `sshd` do not have the ability to label
themselves. This is one of the reasons to run Teleport `node` service instead.
