---
authors: Dave Sudia (david.sudia@goteleport.com)
state: draft
---

# RFD 0189 - Windows Desktop Access guide

## Required Approvers

* Engineering: @r0mant, @zmb3
* Product: @xinding33

## Overview

### Problem

* Currently only 15% of users convert from going to the docs to starting a session
* We need better insight into drop-off points: we only know they clicked out to the docs and whether they started a session, and don’t know where they’re struggling.

### Other data/context

Historically Teleport Cloud has more local user sessions than AD sessions (last 180 days, 36.3k local vs 30.1k AD). This may be skewed by us not getting metrics from major self-hosted customers who are likely using AD.

### Appetite/Resources

We want the scope of this project to be doable for a full-stack engineer with some design input, within 6 weeks.

## Solution

### Hypothesis

Adding a guided experience for adding new local user Windows Desktop resources will improve customer success with the setup, and improve our data on where users struggle with the process. This guided experience assumes users are new to Teleport.

### Use Cases/Requirements
* As an admin
  * I need to set up one server to prove value of adding a local user Windows resource to my cluster
  * I want to know ahead of time what tasks are in the process I am about to start and about how long it will take
  * I want to know ahead of time the prerequisites required to go through this process (i.e. creating a linux host that can connect to the Windows Desktop)
  * I may want to go through those steps and return to this process at a later time
  * I want to have as much of the process done through easily run scripts as possible
  * I want to have links out to the docs for complex topics scripts are doing for me
  * I want to know what those scripts are doing to learn about how Teleport works
  * I need to be able to easily reverse anything I do in this process
  * I need to understand how to set up roles so that the right users can start sessions quickly, and to do so
  * I want to connect to the Windows Desktop to confirm that the process worked
* As a Teleporter
  * I need to know how many people complete guides with this information provided vs the old baseline.

### Rabbit Holes

A thing that could potentially blow out the scope is trying to over-automate things for the user. To keep in scope as implementation progresses, we should favor adding more to the prerequisites that a user should set up over trying to write more complicated scripts.

We could try to do a guide for AD setup, but that is much more complex. There have been past attempts at making a wizard for AD setup that failed due to the complexity, and a decision was made that it should be referred to SE/IE teams.

### Out of Bounds

We don’t want to overhaul the Windows experience right now. It would be possible to streamline this process by not requiring a Windows Desktop Service running on a Linux box, and this is something that customers have asked for, but that would push us well out of our appetite for this project. We’ll revisit this later as another effort.

### Outlines/Sketches

An initial draft of the page flow is as follows:
* Before You Start
  * Steps overview
  * Prequisites
    * Windows machine accessible by a linux box
    * Must have hostname
* Set up Windows box
  * Generate script to install components (similar to linux ssh flow)
* Setup Service
  * Description of what the script does
  * Inputs
    * Name of Windows box
    * Hostname of Windows box
    * Labels
  * Generate script
  * Confirmation it is running?
* Roles
  * Configure roles as part of guide to make sure this user and desired users can access it
  * Add labels we added in previous step
  * Assign to users
  * Verify there is at least one role that maps to the current user
* Connect
  * Connect to the Windows VM
* Next steps
  * Generate scripts that undo everything or provide manual instructions
  * Links to relevant docs they may want to read to dive deeper

## Value

### Opportunity

The more TPRs enrolled in a trial the higher the eventual ARR will be from that account. But more relevant to the PoV process is that larger resource:session ratios also correlate with higher contract values. We need to not only guide people through resource addition but with getting them successful with enabling sessions.

### Measuring Success
* An Insight in PostHog on the funnel of the guide, so we can track where users drop off.
* Local-user Windows Desktop resources added
* Local-user Windows Desktop sessions started

## Implementation

### Design
To be added to by design team.

### Engineering
To be added to by engineering team.
