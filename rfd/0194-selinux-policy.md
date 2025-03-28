---
authors: Andrew LeFevre (andrew.lefevere@goteleport.com)
state: draft
---

# RFD 0194 - Official SELinux Policy for Teleport SSH Agents

## Required Approvers

- Engineering: (@r0mant && @rosstimothy)
- Security: Doyensec

## What

This RFD proposes an official SELinux policy for Teleport SSH services. The policy will enforce that minimal permissions are given to SSH services,
and that security boundaries between the main server process and child session processes are respected.

The policy will be bundled in Teleport agent binaries and a new subcommand will be added to Teleport agents that will allow installation and configuration of the policy.

## Why

Teleport is often deployed in environments with strict security requirements. SELinux provides a way to enforce fine-grained access controls, which
can help prevent privilege escalation and lateral movement, provide peace of mind, and comply with security standards. As agents are the most widely
deployed component of Teleport, it is important to ensure that they are secure and compliant.

## Scope

Only Teleport SSH services are initially in scope for this proposal as a starting point, but the SELinux policy may be extended to other components of Teleport in the future.
The policy will initially allow the following features and functionality of Teleport SSH services:

- SSH connections
- SFTP connections
- Enhanced session recording
- Auditd logging
- PAM support

A new subcommand to `teleport` will be added that will allow users to update or install Teleport's SELinux policy. A flag will be added to `teleport start` that will check that the Teleport SELinux policy is installed and configured correctly, and exit with an error otherwise.

## Details

### SELinux Policy Structure

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
and management difficulties this would create, the SELinux policy will create one domain and grant it all permissions that
the Teleport SSH service and its child processes require.

This is largely the approach OpenSSH takes as well in its official SELinux policy. `sshd` has one main domain that encompasses
the main server processes as well as processes that handle SSH sessions.

Configurable file and port contexts will be created to bind to the necessary ports and read and write to the necessary files, depending on the SSH service configuration. At a minimum this will allow a Teleport SSH service to bind to
the configured SSH port and read and write to the data directory, and dial the configured Teleport Proxy and Auth servers. 

There are a few optional features that can be enabled or disabled on SSH services, the first version of the policy will ensure necessary permissions
for all optional features in scope are allowed to simplify development and deployment.

These include:

- Reading `~/.tsh/environment` on the server before creating a session
    - The `teleport_ssh_service_intermediary` domain will be allowed to read `.tsh/environment` in all users' home directories
- Enhanced session recording with BPF
   - The `teleport_ssh_service` domain will be allowed to create and manage BPF ring buffers, and load BPF programs
- Creation of PAM contexts
    - The `teleport_ssh_service_intermediary` domain will be allowed to `dlopen` the PAM module and call it's exported functions
    - The location of the PAM module will be configurable
- SFTP support
    - The `teleport_ssh_service_intermediary` domain will be allowed to re-execute itself with `sftp` as the first argument

The SELinux policy will be created as a SELinux Reference Policy (refpolicy), to maximize readability and maintainability.

The main alternative is to use SELinux Common Intermediate Language (CIL) which bridges refpolicies and compiled binary policy files.
CIL is extremely powerful but much more difficult to understand by humans, it's meant for consumption by SELinux tools.

#### Future updates

The SELinux policy will be updated in the future to support new features and maintain compatibility with existing features.
Any changes to the SSH service that result in new files being managed, new network activity (listening on new ports, dialing new addresses etc),
new binaries being executed, or new Linux syscalls being called will require updates to the SELinux policy.

### Installation and updating

The policy source will be embedded in future Teleport agent binaries.

A `selinux` subcommand will be added to `teleport` to allow users to install and configure the SELinux policy. This subcommand
will use the Teleport agent configuration file to determine how to configure some of the tunable SELinux policy contexts, such as port
numbers, file paths, and booleans using the `semanage` tool.
`teleport selinux` will only manage the official Teleport SELinux policy. Policies created by users or third parties will not be inspected or
modified. If a SELinux policy installed that shares a name with the official Teleport SELinux policy it will be assumed to be a version of
the Teleport SELinux policy and may be modified.

