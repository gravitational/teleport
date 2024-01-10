---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD 00162 - Improving the on-prem fleet Bot management experience

## Required approvers

* Engineering: @zmb3
* Product: @klizhentas || @xinding33
* Security: @reedloden || @jentfoo

## What

Terminology:

- Bot: An identity within Teleport intended for use by machines as opposed to
  humans. Many individual machines may act as this shared identity.
- `tbot`: The Teleport binary that acts as aBot and generates credentials for
  consumption by client applications.
- Bot instance: A single instance of `tbot` running on a host.

This RFD proposes improvements to the management of fleets of Machine ID Bots.
These improvements are mostly targetted at on-prem deployments, where the
delegated join methods are not available.

The improvements will focus on three points:

- To allow multiple Bot instances to be associated with a single Bot when using
  the `token` join method.
- To allow multiple Bot instances to be joined using a single join token when
  using the `token` join method.
- Providing a way to track and monitor Bot instances.

## Why

Whilst deploying a large fleet of Bots is fairly trivial when using the
delegated join methods, the experience when managing a fleet of bot hosts
in-prem is more challenging. 

The following burdens currently exist:

- When using the `token` join method, a Bot must be created for each host. This
  means that the privileges of many distinct Bots need to be synchronised where
  those hosts are performing the same function.
- When using the `token` join method, a token can only be used once. This means
  creating hundreds of join tokens and managing securely distributing these to
  hosts.
- When managing a large fleet of `tbot` deployments, there is no way to
  track these within Teleport. This makes it more difficult to identify hosts
  which may need updating.

As we look to onboard more Enterprise customers to Machine ID, the pains of
the on-prem experience have become more apparent. Enterprise customers are
more likely to have on-prem deployments and these are likely to be larger in
scale.

## Details

### Current State

Currently, the `token` join method introduces a generation counter as a label
on the Bot user. This counter is contained within the Bot certificate and on
each renewal, this counter is incremented. When the counter within the certificate
de-synchronises with the counter on the user, the Bot is locked out as a security
measure.

The fact that this counter is stored within a label on the Bot user creates a 
one-to-one binding between a single instance of `tbot` and a single Bot user.
This is not the case when using the delegated join methods.
 
### Persistent Bot Instance Identity

Today, there is no persistent identifier for an individual instance of a Bot.
Instead, all `tbot` instances are effectively identified solely by their Bot
identity. There is no easy way to distinguish them. On each renewal, `tbot`
regenerates the private-public key pair that is used within its certificate and
there is no other form of unique ID that is persisted across renewals.

This poses a few challenges:

- For the purposes of auditing, it is not possible to trace actions to a
  specific instance of a Bot.
- For improving the `token` join method to support multiple Bot instances
  associated with a single Bot, there is no identifier to correlate with the 
  generation counter.
- For analytics purposes, it's difficult for us to track the number of
  individual Bot instances in use. We cannot easily determine if it's a single
  very active Bot instance, or many less active Bot instances.

To rectify this, a unique identifier should be established for an instance of
a Bot.

#### Public Key Fingerprint

One option is to modify the behaviour of `tbot` to persist and reuse the
keypair across renewals. We could then use a fingerprint of the public key as a
unique identifier of the Bot instance.

This feels like a natural identifier. It avoids introducing a new attribute to
certificates as the public key is already encoded within certificates. The
nature of public-key cryptography also means that this provides the Bot instance
a way to identify itself without needing an issued certificate.

However:

- Does switching to key reuse reduce security?
- Is a fingerprint a user understandable identifier?
- Key rotation resets the identity of a Bot instance.

##### Alternative: UUID Certificate Attribute

On the initial join of a Bot instance, we could generate a UUID to identify that 
Bot instance and encode this within the certificate. Upon renewals, the UUID
would be copied from the current certificate and into the new one.

Whilst this is fairly easy to implement for the `token` join method, one
challenge for the delegated join methods is that rather than renewing, the
`tbot` instance merely re-joins. As the join RPCs are unauthenticated, the 
previous certificate of the Bot instance is not readily available. We can
either:

- Accept this limitation and treat each renewal of a delegated Bot instance as a
  new Bot instance.
- Add support for calling the join RPCs with a client certificate.

TODO: There was a recent investigation about certificate hierarchies. Integrating
with this would be ideal and would mean this integrates with security reports.

### BotInstance Resource

With a persistent identifier for a Bot instance established, we can now track
information about a specific Bot instance server-side. In addition to providing
a way to store a generation counter per instance, this yields other benefits:

- Allows Bot instances to be viewed within the UI and CLI.
- Allowing Bot instances to submit basic self-reported information about itself
  and its host, e.g:
  - `tbot` version
  - Hostname, OS and OS version
  - The configuration of `tbot`
  - Health status
