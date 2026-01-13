---
authors: Forrest Marshall (forrest@goteleport.com)
state: draft
---

# RFD 0229 - Scopes

## Required approvers

- Engineering: @klizhentas && @rosstimothy

## What

A system for hierarchical organization and isolation for resources and permissions.


## Why

Historically, teleport's resource and permission organization systems have been very "flat". In particular,
administrative and resource-provisioning permissions tend to be "all or nothing". Resource labels and selectors
allow organization of resources/permissions, but said organization can only be performed by what are effectively
global admins. Any user with the ability to create node join tokens can join any node to the cluster
with any labels they like. Similarly, a user with role creation permissions can create a role that grants access
to anything. This poses a significant challenge when trying to delegate any meaningful responsibility and/or to
apply the principal of least privilege to users that need to administer resources/permissions in an meaningful way.

In addition to administrative privileges being global, individual controls within teleport tend to be global in
nature (or at least, global in how they apply to a given user). For example, a user's `client_idle_timeout`
is always determined by the most restrictive value across *all* of their roles, making it difficult to reasonably
model user privileges with a mix of more and less restrictive controls depending on context.

Similarly, teleport user credentials tend to be all or nothing. There isn't a good way to get credentials that
are only useable for the specific task at hand. Instead, if a user is logged into a teleport cluster they always
have all their available permissions applied. This increases blast radius, both in terms of compromise and accidental
misuse.

We would like to provide a mechanism for organizing resources and permissions in a manner that allows for both
isolation and hierarchy. This system should support admins that have powerful control over provisioning of, and
access to, resources within their domain of influence. Including the ability to create a mix of more and less
permissive environments/contexts. Said admin privileges must not be able to affect resources
and permissions outside the scope of their domain(s). We would also like to provide means of limiting the blast
radius of compromise/misuse as part of this organizational system. Finally, we require that this organizational
system be backwards compatible with existing teleport resources, permissions, and usage patterns to the greatest
extent possible.


## Intro

### Overview

We will introduce the concept of a "scope" as a new means of organizing resources and permissions. The scope of
a resource or permission will be a simple attribute formatted as a path-like string (e.g. `/staging/west`, `/prod/east`,
etc).

Permissions that are scoped will apply only to resources of the same scope, or a descendant scope. For example, having
permission to ssh into nodes assigned at scope `/staging` will permit ssh access for nodes that have a scope of `/staging`
or `/staging/west`, but not `/prod` or `/prod/west`.

Scoping will apply to administrative privileges as well. A user with the the permission to join teleport nodes assigned
at `/staging` will *only* be able to join nodes with that scope or a descendant scope. Same goes for role creation/assignment,
with users effectively being able to be granted admin-like powers "within" a scope but not outside of it.

In order to improve useability and reduce blast radius of compromise/misuse, we will also introduce the concept of scope
"pinning". Rather than logging into the teleport cluster as a whole, users will be able to login to a specific scope. This
will result in the credentials granted to the user *only* being usable for the target scope and its descendants. For example,
if a user has permissions at `/prod` and `/staging`, and logs in to `/staging`, they will only be able to see and interact
with `/staging` scoped resources.

We will be targeting a basic user experience that looks something like this:

```shell
$ tsh login --scope=/staging/east
# authentication flow

$ tsh ls
Node Name      Address Labels
-------------- ------- ------
some-node-east ...     ...

$ tsh ssh some-node-east
# success

$ tsh login --scope=/staging/west
# authentication flow

$ tsh ls
Node Name      Address Labels
-------------- ------- ------
some-node-west ...     ...

$ tsh ssh some-node-east
ERROR: not found

$ tsh ssh some-node-west
# success

$ tsh login --scope=/staging
# authentication flow

$ tsh ls
Node Name        Address Labels
---------------- ------- ------
some-node-east   ...     ...
some-node-west   ...     ...
```

Note that the nature of commands after login are unchanged. Ordinary (non-admin) users should be able to ignore the concept
of scoping once they have logged in to the appropriate scope. Scripts that work today with an already logged-in `tsh` should
continue to function as expected with scoped credentials, with the only change being that the resources affected by the operations
are now limited to the subset assigned to the pinned scope.

Scoping will be a large and complex feature to implement, as it will meaningfully change most access-control
checks and APIs in teleport. In order to make this transition more manageable, we will be gradually implementing scoped
features over time, with the initial scopes MVP only providing very basic scoped joining, role management, and ssh access.
The intent will be that users will be able to start adopting a mixed use style as scope features become sufficiently robust
to start addressing their specific usecases.


### Scoping Semantics

A scope is a simple path-like string (e.g. `/staging/west`). A resource will be said to be "scoped" if it has a `scope` attribute
that obeys this format. Likewise, a permission will be scoped if it is granted by a configuration resource with a
`scope` attribute.

Scopes are hierarchical. The scope `/staging` is a "parent" of `/staging/west`, and `/staging/west` is a "child" of `/staging`.
Permissions granted at a parent scope apply to all child scopes.

Scope hierarchy is segment based, not prefix based. I.e. `/staging` is a parent of `/staging/west`, but not of `/stagingwest`.

Scopes are attributes, not objects. There is no need to create a scope object before creating a resource with a given
scope attribute. I.e. if no resource exists with scope `/staging/west`, a node can still be created with that scope without
first performing any kind of scope setup/creation/configuration ceremony.


### Core Design Goals

**Hierarchical Isolation**: Permissions within a given scope cannot be used to "reach up" and affect resources/access defined
in parent scopes or "reach across" and affect resources/access defined in orthogonal scopes.

**Mixed Permissiveness**: Scopes will allow for a mix of more and less permissive controls to be applied to the same user
depending on context.

**Blast Radius Reduction**: Scoping will be a robust tool for further reducing the blast radius of compromised or misused
credentials.

**Delegated/Limited Administration**: Scoping will unlock the ability to create "scoped admins" with powerful control over
resources and permissions within their scopes, without being able to affect resources/permissions outside of their scope.

