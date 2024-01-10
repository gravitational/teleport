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

This RFD proposes improvements to the management of fleets of Machine ID Bots.
These improvements are mostly targetted at on-prem deployments, where the
delegated join methods are not available.

The improvements are two-fold:

- Allowing a single join token to be used to join a number of hosts.
- Providing a way to track individual bot instances.

Terminology:

- Bot: An identity within Teleport intended for use by machines as opposed to
  humans. Many individual machines may act as this shared identity.
- `tbot`: The Teleport binary that acts as aBot and generates credentials for
  consumption by client applications.
- Bot instance: A single instance of `tbot` running on a host.

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

#### A) UUID Certificate Attribute

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

#### B) Public Key Fingerprint

Another option is to modify the behaviour of `tbot` to persist and reuse the
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

#### Decision

TODO

### Bot Instance Data (a.k.a Heartbeats??)

With a persistent identifier for a Bot instance established, we can now track
information about a specific Bot server-side. In addition to providing a way
to store a generation counter per instance, this could yield other benefits:

- Allow Bot instances to be viewed within the UI and CLI.
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
  bool one_shot = 2;
  string version = 3;
  string hostname = 4;
  // In future iterations, additional information can be submitted here.
}

message BotInstanceStatusAuthentication {
  google.protobuf.Timestamp timestamp = 1;
  google.protobuf.Struct metadata = 2;
}

message BotInstanceStatus {
  string join_method = 1;
  string generation = 2;
  repeated BotInstanceStatusAuthentication authentications = 3;
  repeated BotInstanceStatusHeartbeat heartbeats = 4;
}
```

#### Submitting Heartbeat Data

This additional information from the Bot could be submitted in two ways.

##### A) Specific Heartbeat RPC

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

##### B) Submit Heartbeat data on Join/Renew

Pros:

- Avoids introducing a new RPC and ensures that all data within the Heartbeat
  comes from the same instance in time.

Cons:

- Heartbeats are limited to the interval of renewal.

##### Decision

### Improving the `token` join method

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