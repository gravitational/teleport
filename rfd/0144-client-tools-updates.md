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

This RFD describes how client tools (like `tsh` and `tctl`) can be kept up to
date, either using automatic updates or self-managed updates.

Keeping client tools updated helps with security (fixes for known security
vulnerabilities are pushed to endpoints), bugs (fixes for resolved issues are
pushed to endpoints), and compatibility (users no longer have to learn and
understand [Teleport component
compatibility](https://goteleport.com/docs/upgrading/overview/#component-compatibility)
rules).

## Details

### Summary

Client tools (like `tsh` and `tctl`) will automatically download and install
the version of client tools recommended by the Teleport cluster.

The `cluster_maintenance_config` resource will be updated to include a
`tools_version` field to control the version of client tools advertised by the
cluster. This resource will not be configurable on Teleport Cloud. It will be
managed by the Teleport Cloud team.

If cluster administrators want to opt-out of automatic updates and manage
updates themselves, they will be able to watch the `cluster_maintenance_config`
resource for changes and push out updates to endpoints manually.

Inspiration drawn from https://go.dev/doc/toolchain.

### Implementation

#### Client tools

##### Automatic updates

The first time a user executes `tsh login` with a version of `tsh` that
supports automatic updates, the user will be prompted to enroll in automatic
updates. This choice will be saved in client tools configuration.

```
$ tsh --proxy=proxy.example.com login
Keep client tools like tsh and tctl automatically updated? [YES/no]

[...]
```

```
$ cat ~/.tsh/config/config.yaml
enable_autoupdate: true | false
```

Once enrolled, upon login client tools will check the `tools_version`
advertised by the cluster at `/v1/webapi/ping`. If this version differs from
that of the running binary, the recommended version will be downloaded and
installed.

The original binaries will not be overwritten by automatic updates, instead
per-cluster binaries with permissions `0555` will be stored at
`~/.tsh/bin/proxyName/{tctl,tsh}`. A locking mechanism built around
[syscall.Flock](https://pkg.go.dev/syscall#Flock) on Linux and macOS and
[LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
on Windows to only allow a single writer to update the version of client tools
at a time. This will act as both a cache and allow users to connect to
different Teleport clusters (running different versions of Teleport) without
juggling multiple versions of client tools.

```
$ tree ~/.tsh
~/.tsh
├── bin
│  └── proxy.example.com
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

After downloading, client tools will re-execute the downloaded version.  The
child process will inherit all environment variables and flags.
`TELEPORT_TOOLS_VERSION=off` will be added during re-execution to prevent
infinite loops.

An environment variable `TELEPORT_TOOLS_VERSION` will be introduced that can be
`X.Y.Z` (use specific version) or `off` (do not try to update). This
environment variable can be used as a emergency workaround for a known issue,
pinning to a specific version in CI/CD, or for debugging.

Automatic updates will not be used if `tctl` is connecting to the Auth Service
over localhost.

##### Errors and warnings

If the user does not enroll in automatic updates and the cluster administrator
does not self-manage client tools updates as outlined in [Self-managed client
tools updates](#self-managed-client-tools-updates), a series of warnings and
errors with increasing urgency will be shown to the user.

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

Some functionality will not work. Update Teleport to vX.Y.Z from
https://goteleport.com/download or your system package manager.

Run "tsh autoupdate enroll" to enroll in automatic updates and keep client
tools like tsh and tctl automatically updated.

Use the "--skip-version-check" flag to bypass this check and attempt to connect
to this cluster.
```

#### Self-managed client tools updates

Cluster administrators that want to self-manage client tools updates will be
able to watch for changes to client tools versions which can then be used to
trigger other integrations (using MDM software like Jamf) to update the
installed version of client tools on endpoints.

```
$ tctl autoupdate watch
{"tools_version": "1.0.0"}
{"tools_version": "1.0.1"}
{"tools_version": "2.0.0"}

[...]
```

Cluster administrators who prefer to use the Teleport API can create a watcher
on the `types.ClusterMaintenanceConfig` resource and trigger other integrations
on changes to the `tools_version` field.

```
// Create a watcher to monitor for changes to client tools version.
watch, err := p.TeleportClient.NewWatcher(ctx, types.Watch{
    Kinds: []types.WatchKind{
        types.WatchKind{Kind: types.ClusterMaintenanceConfig},
    },
})
if err != nil {
    return trace.Wrap(err)
}
defer watch.Close()

// Loop forever watching for client tools version changes.
for {
    select {
    case e := <-watch.Events():
        if e.Resource == nil {
            continue
        }
        resource, ok := e.Resource.(types.ClusterMaintenanceConfig)
        if !ok {
            continue
        }

        // Process the version change by notifying downstream tooling (like
        // Jamf).
        if err := handle(resource); err != nil {
            log.Printf("Failed to handle version change: %v", err)
        }
    case <-watch.Done():
        fmt.Println("The watcher job is finished.")
        return nil
    }
}
```

##### Cluster configuration

> [!NOTE]
> This resource will not be configurable on Teleport Cloud. It will be managed
> by the Teleport Cloud team.

The `types.ClusterMaintenanceConfig` resource will be updated to include a
`tools_version` field. This field can be updated directly from the resource or
using `tctl`.

```
kind: cluster_maintenance_config
spec:
  # tools_version is the version of client tools the proxy will advertise. Can
  # be auto (match the version of the proxy), off (don't advertise any version),
  # or an exact semver formatted version.
  tools_version: auto | off | X.Y.Z

  [...]
```

```
$ tctl autoupdate update --set-tools-version=1.0.1
Automatic updates configuration has been updated.
```

Three modes will be supported: `auto`, `X.Y.Z` (exact version), and `off`.

* `auto`: Cluster will advertise the version of the proxy as the version of
  client tools.
* `X.Y.Z`: Cluster will advertise a specific version of client tools.
* `off`: Cluster will not advertise any version, clients will not try to
  update.

All new self-hosted clusters will be opted into `auto`. This way, by default
even self-hosted customers will have automatic updates available.

Existing clusters will be opted into `off` to not introduce unexpected
behavior.

The `tools_version` field will then be available from the unauthenticated
endpoint `/v1/webapi/ping`.

```
curl https://proxy.example.com/v1/webapi/ping | jq .
{
    "tools_version": "X.Y.Z",

    [...]
}
```

### Costs

Some additional costs will be incurred as Teleport downloads will increase in
frequency.

### Out of scope

How Cloud will push changes to `cluster_maintenance_config` is out of scope
for this RFD and will be handled by a separate Cloud specific RFD.

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
