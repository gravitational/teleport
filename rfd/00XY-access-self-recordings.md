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

A new `events.IAuditLog` method called `SearchCompletedSessions` (also to be available via Auth gRPC and Proxy Web API) shall be introduced. Its purpose is to search for `session.end` events while requiring `VerbList` for `KindSession`. The Proxy Web UI should be updated to take advantage of this specialized API method when generating lists of session recordings.

While relatively similar to `SearchSessionEvents` the new method shall accept an additional parameter `withParticipant`. When `withParticipant` is nonempty, the method should return only those `session.end` events that involve a [participant](https://github.com/gravitational/teleport/blob/ab57eab5c059b323e4fb50cf02c1134745a19dd1/api/types/events/events.proto#L306-L307) equal to `withParticipant`. Like in the case of `GetSessions`, the RBAC check in `auth.ServerWithRoles` can result in resetting `withParticipant` to the current user, making sure the user is able to view their own recordings.

Note that the participant filtering is applied just to a "page" of events returned from the backend. However, that is already the case with the event type filtering performed in `SearchEvents` is implemented using (an analogue of) `FilterExpression`:

> For example, suppose that you `Query` a table, with a `Limit` value of `6`, and without a filter expression. The `Query` result contains the first six items from the table that match the key condition expression from the request.
> Now suppose that you add a filter expression to the `Query`. In this case, DynamoDB reads up to six items, and then returns only those that match the filter expression. The final `Query` result contains six items or fewer, even if more items would have matched the filter expression if DynamoDB had kept reading more items.
> ~ https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Query.html#Query.Limit

### Advantages

No new configuration options need to be introduced.
No backend data migration is needed.

### Disadvantages

Some Teleport deployments may depend on the legacy behavior of all users being able to join active sessions thanks to the implicit `KindSSHSession` privilege. However, since showing all active sessions to all users constitutes a potential security risk anyway, it should be preferable to get rid of the implicit privilege even in spite of the possibility of breaking an established use case.

With this RFD it becomes impossible to make a user's own session recordings unavailable to the user as the RBAC checks get skipped in those scenarios.
