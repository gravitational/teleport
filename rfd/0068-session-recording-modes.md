---
authors: Gabriel Corado (gabriel.oliveira@goteleport.com)
state: implemented
---

# RFD 68 - Session recording modes

## What

This RFD proposes giving users a configuration to decide how to deal with
sessions that can't be recorded.

## Why

In cases where Teleport is configured to record sessions on the node using the
asynchronous format, disk failures (for example, no space left) causes the node
to be inaccessible. Where Teleport is the only way of accessing those resources,
administrators cannot perform the necessary actions on these nodes to bring them
back to a normal state.

## Goals

* Prevent nodes from being inaccessible when there are session recording
  failures.
* Enable users to configure the strictness of the presence of session
  recordings.

**Non-goals:**

* Infrastructure-related metrics (such as disk space left).
* Enable access when the recording mode is set to the proxy.
* Handle audit logging (events) failures.
* Cover applications and databases "session recordings".

## Details

As mentioned before, recording modes will define how Teleport proceeds in case
of failures. We only consider asynchronous session recordings on the servers
(`session_recording: node` on Teleport config) for this RFD. The failures
covered are related to I/O while writing session recording events to the server
disk.

There are going to be two audit modes:

* **Strict**: Teleport won't tolerate session recording to fail. In practice, if
  Teleport cannot record the session, it will not start or will be terminated
  (in case it is already in progress). This mode will be the recommended one as
  it guarantees all sessions are being recorded.
* **Best-effort**: Session recording failures are tolerated in this mode, meaning
  that Teleport will allow sessions without recording them. System
  administrators might use this mode to be able to access resources continually.

## UX

### Configuration

The configuration is going to be done at the role level. The role option
`record_session` will be extended to hold the `strict` and `best_effort` audit
mode values. It will have one entry per session kind (desktop, databases,
applications, Kubernetes, and SSH). In addition, there will be a `default` entry
for setting a default value used when sessions kind doesn't have one set.

The default value (when none is specified in the user’s role) is `best_effort`.
This allows users to access their servers in case of failures without changing
their roles. Note that this is a behavior change since, currently, Teleport
prevents sessions from happening if there is a failure on the recording.

For this RFD, only the `default`, `k8s` and `ssh` options are going to be added.

**Note**: Currently, the option `record_session.desktop` uses a boolean value to
enable/disable session recordings. When adding recording modes to desktop
access, this property will have to be converted to a string to support accepting
the modes. This change will have to be backward compatible.

**Role example:**

```yaml
kind: role
version: v5
metadata:
  name: alice
spec:
  allow:
    ...
  options:
    record_session:
      default: strict|best_effort
      ssh: strict|best_effort
      k8s: strict|best_effort
```

#### Modes precedence

The strict mode takes precedence to avoid giving undesired users the ability to
start a session in the best-effort mode. For example, if the user has multiple
roles attached to them, and at least one has the option set to `strict`, their
session recording will be on strict mode. This rule also applies to the
`default` values.

When users have multiple roles assigned, the session recording mode will be set
to the strictest. See the example below:

Let's say a user has two roles assigned:

```yaml
# Role 1:
options:
  record_session:
    default: strict
    ssh: best_effort
    k8s: strict

# Role 2:
options:
  record_session:
    default: best_effort
```

If the user:
* starts an SSH session: mode will be set to `best_effort`.
* starts a Kubernetes session: mode will be set to `strict` because role 1 is
  stricter.
* starts a desktop session: none of the roles define a direct value for desktop
  so that the strictest default value will be used. In this case, from role 1.

### Examples

#### Strict mode

```shell
# Prevent users from starting new SSH sessions when it encounters an audit error.
$ tsh ssh root@node
Session could not start due to node error.
ERROR: ssh: could not start shell
```

```shell
# Terminate the SSH session if it encounters an audit error.
$ tsh ssh root@node
root@node:~$ echo "Hello server"
Hello server

Session terminating due to node error.
Closing session...

# Same for Kubernetes exec sessions.
$ tsh kube exec -t pod-name bash
root@container:~$ echo "Hello server"
Hello server

Session terminating due to node error.
Closing session...
```

#### Best effort mode

```shell
# SSH Session starts with a "warning" message.
$ tsh ssh root@node
Warning: node error. This might cause some functionalities not to work correctly.

root@node:~$ echo "Hello server"
Hello server
root@node:~$ exit
logout
the connection was closed on the remote side on 01 Jan 22 00:00 -00

# Same for Kubernetes exec sessions.
$ tsh kube exec -t pod-name bash
Warning: node error. This might cause some functionalities not to work correctly.
root@container:~$ echo "Hello server"
Hello server
root@container:~$ exit
```

```shell
# Keep the session running if it encounters an audit error.
$ tsh ssh root@node
root@node:~$ echo "Hello server"
Hello server
Warning: node error. This might cause some functionalities not to work correctly.
root@node:~$ exit

# Same for Kubernetes exec sessions.
$ tsh kube exec -t pod-name bash
root@container:~$ echo "Hello server"
Hello server
Warning: node error. This might cause some functionalities not to work correctly.
root@container:~$ exit
```

## Security

### Bypassing session recording

Although this RFD doesn't cover session recording toggle, the best effort mode
will purpose scenarios where it is ok not to have session recording. Users with
this mode set to their role might use this to have unrecorded sessions.

Another way to corrupt session recording is to fill the node disk during the
session, in this case, Teleport won't be able to record the session and upload
it. For this, when the session is set to strict, the session will terminate as
soon as Teleport encounters a failure.
