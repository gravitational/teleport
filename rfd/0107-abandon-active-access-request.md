---
authors: Jeff Pihach (jeff.pihach@goteleport.com)
state: draft
---

# RFD 0107 - Abandon Active Access Requests

## Required Approvers

- Engineering: @zmb3

## What

Allow users who are using elevated permissions via an access request to abandon
them ahead of their expiry once they are no longer of use. Once a request has been
abandoned, it can no longer be used to assume elevated privileges.

## Why

Today, when a user is finished with an access request, they can "unassume"
the associated roles and drop back down to a standard level of access. The
request can be leveraged to elevate permissions again until it expires.

By allowing a user to declare that they are done with an access request
and no longer need to use it, we can render the access request invalid
for future use and <reduce the time and surface area....>


By reducing the amount of time that a user has elevated permissions we also
reduce the time and surface area to which an attack could occur in the event
of a compromise.

## Success Criteria

- A user is able to abandon an active access request ahead of its expiry time
  via the CLI or UI.
- Creating a lock to prevent further use of the existing certificate.
- An abandoned access request is unrecoverable and unusable.

## Details

### CLI

A new `abandon` sub command to `tsh request` will be added. This command will
call an GRPC method `AbandonAccessRequest` to mark the access request as
`ABANDONED` and to set a lock on the access request.

### UI

When viewing your access request, in addition to the unassume option, there
will also be a new button made available to abandon the active access request.
The server will call the GRPC method `AbandonAccessRequest` to mark the access
request as `ABANDONED` and to set a lock on the access request.

### Core

```protobuf
// authservice.proto
service AuthService {
  ...
  rpc AbandonAccessRequest(RequestID) returns (google.protobuf.Empty);
  ...
}
```

A new `RequestState` will be added to Access requests to indicate that the
request has been intentionally abandoned. This will allow us to indicate in the
UI that the request has been abandoned by the user.

```protobuf
// types.proto

// RequestState represents the state of a request for escalated privilege.
enum RequestState {
  // NONE variant exists to allow RequestState to be explicitly omitted
  // in certain circumstances (e.g. in an AccessRequestFilter).
  NONE = 0;
  // PENDING variant is the default for newly created requests.
  PENDING = 1;
  // APPROVED variant indicates that a request has been accepted by
  // an administrating party.
  APPROVED = 2;
  // DENIED variant indicates that a request has been rejected by
  // an administrating party.
  DENIED = 3;
  // Abandoned variant indicates that the request has been intentionally
  // abandoned by the target of the access request.
  ABANDONED = 4;
}
```

### Why is a lock required

Once a certificate is issued, without a lock, it's assumed to be valid until it
has expired. Access requests are only checked on login while the certificate is
being generated so setting this state could only prevent subsequent logins from
using the elevated permissions.

It is unclear to the user that running `tsh request abandon <request-id>` won't
invalidate the existing certificate and that they still need to run `tsh logout`. I don't feel that most users will log out separately which invalidates
the root vulnerability this state was to resolve.

We could have `tsh request abandon` log the user out but this will only prevent
that one client from using the access request, not all potential clients.

## Alternative Approach

### Deleting the access request

Instead of adding a new state to the access request we could permit the target
of an access request to delete that request. In doing this we would also modify
how deleting an access request works so that if it was approve d it would also
create a lock for that request.
