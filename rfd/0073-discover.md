---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 73 - Teleport Discover

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
capabilities allowing it to find and register AWS databases, EC2 instances
and (WIP as of this writing) EKS clusters. Despite providing an improved UX
compared to registering the resources manually, connecting resources to cluster
and setting up auto-discovery remains cumbersome with multiple different
`teleport configure` CLI commands to run, configuration files to update and
so on.

Improvements proposed in thie RFD aim to take advantage of the existing auto
discovery mechanisms Teleport has and provide a unified approach for users to
connect their resources.

## Scope

- Works with both self-hosted edition and Teleport Cloud.
- Works with the environments we currently support: AWS and self-hosted.
- Phase 1 will focus on the "Day 1" experience described below.
- Phase 2 and beyond will focus on "Day 2" and further tweaks to "Day 1" experience.

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

Let's explore in more detail what the flow for both of these user personas
would look like.

## Day 1

Day 1 users should be greeted by a wizard-style dialog upon logging into the
web UI of a cluster that does not have any connected resources. The wizard
will guide the user through the flow of connecting their first resource.

Teleport web UI already provides some of the building blocks for the wizard in
the form of "Add server", "Add database", etc. pop-up dialogs but their
instructions are not friendly for newcomers and make it almost impossible to
use successfully without referring to the documentation.

Instead of separate dialogs, Teleport Discover wizard will provide a unified
experience to enable the flow described below.

Users should be able to navigate between the dialog steps back and forth to
connect multiple resource if needed. The unified "Add resource" wizard should
also be available and prominently visible in the web UI allowing users to go
through the same flow in a non-empty cluster.

### Step 1. Gather information

Ask the user what type of resource they would like to connect: an SSH server,
a database, an application, etc.

Then ask the user where the resource is located: self-hosted or AWS. For
databases, also present the supported protocol options. For self-hosted:
PostgreSQL, MySQL, MongoDB, Redis, SQL Server. For AWS: RDS PostgreSQL, RDS
MySQL, RDS SQL Server, Elasticache, MemoryDB.

### Step 2. Install Teleport

For SSH, instruct the user to download and install `teleport` binary on the
server they're intending to connect. Use tabs to display per-distro install
instructions similar to [the docs](https://goteleport.com/docs/server-access/getting-started/#step-14-install-teleport-on-your-linux-host).

For a database, display the same instructions making it clear that Teleport
should be installed on a node that can reach the database.

### Step 3. Configure node

For SSH, this step is a no-op as it doesn't need any additional configuration
so users will proceed to starting an agent at step 4.

For databases, this step will inform the user of any additional configuration
they may need in order to get database access to work. It will depend on the
database type and where the database is hosted.

For self-hosted databases, in the initial version the wizard will display the
links to the respective sections of the documentation guides for preparing
the node (e.g. joining to AD domain for SQL Server) and show the commands for
creating a database user.

For AWS databases that use IAM authentication (PostgreSQL, MySQL), the wizard
will additionally display an appropriate `teleport db configure bootstrap`
command for the user to run which will configure IAM permissions.

### Step 4. Start agent

For SSH, display the command for the user to run on their node, showing either
"Automatic" or "AWS" commands from the existing "Add server" dialog depending
on whether self-hosted or AWS option was picked in step 1.

For a database, display the appropriate `teleport db start` command similar to
the existing "Add database" dialog.

### Step 5. Configure role

Teleport roles by default only include internal user traits as allowed SSH
logins (`{{internal.logins}}`) and database users (`{{internal.db_users}}`)
and database names (`{{internal.db_names}}`). This results into users getting
access denied errors unless they update their roles explicitly.

On this step the wizard will ask the user for their intended SSH logins and/or
database users/names and update the internal user traits appropriately so the
users with the built-in `access` role will be allowed access to their resources.

### Step 6. Connect

This step will ask the user to test the connectivity by logging into their
cluster with `tsh login` and running appropriate connect command, `tsh ssh`
or `tsh db connect`.

### Day 2

Day 2 users already have gotten an initial success with connecting their
resources to Teleport by going through the guided wizard described above.
They have SSH and/or database agent(-s) installed and running.

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
configuration will be turned into a resource (e.g. `kind: Discovery`) which
will be tied to a particular SSH or a database agent.

Web UI will provide a wizard-like dialog that will allow users to enable AWS
auto-discovery by going through the following flow:

1. Select the type of resource to discover e.g. EC2 instances, RDS databases,
   as well as regions and tags to filter by.
2. Select an existing agent that will be running the auto-discovery. In order
   to run EC2 discovery, there should be a running SSH agent. For RDS discovery,
   a database agent.
3. The selected agent will perform initial discovery according to the provided
   filters. This can be implemented by providing an API for the web UI to create
   a "discovery request" which agents will watch.
4. The agent will attempt to fullfill the discovery request and will report
   errors, e.g. insufficient IAM policy, to the user. This can be implemented
   by filling out a Status field on the agent's resource spec.
5. If successful, the UI wizard will display all resources matching the
   discovery request for the user to inspect and confirm. If unsatisfied, the
   user can retrace to an earlier step to re-run the discovery.
6. After user confirmation, the agent will update its auto-discovery config
   which will kick-off regular auto-discovery mechanisms for SSH and RDS.
