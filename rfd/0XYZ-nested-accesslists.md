---
title: RFD 0XYZ - Nested Access lists
authors: Alex McGrath (alex.mcgrath@goteleport.com)
state: in development
---

# Related
  * Partial implementation PR: https://github.com/gravitational/teleport/pull/38738

# What

This RFD proposes adding support for including owners and
members in access lists from other access lists

# Why

This allows a hierarchical access list structure for organizations
which is supported by access list implementations being integrated
with, for example Azure Entra supports including groups in other
groups which we want to be able to mimic the structure of in Teleport
access lists.

Users in an access list hierarchy will inherit the granted roles and
traits for members and owners from access lists referencing the lists
they're in.

# Implementations

New fields will be introduced into the access_list type:

```yaml
kind: access_list
metadata:
  name: e69aa529-2a7f-4de2-87be-97fd94309a9f
spec:
  grants:
    roles:
    - access
    - auditor
    traits: {}
  # list of references to other access lists, for users to include in this access list
  member_access_lists:
  - name: ea4cbbc7-bee1-49b3-bf78-734b4b27ea38
  # list of references to other access lists, for owners to include in this access list
  owner_access_lists:
  - name: 3e9df1e7-0b8a-4984-b2e8-5bc0d7b356a9
  title: access-list-a
version: v1
```

These fields will contain a list of other access lists that will be
included in the access list.

# Implementation considerations

The implementation will not support cycles within the heirarchy as
this would introduce confusing options for configuration. Teleport
will return an error if a cycle are introduced.

Errors over cycles in the heirarchy will be detected and returned at
access list insertion/update time.

Access list heirarchys will only recurse up to 10 layers deep
initially.

Access lists will need to be allowed to have empty grants so access
lists can represent only users and permisisons can be assigned purely
through membership in other lists.

Access list periodic reviews will include in the member review page,
the list of nested access lists and an indicator to suggest that its
an access list not an individual member, but not the full list of
users.

# Examples


User Alice is in the below access list which does not include any
other access lists

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
- `acl-c` includes members in `acl-a`, grants them `manager`
- `acl-b` includes members in `acl-c`, and grants them `auditor` and `reviewer`

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
  # list of references to other access lists, for users to include in this access list
  member_access_lists:
  - name: acl-c
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
  # list of references to other access lists, for users to include in this access list
  member_access_lists:
  - name: acl-a
  title: access-list-a
version: v1

```

When calculating a users access roles, the tree of access lists will
be traversed and so upon login, user Alice will have been granted

- `some-role` from `acl-a`
- `manager` from `acl-c` as it includes members of `acl-a`
- `auditor` and `reviewer` from `acl-b` as it includes members of
  `acl-c`, who also include members of `acl-a`
