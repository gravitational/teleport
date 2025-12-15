---
authors: Andrew LeFevre (andrew.lefevere@goteleport.com)
state: draft
---

# RFD 0208 - Official SELinux Module for Teleport SSH Agents

## Required Approvers

- Engineering: (@r0mant && @rosstimothy)
- Security: Doyensec

## What

This RFD proposes an official SELinux module for Teleport SSH services. The module will enforce that minimal permissions are given to SSH services,
and that security boundaries between the main server process and child session processes are respected.

The module will be bundled in Teleport agent binaries and `teleport-update` will be modified to allow installation and configuration of the module.

## Why

Teleport is often deployed in environments with strict security requirements. SELinux provides a way to enforce fine-grained access controls, which
can help prevent privilege escalation and lateral movement, provide peace of mind, and comply with security standards. As agents are the most widely
deployed component of Teleport, it is important to ensure that they are secure and compliant.

## Scope

Only Teleport SSH services are initially in scope for this proposal as a starting point, but the SELinux module may be extended to other components of Teleport in the future.
The module will initially allow the following features and functionality of Teleport SSH services:

- SSH connections
- SFTP connections
- Enhanced session recording
- Auditd logging
- PAM support

`teleport-update` will be updated to allow users to update or install Teleport's SELinux module. A flag will be added to `teleport start` that will check that the Teleport SELinux module is installed and configured correctly, and exit with an error otherwise.

## Details

### SELinux Module Structure

SSH services need a lot of privileges to function properly. They need to bind to privileged ports, manage cgroups, create processes, and more.
When an SSH session is started, the SSH service will create 2 child processes to handle it: 

- An intermediary process that prepares another child process that will actually run the SSH session. Depending on what is configured, PAM contexts
may be created, users may be created, a shell will be created, the user and group will be changed if necessary, and so on.
- The intermediary process will then create the child session process, which only needs standard permissions of the host user that the SSH session
was created for.

Additionally, the SSH service may create a networking child process to handle X11 forwarding, port forwarding, and SSH Agent forwarding. This process
is a child process of the intermediary process as well.

SELinux groups processes in what are called domains. Child processes will inherit the parent process's domain by default
unless a rule specifically allows a domain transition. The domain of a process is controlled chiefly by the SELinux type
on the file the process was created from on disk. One binary file can only have one domain attached to it.

It would make sense to create multiple domains for each process Teleport SSH services have, and grant specific permissions
to each domain. But this would require multiple copies or symlinks of the Teleport SSH binary on disk, each with a different
SELinux type set. Teleport would also need to be modified to create processes with the correct binary. Due to the installation
and management difficulties this would create, the SELinux module will create one domain and grant it all permissions that
the Teleport SSH service and its child processes require.

This is largely the approach OpenSSH takes as well in its official SELinux module. `sshd` has one main domain that encompasses
the main server processes as well as processes that handle SSH sessions.

Configurable file and port contexts will be created to bind to the necessary ports and read and write to the necessary files, depending on the SSH service configuration. At a minimum this will allow a Teleport SSH service to bind to
the configured SSH port and read and write to the data directory, and dial the configured Teleport Proxy and Auth servers. 

There are a few optional features that can be enabled or disabled on SSH services, the first version of the module will ensure necessary permissions
for all optional features in scope are allowed to simplify development and deployment.

These include:

- Reading `~/.tsh/environment` on the server before creating a session
    - The `teleport_ssh_service_intermediary` domain will be allowed to read `.tsh/environment` in all users' home directories
- Enhanced session recording with BPF
   - The `teleport_ssh_service` domain will be allowed to create and manage BPF ring buffers, and load BPF programs
- Creation of PAM contexts
    - The `teleport_ssh_service_intermediary` domain will be allowed to `dlopen` the PAM module and call its exported functions
    - The location of the PAM module will be configurable
- SFTP support
    - The `teleport_ssh_service_intermediary` domain will be allowed to re-execute itself with `sftp` as the first argument

The SELinux module will be created as a SELinux Reference Module (refmodule), to maximize readability and maintainability.

The main alternative is to use SELinux Common Intermediate Language (CIL) which bridges refmodules and compiled binary module files.
CIL is extremely powerful but much more difficult to understand by humans, it's meant for consumption by SELinux tools.

#### Future updates

The SELinux module will be updated in the future to support new features and maintain compatibility with existing features.
Any changes to the SSH service that result in new files being managed, new network activity (listening on new ports, dialing new addresses etc),
new binaries being executed, or new Linux syscalls being called will require updates to the SELinux module.

### Installation and updating

The module source will be embedded in future Teleport agent binaries. This will ensure that the module is always compatible with the installed version of Teleport.

A shell script will be added that will allow users to install and update the SELinux module. It will extract the embedded module source from
the installed version of Teleport and build and install the SELinux module. This script will be included in release tarballs, but not DEB or RPM packages.