**Minimal Effect on User Experience**: After selecting the scope to login to, the user experience for normal (non-administrative)
tasks will be unchanged.

**Backwards Compatibility**: Scoping will not change the function or meaning of existing teleport resources/permissions.

**Gradual Rollout and Adoption**: Scoping features will be rolled out gradually and mixed use of scoped and unscoped
resources/permissions will be supported.


### Comparison to Other Systems

Scoping has a lot in common with systems like the AWS organization/account model, GCP's folder/project model, and Azure's
own RBAC scopes. Scopes differ in a few key ways:

- Scopes are arbitrarily hierarchical. Resources can be assigned at any level of the hierarchy. Other similar systems tend
  to have a hierarchy where all resources live in the leaf nodes, and often the depth of the hierarchy is fixed.

- Scopes are an attribute, not an object. There is no additional creation step like there would be with a system that organizes
  resources into accounts/projects/etc. Other systems tend to require the hierarchy be defined separately as a standalone entity
  prior to resources being created.

- Credentials can be pinned to arbitrarily narrow scopes. A user can opt to access a resource in `/staging/west` by logging
  into `/staging` *or* logging into `/staging/west`. This gives granular control over the blast radius of credentials, and
  ensures that hierarchies and practices can evolve and refine over time without needing to be fully rebuilt. By virtue of
  the fixed leaf nodes of other systems, options for constraint of credentials via the organizing hierarchy tend to be more
  limited.


## Details

### Scoping of Resources

Within teleport's existing resource format, `scope` will be a new top-level field of type `string`.  Ex:

```yaml
kind: example_resource
metadata:
  name: example
scope: /staging/west # this resource is "scoped" to /staging/west
spec:
  # ...
version: v1
```

For inventory/presence resources like ssh servers, kubernetes clusters, etc, we can add the `scope` field to the existing
resource type without much fear of compatibility issues.  Existing resources that do not have a `scope` field can be thought
of as being assigned to the "root" of the scope hierarchy (this is mostly a philosophical point, as we do not plan to support
permission assignment at root for the time being), and will not be accessible via scoped permissions.

For resources that grant permissions (e.g. roles), the story is more complicated. Assignment of scoped permissions doesn't mesh
well with existing permission-granting resources.  For these types, we will need to have new special scoped variants with
separate APIs. For example, we will be introducing a new `scoped_role` type which will implement a compatible subset of the
functionality of the `role` type and serve the same purpose when using scoping to organize permissions.

This bimodal approach to resource scoping is inkeeping with our user-facing design goal. The resources/apis that ordinary
(non-admin) users interact with will remain unchanged except for the addition of the scope field (which said users can safely
ignore). Administrative APIs/types will receive scope-specific variants that will be tailored to the needs of scoped
administration.

There are downsides to the bimodal approach. Namely, incurring the introduction of many new types and APIs. However, we
judge this to be more feasible than trying to retrofit existing permission-granting resources to support scoping. This is
for a number of reasons:

- High degree of feature incompatibility: many unscoped permission-granting types/APIs have features that do not make sense in
  a scoped context. For example, the `role` type supports options for determining certificate lifetime and extensions, which
  cannot be mapped sanely to a model with scope isolation. Similarly, the `access_list` type supports features like global
  trait modification.
- Infeasibility of gradual rollout: retrofitting existing permission-granting types to support scoping would require a
  comprehensive overhaul of everything that touches said types. This would be a massive undertaking that would be difficult
  to manage, and present an extremely high risk of regressions. It would also significantly front-load the complexity and
  effort associated with scoping, well beyond what is acceptable to meet our timelines and goals.
- Lack of "fail closed" behavior: By treating scoped permissions as a separate system, teleport components not updated to
  properly understand the rules of scoping will simply ignore said permissions, essentially "failing closed" without
  any special work needing to be done. Retrofitting of existing permission-granting types poses a very high risk of failure
  modes where scoped permissions are inadvertently granted inappropriately.
- Lack of isolation: Scoping is a new feature that we anticipate having to iterate heavily upon over the coming months and
  years. While adding a scope attribute to an access resource is fairly straightforward, scoping fundamentally alters
  access-control logic in significant ways. Failing to isolate the scoped permission system codebase from the rest of
  teleport functionality would incur a significant burden in terms of maintainability and coordination of changes.

### Scope Pinning

Scope "pinning" is the term we will use for the process of logging into and being granted credentials for a specific scope.
A user certificate issued in this way will contain a `ScopePin` field which will constrain all subsequent actions by the
user to the pinned scope and its descendants.  A certificate of this kind will be said to be "pinned" to the given scope.

Users will be able to pin to a scope by passing the `--scope` flag to `tsh login`.  If no scope is specified, the user will be
logged in normally, with whatever unscoped privileges they have.

In order to ensure that users are able to make effective use of their scoped privileges, users will be able to list the scopes
for which they have assigned permissions:

```shell
$ tsh scopes ls
/staging/west
/staging/east
/prod/west
/prod/east

$ tsh scopes ls --verbose
Scope                 Roles
--------------------- -----------------------
/staging/west         access, auditor, editor
/staging/east         access, auditor, editor
/prod/west            access
/prod/east            access

$ tsh login --scope=/staging/west
# authentication flow
```

For ease of use, we will also allow scope to be set via environment variable. Ex:

```shell 
$ export TELEPORT_SCOPE=/staging/west
$ tsh login
# authentication flow
```

Within teleport's internals, the scope pin will serve as an additional constraint on all access-control checks. We will require
that any attempted resource access first be against a resource that is scoped to the pinned scope or a descendant scope. If this
check does not pass, access is denied without the need to load or evaluate any other permissions.


### A Scoped Access Check

In order to conform to our core isolation and hierarchy goals, and to support mixed permissiveness across scopes, the form of
scoped access checking will need to diverge from classic teleport access checks.  Ordinarily, in teleport code, an access check
is a one-off decision based on the user's full role set, the pseudocode of which would look something like this:

