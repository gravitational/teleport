---
authors: Stephen Levine (stephen.levine@goteleport.com)
state: draft
---

# RFD 223 - Installation and Upgrade Documentation Organization

# Required Approvers
* Engineering: @klizhentas || @r0mant
* Product / Design: @roraback && @ptgott

## What

When users get started with Teleport, they often look for pages of documentation related to downloading and installing
a Teleport component that they need to use.

This RFD proposes that we reorganize our documentation to make it easier to find these pages.

## Why

Currently, it can be difficult for users to find this documentation because it lives in different places depending
on the component, target OS/platform, and other factors.

For example, a common user workflow is installing a Teleport agent on an EC2 instance and connecting it to the cluster.
Users who do not read the documentation and follow the instructions in the Teleport Web UI have an easy path forward
that involves executing a single command.

However, a user who attempts to read the documentation might follow a path that looks like this:

1. Navigates to goteleport.com/docs
2. Finds no information in the sidebar about installation or upgrading
3. Clicks each sidebar item and finds "Installation" under "Introduction"
4. Clicks "Installation," then "Linux"
5. Sees instructions for installing a Teleport cluster, continues to scroll down to agent
6. Runs an install script that results in an auto-updating agent that is not configured
7. Sees that they need to manually enroll the agent, clicks "Enroll Resources"
8. Clicks "Linux Servers (section)"
9. Sees a page with four different guides that are named very similarly
10. Clicks the "Getting Started Guide", or may get directed there by clicking one of the other guides
11. Sees the install script instructions again, along with other outdated installation instructions
12. Finds and executes manual joining instructions, which are unnecessary if the Web UI is used.

The Teleport Downloads page has a similar issue that is actively being addressed.

This RFD proposes that we add one new top-level "Platform" section, with three subsections:

1. "Installation"
2. "Upgrading"
3. "Operations"

These sections would cover all Teleport components.
Installation sections will cover setup for Managed Updates, and link to more details Upgrading docs where appropriate.

Notably, the Platform section differs from the marketing-branded categories for our product features because it is cross-cutting,
operational, and (in most cases) does not involve in-product UX workflows.
Users install Teleport first, ensure they have a way to upgrade it, ensure they have day-two operations like backups
configured, and then they (or different end-users) dive into the product itself.

A complete user story for a Teleport installation workflow may need to involve in-product configuration.
For example, it is common to configure roles after installing a self-hosted cluster.
As we cannot repeat these instructions for every combination of component and target platform, we can rely on cross-linking to ensure
that the user is guided to the correct page regardless of their starting point.

## Details

### Overview of All Components with Customer-Managed Operational Lifecycles

Teleport includes a large number of independently installable components that require dedicated installation
instructions for an even larger number of target platforms:

1. Cluster Installation
   1. Self-Hosted on Linux
   2. Self-Hosted on Kubernetes
      1. Digital Ocean
      2. Helm
      3. Helm with ArgoCD
      4. EKS
      5. GKE
      6. AKS
      7. IBM Cloud
      8. Helm “Custom”
   3. Self-Hosted on Docker
   4. Self-Hosted on EC2
   5. Self-Hosted on GCP
   6. Self-Hosted on OCI
   7. Self-Hosted on Azure
   8. Self-Hosted via Source Code
2. Agent Installation
   1. Agent on Kubernetes
   2. Agent on Docker
   3. Agent on EC2
      1. Teleport AMI
      2. Custom AMI
      3. Via Deploy Service / AWS OIDC
   4. Agent on GCP
   5. Agent on OCI
   6. Agent on Azure
3. Client Installation
   1. Human
      1. Linux
         1. Resource Access via tsh
         2. Resource Access via Connect
         3. Cluster Admin via tctl
      2. MacOS
         1. Resource Access via tsh
         2. Resource Access via Connect
         3. Cluster Admin via tctl
      3. Windows
         1. Resource Access via tsh
         2. Resource Access via Connect
         3. Cluster Admin via tctl
   2. Machine
      1. Linux
         1. Resource Access via tbot
      2. MacOS
         1. Resource Access via tbot
      3. Windows
         1. Resource Access via tbot
4. Integrations
   1. Plugins
      1. Event Handler
      2. Discord
      3. Email
      4. JIRA
      5. Mattermost
      6. Slack
      7. Msteams
      8. Datadog
      9. Pagerduty
   2. Configuration Systems
      1. Kubernetes API via Teleport Operator
      2. Terraform via Teleport Terraform Provider

Users will generally know which target platform they need to perform the installation on, but they may not know
which Teleport component they need to install.

A Teleport user may need to mix and match instructions for their use case.
For example, they may need to install a self-hosted cluster on GKE, deploy an agent into EKS, and then configure the agent to allow App access.

### Proposed Organization

The following organization is proposed, nested under a top-level Platform section:

1. Installation - New page, maps each use case for Teleport to the correct component
   1. Installing Teleport Agents
   2. Installing Teleport Client Tools
   3. Installing Self-Hosted Teleport Clusters
   4. Installing Teleport Plugins & Integrations
