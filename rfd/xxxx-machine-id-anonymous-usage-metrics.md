---
authors: Noah Stride (noah.stride@goteleport.com)
state: draft
---

# RFD XXXX - Machine ID Anonymous Agent Usage Metrics

## Required Approvers

* Engineering: @zmb3
* Product: @klizhentas && @xinding33

## What

Collect anonymous usage information about invocations of the Machine ID agent
from the agent itself.

Out of scope is the collection of usage information from the Auth server.

## Why

It is currently difficult to determine the uptake of Machine ID by customers,
and for what use-cases they use Machine ID for. This makes product decisions
about determining areas of focus more difficult.

Adding basic anonymous metrics to the `tbot` agent will provide helpful
information on adoption without compromising privacy.

## Details

### Collecting events


### Submitting events

### Consent

Users will explicitly opt-in to anonymous usage metrics with a command-line
parameter or configuration file field e.g:

```sh
tbot --anonymous-analytics
```

or

```yaml
anonymous_analytics: true
```

Examples provided in the documentation will be updated to include this value
to encourage opt-in, but will make clear that this parameter can be removed.

### Anonymization

The events will contain no properties that identify a user, `tbot` or Teleport
instance. Therefore, no anonymization is required at this time.

### Events

#### `tbot.start`

Event properties:

- `run_mode`: one of [`one-shot`, `continuous`]
- `version`: string indicating the version of `tbot`
- `join_type`: string indicating the join type configured
- `helper`: optional string indicating if a helper is invoking `tbot`. For 
  example: `gha:teleport-actions/auth`
- `helper_version`: optional string indicating the version of the helper that
  invoking `tbot`.
- `destinations_configured`: a count of total destinations configured.
- `destinations_configured_database_access`: a count of Database Access 
  destinations configured.
- `destinations_configured_kubernetes_access`: a count of Kubernetes Access
  destinations configured.
- `destinations_configured_application_access`: a count of Application Access
  destinations configured.