```go
roles := LoadRolesForUser(cert)

if CanAccessResource(roles, resource) {
    parameters := GetAccessConstraints(roles, resource)
    return Permit(parameters)
} else {
    return Denial
}
```

Note that allow decisions are not binary. There are often parameters that affect the nature of allowed access. (e.g. allowed
ssh access may come with or without X11 forwarding enabled). These parameters tend to be determined by the most restrictive
value across all roles. Though certain teleport controls actually use the opposite approach, instead taking the least restrictive
value across all roles. Neither approach works well when a user needs to access resources in different environments with different
access control requirements.

Per our scoping design goals, one of our key criteria is that administrative permissions assigned by a child scope cannot
be used to affect the nature of resources/permissions assigned by a parent scope. This means that we cannot allow permissions in
a child scope to modify the parameters of access that is permitted at a parent scope. Additionally, we want to allow for
specialization of controls s.t. different access parameters can be applied based on the specific nature of the access
being attempted. At a very high level, we will be aiming for an access check flow that looks like this:


```go
if !PinScopeAllowsAccessToResourceScope(cert.ScopePin, resource.GetScope()) {
    return Denial
}

for role := range RolesApplicableToResourceScope(cert, resource.GetScope()) {
    if CanAccessResource(role, resource) {
        parameters := GetAccessConstraints(role, resource)
        return Permit(parameters)
    }
}

return Denial
```

The key points of the model are these:

- When a role is evaluated and permits access, it "wins" and determines all access parameters (except where overridden
by external configuration). No other roles affect the access decision once a role has permitted access. So, for example,
if a role permits access *with* X11 forwarding enabled, no other role that denies X11 forwarding can override that decision.

- An access check will never consider roles that are assigned/applicable at an orthogonal or child scope of the resource
being accessed. For an access attempt against a resource at `/staging/west`, only roles assigned at `/`, `/staging`, or `/staging/west`
will be considered. Roles at `/prod`, `/staging/east`, or `/staging/west/testbed` will be ignored.

- Role assignments managed by higher level admins will always be evaluated before those managed by lower level admins (i.e. a role
assignment that is managed at `/staging` will be evaluated before one managed at `/staging/west`). This will ensure that hierarchical
isolation is preserved.

