
---
authors: @klizhentas (sasha@goteleport.com)
state: draft
---

# RFD 0163 - Scoped RBAC

## Required Approvers

* Engineering @r0mant && (@tigrato || @marcoandredinis)
* Security: (@reedloden || @jentfoo)
* Product: (@xinding33 || @klizhentas )

## What

This RFD introduces resource hierarchies and scopes to existing RBAC. 
Our goal is simplify and evolve access control in Teleport without drastic changes or new policy languages. 
Make it easier to integrate Teleport RBAC with cloud IAMs of AWS, GCP and Azure.
This RFD is closely modeled and inspired by Azure RBAC model, the most advanced out of 3 clouds. 

Read about it here https://learn.microsoft.com/en-us/azure/role-based-access-control/overview before diving into this RFD.

## Why

There are several structural issues with the current RBAC model in Teleport.

### Scalability issues 

Every role has to be distributed to every node or proxy that has to perform authorization. 
Current roles are brittle - to evaluate access, every single role has to be fetched and processed, 
because each role can have a deny rule that can block access to any allowed rule.

### Scoping issues

It is not possible to describe “delegated admins” in RBAC, when one user has administrative access over part of the cluster.
It is also not possible to specify that certain role options only apply in certain use-cases. 

For example, the setting `permit-agent-forward: false` will deny agent-forward to any matching resource with no exceptions, even if other roles allow it.

It is not possible to allow admins to grant roles to other users but with certain restrictions, as in the example of issue https://github.com/gravitational/teleport/issues/16914. 

### Complexity 

Roles today have both labels and label expressions, login rules to inject traits and claims and templates. 
This creates a complicated system that is hard to understand and reason about.
Role mapping is brittle and requires updating OIDC/SAML connector resources, which can break access.

### Security issues

