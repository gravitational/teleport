---
author: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD 175 - Static Host Users

## Required Approvers

## What

teleport nodes will be able to create host users ahead of time instead of only
when a user logs in

## Why

users can be provisioned w/o them needing to log in beforehand

## Details

### new resource

copy a lot of stuff for host users from roles

```yaml
kind: static_host_user
metadata:
    name: hostuser
spec:
    login: user1 # don't support the templating thing from roles as
    # that depends on users i think
    # these 2 are identical to their role counterparts
    groups: [abc, def]
    sudoers: [
        # ...
    ]
    # from user traits
    uid: 1234
    gid: 1234
    node_labels: # same as allow rules in roles
        # ...
    node_labels_expression: | # same deal
        # ...
    # we do not need host user mode as it will always be keep
```

### propagation

nodes will get all static host users on startup and create the ones that apply to them (check with the labels).
after that nodes will watch for new host users in their cache

they will get updated users this way too

nodes with host user creation disabled don't do anything

### deletion

key under login/name in backend so we have login info for deletion

### UX

### security

need to make sure that users not added by teleport can't be deleted by a user that has access to
host user resources. add a new group `teleport-created` (like `teleport-system`) that simply marks
that a user was created by teleport. only users with this group can be deleted (TODO: should host
users created the normal way get this group too?)

if a user is not allowed to create host users in their roles, they are also not allowed to create
static host users (maintain parity between static and dynamic host users as much as possible).
also user must have matching login to be able to work with matching static host user (TODO: consider if
there should be an admin bypass.)
