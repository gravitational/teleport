---
authors: Roman Tkachenko (roman@goteleport.com), Xin Ding (xin@goteleport.com)
state: draft
---

# RFD 73 - Teleport Discover "Day 1" and "Day 2" Experiences

## Required approvers

@klizhentas && @xinding33

## What

Proposes a set of UX updates that improve the UX for the users connecting their
resources to Teleport.

## Related RFDs

- [RFD 38: AWS databases auto-discovery](./0038-database-access-aws-discovery.md)
- [RFD 57: AWS EC2 nodes auto-discovery](https://github.com/gravitational/teleport/pull/12410)

## Why

The proposal is aimed at providing an easy way for Teleport administrators to
connect their resources such as SSH servers, databases and Kubernetes clusters
to a Teleport cluster.

Over the past few releases Teleport has been adding automatic discovery
capabilities allowing it to find and register AWS databases, EC2 instances and
(WIP as of this writing) EKS clusters. Despite providing an improved UX compared
to registering the resources manually, connecting resources to cluster and
setting up auto-discovery remains cumbersome with multiple different
`teleport configure` CLI commands to run, configuration files to update and so
on.

Improvements proposed in this RFD aim to take advantage of the existing auto
discovery mechanisms Teleport has and provide a unified approach for users to
connect their resources.

## Scope

- Works with both self-hosted edition and Teleport Cloud.
- Works with the environments we currently support: AWS and self-hosted.
- Phase 1 will focus on the "Day 1" experience described below.
- Phase 2 and beyond will focus on "Day 2" and further tweaks to "Day 1"
  experience.
## Future work

1. Auto-discovery and configuration support for Kubernetes Access (currently being researched)
2. Auto-discovery and configuration support for Application Access
3. Auto-discovery and configuration support for Desktop Access
4. Extend support to GCP
5. Extend support to Azure
## UX

The Teleport Discover flow will aim to provide a guided experience for users
connecting their resources to a Teleport cluster in 2 main scenarios:

1. "Day-1 users": These users are new to Teleport and are exploring what it has
   to offer. Most likely they don't want to connect all their resources yet but
   would like to quickly get to success by connecting their first server or a
   database and see it work.

2. "Day-2 users": Users who are already somewhat familiar with Teleport, got
   their first resource connected and are exploring go-to-production options.
   They would like to have Teleport automatically discover and connect their
   cloud resources.

As much as possible, we want Teleport Discover users to remain in the Web UI,
meaning we don't want to ask them to download `tsh` or `Teleport Connect` or
visit https://goteleport.com/docs/. There are few reasons for this:

1. We want to encourage users to finish the "Day 1" and "Day 2" workflow. A good
   way to do this is limit distractions.

2. We want the least number of dependencies because more dependencies equals
   more potential blockers for users. For example, what if users have trouble
   downloading or installing `tsh`? We just introduced a class of possible
   failure modes.

Below, let's explore in more detail what the flow for both of these user
personas would look like.

## Day 1

Day 1 users should be greeted by a wizard-style dialog upon logging into the web
UI of a cluster that does not have any connected resources. The wizard will
guide the user through the flow of connecting their first resource.

Teleport web UI already provides some of the building blocks for the wizard in
the form of "Add server", "Add database", etc. pop-up dialogs but their
instructions are not friendly for newcomers and make it almost impossible to use
successfully without referring to the documentation.

Instead of separate dialogs, Teleport Discover wizard will provide a unified
experience to enable the flow described below.

Users should be able to add exactly 1 resource at a time. Users can exit the
wizard at any time but a confirmation modal should be presented to users so they
don't accidentally leave the workflow. The unified "Add resource" wizard should
also be available and prominently visible in the web UI allowing users to go
through the same flow in a non-empty cluster.

### Step 0. Initiate Teleport Discover workflow or skip

Ask the user if they want to initiate the Teleport Discover workflow or go
straight to the "dashboard" (i.e. the "servers" screen). This helps users
establish a mental model of Access Manager vs. Access Provider.

### Step 1. Select resource type

Ask the user what type of resource they would like to connect: an SSH server, a
database, an application, etc.

For SSH Servers, since the automatic installation script auto discovers the OS
and installs the correct binary, we don't need to ask users to provide any
additional information. Some helper text to let users know all supports OSes
would be beneficial

For Databases, since there are many options, we should allow users to further
filter down by deployment type: self-hosted or AWS. Once a deployment selection
has been made, present all support database types to the user.

### Step 2. Configure resource

For an SSH Server, present the automatic installation script to the user to copy
and paste. This script auto detects OS and installs the correct binary. No other
actions are required.

For a database, there are three steps:

1. Deploy a database agent (optional if user has already deployed at least one
   database agent)
2. Register the database
3. Configure mTLS (for self-server database) / Configure IAM policy (for AWS
   database)

### Step 3. Set up access

For an SSH Server, this step requires adding Linux principals / users.

For a database, the user needs to define the available logical databases and
users.

### Step 4. Test connection

This step is the same for all resources. There are two actions here:

1. Test connection: user clicks on a button and Teleport runs through a series
   of diagnostic tests to ensure that the connection is set up correct and can
   be established.
2. Connect: user clicks on a button and the connection is made in the Web UI.
   For example, for an SSH Server, clicking on this button would pop up a new
   tab with a session to the newly connected server.

### Day 2

Day 2 users already have gotten an initial success with connecting their
resources to Teleport by going through the guided wizard described above. They
have SSH and/or database agent(-s) installed and running.

As they're thinking about bringing a larger part of their infrastructure into
their Teleport cluster, this is where it makes sense for them to use Teleport's
auto-discovery mechanisms.

Currently, the auto-discovery can only be configured by updating the static
Teleport agent configuration file `teleport.yaml`:

```yaml
ssh_service:
  enabled: "yes"
  aws:
    - types: ["ec2"]
      regions: ["us-west-1"]
      tags:
        "*": "*"
db_service:
  enabled: "yes"
  aws:
    - types: ["rds"]
      regions: ["us-west-1"]
      tags:
        "*": "*"
```

Instead, Teleport will implement ability to configure auto-discovery (enable,
disable, specify resources types to discover, tags, etc.) dynamically via the
API which web UI will utilize.

Similar to application and database dynamic registration, auto-discovery
configuration will be turned into a resource (e.g. `kind: Discovery`) which will
be tied to a particular SSH or a database agent.

Web UI will provide a wizard-like dialog that will allow users to enable AWS
auto-discovery by going through the following flow:

1. Select the type of resource to discover e.g. EC2 instances, RDS databases, as
   well as regions and tags to filter by.
2. Select an existing agent that will be running the auto-discovery. In order to
   run EC2 discovery, there should be a running SSH agent. For RDS discovery, a
   database agent.
3. The selected agent will perform initial discovery according to the provided
   filters. This can be implemented by providing an API for the web UI to create
   a "discovery request" which agents will watch.
4. The agent will attempt to fulfill the discovery request and will report
   errors, e.g. insufficient IAM policy, to the user. This can be implemented by
   filling out a Status field on the agent's resource spec.
5. If successful, the UI wizard will display all resources matching the
   discovery request for the user to inspect and confirm. If unsatisfied, the
   user can retrace to an earlier step to re-run the discovery.
6. After user confirmation, the agent will update its auto-discovery config
   which will kick-off regular auto-discovery mechanisms for SSH and RDS.
