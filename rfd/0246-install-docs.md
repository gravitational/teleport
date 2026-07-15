---
authors: Stephen Levine (stephen.levine@goteleport.com)
state: draft
---

# RFD 246 - Installation and Upgrade Documentation Organization

# Required Approvers
* R&D Owner: @roraback
* Docs: @ptgott

## What

When users get started with Teleport, they often look for pages of documentation related to downloading and installing
a Teleport component that they need to use.

This RFD proposes that we reorganize our documentation to make it easier to find these pages.

## Why

Currently, it is difficult for users to find installation documentation because it lives in different places depending
on the component, target OS/platform, and other factors. Teleport includes multiple components that are interesting to
different users, and each component has many different installation options depending on an individual user's environment.

A company using Teleport may have:
- Operators that need to know how to install a self-hosted cluster on their chosen cloud provider,
- Individual operations teams that need to know how to install Teleport agents into a Kubernetes cluster hosted by a different cloud provider, and
- Developers that need to know how to install Teleport clients on their local workstations.

For example, a common user workflow is installing a Teleport agent on an EC2 instance and connecting it to the cluster.
Users who do not read the documentation and follow the instructions in the Teleport Web UI have an easy path forward
that involves executing a single command.

However, a user who attempts to read the documentation might follow a path that looks like this:

1. Navigates to goteleport.com/docs
2. Locates "Installation" under "Introduction"
3. Clicks "Installation," then "Linux"
4. Sees instructions for installing a Teleport cluster, continues to scroll down to agent
5. Runs an install script that results in an auto-updating agent that is not configured
6. Sees that they need to manually enroll the agent because they did not use the Web UI
7. Clicks "Enroll Resources"
8. Clicks "Linux Servers (section)", then "Introduction to Enrolling Servers"
9. Clicks the "Getting Started Guide", or may get directed there by clicking one of the other guides
10. Sees the install script instructions again, along with other outdated installation instructions
11. Finds and executes manual joining instructions (which are unnecessary if the Web UI is used).

This RFD proposes that we add one new top-level "Install Teleport" section, with subsections that are organized by use case:

1. Install Teleport
   1. Installing Self-Hosted Teleport Clusters
   2. Installing Teleport Agents to Connect Resources
   3. Installing Teleport Clients for Human Access
   4. Installing Teleport Clients for Machine Access
   5. Installing Teleport Plugins & External Integrations
   6. Generic Installation Instructions

These sections would cover all Teleport components.
Installation sections will cover setup for Managed Updates and link to more detailed Upgrading docs where appropriate.

Notably, the "Install Teleport" section differs from the marketing-branded categories for our product features because it is cross-cutting,
operational, and (in general) does not focus on in-product UX workflows.
Users install Teleport, ensure they have a way to upgrade it, ensure they have day-two operations like backups
configured, and then they (or different end-users) dive into the product itself via the category sections.

A complete user story for a Teleport installation workflow may need to involve in-product configuration.
For example, it is common to configure roles after installing a self-hosted cluster.
As we cannot repeat these instructions for every combination of component and target platform, we can rely on cross-linking to ensure
that the user is guided to the correct page regardless of their starting point.

This organizational structure will be especially useful when the installation portion of a workflow may vary significantly on different
cloud platforms, Linux distributions, etc.

## Details

### Overview of All Components with Customer-Managed Operational Lifecycle

Teleport includes a large number of independently installable components that require dedicated installation
instructions for an even larger number of target platforms.

This is an enumeration of all possible installation targets (not a suggested organization, see further below):

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
   7. Agent on Generic Linux
   8. Agent on MacOS via Connect
   9. Agent on MacOS via terminal
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

To address this complexity, the top-level subsections divide users into categories to ensure any incorrect navigation happens early and is obvious to users.
The top-level subsection names will be verbose to avoid incorrect navigation.

### Proposed Organization

The following organization is proposed as an ideal, long-term plan.
These sections would be nested under a top-level "Install Teleport" section.
(See further below for a short-term plan using existing content.)

1. Installing Self-Hosted Teleport Clusters
   1. Installing Teleport Clusters on Kubernetes
      1. Amazon EKS
         1. Installation
         2. Upgrading
         3. (Other operational sections as appropriate, e.g., High Availability, Backups, Troubleshooting, etc.)
      2. etc.
   2. Installing Teleport Clusters on Linux Servers
      1. Amazon EC2
         1. Installation
         2. Upgrading
         3. (Other operational sections as appropriate, e.g., High Availability, Backups, Troubleshooting, etc.)
      2. etc.
2. Installing Teleport Agents to Connect Resources
   1. Amazon EC2 (cross links to "Enroll Resources" list after install, split by DB, App, etc.)
      1. Installation
      2. Configuring services
      3. Upgrading        
   2. etc.
3. Installing Teleport Clients for Human Access
   1. Desktop Application (Teleport Connect)
      1. macOS
      2. etc.
   2. CLI Access
      1. macOS
      2. etc.
4. Installing Teleport Clients for Machine Access
   1. Installing tbot
      1. Install tbot on Linux Servers
         1. Install tbot on Amazon EC2
         2. etc.
      2. Install tbot on Kubernetes
      3. Install tbot in Github Actions
      4. etc.
