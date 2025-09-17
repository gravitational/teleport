---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 227 - EC2 Auto Discovery by AWS Organization

## Required Approvers

* Engineering: @r0mant &&
* Product: @r0mant

## What

Auto enroll EC2 instances from an AWS Organization without enumerating account ids.

## Why

Teleport auto discovers EC2 instances and installs teleport into them.
Instances are configured to join the cluster, and Teleport users can access them.

Setting up and configuring the auto discover mechanism requires an entry per AWS account.

Some deployments have a high number (2000+) of AWS Account IDs, or change the active account ID frequently.

Being able to configure the auto discover once, and have it auto discover the account IDs will greatly benefit the UX of this feature.

## Details

### UX

#### User stories
