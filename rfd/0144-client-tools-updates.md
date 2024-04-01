---
authors: Russell Jones (rjones@goteleport.com) and Bernard Kim (bernard@goteleport.com)
state: draft
---

# RFD 0144 - Client Tools Updates

## Required Approvers

* Engineering: @sclevine && @bernardjkim && @r0mant
* Product: @klizhentas || @xinding33
* Security: @reedloden

## What/Why

This RFD describes how client tools like `tsh` and `tctl` can be kept up to
date, either using automatic updates or self-managed updates.

Keeping client tools updated helps with security (fixes for known security
vulnerabilities are pushed to endpoints), bugs (fixes for resolved issues are
pushed to endpoints), and compatibility (users no longer have to learn and
understand [Teleport component
compatibility](https://goteleport.com/docs/upgrading/overview/#component-compatibility)
rules).

## Details

### Summary

Client tools like `tsh` and `tctl` will automatically download and install the
required version for the Teleport cluster.

Enrollment in automatic updates for client tools will be controlled at the
cluster level. By default all clusters will be opted into automatic updates for
client tools. Cluster administrators using MDM software like JamF will be able
opt-out manually manage updates.

Inspiration drawn from https://go.dev/doc/toolchain.

### Implementation

#### Client tools

##### Automatic updates

When `tsh login` is executed, client tools will check `/v1/webapi/ping` to
determine if automatic updates are enabled. If the cluster's required version
differs from the current binary, client tools will download and re-execute
using the version required by the cluster.

The original client tools binaries won't be overwritten. Instead, additional
binaries will be downloaded and stored in `~/.tsh/bin` with `0555` permissions.
A file named `version.yaml` in the same directory will maintain the mapping
between proxies and tools versions, to support auto-purge and free up disk
space from unused binaries.

To enable concurrent operation of client tools, a locking mechanisms utilizing
[syscall.Flock](https://pkg.go.dev/syscall#Flock) (for Linux and macOS) and
[LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
(for Windows) will be used.

A side effect of automatic updates will be users will be able to connect to
different Teleport clusters (running different versions of Teleport, including
pre-releases) without juggling multiple versions of client tools.

```
$ tree ~/.tsh
~/.tsh
├── bin
│  ├── 15.0.0
│  │  ├── tctl
│  │  └── tsh
│  ├── 15.1.1
│  │  ├── tctl
│  │  └── tsh
│  └── version.yaml
├── current-profile
├── keys
│  └── proxy.example.com
│     ├── cas
│     │  └── example.com.pem
│     ├── certs.pem
│     ├── foo
│     ├── foo-ssh
│     │  └── example.com-cert.pub
│     ├── foo-x509.pem
│     └── foo.pub
├── known_hosts
└── proxy.example.com.yaml
```

Users can cancel client tools updates using `Ctrl-C`. This may be needed if the
user is on a low bandwidth connection (LTE or public Wi-Fi), if the Teleport
download server is inaccessible, or the user urgently needs to access the
cluster and can not wait for the update to occur.

```
$ tsh login --proxy=proxy.example.com
Client tools are out of date, updating to vX.Y.Z.
Update progress: [▒▒▒▒▒▒     ] (Ctrl-C to cancel update)

[...]
```

An environment variable `TELEPORT_TOOLS_VERSION` will be introduced that can be
`X.Y.Z` (use specific version) or `off` (do not update). This environment
variable can be used as a emergency workaround for a known issue, pinning to a
specific version in CI/CD, or for debugging.

During re-execution, child process will inherit all environment variables and
flags.  `TELEPORT_TOOLS_VERSION=off` will be added during re-execution to
prevent infinite loops.

Automatic updates will not be used if `tctl` is connecting to the Auth Service
over localhost.

##### Errors and warnings

If cluster administrator has chosen not to enroll client tools in automatic
updates and does not self-manage client tools updates as outlined in
[Self-managed client tools updates](#self-managed-client-tools-updates), a
series of warnings and errors with increasing urgency will be shown to the
user.

If the version of client tools is within the same major version as advertised
by the cluster, a warning will be shown to urge the user to enroll in automatic
updates. Warnings will not prevent the user from using client tools that are
slightly out of date.

```
$ tsh login --proxy=proxy.example.com
Warning: Client tools are out of date, update to vX.Y.Z.

Update Teleport to vX.Y.Z from https://goteleport.com/download or your system
package manager.

Run "tsh autoupdate enroll" to enroll in automatic updates and keep client
tools like tsh and tctl automatically updated.

[...]
```

If the version of client tools is 1 major version below the version advertised
by the cluster, a warning will be shown that indicates some functionality may
not work.

```
$ tsh login --proxy=proxy.example.com
WARNING: Client tools are 1 major version out of date, update to vX.Y.Z.

Some functionality may not work. Update Teleport to vX.Y.Z from
https://goteleport.com/download or your system package manager.

Run "tsh autoupdate enroll" to enroll in automatic updates and keep client
tools like tsh and tctl automatically updated.
```

If the version of client tools is 2 (or more) versions lower than the version
advertised by the cluster or 1 (or more) version greater than the version
advertised by the cluster, an error will be shown and will require the user to
use the `--skip-version-check` flag.

```
$ tsh login --proxy=proxy.example.com
ERROR: Client tools are N major versions out of date, update to vX.Y.Z.

Your cluster requires {tsh,tctl} vX.Y.Z. Update Teleport from
https://goteleport.com/download or your system package manager.

Run "tsh autoupdate enroll" to enroll in automatic updates and keep client
tools like tsh and tctl automatically updated.

Use the "--skip-version-check" flag to bypass this check and attempt to connect
to this cluster.
```

#### Self-managed client tools updates

Cluster administrators that want to self-manage client tools updates will be
able to watch for changes to client tools versions which can then be used to
trigger other integrations (using MDM software like JamF) to update the
installed version of client tools on endpoints.

```
$ tctl autoupdate watch
{"tools_version": "1.0.0"}
{"tools_version": "1.0.1"}
{"tools_version": "2.0.0"}

[...]
```

##### Cluster configuration

Enrollment of clients in automatic updates will be enforced at the cluster
level.

The `cluster_maintenance_config` resource will be updated to allow cluster
administrators to turn client tools automatic updates `on` (default) or `off`.
A `autoupdate_version` resource will be added to allow cluster administrators
to manage the version of tools pushed to clients.

> [!NOTE]
> Client tools configuration is broken into two resources to [prevent
> updates](https://github.com/gravitational/teleport/blob/master/lib/modules/modules.go#L332-L355)
> to `autoupdate_verson` on Cloud.
>
> While Cloud customers will be able to use `cluster_maintenance_config` to
> turn client tools automatic updates `off` and self-manage updates, they will
> not be able to control the version of client tools in `autoupdate_verson`.
> That will continue to be managed by the Teleport Cloud team.

Both resources can either be updated directly or by using `tctl` helper
functions.

```yaml
kind: cluster_maintenance_config
spec:
  # tools_auto_update allows turning client tools updates on or off at the
  # cluster level. Only turn client tools automatic updates off if self-managed
  # updates are in place.
  tools_auto_update: on|off

  [...]
```
```
$ tctl autoupdate update --set-tools-auto-update=off
Automatic updates configuration has been updated.
```

All Cloud clusters will be opted into `tools_auto_update: on`.

All new self-hosted clusters will be opted into `tools_auto_update: on`.
Existing self-hosted clusters will be opted into `tools_auto_update: off` to
not introduce unexpected behavior.

```yaml
kind: autoupdate_version
spec:
  # tools_version is the version of client tools the cluster will advertise.
  # Can be auto (match the version of the proxy) or an exact semver formatted
  # version.
  tools_version: auto|X.Y.Z
```
```
$ tctl autoupdate update --set-tools-version=1.0.1
Automatic updates configuration has been updated.
```

For Cloud clusters, `tools_version` will always be `X.Y.Z`, with the version
controlled by the Cloud team.

For self-hosted clusters, the default will be `tools_version: auto` matching
the tools version with the proxy version. This way, automatic updates will be
available to self-hosted customers by default, without any extra effort from
the cluster administrator.

The above configuration will then be available from the unauthenticated
endpoint `/v1/webapi/ping` which clients will consult.

```
curl https://proxy.example.com/v1/webapi/ping | jq .
{
    "tools_auto_update": true,
    "tools_version": "X.Y.Z",

    [...]
}
```

### Costs

Some additional costs will be incurred as Teleport downloads will increase in
frequency.

### Out of scope

How Cloud will push changes to `autoupdate_version` is out of scope for this
RFD and will be handled by a separate Cloud specific RFD.

Automatic updates for Teleport Connect are out of scope for this RFD as it uses
a different install/update mechanism. For now it will call `tsh` with
`TELEPORT_TOOLS_VERSION=off` until automatic updates support can be added to
Connect.

### Security

The initial version of automatic updates will rely on TLS to establish
connection authenticity to the Teleport download server. The authenticity of
assets served from the download server is out of scope for this RFD. Cluster
administrators concerned with the authenticity of assets served from the
download server can use self-managed updates with system package managers which
are signed.

Phase 2 will use The Upgrade Framework (TUF) to implement secure updates.
