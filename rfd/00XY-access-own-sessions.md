---
authors: Andrej Tokarčík (andrej@goteleport.com)
state: draft
---

# RFD XY - Users accessing their own SSH sessions

## What

Notwithstanding their actual RBAC privileges, every user should be guaranteed the ability to:
1. join the active SSH sessions they are parties to;
2. view recordings of the SSH sessions they have participated in.

Moreover, if the relevant privileges are in fact missing, the user should not be authorized to join or view any other sessions beyond the scope of (1) and (2).

## Why

Privileges in Teleport are structured as verb--resource pairs (e.g., `VerbList` for `KindSSHSession`) where the verb action applies to *all* resources of the kind.

Unlike many other resources, SSH sessions are associated with a set of users through the notion of "party" or "participant". It is desirable to reflect this session--user relation in the authorization process so that users are always able to access their own sessions without being required to have access to all sessions/recordings indiscriminately.

## Details

### Joining SSH sessions involving self

As it stands, every user is able to join any active session since `VerbList` and `VerbRead` for `KindSSHSession` are [granted as part of the default implicit role](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/services/role.go#L79) that gets added to all role sets.
```go
  types.NewRule(types.KindSSHSession, RO()),
```
This shall be removed, with the result that no `KindSSHSession` privileges are granted implicitly.

The signature of [`GetSessions` from `session.Service`](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/session/session.go#L231-L233) shall be extended with a new parameter `withParticipant`:
```go
  GetSessions(namespace string, withParticipant string) ([]Session, error)
```
When `withParticipant` is nonempty, the method should return only those sessions that involve [a party whose username](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/session/session.go#L130-L131) is equal to `withParticipant`.

The method's [wrapper in `auth.ServerWithRoles`](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/auth/auth_with_roles.go#L206-L212) shall perform the following logic:
1. Check `VerbList` for `KindSSHSession`. If successful, call the inner API method.
2. If the RBAC check failed:
   1. If `withParticipant` is empty, set `withParticipant` to the current user and call the inner API method.
   1. If `withParticipant` is set to the current user, call the inner API method.
   1. If `withParticipant` is set to a different value, return the RBAC failure.
Note that the current user may request `GetSessions` for himself even with no privileges.

### Playing SSH session recordings involving self

As it stands, the list of session recordings is obtained by searching (`SearchEvents`) for `session.end` events. This already makes it difficult to follow the principle of least privilege since the call [requires the ability to list all events](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/auth/auth_with_roles.go#L2998), not only the session-related ones.

The [method `SearchSessionEvents`](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/events/api.go#L614-L622) appears to have been meant to address this privilege creep: it [checks privileges for `KindSession`](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/auth/auth_with_roles.go#L3012) instead of `KindEvent`. However, in its current form `SearchSessionEvents` searches not only for `session.end` [but also for `session.start`](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/lib/events/dynamoevents/dynamoevents.go#L967-L971) which might be the reason that `SearchSessionEvents` is preferred for the purpose of listing session recordings.

`SearchSessionEvents` shall be adapted to search only for `session.end` events and also to accept an additional parameter `withParticipant`. When `withParticipant` is nonempty, the method should return only those `session.end` events that involve a [participant](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/api/types/events/events.proto#L306-L307) equal to `withParticipant`. Like in the case of `GetSessions`, the RBAC check in `auth.ServerWithRoles` can result in resetting `withParticipant` to the current user, making sure the user is able to view their own recordings.

The Proxy Web UI should be updated to call `SearchSessionEvents` instead of `SearchEvents` when showing the list of session recordings.

The `withParticipant` filtering should be applied alongside the already-implemented `eventTypes` filtering, with the efficiency varying according to the storage backend.

#### DynamoDB

To allow for efficient filtering, the event type information is not stored only in the marshaled `Fields` (that can be filtered by only after unmarshaling on the Auth server side) but also pulled out into an item attribute `EventType` (that DynamoDB is aware of and can be filtered by on its side). Analogously, the participant information specific to `session.end` should be extracted into a separate string-set attribute `Participants`.

With a separate attribute it becomes possible to construct a `FilterExpression` requesting only those `session.end` events whose `Participants` [contain](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.ConditionExpressions.html#Expressions.ConditionExpressions.CheckingForContains) a particular user. The [logic provided by `searchEventsRaw`](https://github.com/gravitational/teleport/blob/992c10f547a6b7c24247835d7711fadb46ad9022/lib/events/dynamoevents/dynamoevents.go#L805-L810) makes sure an expected number of matching items is returned for a given `FilterExpression`, working around DynamoDB's underlying [limitations in this regard](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Query.html#Query.Limit).

Properly supporting the new filter will require a migration procedure for the event table:

1. Check if there is a `session.end` event stored without the `Participants` attribute, e.g. using a probe like:
   ```
   aws dynamodb scan \
      --table-name Events \
      --filter-expression "EventType = :name AND attribute_not_exists(Participants)" \
      --expression-attribute-values '{":name":{"S":"session.end"}}'
   ```
1. If there is no such event, i.e. all stored `session.end` events already come with `Participants`, there is nothing to do.
1. If such events exist, for each `session.end` event lacking the `Participants` attribute:
   1. Get the list of participants from the event's `Fields` and store it as a string set under the `Participants` attribute.
   1. Reupload the event similarly to [`migrateDateAttribute`](https://github.com/gravitational/teleport/blob/992c10f547a6b7c24247835d7711fadb46ad9022/lib/events/dynamoevents/dynamoevents.go#L1170).

#### Firestore

Our Firestore support filters event types by going through all the events (within a time range) and [unmarshaling the `Fields` on the Auth server side](https://github.com/gravitational/teleport/blob/992c10f547a6b7c24247835d7711fadb46ad9022/lib/events/firestoreevents/firestoreevents.go#L539-L550). Filtering by participants contained in the `session.end` fields can be easily performed in the same fashion.

#### File log

In order to filter by event type, the event file is processed and the individual entries unmarshaled at the Auth server. It should be straightforward to extend the procedure to support another field to filter by.

### Remarks

No new `teleport.yaml` options need to be introduced.

Some Teleport deployments may depend on the legacy behavior of all users being able to join active sessions thanks to the implicit `KindSSHSession` privilege. However, since showing all active sessions to all users constitutes a potential security risk anyway, it should be preferable to get rid of the implicit privilege even in spite of the possibility of breaking an established use case.

With this RFD it becomes impossible to make a user's own session recordings unavailable to the user as the RBAC checks get skipped in those scenarios.

## Future work

### Proxy web UI

There is an [advanced filtering feature in the works](https://github.com/gravitational/teleport/issues/8155) with the goal to expose the event-type filtering functionality to the user through a checkbox of all the known event types. In a similar manner an autocomplete input field or a dynamic checkbox could be added to the "Session Recordings" screen to allow filtering by session participants. As for users lacking the privileges this widget would provide a useful hint about only their own sessions being shown, as they would be able to see their own username "hard-coded" in the filter.

### Event access control

The present solution should be seen as a preliminary measure tied with the limitations of Teleport's current schemas and/or the less expressive storage backends currently supported by Teleport. Ultimately, a full-fledged [event access control support](https://github.com/gravitational/teleport/issues/5430) should be implemented.
