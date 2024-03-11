---
authors: Bernard Kim (bernard@goteleport.com)
state: draft
---

# RFD 0167 - Automatic Updates Change Proposal


## Required Approvers

* Engineering
* Security:
* Product:


## What
This RFD proposes some major design changes to the automatic updates architecture.

## Why
There were two assumptions that drove the initial design of the automatic updates feature.
1. All Teleport Cloud tenants will be forced on to the same version of Teleport.
2. The Teleport updater will be stable, and it will not require major changes.

Teleport Cloud started rolling out the automatic updates feature starting Teleport 13. Since then, it has become clear that the assumptions that were made do not hold up, and that there are some major limitations that prevent the initial design from supporting the needs of Teleport Cloud.

## Change Overview
Here is the initial [RFD](https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md) for the automatic updates feature. To address the most urgent issues, the automatic updates architecture has already diverged from the initial design. Here is a summary of changes.

### Version Channels
Teleport Cloud now maintains per major version channels. Currently, Teleport Cloud supports v13, v14, and v15 major version channels. The global version channel is still being maintained, but it is now deprecated.

```
# Global version channel (deprecated)
https://updates.releases.teleport.dev/v1/stable/cloud/version -> v14.3.6

# Major version channels
https://updates.releases.teleport.dev/v1/stable/cloud/v13/version -> v13.4.15
https://updates.releases.teleport.dev/v1/stable/cloud/v14/version -> v14.3.6
https://updates.releases.teleport.dev/v1/stable/cloud/v15/version -> v15.1.1
```

### Proxy Version Server
The Teleport proxies now support a version endpoint and serve the latest compatible agent version. The Teleport Cloud proxies are configured to forward version requests to the appropriate upstream major version channel.
```
# Forwards to https://updates.releases.teleport.dev/v1/stable/cloud/v14/version
https://platform.teleport.sh:443/v1/webapi/automaticupgrades/channel/default/version -> v14.3.6
```

### Teleport Updater Version Endpoint
The Teleport updater is now configured to request an agent version from the proxy instead of the global version server.

## Overview
With the above changes, this is now what the automatic updates architecture looks like.

### Version Channels
Teleport Cloud maintains major version channels along with the deprecated global major version channel. Teleport proxies support a version endpoint that serves the latest compatible version of Teleport. The proxies request the latest available version from the appropriate upstream version channel. New Teleport updaters (v14.3.1+), request the target update version from the Teleport proxy. Old Teleport updaters (<v14.3.1) continue to request the target update version from the global version channel.

![version-channels](assets/0167-auto-updates-change-proposal/version-channels.png)

### Publishing
Teleport maintains a stable/cloud channel for APT and RPM packages. After updating Teleport Cloud tenants to vX.X.X, Teleport Cloud deploys a GitHub workflow to publish the necessary packages. The workflow publishes vX.X.X teleport-ent and teleport-ent-updater packages to the stable/cloud repositories, and then updates the version channels.

Teleport also maintains a Helm repository. The teleport/teleport-kube-agent Helm chart is published regularly with the OSS release of Teleport.

![publishing](assets/0167-auto-updates-change-proposal/publishing.png)

### Systemd Deployments
The documentation for deploying Teleport on a Linux server instruct users to install two packages: teleport-ent and teleport-ent-updater. The teleport-ent-updater package installs the teleport-upgrade tool and starts a teleport-upgrade.timer/service to periodically check for available updates. The teleport-upgrade tool invokes the package manager to handle teleport-ent package updates.

![publishing](assets/0167-auto-updates-change-proposal/systemd-deployment.png)

### Kubernetes Deployments
Kubernetes deployments of Teleport are installed using Helm. The teleport/teleport-kube-agent Helm release deploys the teleport-agent resource and a separate teleport-agent-updater resource at the same version. The teleport-agent-updater periodically checks for available updates and updates the teleport-agent resource image version.

![publishing](assets/0167-auto-updates-change-proposal/kube-deployment.png)

## Current Limitations

### Version Channels*
The initial design of auto updates relied on a single version channel. All Teleport updaters request the target update version from this endpoint. This limitation has been mostly addressed. Teleport Cloud now supports per tenant version servers.

Issue: However, there are still a number of Teleport agents using the deprecated version of the Teleport updater. Until all Teleport updaters are updated, Teleport Cloud needs to continue to maintain the global version channel. The global version has to stay at the minimum version of all tenants using the deprecated version of the Teleport updater.

### Packages
The initial design assumed that Teleport Cloud will only publish vX.X.X to the stable/cloud channel package repository when version vX.X.X is compatible with the control plane of all Teleport Cloud users. So it was communicated that it should always be safe to pull the latest version of teleport-ent from the stable/cloud package repository.

Issue: The stable/cloud package repository cannot support the needs of Teleport Cloud. It is expected that users should be able to update/install the latest available version of the teleport-ent package and maintain version compatibility with the Teleport control plane. Since Teleport Cloud supports tenants on multiple major versions, it is not possible for the stable/cloud channel to meet this requirement for all tenants.

### Helm Charts
The teleport/teleport-kube-agent Helm release manages the deployment of both the teleport-agent and the teleport-agent-updater.

Issue: The teleport-agent-updater updates the image of the teleport-agent resource. This results in the version of the teleport-agent diverging from the version specified in the Helm chart. If the Helm chart is redeployed, the teleport-agent will revert back to the original specified value.

There are a number of users unable to enroll in automatic updates because it is incompatible with their ArgoCD deployments. ArgoCD generates the template from the Helm chart and manages the resources itself. It monitors the teleport-agent resource and when it detects that the resource has diverged from the initial spec, it will reconcile the resource.

