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

First off, Teleport client tools will now support an update command to update tsh/tctl to the latest cloud-stable version of Teleport. This will functionally be the same as updating the Teleport package via the system package manager.

Users should be continually be encouraged to keep their Teleport client tools up to date with the latest version of Teleport. To do this, Teleport will now periodically prompt users to update Teleport client tools if a new version of Teleport is available.

Cloud will host an endpoint for Teleport client tools version discovery. tsh/tctl will check this endpoint to identify the current cloud-stable version of Teleport. On update tsh/tctl will compare its version to the current cloud-stable version. If the installed version is behind or ahead of the current cloud-stable version, tsh/tctl will be updated to match the current cloud-stable version. This will ensure Teleport client tools are always executed using the latest compatible version of Teleport when communicating with a Teleport Cloud cluster.

### Detecting Cloud instances

For now this feature will be disabled for self-hosted users and will only be supported for Cloud users.

For tsh/tctl to identify if it is communicating with a Cloud instance of Teleport the `../webapi/ping` endpoint will now include a `cloud` boolean flag in it's ping response.

### Version discovery

Cloud will host the cloud-stable version of Teleport at https://updates.releases.teleport.dev/v1/stable/cloud/client-version. This endpoint will serve a Teleport version e.g. `v13.2.3`.

The client tools are not using the same endpoint that agents use to discover the cloud-stable version of Teleport because having a separate channel allows us to rollback a botched client release without having to rollback the cluster or agents at the same time.

The stable version should be updated after each Cloud Tenant upgrade. This should follow the same workflow as updating the cloud-stable agent version endpoint. The version should be updated soon after Cloud Tenant upgrades have been completed.