2. Upgrading - New page, explains the relevance and importance of upgrading each component
   1. Upgrading Teleport Agents
   2. Upgrading Teleport Client Tools
   3. Upgrading Self-Hosted Teleport Clusters
   4. Upgrading Teleport Plugins & Integrations
3. Operations - New page, explains what it means to operate Teleport (backups, troubleshooting, speciality config like KMS, multi-region, etc.)
   1. Operating Teleport Agents
   2. Operating Teleport Client Tools
   3. Operating Self-Hosted Teleport Clusters
   4. Operating Teleport Plugins & Integrations

Notably, cluster operations that are not handled by Cloud should be included under the marketing-branded sections, not Platform.
For example, a guide to configuring a cluster to support AWS KMS would be included in Operating Self-Hosted Teleport Clusters, while
a guide for getting started with Teleport roles would be included in the "Zero Trust Access" section.
The only exception to this rule is global, shared configuration that is directly related to platform operations and does not
fit into the marketing-branded sections. For example, some of the `cluster_*` or `autoupdate_*` resources may be described in Platform.
These are generally optional for Cloud users to configure.

Installation instructions should not be complete user guides that include in-product configuration.
Installation instructions should link to user guides when they are more appropriate, and user guides should link to
(or import) installation instructions where needed.

### Proposed Action Plan

The proposed action plan is iterative.
It does not involve a large re-write of any existing documentation, and relies on links between sections where appropriate.

Note: these are best understood by opening https://goteleport.com/docs and following from the top.

- Create new sections and new pages described above.
- For each existing installation page, separate agent, client, and cluster sections into new pages. No single guide or instruction page should address more than one.
- "Installation -> Installing Teleport Agents" contains a link to "Enrolling Resources" with separate "Custom Agent Installation" instructions
- "Introduction -> Installation" moves to "Platform -> Installation"
- "Introduction -> Upgrading" moves to "Platform -> Upgrading"
- "Introduction -> Migrate Teleport Plans" moves to "Platform -> Cloud" (tentative)
- "Zero Trust Access -> Exporting Teleport Audit Events" contains links to "Platform -> Installation -> Installing Teleport Plugins & Integrations -> Event Exporter", and vice-versa
- "Zero Trust Access -> Infrastructure as Code -> Teleport Kubernetes Operator" moves to "Platform -> Installation -> Installing Teleport Plugins & Integrations -> Teleport Kubernetes Operator" (install guides only, with cross-linking)
- "Zero Trust Access -> Cluster management -> Cluster Administration guides -> Uninstall Teleport" moves to "Platform -> Installation" (subsections as appropriate)
- "Zero Trust Access -> Cluster management -> Cluster Administration guides -> Run Teleport as a Daemon" moves to "Platform -> Installation -> Installing Teleport Agents -> Linux Servers"
- "Zero Trust Access -> Cluster management" moves to "Platform -> Operations -> Operating Self-Hosted Teleport Clouds"
- "Zero Trust Access -> Self-Hosting Teleport -> Guides for running Teleport using Helm" moves to "Platform -> Installation -> Installing Self-Hosted Clusters -> Kubernetes" (many subsections)
- "Zero Trust Access -> Self-Hosting Teleport" moves to "Platform -> Operations -> Operating Self-Hosted Teleport Clusters" (remaining sections)
- "Machine & Workload Identity" -> Machine ID -> Deploy tbot" links to "Installation -> Installing Teleport Client Tools -> tbot" (or vice-versa, at MWI team discretion.)
- "Identity Security -> Self-Hosting Teleport Access Graph" moves to "Installation -> Installing Self-Hosted Teleport Clusters -> Access Graph"
- "Enroll Resources -> Joining Teleport Agents" moves to "Platform -> Installation -> Installing Teleport Agents" (such that Enroll Resources is always use-case driven, and links to Platform -> Installation for Azure, GCP, etc, instructions.)

Marketing-branded sections may link to Installation, Upgrading, or Operations sections where relevant, and vice-versa.

### Confusing Workflow Reference

This list is an incomplete collection of various flows we need to fix. See the Action Plan above for additional examples.

1. If a Cloud user following "Enroll Resource -> Applications" needs the agent for their application to run on Kubernetes with ArgoCD, they must
   follow "Zero Trust Access -> Self-Hosting Teleport -> Guides for running Teleport using Helm", even though the instructions are not related to
   Access or Self-Hosted. Instead, they could follow a link to "Platform -> Installation -> Installing Teleport Agents" and immediately see
   "Platform -> Installation -> Installing Teleport Agents -> Kubernetes -> ArgoCD".
2. "Introduction -> Installation -> macOS" says that `teleport` is supported on macOS, but provides no guidance for configuring or using it.
3. "Introduction -> Installation -> Amazon EC2" links to AMIs and one possible Self-Hosted specific guide, but has no agent install guide.
4. Agent install docs live under "Introduction -> Upgrading -> Managed Updates for Agents (v2)", instead of a clear Installation page.