### Test Coverage
Because it was assumed that the Teleport updater would be stable, the Teleport updater logic is written in bash and it lacks sufficient testing. There is some automated testing. There are unit tests in place to verify the functionality of the Teleport updater, and there are tests to verify that the Teleport updater can be installed from the stable/cloud repository.

Issue: For such a critical piece of the Teleport architecture, it seems like an insufficient amount of test coverage. Developers are uncomfortable making changes to the Teleport updater.

## Change proposals
These proposals contain minimal implementation details. If the proposals are approved, an execution plan will be written up for each item with more implementation details.

### Deprecate the stable/cloud teleport-ent package
Currently, the teleport-ent-updater package requires the teleport-ent package as a dependency. This means that the user must install the latest version of the teleport-ent package which may or may not be compatible with their Teleport control plane, or they must first specify a compatible version of teleport-ent to install. This puts unnecessary burden on the user, and complicates the installation process.

Step 1: To remove this burden from the user and simplify the installation process, the Teleport updater will support an install command. The install command accepts the necessary configuration and then installs the latest compatible version of the teleport-ent package for the user.

An example of what this new installation process might look like:
```sh
$ apt-get install teleport-ent-updater
$ teleport-upgrade install --proxy=example.teleport.sh
```

This step does not ensure major version compatibility. If a user manually updates the teleport-ent package to the latest available version, they may still get an incompatible major version of Teleport.

Step 2: The Teleport package repository supports per major version channels (stable/vXX). Users were instructed to use these channels prior to the stable/cloud channel. In order to ensure the teleport-ent package does not get updated to an incompatible major version. The Teleport updater will now maintain the channel of the Teleport package repository. Whenever the Teleport cluster is updated to a new major version, the Teleport updater will also update the Teleport package repository channel. This will allow the stable/cloud teleport-ent package to be deprecated.

This step does not ensure minor version compatibility. If a user manually updates the teleport-ent package to the latest available version, they may still get an incompatible minor version of Teleport.

Step 2 (Alternative): Instead of relying on the stable/vXX channels. There is also the option of preventing updates of the teleport-ent package except by the Teleport updater. This would not require the stable/cloud channel to be deprecated. However, this would require a different solution of each support package manager, would could get messy.

Apt supports an apt-mark hold command that can be used to hold back a package from being updated. The Teleport updater can be modified to hold the teleport-ent package after an update, and un-hold when it is performing an update.

Yum has a similar feature that can exclude packages from a system update. This can be done by specifying teleport-ent to be excluded in the /etc/yum.conf file.

Step 3: The Teleport installation process should no longer rely on the package manager to download teleport-ent packages. Instead, the Teleport proxy will now serve the latest compatible version of the teleport-ent package. The Teleport updater will then be responsible for downloading and installing the teleport-ent packages from the Teleport proxy.

This step will ensure version compatibility for the teleport-ent packages downloaded from the proxy. This step also removes version compatibility concerns from the Teleport updater, and the version servers can be deprecated.

### Reduce to a single installation path
Teleport supports different installation scripts for a number of different methods of installation for Teleport. This creates an increased maintenance and testing burden on developers. This has already lead to several incidents, and if these change proposals are accepted there is concern for more issues to emerge.

Step 1: The different installation scripts should be reduced to a single script used regardless of the installation method.

Step 2: After reducing cardinality, it should be more manageable to implement more extensive testing for the single installation script. There should be automated testing in place to verify installation and updates. Teleport supports the 3 latest major versions. So tests should be run against all the supported major versions.

### Give ownership of the teleport-agent to the teleport-agent-updater
The teleport/teleport-kube-agent Helm chart with the updater enabled is not currently compatible with ArgoCD.

Step 1: As a short term workaround, ArgoCD supports an [ignoreDifferences](https://argo-cd.readthedocs.io/en/stable/user-guide/diffing/#application-level-configuration) feature. This feature can be used to ignore differences for a specific resource and field. This step would just require some additional documentation and communication with users.

Step 2: As a longer term solution, ownership of the teleport-agent should be given to the teleport-agent-updater. The teleport-agent-updater should own the teleport-agent resource and be responsible for creating and updating the resource.

## Communications
There have been many user upset and confused about the changes that have been made to the Teleport automatic updates feature. Along with documentation changes, there should be more official communication to inform users of the changes being made.

## Out of Scope

### Move Update Logic
A key requirement for the initial design of the Teleport updater was that the updater must not rely on Teleport itself, because it needs to be able to recover if a broken version of Teleport is shipped. If this broken version of Teleport is unable to execute any commands, it could leave the agent and updater in a permanently broken state.

If these change proposals are accepted, the changes will require multiple steps to implement, and users will need to update their teleport-ent-updater multiple times. To avoid this situation, it might be worth considering some options to modify the update logic without having to update the teleport-ent-updater package.

- Move update logic into Teleport. Teleport could provide a teleport update command that either updates the package, or it generates an update script for the Teleport updater to then execute. The update script could also be a static script that lives in the teleport-ent package.

### Opt Out of Auto Updates
There are some users who deploy and maintain Teleport with methods that are not currently supported.
- Teleport builds from source
- Teleport images sourced from a private ECR
- DIY automatic updates

These methods of deploying and maintaining Teleport are currently incompatible with the auto updates feature. However, these users are most likely in the minority. Ensuring these users are up to date with their Teleport client software can be handled on a case-by-case basis for now.

### Relax Teleport Version Requirements
Something to consider is relaxing the Teleport client/server version requirements. Teleport supports the 3 latest major versions, but it only supports compatibility with clients up to one major version behind and does not support clients that are on a newer version.

