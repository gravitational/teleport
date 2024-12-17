---
authors: Noah Stride (noah.stride@goteleport.com)
state: Implemented (16.2.0)
---

# RFD 00162 - Improving the on-prem fleet Bot management experience

## Required approvers

- Engineering: @zmb3
- Product: @klizhentas || @xinding33
- Security: @reedloden || @jentfoo

## What

Terminology:

- Bot: An identity within Teleport intended for use by machines as opposed to
  humans. Many individual machines may act as this shared identity.
- `tbot`: The Teleport binary that acts as a Bot and generates credentials for
  consumption by client applications.
- Bot instance: A single instance of `tbot` running on a host.

This RFD proposes improvements to the management of fleets of Machine ID Bots.
These improvements are mostly targeted at on-prem deployments, where the
delegated join methods are not available.

The improvements will focus on three points:

- To allow multiple Bot instances to be associated with a single Bot when using
  the `token` join method.
- To allow multiple Bot instances to be joined using a single join token when
  using the `token` join method.
- Providing a way to track and monitor Bot instances.

## Why

Whilst deploying a large fleet of Bots is fairly trivial when using the
delegated and TPM join methods, the experience when managing a fleet of bot
hosts on-prem is more challenging.

The following burdens currently exist:

- When using the `token` join method, a Bot must be created for each Bot instance. This
  means that the privileges of many distinct Bots need to be synchronized where
  those hosts are performing the same function.
- When using the `token` join method, a token can only be used once. This means
  creating hundreds of join tokens and managing securely distributing these to
  hosts.
- When managing a large fleet of Bot instances, there is no way to
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
de-synchronizes with the counter on the user, the Bot is locked out as a security
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

#### Bot Identity Trust

We should be mindful that adding a new persistent identifier may increase our
attack surface, particularly if we allow clients manipulate their persistent
identifier or in any way trust the values communicated to Auth during the join
process.

For example, trusting this identifier might allow a bot to better masquerade
itself as a preexisting instance and avoid discovery by an end user that falsely
assumed no unexpected instances had joined their cluster. Additionally, if we
implement join limits (i.e. tokens that only allow N bots to join), malicious
bots could reuse existing identifiers to bypass join limits.

To mitigate this, we should make certain to cryptographically verify identifiers
during the renewal process. For example, we can embed the identifier as a
certificate field to ensure it cannot be tampered with once issued by the Auth
service, or encrypt the renewed certificates using the previous iteration's
public key to ensure the calling bot owns the private key. Adopting proper mTLS
during the join process should accomplish both of these goals.

##### Verifying Bot Identities

We currently see two methods for cryptographically verifying bot identities at
renewal time:

1. We could expose the existing functionality of the HTTPS-only
   `RegisterUsingToken` over gRPC. The existing gRPC `JoinService` can be
   accessed with and without authentication, so we could inspect the client
   connection to find the existing bot identity, if any, and it would be
   implicitly verified.
2. We could adapt the existing HTTPS implementation of `RegisterUsingToken` to
   additionally accept an encoded existing certificate, and return certificates
   encrypted with the certificate's public key. We can verify the certificate
   was originally signed with our CA, and the client will only be able to
   decrypt the returned identity if they actually have the private key for the
   previous identity.
3. Accept client certificate authentication in HTTPS join methods. gRPC join
   methods already accept client certificates, however these are authenticated
   in the Join Service rather than in Auth directly. As such, gRPC join methods
   will need to pass along the bot instance ID in a trusted field.

Our preference is option (3): bots and other clients join as usual, but will
always provide a client certificate when one is available. Auth will use this
client certificate to pair joining bots with their previous instances.

Additionally, a downside to certificate verification is that the certificate
validity period becomes a factor. If bots are only run intermittently, like
from a CI workflow, their certificates could expire and prevent them from
being identified as the same instance. This is likely to only impact a small
number of cases, however, as most CI provider joins are stateless and have no
certificates to present anyway. Bots that present expired certificates will
either be rejected and will need to join as a new instance.

