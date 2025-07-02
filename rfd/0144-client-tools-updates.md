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

## Teleport Connect automatic updates *(added on 2025-06-27 by @gzdunek)*

Teleport Connect is built with Electron and therefore cannot use the CLI update 
mechanisms, which simply re-executes a command with a different tool version. 
Instead, automatic updates will be implemented with the `electron-updater` package. 
This library is maintained by the author of `electron-builder`, so it should be 
fully compatible with our existing build and packaging setup. It handles all 
the heavy lifting: downloading updates, verifying checksums, and installing 
the updates.

The library supports updates on macOS, Windows, and Linux, except the .tar.gz 
target. Our initial release will not support updating .tar.gz builds; we may 
explore this in the future.

>Note: Electron's auto-update mechanism on macOS requires the ZIP target. 
>It has already been added and will be available via direct download only - 
>it will not be shown on the downloads page.

### Custom update flow

By default, `electron-updater` queries a static update server endpoint (like 
GitHub) which returns a file containing the latest version metadata. 
However, in our client tools update architecture, the latest version is dynamic, 
based on `autoupdate_version` provided by clusters.

To support this, a custom update provider will be implemented which will generate 
the update metadata (like a download URL) based on the client tools version.

### User experience

Updates will be visible in two places:

1. Login Dialog
An auto-update widget will appear when the user opens the login dialog, 
notifying them of a new version and prompting for a restart.
It will be possible to open a detailed view from this widget.
2. Additional Actions Menu (and Teleport Connect -> Check for Updates… menu on macOS)
This opens a detailed view containing a link to release notes, information about 
cluster managing updates, and actions like canceling a download or re-checking 
for updates.

Update checks will be triggered automatically when the login dialog or the app 
update dialog is opened.
To ensure a smooth user experience, available updates are downloaded as soon as 
they're found. Users will only need to restart the app to apply the update - 
either via the auto-update widget/detailed view or manually by closing the app.

### Multi-cluster support

Users of Teleport Connect may be logged into multiple clusters, each potentially 
specifying a different client tools version via `tools_auto_update`.

In the CLI, tools update themselves automatically during login to a cluster, 
with no user interaction, making the process largely transparent.
However, replicating this behavior exactly in a desktop app isn't practical, as 
applying updates requires a manual restart. This could result in an annoying 
user experience.

Example Scenario:
1. User logs into Cluster A → the app is up-to-date.
2. User logs into Cluster B → the app downloads a newer version.
3. User restarts the app to apply the update.
4. User logs back into Cluster A (after certs expire) → the app prompts for 
a downgrade.
5. User restarts the app to apply the downgrade.
6. User logs back into Cluster B (after certs expire) → the app prompts for 
an upgrade again.

This can create a feedback loop of conflicting version updates and repeated 
restarts. One potential improvement is to stop automatic downloads when multiple
client versions are detected, giving more control to the user.
However, this would effectively make updates opt-in, which could delay important 
fixes or features.

Proposed Solution:
* The app will read the client tool versions from all connected clusters, 
and install the latest one by default.
* If multiple conflicting versions are detected, the auto-update widget will 
display a warning when logging into a cluster that does not match the currently 
installed version: "App version is managed by another cluster." Clicking "More" 
will take the user to the detailed view.
* In detailed view, the user will be able to choose which cluster should 
manage updates (a mechanism similar to an update channel), the UI will look as 
follows:
> [ ] Use the latest version from your clusters
> 
> Or select cluster to manage updates:
> 
> 1. teleport-18.asteroid.earth (v18.0.3)
> 
> 2. teleport-17.asteroid.earth (v17.3.3)

The selected cluster will be stored in the app state and cleared when user logs 
out from that cluster.

### Implementation

A custom updater function will be implemented using electron-updater's `Provider` 
interface. This function will return version metadata including:
* Version number
* Download URL
* SHA-256 checksum

>️ Note: `electron-updater` seems to support only SHA-512 checksums, but actually 
> it supports SHA-256 too, via `sha2` field.
> This field doesn't seem deprecated in the source code, but for some reason is 
> not available in types. To avoid complicating our release process with the new 
> checksum type, SHA-256 checksums will be used.
> `electron-updater` refuses to install an update if no checksum is provided.

To fetch the client tool version from clusters, a new RPC to tsh daemon will be 
added:
```grpc
rpc GetAutoUpdate(GetAutoUpdateRequest) returns (GetAutoUpdateResponse);

message GetAutoUpdateRequest {}

message GetAutoUpdateResponse {
 // key is clusterURI.
  map<string, Version> clusters = 1;
}

message Version {
  bool tools_auto_update = 1;
  string tools_version = 2;
}
```
The update logic will resolve the version to install using the following 
precedence:
1. `TELEPORT_TOOLS_VERSION` env var (including `off` value).
2. Version for a cluster from the app state.
3. The latest version from `GetAutoUpdate`.

### Backward compatibility

This auto-update mechanism will be backported to all supported release branches.

However, clusters may specify versions of Teleport Connect that do not support 
auto-updating. 
To disallow updating to such version, Teleport Connect will include a hardcoded 
list of minimum versions supporting auto-updates (e.g., >= 18.1, >= 17.6).  
The list will be created in coordination with the release team.
If a cluster specifies an unsupported version, the app will indicate that no 
update is available.