5. Installing Teleport Plugins & External Integrations
6. Generic Installation Instructions

**However, we do not currently have content to fill most of these sections, and many of our existing pages are reused across platforms.**

To account for this, we can start by re-organizing existing content to match the general organizational principle proposed.

For example, the following organization could be an actionable short-term solution until additional content is added:

1. Provisioning a Teleport Cloud Cluster
   1. (Move Cloud cluster provisioning from Introduction -> Installation)
   2. (Move Migrate Teleport Plans from Introduction -> Installation)
   3. (Move Cloud Cluster Upgrades from Introduction -> Upgrading)
2. Installing Self-Hosted Teleport Clusters
   1. Installing Teleport Clusters on Kubernetes
      1. (Move from ZTA -> Self-Hosting -> Helm Deployments)
      2. (Move from Introduction -> Upgrading -> Manual Upgrades -> Self-hosted Teleport clusters on Kubernetes)
      2. Installing Access Graph | Links to Identity Security -> Self-Host TIS -> Helm
      3. Installing Identity Activity Center | Links to Identity Security -> Self-Host TIS -> IAC
   2. Installing Teleport Clusters on Linux Servers
      1. (Move from VM-specific content of ZTA -> Self-Hosting -> Deployment Guides)
      2. Installing Access Graph | Links to Identity Security -> Self-Host TIS -> Docker
      3. Installing Identity Activity Center | Links to Identity Security -> Self-Host TIS -> IAC
   3. (Move from other sections in ZTA -> Self-Hosting)
3. Installing Teleport Agents to Connect Resources
   1. Installing & Configuring Teleport Agents (Move from Enroll Resources)
   2. Managed Updates for Agents & Bots (v2) (Move from Upgrading)
   3. Manual Upgrades
      1. (Move from Introduction -> Upgrading -> Manual Upgrades -> Teleport Agents running on Kubernetes)
      2. (Move from Introduction -> Upgrading -> Manual Upgrades -> Single Teleport binaries on Linux servers)
4. Installing Teleport Clients for Human Access
   1. Desktop Application (Teleport Connect) | Links to User Guide for Connect
   2. CLI Access | Links to User Guide for tsh
   3. Managed Updates for Clients (v2) (Move from Upgrading)
5. Installing Teleport Clients for Machine Access
   1. Installing & Configuring tbot | Links to M&WI -> Deploy tbot
   2. Managed Updates for Agents & Bots (v2) | Links to Installing Teleport Agents to Connect Resources -> Managed Updates for Agents & Bots (v2)
6. Installing Teleport Plugins & External Integrations
   1. Just-in-Time Access Request Plugins | Links to Identity Governance -> Just-in-Time Access Request Plugins
   2. Event Exporter | Links to ZTA -> Exporting Teleport Audit Events
   3. Teleport Kubernetes Operator | Links to ZTA -> Infrastructure as Code -> Teleport Kubernetes Operator
7. Generic Installation Instructions
   1. Linux
      1. (Move from ZTA -> Cluster management -> Run Teleport as a Daemon)
   2. Docker
   3. Source Code
8. Uninstall Teleport
   1. (Moved from Installation -> Uninstall Teleport)

(Note that this is not a committed action plan and may change as we make progress.)

The Installing and Upgrading sections in Introduction would be removed entirely.

Notably, cluster operations that are not handled by Cloud should be included in the marketing-branded sections (like Zero Trust Access), not in Install Teleport.
For example, a guide to configuring a cluster to support AWS KMS would be included in Install Teleport -> Installing Self-Hosted Teleport Clusters -> KMS, while
a guide for getting started with Teleport roles would be included in the "Zero Trust Access" section.
The only exception to this rule is global, shared configuration that is directly related to platform operations and does not
fit into the marketing-branded sections. For example, some of the `cluster_*` or `autoupdate_*` resources may be described in Install Teleport.
These are generally optional for Cloud users to configure.

Installation instructions should not be complete user guides that include in-product configuration.
Installation instructions should link to user guides when they are more appropriate, and user guides should link to
(or import) installation instructions where needed.

### Confusing Workflow Reference

This list is an incomplete collection of various flows we need to fix. See the Action Plan above for additional examples.

1. If a Cloud user following "Enroll Resource -> Applications" needs the agent for their application to run on Kubernetes with ArgoCD, they must
   follow "Zero Trust Access -> Self-Hosting Teleport -> Guides for running Teleport using Helm", even though the instructions are not related to
   Access or Self-Hosted. Instead, they could follow a link to "Platform -> Installation -> Installing Teleport Agents" and immediately see
   "Platform -> Installation -> Installing Teleport Agents -> Kubernetes -> ArgoCD".
2. "Introduction -> Installation -> macOS" says that `teleport` is supported on macOS, but provides no guidance for configuring or using it.
3. "Introduction -> Installation -> Amazon EC2" links to AMIs and one possible Self-Hosted specific guide, but has no agent install guide.
4. Agent install docs live under "Introduction -> Upgrading -> Managed Updates for Agents (v2)", instead of a clear Installation page.