#### UUID Certificate Attribute

On the initial join of a Bot instance, we could generate a UUID to identify that
Bot instance and encode this within the certificate. Upon renewals, the UUID
would be copied from the current certificate and into the new one.

This method additionally gives us freedom to change various join parameters
while preserving the lineage of a bot identity. Bots could change their keypair
or join method and still be properly associated with their previous iteration.

Whilst this is fairly easy to implement for the `token` join method, one
challenge for the delegated join methods is that rather than renewing, the
`tbot` instance merely re-joins. As the join RPCs are unauthenticated, the
previous certificate of the Bot instance is not readily available. We can
either:

- Accept this limitation and treat each renewal of a delegated Bot instance as a
  new Bot instance. This is likely unacceptable and would limit any advantages
  of this work to the `token` join method.
- Add support for calling the join RPCs with a client certificate.

Given our desire to ensure this identifier is trustworthy, we should prefer to
support the latter case and verify client certificates at re-joining time.

#### Alternative: Public Key Fingerprint

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

This technique does have some downsides:

- It makes it impossible to rotate a bot's private key. However, we do not
  currently support this today, and purging a bot's data directory to do so
  would simply result in a new `BotInstance`, which is likely an acceptable
  workaround.
- Our join process today is unable to cryptographically verify the public key
  presented by a joining bot to ensure that particular keypair has been issued
  an identity already. Clients can provide any public key they like, including
  that of an existing bot.

Given these downsides, we'll prefer to implement UUID instance identifiers.
Most of the technical challenge lies in adapting the join process to accept
client certificate authentication for re-joins, at which point adding a new
certificate field is trivial.

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
  // The currently configured join_method.
  string join_method = 6;
  // Indicates whether `tbot` is running in one-shot mode.
  bool one_shot = 7;
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
  // The join token used for this join or renewal. This is only populated for
  // delegated join methods as the value for `token` join methods is sensitive.
  string join_token = 3;
  // The metadata sourced from the join method.
  google.protobuf.Struct metadata = 4;
  // On each renewal, this generation is incremented. For delegated join
  // methods, this counter is not checked during renewal. For the `token` join
  // method, this counter is checked during renewal and the Bot is locked out if
  // the counter in the certificate does not match the counter of the last
  // authentication.
  int32 generation = 5;
  // The public key of the Bot instance.
  // When authenticating a Bot instance, the full public key must be compared
  // rather than just the fingerprint to mitigate pre-image attacks.
  bytes public_key = 6;
  // The fingerprint of the public key of the Bot instance.
  string fingerprint = 7;
}

