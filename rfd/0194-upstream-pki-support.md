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

## Details

The project is largely divided into two phases:

- Building out the underlying support for Teleport CAs being intermediate CAs
  within an external PKI hierarchy, with a manual process for rotating.
- Building out integrations with specific PKI solutions to automate the process
  of rotating the Teleport CA.

### Phase 1: Intermediate CA Support

#### UX

```shell
$ tctl auth rotate --manual --type spiffe --phase pre-init
Updated rotation phase to pre-init. New keys have been generated, and you can
now provide a certificate to move to the next phase.
$ tctl auth generate-csr --type spiffe -o spiffe.csr
A CSR for the SPIFFE CA has been generated and written to spiffe.csr.
$ tctl auth rotate --manual --type spiffe --phase init --signed-cert spiffe.crt --upstream-ca root.crt
Updated rotation phase to "init". To check status use 'tctl status'
```

#### Implementation

TODO:
- Pre init phase.
- CSR generation.
- Signed cert import.
- Tctl commands.
- Commentary on whether this should extend to JWT.
- Importing and storing the root CA as part of the certificate_authority.

#### Teleport Workload Identity

We must decide whether the Teleport Workload Identity trust domain should be 
isolated from the upstream PKI hierarchy - effectively, should Teleport Workload
Identity distribute the upstream root CA and should workloads using Teleport
Workload Identity trust certificates issued elsewhere in the PKI hierarchy.

Whilst the idea of isolating the trust domain is appealing, there is a number of
challenges to this. Primarily, many TLS/X509 validators, do not support treating
an intermediate CA as a root CA for the purposes of validating a certificate,
and will reject a non-self signed root. This is not per the specification, but,
would pose significant concerns around compatability. 

Therefore, we will proceed with the unisolated option whereby:

- X509 SVIDs will include the intermediate CA certificate in the chain. This 
  must occur for X509 SVIDs presented to Workloads via the Workload API and 
  for X509 SVIDs written to disk.
- The upstream root CA will be distributed in any trust bundle in place of the
  intermediate CA.

TODO:
- We should note here the compromises that using an external PKI hierarchy has 
  on the security model of Teleport Workload Identity.

### Phase 2: Automation & Integration
