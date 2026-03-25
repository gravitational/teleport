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
date, either using managed updates or self-managed updates.

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

Enrollment in managed updates for client tools will be controlled at the
cluster level. By default, all Cloud clusters will be opted into managed
updates for client tools. Cluster administrators using MDM software like Jamf
will be able opt-out manually manage updates.

Self-hosted clusters will be opted out, but have the option to use the same
managed update mechanism.

Inspiration drawn from https://go.dev/doc/toolchain.

### Implementation

#### Client tools

##### Managed updates

When `tsh login` is executed, client tools will check `/v1/webapi/find` to
determine if managed updates are enabled. If the cluster's required version
differs from the current binary, client tools will download and re-execute
using the version required by the cluster. This means that Managed Updates 
support major version differences, as the login command is intercepted 
to check the version first.

To enable managed updates for different set of command such as `tsh ssh` or
`tsh proxy ssh` to verify the version set in cluster during login to ssh
you need to set `TELEPORT_TOOLS_CHECK_UPDATE=t` environment variable.

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
│  ├── .config.json
│  ├── .lock
│  ├── 7de24a1e-8141-4fc8-9a1f-cd4665afa338-update-pkg-v2
│  │  ├── tctl
│  │  └── tsh
│  └── d00ffd3d-700d-47b0-bd65-94506f1362e2-update-pkg-v2
│     ├── tctl
│     └── tsh
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

The configuration file structure should include a map of known hosts and a list of installed tools, ordered by most recently used.

- The map of known hosts stores data received from the cluster configuration, such as version and mode. This version will be used for re-execution unless it is overridden by the `TELEPORT_TOOLS_VERSION` environment variable.
- The list of installed tools has a limited size (at least 3 installations). When a tool is no longer referenced in the configuration, it will be removed during cleanup.

When a specific version of a tool is requested for re-execution, the path is determined from the configuration as: `$TELEPORT_TOOLS_DIR/package/path[tool_name]`

```json
{
  "configs": {
    "proxy.example.com": {
      "version": "17.5.1",
      "disabled": false
    }
  },
  "max_tools": 3,
  "tools": [
    {
      "version": "17.5.1",
      "path": {"tctl": "tctl", "tsh": "tsh"},
      "package": "d00ffd3d-700d-47b0-bd65-94506f1362e2-update-pkg-v2"
    },
    {
      "version": "17.5.2",
      "path": {"tctl": "tctl", "tsh": "tsh"},
      "package": "7de24a1e-8141-4fc8-9a1f-cd4665afa338-update-pkg-v2"
    }
  ]
}
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
on the operating system, platform, and edition. The CDN base URL can be overridden 
by the `TELEPORT_CDN_BASE_URL` environment variable and required to be set for OSS build.
Where edition must be identified by the original client tools binary, URL pattern:
`https://cdn.teleport.dev/teleport-{, ent-}v15.3.0-{linux, darwin, windows}-{amd64,arm64,arm,386}-{fips-}bin.tar.gz`


An environment variable `TELEPORT_TOOLS_VERSION` will be introduced that can be
`X.Y.Z` (use specific semver version) or `off` (do not update). This environment 
variable can be used for manual updates, pinning to a specific version in CI/CD, 
or for debugging.

By setting `TELEPORT_TOOLS_VERSION=X.Y.Z` during `tsh login`, the advertised version 
from the cluster will be ignored, as well as any disabled mode. This specified 
version will be recorded in the client tools configuration file and associated 
with the cluster. When the profile for this cluster is active, the specified 
version will be used for all commands.

During re-execution, child process will inherit all environment variables and
flags. `TELEPORT_TOOLS_VERSION=off` will be added during re-execution to
prevent infinite loops.

When `tctl` is used to connect to Auth Service running on the same host over
`localhost`, `tctl` assumes a special administrator role that can perform all
operations on a cluster. In this situation the expectation is for the version
of `tctl` and `teleport` to match so managed updates will not be used.


##### Errors and warnings