Note that in this model, the *order* in which roles are evaluated is of supreme importance. We are deliberately eliding ordering
details in the above as it is a complex topic that deserves its own dedicated discussion. The salient point for this section is
that ordering will preserve hierarchical isolation by evaluating the role assignments managed by higher level
admins before those managed by lower level admins. A more in-depth discussion of ordering can be found
in the [Access Control Evaluation Details](#access-control-evaluation-details) section later in this document.


### Scoped Roles and Assignments

A new `scoped_role` type will be introduced for the purpose of defining scoped permissions. This type will implement
a subset of the features of the existing `role` type, with features being ported over iteratively over time.

Classical teleport roles are assigned to users by directly editing the user resource's `roles` field. This centralized
approach is not inkeeping with the goal of delegated/limited administration.  Instead, scoped roles will be assigned
via a separate scoped assignment resource.  Admins of a given scope will be able to create scoped role assignments
for users independently, without the need to modify global state.

Inkeeping with the hierarchical isolation principle, scoped roles will only be assignable at the scope of the role
resource itself or a descendant scope.  For example, a `scoped_role` defined at `/staging` could be assigned to users
at `/staging` or `/staging/west`, but not at `/prod` or `/prod/west`. This ensures that role editing privileges in
one scope cannot be used to affect permissions in another scope.

As an additional layer of control, it will be possible to constrain the assignable scopes of a scoped role to
an explicitly defined subset. For example, a scoped role defined at `/staging` could be
configured to be assignable at `/staging/west` and `/staging/east`, but not at `/staging` or `/staging/central`
by specifying `assignable_scopes: ["/staging/west", "/staging/east"]` in the role spec. This has a few important
benefits. First, by allowing this extra layer of control, we allow scoped roles to be used in a manner similar
to a "role template". If two different scopes need similar roles, a parent scope can provide a common definition.
Second, separating the concerns of scoping role definition and assignment ensures that admins can accurately express
intent and prevent misuse.

A `scoped_role` resource for scoped admins might look like something like this:

```yaml
kind: scoped_role
metadata:
  name: staging-admin
  description: Basic administrative privileges for staging env admins
scope: /staging
spec:
  assignable_scopes:
    - /staging/west
    - /staging/east
  allow:
    rules:
    - kind: scoped_token
      verbs: [create, read, update, delete]
    - kind: scoped_role
      verbs: [create, read, update, delete]
    - kind: scoped_role_assignment
      verbs: [create, read, update, delete]
version: v1
```

Note that other than the `scope` and `spec.assignable_scopes` fields, this is identical to a standard teleport role
definition (though scoped roles will only support a subset).  Keeping this parity ensures ease of transition and
reduces cognitive load for users familiar with existing teleport concepts. It also saves us considerable effort since
most of the existing role evaluation logic can be reused and allows for automation of "lossy conversion" as a means of
bootstrapping a scoped configuration.

The scoped equivalent of a role's `options` block, where parameters like X11 forwarding and session recording mode
are defined, will need to support scope isolation rules just like the rest of the role. For example, if a user has
a role at `/staging` that permits ssh access with X11 forwarding enabled, and a role at `/prod` that permits ssh access
without enabling X11 forwarding, the role in `/staging` must not be able affect access s.t. X11 forwarding is enabled
when accessing `/prod` scoped resources. This may seem like an obvious point, but it bares calling out explicitly since
many of the `options` block parameters are currently global settings that determine certificate parameters at issuance
(`permit-X11-forwarding`, `permit-agent-forwarding`, etc). We will need to refactor relevant logic to determine these
values on a per-access basis rather than at certificate issuance time.

Scoped roles will also not support `deny` rules in the manner that classic roles do. The concept of a role-determined
`deny` rule is incompatible with certain key scoping features. Most notably, we intend to introduce scoped machine identities
where admins can create bots with custom role sets to take actions in their scopes. If deny rules worked like they do
in classic roles, this would result in admins in leaf scopes being able to "escape" deny rules assigned to them in parent
scopes via the creation of bots.

### Scoped Tokens and Joining

The scope of an agent will be determined by their join tokens. Scoped administrators will be able to create join tokens
assigned to scopes where they have `token:create` permissions. When an agent joins the teleport cluster using a scoped
join token, the agent will be automatically assigned that scope.

Creation of scoped join tokens will mirror the existing token creation API where possible. Ex:

```shell
$ tctl scoped token add --type=node --scope=/staging/west
```

The resulting token value will be usable in the same way an ordinary join token works today. Ex:

```shell
$ teleport start --token=some-token-value ...
```

No special scope-related parameters will need to be passed to the agent. Any agent-side configuration that works with unscoped
static tokens today will also work with scoped tokens.

The scope of an agent/resource isn't just about limiting the access that users have to the agent. Inkeeping with hierarchical
isolation principles, we also need to ensure that agents themselves cannot be used to "reach up" and affect access outside
of their scope (e.g. by maliciously advertising themselves with the wrong scope attribute on their heartbeat). This means
that scoping of an agent is also a security control *for the agent's own permissions*.

One deceptively tricky aspect of scoping is scoping of agents and their associated presence resources. Due to the often
ephemeral nature of teleport agents, teleport does not retain a long-term record of agent inventory/configuration. Any
security-controls applied to an agent (e.g. its system roles) are hardcoded into the agent's certificate at join time,
as determined by the details of the join token being used. This ensures that an agent's presence information TTL'ing
out does not allow it to escape security controls.

In the context of scoping, the implication of ephemeral presence is that the scope of an agent *must* be a statically assigned
control attached to the agent's certificate, rather than a dynamic value stored in the teleport backend. To support this,
we will be adding a new `AgentScope` field to host certificates. Support for this field will need to be implemented s.t. it cannot
be escaped by reissue/etc, and agent-facing APIs will need to be updated to support scope-aware permission checks as needed.
Primarily, this will mean limiting agent's ability to read any scoped configuration resources (e.g. scoped roles) to only the
subset that are relevant to the agent's function, and ensuring that presence information emitted by the agent always contains
the correct scope(s).


### Scoped SSH Access

The first teleport access protocol we will be working on adding scoping support to will be ssh access. This is
the best starting point for a number of reasons.  First, it is the most widely used teleport access protocol. Second, ssh access
has a one-to-one mapping between agent and target resource, meaning it has the simplest scoped security model (the scope of the
agent *is* the scope of the resource). Finally, the ssh access codebase has already been refactored to use the new PDP-style
access-control decision format, which simplifies refactoring.

At its most basic, creating a scoped ssh server will look something like this:

```shell
$ tsh login --user=admin@example.com --scope=/staging/west
# authentication flow

$ join_token="$(tctl scoped token add --type=node)"

$ teleport start --token=$join_token --hostname=node.example.com ...
```

With access looking like:

```shell
$ tsh login --user=some-user@example.com --scope=/staging/west
# authentication flow

$ tsh ls
Node Name        Address Labels
---------------- ------- ------
node.example.com ...     ...

$ tsh ssh node.example.com
```

In essence, it will be unchanged from ordinary ssh access except that users and admins specify the scope they wish to login to,
and admins create scoped join tokens rather than ordinary join tokens.

SSH access as a whole is a complex feature with many related controls/permissions (X11 forwarding, enhanced recording, etc).
In the interest of gradual rollout and adoption, we will be targeting a minimum viable subset of scoped ssh access controls
to port over to scoped roles initially, with more making their way over time. The goal will be to create a robust and secure
starting point from which we can iterate based on feedback and real-world demand.

As part of ssh access work, we will be updating routing to be scope aware. Many users have setups where multiple teleport
nodes across different environments end up advertising the same hostname(s). This ambiguity means that sometimes
`tsh ssh <user>@<hostname>` is ambiguous. We currently allow cluster administrators to configure teleport into one of two
routing modes. By default, we reject ambiguous dial attempts and force the caller to dial by UUID.  Admins can opt into
route-to-most-recent, where the dial always hits the agent that heartbeat most recently.  With scoping, we now have a very
powerful tool for disambiguating dials. Dial attempts will now take into account the user's pinned scope. This means that,
for example, attempts to dial `myhost.example.com` in `/staging/west` will not be made ambiguous by the existence of
`myhost.example.com` in `/prod/east`.


### Scoped Access Lists

Access lists are the first of the higher level access management tools we will be brining to scopes. At its core, an access
list is a list of users and a set of privileges to be assigned to all users in the list. Classic teleport access lists
are capable of some fairly advanced features, including modifying user traits and delegating responsibility for management
of the list to owners who otherwise do not have access list modification/creation permissions.

The MVP for a scoped access list will err on the side of simplicity, only supporting a member list and a list of scoped
roles to be assigned to all members. This simplification is in keeping with our gradual rollout and adoption goal, as it
allows us to deliver useful features faster and iterate, but it is also reflective of the fact that scoped access lists
inherit a large amount of utility from the scoping system itself. For example, the ownership feature of classic access
lists, while still a useful abstraction in scoped access lists, is less critical since scoping provides us scoped admins
which serve an very similar purpose.

Scoped access list/member resources will look something like this:

```yaml
kind: scoped_access_list
metadata:
  name: 318ea8be-129c-41f4-ad95-fd830e14e3e7
scope: /staging
spec:
  title: "west staging access"
  grants:
    scoped_roles:
      - role: access
        scope: /staging/west
      - role: access
        scope: /staging/east
version: v1
---
kind: scoped_access_list_member
metadata:
  name: a3e64073-8980-41aa-a7a0-dd23de40c38e
scope: /staging
spec:
    access_list: 318ea8be-129c-41f4-ad95-fd830e14e3e7
    name: alice@example.com
    membership_kind: user
version: v1
---
kind: scoped_access_list_member
metadata:
  name: c4c9d4b3-c57c-40de-a15e-e2425267a716
scope: /staging
spec:
  access_list: 318ea8be-129c-41f4-ad95-fd830e14e3e7
  name: bob@example.com
  membership_kind: user
version: v1
```

Per scope isolation rules, scoped access lists will only be able to grant roles at their own scope or descendant scopes, and
only be able to assign roles that are defined in their own scope or ancestor scopes.  This ensures that role authors cannot
use an access list to "reach up" and affect permissions outside of their scope, and that access lists themselves cannot be
used to do the same. Access list member resources will always be bound to the scope of the access list itself.

Note that while most of the complexity of access lists is being deferred to future work, the membership kind field is present
from the beginning. This is because the first more advanced access list feature we want to port over is nested access lists (i.e.
lists that include other lists as members). This is a powerful feature that is widely used in classic access lists and is
a hard prerequisite for implementing some identity provider group sync features which we know are in demand for scoping.

In addition to eliding some features, scoped access lists will also have another key difference from classic access lists. Classic
access lists apply the permissions/traits they grant by functioning as a "login hook". A user's granted permissions are
recalculated inline on each login. Scoped access lists will instead asynchronously apply their permissions by generating and
managing regular scoped role assignment resources. This will allow us to fully decouple scoped access lists from the login process,
improving performance and maintainability. It also means that users will be able to see their granted permissions appear in
near realtime in `tsh scopes ls` (though they will still need to relogin to acquire newly granted permissions). Note that because
user traits are not necessarily knowable asynchronously, this means that trait-based membership conditions will need to be
propagated to the assignment itself in order to ensure that assignments can be invalidated during login time.


### Introspection

In parallel with the rollout of the above features, we will also begin working on tooling to better support introspection
of the hierarchy of scoped resources and permissions. This will likely be a long-term effort as we learn more about what
users need to effectively manage scoped resources and permissions at scale. The first two items in this effort will be
the `tsh scopes ls` command for use by users to discover scopes where they have permissions (discussed above),
 and a `tctl scopes status` command that will provide a status readout on scoped resources and permissions across the cluster
(or within the currently pinned scope). The intent of `tctl scopes status` will be to allow administrators to quickly get
an overview of the state of scoped resources and permissions without getting bogged down in details. We will be targeting
a UX something like this:

```shell
$ tctl scopes status
Scope         Roles Lists Assignments Agents Resources
------------- ----- ----- ----------- ------ ---------
/staging      5     0     10          0      0
/staging/east 2     1     4           14     16
/staging/west 3     2     6           7      9
```

The intent of this display will be to allow admins to quickly get a "bird's eye view" of the state of scope usage,
and to empower them to make meaningful follow-up queries as appropriate (e.g. `tctl get scoped_role --scope=/staging/west`).
The API underlying this command will need to be heavily rate-limited and eventually incorporate caching of some form
as resource counts are expensive to compute, but having this kind of view will be critical to effective long-term use
of scoping in medium and large scale teleport deployments.

For compatibility's sake, the scope status API will also need to support tiered granularity depending on the permissions
of the caller. We don't want to render it useless when a new resource type is added that the caller doesn't have permission
to view. The API and UI will therefore need to support the concept of individual columns being inaccessible due to lack of
permissions.

Long term, we may want to introduce some additional visualization tools to improve comprehension of scope hierarchy. Given
the format of scopes as file-like paths, a tree-like visualization would likely be a good fit.


## Access Control Evaluation Details

The manner in which access controls are determined for scoped identities differs meaningfully from classic teleport
role evaluation. The most obvious difference is scope hierarchy itself.  However, there are other more subtle differences
that are important to understand. Notably:

- Scoped role evaluation considers only a single role when determining the parameters/controls to enforce for a given
access. This means that scoped roles do not have cross-role side-effects (see the preceding [A Scoped Access Check](#a-scoped-access-check)
discussion for details).

- *Order* of scoped role evaluation matters. Because we halt on the first allow decision and pull all parameters from
  the allowing role, the order in which roles are evaluated becomes critical.

- Scoped roles cannot be used as a source of truth for controls which must be enforced globally.

- Scoped roles cannot express controls that must be enforced in contexts where the target resource/scope is not known.


### Scope of Origin and Effect

In order for scoped role evaluation order to fully make sense, we first need to step back a bit and think about how scoping
intersects with access control in a philosophical sense.

When a scoped administrator authors and applies a policy, two different scopes are at play. The first is the scope of the
resource that is the policy definition. This is the top level `scope` resource field, and is common to all scoped types.
For a scoped type that effectuates a policy/permission, however, there is also a scope (or scopes) at which the granted
permissions are intended to apply. Consider this example of a scoped role assignment:

```yaml
kind: scoped_role_assignment
metadata:
  name: some-assignment
scope: /staging
spec:
  user: alice
  assignments:
    - role: access
      scope: /staging/west
version: v1
```

In the above example, the top-level `scope` field indicates the scope of the assignment resource itself. This scope determines
what admins are able to modify the assignment. The `spec.assignments.scope` field indicates the scope at which the assigned role's
permissions are intended to apply. Despite the assignment resource living at `/staging`, the assigned role's permissions only apply
when accessing resources at `/staging/west`. In other words, we can think of the permissions implied by the assignment as having
both a scope that they are applied *from* and a scope that they are applied *to*. This separation of concerns is critical as it
allows us to express the ownership/authority of the policy separately from the intended target of the policy.

Going forward, we will refer to these two different kinds of scope as the *Scope of Origin* and the *Scope of Effect*: 

- *Scope of Origin*: The scope of the resource that describes the policy to be applied by. This scope informs us of
the authority from which the policy originates. It is, in effect, a representation of the *provenance* of the policy. A higher
level scope of origin represents a policy that expresses the intent of higher level admins, and therefore carries
greater weight. In particular, it should not be possible for a policy with a higher/ancestral scope of origin
to be overridden or negated by a policy with a lower/descendant scope of origin.

- *Scope of Effect*: This is the scope at which the policy is intended to apply. This scope represents the intended target of
the policy. A policy with a narrower scope of effect is more specific, and a policy with a broader scope of effect is more general.
The scope of effect *must* be equivalent to or descendant from the scope of origin in order to ensure that policies cannot "reach up"
and affect resources/access outside of the authority of the admins that defined/assigned the policy. Scope of effect is otherwise
independent of scope of origin, and it may be appropriate in some circumstances for the most *specific* scope of effect to take
precedence, something that we cannot do with *Scope of Origin*, which must always respect hierarchical isolation.


### Role Evaluation Order

As discussed previously, scoped roles will disallow cross-role side effects. Instead of trying to "sum" controls/parameters, the nature of an
access attempt will be determined by the first role that permits the attempted access.  In order to ensure correct and flexible behavior, we must
therefore decide on a role evaluation order that respects scoping principles, supports flexibility and expressiveness, and is deterministic.

In order to achieve our role ordering goals, we will be using a three-tiered approach to ordering. A long-form example is discussed
below but, in short, the three determinants are (in order): scope of the originating assignment resource (*Scope of Origin*),
scope of the role's effective privilege assignment (*Scope of Effect*), and lexicographic order of the role name.

Consider the example of a user with the following role assignment state:

```
Scope of Origin Scope of Effect Role Name
--------------- --------------- -----------------
/staging        /staging        staging-auditor
/staging        /staging/west   staging-owner
/staging/west   /staging/west   staging-west-dev
/staging/west   /staging/west   staging-west-user
```

In plain words, the user has been assigned four roles:
- The `staging-auditor` role is assigned by an admin from `/staging` and applies when accessing resources across all of `/staging`.
- The `staging-owner` role is assigned by an admin from `/staging` and applies when accessing resources at `/staging/west`.
- The `staging-west-dev` role is assigned by an admin from `/staging/west` and applies when accessing resources at `/staging/west`.
- The `staging-west-user` role is assigned by an admin from `/staging/west` and applies when accessing resources at `/staging/west`.

The question now is, what order should these roles be evaluated in (i.e. if multiple roles allow a given action, which should take precedence)?

The first thing we do is order by *Scope of Origin*. The two roles assigned from `/staging` must be evaluated before the two roles assigned
from `/staging/west`. This is because the former represent the intent of higher level admins, and so must take precedence
in order to preserve hierarchical isolation. So we know that `staging-auditor` and `staging-owner` must be evaluated before `staging-west-dev`
and `staging-west-user`.

Ordering by *Scope of Origin* does not fully resolve ambiguity, as we still need to determine the relative order of roles assigned from the
same *Scope of Origin*. To resolve this, we will use the *Scope of Effect* as a secondary ordering determinant. Here, we will use specificity
preference. If two role assignments originate from the same scope, the one with the more specific scope of effect will be evaluated first.
This allows admins to express specialized intent at narrower scopes without violating hierarchical isolation principles. With this rule in place,
we can determine that `staging-owner` must be evaluated before `staging-auditor`, since both originate from `/staging`, but `staging-owner` has a
more specific *Scope of Effect*.

Finally, in the cases where both the *Scope of Origin* and *Scope of Effect* are the same for two roles, we will fall back to lexicographic
order of the role name. This is somewhat arbitrary, but it is simple, deterministic, and easy to understand. With this last principle in place,
we can conclusively determine the order of role evaluation in all cases. In our above example, the final order of role evaluation will be:

1. `staging-owner`, due to having the highest *Scope of Origin* and most specific *Scope of Effect*.
2. `staging-auditor`, due to having a higher *Scope of Origin* than the remaining roles.
3. `staging-west-dev`, due to having a lexicographic precedence over `staging-west-user`.
4. `staging-west-user`, due to being the last remaining role.

We see that with the above ordering, the intent of higher level admins is preserved, while still allowing for a given admin to
specialize the policies they author at narrower scopes. Admins of `/staging` can confidently write policies with a *most specific rule wins*
philosophy, without worrying about the policies written by admins of `/staging/west` interfering with their intent.

Considered another way, recall the example pseudocode from the [A Scoped Access Check](#a-scoped-access-check) section:

```go
if !PinScopeAllowsAccessToResourceScope(cert.ScopePin, resource.GetScope()) {
    return Denial
}

for role := range RolesApplicableToResourceScope(cert, resource.GetScope()) {
    if CanAccessResource(role, resource) {
        parameters := GetAccessConstraints(role, resource)
        return Permit(parameters)
    }
}
return Denial
```

We can now expand the pseudocode to include role ordering:

```go
if !PinScopeAllowsAccessToResourceScope(cert.ScopePin, resource.GetScope()) {
    return Denial
}

for scopeOfOrigin := range DescendScopeHierarchy(resource.GetScope()) { // "/staging/west" -> ["/", "/staging", "/staging/west"]
    for scopeOfEffect := range AscendScopeHierarchy(resource.GetScope()) { // "/staging/west" -> ["/staging/west", "/staging", "/"]
        for role := range GetMatchingRoles(cert, AssignedFrom(scopeOfOrigin), AssignedTo(scopeOfEffect)) {
            if CanAccessResource(role, resource) {
                parameters := GetAccessConstraints(role, resource)
                return Permit(parameters)
            }
        }
    }
}

return Denial
```

Note that the function of the check is essentially unchanged, except that it is now expanded to reveal the way in which scopes
are used to order the roles being evaluated.


### Global and Scope-Bound Controls

Because of the limitations imposed by scope isolation and single-role evaluation, certain access controls that teleport
traditionally implements via roles are not suitable for scoped roles, or need to function differently. This is an artifact
of the change in how the role as a concept associates/binds a control with an action. Scoped roles always bind a control
to a specific action based on scope and the nature of the action itself (e.g. in the case of ssh access, the controls
are bound by a combination of the role being assigned to the user, the scope of the target node, and the labels of the
target node). This presents a challenge, as some existing teleport controls are intended to be enforced independent of
knowledge of the target resource.

As an example, consider the `client_idle_timeout` control. In classic teleport roles, each user has a single possible
client idle timeout, determined by the least permissive value across all of their roles and the global cluster networking
config. This allows the proxy networking stack to enforce client idle timeout using only the user's certificate, without
needing to know the target resource being accessed. In a scoped context, this doesn't work. A role-level `client_idle_timeout` 
control cannot be enforced without knowledge of which role is granting access to the target resource (to do otherwise would
violate hierarchical isolation).

It is infeasible at this time to make all teleport access controls resource-aware. We can solve this problem, in part,
by simply enforcing global controls. In the `client_idle_timeout` example, the proxying layer can enforce the global
maximum client idle timeout defined in cluster networking config. Then, if a specific access attempt merits a more
strict timeout, the resource-aware layer can enforce that as needed. This approach works well for many usecases,
but may not be suitable for users with particularly permissive sub-environments, or for controls that don't lend
themselves well to "double enforcement".

In order to ensure that highly permissive sub-environments can coexist with more restrictive global controls, we will
introduce the concept of "scope-bound" controls. These will work similarly to the global controls teleport already
supports, but will be definable at the scope level. Say, for example, that we needed to define different client idle
timeouts for different scopes. We could do this via a `scopes_networking_config` resource like so:

```yaml
kind: scopes_networking_config
# ...
spec:
  rules:
    - scope: /prod
      client_idle_timeout: 15m
    - scope: /staging/west
      client_idle_timeout: 1h
    - scope: /staging/east
      client_idle_timeout: 45m
    - scope: /dev
      client_idle_timeout: 6h
version: v1
```

In contexts where controls are enforced without knowledge of the target resource (e.g. certain networking/routing layers),
the scope of the user's login session can be used to determine which per-scope settings to apply. This will allow
more permissive environments to be set up without requiring significant refactoring of existing components.
For example, given the above configuration, a user who has run `tsh login --scope=/dev` can be safely granted the 6 hour
client idle timeout even in contexts where the target resource is not known, because their credentials are pinned to the
target scope, and so the networking stack can statically know that the user will not be able to use their credentials to
access resources outside of the `/dev` scope.

Scope-bound controls of this kind are powerful, but they do come with a limitation that is important to understand.
In order to obey the principle of least privilege, we must enforce the strictest applicable control from within the
pinned scope hierarchy. In the above example, this means that if a user is pinned to `/staging`, they will be subject
to the 45m client idle timeout defined in `/staging/east` even if they are accessing a resource in `/staging/west`.
This is because the pinned scope of `/staging` includes both `/staging/east` and `/staging/west`, and so resource-agnostic
enforcement points must assume the strictest control across all children of the pinned scope.

The effect of the above limitation is that most scope-bound controls will need to be editable only by global/root admins.
If we allowed scoped admins to edit scope-bound controls, we would be unable to prevent them from affecting access outside
of their scopes. We do not believe this will be a significant limitation. Scoped admins should generally be managing access
within their scopes via scoped roles and assignments. Very few controls actually *require* resource-agnostic enforcement,
and those that do tend to be simple controls like `client_idle_timeout` that do not require frequent modification or fine-grained
delegation.


### Benefits and Implications of Single Role Evaluation

As we see in the discussion above, the decision to embrace single role evaluation puts some limitations on the power and
flexibility of scoped roles.  However, there are also a number of interesting implications/benefits, above beyond simply
improving scope isolation and enabling more permissive sub-environments. A few of the key highlights:

- Each role fully communicates and preserves administrator intent (except where overridden by global/scope-bound controls),
  greatly simplifying the process of tracing and reasoning about access control decisions.
- It is safe to skip/ignore a role that is malformed since it cannot affect the evaluation of other roles (solving many
  issues faced today, such as users getting locked out due to malformed assignments/connectors).
- Role evaluation has the potential to be more performant since we can short-circuit on the first allow.
- It is safe to allocate credentials with one or more roles omitted (potentially very powerful when users want
to delegate their access to tooling/agents).
- If/when we introduce scoped access requests, we will be able to do many additional things that are not possible
with standard teleport access requests:
    - Create credentials with *only* the requested roles.
    - Allow users to selectively assume only a subset of requested roles at a time.
    - Eliminate side-effects in resource requests, and allow certs with resource request grants to continue to
    be usable for other access.
    - Allocate a single credential that contains multiple separate role+resource bindings for resource-based requests.
- The ability to safely ignore missing roles improves/enables many powerful patterns/features:
    - Meta policies can be introduced that safely invalidate individual roles without risking breaking general access.
    - Validation can be relaxed for many types, namely assignment resources, making it easier to automate creation of
    resources without worrying about ordering.
    - It becomes much safer to introduce features that indirectly manage role life cycles (we are currently very very wary
    about anything that automatically deletes a role).
- It will become possible for bots to output credentials with a *subset* of their privileges assigned to them.
- We can introduce simple flows to allow admins to "test drive" individual roles without needing to worry about their
  existing role set introducing side-effects (though traits still make this kind of test-driving imperfect).
- Audit logs can be updated to include the specific role that granted access, greatly improving traceability.


## Beyond Initial MVP

The features described above represent our initial target MVP. What we have been referring to as "phase 1" in most scoping
design discussions. The initial MVP will provide a foundation for experimentation with scopes in a meaningful way, but
will not be sufficient for most users to adopt scoping broadly. This document does not discuss features beyond the MVP
goal in detail. Post-MVP work will be driven by user feedback and demand, and for most features/APIs we have yet to
thoroughly explore how the scoping model will apply. That said, we do have some mid and long term goals in mined,
discussed in brief below.

### Mid Term Goals

The below features are ones we intend to tackle in the months following the initial MVP release, some of which we may start
work on in parallel while polishing and stabilizing the MVP:

- **Scope-Aware UI**: The initial MVP will only provide CLI-based scoping features. Follow-up work will add scope-awareness
  to the web UI and Teleport Connect, including a polished UX for scope pinning and scoped administration.

- **Scoped Kubernetes Access**: Scoped administration and access for the kubernetes access protocol. Like scoped ssh access,
  this will involve a combination of adding scoped joining/resource-creation, and updating kube related access-control
  checks to be scope aware. This will also be our first foray into scoping for protocols that do not have a one-to-one
  mapping between agent and target resource.

- **Scoped Audit Data**: Audit events related to scoped access will be updated to encode the "origin" scope of the event.
  Access-control for reading scoped audit data will also be implemented, the express goal being to allow scoped admins to
  view, or assign the right to view, audit data related to their scopes.

- **Advanced Joining and Discovery**: Joining modes other than static token will be added to scoped tokens, and a scoped
  discovery service will be implemented s.t. resources can be discovered and automatically assigned to appropriate scopes.
  This may involve an intermediate stage where unscoped discovery services are able to join scoped resources, but we would
  like to eventually allow scoped admins to create and manage their own separate discovery services.

- **Scoped Machine Identities**: Bot/machine-id features will be updated to be scope aware. In particular, we will be looking
  to support scoped machine identities s.t. local admins can create machine identities within their scopes to support automated
  operations limited to their scope. Note that this actually becomes a fairly hard preclusion to role-based scoped deny rules
  ever being implemented. Scoped deny rules would be escapable by the creation of scoped machine identities.

- **Scoped Session TTLs**: Session TTLs are a particularly tricky control to get right in a scoped context. Traditionally,
  a session TTL has been equivalent to a certificate TTL. In a scoped context, this may not always make sense. It would
  violate scope isolation principles to allow a scoped role in a child scope to affect the TTL of credentials usable in
  parent/orthogonal scopes. It may also be confusing to users if a given credential appears to be "expired" at different
  times depending on the resource being accessed.

- **Scoped Development Guidelines**: As we get a better handle on what the best patterns/practices are for adding scoping
  to new and existing APIs/logic, we will circle back and compile development guidelines to help us ensure consistency and
  correctness as we begin to scale up our scoping work.
 

### Long Term Goals

The below features are ones we have strong plans to implement, but are not planning to tackle until more or all of the mid
term goals have been addressed:

- **Additional Access Protocols**: Scoping is extended to other teleport access protocols (db, apps, etc). Exact order to be
  determined based on ease of implementation and user demand.

- **Scoped Workload Identity**: Intended to function similarly to existing workload identity features, but specialized to
  meet the needs of scoped administration of workloads.

- **Advanced Access Protocol Features**: The initial rollout of scoping to access protocols will focus on core features only.
  Follow-up work will add extended functionality (e.g. scoped session joining/moderation controls for ssh access).

- **Scope Aware Ecosystem**: Scope awareness is added to more of the wider teleport ecosystem/tooling (e.g. access graph, audit
  event exporter, etc).

- **Scoped Impersonation**: Stakeholders have specifically called out impersonation-like features as being essential to
  the ease of use for scoped admins. There needs to be some mechanism by which scoped admins can effectively "test" the
  permissions they write exclusive of their own permissions. 

- **Scoped Join Token Label Pinning**: Scoped join tokens will be updated to be able to statically assign labels to the
  agents/resources they are used to join. This will allow scoped admins to create more robust access-control policies
  by handing out tokens that are limited not just by scope, but also by label.

- **Scoped Primary Keys**: This is a long-term internal refactoring goal. Initial scoping work, inkeeping with the goal of gradual
  rollout and adoption, will be continuing to use teleport resource names as primary keys/unique identifiers. This will mean
  that primary keys will be "first come first serve" across scopes, which results in a sub-optimal user experience. We would like
  to move to a model where resources are uniquely identified by a combination of scope and name, allowing the same name to be
  reused in different scopes.

- **Advanced Access Control Features**: Additional access-control types/features (e.g. access requests) will have scoping
  support or scoped equivalent introduced on a case-by-case basis.


### Deferred Goals

Certain features we are intentionally deferring, possibly indefinitely. These are features we have either concluded are
incompatible with scoping as we understand it today, or ones we believe would be tricky enough to implement that they are
not worth tackling until we have more thoroughly explored the design space:

- **Deny Rules In Scoped Roles**: As discussed above, the concept of a role-determined `deny` rule is incompatible
  with other key scoping features. Some usecases may require something *like* a deny rule, but due to their problematic
  nature, whatever feature we add to fill that need should not be approached until other scoping features are mature enough
  that we can be confident we fully understand the implications of a feature with such strong side-effects.

- **Scoped Trusted Clusters**: We believe that scoping like won't map well to cross-cluster trust relationships.
  Furthermore, for many use-cases, the hope is that scoping will eventually be able to replace the need for multiple
  clusters entirely.

- **Scoped Trait Assignment/Modification**: Trait modification is a powerful feature of classic teleport access lists,
  but traits being global is very deeply baked into teleport. If we decided to introduce scoped traits, we'd likely want
  to wait until a post-PDP world where changing the nature of traits would be less onerous.

- **Root Scope Privileges**: We are deliberately not supporting assignment of permissions at the root scope (i.e. `/`)
  for the time being. We are treating this as a "reserved" feature, the intent being that we may introduce special-casing
  around root privileges in the future. In theory, a root scope might be a path to unifying classic and scoped teleport
  role features in a backwards-compatible manner, though likely that would not be possible without introducing some amount
  of breaking changes, at least on the scoping side. Reservation of the root scope, in combination with keeping scopes
  behind an unstable flag, is intended to help us keep our options open in this regard.
