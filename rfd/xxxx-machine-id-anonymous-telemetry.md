---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD XXXX - Machine ID Anonymous Agent Telemetry

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

### Event collection and submission

For now, a single event on startup will be submitted by `tbot`.

This will be implemented as part of `tool/tbot` rather than `lib/tbot`, meaning
that events will be submitted when using the `tbot` binary but not when
embedding the `tbot` library within another binary. This means data will not
be polluted by internal uses of the `tbot` library (e.g the operator).

Event collection and submission will be started concurrently to the `tbot`
functionality, and should not impede the primary function of `tbot`. In the
case of failure, a warning message should be omitted.

Events will be submitted directly to the public endpoint of the `prehog`
service.

### Event storage


### Consent

Users will explicitly opt-in to anonymous usage telemetry with a command-line
parameter or configuration file field e.g:

```sh
tbot --anonymous-telemetry-opt-in
```

or

```yaml
anonymous_telemetry_opt_in: true
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
instance. Therefore, no additional anonymization is required at this time.

### Events

#### `tbot.start`

Event properties:

- `run_mode`: one of [`one-shot`, `continuous`]
- `version`: string indicating the version of `tbot`
- `join_type`: string indicating the join type configured
- `helper`: optional string indicating if a helper is invoking `tbot`. For 
  example: `gha:teleport-actions/auth`
- `helper_version`: optional string indicating the version of the helper that
  invoking `tbot`
- `destinations_configured`: a count of total destinations configured
- `destinations_configured_database_access`: a count of Database Access 
  destinations configured
- `destinations_configured_kubernetes_access`: a count of Kubernetes Access
  destinations configured
- `destinations_configured_application_access`: a count of Application Access
  destinations configured.

