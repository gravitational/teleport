---
authors: Dave Sudia (david.sudia@goteleport.com)
state: draft
---

# RFD 0187 - Redesign Enroll New Resource Page

## Required Approvers

* Engineering: @r0mant
* Product: @xinding33

## Overview

### Problem

* The current “Enroll a Resource” flow is confusing for new users.
  * This slows down PoVs and trials hurts their success by making it harder for new user to get started proving out how successful Teleport will be in meeting their needs.
* There are overlapping ways of enrolling a resource that we do not differentiate
enough for the user. Example: if I have an ubuntu server running in EC2, do I pick the Ubuntu flow or the EC2 Auto-discovery?
  * As a new user I may want to simply see how the experience of adding and logging into a server works. I don’t know that picking the latter will start me on a very complex path that requires lots of permissions.
  * Or I may need to see how Teleport works at scale before extending a trial into a PoV and don’t know the latter would better suit my needs.
* The highest drop off in the resource enrollment process is on the first page of a guide. This might be because people click on a guide and then realize it is not what they’re looking for.
* The current page’s search function is not accurate enough; it doesn’t cover all the terms that users may search for (e.g., searching “SSH” does not find relevant options).
* The lack of a filter system combined with the limited search makes it challenging for users to find the right options for their needs quickly. The goal is to make it easier for users to locate the correct resource type, and correct level of complexity.

### Other data/context

* Because of the above issues (and issues with individual guides), for assisted PoVs, the SE team now recommends people avoid the Enroll a Resource page, and go straight to the docs.
* Data for the above was collected from interviews with users and reports/interviews with the SE team

### Appetite/Resources
This project aims to be scoped to be feasible for one full-stack engineer with support from design and product input, aiming for a completion timeline of approximately 6 weeks.

## Solution

### Hypothesis

By providing an improved experience for picking options on the Enroll a Resource page, users will be able to find relevant guides more easily, and new users in particular will be better equipped to determine if Teleport is a good solution for their needs. An improved enrollment process will improve PoV and trial success rates.

### Use Cases/Requirements
* As an admin
  * I need to find the correct enrollment guide based on the resource type I want to protect
  * I want to filter options by resource type (e.g. db, server, k8s) and cloud provider/hosting platform (e.g., AWS, Azure, self-hosted)
  * I want a clear path for both simpler and more complex configurations, i.e. for Day 1 (initial setup/trial) and Day 2 (larger scale) operations
* As a new user to the product
  * I want the platform to help me determine what kind of resources I should protect based on common usage or best practices
  * I want clear, simple paths to follow that align with my current organizational needs
  * I want to have an idea of how complex each setup is before committing to it
* As a Teleporter
  * I want to know if users’ use of search increases or decreases
  * I want to know if users use the filters
  * I want to know which guides are most-accessed during trial and PoV periods to determine what to highlight for new users

### Rabbit Holes

We could go really granular on filters, which would require creating and maintaining more identifiers on each resource panel. We should minimize the filter list, maybe by mirroring the current dashboard view options. Doing a guided questionnaire came up during brainstorming. How/where/when to access that and how long/complex it would be could blow out scope. If scope needs to be cut, it could be cut there.

### Out of Bounds

This project does not intend to overhaul existing resource-specific documentation or setup guides but aims to streamline how users access and begin these guides. There are issues with individual guides that contribute to PoV/trial failure, but enhancements to setup guides fall under other RFDs.

## Outlines/Sketches
The [FigJam board](https://www.figma.com/board/65tcBiTgE9B9j05NVO669b/Untitled?node-id=0-1&t=T5ukhCY6H8x3Jxud-1) has original notes.

#### User Interface Changes
* Implement “pinned” enrollment guides. Guides that are good for getting started will be pinned by default for all new clusters.
  * Users can unpin when they don’t want those at the top anymore.
  * Users can pin/unpin additional cards as desired
* Add filters
  * Hosting method/platform (AWS/GCP/Azure/Self-Hosted)
  * Resource type (DB/Server/Kubernetes/Desktop/Application)
* Improve search to capture more relevant terms (i.e. “SSH”, “k8s”, linux distros)
* Collapse Linux options into a unified category, simplifying the selection process for users who may not need to distinguish between distributions at the initial stage
  * Make searchable by distro (ubuntu, redhat, centos, debian, etc)

## Value

### Opportunity

We know that the shorter a PoV process the higher our win rate. Improving the resource enrollment flow with a focus on helping new users get to the right guides (whether that is a test of a quick setup or of a scale solution) will hopefully lead to higher conversion rates for PoVs and trials.

### Measuring Success

* Monitor overall improvements in conversion rates from initial interest (e.g., entering the “Enroll a Resource” page) to starting active sessions.
* Set target metrics for reducing drop-off rates on the first page of guides (where we have the most drop off)

## Implementation

### Design
* [Link to page design](https://www.figma.com/design/uLevdNsEnIvvLDSZ9sQqXM/Discover-Access?node-id=2730-11748&t=xsYYolkkJeUJAhGi-4)
* [Link to resource card states](https://www.figma.com/design/Gpjs9vjhzUKF1GDbeG9JGE/Application-Design-System?node-id=18913-20082&t=teeY2YmoBg0jphJi-4)

Some notes:
* The resource cards that will be pinned by default are below.
* **Every** resource card should have a second line with additional contextual info—precise details to come.
* We cut filtering by scale/complexity and by guided/docs during design phase (and they're now deleted from the filters list above).
* Pinned items appear both at the top of the page as a different-style card AND in the flow of the page.
* Not necessarily included in this scope of work, but I also updated and expanded our AWS service icons in the design library and exported to Google Drive.

#### Specs

##### Default pinned cards
* Linux
* Self-Hosted Kubernetes
* Web Application
* Connect My Computer

##### New Card Content - only changed/updated cards
* Linux Server
    * SSH
* Connect My Computer
    * Teleport Desktop App
* Dynamic Database Registration
    * Self-Hosted
* SAML Application
    * Teleport as IDP
* Web Application
    * HTTP Proxy
* MongoDB Atlas
    * Database as a Service
* Snowflake
    * Database as a Service
* AWS CLI/Console Access
    * Amazon Web Services (AWS)
* Grafana SAML
    * Teleport as IDP

### Engineering
To be added to by engineering team.
