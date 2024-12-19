---
authors: Noah Stride (noah@goteleport.com), Nic Klaassen (nic@goteleport.com)
state: draft
---

# RFD 0194 - Upstream PKI Support

## Required Approvers

* Engineering: ???
* Product: ???? 

## What

Today, all certificate authorities within Teleport are root certificate
authorities. This RFD proposes adding support for a certificate authority
within Teleport to be an intermediate certificate authority within an external
PKI hierarchy owned by a user.

This RFD primarily focuses on the "SPIFFE" certificate authority which issues
credentials for Teleport Workload Identity. However, decisions made within this
RFD should consider the broader implications.

## Why

There are a number of reasons why an organization may desire, or require, that
Teleport Workload Identity credentials are issued by an intermediate to a 
root certificate authority that is controlled by the organization.

For example:

- It may be required as part of compliance policy. 
- Ability to revoke certificates issued by Teleport Workload Identity, without
  using Teleport Workload Identity.
- Ability to rotate the Teleport Workload Identity certificate authority without
  needing to distribute the new certificate authority to validators working with
  workload identities.
