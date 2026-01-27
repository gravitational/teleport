---
authors: Marco Dinis (marco.dinis@goteleport.com)
state: draft
---

# RFD 0245 - Embed Discovery Service in Auth

## Required Approvers

* Engineering: @r0mant
* Security:
* Product: @r0mant

## What

Run the Discovery Service alongside Auth Service.

## Why

Discovery Service is used to auto-enroll resources into the cluster.
Those resources are located in user's cloud accounts.

Setting up discovery requires users to either follow a [guide](https://goteleport.com/docs/enroll-resources/auto-discovery/) or to use our Discovery guides in the UI, by clicking in "Enroll New Resource" and then selecting one of the auto-enrollment guides.

Today, both flows require the user to deploy a Discovery Service, adding friction because users have to deploy and maintain a new Teleport installation.
Forcing users to leave the flow is also disruptive and creates traction.

When configuring discovery, users can either use an integration or ambient credentials in order to provide access to their cloud account.

When using integration credentials, the deployment location of the Discovery Service is irrelevant because it will not load any credentials from environment variables or well known files.
In those scenarios, having an **always running Discovery Service** improves the UX because:
- no management of an extra teleport service deployment is required
- users don't need to leave the flow, increasing the probability of successfully enrolling their resources

For Cloud tenants, we already run a Discovery Service alongside Auth Services.
So, no changes are expected there.

## Details

### UX

#### User stories

**Teleport cloud users**

Nothing changes for Teleport Cloud users because there's an always running

**User sets up EC2 enrollment via WebUI**

Alice logs in to the cluster and follows the EC2 Auto Discover guide.

The first step is to set up an integration or using an existing one.

Currently, the second step is to deploy a Discovery Service, but that is no longer necessary.

Alice lands in the "Create Discovery Config" step after selecting the integration, and now only needs to configure which regions should be considered and, optional, define tag filters.

**User sets up Azure VM enrollment via docs guide using ambient credentials**

Alice follows the Azure VM enrollment guide.

No longer needs to deploy a Discovery Service.

Might need to expose ambient credentials to the Auth server in order to allow the embedded discovery service to access the required Azure APIs.

TBC...