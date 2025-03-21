---
authors: Dave Sudia (david.sudia@goteleport.com)
state: draft
---

# RFD 0188 - Resource Enrollment Overview/Prereqs

## Required Approvers

* Engineering: @r0mant
* Product: @xinding33

## Overview

### Problem

* Users enrolling new resources sometimes get multiple steps in before they realize they don’t have the ability to continue with the process. This can be because
  * They don’t have required permissions, for instance
    * In an AWS account to create a discovery service
    * In a database server to grant roles to users
  * They don’t have information they need prepared
    * Names of databases/users/etc
  * Their environment does not match, i.e. they pick EC2 Auto Enrollment but don’t have their instances in Fleet Manager
* The guide they’ve chosen doesn’t match their need/intent. They might pick a solution that tries to auto-enroll many resources when they just want to try one, or vice versa.
* Because of user’s common issues with the guides, SEs are now directing users away from the Enroll New Resource page to the docs.
* Given that many of our highest-value PoVs are self-guided and don’t benefit from SE consultation, it’s even more important to have quality guides that users can succeed with.

### Other data/context
* These are common issues reported by SEs and in user interviews

### Appetite/Resources
We want the scope of this project to be doable for a full stack engineer with assistance from one  product engineer and one designer, within 6 weeks.

## Solution

### Hypothesis

At the beginning of each guide, either on a new page (for most guides) or in a new section (like with app enrollment that is a single pop-up panel) we will provide a brief overview of the process: what it entails, what the guide’s goal is (set up all your machines, set up one, etc.) and the best use case for it. We will also provide a list of prerequisites like permissions required to successfully complete the guide, and data required like OS/database names.

### Use Cases/Requirements
* As an admin
  * I need to be successful in completing enrollment guides the first time I try.
  * I need to know what permissions and data I need to complete a guide.
  * I want to use guides that best match my intent and needs.
* As a Teleporter
  * I need to know how many people complete guides with this information provided vs the old baseline.

### Rabbit Holes

### Out of Bounds

Some of the guides could use a change in the order of their steps or pages, and several need fixes to be more reliable. This RFD does not capture those efforts. We believe a quick win to improve success rates is simply to give users more information before they get started. We want to improve specific guides but in this first effort we are limiting ourselves to the faster work on the frontend.

### Outlines/Sketches
* MacOS
  * Overview
    * Sets up a single machine
    * This is intended for machines like Minis used for builds, not for user laptops/desktops
    * Installs an agent binary and a teleport.yaml config file and uses a token with Node permissions to add it as a machine to access
  * PreReqs
    * Existing terminal access to the machine, via SSH (can be removed after installation) or desktop
    * List of OS users you want to connect as
    * Particular minimum version of MacOS supported?
* Amazon Linux/Ubuntu/RHEL/Debian
  * Overview
    * Sets up a single machine, good for getting started quickly with linux access
    * Installs an agent binary and a teleport.yaml config file and uses a token with Node permissions to add it as a machine to access
  * PreReqs
    * Existing SSH access to the machine (can be removed after installation)
    * Bump supported versions to supported ones
      * Debian 10+
      * Ubuntu 18.04+
      * RHEL/CentOS Stream 9+
    * List of OS users you will want to connect as
* Application
  * Automatic
    * Overview
      * Installs an agent binary and a teleport.yaml config file, uses a token with App permissions.
      * If teleport is already running to connect the node as a resource, you will need to make manual changes: add “app” to token roles, add the app config, and restart teleport service.
    * PreReqs
      * SSH access to the machine to run the script
  * Manual
    * Overview
      * If teleport is already running to connect the node as a resource, you will need to make manual changes: add “app” to token roles, add the app config, and restart teleport service.
* AWS MySQL/Aurora/Postgres
  * Overview
    * This process can be used to enroll one db or auto-discover all RDS dbs in an area.
    * As part of this process, you will be deploying a Database Service that proxies to the db. We provide options for doing this automatically or manually.
  * PreReqs
    * Existing AWS OIDC integration
    * Or, to set one up, you will need X permissions in AWS
    * Region of the database you want to connect
    * The VPC your database is in
    * Known subnets that have an outbound internet route and a local route to the database subnets.
    * A security group for the proxy that allows X ingress and X egress
    * List of database users you want to be able to connect as
    * List of database names within the database server you want to connect to
    * Existing access to the database to grant a role to the desired users
    * Ability to configure IAM policy
* EC2 Auto-Enrollment
  * Overview
    * This process is used to enroll all Systems/Fleet Manager managed EC2 instances in a geographical region
  * PreReqs
    * Existing AWS OIDC integration
    * Or, to set one up, you will need X permissions in AWS
    * Ability to add managed policies to EC2 instance profiles
    * SSM Agent running on any/all instances you want to enroll (e.g. you must be able to see them in Fleet Manager)
    * List of OS users you want to be able to connect as
* K8s
  * Overview
    * This guide uses Helm to install the Teleport agent into a cluster, and by default turns on auto-discovery of all apps in the cluster
  * PreReqs
    * Egress from the cluster to Teleport
    * Kubernetes API access to install the Helm chart
* Self-hosted MySQL/Postgres
  * Overview
    * You will be installing and running a database proxy service
    * This can be done in a central location so that it can access many dbs (better for scale)
    * Or locally next to the db you want to access (better for experiment/getting started)
    * Either way proxy must have egress to the Teleport cluster and ability to reach the database via a hostname/port
    * You will be configuring mTLS with the Teleport proxy
  * PreReqs
    * SSH access to the server running the database, and ability to either SCP files or run a command to get the TLS files from Teleport
    * Ability to modify the configuration and authentication files for the database
    * List of database users you want like to connect as
    * List of database names you want to connect to
* Desktop
  * These cases will be handled in an RFD for a new Desktop Access guide

## Value

### Opportunity

Improving user success in enrolling new resources should improve PoV and trial success, particularly in self-guided PoVs, which represent some of our highest value PoVs.

### Measuring Success
* Funnel data about where users fall off during resource enrollment guides. We should see an improvement in completion vs the current baseline.

## Implementation

### Design
To be added to by design team.

### Engineering
To be added to by engineering team.
