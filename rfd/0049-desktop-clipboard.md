---
authors: Zac Bergquist (zac@goteleport.com)
state: draft
---

# RFD 0049 - Desktop Access: Clipboard Support

## What

This RFD defines the high-level goals and architecture for supporing copy and
paste between a Teleport user's workstation and a remote Windows Desktop.

## Details

### Goals

The primary goal of this RFD is to support basic copy paste of small amounts of
text between a Teleport user's workstation and a remote desktop. For example,
copying shell commands, URLs, snippets of log files, etc.

### Non-Goals

The following are all considered out of scope for this RFD:

- large (greater than a few MB) amounts of data
- file transfer (this is a high priority feature but will implemented separately
  in order to keep scope reasonable)

### Overview of RDP

#### Introduction

An RDP connection contains a number of "virtual channels" that provide separate
logical streams of data. Today, we use the _global_ channel for desktop data, and
the _device redirection_ ("RDPDR") channel to implement a smart card emulator.

Clipboard support for RDP is implmemented as a series of messages sent over a
dedicated virtual channel called "CLIPRDR". The details of the protocol are specified
in [MS-RDPECLIP](https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-rdpeclip/fb9b7e0b-6db4-41c2-b83c-f889c1ee7688).

The spec often refers to the two clipboards as the _shared clipboard_ and the
_local clipboard_. The terminology can be confusing, but you can think of the
_shared clipboard_ as the location that is copied from, and the _local
clipboard_ as the location that is pasted to. For example, if a Teleport user
copies text from their workstation and wants to paste it in a remote Windows
Desktop, then the user is the shared clipboard owner and the desktop is the
local clipboard owner. The roles are reversed for the other direction. When text
is copied from the remote desktop and pasted on a user's workstation, the
Windows Desktop is the shared clipboard owner and the Teleport user is the local
clipboard owner.

#### Data Types

RDP supports several different types of clipboard data:

1. Generic: opaque data, not manipulated in any way
2. Palette: a series of RGB-tuples, specially encoded for transport on the wire
3. Metafile: an application-independent vector format for image transfers
4. File List: a list of files to be transferred
5. File Stream: the contents of a file, allowing transfer of specific chunks

Initially, Teleport will only support option 1 - generic clipboard data. Options
2 and 3 are specialized encodings that we don't need at this point, and options
4 and 5 will be useful for file transfer but are out of scope for this RFD.

#### Data Flow and Delayed Rendering

In order to minimize the amount of network bandwidth required, RDP implements a
principle called _delayed rendering._ In short, when data is copied to the
shared clipboard, the shared clipboard owner notifies the other end that there
is new data available. This notification includes the type of the data that was
copied, but not the actual data. At some point later in time, when a paste
operation is attempted on the local clipboard, the local clipboard owner makes a
request to the shared clipboard owner for the data, and the shared clipboard
owner responds with the actual data. This approach ensures that a copy
operation that copies a large amount of data uses very little network bandwidth
if that data is never pasted on the other end of the connection.

```
+-------------------+                 +-----------------+
| shared_clipboard  |                 | local_clipboard |
+-------------------+                 +-----------------+
 -------\ |                                    |
 | Copy |-|                                    |
 |------| |                                    |
          |                                    |
          | Format List PDU                    |
          | (clipboard data not sent)          |
          |----------------------------------->|
          |                                    |
          |                                    | Updates local clipboard IDs
          |                                    |----------------------------
          |                                    |                           |
          |                                    |<---------------------------
          |                                    |
          |           Format List Response PDU |
          |<-----------------------------------|
          |                                    |
          |                                    |
          |                                    |
          |                                    |
          |                                    | --------\
          |                                    |-| Paste |
          |                                    | |-------|
          |                                    |
          |            Format Data Request PDU |
          |                 (request for data) |
          |<-----------------------------------|
          |                                    |
          | Format Data Response PDU           |
          | (clipboard data returned)          |
          |----------------------------------->|
          |                                    |
```

This approach differs from protocols like
[Remote Frame Buffer](https://datatracker.ietf.org/doc/html/rfc6143) (VNC) or
[Teleport Desktop Protocol](./0037-desktop-access-protocol.md), where the
clipboard data is sent along with the notification that there was an update.

### Security

The sharing of a clipboard between two machines requires a high level of trust.
An untrusted peer or misbehaving RDP server can:

- monitor all clipboard activity on the other end of the connection by
  requesting a "paste" any time it receives a notification of a clipboard update
- replace or alter clipboard data before sending it to the remote machine to be
  pasted

These are _features_ of RDP - clipboard sharing wouldn't work without them,
though it highlights the importance of trusting the other end of the connection.

For these reasons, clipboard access will be enabled via the `desktop_clipboard`
role option as specified in [RFD 33](./0033-desktop-access.md). This option will
default to `true`, but the presence of a single role that disables the clipboard
will turn it off. When clipboard access is not enabled, Teleport will not
respond to any clipboard messages received by either the browser or the RDP
server.

### User Experience

The desktop access client runs in a web browser, which provides limited access
to the system clipboard. The
[asynchronous clipboard API](https://developer.mozilla.org/en-US/docs/Web/API/Clipboard_API)
provides read and write access to the system clipboard, and is an obvious
candidate for Teleport. Unfortunately, this API is relatively new and browser
support varies widely. Access to this clipboard API is meant to be protected by
the
[permissions API](https://developer.mozilla.org/en-US/docs/Web/API/Permissions_API).
At the time of this writing:

- The Firefox permissions API does not support the `clipboard-read` or
  `clipboard-write` permissions.
- Safari does not support the permissions API at all, and instead implements
  [its own restrictions](https://developer.mozilla.org/en-US/docs/Web/API/Permissions_API)
  on the clipboard API. Most importantly, the system clipboard can only be read
  from or written to in response to a user gesture.

Until browser support is improved, using the clipboard API with these
restrcitions would require that we maintain a large amount of browser-specific
code. Additionally, even with browser-specific code, some browsers simpy won't
work, as we need:

- access to the clipboard upon receiving a TDP message, which will be decoupled
  from the user gesture
- to detect copy actions that were performed outside the browser

Until then, we will take an approach similar to
[Apache Guacamole](https://guacamole.apache.org/doc/gug/using-guacamole.html#using-the-clipboard). This approach places a native browser textbox
in the UI that acts as a bridge between the local clipboard and the remote
desktop.

- Pasting (or even typing) in this text box will send the data to the remote
  desktop and place it on the desktop's system clipboard.
- Cutting/copying text in the remote desktop will update the text in this
  textbox. It is then up to the user to copy the text from here in order to
  place it on their local clipboard.

This approach allows us to leverage standard browser APIs for detecting changes
to the text box, and isolates the Teleport user's system clipboard from the
remote machine. In other words, Teleport will never write to the system
clipboard, it will only make data from the remote clipboard available, and allow
the user to copy that data to the system clipboard should they decide to.

This approach protects against the issues mentioned in the previous section,
as the remote desktop can only "see" what you put in the textbox and can never
directly access your clipboard. It does require slightly more user interaction
to complete a task though.

- A copy-paste from local workstation to remote desktop requires a single copy
  and two pastes (one paste into the browser textbox and then a second paste
  from inside the remote desktop session).
- A copy-paste from a remote desktop to the local workstation requires two
  copies (one copy from within the remote session, and a second copy from the
  browser textbox) and a single paste.

