---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0110 - Usage reporting: developer guidelines

## Required Approvers

Engineering: @timothyb89 && @michellescripts && @marcoandredinis && @gzdunek

## What

To provide developer documentation on how the usage event reporter works and where to intervene to add new events or mutate existing ones.

## Why

Usage events are generated and handled in more than a few places, both in Teleport (as a result of cluster operations or as a result of the user interacting with the web UI) and in client-side applications (Teleport Connect and Machine ID), and there's between two and four places in the code that one needs to change to be able to start generating a new event, or to change the data associated with an existing one. It's useful to have a reference to know where to look rather than having to understand the entire system anew every time.

## Further reading

This list should be kept up to date whenever a new relevant RFD is added.

- [RFD - Product Metrics](https://github.com/gravitational/teleport/blob/rfd-product-metrics/rfd/xxxx-product-metrics.md) ([#19845](https://github.com/gravitational/teleport/pull/19845))
- [Cloud RFD 0053 - PreHog (reporting phase 1)](https://github.com/gravitational/cloud/blob/master/rfd/0053-prehog.md)
- [RFD 0094 - Teleport Discover Metrics](https://github.com/gravitational/teleport/blob/master/rfd/0094-discover-metrics.md)
- [RFD 0097 - Teleport Connect usage metrics](https://github.com/gravitational/teleport/blob/master/rfd/0097-teleport-connect-usage-metrics.md)
- [RFD 0106 - Machine ID Anonymous Agent Telemetry](https://github.com/gravitational/teleport/blob/master/rfd/0106-machine-id-anonymous-telemetry.md)
- [RFD 0108 - Agent Census](https://github.com/gravitational/teleport/blob/master/rfd/0108-agent-census.md)
