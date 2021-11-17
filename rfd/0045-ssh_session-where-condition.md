---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: draft
---

# RFD 45 - RBAC `where` conditions for active sessions list/read

## What

Manage access to active sessions (resource kind `ssh_session`) by RBAC
`where` conditions, in the same manner as the RFD 44 *RBAC `where` conditions
for session recordings list/read* provides access management for session
recordings (resource kind `session`).

## Why

To be able to restrict access of certain users to only a subset of active
sessions, notably only their own active sessions.

## Details

### Work around implicit allow rule

Unlike `session`, the `ssh_session` kind is referred to by an [implicit rule](https://github.com/gravitational/teleport/blob/36998cf5669ebda59190453d265e2df24161992b/lib/services/role.go#L57)
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
      where: '!contains(ssh_session.parties, user.metadata.name)'
```

### Replacing parties by usernames

The `ssh_session` identifier is used to expose the fields of the
[`session.Session` struct](https://github.com/gravitational/teleport/blob/36998cf5669ebda59190453d265e2df24161992b/lib/session/session.go#L84).

In general, the RBAC `contains` predicate can only be used to detect the
occurrence of a string in a slice of strings.  However, the `parties` field of
`session.Session` is a slice of more complex `Party` objects, not strings.
To support `where` conditions as above, a list of usernames is extracted from
the original `Party` slice in order that the usernames be then bound to
`ssh_session.parties` instead.
