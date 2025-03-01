---
authors: Dave Sudia (david.sudia@goteleport.com)
state: draft
---

# RFD 0190 - Add Labels During Guided Enrollment

## Required Approvers

* Engineering: @r0mant
* Product: @xinding33

## Overview

### Problem

* Users completing a guided enrollment flow end up with enrolled resources that do not have Teleport labels on them
  * This prevents them from immediately providing access to users to those resources
  * They have to go to those resources and update the labels, which is unfamiliar for new users doing a single enrollment like an SSH server, and difficult for scaled guides like EC2 AutoEnrollment


### Other data/context

These are common issues reported by SEs and in user interviews

### Appetite/Resources

We want the scope of this project to be doable for a full stack engineer with assistance from one  product engineer and one designer, within 6 weeks.

## Solution

### Hypothesis

Adding the ability to add labels at a step during each guide will improve user satisfaction with the guided flows, and improve PoV/Trial success.

### Use Cases/Requirements
* As an admin
  * I want to be able to optionally add labels to resources created while I go through a guided flow
* As a new user
  * I want to know why I would want to or not want to add labels, and how to add them in a way that is effective for user access
* As a Teleporter
  * I need to know how many people use the label function during a guided flow.

### Rabbit Holes

A thing that could potentially blow out the scope is trying to do custom labels for
auto-enrolled resources. Since these already pull labels we will advise users to
change labels in AWS

### Out of Bounds

Self-hosted Postgres and MySQL already have this capability.

### Outlines/Sketches

We want this to be added to the following guides. These are in priority order.
* Self-hosted SSH - Mac and Linux
  * Labels should end up in the script we generate
* EC2 Auto-Enrollment
  * Add an admonition in the flow explaining that labels will be imported, and explain they should update tags to customize it
* RDS - MySQL/Aurora/Postgres
  * Add an admonition in the flow explaining that labels will be imported, and explain they should update tags to customize it
* EKS
  * Add an admonition in the flow explaining that labels will be imported, and explain they should update tags to customize it
* Kubernetes
  * Labels should get inserted into the helm chart command provided
* Application
  * Labels should get added into the auto-generated script
* macOS
  * Labels should end up in the script we generate

#### UX changes

We need a tooltip or similar design element explaining label strategy.

#### Stretch goal

Standardize the Application flow to be like the other flows.


## Value

### Opportunity

Improving user success in enrolling resources that are immediately accessible to the right users should improve PoV and trial success.

### Measuring Success
* Increase in sessions started on resources where labels were added during the guided flow

## Implementation

### Design
To be added to by design team.

### Engineering
To be added to by engineering team.