- Record metadata from delegated joins to enrich information about the Bot.
  E.g show the linked repository / CI run number
- Billing based on Bot instances rather than Bots.

Some of this information is known and verified by the server - for example, the
certificate generation or the join metadata. Some of this information is
self-reported and should not be trusted. The information from these two sources
should be segregated to avoid confusion.

BotInstance will be a new resource type introduced to track this information.

```protobuf
syntax = "proto3";

import "teleport/header/v1/metadata.proto";

// A BotInstance
message BotInstance {
  // The kind of resource represented.
  string kind = 1;
  // Differentiates variations of the same kind. All resources should
  // contain one, even if it is never populated.
  string sub_kind = 2;
  // The version of the resource being represented.
  string version = 3;
  // Common metadata that all resources share.
  teleport.header.v1.Metadata metadata = 4;
  // The configured properties of a BotInstance.
  BotInstanceSpec spec = 5;
  // Fields that are set by the server as results of operations. These should
  // not be modified by users.
  BotInstanceStatus status = 6;
}

message BotInstanceSpec {
  // Empty as is not user configurable.
  // Eventually this could be leveraged for simple command and control?
}

// BotInstanceStatusHeartbeat contains information self-reported by an instance
// of a Bot. This information is not verified by the server and should not be
// trusted.
message BotInstanceStatusHeartbeat {
  google.protobuf.Timestamp timestamp = 1;
  bool is_startup = 2;
  string version = 3;
  string hostname = 4;
  // In future iterations, additional information can be submitted here.
}

message BotInstanceStatusAuthentication {
  google.protobuf.Timestamp timestamp = 1;
  string join_method = 2;
  google.protobuf.Struct metadata = 3;
  // On each renewal, this generation is incremented. For delegated join
  // methods, this counter is not checked during renewal. For the `token` join
  // method, this counter is checked during renewal and the Bot is locked out if
  // the counter in the certificate does not match the counter of the last
  // authentication.
  int32 generation = 4;
}

message BotInstanceStatus {
  string bot_name = 1;
  // Last X records kept, with the second oldest being removed once the limit
  // is reached. This avoids the indefinite growth of the resource but also
  // ensures the initial record is retained.
  repeated BotInstanceStatusAuthentication authentications = 2;
  // Last X records kept, with the second oldest being removed once the limit
  // is reached. This avoids the indefinite growth of the resource but also
  // ensures the initial record is retained.
  repeated BotInstanceStatusHeartbeat heartbeats = 3;
}
```

#### Recording Authentication Data

Specific edge-cases to handle:

- BotInstance does not exist but renewal is received
  - Reject renewals, trigger `tbot` to exit and suggest reset, OR
  - Create a BotInstance and continue as normal. Warn/Error log.
- Join method/token changes:
  - Reject renewals, trigger `tbot` to exit and suggest reset, OR
  - Emit warning and continue.
  - Consider case where linked Bot changes

#### Recording Heartbeat Data

Pros:

- Avoids making significant changes to the existing join/renew RPCs.
- Allows for Heartbeats to be submitted at a different frequency to renewals.

Cons:

- Information about the Bot instance would be incomplete immediately after
  joining.
- Some information can only be updated during the join/renew
  e.g generation counter and last join metadata. So we'd still need to update 
  the join/renew RPCs to support this. However, no changes would need to be
  made to the RPC message.
- Information within the Heartbeat could come from different instances in time. 

Specific edge-cases to handle:

- BotInstance does not exist but heartbeat is received

##### Alternative: Submit Heartbeat data on Join/Renew

Pros:

- Avoids introducing a new RPC and ensures that all data within the Heartbeat
  comes from the same instance in time.
- Allows self-reported information to be used as part of renewal decision.
  This is not a strong defence as it is self-reported and cannot be trusted.

Cons:

- Heartbeats are limited to the interval of renewal.

#### API

### Changes to the `token` Join Method

No longer consumed on join.

### CLI Changes

#### `tbot`

`tbot reset`

#### `tctl`

`tctl bot instances list`
`tctl bot instances list --bot <bot name>`

Additionally, `tctl rm`/`tctl get` should be able to operate on BotInstance.

There is no requirement for it to be possible to create or update a BotInstance
with `tctl`.

### Implementation

1. a
2. b
3. c

## Security Considerations

### Audit Events

TODO

## Alternatives

### Skip the Heartbeats and just improve the `token` join method

TODO

One challenge would be contention over the user resource if a large number of
Bot instances are trying to renew their certificates at the same time. Our
Backend lacks support for transactional consistency and this increases the risk
of two Bot instances renewing simultaneously and producing an inconsistent state
that locks one of them out.