// BotInstanceStatus holds the status of a BotInstance.
message BotInstanceStatus {
  // The unique identifier for this bot.
  string id = 1;
  // The name of the Bot that this instance is associated with.
  string bot_name = 2;
  // The initial authentication status for this bot instance.
  BotInstanceStatusAuthentication initial_authentication = 3;
  // The N most recent authentication status records for this bot instance.
  repeated BotInstanceStatusAuthentication latest_authentications = 4;
  // The initial heartbeat status for this bot instance.
  BotInstanceStatusHeartbeat initial_heartbeat = 5;
  // The N most recent heartbeats for this bot instance.
  repeated BotInstanceStatusHeartbeat latest_heartbeats = 6;
}
```

The name used for a BotInstance will be a concatenation of the Bot name and its
unique identifier (UUID).

When storing the BotInstance in the backend, the key will be:
`bot_instances/{bot_name}/{uuid}`. This will allow for efficient listing
of BotInstances for a given Bot.

Like agent heartbeats, the BotInstance will expire after a period of inactivity.
This avoids the accumulation of ephemeral BotInstances.

#### Recording Authentication Data

Upon each join and renewal, the BotInstance record will be updated with an
additional entry in the `status.authentications` field. If there is X entries,
then the oldest entry will be removed. This prevents growth without bounds but
also ensures that the original record is retained.

In addition, the TTL of the BotInstance resource will be extended to cover the
validity period of the issued certificate, plus a short additional time to allow
for some imprecision.

If a BotInstance does not exist, then one will be created. In the case that
this occurs for a bot using the `token` join method and this is a renewal,
a warning will be emitted and the initial generation of the BotInstance will
be sourced from the certificates current generation counter. This behaviour
will support the migration of existing `tbot` instances to the new BotInstance
behaviour.

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
instead the endpoint will ensure that the presented client certificate is for
the Bot linked to the instance.

This endpoint will be called by `tbot` immediately after it has initially
authenticated. After a heartbeat has successfully completed, another should be
scheduled for a half hour after. A small amount of jitter should be added to
the heartbeat period to avoid a thundering herd of heartbeats.

If the heartbeat fails, then `tbot` should retry on an exponential backoff.

##### Alternative: Submit Heartbeat Data on Join/Renew

Alternatively, we could add a Heartbeat field to the join/renew RPCs.

Pros:

- Avoids introducing a new RPC and ensures that all data within the Heartbeat
  comes from the same instance in time.
- Allows self-reported information to be used as part of renewal decision.
  This is not a strong defense as it is self-reported and cannot be trusted.
- Avoids a state where the BotInstance is incomplete immediately after joining
  and before it has called SubmitHeartbeat.

Cons:

- Adds Bot specific behaviour to RPCs that are also used for Node joining.
- Heartbeats are limited to the interval of renewal.

Given these cons, we'll opt introduce the new heartbeat RPC.

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
  // The name of the bot associated with the instance.
  string bot_name = 1;
  // The unique identifier of the bot instance to retrieve.
  string id = 2;
}

// Request for ListBotInstances.
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
  string bot_name = 1;
  // The unique identifier of the bot instance to delete.
  string id = 2;
}
```

### Changes to the `token` Join Method

As we now have a way to track the generation for a specific Bot instance, we
can allow multiple Bot instances to be associated with a single Bot. This
also means that the token no longer needs to be consumed on a join.

However, this does introduce a change in our security guarantees, and without
additional tooling support and sensible defaults, the change may incentivize end
users to create long-lived join tokens instead of using a more appropriate join
method, or automating issuance of short lived tokens.

To this end, we should introduce a per-bot-instance join count limit, and
configure that to be 1 join by default. This matches today's behavior, and
will help ensure users do not accidentally create a token that provides more
access than expected: "infinite use" tokens with massive join limits and/or very
long TTLs will need to be explicitly specified.

We may additionally want to put hurdles in the way of things like extremely long
token TTLs. There are legitimate low-security use cases for these, but we could
introduce a soft limit in `tctl` preventing automatic token creation with TTL
longer than 7 days, forcing users to manually create the token if they really
need it to last longer.

Additionally, renewal logic will need to be adjusted to read and update the
generation counter from the BotInstance rather than the Bot user. We should also
take care to ensure this counter behaves sensibly even when many bots are
attempting to join the cluster concurrently. The generation counter today
already has issues with concurrent joins, and it's even more important to get
this right when we can expect contention over a single bot instance resource.

### CLI Changes

#### `tbot`

`tbot reset` will be added to allow a `tbot` instance to be reset. This will
simply clear out any artifacts within the `tbot` storage directory.

In addition, if the bot detects a change in join token or join method, it should
automatically rotate its keypair. This will ensure it presents as a fresh
BotInstance to the AuthServer.

A log message should be output that identifies the linked BotInstance at
startup and on each heartbeat. This will allow users to easily correlate the
`tbot` installation with a BotInstance.

#### `tctl`

Commands to list all BotInstances and the BotInstances for a specific Bot should
be added to the `tctl bots` family:

`tctl bots instances list`
`tctl bots instances list --bot <bot name>`

The `tctl tokens add` command should be extended to allow a new token to be
associated with an existing Bot now that multiple Bot instances can be run
against a single Bot:

`tctl tokens add --type=bot --bot <bot name>`

Additionally, `tctl rm`/`tctl get` should be able to operate on BotInstance.

There is no requirement for it to be possible to create or update a BotInstance
with `tctl`.