The policy will allow the Teleport agent processes with the `selinux` subcommand only to create or update its SELinux policy. This will be enforced by
creating a `teleport_selinux_management` domain, which will not be allowed to be a parent of any other Teleport related domain. Otherwise an attacker could
use the SSH service to run the `selinux` subcommand and potentially modify the policy.

If the SSH service itself was allowed to manage its own SELinux policy, an attacker that compromised the SSH service could modify the policy to
be more permissive, defeating the point of the policy in the first place.

If SELinux is not present on the host, the subcommand will tell the user this and exit. If SELinux is present but the Teleport agent policy is
not installed, the subcommand will extract the embedded policy and use `checkmodule`, `semodule_package`, and `semodule` to compile and install the policy.

Installation scripts and the `teleport-update` tool will be updated to install and configure the SELinux policy with the `selinux` subcommand.

SELinux has 3 modes, disabled, permissive (policy violations are only logged) and enforcing (attempts to violate the policy are denied).
The SELinux mode will not be modified by the `selinux` subcommand, users will be responsible for enabling SELinux if they so choose.

The `selinux` subcommand will have a `--dry-run` flag that will print what actions it would need to take to manage the policy if run normally and
print the policy that would be installed based on the provided Teleport agent configuration file.

#### Examples

```
# installing a policy
$ teleport selinux -c /etc/teleport.yaml
SELinux policy installed successfully
# with --dry-run
$ teleport selinux -c /etc/teleport.yaml --dry-run
<SELinux policy contents>
> checkmodule -m teleport.te -o teleport.mod
> semodule_package -o teleport.pp -m teleport.mod
> semodule -i teleport.pp
```

### Compliance

To ensure the SSH service SELinux policy is installed and SELinux is configured to enforce it, a `--selinux` flag will be added to the `teleport start` subcommand
of Teleport agents. If the SELinux policy is not installed or SELinux is not present and in enforcing mode and `--selinux` is passed, the Teleport agent
will exit with an error.

Additionally since the SSH service is the only Teleport agent service that will be initially supported by the policy, if the `--selinux` flag is passed
and the Teleport agent does not have the SSH service enabled, or at least one other Teleport service is enabled then the Teleport agent will exit with an error.
Additionally if there is a SELinux policy for Teleport agents installed but it does not match the policy that would be generated with the version of the
policy embedded in the Teleport agent and the Teleport agent's configuration file it will notify the user and exit with an error. Users will then have to
run the `selinux` subcommand to update the SELinux policy.

### UX

A `--troubleshoot` flag will be added to the `teleport selinux` subcommand that will attempt to use the `audit2why` command
to read and interpret Teleport SELinux policy denies to aid in troubleshooting on client hosts.

### Host support

Initially RHEL based distributions, primarily RHEL 7 and 8, and Rocky Linux 8 will be supported.
Other distributions such as Debian based distributions may be supported in the future.

### Security

This policy when installed and SELinux is set to the enforcing mode will only increase the security of Teleport SSH services. The authenticity of
the policies contained in release assets served by release channels is out of scope for this RFD.

### Test plan

Install the policy in permissive mode before running through test cases that involve the SSH service. Check that exercising the test cases do not
create any policy violations.

Specifically, the following SSH service features will be tested with the policy installed:
- Creation of host users and groups
- Enhanced session recording with BPF
- Creation of PAM contexts
- TCP port forwarding
- X11 forwarding
- SFTP support

### Future work

The policy could be updated to make permissions of optional SSH service features controlled by SELinux booleans to remove even more unneeded privileges.

The policy could be extended to cover other Teleport agent components, the agent auto-updater, and/or the Auth and Proxy services.

Additional Linux distributions could be supported, such as Debian based distributions.