The [deploy-auto-update-changes.yml](https://github.com/gravitational/cloud/blob/master/.github/workflows/deploy-auto-update-changes.yml) workflow will need to be updated to include an additional job that updates the cloud-stable version of client tools.

Unlike the agent version discovery model, there will not be an endpoint to identify if a critical update is available. The reason for this is because updates are not executed in a scheduled update window. Teleport client tools will check this endpoint everytime `login` or `status` is executed, and update accordingly.

We've considered querying the Teleport version information from the ping response, but we've decided against this approach. One reason is because we would not be able to update the client tools version independently of the cluster. Another reason is because we're considering removing the Teleport minor version information from the ping response. The minor version information could be used by attackers to identify and attack unpatched customers.

### Package update

The Teleport client tools will be updated using the package manager available on the system. Teleport supports multiple package repositories and installation methods, so each system will need to be handled differently. This will probably require a decent amount of testing and maintenance work, but this is looking like the simplest and most straight forward solution.

Support for Teleport client tools update will roll out in stages and ther will most likely be many edge cases that we'll need to handle along the way.

Teleport client tools are supported on Linux, Mac, and Windows. Before performing an update, the OS/distro will need to be identified. Teleport will first look at runtime.GOOS to determine the OS.

#### Windows

Teleport client tools installation via the package manager is not currently supported on Windows systems. Installation will require manual installation of the binary from the Teleport CDN.

#### MacOS

Before performing an update on MacOS systems, Teleport needs to identify whether to install Teleport Enterprise vs Teleport OSS. The MacOS installer command doesn't provide a helpful command to distinguish between Teleport Enterprise and Teleport OSS package installation, so Teleport will instead attempt to identify this information by parsing the teleport version command.

Once the Teleport edition has been identified, the proper Teleport package will be downloaded from the CDN and installed using the installer command.

In order for software on MacOS to use TouchID, the software needs to be signed and notorized by Apple. So an additional step is required on MacOS. Teleport provides a separate tsh only package that is compatible with TouchID. This package needs to be installed after the initial teleport package installation.

#### Linux

The current list of supported Linux distributions include RHEL/CentOS 7+, Amazon Linux 2+, Amazon Linux 2023+, Ubuntu 16.04+, Debian 9+, SLES 12 SP 5+, and SLES 15 SP 5+.

For Linux systems Teleport will need to look at the /etc/os-release file to determine the distribution.

The supported Linux distributions then need to be separated by the package manager that is used. Teleport supports an APT, YUM, and Zypper repository. Here are the following distribution to package manager mappings:

- Debian 9+/Ubuntu 16.04+ -> apt
- Amazon Linux 2/RHEL 7/CentOS 7 -> yum
- Amazon Linux 2023/RHEL 8+ -> dnf
- SLES 12 SP5+/SLES 15 SP5+ -> zypper

#### Enterprise vs Open Source

Teleport supports multiple channels including stable/cloud and stable/rolling. The stable/cloud channel receives releases compatible with Cloud, and so it only supports Teleport Enterprise packages. The stable/rolling channel receives all published Teleport releases including Teleport OSS packages.

Before performing an update, Teleport needs to verify whether Teleport Enterprise or Teleport OSS has been installed, and if the package repository has been properly configured with the correct release channel. This step will be different for each system. Here are the following distribution to command mappings:

- Debian 9+/Ubuntu 16.04+ -> dpkg --status < package-name >
- Amazon Linux 2/RHEL 7/CentOS 7 -> yum info --installed < package-name >
- Amazon Linux 2023/RHEL 8+ -> dnf info --installed < package-name >
- SLES 12 SP5+/SLES 15 SP5+ -> zypper info < package-name >

#### Root/Admin privileges

Note that these methods of updating Teleport client tools will require sudo/admin privileges.

### Automatic update

Once Teleport client tools support the update command, the next step will be to add support for automatic updates. On login, Teleport will check for available updates by pulling the Teleport version from the client cloud stable version endpoint. If the currently installed version of Teleport client tools does not match the available client cloud stable version, the user will be prompted to update the Teleport client tools.

#### Config

Automatic updates behavior can be configured through tsh configuration. Global configuration for tsh is set in the /etc/tsh.yaml file while user specific configuration is set in the ~/.tsh/config/config.yaml file.

To disable automatic updates, the user can set the environment variable `DISABLE_AUTO_UPDATE=true`. The user will no longer be prompted to update their Teleport client tools. The user will need to run the update command themselves when they are ready to update the Teleport client tools. Automatic updates will be enabled by default.

The automatic update prompt can be disabled. If the environment variable `DISABLE_UPDATE_PROMPT=true` is set, the Teleport client tools will update without prompting for confirmation from the user. This will be disabled by default.

The following configuration would configure Teleport to automatically update Teleport client tools without prompting the user to confirm the update.
```
#/etc/tsh.yaml
DISABLE_AUTO_UPDATE=false
DISABLE_UPDATE_PROMPT=true
```

## UX

The Teleport client tools update command will support a few configuration flags:
- --yes confirms the update without an interactive prompt
- --channel specifies a custom release channel
- --version specifies a custom teleport version

#### Root/Admin privileges

If the user does not have root/admin privileges, the update will fail with the following message:
```
Teleport client tools update requires root/sudo privileges
```

#### Teleport client tools update available is not required

If the Teleport client tools are already on the latest compatible version, the update will be canceled with the following message:
```
Teleport client tools is running v13.4.5; No update required
```

#### Teleport client tools not installed via package manager

If the Teleport client tools were not installed via the package manager, the update will fail with the following message:
```
Unable to verify teleport package status; Please ensure teleport is installed via the package manager
```

#### Teleport client tools update not supported

Support for Teleport client tools will be implemented separately for each system. If the update command is not yet supported on a system, the update will fail with the following message:
```
Update is not yet supported on Windows
```

#### Teleport client tools update available

If an update is available, the user will be prompted to confirm the update:
```
Updating Teleport client tools from v13.4.3 to v13.4.5
Do you want to continue? [Y/n]
```

The output from the execution of the package manager installation commands will be piped to stdout and stderr:
```
...
Get:1 https://apt.releases.teleport.dev/ubuntu jammy InRelease [158 kB]
Fetched 158 kB in 4s (43.0 kB/s)
Reading package lists... Done
Reading package lists... Done
Building dependency tree... Done
Reading state information... Done
teleport-ent is already the newest version (13.4.3).
0 upgraded, 0 newly installed, 0 to remove and 16 not upgraded.
```

#### Automatic updates

If automatic updates is enabled, the user will be prompted to update Teleport client tools on login.
```
$ tsh login --proxy=example.teleport.sh
...
A new version of Teleport clients tools is available (v13.4.5). Would you like to update? [Y/n]
```

To avoid breaking existing scripts that use Teleport client tools, Teleport will only attempt automatic updates when a tty is detected.

## Documentation

There should be minimal documentation changes required for this feature. Users will not need to opt-in or take any actions to take advantage of this feature. The tsh and tctl documentation will be updated to include the update commmand.

The documentation should include a basic overview of how auto updates works for client tools. Documention will also include details describing how to use the configuration flags and configuration environment variables.

## Security Considerations

### Downloads

Teleport supports manual installation of Teleport binaries from our CDN. Supporting automatic updates with this installation method will require validating the tarball downloads.

Teleport provides sha256 checksums at `https://get.gravitational.com/teleport-ent-<version>-<os>-<arch>-bin.tar.gz.sha256`, which can be used to verify the tarball downloads. We could also consider sending sha256 checksums in the ping response.

For added security, one option to consider is to sign the binaries and have tsh/tctl validate the signatures before finishing an update. This would allow us to serve binaries, hashes and signatures from the same CDN. Because even if the CDN is compromised, the attacker would not be able to craft valid signatures.

### Tampering

Once the binary has been downloaded, it could be susceptible to bit rot or tampering.

To protect against this, we could keep a hash on disk to verify integrity on every invocation. Someone with enough rights to tamper with the client tools binary will also have the rights to tamper with the hash, so we'd also need to store the signature as well.

## Tasks

- [ ] Create the required client version discovery endpoint and implement the necessary workflows required to update and maintain the endpoint
- [ ] Add update support for each system will be implemented separately
- [ ] Support for automatic updates will be implemented after support for the update command has been implemented
- [ ] Write documenation with details describing how to use the update command and automatic updates

## Inspiration

### Terraform

Terraform uses a separate version manager tool called `tfenv` to install and switch between different versions of `terraform`. How it works is first you install a version using `tfenv install 1.3.9`. This will download the tarball and install terraform in `$HOME/.tfenv/versions/1.3.9/terraform`. Then you can select which version to use with `tfenv use 1.3.9`. The `1.3.9` binary is then symlinked to `$HOME/.tfenv/bin/terraform`.

### Tailscale

Tailscale supports the `tailscale update` command on Windows and some Linux distros. The imlpementation looks like it checks the OS/distro and then uses the system specific package manager to update the tool. On macOS they support automatic updates using https://github.com/mas-cli/mas. This feature requires `tailscale` to be install via the app store.

## Alternatives

### Caching

We've explored the idea of caching the client tools in the user's $HOME directory.

The benefits of this approach are:
- It would not require sudo/admin privileges to install a new version of the client tools.
- We would only need to handle a single method of installing the binary via direct tarball download.

The downsides of this approach are:
- Since we have multiple installation methods, it may be confusing to the users if there is a discrepancy between the version of client tools installed with the package manager and the cached version. This could potentially be a security concern if users are not able to correctly identify what versions of the client tools are installed on the system.
- Installation without a package manager means that we now have the extra responsibility of validating downloads.
