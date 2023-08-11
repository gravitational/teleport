---
authors: Bernard Kim (bernard@goteleport.com)
state: draft
---

# RFD 0144 - Client tools auto update

## Required Approvers

* Engineering: @r0mant && @jimbishopp
* Product: @klizhentas || @xinding33
* Security: @jentfoo

## What

A workflow for Teleport client tools (both tsh and tctl) to automatically stay up to date with the latest cloud-stable version of Teleport.

## Why

One of our goals for Teleport Cloud is to increase the cadence at which clusters can be updated.
This requires that Teleport Agents and Teleport client tools be up to date with the latest compatible version of the cluster.

Manually updating Teleport Agents and client tools can be a tedious task, so customers can end up delaying their updates.
Simplifying this process will help customers update and stay up to date with their Teleport version.

Development for Automatic Updates of Teleport Agents has already been completed, so the success criteria for this RFD only includes developing a simpler way to keep Teleport client tools up to date.

## Details

Teleport will now store a single cached version of the client tools in the user's $HOME directory. tsh/tctl will execute this cached version when communicating with the Teleport Cloud cluster.

Cloud will host an endpoint for Teleport client tools version discovery. tsh/tctl will check this endpoint to identify the current cloud-stable version of Teleport. If the cached version is behind the current cloud-stable version, the cache will be updated to match the current cloud-stable version. This will ensure Teleport client tools are always executed using the latest compatible version of Teleport when communicating with a Teleport Cloud cluster.

For now this feature will be disabled for self-hosted users and will only be supported for Cloud users.

### Version discovery

Cloud will host the cloud-stable version of Teleport at https://updates.releases.teleport.dev/v1/stable/cloud/client-version. This endpoint will serve a Teleport version e.g. `v13.2.3`.

The client tools are not using the same endpoint that agents use to discover the cloud-stable version of Teleport because having a separate channel allows us to rollback a botched client release without having to rollback the cluster or agents at the same time.

The stable version should be updated after each Cloud Tenant upgrade. This should follow the same workflow as updating the cloud-stable agent version endpoint. The version should be updated soon after Cloud Tenant upgrades have been completed.

Unlike the agent version discovery model, there will not be an endpoint to identify if a critical update is available. The reason for this is because updates are not executed in a scheduled update window. Teleport client tools will check this endpoint on every login and update accordingly if a new version is available.

On every login, tsh will pull the current cloud-stable version of Teleport and store the value in the .tsh directory under `$HOME/.tsh/cloud-stable-version`.

### Caching

A single version of the Teleport client tools will be stored in the user's $HOME directory. Teleport already makes use of a .tsh directory for storing tsh config. Cached versions of tsh/tctl will also live within this directory under ` $HOME/.tsh/cache/<version>/{tsh,tctl}`. Whenever a new version of the client tools are available, the existing cached version will be replaced.

On every login, tsh will compare the currently cached version of the client tools with the current stable version stored in `$HOME/.tsh/cloud-stable-version`. If the cached version is behind the current stable version, a new version of the client tools will be downloaded to replace the existing ones. When downloading the Teleport client tools, the binaries will be downloaded directly. We need to make sure to download a version that is compatible with the system.

Because only a single version of the client tools will be cached at a time, we do not need to worry about cleaning up the cache.

### Package upgrade

As an alternative to caching, we've considered directly upgrading the Teleport packages.

This approach will be difficult to implement because we support multiple package repositories and installation methods. The amount of maintenance and testing required to support this solution is probably not worth the effort at this time.

Another downside to this approach would be that upgrades would require sudo/admin privileges.

### Config

By default, the client tools will have auto-updates enabled. I don't see compelling reason to allow users to disable auto updates for client tools. But if we want to allow this feature to be disabled, we can add support in the tsh configuration files by adding a DISABLE_AUTO_UPDATES flag.

## UX

Ideally, we'd like this feature to be integrated seamlessly. Users of the Teleport client tools should not need to take any actions to enable auto updates for their client tools.

We should be transparent about the client tools updates. Whenever a new cloud-stable version is available for download, the progress of the update will be output to stdout. e.g. `New cloud-stable version of Teleport detected`, `Downloading latest version of tsh/tctl... `, `Updated tsh/tctl to version v13.2.3!`.

## Documentation

There should be minimal documentation changes required for this feature. Users will not need to opt-in or take any actions to take advantage of this feature.

The documentation should include a basic overview of how auto updates works for client tools. Documenation should also be available describing how to opt-out of auto updates.

## Security Considerations

All downloads will be verified using the provided sha256 checksums. As long as our CDN is secure, the update process should be secure.