Every role assignment and trait is encoded in certificate, and each time a user gets their roles updated, they have to get a new certificate. 
Old certificates can be re-used to get privileges that have been removed, 
creating “a new enemy problem” described in [Zanzibar Paper](https://research.google/pubs/zanzibar-googles-consistent-global-authorization-system/).

Many roles allow “role escapes”, as any person who gets assigned a role that can create other roles, 
would become an admin, see for example issue https://github.com/gravitational/teleport.e/issues/3111

### Goals

Our key goal is to evolve Teleport’s roles without asking users to rewrite their existing RBAC.
We also would like to better integrate Teleport RBAC with cloud provider’s IAM systems out of the box.
We would like to give Teleport’s users “batteries included” approach, when they can get 90% of the use-cases done without c
reating any new roles, or modifying existing ones.

### Non-Goals

We are not going to implement backwards-incompatible changes that require our customers rewrite their RBAC stack or adopt a completely new policy language. 

## Details
To understand the required changes, let’s first take a look at Teleport RBAC structure. 

### RBAC Primer

Let’s start with fundamental Teleport RBAC concepts and highlight some issues as we review them.

#### Roles

Each user or bot in Teleport is assigned one or several roles. 
For the full reference, take a look at the documentation at https://goteleport.com/docs/reference/resources/#role.

Here is a role structure:

```yaml
kind: role
version: v7
metadata:
  # role name is unique across the entire cluster
  name: example
spec:
  # options are a set of knobs that apply across the entire role set.
  # the most restrictive options wins
  options:
    # max_session_ttl defines the TTL (time to live) of certificates
    # issued to the users with this role.
    max_session_ttl: 8h
    # forward_agent controls whether SSH agent forwarding is allowed
    forward_agent: true
  # The allow section declares a list of resource/verb combinations that are
  # allowed for the users of this role. By default, nothing is allowed.
  #
  # Allow rules specify both actions that are allowed and match the resources
  # they are applying to.
  allow:
    # Some Allow fields specify target protocol
    #  login principals, like in this example, SSH logins
    logins: [root, '{{internal.logins}}']

    # In this example, the fields specify a set of windows desktop logins
    windows_desktop_logins: [Administrator, '{{internal.logins}}']

    # There are multiple types of labels and label expressions that 
    # match different computing resources.
    node_labels:
      # literal strings:
      'env': 'test'
    # regexp expressions
      'reg': '^us-west-1|eu-central-1$'

    # node_labels_expression has the same purpose as node_labels but
    # supports predicate expressions to configure custom logic.
    # A user with this role will be allowed to access nodes if they are in the
    # staging environment *or* if they belong to one of the user's own teams.
    node_labels_expression: |
      labels["env"] == "staging" ||
      contains(user.spec.traits["teams"] , labels["team"])
 
    # rules allow a user holding this role to modify other resources
    # matching the expressions below.
    # rules match both resources and specify what actions are allowed
    rules:
      - resources: [role]
        verbs: [list, create, read, update, delete]

  # The deny section uses the identical format as the 'allow' section.
  # The deny rules always override allow rules.
  deny: {}
```

We can spot several issues with this role design:

* There is no way to specify the scope that role applies to, all roles apply globally to all resources they match and all users they assign to all the time.
* There is no way to specify resource hierarchies, like computing environments (env vs prod), which makes it hard to partition the infrastructure and forces customers to specify labels.


#### Labels

Teleport RBAC’s advice to engineers to partition their resource is to first, label their computing resource or use AWS labels, and second, match the labels in RBAC.

Admins can set the tags on the resource configuration file statically, or, for some resources, use `server_info` to set the tags for each resource:

```yaml
# server_info.yaml
kind: server_info
metadata:
   name: si-<node-name>
spec:
   new_labels:
      "foo": "bar"
```

This creates several issues:

* It is not always secure to delegate labeling to owners of computing resources, as anyone with root access to the node config file can update its labels impacting everyone else.
* It is not scalable, as it’s not always useful to set and updates tags for each individual resource
* It’s hard or impossible to partition infrastructure with two-dimensional labels, although users can use `env: prod` to mark all resources in the production environment, there is no way to say that `env: lab` is a subset of `env: prod`.

#### Current Roles mapping

In Teleport there are multiple ways to map roles to users: static roles mapping to local users and bots, 
dynamic mapping to SSO via connectors and on-demand assignment via access lists and access requests.

Most users start with local static mapping and SSO mapping, later graduating to access requests and access lists.

https://goteleport.com/docs/access-controls/sso/#creating-an-authentication-connector

For SSO users, on login, Teleport checks SSO connector, and maps attributes of a user to a list of roles:

```yaml
# connector.yaml
kind: saml
version: v2
metadata:
  name: corporate
spec:
  attributes_to_roles:
    - {name: "groups", value: "okta-admin", roles: ["access"]}
     # regular expressions with capture are also supported. 
     # the next line instructs Teleport
     # to assign users to roles `admin-1` if his SAML "group" 
     # attribute equals 'ssh_admin_1':
   - { name: "group", value: "^ssh_admin_(.*)$", roles: ["admin-$1"] }
```

This creates several security and scalability issues:

* Every time administrators need to change roles assignment, they have to update the resource.
* Every time the Identity Provider changes user’s attributes, the users would have to relogin to get new roles. 

For local users, administrators have to specify roles and traits in the local resource:

```yaml
kind: user
version: v2
metadata:
  name: joe
spec:
  # roles is a list of roles assigned to this user
  roles:
  - admin
  # traits are key, list of values pairs assigned to a user resource.
  # Traits can be used in role templates as variables.
  traits:
    logins:
    - joe
    - root
```

This also creates some challenges, as administrators are forced to update local resources each time they have to assign a user new permission.

### Modifications

Let’s now introduce the missing pieces of the puzzle and review how new roles will simplify or deprecate legacy concepts.

#### Hierarchical Resource Groups

Resource groups are one such missing piece - in most cloud environments, resources are split into hierarchy, 
for example, host `luna` is a member of a resource group `lab`, in turn a member of environment `prod`, which is in turn a member of a Teleport cluster.

In Teleport, we will make cluster a default root of this hierarchy. Every computing resource by default will be a direct member of a root `cluster` resource group.

One resource can be assigned to multiple resource groups at a time, or none. 

In our example, the cluster administrator would define lab resource group in the following way:

```yaml
kind: resource_group
metadata:
  name: lab
spec:
  parent: prod_env
```

Administrators can assign resources to resource groups:

```yaml
kind: resource_group
metadata:
  name: lab
spec:
   parent: prod_env
 match_kinds:
   - node
   - database
   - role
   - access_list
   - '.*'
 match_labels:  
   env: prod
```

In this case, any resource that matches `env:prod` label will be assigned to this resource group. We will use the same label matching algorithm as in today's `discovery_service`. 

This will let administrators to gradually migrate their existing flat infrastructure to resource groups one.

In some cases it makes sense to specify parent resource group inline:

```yaml
kind: role
spec:
  parent_resource_group: /env/prod
```

By default, if unspecified, a resource is a member of a root-level - `/` cluster resource group. If specified by the resource, it won't be a member of a root `/` resource group.

Resource groups are hierarchical, and we can refer to the `lab` resource group by its full path as `/env/prod/lab`. 

Most Teleport resources, with some exceptions, like users, SSO connectors can be a member of a resource group. 

We will list those resources separately below.

##### Default Resource groups via auto-discovery

Teleport can create resource groups if admins turn on auto discovery. This will significantly simplify configuration. 

Here are some of the resource groups that Teleport Discovery service will create:

* For AWS, Teleport discovery service will place each computing resource in `/aws/[account-id]/[region]/[resource-type]/[resource-id]`.
  + When EKS auto-discovery is on, this hierarchy will include discovered apps - `/aws/account-id/[region]/k8s/[cluster-name]/namespaces/[namespace]/[app-id]`
* For Azure, Teleport will use Azure's hierarchy - `/azure/[management-group]/[subscription]/[resource-group]/[resource-type]/[resource-id]`
* For GCP, we will use GCP hierarchy of `/gcp/[organization]/[folder]/[project]/[resource-type]/[resource-id]`

Discovery service will create and remove these hierarchies based on the cloud state, and will create resources with `parent_resource_group` field to place them in those resource groups.

If users are not happy with a default hierarchy, they can create a different one.

#### Access Lists

Teleport has a concept of access lists, that lists an owner, members, and optionally a parent access list.
Access List in Teleport represents a group of users with a hierarchy. 

We will further assume that the root of this hierarchy is a cluster. 

Unlike in resource groups, a user can be an owner and a member of none, one or several access lists at once.

In addition to that, access list grants a role to a set of members, like in this example:

```yaml
kind: access_list
metadata:
  name: "lab-engineers"
spec:
  desc: "Access list for lab engineers"
  grants:
    roles: [access]
  members:
    - name: bob@example.com
```

We will return to access lists later, but let’s now recall that access lists contain a list of members, who are, in turn, granted one or several roles.

#### Scopes

By default, in Teleport a role is granted it applies to all resources in the Teleport cluster.

However, with this change, will be able to grant a set of roles that apply only to resources that belong to a specific resource group. 

In this case, we will say that the roles apply at the scope of the resource group.

Scopes define a set of resources roles apply to. 

We will introduce scopes in a couple of places, first, for access list:

```yaml
kind: access_list
metadata:
  name: "lab-engineers"
spec:
  desc: "Access list for lab engineers"
  # this grant applies only at the scope of the resource group `/env/prod/lab`
  scopes: [‘/env/prod/lab']
  grants:
    roles: [access]
  members:
    - name: bob@example.com
```

By default, all existing access lists will grant roles at the cluster scope, cascading to all resources, just like before the migration. 

However, going forward, admins will be able to set scopes to more granular levels.

The second place where we introduce scopes is in the roles:

```yaml
kind: role
metadata:
 name: access
spec:
  grantable_scopes: ['/env/prod']
```

Grantable scopes specifiy maximum scope this role can be granted on. 

**Important:** By default, if the `grantable_scopes` are missing, we assume empty scope - that will prevent the role from being granted on any scopes. When migrating existing roles, we would set `/` - root scope to avoid breaking the cluster. 

To sum it up, any role is granted to a set of users present in the access list, to a set of resources specified in the resource group.

Grants are cascading, if a role is granted to a parent access list, it is also granted to members of any child access lists.

If a role is granted at a scope of an access list, and this role can in turn give ability to request roles or impersonate, the roles and resources 
and impersonation must be bound to the same scope or the scope smaller than the original one.

For example, let’s assume that the access list granted Alice the requester role described below at the scope of `/env/prod/lab`.  

In this case, Alice would get the ability to search and request resources with an access role, but only in the scope of `/env/prod/lab`.

```yaml
# requester.yaml
kind: role
version: v5
metadata:
  name: requester
spec:
  allow:
    request:
      search_as_roles:
        - access
```

The same applies to impersonation, if access list granted Alice the role `impersonator` below at scope `/env/prod/lab`, 
Alice would be able to impersonate role `jenkins`, but only at the scope `/env/prod/lab` or a more specific one, e.g. `/env/prod/lab/cabinet-west`.

```yaml
kind: role
version: v5
metadata:
  name: impersonator
spec:
  allow:
    impersonate:
      users: ['jenkins']
      roles: ['jenkins']
```

**Note:** While it’s tempting to support scope templates, we will push this out of the scope of this RFD.

Each scope is a valid URI, either starting with a path `/leafs/path/etc` or with scheme prefix: `admin:/path`. URI syntax will let us expand definition of the scope going forward.

#### The Access verb

Alice would like to create a role that denies access to all apps in some scope. To achieve this without labels, we introduce a new verb `access`:

```yaml
kind: role
metadata:
   name: no-apps
spec:
  deny:
      rules:
      - resources: [app]
        verbs: [access]
```

When granted at scope, the role above will deny access to any apps in this scope.

We will use `access` verb during migration. The following V7 and V8 roles are equivalent:

```yaml
kind: role
version: V7
metadata:
   name: all-apps
spec:
  app_labels:
    '*': '*'
```

```yaml
kind: role
version: V8
metadata:
   name: all-apps
spec:
  allow:
    rules:
    - resources: [app]
      verbs: [access]
```

We will use the verb `access` for any supported resource, `node`, `k8s`, `db`, `app`, `desktop`, etc.

Most of Teleport's preset roles have labels `*`: `*`, so migration will be straightforward.

#### Roles and Access Lists in resource groups

A special case is when a role or an access list is assigned to a certain resource group.  

Only roles that have `grantable_scope` matching the resource group can be assigned to the resource group.

The same applies to access lists, the scope of the access list grants should always match the scope of the roles it grants access to and 
should not exceed the scope of the access list itself.

In both of those cases, parent resource group must be specified both for access lists and roles and should equal or be more specific than the scope it was created in:

```yaml
kind: role
metadata:
  name: lab-admin
spec:
  grantable_scopes: ['/env/prod/lab']
  parent_resource_group: /env/prod/lab
```

```yaml
kind: access_list
metadata:
  name: lab-personnel
spec:
  scopes: ['/env/prod/lab']
  parent_resource_group: /env/prod/lab
```

Roles created within a scope will have `grantable_scope` and `parent_resource_group` to be equal to scope, or more specific. 

For example, any role created within a scope `/env/prod/lab` must have the same `grantable_scope` and `parent_resource_group`  - `/env/prod/lab` or more specific one, 
for example `/env/prod/lab/west-wing`.

The roles and access lists created in the scope must grant access to scopes equal, or more specific than the scope they were created in to prevent lateral expansion or permissions.

These invariants will let us make sure that any role created within a scope, will only grant permissions in the same, or more specific scope. 

We will apply the same invariants to any other resources created within a scope. 

#### Scoped Join Tokens

Join tokens with `scopes` present will limit the resource groups the resources can join to.

For example, Teleport service using the scoped token below will only be able to join the resource group `dev`

```yaml
# token.yaml
kind: token
version: v2
metadata:
  name: my-token-name
spec:
  scopes: ['/dev']
  roles: 
    - Node
    - App
```

Join tokens with a `parent_resource_group` set can only have scope equal to this resource group.

Join tokens created by roles granted within a scope must have `parent_resource_group` and `scopes` equal to this scope `/dev` or a more specific scope, e.g. `/dev/lab`. 

**Note:** To implement this, the token can be exchanged for the host certificate with `parent_resource_id` encoded in it. This way nodes can't set the nodes to any resource groups other than the parent.
By default, all existing join tokens will use `/` as a default resource group.

#### Access Requests

Access requests are bound to the scope they are created within, when Alice requests access to environment `/dev/lab` with role access, 
the access request will capture the scope `/dev/lab` and create grant at this scope for Alice when approved.

#### Scoped Audit Log Events and Session Recordings

The audit events and session recordings generated by activity in some scope, will have a property that binds them to the same scope. 

This will let us filter and grant access to a subset of events and session recordings within a scope.

#### Resources that can’t be created at non-admin scope

Some resources don't have a clear cut behavior at nested scopes, like SSO connectors, or are difficult to define, like for users. To address this issue, we will define a new `admin:` scope hierarchy that is parallel
to the `/` hierarchy and will require roles that grant access to this set of resources to use this and only this hierarchy.

Here is a list of resources that can’t be created at any scopes other than `admin:`

* SSO connectors
* Users
* Bots
* Clusters
* Login rules
* Devices
* UI Configs
* Cluster auth preference
* Join tokens for roles Proxy, Auth

In the `V7` role version, we will let users to mix `admin:` scope that includes resources other than `admin:`, however in role `V8` we will require the role to **only** include the admin scope resources:

```yaml
kind: role
version: v8
metadata:
  # role name is unique across the entire cluster
  name: example
spec:
  grantable_scopes: 'admin:'
  allow:
    # only admin scope is allowed in V8, 
    rules:
      - resources: [user, bot]
        verbs: [list, create, read, update, delete]

  # The deny section uses the identical format as the 'allow' section.
  # The deny rules always override allow rules.
  deny: {}
```

Access lists that grant roles in `admin:` scope also have to have scopes to be explicitly set to only `admin:` scope and nothing else.

```yaml
kind: access_list
metadata:
  name: access-to-lab
spec:
  grants: 
    roles: [access]
    traits:
      'internal.logins' : 'root'
  # only root and `admin:` scope is allowed if grants include roles that contain resources requiring `admin:` scope.
  scopes: ['admin:']
  members:
    - bob@example.com
```

This will help to separate non-admin and admin resources more clearly and in the UI we should mark roles and access list in `admin:` scopes with label `admin`.

### Trusted Clusters

With new scoped RBAC approach, leaf clusters will sync users from the root cluster, similarly to how we sync users from Okta via SCIM. 
Leaf clusters will also sync root cluster's Access Lists similarly to how Teleport syncs access lists from Okta, see RFD 0019e - Okta Access Lists Sync.
Combined together, Users and Access Lists sync will let leaf clusters mirror permissions from the root cluster, while remaining independent, as leafs can have their own access lists and users.

To let users access resources in specific clusters, we will use `/=leafs/[cluster-name]` scope. Leafs hierarchy is a part of a root `/` cluster hierarchy. This will allow transparent migration of resources. To avoid name collisions we will reserve paths that contain `=` and will prohibit users from creating resource groups that have character `=` in them.

```yaml
kind: access_list
metadata:
  name: access-to-lab
spec:
  grants: 
    roles: [access]
    traits:
      'internal.logins' : 'root'
  scopes: ['/dev/lab']
  members:
    - bob@example.com
```

Leaf clusters syncing access lists and users from the root clusters should avoid name collisions - if a local or SSO leaf cluster, role or access list exists, the sync should avoid overwriting the local leaf cluster data, emitting audit event that mentions that the system did not sync the list.

This architecture lets leafs to have their own indpendent grants, while mirroring users, access lists from the root, which represents majority of today's use cases.

### Features we will deprecate over time

All existing Teleport features will keep working with no changes, however, with this design we plan to deprecate:

* The mappings in connectors `attributes_to_roles` in favor of Access Lists integrated with SCIM and identity providers. Any grants of roles will be governed by access lists.
* Certificate extensions with roles and traits. A new access control system will no longer rely on certificate metadata to identify what roles have been assigned to users. The only data that new access control requires is information about user identity - username. Teleport will propagate grants via backend. This will lower Teleport’s resiliency to auth server failures, but this will be compensated with modern database backends like CockroachDB that provide multi-region resiliency and failover. 
* Label matchers and resource matchers in roles. We will have to support those for a long time, but those labels will always apply at the granted scope to resources in a resource group, and will become redundant, with later versions of Teleport relying on auto-discovery and assignment of resources to resource groups.
* Login rules. We will recommend replacing login rules with access lists that provide similar functionality.

## User stories

Let’s get back to the issues we outlined  in the start of this RFD and review how the new system will  help to resolve them.

### Gradual scoping

Most Teleport customers will start with all resources and roles in the cluster scope. 

We will let them introduce resource groups gradually. Let’s create two resource groups, `prod` and `dev` with resource groups `west` and `lab`.

Here is the resource groups hierarchy, where we will assume that mars and luna servers matched the assignments:

```mermaid
graph TD;
    luna(Server luna)-->west;
    west-->prod;
    prod-->cluster;
    lab-->dev;
    mars(Server mars)-->lab;
    dev-->cluster;
```

```yaml
kind: resource_group
metadata:
  name: prod
---
kind: resource_group
metadata:
  name: west
  parent_resource_group: prod
  # would be nice if we could match on AWS specific right away with match_aws
  match_aws:
    account_id: aws-account-id
    region: west
---
kind: resource_group
metadata:
  name: dev
---
kind: resource_group
metadata:
  name: lab
spec:
  parent_resource_group: dev
  # here we will just match on labels
  match_labels:
    kinds: [node]
    env: lab
```

This will let administrators create a resource hierarchy by  mapping computing resources using AWS metadata or labels.

We will use this setup in the following examples.

### SSH access to specific hosts

The most prominent use-case is our over-engineered access role. We can keep this role as is. Today, it grants blanket access to any computing resource of Teleport.

Alice, who is an administrator, would like to restrict access for a user bob@example.com to any server in the lab as root

```yaml
kind: access_list
metadata:
  name: access-to-lab
spec:
  grants: 
    roles: [access]
    traits:
      'internal.logins' : 'root'
  scopes: ['/dev/lab']
  members:
    - bob@example.com
```

Teleport will grant role access  and traits internal.logins: root to `bob@example.com`, but only when Bob would access servers in the resource group `/dev/lab`. 

This grant will not be valid out of the scope of `/dev/lab`, so Bob won’t be able to SSH as root to any other servers.

K8s access to specific clusters

Teleport can autodiscover clusters echo and bravo with namespaces default and prod, creating the following resource group hierarchy:

```
/k8s/namespaces/prod/bravo
/k8s/namespaces/prod/echo

/k8s/namespaces/default/bravo
/k8s/namespaces/default/echo
```

Note that here we have set namespaces, and not cluster names as the root of the resource hierarchy,
so we can group different cluster names by namespace.

We can then use this hierarchy to create access lists specifying access to default namespace in any cluster:

```yaml
kind: access_list
metadata:
  name: access-to-default
spec:
  grants: 
    roles: [access]
    traits:
      'internal.logins' : 'root'
  scopes:  ['/k8s/namespaces/default']
  members:
    - bob@example.com
```

### Scoped search-based access requests

Search-based access requests let users to search and request access to individual resources. Here is a standard requester role:

```yaml
# requester.yaml
kind: role
version: v5
metadata:
  name: requester
spec:
  allow:
    request:
      search_as_roles:
        - access
```

Here is a standard reviewer role:

```yaml
# reviewer.yaml
kind: role
version: v5
metadata:
  name: reviewer
spec:
  allow:
    review_requests:
      roles:
        - access
      preview_as_roles:
        - access
```

Without changing those roles, we can assign both requester and reviewer roles in a specific scope with access list:

```yaml
kind: access_list
metadata:
  name: access-to-default
spec:
  grants: 
    roles: [requester, reviewer]
  scopes:  ['/dev']
  members:
    - bob@example.com
    - alice@example.com
```

In this case, `bob@example.com` and `alice@example.com` will get an ability to search, request and review requests, but only in the scope of any resource in `/dev` resource group.

Customers frequently ask a question of how to scale this with multiple teams, 
with this approach, we’d have to create an access list for each individual team. 

Previously we’ve been recommending to use role templates. 
However, new access lists integration mirrors any group hierarchy in identity providers, 
so there is no need to use templates - Teleport will create access lists and keep members up to date.

The only thing we are missing is to let customers specify the scope when importing Okta groups or apps as access lists. 
For example, access list for Okta group `devs` can automatically have scope `/dev`

Additionally, one access list can be a member of another access list. Let’s review a case when we have a group devs that needs access to both staging and production.

Let’s create access list for Alice and Bob, this special access list does no grants and scopes, we are going to use it to keep list of our developers:

```yaml
kind: access_list
metadata:
  name: dev-team
spec:
  members:
    - bob@example.com
    - alice@example.com
---
kind: access_list
metadata:
  name: access-to-prod
spec:
  grants: 
    roles: [requester, reviewer]
  scopes:  ['/dev']
  member_lists:
    - dev-team
---
kind: access_list
metadata:
  name: access-to-stage
spec:
  grants: 
    roles: [requester, reviewer]
  scopes:  ['/prod']
  member_lists:
    - dev-team
```

**Note:** We have to make sure that the child access list is a strict subset of the roles, traits and scopes of it's parent or has no scopes or grants at all.

### Scoped Impersonation

To make sure Alice and Ketanji can impersonate Jenkins, but only when accessing dev infrastructure, we will grant impersonator role via access list with scope `/dev`

```yaml
kind: role
version: v5
metadata:
  name: impersonator
spec:
  allow:
    impersonate:
      users: ['jenkins']
      roles: ['jenkins']
---
kind: access_list
metadata:
  name: access-to-dev
spec:
  grants: 
    roles: [impersonator]
  scopes:  ['/dev']
  members:
    - alice@example.com
    - ketanji@example.com
```

### Scoped admins

Large organizations would like to grant some users admin rights scoped for part of the infrastructure. 
Scoped admins can manage access to users and resources within the scopes of their access lists and resource groups.

Let’s review how we can create a scoped admin structure and even let delegated admins create new roles in the scopes of their environments.
Scoped admins can create new roles, and access lists, however only if those roles have `grantable_scopes` and access lists have `scope` equal or more specific 
than the scope of the granted roles of admins.

Let’s say Alice would like to delegate admin rights for the lab environment to Ketanji and Bob:

```yaml
kind: access_list
metadata:
  name: scoped-dev-admin-dev
spec:
  scopes: ['/dev/lab']
  grants: 
  roles: [editor, auditor]
  members:
    - bob@example.com
    - ketanji@example.com
```

Ketanji can now create roles, access lists and join tokens, but they all will have scopes set to /dev/lab

```yaml
kind: role
version: v5
metadata:
  name: impersonator
spec:
  grantable_scope: '/dev/lab'
  parent_resource_group: '/dev/lab' 
  allow:
    impersonate:
      users: ['jenkins']
      roles: ['jenkins']
---
kind: access_list
metadata:
  name: scoped-dev-admin-dev-access
spec:
  parent_resource_group: '/dev/lab'
  scopes: ['/dev/lab']
  grants: 
  roles: [access]
  members:
    - bob@example.com
    - ketanji@example.com
---
# token.yaml
kind: token
version: v2
metadata:
  name: my-token-name
spec:
  parent_resource_group: '/dev/lab'
  roles: 
    - Node
    - App
```

By propagating scope for any resource created by a user leveraging the roles the granted scope, 
we  make sure that our security invariant remains - Ketanji and Bob can’t expand the scope of their cluster access beyond /dev/lab.

### Allow agent forwarding for some hosts

Issue https://github.com/gravitational/teleport/issues/23790 is looking for a way to allow agent forwarding for specific hosts.

This can be done with access lists and scoped assignment:

```yaml
kind: access_list
metadata:
  name: access-to-default
spec:
  grants: 
    roles: [access-with-agent-forward]
  scopes:  ['/dev/lab']
  members:
    - bob@example.com
```

This will let bob to ssh with agent forwarding on into hosts in scope of /dev/lab only.

## Implementation

Teleport will stop issuing new certificate extensions for access granted by Access Lists or Access Requests. 

Instead, Teleport will create and distribute internal `Grant` resources. These resources will be used by Teleport only and will never be exposed to users.

Here is a high-level structure of an internal `grant` resource:

```yaml
kind: grant
version: v1
metadata:
  # name is a unique random identifier uuid-v4 of this grant 
  name: uuid
spec:
  # the time when the grant expires in RFC3339 format
  create_time: "2023-02-22T21:04:26.312862Z"
  # the time when the grant has been updated in RFC3339 format
  update_time: "2023-02-22T21:04:26.312862Z"
  # the time when the grant expires in RFC3339 format
  expires: "2021-08-14T22:27:00Z"
  # roles that are granted
  roles: [admin, editor]
  # traits that are granted
  traits:
    - 'internal.logins': 'root'
  identity: 'alice@example.com'
  scopes: ['/dev/lab']
  # not all grants need to be always evaluated,
  # when set, this grant will be evaluated only when users specify access request,
  # and this grant is issued as a part of access request
  access_request_id: 'bc8ca931-fec9-4b15-9a6f-20c13c5641a9'
  # resource_uuids is used for search based access requests and further limits access to specific resources,
  # if those are allowed by granted roles in the given scopes.
  resource_uuids:
  - 'e99dcc3b-11ba-4b87-96c5-c3f1e886e9ec'
```

We will also assume that resource groups, roles `V8` and Access Lists `V2` all have `create_time` and `update_time` fields.

Teleport will store grants reflecting the scope hierarchy, for example, if there are two grants, one for `alice@example.com` 
at scope `/dev/lab` and another for `bob@example.com` for scope `/grants/dev`, we will store them in a tree:

```
# the _root node only contains the empty grant to capture last update time.
/grants/dev/_root
/grants/dev/_members/bob@example.com: grant-2-uuid

/grants/dev/lab/root
/grants/dev/lab/_members/alice@example.com: grant-1-uuid
```

Every time we add, or update the grant in the hierarchy, we will also update it's `_root` with the timestamp.

We will also store `ResourceGroup` as a hierarchy:

```
/resource_groups/dev/_root
/resource_groups/dev/_members/luna

/resource_groups/dev/lab/_root
/resource_groups/dev/lab/_members/mars
```

Every time we add or remove a resource, we update the `_root` of the hierarchy with the timestamp.

When stored this way, each part of Teleport can subscribe and fetch grants for resource groups relevant to it, and would know when the grant hierarchy or resource hierarchy was last updated.

### Grants and access requests

When access request is approved, Teleport will create a grant with `access_request_id` set to the approved access request id:

```yaml
kind: grant
version: v1
metadata:
  # name is a unique random identifier uuid-v4 of this grant 
  name: bc8ca931-fec9-4b13-9a6f-20c13c5835a7
spec:
  # this grant is issued as a part of access request
  access_request_id: 'bc8ca931-fec9-4b15-9a6f-20c13c5641a9'
  # resource_uuids is used for search based access requests and further limits access to specific resources,
  # if those are allowed by granted roles in the given scopes.
  resource_uuids:
  - 'e99dcc3b-11ba-4b87-96c5-c3f1e886e9ec'
```

Teleport will only evaluate those grants when user's certificate has `access-request-id` in the certificate extension.

Instead of `assumed-role` and `resource-uuids` extensions issued in certificates and described in [RFD 0059 Search Based Access Requests](https://github.com/gravitational/teleport/blob/master/rfd/0059-search-based-access-requests.md#certificate-issuance-and-rbac), Teleport will issue a certificate with new extension `access-request-id: [bc8ca931-fec9-4b15-9a6f-20c13c5641a9]`
that will evaluate grants assigned to the identity at the matching scope and access request ids.

### The "New enemy" problem

The Zanzibar paper describes two cases of "new enemy problem" (https://research.google/pubs/zanzibar-googles-consistent-global-authorization-system/):

**Case 1 - Neglecting update order**

* Step 1. Alice removes Bob from the ACL of a folder.
* Step 2. Alice asks Charlie to add new docs to a folder.
* Step 3. Bob should not see new documents, but can do so, if the ACL neglects the ordering. (Because in Zanzibar, individual documents may inherit ACL of the parent folder).

**Case 2 - Misapplying old ACL to new content**

* Step 1. Alice removes Bob from the ACL of the document.
* Step 2. Alice asks Charlie to add new contents to the document.
* Step 3. Bob should not be able to see new content, but may do so, if ACL evaluates Bob's permissions at Step 2 before reading Step 1.

Let's review how those cases can apply to Teleport's Grants.

Bob was granted role `access` in scope `/dev/lab`.

**Case 1 - Neglecting update order** 

* Step 1. Alice removes Bob from the access list granting `access` in scope of `/dev/lab`
* Step 2. Alice adds new resource `luna` to the scope of `/dev/lab`
* Step 3. Bob should not get access to `luna`, but can do so, if the Teleport Proxy evaluating access skips `Step 1` before evaluating `Step 2`

Let's assume that every time a resource is added or removed to a resource group, it has it's timestamp updated.

* Before any of the step, we can assume that grant hierarchy `G` - `/dev/lab` has some timestamp `Tg` and a resource hierarchy - `R` has some timestamp `Tr`.
* At a `Step 1`, at the timestamp `T1` Alice removed Bob from access list granting role `access` in the scope `/dev/lab`. Teleport has updated the `_root` nodes of grant hierarchy `/dev` and `/dev/lab` with this timestamp in the backend.
* At a `Step 2`, at the timestamp `T2` > `T1`, Alice added a new resource `luna` to the scope of the `/dev/lab`. Teleport has updated the `_root` node of the resource group hierarchy `/dev` and `/dev/lab` with this timestamp in the backend.
* At a `Step 3`, Teleport proxy would have to evaluate access to host luna. The proxy would have to make sure that it's copy of the grant hierarchies `/dev/lab` is not stale.

In our example `T2 > T1 > Tr` and `T2 > T1 > Tg`. There are two sub cases when `Tr > Tg` and `Tr < Tg`

Here are possible combinations of versions that The Proxy can encounter:

```
1. G - Tg, R - Tr
2. G - T1, R - Tr
3. G - Tg, R - T2
4. G - T1, R - T2
```

Let's review the cases in the reverse order:

* In the cases 3 and 4, the Proxy sees that the resource group has been updated after the grant, because `T2 > T1 > Tg` and can proceed.
* In the case 2, The proxy sees that `R` may be stale, because it's `Tr` less than `T1 and has to make a call to the backend making sure it fetched update of `R` least as fresh as `T1` with membership of `luna`.
* In case 1, if `Tr > Tg`, the proxy can proceed, otherwise if `Tr < Tg`, the proxy has to make sure it has fetched the latest update of `Tr` at least as fresh as `Tg` with membership of `luna`.

How would Teleport "make sure that it has fetched a version of a resource at least as fresh as a timestamp?". 

Some backends make it easy, like Etcd giving a generation version for every resource, making sure you are not looking at the stale snapshot. 

Backends like DynamoDB are making it very hard or impossible, the only way with those backends is for the client to keep track when was the last time if has last fetched the resource `R` in cases 1 and 2, and update this value for `R` in it's in memory-cache.

We also are making assumption that time is in sync between all clients. In real-world, it's more complicated, because we have to consider the times when roles have been updated and fetched, but the algorithm will remain the same.

TODO: question for a scale team on implementation details for backends.

### Migration

We will release a new versions of resources in multiple phases:

* `RoleV8` will introduce `parent_resource_group` and `grantable_scope`, `create_time`, `update_time`. `RoleV9` will remove `node_labels`,  `node_labels_expression`. Users can gradually migrate to new versions, first by leveraging scopes and resoruce groups, and then removing labels matching. For new `RoleV8` roles by default, if the `grantable_scope` is missing, we assume empty scope - that will prevent the role from being granted on any scopes. When migrating existing roles from `V7`, we would set `/` - root scope to avoid breaking the cluster. When migrating preset roles, we will replace `app_labels` with verb `access` when possible. See "The access verb" section for details.
* Access List `V2` will also introduce `create_time` and `update_time`,  `scope` and `member_lists`.
* We will craete a new resrouce `ResourceGroup` referenced in this RFD `V1` will have `create_time` and `update_time`.
* The connector resources `saml` and `oidc` will loose `attributes_to_roles` and `claims_to_roles` respectively in their `V3`.
* We will deprecate login rules in the first major release of 2025.
* We will stop encoding roles and traits in certificatees in the first major release of 2025.

## Scalability

We have to make sure that resource group assignments scale with large-scale clusters. 

Also, each computing resource would only need to pull roles for its scope and above. With a properly built infrastructure hierarchy, this will significantly reduce the amount of roles that have to be distributed and evaluated for each computing resource.

## UX

We can start displaying resource groups as a special type of label in the existing UI, and represent it as a label: teleport.dev/resource_group:/env/prod/lab. 

Teleport Discover should integrate with scopes by importing AWS accounts, GCP, Azure resource groups and resources within Teleport’s Resource groups.

## Security

* Access Lists grants will not result in roles and traits encoded in certificates. Instead, grants will be evaluated at each point of access.
* Creating or modifying resource groups and Access Lists is a potentially disruptive action, and we will protect it with MFA check similarly to other administrative actions. See [Admin Actions RFD 131](https://github.com/gravitational/teleport/blob/master/rfd/0131-adminitrative-actions-mfa.md) for details.