`teleport-update` will be modified to support managing SELinux modules for Teleport SSH. `teleport-update` will install the SELinux
module if configured to, and will remove it if it was previously installed and the user specified that they don't want the module
installed now. If SELinux is not present on the host, the `teleport-update` will tell the user this and exit. If SELinux is present but 
the Teleport agent module is not installed, the `teleport-update` will extract the embedded module and use `checkmodule`, `semodule_package`,
and `semodule` to compile and install the module. Directories and files that Teleport SSH requires will be created if they do not exist
after the module is installed, so that they can be labeled correctly. If necessary files do not have the correct SELinux labels Teleport SSH
will not be able to function properly.
If `teleport-update` is unable to install the module, it will rollback to the module that was previously installed, if any.

`teleport-update` will only manage the official Teleport SELinux module. Modules created by users or third parties will not be inspected or
modified. If an SELinux module installed that shares a name with the official Teleport SELinux module it will be assumed to be a version of
the Teleport SELinux module and may be modified.

If the SSH service itself was allowed to manage its own SELinux module, an attacker that compromised the SSH service could modify the module to
be more permissive, defeating the point of the module in the first place.

`teleport-update` is included in release tarballs and DEB/RPM packages, the new shell script will be included in
release tarballs, and the installation script at https://goteleport.com/download ultimately downloads release
tarballs or DEB/RPM packages so there should always be a way to install the SELinux module no matter how Teleport is installed.

SELinux has 3 modes, disabled, permissive (module violations are only logged) and enforcing (attempts to violate the module are denied).
The SELinux mode will not be modified by `teleport-update`, users will be responsible for enabling SELinux if they so choose.

The `teleport-update` will have a `--dry-run` flag added to the necessary subcommands that will print what actions it would need to take to
manage the module if run normally and print the module that would be installed based on the provided Teleport agent configuration file.

The following RPM packages will be required to install the SELinux module:
- selinux-policy-devel
- policycoreutils

#### Examples

```
# installing a module with auto-updates enabled
$ teleport-update enable --enable-selinux
SELinux module installed successfully
# installing a module manually (ex. self-hosted)
$ ./install-selinux.sh
SELinux module installed successfully
# with --dry-run
$ teleport-update enable --enable-selinux --dry-run
<SELinux module contents>
> checkmodule -m teleport.te -o teleport.mod
> semodule_package -o teleport.pp -m teleport.mod
> semodule -i teleport.pp
```

### Compliance

To ensure the SSH service SELinux module is installed and SELinux is configured to enforce it, a `--selinux` flag will be added to the `teleport start` subcommand
of Teleport agents. If the SELinux module is not installed or SELinux is not present and in enforcing mode and `--selinux` is passed, the Teleport agent
will exit with an error.

Additionally since the SSH service is the only Teleport agent service that will be initially supported by the module, if the `--selinux` flag is passed
and the Teleport agent does not have the SSH service enabled, or at least one other Teleport service is enabled then the Teleport agent will exit with an error.
Additionally if there is a SELinux module for Teleport agents installed but it does not match the module that would be generated with the version of the
module embedded in the Teleport agent and the Teleport agent's configuration file it will notify the user and exit with an error. Users will then have to
run `teleport-update` to update the SELinux module.

### UX

A `troubleshoot` subcommand will be added to the new `teleport selinux` subcommand that will attempt to use the `audit2why` command
to read and interpret Teleport SELinux module denies to aid in troubleshooting on client hosts. It will print logged denials of the SELinux module
and some information about why the actions were denied.

### Host support

Initially RHEL based distributions, primarily RHEL 8 and 9, and Rocky Linux 8 and 9 will be supported.
Other distributions such as Debian based distributions may be supported in the future.

### Security

This module when installed and SELinux is set to the enforcing mode will only increase the security of Teleport SSH services. The authenticity of
the modules contained in release assets served by release channels is out of scope for this RFD.

### Test plan

Install the module in permissive mode before running through test cases that involve the SSH service. Check that exercising the test cases do not
create any module violations.

Specifically, the following SSH service features will be tested with the module installed:
- Creation of host users and groups
- Enhanced session recording with BPF
- Creation of PAM contexts
- TCP port forwarding
- X11 forwarding
- SFTP support

Infrastructure to support automated testing of Teleport SSH with the SELinux module installed on RHEL 8 and 9 will be added as well.
The automated tests will give a clear indicator of when changes in Teleport require the SELinux module to be updated as well.
The tests will be run on every commit pushed to the master and release branches that update Teleport.

### Future work

The module could be updated to make permissions of optional SSH service features controlled by SELinux booleans to remove even more unneeded privileges.

The module could be extended to cover other Teleport agent components, and/or the Auth and Proxy services.

Additional Linux distributions could be supported, such as Debian based distributions.
