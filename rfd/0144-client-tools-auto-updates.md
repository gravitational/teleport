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

Support for Teleport client tools (both tsh and tctl) to automatically stay up to date with the latest cloud-stable version of Teleport.

## Why

One of our goals for Teleport Cloud is to increase the cadence at which clusters can be updated.
This requires that Teleport Agents and Teleport client tools be up to date with the latest compatible version of the cluster.

Manually updating Teleport Agents and client tools can be a tedious task, so customers can end up delaying their updates.
Simplifying this process will help customers update and stay up to date with their Teleport version.

Development for Automatic Updates of Teleport Agents has already been completed, so the success criteria for this RFD only includes developing a simpler way to keep Teleport client tools up to date.

## Details

Teleport will now store a single cached version of the client tools in the user's $HOME directory. tsh/tctl will execute this cached version when communicating with the Teleport Cloud cluster.

Cloud will host an endpoint for Teleport client tools version discovery. tsh/tctl will check this endpoint to identify the current cloud-stable version of Teleport. If the cached version is behind or ahead of the current cloud-stable version, the cache will be updated to match the current cloud-stable version. This will ensure Teleport client tools are always executed using the latest compatible version of Teleport when communicating with a Teleport Cloud cluster.

### Detecting Cloud instances

For now this feature will be disabled for self-hosted users and will only be supported for Cloud users.

For tsh/tctl to identify if it is communicating with a Cloud instance of Teleport the `../webapi/ping` endpoint will now include a `cloud` boolean flag in it's ping response.

### Version discovery

Cloud will host the cloud-stable version of Teleport at https://updates.releases.teleport.dev/v1/stable/cloud/client-version. This endpoint will serve a Teleport version e.g. `v13.2.3`.

The client tools are not using the same endpoint that agents use to discover the cloud-stable version of Teleport because having a separate channel allows us to rollback a botched client release without having to rollback the cluster or agents at the same time.

The stable version should be updated after each Cloud Tenant upgrade. This should follow the same workflow as updating the cloud-stable agent version endpoint. The version should be updated soon after Cloud Tenant upgrades have been completed.

