---
title: RFD 0170 - Nested Access lists
authors: Alex McGrath (alex.mcgrath@goteleport.com)
state: in development
---

# Related
  * Partial implementation PR: https://github.com/gravitational/teleport/pull/38738

# What

This RFD proposes adding support for including owners and
members in Access Lists from other Access Lists

# Why

This allows a hierarchical Access List structure for organizations
which is supported by Access List implementations being integrated
with. For example, Azure Entra supports including groups in other
groups, which we want to be able to mimic the structure of in Teleport
Access Lists.

Users in an access list hierarchy will inherit the granted roles and
traits for members and owners from access lists referencing the lists
they're in.

# Implementation

The `AccessListMember` type will be modified to include a new field:

```go
type AccessListMemberSpec struct {
  // ...

  // MembershipKind describes the kind of membership,
  // either "MEMBERSHIP_KIND_USER" or "MEMBERSHIP_KIND_LIST".
  MembershipKind string `json:"membership_kind" yaml:"membership_kind"`
}
```

As well as `accesslist.Owner`:

```go
type Owner struct {
  // ...

  // MembershipKind describes the kind of ownership,
  // either "MEMBERSHIP_KIND_USER" or "MEMBERSHIP_KIND_LIST".
  MembershipKind string `json:"membership_kind" yaml:"membership_kind"`
}
```

These fields will be used to indicate whether the member/owner refers to
a Teleport user or an Access List.

Two fields will be added to the `AccessList.Status` struct as well:

```go
type Status struct {
	// ...
	OwnerOf []string `json:"owner_of" yaml:"owner_of"`
	MemberOf []string `json:"member_of" yaml:"member_of"`
}
```

These fields will be used to store the names (UUIDs) of Access Lists which
the current Access List is an explicit member or owner of, respectively.

# Implementation considerations

## Limitations

### Cycles

The implementation will not support cycles within the hierarchy, as
this would introduce confusing options for configuration. Teleport
will return an error if self-referential membership or ownership is
detected while modifying an Access List's members or owners.

### Nesting depth

The implementation will not support a hierarchy that is
more than 10 levels deep. An error will be returned if a hierarchy
exceeds this depth while modifying an Access List's members or owners.
This may be configured to a higher value in the future.

Errors relating to cycles and depth will also be detected and returned
at Access List insertion time.

## Access List reviews

In the member review page for an Access List review, the list of members
will include both explicit members and nested Access Lists. An indicator
will be included to show that the member is an Access List, not an
individual user.

## Impact on Access Requests

Access Requests' suggested reviewers will include users with inherited
ownership from nested Access Lists.

The Suggested Lists field will remain operating as it presently does,
only showing the list that actually grants the resource.

## Membership and Ownership

### Inheritance

Membership will be inherited when a user is a member in an Access List that
is a member of another Access List.

Ownership will be inherited when a user is a member in an Access List that
is an owner of another Access List.

These rules are recursive - if an Access List is a member of an
Access List that is a member of another Access List, the user will
inherit membership in all three Access Lists (or, ownership, if the
first Access List in the chain is added as an owner).

### Requirements

Membership and ownership requirements will be checked at each level of the
hierarchy. If a user does not meet the requirements at a given level,
they will not inherit membership or ownership from that level or any
levels "above" it in the hierarchy.

## Grant inheritance

When calculating a user's login state, the tree of Access Lists will
be traversed, granting roles and traits from each Access List in the
hierarchy that the user has either explicit or inherited membership or
ownership in.

Explicit owners will not inherit roles or traits from any nested
Access Lists they own. They will only be granted roles and traits from
the Access List(s) they are explicit owners of.

## Other considerations

Access lists will need to be allowed to have empty grants so Access
Lists can represent only users, and permissions can be assigned purely
through inclusion in other Access Lists.

# Example

- User Alice is a member in `acl-a`, which does not include any other
Access Lists as members:

```yaml
kind: access_list
metadata:
  name: acl-a
spec:
  grants:
    roles:
    - some-role
    traits: {}
  title: access-list-a
version: v1
```

- `acl-c` includes members of `acl-a`, and grants them `manager`
- `acl-b` includes members of `acl-c`, and grants them `auditor` and `reviewer`

```yaml
kind: access_list
metadata:
  name: acl-b
spec:
  grants:
    roles:
    - auditor
    - reviewer
    traits: {}
  # In actuality, members are not stored directly on the Access List resource, this is just for brevity
  members:
    - acl-c
  title: access-list-a
version: v1
---
kind: access_list
metadata:
  name: acl-c
spec:
  grants:
    roles:
    - manager
    traits: {}
  members:
    - acl-a
  title: access-list-a
version: v1

```

When calculating a user's access roles, the tree of Access Lists will
be traversed, and so, upon login, user Alice will be granted:

- `some-role` from `acl-a`
- `manager` from `acl-c`, as it includes members from `acl-a`
- `auditor` and `reviewer` from `acl-b`, as it includes members from
  `acl-c`, which in turn includes members from `acl-a`

# Future considerations

- Further development of membership reviews should be considered to
expand reviews to include members of nested lists and to include
information in the UI that nested lists may need to be separately reviewed.