If cluster administrator has chosen not to enroll client tools in managed
updates and does not self-manage client tools updates as outlined in
[Self-managed client tools updates](#self-managed-client-tools-updates), a
series of warnings and errors with increasing urgency will be shown to the
user.

If the version of client tools is within the same major version as advertised
by the cluster, a warning will be shown to urge the user to enroll in managed
updates. Warnings will not prevent the user from using client tools that are
slightly out of date.

```
$ tsh login --proxy=proxy.example.com
WARNING: Client tools are out of date, update to vX.Y.Z.

Update Teleport to vX.Y.Z from https://goteleport.com/download or your system
package manager.

Enroll in managed updates to keep client tools like tsh and tctl
automatically updated. https://goteleport.com/docs/upgrading/client-tools-autoupdate/

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

Enroll in managed updates to keep client tools like tsh and tctl
automatically updated. https://goteleport.com/docs/upgrading/client-tools-autoupdate/
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

Enroll in managed updates to keep client tools like tsh and tctl
automatically updated. https://goteleport.com/docs/upgrading/client-tools-autoupdate/

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

Enrollment of clients in managed updates will be enforced at the cluster
level.

The `autoupdate_config` resource will be updated to allow cluster
administrators to turn client tools managed updates `on` or `off`.
A `autoupdate_version` resource will be added to allow cluster administrators
to manage the version of tools pushed to clients.

> [!NOTE]
> Client tools configuration is broken into two resources to [prevent
> updates](https://github.com/gravitational/teleport/blob/master/lib/modules/modules.go#L332-L355)
> to `autoupdate_version` on Cloud.
>
> While Cloud customers will be able to use `autoupdate_config` to
> turn client tools managed updates `off` and self-manage updates, they will
> not be able to control the version of client tools in `autoupdate_version`.
> That will continue to be managed by the Teleport Cloud team.

Both resources can either be updated directly or by using `tctl` helper
functions.

```yaml
kind: autoupdate_config
spec:
  tools:
    # tools mode allows to enable client tools updates or disable at the
    # cluster level. Disable client tools managed updates only if self-managed
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

Managed updates for Teleport Connect are out of scope for this RFD as it uses
a different install/update mechanism. For now it will call `tsh` with
`TELEPORT_TOOLS_VERSION=off` until managed updates support can be added to
Connect.

### Security

The initial version of managed updates will rely on TLS to establish
connection authenticity to the Teleport download server. The authenticity of
assets served from the download server is out of scope for this RFD. Cluster
administrators concerned with the authenticity of assets served from the
download server can use self-managed updates with system package managers which
are signed.

Phase 2 will use The Upgrade Framework (TUF) to implement secure updates.

## Teleport Connect automatic updates *(added on 2025-07-10 by @gzdunek)*

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

It's worth mentioning that applying updates on Windows and Linux may require  
additional user interaction, since the app is installed per-machine there.
After the user clicks 'Restart' on manually closes the app, a system pop-up
will appear, asking for an admin password. To make it less surprising to users, 
the UI for Windows/Linux will include a message: "you may be prompted for 
admin password to install the update".

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

This can create a feedback loop of constant updates and repeated restarts.

Proposed Solution:
* The app will read the client tool versions and minimum client version from all 
connected clusters, and will try to find the most compatible version (fulfilling 
our compatibility promise).
* If no compatible version is found, the auto-update mechanism will stop working
until the user selects a cluster managing updates in the detailed view.
The selected cluster will be stored in the app state and cleared when the user 
logs out from that cluster.
The UI will look as follows:
> App updates are disabled
> 
> Your clusters require incompatible client versions.
> To enable app updates, select which cluster should manage them.
> 
> [ ] (disabled checkbox) Use the most compatible version from your clusters
> 
> Or select a cluster to manage updates:
> 
> 1. teleport-18.asteroid.earth
>
>    18.0.3 client, only compatible with this cluster.
> 
> 2. teleport-17.asteroid.earth
>
>    17.3.3 client, compatible with teleport-17.asteroid.earth, teleport-18.asteroid.earth.
>
> 3. teleport-16.asteroid.earth
>
>    16.3.3 client, compatible with teleport-16.asteroid.earth, teleport-17.asteroid.earth.
* The auto-update widget will always show an info/warning alert if the app version
does not match the target cluster client tools version.

In a multi-cluster setup, users will always have the ability to choose which 
cluster manages updates. Users have different needs, and we don't have enough
data to reliably solve the multi-cluster version problem in a useful way.
To help the users make decision on which cluster to choose, we will show 
compatibility information for clusters.

### Implementation

A custom updater function will be implemented using electron-updater's `Provider` 
interface. This function will return version metadata including:
* Version number
* Download URL
* SHA-512 checksum

>️ Note: The release process will generate both SHA-256 and SHA-512 checksums.

To fetch the client tool version from clusters, a new RPC to tsh daemon will be 
added:
```grpc
rpc GetAutoUpdate(GetAutoUpdateRequest) returns (GetAutoUpdateResponse);

message GetAutoUpdateRequest {}

message GetAutoUpdateResponse {
  repeated Version versions = 1;
}

message Version {
  string root_cluster_uri = 1;
  bool tools_auto_update = 2;
  string tools_version = 3;
  string min_tools_version = 4;
}
```
The update logic will resolve the version to install using the following 
precedence:
1. `TELEPORT_TOOLS_VERSION` env var, if defined.
2. `tools_version` from the cluster manually selected to manage updates, if selected.
3. Most compatible version, if can be found.
4. If there's no version at this point, stop auto updates.
They will resume working if the user either selects a cluster that manages updates, 
logs out of incompatible clusters, or if the cluster versions become compatible again.

### Backward compatibility

This auto-update mechanism will be backported to all supported release branches.

However, clusters may specify versions of Teleport Connect that do not support 
auto-updating. 
To disallow updating to such version, Teleport Connect will include a hardcoded 
list of minimum versions supporting auto-updates (e.g., >= 18.1, >= 17.6).  
The list will be created in coordination with the release team.
If a cluster specifies an unsupported version, the app will indicate that no 
update is available.
