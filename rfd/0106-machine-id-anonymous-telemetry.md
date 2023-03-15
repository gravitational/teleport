---
authors: Noah Stride (noah.stride@goteleport.com)
state: implemented ([v12.1.0](https://github.com/gravitational/teleport/pull/22174)]
---

# RFD 0106 - Machine ID Anonymous Agent Telemetry

## Required Approvers

* Engineering: @zmb3
* Product: @klizhentas && @xinding33

## What

Collect anonymous usage information about invocations of the Machine ID agent.

Out of scope is the collection of usage information from the Auth server.

## Why

It is currently difficult to determine the adoption of Machine ID, and for what
use-cases they use Machine ID for. This makes product decisions about 
determining areas of focus more difficult.

Basic anonymous telemetry from the `tbot` agent will provide helpful
information on adoption without compromising privacy.

## Details

### Event collection

For now, a single event on startup will be submitted by `tbot`.

This will be implemented as part of `tool/tbot` rather than `lib/tbot`, meaning
that events will be submitted when using the `tbot` binary but not when
embedding the `tbot` library within another binary. This means data will not
be polluted by internal uses of the `tbot` library (e.g the operator).

Event collection and submission will be started concurrently to the `tbot`
functionality, and should not impede the primary function of `tbot`. In the
case of failure, a warning message should be omitted. The routine will read in
the configuration of `tbot`, extract the relevant values and encode these within
an event protobuf.

### Event submission

Events will be submitted directly to an unauthenticated `tbot` specific endpoint
of the `prehog` service:

```protobuf
syntax = "proto3";

package prehog.v1alpha;

import "google/protobuf/timestamp.proto";

message SubmitTbotEventRequest {
  // UUID identifying that tbot session. This is future-proofing for if we
  // decide to add multiple events in future, and need to tie them together.
  string distinct_id = 1;
  // optional, will default to the ingest time if unset
  google.protobuf.Timestamp timestamp = 2;

  oneof event {
    // See the events section for the fields included within startup.
    TbotStartEvent start = 3;
  }
}

message SubmitTbotEventResponse {}

service TbotReportingService {
  rpc SubmitTbotEvent(SubmitTbotEventRequest) returns (SubmitTbotEventResponse) {}
}
```

As only a single event will be submitted, batching is not of concern.

### Event storage

Events received by `prehog` will be encoded in a PostHog compatible format
and then submitted to PostHog.

Tbot events will be share the same project as clusters and website events.

### Consent

Users will explicitly opt-in to anonymous usage telemetry with an environment
variable, e.g:

```sh
# Send anonymous telemetry on startup about the usage of tbot in order
# to help us understand what parts of Machine ID to improve.
# Find out more at goteleport.com/awesome-docs-page
export TELEPORT_ANONYMOUS_TELEMETRY=1
tbot start --snip--
```

For helpers like GitHub actions, this will be another option the user can
provide, e.g:

```yaml
steps:
  - name: Install Teleport
    uses: teleport-actions/setup@v1
    with:
      version: 11.0.3
  - name: Authorize against Teleport
    id: auth
    uses: teleport-actions/auth@v1
    with:
      # Send anonymous telemetry on startup about the usage of tbot in order
      # to help us understand what parts of Machine ID to improve.
      # Find out more at goteleport.com/awesome-docs-page
      anonymous-telemetry: true
```

Examples provided in the documentation will be updated to include this value
to encourage opt-in, but will make clear that this parameter can be removed.

A documentation page will be created that will explain what telemetry is
collected and where this telemetry is stored.

When telemetry is enabled, a log message will be output on startup stating that
telemetry is being collected and linking to the relevant documentation.

When telemetry is not enabled, a log message will be output on startup
informing the user that Telemetry is not enabled and linking to the relevant
documentation.

### Anonymization

The events will contain no properties that identify a user, `tbot` or Teleport
cluster. Therefore, no additional anonymization is required at this time.

### Events

#### `tbot.start`

Event properties:

- `tbot.run_mode`: one of [`one-shot`, `daemon`]
- `tbot.version`: string indicating the version of `tbot`
- `tbot.join_type`: string indicating the join type configured
- `tbot.helper`: optional string indicating if a helper is invoking `tbot`. For 
  example: `gha:teleport-actions/auth`
- `tbot.helper_version`: optional string indicating the version of the helper 
  that invoking `tbot`
- `tbot.destinations_other`: a count of destinations configured that are not
  associated with Database Access, Kubernetes Access or Application Access
- `tbot.destinations_database`: a count of Database Access 
  destinations configured
- `tbot.destinations_kubernetes`: a count of Kubernetes Access
  destinations configured
- `tbot.destinations_application`: a count of Application 
  Access destinations configured.

