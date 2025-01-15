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
cluster level. By default all Cloud clusters will be opted into automatic
updates for client tools. Cluster administrators using MDM software like Jamf
will be able opt-out manually manage updates.

Self-hosted clusters will be be opted out, but have the option to use the same
automatic update mechanism.

Inspiration drawn from https://go.dev/doc/toolchain.

### Implementation

#### Client tools

##### Automatic updates

When `tsh login` is executed, client tools will check `/v1/webapi/find` to
determine if automatic updates are enabled. If the cluster's required version
differs from the current binary, client tools will download and re-execute
using the version required by the cluster. All other `tsh` subcommands (like
`tsh ssh ...`) will always use the downloaded version.

The original client tools binaries won't be overwritten. Instead, an additional
binary will be downloaded and stored in `~/.tsh/bin` with `0755` permissions.

To validate the binaries have not been corrupted during download, a hash of the
archive will be checked against the expected value. The expected hash value
comes from the archive download path with `.sha256` appended.

To enable concurrent operation of client tools, a locking mechanisms utilizing
[syscall.Flock](https://pkg.go.dev/syscall#Flock) (for Linux and macOS) and
[LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
(for Windows) will be used.

```
$ tree ~/.tsh
~/.tsh
├── bin
│  ├── tctl
│  └── tsh
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

All archive downloads are targeted to the `cdn.teleport.dev` endpoint and depend 
on the operating system, platform, and edition. Where edition must be identified 
by the original client tools binary, URL pattern:
`https://cdn.teleport.dev/teleport-{, ent-}v15.3.0-{linux, darwin, windows}-{amd64,arm64,arm,386}-{fips-}bin.tar.gz`

An environment variable `TELEPORT_TOOLS_VERSION` will be introduced that can be
`X.Y.Z` (use specific semver version) or `off` (do not update). This
environment variable can be used as a emergency workaround for a known issue,
pinning to a specific version in CI/CD, or for debugging.

During re-execution, child process will inherit all environment variables and
flags. `TELEPORT_TOOLS_VERSION=off` will be added during re-execution to
prevent infinite loops.

When `tctl` is used to connect to Auth Service running on the same host over
`localhost`, `tctl` assumes a special administrator role that can perform all
operations on a cluster. In this situation the expectation is for the version
of `tctl` and `teleport` to match so automatic updates will not be used.

> [!NOTE]
> If a user connects to multiple root clusters, each running a different
> version of Teleport, client tools will attempt to download the differing
> version of Teleport each time the user performs a `tsh login`.
>
> In practice, the number of users impacted by this would be small. Customer
> Cloud tenants would be on the same version and this feature is turned off by
> default for self-hosted cluster.
>
> However, for those people in this situation, the recommendation would be to
> use self-managed updates.

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

Enroll in automatic updates to keep client tools like tsh and tctl
automatically updated. https://goteleport.com/docs/upgrading/automatic-updates

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

Enroll in automatic updates to keep client tools like tsh and tctl
automatically updated. https://goteleport.com/docs/upgrading/automatic-updates
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

Enroll in automatic updates to keep client tools like tsh and tctl
automatically updated. https://goteleport.com/docs/upgrading/automatic-updates

Use the "--skip-version-check" flag to bypass this check and attempt to connect
to this cluster.
```

#### Self-managed client tools updates

Cluster administrators that want to self-manage client tools updates will be
able to get changes to client tools versions which can then be
used to trigger other integrations (using MDM software like Jamf) to update the
installed version of client tools on endpoints.

By defining the `proxy` flag, we can use the get command without logging in.

```
$ tctl autoupdate client-tools status --proxy proxy.example.com --format json
{
    "mode": "enabled",
    "target_version": "X.Y.Z"
}
```

##### Cluster configuration

Enrollment of clients in automatic updates will be enforced at the cluster
level.

The `autoupdate_config` resource will be updated to allow cluster
administrators to turn client tools automatic updates `on` or `off`.
A `autoupdate_version` resource will be added to allow cluster administrators
to manage the version of tools pushed to clients.

> [!NOTE]
> Client tools configuration is broken into two resources to [prevent
> updates](https://github.com/gravitational/teleport/blob/master/lib/modules/modules.go#L332-L355)
> to `autoupdate_version` on Cloud.
>
> While Cloud customers will be able to use `autoupdate_config` to
> turn client tools automatic updates `off` and self-manage updates, they will
> not be able to control the version of client tools in `autoupdate_version`.
> That will continue to be managed by the Teleport Cloud team.

Both resources can either be updated directly or by using `tctl` helper
functions.

```yaml
kind: autoupdate_config
spec:
  tools:
    # tools mode allows to enable client tools updates or disable at the
    # cluster level. Disable client tools automatic updates only if self-managed
    # updates are in place.
    mode: enabled|disabled
```
```
$ tctl autoupdate client-tools enable
client tools auto update mode has been changed

$ tctl autoupdate client-tools disable
client tools auto update mode has been changed
```

By default, all Cloud clusters will be opted into `tools.mode: enabled`. All
self-hosted clusters will be opted into `tools.mode: disabled`.

```yaml
kind: autoupdate_version
spec:
  tools:
    # target_version is the semver version of client tools the cluster will
    # advertise.
    target_version: X.Y.Z
```
```
$ tctl autoupdate client-tools target X.Y.Z
client tools auto update target version has been set

$ tctl autoupdate client-tools target --clear
client tools auto update target version has been cleared
```

For Cloud clusters, `target_version` will always be `X.Y.Z`, with the version
controlled by the Cloud team.

The above configuration will then be available from the unauthenticated
proxy discovery endpoint `/v1/webapi/find` which clients will consult.
Resources that store information about autoupdate and tools version are cached on 
the proxy side to minimize requests to the auth service. In case of an unhealthy 
cache state, the last known version of the resources should be used for the response.

```
$ curl https://proxy.example.com/v1/webapi/find | jq .auto_update
{
    "tools_auto_update": true,
    "tools_version": "X.Y.Z",
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
