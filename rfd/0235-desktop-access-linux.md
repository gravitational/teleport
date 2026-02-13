---
authors: Przemko Robakowski (przemko.robakowski@goteleport.com)
state: draft
---

# RFD 235 - Desktop Access - Linux

## Required Approvals

- Engineering: @zmb3 @rosstimothy
- Product: @klizhentas

## What

RFD 33 defines the high-level goals and architecture for Teleport Desktop
Access.

This RFD specifies how Teleport Desktop Access integrates with Linux hosts.

## Why

We currently support Windows Desktop Access using RDP protocol, but there's no way to connect
to desktop environments on Linux boxes. Current RDP servers, like [xrdp](https://www.xrdp.org/),
usually don't work with IronRDP library and even when they do, they don't support smart card
authentication which we use for providing passwordless access. We would like to close this gap
by adding Teleport-native Linux Desktop Access to minimize the amount of configuration users have
to do outside Teleport and to bring the Linux desktop experience on par with Windows.

## Details

### Architecture

Linux Desktop Access is implemented using `linux_desktop_service`, which
translates the Teleport desktop protocol (TDPB defined in RFD 232) into the X11 protocol.

Linux Desktop Service will run in agent mode; no agent-less mode will be supported.

In the initial release, the web UI will be supported; Connect support will be added later.

```
+--------+                 +---------+
| web UI |                 | Connect |
+--------+                 +---------+
    ^                          ^
    | TDPB over WebSocket      | TDPB over gRPC
    |                          |
    |       +--------+         |
    +-----> | proxy  | <-------+
            +--------+
                ^
                | TDPB over mTLS over reverse tunnel
                |
+---------------|------------------+
|               v                  |
| +-------------------------+      |
| |  linux_desktop_service  |      |
| +-------------------------+      |
|     ^                 |          |
|     | X11             | starts   |
|     v                 v          |
| +--------+  X11  +-------------+ |
| |  Xvfb  |<----->|   Desktop   | |
| +--------+       | Environment | |
|                  + ------------+ |
|                                  |
|                       Linux host |
+----------------------------------+
```

### UX

#### Setup/discovery

Setup will require creating a new node or updating an existing one with the new `linux_desktop` role and
`linux_desktop_service`
enabled (it will be disabled by default). Existing nodes will require a new token that has the `linux_desktop` role
included.

To help with onboarding, there will be a new tile in the `Enroll a New Resource` view for Linux desktops.
For the first release, it will lead to documentation. A fully fledged guided installation, similar to enrolling SSH
nodes,
will be added later.

#### User logging in

Linux Desktop Service will create a `LinuxDesktop` resource with its UUID as its name. These resources will be presented
in the UI as one of the unified resources, the same way we present Windows desktops. The service will use the inventory
stream to provide efficient heartbeats.

After the user selects a username to log in, the UI will redirect to
`/web/cluster/:cluster/linux_desktops/:uuid/:login`.
The user will be presented with a list of available Xsessions (i.e. desktop environments). After selecting one of the
items,
the user will be redirected to `/web/cluster/:cluster/linux_desktops/:uuid/:login/:xsession`. This will enable the user
to
create bookmarks both to session selection screen and to specific Xsession.

If there's only one entry in `/usr/share/xsessions`, the selection screen will be skipped.

The started session will reuse visual components used for Windows desktop sessions to make the experience consistent.

### Xsessions/desktop environments

The list of Xsessions available on the target machine will be obtained by listing files in `/usr/share/xsessions`.
This is the same mechanism that is used by different display managers, like GDM and LightDM, to populate their
list of Xsessions.

On connection, Linux Desktop Service will read the selected desktop entry file to get the command to execute stored in
the `Exec=`
section. It will then find a free display number, start `Xvfb` with the requested screen size, and run the command
obtained
earlier as the user requested in the UI. It will use the X11 protocol to interact with `Xvfb`.

Only X11 environments will be supported, there will be no support for Wayland. Tested desktop environments should
include at least Xfce, KDE Plasma and GNOME 48 (the last version that supports X11).

Desktop environment will be started using a similar mechanism to starting a shell for SSH access using `teleport exec`
command. Support will be added for host user creation, PAM integration, SELinux, user accounting, and audit
context propagation.

### Authentication

New fields `linux_desktop_logins` and `linux_desktop_labels` will be added to the role resource to support RBAC. They
will
function the same way `windows_desktop_logins` and `windows_desktop_labels` work for Windows desktops. Logins will be
populated from leaf clusters using the same mechanism that Windows desktops use.

On connection, Linux Desktop Service will verify the user using an mTLS certificate. No other authentication is
required.

Unix socket created by `Xvfb` will be secured using `Xauthority` file generated by Teleport and shared between `Xvfb`,
the desktop environment, and Linux Desktop Service. `Xvfb` will not create any TCP sockets.

### Frame encoding

Modified regions will be discovered using [DAMAGE](https://www.x.org/archive/X11R7.5/doc/damageproto/damageproto.txt)
extension. Each region will be encoded
using [QOI](https://en.wikipedia.org/wiki/QOI_(image_format)) + [zstd](https://github.com/facebook/zstd)
combination using a Rust encoder to maximize performance. This encoding is supported by IronRDP in the `qoiz` feature,
so we can leverage their decoder in the UI.

### Screen resize

Screen will be resized
using [XRANDR](https://gitlab.freedesktop.org/xorg/proto/xorgproto/-/raw/master/randrproto.txt?ref_type=heads)
extension. `Xvfb` will be started using the maximum supported screen size (8192x8192) and immediately resized down to
the requested size. This is needed because `Xvfb` won't allow resizes to a size bigger than the initial one.

### Mouse and keyboard input

Mouse and keyboard events will be translated
into [XTEST](https://www.x.org/releases/X11R7.7-RC1/doc/xextproto/xtest.html)
extension calls. It supports all currently supported events.

Information about pointer changes will be obtained
using [XFIXES](https://cgit.freedesktop.org/xorg/proto/fixesproto/plain/fixesproto.txt)
extension: `CursorNotify` and `GetCursorImage`.

### Clipboard sharing

Clipboard will be shared by monitoring and managing `CLIPBOARD` selection. Data will be copied in both directions as
soon as it is available, the same way we do it for Windows desktops. The message flow between UI and Linux Desktop
Service and permission model will be the same as defined in RFD 49 for Windows Desktop.

### Directory sharing

FUSE filesystem will be created using [go-fuse](https://github.com/hanwen/go-fuse). It will be mounted in the user's
home
directory, and it will redirect and translate all requests to TDPB messages that will be handled by the UI.

If FUSE is not available on the Linux host, directory sharing will be disabled and a warning will be emitted.

### Concurrent sessions

Each login will start a separate session. This is in contrast to how Windows Desktop Access works, as it will reuse
an existing session when the current user logs out, but it's similar to how SSH sessions work.

### Session recordings

Sessions will be recorded in the same way as we do for Windows desktops. Playback will reuse most of the code as well.
Both node and proxy modes will record in Linux Desktop Service, session recording will be configured by existing
`record_session.desktop`.

Encrypted recordings will also be supported.

### Locking and client idle timeout, connection limits

`srv.StartMonitor` will be used for tracking client activity and lock status. New lock target `LinuxDesktop` will be
added.

### MFA

The per-session MFA mechanism currently used by Windows desktops will be reused for Linux desktops.

### Configuration

New `teleport.yaml` section for `linux_desktop_service`:

```yaml
linux_desktop_service:
  enabled: yes # default false
  listen_addr: 0.0.0.0:3029
  public_addr: linux.desktop.example.com:3080
  # optional, xsessions will provide regexes for filtering available sessions to present in UI
  xsessions:
    included: "^Xfce.*" # defaults to ^.*$
    excluded: ".*restricted" #defaults to no excludes
  # optional static labels
  labels:
    environment: dev
  # optional dynamic labels using periodic command
  commands:
    - name: arch
      command: [ uname, -p ]
      period: 1h0m0s
```

For an entry to be included and shown to the user, it has to match the `included` filter and must not be excluded by
`excluded`.

### CLI changes

`tctl desktops ls` will no longer be an alias for `tctl windows_desktops ls`. It will be modified to show both Windows
and
Linux desktops with additional column showing the type of the desktop.

`tctl linux_desktops ls` will be added that will show only Linux desktops.

`tctl get/rm linux_desktop/:uuid` will be added.

New flag `--linux-desktop` will be added to `tctl lock`. `tctl lock --server-id` will also support Linux desktops.

`tctl token add` will support new system role `linux_desktop`.

No changes to `tsh` are needed.

### Events

For audit log purposes, we will add new `linux.desktop.session.start` and `linux.desktop.session.end` events.
Other events will be shared with Windows desktops. That includes `desktop.clipboard.*`, `desktop.directory.*`,
and `client.disconnect`.

For usage reporting, a new resource kind `RESOURCE_KIND_LINUX_DESKTOP` will be added to prehog and
[TPR query](https://github.com/gravitational/cloud/blob/26eaac92bd0e297dcb127797b2e8d95706e34c5b/jobs/exporter/athena.go#L182-L186)
will be updated to count instances of this new resource. Events will be sent using `UsageReporter.AnonymizeAndSubmit`.

### Feature matrix

Here's a summary of planned features:

| Feature                                                                                  | Planned | Initial release | Details                                                                                                                                        |
|------------------------------------------------------------------------------------------|---------|-----------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| [Enroll a New Resource](#setupdiscovery)                                                 | ✅       | ✅/❌             | for the first release only link to the documentation will be provided                                                                          | 
| Web UI                                                                                   | ✅       | ✅               | feature parity with Windows desktops, reusing as much code as possible                                                                         |
| Connect                                                                                  | ✅       | ❌               | feature parity with Windows desktops, reusing as much code as possible                                                                         |
| [Session recordings](#session-recordings)                                                | ✅       | ✅               | the same mechanism as Windows desktops, both node and proxy modes will record at Linux Desktop Service, configured by `record_session.desktop` |
| Enhanced session recordings                                                              | ❌       | ❌               |                                                                                                                                                |
| [Per-session MFA](#MFA)                                                                  | ✅       | ✅               | the same mechanism as Windows desktops (messages sent through TDPB)                                                                            |
| [New fields in role (YAML)](#authentication)                                             | ✅       | ✅               | `linux_desktop_labels`, `linux_desktop_labels_expression`, `linux_desktop_logins` (with support for expressions)                               |
| Roles UI changes                                                                         | ✅       | ❌               |                                                                                                                                                |                              
| [Clipboard sharing](#clipboard-sharing)                                                  | ✅       | ✅               | managing `CLIPBOARD` selection                                                                                                                 |
| [Directory sharing](#directory-sharing)                                                  | ✅       | ❌               | custom FUSE FS using [go-fuse](https://github.com/hanwen/go-fuse)                                                                              |
| [Events and telemetry](#events)                                                          | ✅       | ✅               |                                                                                                                                                |
| [Static labels](#configuration)                                                          | ✅       | ✅               |                                                                                                                                                |
| [Dynamic labels](#configuration)                                                         | ✅       | ❌               | `commands` field in service configuration, will work the same as for SSH                                                                       |
| Resource-based labels (`server_info`)                                                    | ❌       | ❌               |                                                                                                                                                |
| Automatic user creation                                                                  | ✅       | ❌               | reusing `create_host_user_mode`, `host_groups` and `host_sudoers`                                                                              |
| Static user creation                                                                     | ✅       | ❌               | new `linux_desktop_labels` and `linux_desktop_labels_expressions` matcher fields, reconciler will be shared and will use instance connector    |
| SELinux support                                                                          | ✅       | ❌               | by passing `ExecCommand.SetSELinuxContext=true` during `teleport exec` reexec                                                                  |
| PAM support                                                                              | ✅       | ❌               | by passing `ExecCommand.PAMConfig` during `teleport exec` reexec                                                                               |
| User accounting                                                                          | ✅       | ❌               | by passing `ExecCommand.UaccMetadata` during `teleport exec` reexec                                                                            |
| auditd context propagation                                                               | ✅       | ❌               | by passing `ExecCommand.Login/Username/ClientAddress` during `teleport exec` reexec                                                            |
| [tctl support](#cli-changes)                                                             | ✅       | ✅               |                                                                                                                                                |
| [Locks, client idle timeout, limits](#locking-and-client-idle-timeout-connection-limits) | ✅       | ✅               | using `srv.StartMonitor`                                                                                                                       |
| Lock UI                                                                                  | ✅       | ❌               |                                                                                                                                                | 
