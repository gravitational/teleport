---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 215 - MWI Terraform Provider 

## Required Approvals

- Engineering: (TimB (@timothyb89) || DanU (@boxofrad)) && Hugo (@hugoShaka)
- Product: DaveS (@thedevelopnik) 

## What

Introduce functionality to a Terraform provider to provide credentials for
other Terraform providers to access resources via Teleport Machine & Workload
Identity.

## Why

Teleport provides access to many kinds of resources (e.g Kubernetes clusters,
AWS accounts, GCP projects, etc.) that customers manage with Terraform. Today,
these customers may use long-lived credentials to access these resources and
have poor insight into what resources these credentials grant access to.

Some customers have explored leveraging Teleport's Machine and Workload Identity
to provide short-lived credentials to Terraform providers. However, this 
requires the ability to run the `tbot` binary within the environment that the
Terraform plan/apply runs (which excludes environments like Terraform Cloud) and
this implementation is generally cumbersome.

Providing the ability to generate short-lived credentials within a Terraform
provider for resources protected by Teleport will:

- Allow customers to eliminate the use of long-lived static credentials with
  high levels of privilege for their Terraform CI/CDs access to resources.
- Allow customers to better understand and control what resources these
  Terraform CI/CDs can access.
- Simplify existing Terraform CI/CDs where `tbot` runs outside the Terraform
  plan/apply itself.

## Details