### Analytics

A PostHog event should be emitted for each BotInstance heartbeat. This will
allow us to track active bots in a similar way to how we track active agents.

```protobuf
// a heartbeat for a Bot Instance
//
// PostHog event: tp.bot.instance.hb
message BotInstanceHeartbeatEvent {
  // anonymized name of the instance, 32 bytes (HMAC-SHA-256);
  bytes bot_instance_name = 1;
  // the version of tbot
  string version = 2;
  // indicates whether or not tbot is running in one-shot mode
  bool one_shot = 3;
  // indicates the configured join method of `tbot`.
  string join_method = 4;
}
```

Existing analytics for join, renewal and certificate generation should be
extended to include the BotInstance ID anonymized. This will allow them to be
linked together.

### Migration/Compatibility

The "create if not exists" behaviour of the BotInstance resource will mean that
existing Bot instances will have a BotInstance resource created on their first
renewal after this feature is released. Their existing generation counter will
be trusted on this first renewal. This allows for a seamless migration to the
new system.

Older `tbot` instances will not submit heartbeats. This means that their
BotInstance will only contain authentication data. Any CLI or GUI that shows
BotInstances should show a gracefully degraded state in this case that explains
that the `tbot` needs to be upgraded.

## Security Considerations

### Audit Events

An audit event should be added for the deletion of a BotInstance. The
name of the BotInstance should be added to the existing join, renewal and
certificate generation audit events.

Additionally, we should ensure bot instance identifiers are present in existing
audit events to ensure actions taken by bots can be traced back to specific
instances.

### Resistance to collision/pre-image attacks

We should be cautious that a BotInstance cannot be impersonated using a
second pre-image attack. This risk is introduced by using the public key
fingerprint as an identifier.

To mitigate this, we should ensure that the full public key is compared
to the recorded one when authenticating a BotInstance rather than merely
comparing the fingerprint.

In addition, a more modern hashing algorithm should be used to calculate the
fingerprint. In this case, we have selected SHA256 as this is more resistant
compared to hash functions such as MD5 or SHA1.

## Alternatives

### Defer the Heartbeats work and solely improve the `token` join method

We could modify the way we record the generation counter for the `token` join
method without introducing the BotInstance resource. Instead of storing a
single counter within the Bot User labels, we could store a JSON encoded map
of counters.

One challenge would be contention over the user resource if a large number of
Bot instances are trying to renew their certificates at the same time. Our
Backend has limited support for transactional consistency and this increases the
risk of two Bot instances renewing simultaneously and producing an inconsistent
state that locks one of them out.

### Introduce renewal-less and generation-less `token` join method

One option is to introduce a new join method that does not produce renewable
certificates. There would be no need for the fragile generation counter
and the join token would be continually re-used to join as is done for the
delegated join methods. This also circumvents the need for a one-to-one binding
between a Bot instance and a Bot.

This token would be incredibly sensitive and if stolen, there would be no
automated mechanisms to detect this as exists today with the generation counter.

It likely makes more sense to improve the existing `token` join method rather
than introduce a variant which behaves differently and is less secure. It would
increase the complexity of the codebase and the user experience.

## Out of Scope

These tasks are out of scope of this RFD but could be considered natural
follow-on tasks.

### Multi-phase Commit of Generation Counter

Currently, the generation counter is fragile as it is incremented server side
without confirmation that `tbot` has been able to use and persist the new
credentials. If `tbot` does not receive confirmation of the renewal or is
unable to persist the new credentials, it will be locked out on it's next
attempt to renew.

We could introduce a multi-phase commit of the generation counter. This would
provide more robustness to the renewal process.

### Locking of Individual Bot Instances

Currently, it's only possible to lock out an entire Bot user. This means that
when managing a large fleet, it would not be able to lock out a specific host
that had been compromised. This is likely to be a major friction point for those
deploying a large number of Bot instances.

It also increases the significance of the fragility of the generation counter.

### Bot Command and Control

The BotInstance resource could be extended to allow `tbot` to be controlled
remotely.
