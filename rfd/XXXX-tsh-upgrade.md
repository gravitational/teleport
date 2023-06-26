---
authors: Bernard Kim (bernard@goteleport.com)
state: draft
---
# RFD XXXX - tsh upgrade

## Required Approvers

* Engineering: ...
* Infra: ...
* Kube: ...

## What

A tsh subcommand that will prompt the user to upgrade if the version of tsh is older than the cluster version.

## Why

One of our goals for Teleport Cloud is to increase the cadence at which clusters can be upgraded.
This requires that Teleport Agents and Teleport client tools be up to date with the latest compatible version of the cluster.

Manually upgrading Teleport Agents and client tools can be a tedious task, so customers can end up delaying their upgrades.
Simplifying this process will help customers upgrade and stay up to date with their Teleport version.

Development for Automatic Upgrades of Teleport Agents has already been completed, so the success criteria for this RFD only includes developing a simpler way to keep Teleport client tools up to date.

## Details

The main idea here is to add an upgrade command to the Teleport client tools. This will detect the version of the Teleport cluster.
If the version of tsh is older than the version of the cluster, the user will be prompted to upgrade.
The command should then re-download and re-install itself.

### Identifying cluster version

Detecting the version of the Teleport cluster should be straight forward.
Teleport supports a ping RPC that returns a response containing information about the Teleport version of the cluster and the minimum client version required.
In the case that the cluster version of Teleport is newer than the tsh version, the user will be prompted to upgrade.

### Identifying installation method

The Teleport client tools should be re-installed using the original installation method.
Teleport documents several different methods for installing the client tools. https://goteleport.com/docs/installation/
- Teleport can be installed from source and built using go. This scenario can be handled the same as if Teleport was installed by downloading the pre-build binaries.
- Teleport pre-built binaries can be downloaded for the desired platform.
- Teleport can be installed through the system package managers. Teleport currently maintains DEB and RPM repositories.
- Teleport is available for installation using brew on macOS, but it is not officially supported by Teleport and not recommended.
- Teleport provides macOS installation packages. One package installs all Teleport binaries, while the other installs only the tsh client.
- Teleport provides a tsh only zip installation for windows.

Additionally, Teleport supports a couple different releases: Open Source, Enterprise, and a FIPS compliant release.
The upgrade process will need to detect which release was installed and make sure to re-install the same release.

In order to identify which installation method was used to install the Teleport client tools, various checks will be executed.
The default installation method will be downloading the pre-built binaries.

#### Linux

Teleport currently maintains DEB and RPM repositores.
So before upgrading, the upgrade command needs to identify whether the system it's running on is a Debian or Red Hat distribution.
The upgrade command will look at the /etc/lsb-release or /etc/redhat-release files to identify the Linux distribution and to determine which package manager to use.

The installation method can be further narrowed down by using the package manager CLI tools dpkg for Debian, and rpm for Red Hat.
If Teleport was installed using the manager, dpkg or rpm can be used to identify which release was installed: teleport, teleport-ent, or teleport-ent-fips.
Once the release has been identified, the Teleport client tools can now be upgraded using the package manager.

If Teleport was not installed using the package manager, the Teleport client tools will be upgraded by downloading the pre-built binaries.

#### macOS

On macOS, the Teleport package is available for installation using the macOS package installer.
To detect whether or not Teleport was installed using the installer, the pkgutil command can be used to search for the Teleport package.
Only Teleport Open Source and Teleport Enterprise are supported on macOS.

A signed release of tsh is also supported on macOS and is required to use TouchID.
So tsh will need to be upgraded separately after the initial upgrade of the other Teleport client tools.

#### Windows

Only the tsh command is supported on Windows machines.
tsh only supports an Open Source release, so for windows machines, the tsh upgrade command simply needs to download the desired tsh binary and install it.

## UX

The tsh upgrade command should simply upgrade the Teleport client tools while providing some basic observability about the currently installed version of the Teleport client tools, the Teleport cluster version, and what version the client tools will be upgraded to.
The upgrade command will require root privileges.

If the Teleport client tools are up to date, simply output the client and server version and exit
```console
$ sudo tsh upgrade
Teleport v13.1.2
Proxy version 13.1.2
Teleport client tools are already up to date. Nothing to be upgraded.
```

If the Teleport client tools are behind and need to be upgraded, the user will be prompted to upgrade
```console
$ sudo tsh upgrade
Teleport v12.0.0
Proxy version v13.1.2
Teleport client tools version is behind the Teleport cluster version, do you wish to upgrade? [y/N]:
no
Exiting without upgrading Teleport client tools. Teleport servers are compatible with clients that are on the same major version or one major version older. We recommend updating before Teleport client tools become incompatible. If you would like to upgrade manually, installation instructions can be found at https://goteleport.com/docs/installation/

$ sudo tsh upgrade
Teleport v12.0.0
Proxy version v13.1.2
Teleport client tools version is behind the Teleport cluster version, do you wish to upgrade? [y/N]:
yes
...
Teleport client tools have been upgraded to v13.1.2
```

## Security Considerations

Upgrades will mostly rely on the system package managers.
So as long as the package managers and repositories are securely maintained, the upgrade process should also be secure.
If the upgrade is being executed without a package manager, the downloads will be verified using the provided sha256 checksum.
