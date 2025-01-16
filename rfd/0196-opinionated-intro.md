---
authors: Dave Sudia (david.sudia@goteleport.com)
state: draft
---

# RFD 0196 - Opinionated Intro to the Product

## Overview

### Problem

* Over the years we have done a great job of adding features that users need: connection types, interfaces (web, command line, Connect, etc), role customizations, etc. This is a good thing, but has made the product intimidating to new users. They are not sure where to start, and out of all our options which are the best for them.
* We have simplified some onboarding flows to make it easier to get started, like the script to quickly add an SSH server, or the RDS/EC2 auto-discover paths. However, these obscure the architecture of Teleport, so when things don’t work it’s difficult for users to debug. They don’t understand how Teleport works, so we end up burning SE and support time helping to debug, and teaching users. When they want to move to long-term solutions like infrastructure-as-code it’s not clear how to recreate their setup because they don’t understand how it is currently built.

### Other data/context

This RFD is based on interviews with leaders and implementers from the Product, Engineering, Design, Marketing and SE teams. There is widespread consensus that confusion for new users is an issue in the product, and a more opinionated introduction to the product is needed. It’s difficult to provide a precise metric for how much business we are losing to this issue, and how much this is costing us in person-hours, but qualitatively every interviewee agreed both of those things are happening.

### Appetite/Resources

We want the scope of this project to be doable for a full-stack engineer, a designer, and a product engineer, within one quarter.

### Solution

#### Hypothesis

Having a menu of tutorials available for new admins in a cluster will improve user experience, leading to faster and more successful setup (shorter PoVs, more converted trials), utilization of more features, and reduced burden on SEs during setup and support during contract. This menu should simultaneously make it easy for users to get started with the most relevant actions they can take to get started, and act as a knowledge map to expose what they actions/features could take advantage of. The menu will be a Choose-Your-Own-Adventure rather than a prescriptive order of tasks to complete, but each item will be opinionated on how to execute it, in order to help users learn about the product.

#### Use Cases/Requirements
* As an admin for a new cluster
  * I want to find the most relevant guides for my use case as fast as possible.
  * I want to be able to hide and show the intro at any time.
  * I want to be able to hide the intro permanently, or in some way mark that I’m done with it.
  * I want to be able to remove/hide parts of the intro that aren’t relevant to me.
  * I want to gain an understanding of how Teleport works when I start using the product.
  * I want to be shown how to do a given individual task in a directed, opinionated way. I can form my own opinions as I become more familiar with the product.
  * As I do tasks I want to become familiar with the ways I could use the product so I can form my own opinions about the best way my organization can use it.
  * I want to know why I would use a given feature or interface of the product, its strengths and weaknesses, to help form my own opinion about how to use it best in my organization.
* As a Teleporter
  * I want this intro to be easy to maintain and keep current with the product and docs.
  * I want to measure usage of the intro.
  * I want new admins to be exposed to the various interfaces of the product like command-line, web, Connect.

### Rabbit Holes

This implementation is limited to the persona of a new admin. In the future we should have similar intros for other personas like dev users, SSO users (an access use case similar to our own internal platform.teleport.sh), admins coming into an existing cluster, etc. Product will do future work with design and other teams to formulate these personas. This means we need to define “a new admin.” Since we do not have user types in the app, simply roles, we’ll need to define this via one or a combination of roles, and this should only show up for newly created clusters.

### Out of Bounds

## Outlines/Sketches

There is an intro page that is the landing page for a new user, and is accessible somehow, possibly from the left side menu, eventually removable/hideable by the user. The intro page gives context on the guide itself and how to use it (i.e. that things don’t have to be done in order, things can be hidden, etc.

Below the context is a list of guides split into themes that form a knowledge map. Individual items can be hidden.

Above the guide is a progress bar or other indicator that gives the user some sense of how close to “done” they are with learning the product. This is not an indicator of them being done fully implementing the product.

Within the content we want to weave a concepts in:
* Tokens
* Agents
* Proxy
* The role of roles (sorry)

Additionally we want to make it possible for the users to self-select relevant content where possible. To add every implementation form for every topic (i.e. for SSH showing scripted setup, IaC setup, point-and-click setup) would be difficult to maintain and confusing. The point is to be opinionated. By showing 1-2 of our implementation details to each guide users, even if just by reading some, can get a sense of the whole and start to figure out how they want to do things for real. To help users self-select and navigate to relevant content, we’ll tag each guide with methods covered. Within the content we can make nods to other methods (i.e. “you could use these installation steps in a cloudinit script in IaC”).

Tags:
* UI
  * Connect
  * CLI
  * Console
* Technology
  * Linux scripting
  * Infrastructure-as-Code
  * Kubernetes CRDs
  * Point-and-Click
* Team relevance
  * Security
  * Compliance
  * FIPS/FedRAMP
  * Scalability

### Rough outline of the content
* Quickstart
  * You are a local user, what does that mean?
  * You have the “access” role, what does that mean?
  * Add a resource of your choice
    * Weave in intro to tokens and agents
  * Access it
  * Check audit logs
* Users/Auth
  * Local
  * SSO
* Roles
  * Strategy
  * Creation
  * Assignment
* Resources
  * SSH
  * DB
    * RDS
    * Self-Hosted
  * Application
  * Cloud Console
  * Kubernetes
  * Windows
  * Machine ID
* Features
  * Audit Log/Session Recording
  * Access Requests
  * Session Locks/Monitoring
  * Access Lists

The general flow here is “add users, make roles and assign them, add resources that those users with those roles can now access, do things with the access that has happened.” The quickstart section takes them through that flow for their own user and default role, and then they can expand that schema to the rest of the product.

Key context to provide in each guide is some pros/cons and the why of each topic, especially for topics like Users/Auth. Why use local users vs SSO? This is a good place to incorporate info from SE like in the SSO one saying “you may not have permission to set up your SSO yet, in that case local users are a great place to get started.”

Rought sketches and jumping off points for design are here:
* [FigJam board](https://www.figma.com/board/GD04hsWTsxwQyx6LrAqGg3/Opinionated-Intro-to-the-Platform?node-id=0-1&t=0kCVuGbmrsFmv5FH-1)

#### Last Resort

Some users just aren’t able to add any resources because of their permissions. We will give them an escape hatch of linking to our Instruqt Labs, but need to figure out a way to only show that if absolutely necessary.

## Value

### Opportunity

This effort will hopefully reduce PoV length and increase conversion rate, saving staff time and increasing ARR. For organizations that come in through trials or opt for self-guided PoVs we hope to see increased conversion. If we increase conversion rates even 20% that represents $X in new ARR.

### Measuring Success
* UI events into our user-measurement data for usage of the intro menu. We should see steady usage of the menu overall. We can target changes/improvements based on usage of each portion of it.
* Faster time to first session
* Increased resources added during PoVs
* Increased conversion of PoVs and trials
* Decreased duration of PoVs

## Implementation

### Design

### Engineering

### Docs

### Product

Product will be responsible for writing the majority of the content and coordinating the other teams.
 
