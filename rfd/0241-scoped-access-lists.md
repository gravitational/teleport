---
authors: Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 241 - Scoped Access Lists

## Required Approvers

* Engineering: @fspmarshall && (@r0mant || @smallinsky || @kopiczko || @hugoShaka)
* Security: @rosstimothy || @rob-picard-teleport
* Product: @klizhentas

## What

At its core, an access list is a list of users and a set of privileges to be
assigned to all users in the list.
Scoped access lists are a new scoped variant of traditional access lists
that can be used to grant scoped roles to groups of users, and to grant those
roles at specific scopes.

Required Reading:

- [RFD 229 - Scopes](./0229-scopes.md)

Recommended Reading:

- [RFD 6e - Access Lists](https://github.com/gravitational/teleport.e/blob/master/rfd/0006e-access-lists.md)
- [RFD 170 - Nested Access Lists](./0170-nested-accesslists.md)

Scoped access lists will initially aim to provide a small subset of the
functionality of traditional access lists, tailored for the specific use case
of using scoped access lists to manage scoped privileges in the context of
automation and identity provider sync.

## Why

Where reasoning about users in terms of groups or lists is common and natural,
the main goal of scoped access lists is to allow groups of users to be defined
such that all members of those user groups can be granted a common set of
scoped privileges.

These user groups (access lists and their members) will be able to be defined
via automation such as terraform, or synced from an external identity provider.

In the current state of the scopes implementation, there is no way to assign
scoped roles to groups of users, admins must create a scoped role assignment
for every (user, scoped_role) pair.

With scoped access lists, admins will define a scoped access list for each
logical group of users.
The scoped access list definition will include the set of scoped roles that
should be granted to all members of that access list.
Then access list memberships may be synced from an identity provider, defined in
Terraform, or created with `tctl`.

## Details

### UX

#### User story

Examplecorp operates out of two regions, east and west.
Each region has their own groups of admins and users.
They want to grant each admin group broad administrative privileges over
everything in their region, and each user group access to everything in their
region, without allowing admins or users in each region to modify or access
anything in the other region (due to the ongoing east vs west rivalry).

They have defined the following scoped roles that they want to grant to their users:

```yaml
kind: scoped_role
metadata:
  name: ops-admin
scope: /ops
spec:
  assignable_scopes:
  - /ops/**
  rules:
  - resources:
    - scoped_role
    - scoped_role_assignment
    - scoped_access_list
    - scoped_access_list_member
    verbs: [create, list, readnosecrets, update, delete]
---
kind: scoped_role
metadata:
  name: ops-staging-access
scope: /ops
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
scope: /ops
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
They have all their users and groups already defined in Terraform, but they
don't want to map these into a `scoped_role_assignment` for each user defining
exactly which roles the user should be granted at which scopes.
Instead, they'd rather be able to define in one place which roles each user
group should be granted.
Creating an assignment for each user into their group will be much simpler and
more concise than repeating the role assignments for each user.

The user groups they have are `west-admins`, `west-users`, `east-admins`, and `east-users`,
so they define the following scoped access lists, one for each group:

```yaml
kind: scoped_access_list
metadata:
  name: west-admins
scope: /ops
spec:
  title: "west admins"
  grants:
    scoped_roles:
      - role: ops-admin
        scope: /ops/west
version: v1
---
kind: scoped_access_list
metadata:
  name: west-users
scope: /ops
spec:
  title: "west users"
  grants:
    scoped_roles:
      - role: ops-staging-access
        scope: /ops/west
      - role: ops-prod-access
        scope: /ops/west
version: v1
---
kind: scoped_access_list
metadata:
  name: east-admins
scope: /ops
spec:
  title: "east admins"
  grants:
    scoped_roles:
      - role: ops-admin
        scope: /ops/east
version: v1
---
kind: scoped_access_list
metadata:
  name: east-users
scope: /ops
spec:
  title: "east users"
  grants:
    scoped_roles:
      - role: ops-staging-access
        scope: /ops/east
      - role: ops-prod-access
        scope: /ops/east
version: v1
```

They can `tctl create lists.yaml` or define them in Terraform.

Then, for each user all they need to create is a `scoped_access_list_member`
for their group.
This can also be done with Terraform, or (one day) synced from an IdP, or done
with a `tctl scoped acl users add` command.

For example, to manually add `alice@example.com` to the `west-admins` group, they could run

```bash
$ tctl scoped acl users add west-admins alice@example.com
```

This would grant `alice@example.com` the `ops-admin` role at scope `/ops/west`.

Next Alice realizes she actually needs to access one of these SSH servers, and
her fellow admins in the west region do too, but the `ops-admin` role doesn't
grant any access.
As an admin over the `/ops/west` scope she can create a new scoped access list in the `/ops/west`
scope that will grant the roles she needs:

```yaml
kind: scoped_access_list
metadata:
  name: west-admin-users
scope: /ops/west
spec:
  title: "west admins that want to be users too"
  grants:
    scoped_roles:
      - role: ops-staging-access
        scope: /ops/west
      - role: ops-prod-access
        scope: /ops/west
version: v1
```

Alice wants all current members of the `west-admins` list to be members of this
new `west-admin-users` list, so she creates a single nested list assignment, in
Terraform or with the following command:

```yaml
$ tctl scoped acl users add --kind list west-admin-users west-admins
```

This adds the `west-admins` list as a member of the `west-admin-users` list,
thereby granting membership in the `west-admin-users` list to all members of
the `west-admins` list.

Note: an admin over the full `/ops` scope could have simply added the
`west-admins` list as a member of the existing `west-users` list, but as Alice
is only an admin over `/ops/west` she could only define the `west-admin-users`
list in the `/ops/west` scope.

### Overview

Scoped roles are currently assigned to users via `scoped_role_assigment`
resources that look like the following:

```yaml
kind: scoped_role_assignment
metadata:
  name: uuid1
scope: /ops
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
- `ops-admin` in scope `/ops/west`
- `ops-access` in scope `/ops`

Scoped access lists will not be an alternative to scoped role assignments,
rather, they will essentially be a way of automating the definition of scoped
role assignments for many users at once.

For example, if you wanted to make both `alice@example.com` and `bob@example.com`
admins over the `/ops/west` scope, you could define a scoped access list as follows:

```yaml
kind: scoped_access_list
metadata:
  name: west-admins
scope: /ops
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
kind: scoped_access_list_member
metadata:
  name: a3e64073-8980-41aa-a7a0-dd23de40c38e
scope: /ops
spec:
    access_list: west-admins
    name: alice@example.com
    membership_kind: user
version: v1
---
kind: scoped_access_list_member
metadata:
  name: c4c9d4b3-c57c-40de-a15e-e2425267a716
scope: /ops
spec:
  access_list: west-admins
  name: bob@example.com
  membership_kind: user
version: v1
```

Under the hood the scoped access cache would materialize this scoped access
list into the following scoped role assignments:

```yaml
kind: scoped_role_assignment
metadata:
  name: uuid1
scope: /ops
spec:
  user: alice@example.com
  assignments:
  - role: ops-admin
    scope: /ops/west
  - role: ops-access
    scope: /ops
status:
  origin:
    creator: scoped_access_list
    creator_name: west-admins
    date_created: "2025-12-18T04:00:00Z"
version: v1
---
kind: scoped_role_assignment
metadata:
  name: uuid2
scope: /ops
spec:
  user: bob@example.com
  assignments:
  - role: ops-admin
    scope: /ops/west
  - role: ops-access
    scope: /ops
status:
  origin:
    creator: scoped_access_list
    creator_name: west-admins
    date_created: "2025-12-18T04:00:00Z"
version: v1
```

These "materialized" scoped role assignments would be inserted into the scoped
role assignment cache.
Everything that already works with scoped role assignments today will not need
to know or care about scoped access lists, they will just handle the resulting
assignments as usual.

### Nested list membership

Scoped access list members can either be users or other scoped access lists.
When listA has member listB, it means that all members of listB are nested
members of listA.
In this case listA has delegated part of its membership definition to listB.
In general, a user is a nested member of listA if they are a direct member of
listA or they are a nested member of any list that is a direct member of listA.

For example, consider a scoped access list named `west-access` that grants
access to SSH servers in the `/ops/west` scope.
If an admin wanted to grant SSH access to all members of the `west-users` and
`west-admins` lists, they could make `west-users` and `west-admins` members of
the `west-access` list.
In doing this, the `west-access` list would be delegating the definition of
its user members to its member lists.
All members of `west-users` and `west-admins` would become nested members of the
`west-access` list.

To keep with hierarchical scope isolation rules, edits at a child scope must
not be able to cause privilege assignments in ancestor or orthogonal scopes.
This implies that member lists must be in the same scope or an ancestor scope
of their parent list.
For example, a list in `/ops/west` may only have member lists in `/ops/west` or `/ops`.

Without this restriction, if a list in `/ops/west` had a member list in
`/ops/west/vancouver`, then admins of `/ops/west/vancouver` would be able to
add arbitrary members and gain privileges in the parent scope `/ops/west`,
violating hierarchical scope isolation.

`scoped_access_list_member` resources will have a `membership_kind` field
specifying whether the member is a user or a nested list.

```
kind: scoped_access_list_member
metadata:
  name: member-resource-uuid
scope: /ops
spec:
  access_list: west-access
  name: west-admins
  membership_kind: list
version: v1
```

Nested list memberships effectively form a graph where nodes are scoped access
lists and edges are scoped access list memberships.
To avoid global locking and graph traversal on `scoped_access_list_member` writes:

- We will not attempt enforce a maximum depth of nested lists.
- Cycles will explicitly be allowed, that is, listA and listB are allowed to be
  members of each other.

The traditional access list implementation has attempted to enforce a maximum
depth and prevent cycles, but this has required a global backend lock, which in
practice has still been ineffective in enforcing consistency.
The team is in the process of removing the global lock and removing these invariants.
There is no real reason to disallow cycles, we can make sure that the
materialization algorithm handles them appropriately, cycles will result in a
stable union of members for all lists in the cycle.

### Materialization of scoped role assignments

The term "materialization" is used here to mean the computation and storage of
concrete scoped role assignments from their source of truth, which is the
current set of scoped access lists and their members.
Rather than referencing scoped access lists (and memberships) during login
events, the set of materialized scoped role assignments will be computed ahead
of time and stored in the scoped role assignment cache.

Every (user, list) pair, where user is a nested member of list, will result in
1 materialized scoped role assignment.
Each materialized assignment for (user, list) will grant exactly the roles and
scopes defined in the spec of that list.
The scope of the materialized assignment will be the same scope as the
justifying scoped access list.

For example, if alice is a direct member of listA, and listA is a direct member
of listB, then alice is a nested member of both listA and listB.
2 scoped role assignments will be materialized, one for (alice, listA) and
another for (alice, listB).

Note: if many users are members of many lists, this could result in a ton of
materialized assignments.
For example, if 20k users are all members of 1k lists, this would result in 20
million materialized scoped role assignments.
In general the number of assignments will be ~ (num users) x (avg lists per user).
We want to maintain the invariant that the scope of each materialized
assignment is the scope of the justifying scoped access list.
If scale is a concern here we could consider aggregating scoped role
assignments for each user by scope, so there would be at most 1 materialized
scoped role assignment for each (user, scope) pair.
In this case if 20k users were members of any number of lists across 20 scopes,
there would be 400k materialized assignments.
The downside here would be that the aggregated scoped roles assignments could
become large, and the source of the materialized assignment would be more
difficult to reason about.
The current plan is not to aggregate scoped role assignments.

Earlier in the design for scoped access lists there was discussion of writing
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
   faster.
7. Everything that needs to read scoped role assignments already needs to use
   the custom scoped access cache for scoped traversal, there are not going to
   be access paths that read directly from the backend and bypass the cache.

### Notable differences from traditional access lists

Traditional Teleport access lists are capable of some fairly advanced features,
including modifying user traits and delegating responsibility for management of
the list to owners who otherwise do not have access list modification/creation
permissions.

We will be initially omitting all traditional access list features not strictly
necessary for the goal of granting common sets of scoped privileges to groups
of users.
Additionally, some access list features are categorically incompatible with the
nature of scopes and scoped role assignment, and some features will need to
function differently under the hood.
Some notable differences include:

- There will be no support for access list owners.
  Scoped access lists will rely on standard scoping rules to determine who can
  manage them.
  A concept of owners may be added in the future, but would likely not add
  significant additional value when scoped admins can already manage access lists
  and role assignments directly.

- Scoped access lists will likely never support the concept of trait
  modification.
  Scoped trait modification would be an incredibly complex feature to
  implement, and likely would be of limited utility.

- Scoped access lists will not rely on login hooks to determine membership and
  apply privileges.
  Instead, membership will be determined asynchronously and flattened into a
  set of materialized `scoped_role_assignment`s.
  This is necessary to ensure that teleport can efficiently determine the full
  set of scoped privileges in user-facing hot paths, without the need to
  relogin.

- Scoped access lists will likely never support the concept of
  `membership_requires` or `ownership_requires` blocks.
  As noted above, scoped access lists need to be able to have membership
  determined reliably outside of login hooks (i.e. in scenarios where there is a
  high likelihood that a users traits are either unknown or outdated).

- Scoped access lists and associated resources will use a backend
  synchronization model based on AtomicWrite instead of RunWhileLocked.
  AtomicWrite is a significantly better choice from a correctness standpoint.
  Any operation protected by RunWhileLocked may fail part-way through.
  Additionally, the "lock" used by RunWhileLocked is itself fallible and
  time-based.
  Use of AtomicWrite will also allow us to avoid having a single global
  synchronization point for all list operations.
  Something that is a hard requirement in the scopes world, which is intended to
  support much broader distribution of admin privileges (and therefore much
  higher concurrency on admin APIs).

### Prerequisites/External Limitations

The current implementation of the `scoped_role_assignment` resource is too
strict/opinionated to work well with scoped access lists.

In particular, scoped access lists need to be able to assign roles defined in
ancestor scopes, which is not currently supported.

Additionally, the current implementation of `scoped_role_assignment` resources
does not have a well-defined model for handling invalid or dangling assignments
that may be possible while changes are propagating through the cache, such as
an assignment referencing a deleted role or attempting to assign it at a scope
that is no longer permissible.

While these limitations do not directly block scoped access list implementation,
scoped access lists will not be fully functional until they are addressed, and
may be error-prone in some edge cases.

We will loosen the restrictions on `scoped_role_assignment` resources in order
to allow assignment of roles defined in parent/ancestor scopes.

We will implement a model for handling invalid or dangling assignments.
All assignments will be checked when they are loaded, and invalid assignments
will be dropped.

### Identity provider sync

Teleport already has multiple integrations with third party identity providers
that automatically import groups and create traditional Access Lists to model
them in Teleport, with bidirectional sync.
These include AWS IAM Identity Center, Microsoft Entra ID, Okta, and SailPoint.

This part of the design for scoped access lists has not been thoroughly
explored yet, but in principal, each IdP integration that currently creates
traditional unscoped access lists could be configured to instead create scoped
access lists at a certain default scope or at a specific scope for each group.
This will need to be explored in detail for each IdP integration.
For features like automatic access requests to Okta apps or AWS IAM roles, we
will also need to wait for the scoped access model to implement support for
these features.

### Security

#### Scoping Rules

Scoped access lists will follow the standard scoping rules for their own
resource scope.
This means that a scoped access list with resource scope `/ops/west` will
only be editable by users with write access to the `scoped_access_list`
resource kind in `/ops` or `/ops/west`.

Scoped access lists will only be able to assign roles defined in their own
scope or an ancestor scope, and only assign said roles *to* their own scope or
a descendant scope.
These rules mirror the rules for `scoped_role_assignment` resources, which is
necessary as scoped access lists are, at their core, an alternative means of
managing a set of `scoped_role_assignment` resources.

When a scoped access list results in the materialization of a
`scoped_role_assignment` resource, the assignment will be created in the scope
of the access list itself.

#### Invariants

* Scoped access lists can only assign roles to their own scope or a descendent scope.

This can be validated purely from the scoped access list spec before storing.

* Scoped access lists can only assign scoped roles that exist
* Scoped access lists can only assign scoped roles that are defined in their
  own scope or an ancestor scope.
* Scoped access lists can only assign roles to an assignable_scope of the role.

Backend key `/scoped_role/role_lock/<role-name>` is already randomized every time a
scoped role assignment is created or deleted, and scoped role
Create/Update/Delete assert that it isn't concurrently modified.
Since scoped access lists are essentially a way of automating scoped role
assignments, they can use the same strategy.

ScopedAccessList Create and Update will check that all referenced scoped roles
exist, are defined in their own scope or an ancestor scope, and are assignable
at the assigned scope.
They will use AtomicWrite to assert the checked revision of each referenced
scoped role, and modify `/scoped_role/role_lock/<role-name>` for each referenced
role to make sure it isn't modified immediately after an access list create/update.
This requires 2 conditions per referenced role, which will limit the number of
roles a single scoped access list can reference.
This same limitation applies to scoped role assignments already, the current limit is 16.

UpdateScopedRole does not modify the role's scope, but may modify its
assignable scopes.
In this case it will check that the change doesn't invalidate any assignments
in extant scoped access lists, and will continue to use AtomicWrite to verify
that `/scoped_role/role_lock/<role-name>` hasn't concurrently changed.

DeleteScopedRole will continue to use AtomicWrite to verify that
`/scoped_role/role_lock/<role-name>` hasn't concurrently changed to verify that
no new scoped access lists reference it.

* Scoped access list members must only reference scoped access lists that exist.
* Nested scoped access list members must be in the same or ancestor scope of the parent list.

CreateScopedAccessListMember will check existence and scope of any referenced scoped access lists.
It will use AtomicWrite on their revision and modify `/scoped_access_list/member_lock/<list-name>`.

DeleteScopedAccessList will check there are no extant members, and use
AtomicWrite to verify that `/scoped_access_list/member_lock/<list-name>` is not concurrently modified.

Scope of scoped access lists can not be modified.

* Materialized scoped role assignments will be created in the scope of the justifying access list.
* Materialized scoped role assignments will be created for each extant (user, list) where user is a member of list.
* Materialized scoped role assignments will be deleted when user ceases to be a member of a list.

These are not being written to the backend and thus cannot use AtomicWrite.
These invariants will be enforced when the scoped access cache is initialized,
and when scoped access lists and their members are created/updated/deleted.

### Privacy

Scoped access list member resources will reference usernames, but this is
already true of scoped role assignments.

### Proto Specification

```proto3
// ScopedAccessList defines a set of grants that will be applied to all members of the list.
message ScopedAccessList {
  // Kind is the resource kind.
  string kind = 1;

  // SubKind is the resource sub-kind.
  string sub_kind = 2;

  // Version is the resource version.
  string version = 3;

  // Metadata contains the resource metadata.
  teleport.header.v1.Metadata metadata = 4;

  // Scope is the scope of the access list resource.
  string scope = 5;

  // Spec is the access list specification.
  ScopedAccessListSpec spec = 6;
}

// ScopedRoleAssignmentSpec is the specification of a scoped access list.
message ScopedAccessListSpec {
  // title is a plaintext short description of the Access List.
  string title = 1;
  // description is an optional plaintext description of the Access List.
  string description = 2;
  // grants describes the access granted by membership to this Access List.
  ScopedAccessListGrants grants = 3;
}

// ScopedAccessListGrants describes what access is granted by membership to the Access
// List.
message ScopedAccessListGrants {
  // scoped_roles are the scoped roles that are granted to users who are
  // members of the Access List.
  repeated ScopedRoleGrant scoped_roles = 1;
}

// ScopedRoleGrant describes a scoped role granted at a specific scope.
message ScopedRoleGrant {
  // role is the name of the granted role. The scope of the role must be the
  // scope of the access list or an ancestor scope.
  string role = 1;
  // scope is the scope the role will be assigned at. It must be the scope of
  // the access list or a descendant scope.
  string scope = 2;
}

// ScopedAccessListMember describes a member of a scoped access list.
message ScopedAccessListMember {
  // Kind is the resource kind.
  string kind = 1;

  // SubKind is the resource sub-kind.
  string sub_kind = 2;

  // Version is the resource version.
  string version = 3;

  // Metadata contains the resource metadata.
  teleport.header.v1.Metadata metadata = 4;

  // Scope is the scope of the access list member resource. It must match the
  // scope of the scoped access list.
  string scope = 5;

  // spec is the specification for the scoped access list member.
  ScopedAccessListMemberSpec spec = 6;
}

// ScopedAccessListMemberSpec is the specification for the scoped access list member.
message ScopedAccessListMemberSpec {
  // access_list is the associated scoped access List.
  string access_list = 1;

  // name is the name of the member of the scoped access List.
  string name = 2;

  // membership_kind describes the type of membership.
  MembershipKind membership_kind = 3;
}

// MembershipKind represents the different kinds of list membership.
enum MembershipKind {
  // MEMBERSHIP_KIND_UNSPECIFIED represents list members that are of
  // unspecified membership kind, defaulting to being treated as type USER.
  MEMBERSHIP_KIND_UNSPECIFIED = 0;
  // MEMBERSHIP_KIND_USER represents list members that are normal users.
  MEMBERSHIP_KIND_USER = 1;
  // MEMBERSHIP_KIND_LIST represents list members that are nested Access Lists.
  MEMBERSHIP_KIND_LIST = 2;
}

// ScopedAccessService provides an API for managing scoped access-control resources.
service ScopedAccessService {
  // Existing RPCs...

  // GetScopedAccessList gets a scoped access list by name.
  rpc GetScopedAccessList(GetScopedAccessListRequest) returns (GetScopedAccessListResponse);

  // ListScopedAccessLists returns a paginated list of scoped access lists.
  rpc ListScopedAccessLists(ListScopedAccessListsRequest) returns (ListScopedAccessListsResponse);

  // CreateScopedAccessList creates a scoped access list.
  rpc CreateScopedAccessList(CreateScopedAccessListRequest) returns (CreateScopedAccessListResponse);

  // UpdateScopedAccessList updates a scoped access list.
  rpc UpdateScopedAccessList(UpdateScopedAccessListRequest) returns (UpdateScopedAccessListResponse);

  // DeleteScopedAccessList deletes a scoped access list.
  rpc DeleteScopedAccessList(DeleteScopedAccessListRequest) returns (DeleteScopedAccessListResponse);

  // GetScopedAccessListMember gets a scoped access list member by name.
  rpc GetScopedAccessListMember(GetScopedAccessListMemberRequest) returns (GetScopedAccessListMemberResponse);

  // ListScopedAccessListMembers returns a paginated list of scoped access list members.
  rpc ListScopedAccessListMembers(ListScopedAccessListMembersRequest) returns (ListScopedAccessListMembersResponse);

  // CreateScopedAccessListMember creates a scoped access list member.
  rpc CreateScopedAccessListMember(CreateScopedAccessListMemberRequest) returns (CreateScopedAccessListMemberResponse);

  // DeleteScopedAccessListMember deletes a scoped access list member.
  rpc DeleteScopedAccessListMember(DeleteScopedAccessListMemberRequest) returns (DeleteScopedAccessListMemberResponse);
}

// GetScopedAccessListRequest is a request for a scoped access list.
message GetScopedAccessListRequest {
  // Name is the name of the scoped access list.
  string name = 1;
}

// GetScopedAccessListResponse is a response for a scoped access list.
message GetScopedAccessListResponse {
  // list is the scoped access list.
  ScopedAccessList list = 1;
}

// ListScopedAccessListsRequest is a request to list scoped access lists.
message ListScopedAccessListsRequest {
  // page_size is the maximum number of results to return.
  int32 page_size = 1;

  // page_token is the pagination cursor used to start from where a previous request left off.
  string page_token = 2;

  // Filtering options TBD, probably at least by scope and by membership.
}

// ListScopedAccessListsResponse is a response listing scoped access lists.
message ListScopedAccessListsResponse {
  // lists is the list of scoped access lists.
  repeated ScopedAccessList lists = 1;
  // next_page_token is a pagination cursor usable to fetch the next page of results.
  string next_page_token = 2;
}

// CreateScopedAccessListRequest is a request to create a scoped access list.
message CreateScopedAccessListRequest {
  // list is the scoped access list.
  ScopedAccessList list = 1;
}

// CreateScopedAccessListResponse is a response to creating a scoped access list.
message CreateScopedAccessListResponse {
  // list is the scoped access list.
  ScopedAccessList list = 1;
}

// UpdateScopedAccessListRequest is a request to update a scoped access list.
message UpdateScopedAccessListRequest {
  // list is the scoped access list.
  ScopedAccessList list = 1;
}

// UpdateScopedAccessListResponse is a response to updating a scoped access list.
message UpdateScopedAccessListResponse {
  // list is the scoped access list.
  ScopedAccessList list = 1;
}

// DeleteScopedAccessListRequest is a request to delete a scoped access list.
message DeleteScopedAccessListRequest {
  // name is the name of the scoped access list to delete.
  string name = 1;

  // revision asserts the revision of the scoped access list to delete (optional).
  string revision = 2;
}

// DeleteScopedAccessListResponse is a response to deleting a scoped access list.
message DeleteScopedAccessListResponse {}

// GetScopedAccessListMemberRequest is a request for a scoped access list member.
message GetScopedAccessListMemberRequest {
  // scoped_access_list is the name of the scoped access list that the member belongs to.
  string scoped_access_list = 1;
  // member_name is the name of the member (user or scoped access list) that belongs to the scoped access list.
  string member_name = 2;
}

// GetScopedAccessListMemberResponse is a response for a scoped access list.
message GetScopedAccessListMemberResponse {
  // member is the scoped access list member.
  ScopedAccessListMember member = 1;
}

// ListScopedAccessListMembersRequest is a request to list all scoped access list members.
message ListScopedAccessListMembersRequest {
  // page_size is the maximum number of results to return.
  int32 page_size = 1;

  // page_token is the pagination cursor used to start from where a previous request left off.
  string page_token = 2;

  // Filtering options TBD, probably at least by scoped access list and member.
}

// ListScopedAccessListMembersResponse is a response listing scoped access list members.
message ListScopedAccessListMembersResponse {
  // members is the list of scoped access list members.
  repeated ScopedAccessListMember members = 1;
  // next_page_token is a pagination cursor usable to fetch the next page of results.
  string next_page_token = 2;
}

// CreateScopedAccessListMemberRequest is a request to create a scoped access list member.
message CreateScopedAccessListMemberRequest {
  // member is the scoped access list member.
  ScopedAccessListMember member = 1;
}

// CreateScopedAccessListMemberResponse is a response to creating a scoped access list member.
message CreateScopedAccessListMemberResponse {
  // member is the scoped access list member.
  ScopedAccessListMember member = 1;
}

// DeleteScopedAccessListMemberRequest is a request to delete a scoped access list member.
message DeleteScopedAccessListMemberRequest {
  // scoped_access_list is the name of the scoped access list the member is of.
  string scoped_access_list = 1;
  // member_name is the name of the member to delete.
  string member_name = 2;

  // revision asserts the revision of the scoped access list member to delete (optional).
  string revision = 3;
}

// DeleteScopedAccessListMemberResponse is a response to deleting a scoped access list member.
message DeleteScopedAccessListMemberResponse {}
```

### Scale

The current design materializes scoped role assignments for each (user, list)
pair where user is a member of list, this could scale to large numbers of
resources that will consume memory on the auth service.
This is discussed in [Materialization of scoped role assignments](#materialization-of-scoped-role-assignments).

### Backward Compatibility

This is a greenfield feature with limited backward compatibility considerations.

### Audit Events

Audit events will be emitted when scoped access lists are created, updated, or deleted.

### Observability

Traces will be added for scoped role assignment materialization to monitor performance.
Metrics will be added for the number of scoped access lists, scoped access list
members, and materialized scoped role assignments.

### Product Usage

TBD, scopes are currently rolling out to limited design partners.

### Test Plan

Scoped access lists will be added to the test plan.
