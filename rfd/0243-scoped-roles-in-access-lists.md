---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 0243 - Scoped Roles in Access Lists

## Required Approvers

* Engineering: @fspmarshall && (@r0mant || @smallinsky || @kopiczko || @hugoShaka)
* Security: @rosstimothy || @rob-picard-teleport
* Product: @klizhentas

## What

This RFD describes a design for supporting the assignment of scoped roles at
specific assigned scopes from access lists.
This does _not_ describe a design for making access lists scoped themselves.

Access lists currently allow the assignment of roles as follows:

```yaml
kind: access_list
metadata:
  name: example-list
spec:
  grants:
    roles:
      - editor
      - auditor
version: v1
```

With this change scoped roles will be assignable at specific scopes:

```yaml
kind: access_list
metadata:
  name: example-scoped-list
spec:
  grants:
    scoped_roles:
      - role: ops-staging-access
        scope: /ops/west
      - role: ops-prod-access
        scope: /ops/west
version: v1
```

Recommended Reading:

* [RFD 229 - Scopes](./0229-scopes.md)
* [RFD 6e - Access Lists](https://github.com/gravitational/teleport.e/blob/master/rfd/0006e-access-lists.md)
* [RFD 170 - Nested Access Lists](./0170-nested-accesslists.md)

Related Reading:

* [A previous RFD attempt for scoped access lists](https://github.com/gravitational/teleport/pull/62388)

## Why

Scoped roles are a new scoped variant of traditional roles that can be used to
grant privileges at specific scopes.
They currently must be granted to users via a `scoped_role_assignment` for each user.
Allowing access lists to grant a set of scoped roles to all list members will
make it easier to assign scoped roles to groups of users and is another step on
the path to bring feature parity to the new scoped authentication/authorization
model.

## Details

### UX

#### User story - IdP sync with nested list membership

Examplecorp operates out of two regions, east and west.
Each region has their own groups of admins and users.
They want to grant each admin group broad administrative privileges over
everything in their region, and each user group access to servers in their
region, without allowing admins or users in each region to modify or access
anything in the other region.

They have defined the following scoped roles that they want to grant to their users:

```yaml
kind: scoped_role
metadata:
  name: ops-admin
scope: /
spec:
  assignable_scopes:
  - /ops/**
  rules:
  - resources:
    - scoped_role
    - scoped_role_assignment
    verbs: [create, list, readnosecrets, update, delete]
---
kind: scoped_role
metadata:
  name: ops-staging-access
scope: /
spec:
  assignable_scopes:
  - /ops/**
  node_labels:
  - name: 'env'
    values: ['staging']
  logins: ['opsuser', 'root']
---
kind: scoped_role
metadata:
  name: ops-prod-access
scope: /
spec:
  assignable_scopes:
  - /ops/**
  node_labels:
  - name: 'env'
    values: ['prod']
  logins: ['opsuser']
```

But now they are faced with the challenge of assigning these roles to all their
thousands of users.
They have all their users and groups already defined in their IdP and are
already importing their user groups as Teleport access lists using one of our
existing integrations (Okta, Entra), and now they want to assign these scoped
roles to user groups.

The access lists they are already importing with members are `west-admins`,
`west-users`, `east-admins`, and `east-users`.
To assign the scoped roles defined above to members of these existing access
lists, they define new access lists granting the scoped roles, and then make
their existing imported lists members of the new lists.
Here are the list definitions:

```yaml
kind: access_list
metadata:
  name: west-admins-scoped
spec:
  title: "west admins scoped"
  grants:
    scoped_roles:
      - role: ops-admin
        scope: /ops/west
version: v1
---
kind: access_list
metadata:
  name: west-users-scoped
spec:
  title: "west users scoped"
  grants:
    scoped_roles:
      - role: ops-staging-access
        scope: /ops/west
      - role: ops-prod-access
        scope: /ops/west
version: v1
---
kind: access_list
metadata:
  name: east-admins-scoped
spec:
  title: "east admins scoped"
  grants:
    scoped_roles:
      - role: ops-admin
        scope: /ops/east
version: v1
---
kind: access_list
metadata:
  name: east-users-scoped
spec:
  title: "east users scoped"
  grants:
    scoped_roles:
      - role: ops-staging-access
        scope: /ops/east
      - role: ops-prod-access
        scope: /ops/east
version: v1
```

Then to make all users of the existing groups members of these new access
lists, they create the list memberships:

```bash
# make west-admins a member of west-admins-scoped
tctl acl users add --kind=list west-admins-scoped west-admins
# make west-users a member of west-users-scoped
tctl acl users add --kind=list west-users-scoped west-users
# make east-admins a member of east-admins-scoped
tctl acl users add --kind=list east-admins-scoped east-admins
# make east-users a member of east-users-scoped
tctl acl users add --kind=list east-users-scoped east-users
```

These access lists and memberships could also be managed with Terraform instead
of `tctl`.

#### Access list UX

Initially, users will be able to add scoped role grants to access lists via
tctl, terraform, and the k8s operator.
Scoped role grants in access lists will be viewable with `tctl get access_list/name`.

We will need to add the ability to view and edit scoped role assignments in the
web UI.
The name and assigned scoped of granted scoped_roles will be viewable on the
access list page.
When editing granted permissions, a scoped role input will be presented if the
cluster contains any scoped roles defined in the root scope.
The editor will not allow adding required permissions if the role contains any
scoped_role grants, and will not allow adding scope_role grants if the role has
any required permissions (see [invariants](#invariants)).

### Overview

Scoped roles are currently assigned to users via `scoped_role_assignment`
resources that look like the following:

```yaml
kind: scoped_role_assignment
metadata:
  name: uuid1
scope: /
spec:
  user: admin@ops-west.example.com
  assignments:
  - role: ops-admin
    scope: /ops/west
  - role: ops-staging-access
    scope: /ops
version: v1
```

The above would grant to the `admin@ops-west.example.com` the following roles:

* `ops-admin` in scope `/ops/west`
* `ops-access` in scope `/ops`

Access lists will not really be an alternative to scoped role assignments,
rather, they will essentially be a way of automating the definition of scoped
role assignments for many users at once.

For example, if you wanted to make both `alice@example.com` and `bob@example.com`
admins over the `/ops/west` scope, you could define an access list as follows:

```yaml
kind: access_list
metadata:
  name: west-admins
spec:
  title: "west admins"
  grants:
    scoped_roles:
      - role: ops-admin
        scope: /ops/west
      - role: ops-access
        scope: /ops
version: v1
---
kind: access_list_member
metadata:
  name: a3e64073-8980-41aa-a7a0-dd23de40c38e
spec:
    access_list: west-admins
    name: alice@example.com
    membership_kind: MEMBERSHIP_KIND_USER
version: v1
---
kind: access_list_member
metadata:
  name: c4c9d4b3-c57c-40de-a15e-e2425267a716
spec:
  access_list: west-admins
  name: bob@example.com
  membership_kind: MEMBERSHIP_KIND_USER
version: v1
```

Under the hood the scoped access cache would materialize from this access list
the following scoped role assignments:

```yaml
kind: scoped_role_assignment
sub_kind: materialized
metadata:
  name: acl-west-admins-alice@example.com
scope: /
spec:
  user: alice@example.com
  assignments:
  - role: ops-admin
    scope: /ops/west
  - role: ops-access
    scope: /ops
status:
  origin:
    creator: access_list
    creator_name: west-admins
    date_created: "2025-12-18T04:00:00Z"
version: v1
---
kind: scoped_role_assignment
sub_kind: materialized
metadata:
  name: acl-west-admins-bob@example.com
scope: /
spec:
  user: bob@example.com
  assignments:
  - role: ops-admin
    scope: /ops/west
  - role: ops-access
    scope: /ops
status:
  origin:
    creator: access_list
    creator_name: west-admins
    date_created: "2025-12-18T04:00:00Z"
version: v1
```

These "materialized" scoped role assignments would be inserted into the scoped
role assignment cache.
Everything that already works with scoped role assignments today will not need
to know or care about access lists, they will just handle the resulting
assignments as usual.

We will introduce `sub_kind: materialized` to distinguish materialized
scoped_role_assignments from user-created ones and prevent name collisions.
Materialized assignments will be named `acl-<list-name>-<user-name>`, but this
will not be guaranteed to be stable into the future.
Scoped role assignments with a sub_kind will use `<name>/<sub_kind>` as a
primary key in the backend and the scoped access cache.

### Inherited list membership

Access list members can either be users or other access lists.
When listA has member listB, it means that all members of listB are inherited
members of listA.
In this case listA has delegated part of its membership definition to listB.
In general, a user is a member of listA if they are an explicit member of
listA or they are a member of any list that is an explicit or inherited member
of listA.

For example, consider an access list named `west-access` that grants access to
SSH servers in the `/ops/west` scope.
If an admin wanted to grant SSH access to all members of the `west-users` and
`west-admins` lists, they could make `west-users` and `west-admins` members of
the `west-access` list.
In doing this, the `west-access` list would be delegating the definition of
its user members to its member lists.
All members of `west-users` and `west-admins` would become inherited members of
the `west-access` list.

Nested list memberships effectively form a graph where nodes are access lists
and edges are access list memberships.

### Owner grants

Access lists can grant privileges not only to members but also to owners of the
list under the `owner_grants` field, this will also support scoped role grants.

```yaml
kind: access_list
metadata:
  name: owner-grants-example
spec:
  title: "owner grants"
  owner_grants:
    scoped_roles:
      - role: ops-admin
        scope: /ops/west
      - role: ops-access
        scope: /ops
  owners:
    - name: alice@example.com
      membership_kind: MEMBERSHIP_KIND_USER
    - name: admins
      membership_kind: MEMBERSHIP_KIND_LIST
version: v1
```

Access list owners are listed directly in the access list spec, they can either
be direct user owners or other lists can be named as owners.
When an access list `a` is named as an owner of access list `b`, all _members_
of `a` become owners of `b` and receive the grants.
Owners of `a` do not become owners of `b`.

### Materialization of scoped role assignments

The term "materialization" is used here to mean the computation and storage of
concrete scoped role assignments from their source of truth, which is the
current set of access lists and access list members.
Rather than referencing access lists (and memberships) and traversing the graph
during scoped login events, the set of materialized scoped role assignments
will be computed ahead of time and stored in the scoped role assignment cache.
The reasoning for this is to ensure that teleport can efficiently determine the
full set of scoped privileges in user-facing hot paths, without the need to
relogin.
Being able to efficiently discover scopes at which a user has privileges
without requiring reauthentication is critical for a good user experience.

Every (user, list) pair, where user is an explicit or inherited member or owner
of list and list grants scoped roles, will result in 1 materialized scoped role
assignment.
Each materialized assignment for (user, list) will grant exactly the scoped
roles defined in the spec of that list (for members, owners, or both depending
on the user's relationship with the list).

The scope of the materialized assignment will be the root scope `/`.
As an unscoped policy resource, access lists have an authority/provenance
equivalent to the `/` (root) scope.
If we materialized assignments s.t. they were owned by a scope lower than root,
that would permit assignment editing permissions in child scopes to "reach up"
and change the intent/effect of the higher level access list policy, which
would violate scope isolation/hierarchy.

For example, if alice is an explicit member of listA, and listA is an explicit
member of listB, then alice is member of both listA and listB.
2 scoped role assignments will be materialized, one for (alice, listA) and
another for (alice, listB).

If listB is an owner of listC, then alice's membership in listB makes her an
owner of listC, and a scoped role assignment will be materialized for (alice,
listC).

The materialized assignments will be initialized along with the scoped access
cache and kept up to date via a backend watcher on access lists, access list
members, and scoped roles.
The implementation will gracefully handle access list graphs of an arbitrary
depth that may contain cycles, although performance may degrade in diabolical
cases.
Performance characteristics will be benchmarked once this system is implemented
to ensure they are reasonable (TBD: reasonable performance characteristics).

Note: if many users are members of many lists, this could result in a lot of
materialized assignments.
For example, if 20k users are all members of 1k lists all of which grant scoped
roles, this would result in 20 million materialized scoped role assignments.
In general the number of assignments will be approx `(num users) x (avg lists per user)`.
If scale becomes a concern here we could consider aggregating scoped role
assignments for each user, so there would be approximately 1 materialized
scoped role assignment for each user, more if they are assigned too many unique
roles to fit in a single gRPC message and the assignments need to be batched.
In this case if 20k users were members of any number of lists across 20 scopes,
there would be approximately 20k materialized assignments.
The downsides here would be that the source of the materialized assignment would
be more difficult to reason about, the cache would be more difficult to
maintain as access lists and memberships change, and batching large assignments
would introduce additional complexity.
The current plan is not to aggregate scoped role assignments.

Earlier in a design for scoped access lists there was discussion of writing
these materialized assignments out to the cluster's backend database.
For the time being, we are opting not to do this and instead keep the
materialized assignments in the scoped access cache only.

Some reasons not to store materialized assignments in the backend are:

1. The source of truth is the access list and member resources, storing the
   materialized assignments would store redundant data, with redundancy there
   is potential for drift or conflicts that need to be resolved.
2. Multiple auth servers would be trying to write the same materialized
   assignments to the backend, they may disagree or conflict, especially if
   auth servers are running different versions.
3. Dangling assignments would need to be detected and cleaned, while avoiding
   deletion of new assignments created by a different auth server.
4. Materialized assignments need to be computed and cached in memory on each
   auth server anyway to keep the backend up to date.
5. Keeping them out of the backend will reduce read and write load on the backend.
6. Loading many resources from the backend can be very slow on startup, e.g. 5
   minutes to read 2 million resources from dynamo. Recomputing is likely to be
   faster in most cases.
7. Everything that needs to read scoped role assignments already needs to use
   the custom scoped access cache for scoped traversal, there are not going to
   be access paths that read directly from the backend and bypass the cache.

### Access list limitations

Due to the requirement of being able to efficiently discover scopes at which a
user has privileges without requiring reauthentication and the resulting scoped
role assignment materialization strategy, some limitations will apply to access
lists that assign scoped roles and their member lists.

We do not want scoped role assignments to depend directly on user traits, which
are dynamic and may change on relogin as they are assigned by an external IdP.
To maintain this, any access list that grants a scoped role must not include any
`membership_requires` or `ownership_requires` block.
These blocks normally prevent access list membership from applying based on
user traits.
This limitation will apply to access lists that directly grant scoped roles,
and any of their member lists (which would transitively grant scoped roles).

This will be enforced during access list and access list member creation on a
best effort basis since strict enforcement would require full access list graph
traversal while holding a global lock.
If membership or ownership requires blocks end up being added to an access list
that grants scoped roles, the scoped_role_assignment materializer will detect
the change, log the invalid list, and stop materializing
scoped_role_assignments for memberships in that list.

### Prerequisites/External Limitations

The current implementation of the `scoped_role_assignment` resource is too
strict/opinionated to work well with access lists.

In particular, the current implementation of `scoped_role_assignment` resources
does not have a well-defined model for handling invalid or dangling assignments
that may be possible while changes are propagating through the cache, such as
an assignment referencing a deleted role or attempting to assign it at a scope
that is no longer permissible.

While this limitation does not directly block access list implementation,
scoped role assignments via access lists may be broken or error-prone in some
edge cases.
The plan is to implement a model for handling invalid or dangling assignments
where all assignments will be checked when they are loaded, and invalid
assignments will be dropped.

Also, in order for the scoping model to be fully consistent, access lists must
only be able to assign roles defined in the root scope '/'.
This is necessary to prevent admins of only specific scopes from modifying a
role assigned by an access list defined in the root scope, and follows from the
current limitation that scoped_role_assignments must have the same scope of
definition as each role that they assign.
Defining scoped_roles in the root scope is currently not allowed, we will need
to update/allow this.

### Identity provider sync

Teleport already has multiple integrations with third party identity providers
that automatically import groups and create Access Lists to model them in
Teleport with bidirectional sync.
These include AWS IAM Identity Center, Microsoft Entra ID, Okta, and SailPoint.

At least initially, we will not alter these integrations to automatically
create access lists granting scoped roles.
Instead, admins can create other access lists to grant scoped roles, and define
the IdP imported access lists as members of those lists.

### Mixing scoped and unscoped role grants

Access lists will technically be allowed to grant both regular and scoped roles:

```yaml
kind: access_list
metadata:
  name: example-list
spec:
  grants:
    roles:
      - editor
      - auditor
    scoped_roles:
      - role: ops-staging-access
        scope: /ops/west
      - role: ops-prod-access
        scope: /ops/west
version: v1
```

When users do a "regular"/unscoped login, they will receive the unscoped roles
as usual.
When they do a scoped login at a specific scope, they will receive all roles
granted in scopes non-orthogonal to the scope they're logging in to (this is
the effect of the materialized scoped role assignments).

### Security

#### Invariants

* Access lists can only assign scoped roles that exist
* Access lists can only assign roles defined in the root scope
* Access lists can only assign roles to an assignable_scope of the role

These invariants will be enforced at two levels:
* first is enforcement at the backend/storage layer, as described below,
  skipped for forced writes (Upserts).
* second is the materialized scoped_role_assignments are always checked for
  these invariants when building an access checker for user login or resource
  access checks. This does not actually enforce the invariant on access lists,
  but enforces that the assignments that result from any access list can only
  assign a scoped role that exists in the root scope at an assignable_scope of
  that role.

The scoped access backend already has an AtomicWrite strategy for handling
writes to scoped_roles and/or scoped_role_assignments while enforcing this kind
of invariant.
As access lists act similarly to scoped_role_assignments, they can use the same
strategy.
Namely, a value at backend key `/scoped_role/role_lock/<role-name>` is used to
synchronize writes to the named scoped role and all assignments referencing the
role.
Create and Delete on scoped_role_assignments atomically assert the revision
of each referenced scoped_role, and modify the lock value for each role so that
writes to scoped_roles can efficiently detect concurrent modification to assignments.
Create/Update/Delete on scoped_roles atomically assert that the revision of
this lock has not been modified between the time the invariants are checked and
the time the role is written.

AccessList Create and Update will check that all referenced scoped roles
exist, are defined in the root scope, and are assignable at the assigned scope.
They will use AtomicWrite to assert the checked revision of each referenced
scoped role, and modify `/scoped_role/role_lock/<role-name>` for each referenced
role to make sure it isn't modified immediately after an access list create/update.
This requires 2 conditions per referenced role, which will limit the number of
roles a single access list can reference.
This same limitation applies to scoped role assignments already, the current
limit is 16.

UpdateScopedRole may modify a role's assignable scopes.
In this case it will check that the change doesn't invalidate any assignments
in extant access lists, and will continue to use AtomicWrite to verify that
`/scoped_role/role_lock/<role-name>` hasn't concurrently changed.

DeleteScopedRole will continue to use AtomicWrite to verify that
`/scoped_role/role_lock/<role-name>` hasn't concurrently changed to verify that
no new access lists reference it.

* Access lists that assign scoped roles cannot contain `membership_requires` or `ownership_requires`

UpsertAccessList and UpdateAccessList will statically verify that the list
cannot contain both scoped role grants and requirement blocks.

* Member lists of lists that assign scoped roles cannot contain
  `membership_requires` or `ownership_requires`

UpdateAccessList (if it adds a scoped role assignment or requirement block),
and access list member creation methods, will have to traverse the access list
graph to assert that no transitive members of access lists that grant scoped
roles can contain requirement blocks.

It's likely to be prohibitively expensive to do this check while holding a
global backend lock (and it's not clear if it's possible to implement with
AtomicWrite) so this check will be best-effort.
If the invariant is violated, access lists with requirement blocks will not
result in scoped role assignments being materialized (they will be dropped by
the assignment materializer and the invalid list or membership will be logged).

* Materialized scoped role assignments will be created for each extant (user, list)
  where user is a member and/or an owner of list.
* Materialized scoped role assignments will be deleted when user ceases to be a
  member or owner of a list.

These are not being written to the backend and thus cannot use AtomicWrite.
These invariants will be enforced when the scoped access cache is initialized,
and when access lists and their members are created/updated/deleted.
To avoid non-deterministic behavior while scoped role assignments are initially
being materialized, process readiness will be gated on the scoped access cache
being ready.

### Privacy

The changes described here do not affect privacy.

### Proto Specification

```diff
--- a/api/proto/teleport/accesslist/v1/accesslist.proto
+++ b/api/proto/teleport/accesslist/v1/accesslist.proto
@@ -169,6 +169,19 @@ message AccessListGrants {
   // traits are the traits that are granted to users who are members of the
   // Access List.
   repeated teleport.trait.v1.Trait traits = 2;
+
+  // scoped_roles are the scoped roles that are granted to users who are
+  // members of the Access List.
+  repeated ScopedRoleGrant scoped_roles = 3;
+}
+
+// ScopedRoleGrant describes a scoped role granted at a specific scope.
+message ScopedRoleGrant {
+  // role is the name of the scoped role to be granted.
+  string role = 1;
+  // scope is the scope the role will be assigned at. It must be an assignable
+  // scope of the role.
+  string scope = 2;
 }
```

### Scale

The current design materializes scoped role assignments for each (user, list)
pair where user is a member of list, this could scale to large numbers of
resources that will consume memory on the auth service.
This is discussed in [Materialization of scoped role assignments](#materialization-of-scoped-role-assignments).

### Backward Compatibility

In case of cluster downgrade, the new `scoped_roles` field, if present in any
access list grants, will not be seen by older auth server versions and should
not cause any validation problems.

In case we decide to persist materialized scoped_role_assignments to the
backend in a future version, all backend reads will filter out assignments with
`sub_kind: materialized` so that they don't conflict with auth servers on an
older version still materializing in memory only.

### Audit Events

No new audit events will be created or emitted as no new resources or actions
are being added.

### Observability

Traces will be added for scoped role assignment materialization to monitor performance.
Metrics will be added for the number of materialized scoped role assignments.

### Product Usage

TBD, scopes are currently rolling out to limited design partners.

### Test Plan

Scoped role assignment via access lists with and without nested memberships
will be added to the test plan.
