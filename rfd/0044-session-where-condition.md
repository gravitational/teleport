---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: implemented
---

# RFD 44 - RBAC `where` conditions for session recordings list/read

## What

The `where` condition in RBAC rules for session recordings should be allowed to refer to the session recording as in:

```yaml
spec:
  allow:
    rules:
    - resources: [session]
      verbs: [list, read]
      where: contains(session.participants, user.metadata.name)
```

For list requests, the condition should become part of the query sent to the backend.

## Why

To be able to provide users with the capability to view recordings of the sessions they have participated in, without having to be granted the privilege to view all session recordings.

## Details

### Subconditions involving `session`

A new identifier `session` for use in `where` clauses is introduced.

For read requests, any occurrence of the identifier refers to the `session.end` event associated with the requested session ID -- the whole `where` condition is then evaluated with this binding. If the condition is satisfied, the read request continues with a query to the backend.

For list requests, the `where` condition cannot be evaluated as is since there is no particular event to bind to the `session` identifier. Instead, the largest admissible subcondition involving `session` shall be extracted and passed to the backend as an additional filtering condition. For example, consider the `where` condition:
```
(contains(session.participants, user.metadata.name) && !equals(user.metadata.name, "blocked"))
  || equals(user.metadata.name, "admin")
```
* If `user.metadata.name` is equal to `"admin"`, the largest admissible subcondition would be `true` -- the list query would be performed on the backend with no additional filtering conditions.
* If `user.metadata.name` is equal to `"blocked"`, the largest admissible subcondition would be `false` -- the whole request would be rejected with `AccessDenied`.
* Otherwise, the largest admissible subcondition would be `contains(session.participants, user.metadata.name)` -- the list query would be performed on the backend with this additional filtering condition applied.

### API

As it stands, the list of session recordings is obtained by searching (`SearchEvents`) for `session.end` events. This already makes it difficult to follow the principle of least privilege since the call [requires the ability to list all events](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/auth/auth_with_roles.go#L2998), not only the session-related ones.

The [method `SearchSessionEvents`](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/events/api.go#L614-L622) appears to have been meant to address this privilege creep: it [checks privileges for `KindSession`](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/auth/auth_with_roles.go#L3012) instead of `KindEvent`. The Proxy Web UI shall be updated to call `SearchSessionEvents` instead of `SearchEvents` when building lists of session recordings.

Moreover, `SearchSessionEvents` shall be modified to search only for `session.end` events (instead of both `session.start` and `session.end`) and to accept an additional parameter for the `where` subcondition described above.

### Backends

#### DynamoDB

Currently, the fields of an event are stored as a JSON string in the `Fields` attribute. To allow for efficient filtering, the fields should be stored as a proper DynamoDB map instead since DynamoDB allows to refer to map elements inside the filter/condition expressions (e.g. `contains(FieldsMap.participants, :participant)`). This will also enable efficient event-specific queries for any other use case in the future.

The whole event table shall therefore be migrated so that the current `Fields` (DynamoDB string) is converted into a new `FieldsMap` attribute (DynamoDB map).

The received `session`-related subcondition shall be converted into a DynamoDB filter expression.

#### Firestore

The Firestore event table should be migrated to store event fields as a native map type. The schema should be extended with any necessary indexes to allow for backend-side filtering by both the event type and the fields map attributes.

The received `session`-related subcondition shall be converted into a Firestore query object.

#### File log

The event file is processed and the individual entries unmarshaled at the Auth server.

The received `session`-related subcondition shall be converted into a boolean function applicable to the unmarshaled event fields. 
