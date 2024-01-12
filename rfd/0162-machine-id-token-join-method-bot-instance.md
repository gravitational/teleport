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

To mitigate the risk of pre-image attacks, SHA256 will be used to determine the
fingerprint of the public key. In addition, the full public key should be
recorded and verified against when authenticating a Bot instance action.

It should be noted that with this technique, rotating the keypair of a `tbot`
instance would reset the identity of that instance. Rotation of this keypair
would be unusual and this side effect seems expected.

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
  new Bot instance. This is likely unacceptable and would limit any advantages
  of this work to the `token` join method.
- Add support for calling the join RPCs with a client certificate.

This technique could reuse a recently proposed LoginID attribute. This would 
allow features such as security reports and automated anomaly detection to work
seamlessly across humans and machines.

### BotInstance Resource

The
[Resource Guidelines RFD](https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md)
will be followed.

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
  // The timestamp that the heartbeat was recorded by the Auth Server. Any
  // value submitted by `tbot` for this field will be ignored.
  google.protobuf.Timestamp recorded_at = 1;
  // Indicates whether this is the heartbeat submitted by `tbot` on startup.
  bool is_startup = 2;
  // The version of `tbot` that submitted this heartbeat.
  string version = 3;
  // The hostname of the host that `tbot` is running on.
  string hostname = 4;
  // The duration that `tbot` has been running for when it submitted this
  // heartbeat.
  google.protobuf.Duration uptime = 5;
  
  // In future iterations, additional information can be submitted here.
  // For example, the configuration of `tbot` or the health of individual
  // outputs.
}

// BotInstanceStatusAuthentication contains information about a join or renewal.
// Ths information is entirely sourced by the Auth Server and can be trusted.
message BotInstanceStatusAuthentication {
  // The timestamp that the join or renewal was authenticated by the Auth
  // Server.
  google.protobuf.Timestamp authenticated_at = 1;
  // The join method used for this join or renewal.
  string join_method = 2;
  // The metadata sourced from the join method.
  google.protobuf.Struct metadata = 3;
  // On each renewal, this generation is incremented. For delegated join
  // methods, this counter is not checked during renewal. For the `token` join
  // method, this counter is checked during renewal and the Bot is locked out if
  // the counter in the certificate does not match the counter of the last
  // authentication.
  int32 generation = 4;
}

// BotInstanceStatus holds the status of a BotInstance.
message BotInstanceStatus {
  // The public key of the Bot instance.
  // When authenticating a Bot instance, the full public key must be compared
  // rather than just the fingerprint to mitigate pre-image attacks.
  bytes public_key = 1;
  // The fingerprint of the public key of the Bot instance.
  string fingerprint = 2;
  // The name of the Bot that this instance is associated with.
  string bot_name = 3;
  // Last X records kept, with the second oldest being removed once the limit
  // is reached. This avoids the indefinite growth of the resource but also
  // ensures the initial record is retained.
  repeated BotInstanceStatusAuthentication authentications = 4;
  // Last X records kept, with the second oldest being removed once the limit
  // is reached. This avoids the indefinite growth of the resource but also
  // ensures the initial record is retained.
  repeated BotInstanceStatusHeartbeat heartbeats = 5;
}
```

The name used for a BotInstance will be a concatenation of the Bot name and the
SHA256 fingerprint of the instance's public key
e.g `my-robot/2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae`.

When storing the BotInstance in the backend, the key will be:
`bot_instances/{bot_name}/{fingerprint}`. This will allow for efficient listing
of BotInstances for a given Bot.

Like agent heartbeats, the BotInstance will expire after a period of inactivity.
This avoids the accumulation of ephemeral BotInstances.

#### Recording Authentication Data

Upon each join and renewal, the BotInstance record will be updated with an
additional entry in the `status.authentications` field. If there is X entries,
then the second-oldest entry will be removed. This prevents growth without
bounds but also ensures that the original record is retained.

In addition, the TTL of the BotInstance resource will be extended.

If a BotInstance does not exist, then one will be created. In the case that
this occurs for a bot using the `token` join method and this is a renewal,
a warning will be emitted and the initial generation of the BotInstance will
be sourced from the certificates current generation counter. 

#### Recording Heartbeat Data

A new RPC will be added for submitting heartbeat data:

```protobuf
syntax = "proto3";

package teleport.machineid.v1;

service BotInstanceService {
  // SubmitHeartbeat submits a heartbeat for a BotInstance.
  rpc SubmitHeartbeat(SubmitHeartbeatRequest) returns (SubmitHeartbeatResponse);
}

// The request for SubmitHeartbeat.
message SubmitHeartbeatRequest {
  // The heartbeat data to submit.
  BotInstanceStatusHeartbeat heartbeat = 1;
}

