---
authors: Xin Ding (xin@goteleport.com)
state: draft
---

# RFD 120 - Cloud Agent Auto Upgrades UX

## Required Approvals

* Engineering: @jimbishopp && @r0mant
* Product: @klizhentas
* Security: @reedloden

## What

Teleport will add support for Agent Auto Upgrades in v13.0.0. This document
details tactical UX changes to expose the Agent Auto Upgrade capabilities to
Teleport Cloud Admins.

### Terminology

* (Teleport) Practitioner: People who are actively using Teleport in their
  day-to-day.
* (Teleport) Admins: A subset of Practitioners. Admins deploy, configure, and
  otherwise set up Teleport.
* (Teleport) End Users: A subset of Practitioners. End Users use Teleport to
  access infrastructure resources.
* (Teleport) Agent: The `teleport` process which can run one more more Teleport
  services.
* (Teleport) Cluster Alerts: Messages in the Teleport Web UI and appropriate
  Teleport CLIs that alert Practitioners of relevant concerns.
* Unaccompanied Agent: A Teleport Agent deployed without an accompanying auto upgrader
  service.

## Why

Simply adding support for Agent Auto Upgrades doesn't help if Admins don't adopt
it. We want all Teleport customers to enable Agent Auto Upgrades in order to
reduce the significant workload associated with manually managing and upgrading
a fleet of Teleport Agents.

## How

There are two key tactical UX change that will increase the adoption of Agent
Auto Upgrades among Teleport Cloud Admins.

1. Where possible, push Admins to deploy Teleport Agents via supported package
   managers (i.e. `helm`, `apt`, and `yum`) as this will deploy the
   `teleport-ent-cloud-updater` package alongside the `teleport` package. It's
   important that we provide a method (potentially via an optional flag with a
   sane default) for Admins to point the auto upgrader service to the right
   server. Post installation, the script should also inform Admins about how
   Agent Auto Upgrades works, with an accompanying docs link to more details.
2. Notify Admins via Cluster Alerts when Unaccompanied Agents are detected. The
   Cluster Alert should also present a docs link informing Admins how to
   properly add an auto upgrader service.

The first tactical change requires modifications to:

* https://goteleport.com/download/.
* Any commands exposed in the Teleport Web UI, including those in Teleport
   Discover.

The second tactical change requires:

* Detection of and Cluster Alerts associated with Unaccompanied Teleport Agents.
* An easy (ideally one command) way to add an auto upgrader service to an
   Unaccompanied Teleport Agent.