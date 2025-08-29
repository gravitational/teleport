---
authors: Forrest Marshall (forrest@goteleport.com)
state: draft
---

# RFD XXXX - Scopes

## What

A system for heirarchical organization and isolation for resources and permissions.


## Why

Historically, teleport's resource and permission organization systems have been very "flat". In particular,
administrative and resource-provisioning permissions tend to be "all or nothing". Resource labels and selectors
allow organization of resources/permissions, but said organization can only be performed by what are effectively
global admins. Any user with the ability to create node join tokens can join any node to the cluster
with any labels they like. Similarly, a user with role creation permissions can create a role that grants access
to anything. This poses a significant challenge when trying to delegate any meaningful responsibility and/or to
apply the prinicial of least privilege to users that need to administer resources/permissions in an meaningful way.

Similarly, teleport user credentials tend to be all or nothing. There isn't a good way to get credentials that
are only useable for the specific task at hand. Instead, if a user is logged into a teleport cluster they always
have all their available permissions applied. This increases blast radius, both in terms of compromise and accidental
misuse.

We would like to provide a mechanism for organizing resources and permissions in a manner that allows for both
isolation and heirarchy. This system should support admins that have powerful control over provisioning of, and
access to, resources within their domain of influence. Said admin privileges must not be able to affect resources
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
ERROR: access denied

$ tsh ssh some-node-west
# success
```

Note that the nature of commands after login are unchanged. Ordinary (non-admin) users should be able to ignore the concept
of scoping once they have logged in to the appropriate scope. Scripts that work today with an already logged-in `tsh` should
continue to function as expected with scoped credentials, with the only change being that the resources affected by the opeartions
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

Scopes are heirarchical. The scope `/staging` is a "parent" of `/staging/west`, and `/staging/west` is a "child" of `/staging`.
Permissions granted at a parent scope apply to all child scopes.

Scope heirarchy is segment based, not prefix based. I.e. `/staging` is a parent of `/staging/west`, but not of `/stagingwest`.

Scopes are attributes, not objects. There is no need to create a scope object before creating a resource with a given
scope attribute. I.e. if no resource exists with scope `/staging/west`, a node can still be created with that scope without
first performing any kind of scope setup/creation/configuration ceremony.


### Core Design Goals

**Heirarchical Isolation**: Permissions within a given scope cannot be used to "reach up" and affect resources/access defined
in parent scopes or "reach across" and affect resources/access defined in orthogonal scopes.

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

- Scopes are arbitrarily heirarchical. Resources can be assigned at any level of the heirarchy. Other similar systems tend
  to have a heirarchy where all resources live in the leaf nodes, and often the depth of the heirarchy is fixed.

- Scopes are an attribute, not an object. There is no additional creation step like there would be with a system that organizes
  resources into accounts/projects/etc. Other systems tend to require the heirarchy be defined separately as a standalone entity
  prior to resources being created.

- Credentials can be pinned to arbitrarily narrow scopes. A user can opt to access a resource in `/staging/west` by logging
  into `/staging` *or* logging into `/staging/west`. This gives granular control over the blast radius of credentials, and
  ensures that heirarchies and practices can evolve and refine over time without needing to be fully rebuilt. By virtue of
  the fixed leaf nodes of other systems, options for constraint of credentials via the organizing heirarchy tend to be more
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
of as being assigned to the "root" of the scope heirarchy (this is mostly a philosophical point, as we do not plan to support
permission assignment at root for the time being), and will not be accessible via scoped permissions.

For resources that grant permissions (e.g. roles), the story is more complicated. Assignment of scoped permissions doesn't mesh
well with existing permission-granting resources.  For these types, we will need to have new special scoped variants with
separate APIs. For example, we will be introducing a new `scoped_role` type which will implement a compatible subset of the
functionality of the `role` type and serve the same purpose when using scoping to organize permissions.

This bimodal approach to resource scoping is inkeeping with our user-facing design goal. The resources/apis that ordinary
(non-admin) users interact with will remain unchanged except for the addition of the scope field (which said users can safely
ignore). Administrative APIs/types will receive scope-specific variants that will be tailored to the needs of scoped
administration.


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

For ease of use, we will also provide a shortcut for reauthenticating to a different scope while keeping all other `tsh login`
parameters unchanged:

```shell
$ tsh scopes login /staging/west
# authentication flow
```

Within teleport's internals, the scope pin will serve as an additional constraint on all access-control checks. We will require
that any attempted resource access first be against a resource that is scoped to the pinned scope or a descendant scope. If this
check does not pass, access is denied without the need to load or evaluate any other permissions.

Pinned scope will also provide a default value for APIs that accept scope parameters.  For example, a scoped admin creating
a join token will automatically have the token created at their pinned scope unless they explicitly specify something else.
In this way, scope pinning will provide a means of reducing the friction associated with scoped administration since most admins
will simply log directly into the scope they wish to administer.


### A Scoped Access Check

In order to conform to our core isolation and heirarchy goals, the form of scoped access checking will need to diverge
somewhat from classic teleport access checks.  Ordinarily, in teleport code, an access check is a one-off decision, the
pseudocode of which would look something like this:

```go
roles := LoadRolesForUser(cert)

