---
authors: Edoardo Spadolini (edoardo.spadolini@gmail.com)
state: draft
---

# RFD 0076 - Downtime minimization

## Required Approvers
* Engineering: TBD
* Security: TBD
* Product: TBD

## What

This RFD describes the causes of downtime that can occur during the course of normal cluster operation, regular maintenance (upgrades of cluster components - mainly auth nodes and proxy nodes, scheduled CA rotations with long grace period for all CAs) and emergency maintenance (typically user CA rotation with little to no grace period), and discusses the changes that can be made to improve the availability of services accessed through Teleport within the constraints of the current design, and recommends potential future steps to further improve things by completely sidestepping some problems in certain situations.

## Why

As Teleport becomes a fundamental part of how infrastructure and computing resources are accessed throughout an organization, it's important to know when and why certain parts of the cluster can be unavailable and for how long, and if it's possible to reduce or ideally completely negate this downtime by spending more or less effort and resources.

## Details

TBD
