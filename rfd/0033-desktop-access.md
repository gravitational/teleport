---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: implemented
---

# RFD 33 - Desktop Access

## What

Teleport Desktop Access allows users to log into remote desktop GUI environments.

This RFD describes the generic Desktop Access model and client that can be used
with any target OS. Separate RFDs will describe the integration with specific
OSs (like Windows, MacOS, Linux, etc).

## Why

Most modern infrastructure is managed via programmable textual interfaces, like
ssh, kubernetes or SQL, or HTTP-based web apps. However, there's a significant
number of IT systems that only run as a GUI, usually on Windows. Teleport
Desktop Access aims to support accessing those systems, with the usual added
benefits of Teleport like audit logging, SSO and RBAC.

In addition to Windows-only IT systems, Desktop Access can be used to access
remote development machines, without the limitations of terminal-only SSH.

## Details

### Architecture

Desktop Access works similarly to other existing Teleport services: an
authenticated client application talks to the Teleport Proxy, which proxies the
traffic to a backend service. Along the way, services enforce authorization and
record the session.

In Desktop Access, the client is a web application, bundled as part of the
Teleport Web UI (similar to the web SSH client). The backend service (e.g.
`windows_desktop_service`) is OS-specific and can represent multiple desktop
hosts.

```
+--------+
| web UI |
+--------+
    ^
    | desktop protocol over websocket
    v
+--------+
| proxy  |
+--------+
    ^
    | desktop protocol over mTLS
    v
+---------+
| backend |
| service |<----------------+
+---------+                 |
   ^                        |
   | OS protocol (e.g. RDP) |
   v                        v
+---------+              +---------+
| desktop |              | desktop |
| host 1  |              | host 2  |
+---------+              +---------+
```

From web UI to the backend service, the underlying wire protocol is a custom
protocol, which we'll call "desktop protocol". Between the backend service and
the actual desktop hosts, the wire protocol is a standard protocol for the
target OS (like RDP or X11), which we'll call "OS protocol".

### Protocol

The wire protocol used between the Teleport UI and backend service is a custom
binary protocol. It is described in more detail in RFD 37.

We are not using any standard remote desktop protocol, like
RDP/X11/VNC/Guacamole for two reasons:
- interoperability with 3rd party software is not a requirement
- avoid legacy baggage and complexity of standard protocols, focus on
  simplicity and performance

We are also not using generic encoding protocols, like gRPC, to keep
client-side parsing simple and reduce the JS bloat.

### Client

The client is a web app built into the Teleport Web UI. It's a JavaScript/TypeScript application that:
- picks one of the allowed OS usernames for the logged-in Teleport user based
  on their roles
- creates a websocket connection
- handles the incoming desktop protocol messages and renders them onto an HTML
  canvas
- captures user input, encodes it into desktop protocol messages and sends them
  over the websocket

TODO: wireframes?

### Configuration

There are no additional configuration options for Desktop Access in the web UI
or the proxy. The Desktop Access client is part of the web UI and uses the same
web port on the proxy for connecting a websocket.

All relevant configuration to enable the feature will live in OS-specific
backend services, described in their own RFDs (like RFD 34 for Windows).

### Authorization

Role definitions for RBAC have a few new fields for Desktop Access:

- `${OS}_desktop_logins` - list of desktop login names allowed/denied for
  desktop hosts with a given OS
- `${OS}_desktop_labels` - list of labels to match the target desktop hosts
  against
- `desktop_clipboard` - option for allowing copy/paste to/from the desktop
  host; default is `true`
- `record_desktop_session` - option for enabling/disabling recording of
  sessions to target desktop hosts; for very long and not security-sensitive
  sessions, recording can be disabled to save on storage costs; default is
  `true`

Example:
```yaml
kind: role
version: v3
metadata:
  name: windows_db_admin
spec:
  options:
    desktop_clipboard: true
    record_desktop_session: true
  allow:
    # Allow windows sessions as user DBAdmin in test and staging environments.
    windows_desktop_logins: ['DBAdmin']
    windows_desktop_labels:
      'env': ['staging', 'test']
  deny:
    # Prevent logins to prod environment or as Administrator.
    windows_desktop_logins: ['Administrator']
    windows_desktop_labels:
      'env': ['prod']
```

Like with SSH access, the `windows_desktop_logins` field will support a couple
special variables. An `{{internal.windows_logins}}` variable for local users
will map to any logins that are supplied when the user is created with
`tctl users add alice --windows-logins=Administrator,DBUser`. For SSO users, the
`{{external.attribute}}` variable allows access to SAML assertions or OIDC
claims.

### Storage schema

There are 2 new kinds of objects stored in Teleport: `${OS}DesktopService` and
`${OS}Desktop`. For example `WindowsDesktopService` and `WindowsDesktop`. The
former tracks registration of an `${OS}_desktop_service` instance of Teleport.
The latter tracks an individual target desktop host for logging into.

OS-specific backend services are responsible for creating the `${OS}Desktop`
objects, including their RBAC labels.

The schema for these objects is defined in their respective RFDs.

### Session recording

Video output in our desktop protocol uses bitmaps and not a standard video
encoding format. We will record the video output data as-is, with timestamps
added to each protocol message (or perhaps
[APNG](https://en.wikipedia.org/wiki/APNG)).

To playback session recordings, we will replay them in the same canvas-based JS
code as the original session used live.

#### Recording export

Some users might want to export the recording for sharing. To allow playback
outside of the Teleport web UI, the exported recording should use a standard
video format like MP4 or WebM.

To convert our storage format to the video format, we will use [wasm-based
ffmpeg](https://github.com/ffmpegwasm/ffmpeg.wasm) in the browser. Conversion
will happen on-demand during export and Teleport will not store the converted
video file. The web UI code might need to first convert our internal storage
format into one of the simpler formats that `ffmpeg` accepts as input.