The [deploy-auto-update-changes.yml](https://github.com/gravitational/cloud/blob/master/.github/workflows/deploy-auto-update-changes.yml) workflow will need to be updated to include an additional job that updates the cloud-stable version of client tools.

Unlike the agent version discovery model, there will not be an endpoint to identify if a critical update is available. The reason for this is because updates are not executed in a scheduled update window. Teleport client tools will check this endpoint everytime `login` or `status` is executed, and update accordingly if the cached version does not match the cloud-stable version.

We've considered querying the Teleport version information from the ping response, but we've decided against this approach. One reason is because we would not be able to update the client tools versions independently of the cluster. Another reason is because we're considering removing the Teleport minor version information from the ping response. The minor version informatoin could be used by attackers to identify and attack unpatched customers.

### Caching

A single version of the Teleport client tools will be stored in the user's $HOME directory. Teleport already makes use of a .tsh directory for storing tsh config. Cached versions of tsh/tctl will also live within this directory under ` $HOME/.tsh/bin/{tsh,tctl}`. Whenever a new version of the client tools are available, the existing cached version will be replaced.

The .tsh directory is only accessilbe to the user and we should make sure the new `bin` directory also only has user permissions.

Everytime `login` or `status` command is executed, tsh will compare the currently cached version of the client tools with the current cloud-stable version of Teleport. If the cached version is behind or ahead of the current stable version, a new version of the client tools will be downloaded to replace the existing ones. When downloading the Teleport client tools, the binaries will be downloaded directly. We need to make sure to download a version that is compatible with the system.

Because only a single version of the client tools will be cached at a time, we do not need to worry about cleaning up the cache.

### Package upgrade

As an alternative to caching, we've considered directly upgrading the Teleport packages.

This approach will be difficult to implement because we support multiple package repositories and installation methods. The amount of maintenance and testing required to support this solution is probably not worth the effort at this time.

Another downside to this approach would be that upgrades would require sudo/admin privileges.

### Config

Automatic updates for client tools can be configured through tsh configuration. If `DISABLE_AUTO_UPDATE=true`, then auto updates for client tools will be disabled. If `DISABLE_UPDATE_PROMPT=true`, then the user will not be prompted to confirm the update. Both values will default to false.

### Binary Execution

The client tools will need a way to execute the cached binaries. When tsh/tctl is executed, it will now need to decide whether to execute the command as usual, or fork a new process using the cached binary.

One way to handle execution of cached binaries is to use the os.Executable function. os.Executable give us the path of the binary executable. The flow might look something like:
- If executing `tsh login` execute as usual (cached tsh binary will not support TouchID on macOS)
- If executable is in cache directory (.tsh/bin/tsh), then execute command as usual
- Else try to execute tsh from cache if exists

An alternative method could be use to a flag like `--no-cache` to determine if a cached version should be executed. The flow might look something like:
- If `--no-cache=true`, execute as usual
- If `--no-cache=false`, try to execute tsh from cache with `--no-cache=true`
- If tsh does not exist in cache, execute as usual

Another option could be to have users update their `PATH` env with `export PATH=$HOME/.tsh/bin:$PATH`. Adding the tsh bin directory to the `PATH` environment in the user's `.profile` can be handled by tsh. Users would then execute the cached binary directly. This seems like a simpler approach than the above methods. The problem is that the cached binary would not support TouchID for macOS users.

I believe macOS software needs to be notarized as a requirement for TouchID. For this reason Teleport provides a separate signed tsh download at `https://cdn.teleport.dev/tsh-13.3.4.pkg`. We could change the download method for macOS users, but this might not be very straight forward to implement.

## UX

Ideally, we'd like this feature to be integrated seamlessly. Users of the Teleport client tools should not need to search for documentation and spend time figuring out how to enable auto updates for their cleint tools.

After client tools auto updates is deployed and the first time the client tools are executed, the user will be asked two questions: 1) if they would like to update to the latest version (default yes) and 2) if they would like to enroll in auto updates for client tools (default yes).

Users that would like to opt-in or opt-out of auto updates at a later time can edit their tsh configuration file and edit the `DISABLE_AUTO_UPDATE` value.

If a user opts out of auto updates, the user will still be prompted with the first question every time an update is available, but not the second question.

We should provide some observability into the download status. Whenever a new cloud-stable version is available for download, the progress of the update will be output to stdout. e.g. `New cloud-stable version of Teleport detected`, `Downloading latest version of tsh/tctl... `, `Updated tsh/tctl to version v13.2.3!`.

To avoid breaking existing scripts, a `--[no-]auto-update` flag will be included. Without the flag given, when a tty is detected and an update is available, the user will be prompted to update. If the flag is not provided and a tty is not detected, updates will be skipped to avoid breaking scripts.

## Documentation

There should be minimal documentation changes required for this feature. Users will not need to opt-in or take any actions to take advantage of this feature.

The documentation should include a basic overview of how auto updates works for client tools. Documention should also be available describing how to use the configuration values `DISABLE_AUTO_UPDATE`, `DISABLE_UPDATE_PROMPT`, and the `--skip-auto-update` flag.

## Security Considerations

### Downloads

Teleport provides sha256 checksums at `https://get.gravitational.com/teleport-ent-<version>-<os>-<arch>-bin.tar.gz.sha256`, which can be used to verify the tarball downloads. We could also consider sending sha256 checksums in the ping response.

For added security, one option to consider is to sign the binaries and have tsh/tctl validate the signatures before finishing an update. This would allow us to serve binaries, hashes and signatures from the same CDN. Because even if the CDN is compromised, the attacker would not be able to craft valid signatures.

### Tampering

Once the binary has been downloaded, it could be susceptible to bit rot or tampering.

To protect against this, we could keep a hash on disk to verify integrity on every invocation. Someone with enough rights to tamper with the client tools binary will also have the rights to tamper with the hash, so we'd also need to store the signature as well.

We do not believe that moving the client tools executable under the users control significantly increases any local attacks, so we may keep this out of scope for this RFD.