if CanAccessResource(roles, resource) {
    parameters := GetAccessConstraints(roles, resource)
    return Permit(parameters)
} else {
    return Denial
}
```

Note that allow decisions are not binary. There are often parameters that affect the nature of allowed access (e.g. allowed
ssh access may come with or without X11 forwarding enabled).

Per our scoping design goals, one of our key criteria is that administrative permissions assigned at a child scope cannot
be used to affect the nature of resources/permissions at a parent scope.  This means that we cannot allow permissions in
a child scope to modify the parameters of access that is permitted at a parent scope. We therefore end up with an access
check flow that looks something like this:

```go
if !PinScopeAllowsAccessToResourceScope(cert.ScopePin, resource.GetScope()) {
    return Denial
}

var roles []Role
for scope := range DescendScopeHeirarchy(resource.GetScope()) { // "/staging/west" -> ["/", "/staging", "/staging/west"]
    roles = append(roles, LoadRolesForUserAtScope(cert, scope)...)
    if CanAccessResource(roles, resource) {
        parameters := GetAccessConstraints(roles, resource)
        return Permit(parameters)
    }
}

return Denial
```

Note that we start from the uppermost parent scope, and iteratively descend. At the first scope where access is allowed,
we determine the full parameters of access *at that scope*. Assignment of additional roles/permissions at child scopes
have no effect per scope isolation rules.


### Scoped Roles and Assignments

A new `scoped_role` type will be introduced for the purpose of defining scoped permissions. This type will implement
a subset of the features of the existing `role` type, with features being ported over iteratively over time.

Classical teleport roles are assigned to users by directly editing the user resource's `roles` field. This centralized
approach is not inkeeping with the goal of delegated/limited administration.  Instead, scoped roles will be assigned
via a separate scoped assignment resource.  Admins of a given scope will be able to create scoped role assignments
for users independently, without the need to modify global state.

Inkeeping with the heirarchical isolation principle, scoped roles will only be assignable at the scope of the role
resource itself or a descendant scope.  For example, a `scoped_role` defined at `/staging` could be assigned to users
at `/staging` or `/staging/west`, but not at `/prod` or `/prod/west`. This ensures that role editing privileges in
one scope cannot be used to affect permissions in another scope.

As an additional layer of control, it will be possible to constrain the assignable scopes of a scoped role to
an explicitly defined subset. For example, a scoped role defined at `/staging` could be
configured be assignable at `/staging/west` and `/staging/east`, but not at `/staging` or `/staging/central`
by specifying `assignable_scopes: ["/staging/west", "/staging/east"]` in the role spec. This has a few important
benefits. First, by allowing this extra layer of control, we allows scoped roles to be used in a manner similar
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
definition.  Keeping this parity ensures ease of transition and reduces cognitive load for users familiar with
existing teleport concepts. It also saves us considerable effort since most of the existing role evaluation logic
can be reused.


### Scoped Tokens and Joining

The scope of an agent will be determined by their join tokens. Scoped administrators will be able to create join tokens
assigned to scopes where they have `token:create` permissions. When an agent joins the teleport cluster using a scoped
join token, the agent will be automatically assigned that scope.

Creation of scoped join tokens will mirror the existing token creation API where possible. Ex:

```shell
$ tctl scoped token add --type=node --scope=/staging/west
```

As an additional nicety, we will allow scoped admins to omit the `--scope` parameter and have it default to their currently
pinned scope. This will help admins working with more flat (namespace-like) heirarchies to avoid needing to reason about
scoping too much.

The resulting token value will be usable in the same way an ordinary join token works today. Ex:

```shell
$ teleport start --token=some-token-value ...
```

No special scope-related parameters will need to be passed to the agent. Any agent-side configuration that works with unscoped
static tokens today will also work with scoped tokens.

The scope of an agent/resource isn't just about limiting the access that users have to the agent. Inkeeping with heirarchical
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
we will be adding a new `AgentScope` filed to host certificates. Support for this field will need to be implemented s.t. it cannot
be escaped by reissue/etc, and agent-facing APIs will need to be udpated to support scope-aware permission checks as needed.
Primarily, this will mean limiting agent's ability to read any scoped configuration resources (e.g. scoped roles) to only the
subset that are relevent to the agent's function, and ensuring that presence information emitted by the agent always contains
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
nodes across different environements end up advertising the same hostname(s). This ambiguity means that sometimes
`tsh ssh <user>@<hostname>` is ambiguous. We currently allow cluster administrators to configure teleport into one of two
routing modes. By default, we reject ambiguous dial attempts and force the caller to dial by UUID.  Admins can opt into
route-to-most-recent, where the dial always hits the agent that heartbeat most recently.  With scoping, we now have a very
powerful tool for disambiguating dials. Dial attempts will now take into account the user's pinned scope. This means that,
for example, attempts to dial `myhost.example.com` in `/staging/west` will not be made ambiguous by the existence of
`myhost.example.com` in `/prod/east`.


### Scoped Access Lists

TODO: description of scoped access list type and design considerations

## Beyond MVP

TODO: additional access protols, discovery, auditing, etc
