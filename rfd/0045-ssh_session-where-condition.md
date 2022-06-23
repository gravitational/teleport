---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: implemented
---

# RFD 45 - RBAC `where` conditions for active sessions list/read

## What

Manage access to active sessions (resource kind `ssh_session`) by RBAC
`where` conditions, in the same manner as the RFD 44 *RBAC `where` conditions
for session recordings list/read* provides access management for session
recordings (resource kind `session`).

These deny checks are to be employed on top of the new RBAC rules for listing and joining sessions introduced in [RFD 43](https://github.com/gravitational/teleport/blob/master/rfd/0043-kubeaccess-multiparty.md). This means that the user must pass both the resource checks introduced in this RFD and the RBAC `join_policy` checks from RFD 43 in order to join a session.

## Why

To be able to restrict access of certain users to only a subset of active
sessions, notably only their own active sessions.

## Details

### Work around implicit allow rule

Unlike `session`, the `ssh_session` kind is referred to by an [implicit rule](
https://github.com/gravitational/teleport/blob/066f0dbbad801ad822b81482948732641773d8fe/lib/services/role.go#L57)
granting unrestricted list/read privileges to all users:

```go
	types.NewRule(types.KindSSHSession, RO()),
```

Adding a `where` section to an explicit allow rule for `ssh_session` would
therefore take no effect.  To restrict access to active sessions, one has to
add (the negation of) the desired condition to a deny rule, as those are
applied earlier than allow rules:

```yaml
spec:
  deny:
    rules:
    - resources: [ssh_session]
      verbs: [list, read]
      where: '!contains(ssh_session.participants, user.metadata.name)'
```

### `ssh_session` identifier API

The `ssh_session` identifier exposes the private `lib.session.ctxSession`
struct. `ctxSession` is a subset of [`session.Session`](
https://github.com/gravitational/teleport/blob/066f0dbbad801ad822b81482948732641773d8fe/lib/session/session.go#L84)
with the addition of the `participants` field.

In general, the RBAC `contains` predicate can only be used to detect the
occurrence of a string in a slice of strings. However, the `Parties` field of
`session.Session` is a slice of more complex `Party` objects, not strings.
To support `where` conditions as above, a list of usernames is extracted from
the original `Party` slice in order that the usernames be then bound to
`ssh_session.participants` instead.
