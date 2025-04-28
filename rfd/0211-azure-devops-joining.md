---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 211 - Azure DevOps Joining

## Required Approvals

- Engineering: TimB && DanU
- Product: DaveS 

## What

Allow Bots & Agents to secretlessly authenticate to Teleport from Azure DevOps
pipelines.

## Why

Azure DevOps is a popular CI/CD platform, and today, leveraging Teleport 
Machine & Workload Identity from Azure DevOps is not possible without
laborious workarounds.

An Azure Devops join method would allow Bots/Agents to authenticate to Teleport
without the use of long-lived secrets, and provide richer metadata for audit
logging & authorization decision purposes.

## Details

Goals:

- Allow authentication to Teleport from Azure DevOps pipelines without the use
  of long-lived secrets.
- It should be possible to scope this authentication to a specific Azure DevOps
  pipeline. 
- Mitigate common attacks such as token reuse.

### Background on Azure DevOps & Azure authentication

