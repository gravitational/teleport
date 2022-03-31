---
authors: Gabriel Corado (gabriel.oliveira@goteleport.com)
state: draft
---

# RFD 64 - Audit modes

## What

This RFD proposes giving users a configuration to decide how to deal with
sessions that can't be recorded or have audit logging failures.

## Why

In cases where Teleport is configured to record sessions on the node using the
asynchronous format, disk failures (for example, no space left) causes the node
to be inaccessible. Where Teleport is the only way of accessing those resources,
administrators cannot perform the necessary actions on these nodes to bring them
back to a normal state.

## Goals

* Prevent nodes from being inaccessible when there are audit errors.
* Enable users to configure the strictness of audit failures.

**Non-goals:**

* Infrastructure-related metrics (such as disk space left).
* Enable access when the recording mode is set to the proxy.

## Details

Audit modes will define how Teleport proceeds in case of audit failures. Those
failures vary depending on how session recording is configured:

| Recording mode                 | Failures handled                                             |
| ------------------------------ | ------------------------------------------------------------ |
| `node` and `proxy`             | I/O errors while writing audit logs/session recording data on the server disk. |
| `node-async` and `proxy-async` | Connection errors with Proxy/Auth server while streaming audit logs/session recording data. |

There are going to be two audit modes:

* **Strict**: Teleport won't tolerate session recording or audit logging to fail.
  In practice, if Teleport cannot record the session, it will not start or will
  be terminated (in case it is already in progress). This mode will be the
  recommended one as it guarantees all sessions are being recorded.
* **Best-effort**: Audit failures are tolerated in this mode, meaning that Teleport
  will allow sessions without recording them. System administrators might use
  this mode to be able to access resources continually.

## UX

### Configuration

The configuration is going to be done at the role level. The role option
`record_session` will be extended to hold the audit mode values: `strict` and
`best_effort`. It will have one entry per session kind (desktop, databases,
applications, Kubernetes, and SSH) and an `all` entry for cases where users
want the same behavior for all kinds.

**Note**: Currently, the option  `record_session.desktop` uses a boolean value to
disable/enable session recording. It will have to be converted into a new
format: `true` will be charged to the default audit mode, and `false` will be
`off`. Disabling session recording is out of the scope of this RFD, meaning that
the `off` value will only be present on the `desktop` kind.

**Role example (with all session kinds):**

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
      all: strict|best_effort
      ssh: strict|best_effort
      app: strict|best_effort
      db: strict|best_effort
      k8s: strict|best_effort
      desktop: strict|best_effort|off
```

### Examples

### Strict mode

```shell
# Prevent users from starting new SSH sessions when it encounters an audit error.
$ tsh ssh root@node
Node could not record audit events: no space left on device.
ERROR: ssh: could not start shell
```

```shell
# Terminate the SSH session if it encounters an audit error.
$ tsh ssh root@node
root@node:~$ echo "Hello server"
Hello server

Node could not record audit events: no space left on device.
Closing session...

# Same for Kubernetes exec sessions.
$ tsh kube exec -t pod-name bash
root@container:~$ echo "Hello server"
Hello server

Node could not record audit events: no space left on device.
Closing session...
```

```shell
# Non-interactive sessions (such as Applications and K8S) will fail the next request made.
$ tsh app login dumper
$ curl ...
Forbidden error...

$ tsh kube login my-cluster
$ kubectl ...
Forbidden error...
```

### Best effort mode

```shell
# SSH Session starts with a "warning" message.
$ tsh ssh root@node
Disabling recording and audit events for this session, reason: node error: no space left on device

root@node:~$ echo "Hello server"
Hello server
root@node:~$ exit
logout
the connection was closed on the remote side on 01 Jan 22 00:00 -00

# Same for Kubernetes exec sessions.
$ tsh kube exec -t pod-name bash
Disabling recording and audit events for this session, reason: node error: no space left on device
root@container:~$ echo "Hello server"
Hello server
root@container:~$ exit
```

```shell
# Keep the session running if it encounters an audit error.
$ tsh ssh root@node
root@node:~$ echo "Hello server"
Hello server
Disabling recording and audit events for this session, reason: node error: no space left on device
root@node:~$ exit

# Same for Kubernetes exec sessions.
$ tsh kube exec -t pod-name bash
root@container:~$ echo "Hello server"
Hello server
Disabling recording and audit events for this session, reason: node error: no space left on device
root@container:~$ exit
```

```shell
# Non-interactive sessions (such as Applications and K8S) will not fail subsequent requests.
$ tsh app login dumper
$ curl ...
...success output...

$ tsh kube login my-cluster
$ kubectl ...
...success output...
```

## Security

### Bypassing session recording

Although this RFD doesn't cover session recording toggle, the best effort mode
will purpose scenarios where it is ok not to have session recording. Users with
this mode set to their role might use this to have unrecorded sessions. The
strict mode takes precedence to avoid giving undesired users the ability to
start a session in the best-effort mode. For example, if the user has multiple
roles attached to them, and at least one has the option set to `strict`, their
session will be on strict mode.

Another way to corrupt session recording is to fill the node disk during the
session, in this case, Teleport won't be able to record the session and upload
it. For this, when the session is set to strict, the session will terminate as
soon as Teleport encounters a failure.
