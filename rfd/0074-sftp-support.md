---
authors: Andrew LeFevre (andrew.lefevre@goteleport.com)
state: implemented
---

# RFD 74 - SFTP Support

## Required Approvers

- Engineering: `@r0mant && @jakule`
- Product: `@klizhentas || @xinding33`

## What

Add SFTP support to `tsh` and Teleport Node and Proxy services. This will
not replace scp, both will be supported.

## Why

[OpenSSH 9.0](https://www.openssh.com/txt/release-9.0) changed `scp` to
use the [SFTP](https://datatracker.ietf.org/doc/html/draft-ietf-secsh-filexfer-02)
protocol by default instead of scp/rcp protocol. This is to
combat many security issues present in the legacy scp/rcp protocol. Users
running the latest OpenSSH version will currently find that `scp` and various
tools that use it under the hood, such as Ansible and various JetBrain IDEs
will not work through Teleport anymore. This can be fixed temporarily by passing
a flag to `scp`, but that can be difficult when `scp` is being invoked indirectly
by other tools.

## Details

[github.com/pkg/sftp](https://pkg.go.dev/github.com/pkg/sftp) will be leveraged
heavily to provide both an SFTP client and server.

SFTP support will be added in three stages.

1. Adding SFTP subsystem support to Teleport's SSH server.
When an SFTP request is first received on an SSH connection, the Teleport daemon
will be re-executed as the logon user of the SSH connection. This will require
adding a hidden `sftp` sub command to the `teleport` binary. The parent process will
create 2 anonymous pipes and pass them to the child (`teleport sftp`) so the child
can access the SFTP connection. The `ssh_file_copying` option will be added to
the `ssh_server` yaml config to control whether scp and sftp will be enabled or not
for per node.

2. Adding SFTP protocol support to `tsh scp` sub command, and
ensuring SFTP transfers work on the web UI. `tsh scp` will continue to use the scp/rcp
protocol by default for backwards compatibility, but the SFTP protocol can be
optionally enabled with the `-s` flag.

3. Making the SFTP protocol be used by default for
`tsh scp`, and adding the `-O` flag to allow optional usage of thee scp/rcp protocol
for backwards compatibility. This stage will take place in a future major release
(11?) and will be documented as a potentially breaking change.

Both the `-s` and `-O` flags are what OpenSSH uses to allow the user to choose
what protocol to use, and the deprecation process of making SFTP optional then
default is what OpenSSH did as well.

A potential fourth stage would be adding the `tsh sftp` sub command which would
be very similar in behavior to OpenSSH's `sftp` command. This will only be done
if users express a need for it.

#### Roles

A role option `ssh_file_copying` will be added that will define if file
copying via scp/rcp or sftp protocols will be allowed. This role option will
be true by default.

If a user has multiple roles that have different values for `ssh_file_copying`,
then file copying will be disabled if the role restricting file copying matches
the server the user is trying to access. For example:

```yaml
ind: role
version: v5
metadata:
  name: allow-copy-test
options:
  ssh_file_copying: true
spec:
  allow:
    labels:
      env: test

kind: role
version: v5
metadata:
  name: deny-copy-prod
options:
  ssh_file_copying: false
spec:
  allow:
    labels:
      env: prod
```

File copying would be disabled for nodes with label "env: prod" but enabled
for "env: test" if the user has both of these roles attached.

### Security

SFTP uses the SSH protocol to provide confidentiality and integrity, as it is
used inside an existing SSH connection. It also has the benefit of not starting
processes with attacker-controlled arguments like the scp/rcp protocol does.

As mentioned above, Teleport Node services will re-execute themselves as the
SSH login user to handle SFTP connections. This will ensure users can only
access and modify files they are allowed to.

### Auditing

A new auditing event will be added: a SFTP event that will be emitted whenever
a SFTP file operation is attempted (ie Open, Read, Write, Close). This will contain
details of the SFTP request and any errors that result from the Teleport Node handling it.

### UX

The UX of `tsh scp` with SFTP enabled will be very similar to that of `tsh scp`
with scp/rcp enabled, but there will be some differences of behavior between the
two. SFTP does not natively support expanding home directories, so absolute paths
will have to be used for remote paths. Likewise passing quoted commands to be
remotely executed (ex. ``tsh scp my_file 'user@host:/tmp/`whoami`.txt'``) will
not work as `teleport sftp` is run without any other arguments.

The web UI for transferring files will need to have a method of allowing the
user to choose between scp and SFTP. A knob could be added for this purpose,
and it would be grayed out if the user is not allowed to use either scp or
SFTP due to role constraints.

Examples of `tsh scp` with SFTP enabled:

```bash
# explicitly use sftp protocol
tsh scp -s ~/Downloads/notes.txt user@cluster.host:/home/user/Documents
# explicitly use scp/rcp protocol
tsh scp -O user@cluster.host:/home/user/Documents/notes.txt ~/Downloads/notes.txt
```

### Future Work

- Add a `tsh sftp` subcommand that closely mirrors OpenSSH's `sftp` command
