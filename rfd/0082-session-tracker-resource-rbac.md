---
authors: Joel Wejdenstål (jwejdenstal@goteleport.com)
state: implemented
---

# RFD 82 - Session Tracker Resource RBAC

## What

Introduce a new resource kind `session_tracker` that may be used in the RBAC verb system
to manage access for existing session tracker resources using list/read resource verbs in addition
to dynamic `where` conditions in the same manner as RFD 44.

## Why

Session tracker visibility is currently tightly coupled with the permission to join a given session.
They are invisible by default and may only be read by users with permission to join a session in some mode
using the `join_sessions` primitive from Moderated Sessions. The functionality proposed by this RFD will allow
one to configure access to list/read session tracker resources without being able to join the sessions in question.

## Details

### API

Only the `list` and `read` verbs are supported for session tracker resources. Supporting other verbs makes little
sense as tracker resources may only be safely managed by the node that created them. The `list` verb will allow access
to the `GetActiveSessionTrackers` RPC while `read` will provide access to the `GetSessionTracker` RPC.

The `where` clause may be used to specify an expression that will be used to selectively filter or allow/deny a request
based on tracker contents.

The environment in which the expression will execute defines the following variables derived from the tracker object and request user:
- `user`: standard user object as defined in RFD 44
- `tracker`: object
  - `session_id`: string
  - `kind`: string
  - `participants`: array of strings
  - `state`: string
  - `hostname`: string
  - `address`: string
  - `login`: string
  - `cluster`: string
  - `kube_cluster`: string
  - `host_user`: string
  - `host_roles`: array of strings

Configuration example that denies list/read access to all sessions you are a participant of:

```yaml
spec:
  deny:
    rules:
    - resources: [session_tracker]
      verbs: [list, read]
      where: 'contains(tracker.participants, user.metadata.name)'
```

### Effects on the default `auditor` role

To preserve earlier functionality from the legacy session metadata system, we should allow the auditor to list all active sessions by default but without the ability to join them. This involves adding this section to the predefined role definition:

```yaml
spec:
  allow:
    rules:
    - resources: [session_tracker]
      verbs: [list, read]
```

For existing clusters, we preserve the current auditor role if the user has modified it from the preset definition.
If it has not been modified, we append the above configuration rule to the role to which will put the functionality on par with Teleport 9. We will check this by comparing the deserialized role definition against the template minus the new rule.