// The response for SubmitHeartbeat.
message SubmitHeartbeatResponse {
  // Empty
}
```

The endpoint will have a special auth/authz check. RBAC will not be used and
instead the endpoint will check:

- The presented client certificate is for the Bot linked to the instance.
- The presented client certificate's public key matches the public key recorded
  for the BotInstance.

This endpoint will be called by `tbot` immediately after it has initially
authenticated. After a heartbeat has succesfully completed, another should be
scheduled for an hour after. A small amount of jitter should be added to the
heartbeat period to avoid a thundering herd of heartbeats.

If the heartbeat fails, then `tbot` should retry on a exponential backoff.

##### Alternative: Submit Heartbeat data on Join/Renew

Alternatively, we could add a Heartbeat field to the join/renew RPCs.

Pros:

- Avoids introducing a new RPC and ensures that all data within the Heartbeat
  comes from the same instance in time.
- Allows self-reported information to be used as part of renewal decision.
  This is not a strong defence as it is self-reported and cannot be trusted.
- Avoids a state where the BotInstance is incomplete immediately after joining
  and before it has called SubmitHeartbeat.

Cons:

- Adds Bot specific behaviour to RPCs that are also used for Node joining.
- Heartbeats are limited to the interval of renewal.

#### API

Additional RPCs will be added to the BotInstance service to allow these to
be listed and deleted:

```protobuf
syntax = "proto3";

package teleport.machineid.v1;

service BotInstanceService {
  // GetBotInstance returns the specified BotInstance resource.
  rpc GetBotInstance(GetBotInstanceRequest) returns (BotInstance);
  // ListBotInstances returns a page of BotInstance resources.
  rpc ListBotInstances(ListBotInstancesRequest) returns (ListBotInstancesResponse);
  // DeleteBotInstance hard deletes the specified BotInstance resource.
  rpc DeleteBotInstance(DeleteBotInstanceRequest) returns (google.protobuf.Empty);
}

// Request for GetBotInstance.
message GetBotInstanceRequest {
  // The name of the BotInstance to retrieve.
  string name = 1;
}

// Request for ListFoos.
//
// Follows the pagination semantics of
// https://cloud.google.com/apis/design/standard_methods#list
message ListBotInstancesRequest {
  // The name of the Bot to list BotInstances for. If empty, all BotInstances
  // will be listed.
  string filter_bot_name = 1;
  // The maximum number of items to return.
  // The server may impose a different page size at its discretion.
  int32 page_size = 2;
  // The page_token value returned from a previous ListBotInstances request, if
  // any.
  string page_token = 3;
}

// Response for ListBotInstances.
message ListBotInstancesResponse {
  // BotInstance that matched the search.
  repeated BotInstance bot_instances = 1;
  // Token to retrieve the next page of results, or empty if there are no
  // more results exist.
  string next_page_token = 2;
}

// Request for DeleteBotInstance.
message DeleteBotInstanceRequest {
  // The name of the BotInstance to delete.
  string name = 1;
}
```

### Changes to the `token` Join Method

As we now have a way to track the generation for a specific Bot instance, we
can allow multiple Bot instances to be associated with a single Bot. This
also means that the token no longer needs to be consumed on a join.

Eventually, we may wish to add a way to specify a number of joins which can
occur with a token. This provides a way to easily control the lifetime of a 
token when deploying to a fleet of a pre-known size.

The renewal logic will need to be adjusted to read the generation counter from
the BotInstance rather than the Bot user.

### CLI Changes

#### `tbot`

`tbot reset` will be added to allow a `tbot` instance to be reset. This will
simply clear out any artifacts within the `tbot` storage directory.

#### `tctl`

`tctl bots instances list`
`tctl bots instances list --bot <bot name>`
`tctl tokens add --type=bot --bot <bot name>`

Additionally, `tctl rm`/`tctl get` should be able to operate on BotInstance.

There is no requirement for it to be possible to create or update a BotInstance
with `tctl`.

### Analytics

A PostHog event should be emitted for each BotInstance heartbeat. This will
allow us to track active bots in a similar way to how we track active agents.

### Implementation

1. a
2. b
3. c

## Security Considerations

### Audit Events

Deletion of a BotInstance should be audited.

## Alternatives

### Defer the Heartbeats work and solely improve the `token` join method

We could modify the way we record the generation counter for the `token` join
method without introducing the BotInstance resource. Instead of storing a
single counter within the Bot User labels, we could store a JSON encoded map
of counters.

One challenge would be contention over the User resource if a large number of
Bot instances are trying to renew their certificates at the same time. Our
Backend has limited support for transactional consistency and this increases the
risk of two Bot instances renewing simultaneously and producing an inconsistent
state that locks one of them out.
