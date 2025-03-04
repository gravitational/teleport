---
authors: Andrew LeFevre (andrew.lefevere@goteleport.com)
state: draft
---

# RFD 0194 - Official SELinux Policy for Teleport SSH Agents

## Required Approvers

- Engineering: (@r0mant && @rosstimothy)
- Security: Doyensec

## What

This RFD proposes an official SELinux policy for Teleport SSH agents. The policy will enforce that minimal permissions are given to SSH agents,
and that security boundaries between the main server process and child session processes are respected.

The policy will be bundled in Teleport agent binaries and a new subcommand will be added to Teleport agents that will allow installation and configuration of the policy.

## Why

Teleport is often deployed in environments with strict security requirements. SELinux provides a way to enforce fine-grained access controls, which
can help prevent privilege escalation and lateral movement, provide peace of mind, and comply with security standards. As agents are the most widely
deployed component of Teleport, it is important to ensure that they are secure and compliant.

Only SSH agents are initially in scope for this proposal as a starting point, but the SELinux policy may be extended to other components of Teleport in the future.

## Details

### SELinux Policy Structure

SSH agents need a lot of privileges to function properly. They need to bind to privileged ports, manage cgroups, create processes, and more.
When an SSH session is started, the SSH service will create 2 child processes to handle it: 

- An intermediary process that prepares another child process that will actually run the SSH session. Depending on what is configured, PAM contexts
may be created, users may be created, a shell will be created, the user and group will be changed if necessary, and so on.
- The intermediary process will then create the child session process, which only needs standard permissions of the host user that the SSH session
was created for.

Additionally, the SSH agent may create a networking child process to handle X11 forwarding, port forwarding, and SSH Agent forwarding. This process is a child process of the intermediary process as well.

4 main SELinux domains will be created to ensure each process in the chain is granted only the necessary permissions: `teleport_ssh_agent`, `teleport_ssh_agent_intermediary`, `teleport_ssh_agent_session`, and `teleport_ssh_networking` respectively.

Configurable file and port contexts will be created for the `teleport_ssh_agent` domain, and configurable file contexts
will be created for the `teleport_ssh_agent_intermediary` domain to allow those processes to bind to the necessary ports and read and write
to the necessary files, depending on the SSH agent configuration.

There are a few optional features that can be enabled or disabled on SSH agents, the first version of the policy will ensure necessary permissions for all optional features are allowed to simplify development and deployment.

These include:

- Reading `~/.tsh/environment` on the server before creating a session
    - The `teleport_ssh_agent_intermediary` domain will be allowed to read `.tsh/environment` in all users' home directories
- Creation of host users and groups
    - The `teleport_ssh_agent` domain will be allowed to call necessary binaries such as `useradd`, `usermod` and `groupadd`
- Enhanced session recording with BPF
   - The `teleport_ssh_agent` domain will be allowed to create and manage BPF ring buffers, and load BPF programs
- Creation of PAM contexts
    - The `teleport_ssh_agent_intermediary` domain will be allowed to `dlopen` the PAM library and call it's exported functions
- TCP port forwarding
    - The `teleport_ssh_networking` domain will be allowed to listen to arbitrary ports and connect to arbitrary network addresses
- X11 forwarding
    - The `teleport_ssh_networking` domain will be allowed to create and manage X11 sockets
- SFTP support
    - The `teleport_ssh_agent_intermediary` domain will be allowed to re-execute itself with `sftp` as the first argument

The SELinux policy will be created as a SELinux Reference Policy (refpolicy), to maximize readability and maintainability.

The main alternative is to use SELinux Common Intermediate Language (CIL) which bridges refpolicies and compiled binary policy files. CIL is extremely powerful but much more difficult to understand by humans, it's meant for consumption by SELinux tools.

### Installation and updating

The policy source will be embedded in future Teleport agent binaries.

A `selinux` subcommand will be added to Teleport agent binaries to allow users to install and configure the SELinux policy. This subcommand
will will use the Teleport agent configuration file to determine how to configure some of the tunable SELinux policy contexts, such as port
numbers, file paths, and booleans using the `semanage` tool. If SELinux is not present on the host, the subcommand will tell the user this
and exit. If SELinux is present but the Teleport agent policy is not installed, the subcommand will extract the embedded policy and use
`checkmodule`, `semodule_package`, and `semodule` to compile and install the policy.

Installation scripts and the `teleport-update` tool will be updated to install and configure the SELinux policy with the `selinux` subcommand.

The SELinux mode will not be modified by the `selinux` subcommand, users will be responsible for enabling SELinux if they so choose.

### Compliance

To ensure the SSH agent SELinux policy is installed and SELinux is configured to enforce it, a `--selinux` flag will be added to the `teleport start` subcommand
of Teleport agents. If the SELinux policy is not installed or SELinux is not present and in enforcing mode and `--selinux` is passed, the Teleport agent
will exit with an error.

### Host support

Initially RHEL based distributions, primarily RHEL 7 and 8, and Rocky Linux 8 will be supported.
Other distributions such as Debian based distributions may be supported in the future.

### Security

This policy when installed and SELinux is set to the enforcing mode will only increase the security of Teleport SSH agents. The authenticity of
the policies contained in release assets served by the download server is out of scope for this RFD.

### Test plan

Install the policy in permissive mode before running through test cases that involve the SSH service. Check that exercising the test cases do not
create any policy violations.

### Future work

The policy could be updated to make permissions of optional SSH agent features controlled by SELinux booleans to remove even more unneeded privileges.

The policy could be extended to cover other Teleport agent components, the agent auto-updater, and/or the Auth and Proxy services.

Additional Linux distributions could be supported, such as Debian based distributions.
