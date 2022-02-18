---
authors: Zac Bergquist (zac@goteleport.com), Isaiah Becker-Mayer (isaiah@goteleport.com)
state: implemented
---

# RFD 0048 - Desktop Access: Session Recording

## What

RFD 47 defines the high-level goals and architecture for the recording of
Desktop Access sessions.

## Details

### Goals

This feature with several goals in mind:

- performance - session playback should as be at least as fluid as the live
  session
- consistency - the user experience of viewing a desktop session should feel
  familiar to those who have experience browsing and playing back SSH sessions
- extendability - new capabilities can be added in the future:
  - playback speed
  - export to video file for viewing outside browser
  - identifying key "events" in the playback (file transfer, clipboard action,
    etc.)

### Non-Goals

While we will not rule these features out in a future update, the following
items are not a high priority for the initial implementation of session
recording:

- Video Export. In the initial implementation, we will only support viewing
  desktop sessions in the browser.
- File Size Optimization. TDP sends a lot of PNG frames over the wire. We expect
  that session recordings may consume large amounts of disk space.

### Prior Art

The design and implementation of session recording for desktop access is
inspired by the recording features of SSH sessions, described in
[RFD 0002](./0002-streaming.md).

To summarize RFD 0002, session recordings are an ordered set of structured audit
events generated from a protobuf spec and written to persistent storage.

#### Recording

The key component for session recording is the
[`AuditWriter`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L147-L166),
which is an implementation of `io.Writer` that packages up a `[]byte` into an
audit event protofbuf (a `SessionPrint` event in the case of SSH). These events
are then written to a `Stream` which is responsible for batching events and
ultimately writing them to persistent storage (a file or S3-compatible storage).

In asynchronous recording mode, we also leverage a
[`TeeStreamer`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L1074)
which is responsible for filtering out `SessionPrint` events that should not be
written to Teleport's main audit log.

Additionally, the web UI currently searches for `session.end` events in order to
populate the session recordings page. We will add a new audit event for
`desktop.session.end` that captures similar information:

```protobuf
// WindowsDesktopSessionEnd is emitted when a user ends a Windows desktop session.
message WindowsDesktopSessionEnd {
    Metadata Metadata = 1;
    UserMetadata User = 2;
    SessionMetadata Session = 3;

    // WindowsDesktopService is the name of the service proxying the RDP session.
    string WindowsDesktopService = 4;
    // DesktopAddr is the address of the desktop being accessed.
    string DesktopAddr = 5;
    // Domain is the Active Directory domain of the desktop being accessed.
    string Domain = 6;
    // WindowsUser is the Windows username used to connect.
    string WindowsUser = 7;
    // DesktopLabels are the labels on the desktop resource.
    map<string, string> DesktopLabels = 8;
    // StartTime is the timestamp at which the session began.
    google.protobuf.Timestamp StartTime = 9;
    // EndTime is the timestamp at which the session ended.
    google.protobuf.Timestamp EndTime = 10;
    // DesktopName is the name of the desktop resource.
    string DesktopName = 11;
    // Recorded is true if the session was recorded, false otherwise.
    bool Recorded = 12;
    // Participants is a list of participants in the session.
    repeated string Participants = 13;
 }
```

Notably:

- Start and end times allow the UI to display the length of the session
- Recorded indicates whether the session should appear in the recordings list
- Participants allows for compatibility with RBAC for sessions

#### Playback

In order to maintain compatibility with older versions of Teleport, SSH session
playback converts this stream of `SessionPrint` events into a legacy format
consisting of two files:

- an "events" file containing timing data
- a "chunks" file containing the raw PTY data

For SSH, this format has the nice property of the "chunks" file being useful on
its own, as the raw data can be viewed and searched independent of the timing
data (outside of the Teleport player). This separation of timing data is not
necessary for desktop sessions, as the raw data cannot be interpreted outside of
the Teleport player, so this conversion step will not be necessary for desktop
sessions.

Teleport's existing API also exposes an endpoint for streaming audit events associated
with a particular session.

```
func StreamSessionEvents(
    ctx context.Context,
    sessionID string,
    startIndex int64) (chan events.AuditEvent chan error)
```

On the backend, the log is downloaded from external storage, the events are
parsed via an `AuditReader`, and finally written to the events channel. There is
no attention paid to timing at this step. Events are written to the channel as
soon as they are available and the process is naturally throttled by how fast
the consumer reads from the channel.

### Overview

During a live session, desktop images are sent to the browser via a series of
PNG frames encoded in a TDP message (message #2, "PNG frame"). At a high level,
the session recording feature needs to archive these messages in the audit log
so that they can be played back at a later time.

During playback, the majority of the frontend code works exactly as it does
during a live session - the PNGs are rendered on an HTML canvas.

The major differences in playback mode are:

- no user-input is captured or sent across the wire (mouse clicks, scroll wheel, etc)
- playback features such as play/pause (at a minimum), and seek are desired

#### Compatibility

The session recording feature further increases the importance of
backwards-compatible changes to the TDP spec, as newer versions of Teleport must
be able to play back recordings captured by older versions.

This means we must favor adding new messages over modifying existing messages,
and retain support for deprecated messages in order to keep recordings valid.

#### Recording

Teleport session recordings are stored as an ordered sequence of gzipped,
protobuf-encoded audit events. Desktop session recording will work in the same
way. The maximum length of a protobuf-encoded message is 64K, so in the event a
message exceeds the maximum length, it will be broken up into two separate
events.

In order to support a new type of session recording, a new audit event will be
defined for capturing desktop session data. The `DesktopRecordingEvent` will
implement the `AuditEvent` interface and is analagous to the `PrintEvent` used
for SSH recordings.

```protobuf
// DesktopRecordingEvent happens when a Teleport Desktop Protocol message
// is captured during a Desktop Access Session.
message DesktopRecordingEvent {
    // Metadata is a common event metadata
    Metadata Metadata = 1
        [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

    // Message is the encoded TDP message. It is not marshaled to JSON format
    bytes Message = 2 [ (gogoproto.nullable) = true, (gogoproto.jsontag) = "-" ];

    // DelayMilliseconds is the delay in milliseconds from the start of the session
    int64 DelayMilliseconds = 3 [ (gogoproto.jsontag) = "ms" ];
}
```

The `Message` field contains an artibrary TDP-encoded message, as defined in
[RFD 0037](./0037-desktop-access-protocol.md). The vast majority of these events
will contain TDP message 2 (PNG frame), but we will also capture:

- message 1 - client screen spec: to know the initial size of the screen
- message 6 - clipboard data: to indicate when the clipboard was used
- message 3 - mouse move
- message 4 - mouse button

Of note: we will not be capturing other user input events (keyboard or mouse
wheel/scroll), as:

1. They are not necessary for playback. The result of keyboard input or
   scrolling captured in the session's PNG frames instead.
2. The keyboard input may contain sensitive information, like passwords.

We do, however, need to capture mouse movement and button clicks, as the RDP
bitmaps sent from the Windows desktop do _not_ include the mouse cursor
(RDP clients show the native OS cursor and send its position to the desktop).

When this new event is introduced, several parts of the code that assume
`SessionPrintEvent` is the only type of session recording will need to be
udpated:

- `AuditWriter` assumes every `[]byte` it receives needs to be packaged up into
  a `SessionPrintEvent`. We will extend `AuditWriterConfig` with a new field:
  `MakeEvent func(b []byte) AuditEvent`. This will enable SSH sessions to
  configure `AuditWriter` to produce `SessionPrintEvent`s, while allowing
  desktop sessions to emit `DesktopRecordingEvent`s.
- `AuditWriter` does some tracking of `lastPrintEvent` in order to update
  timestamps and chunk indicdes. This needs to be generalized to be able to
  track the last event in desktop mode as well.
- The `TeeStreamer` that filters out `SessionPrintEvent`s needs to be updated to
  also filter out `DesktopRecordingEvent`s, as there will be a large number of
  these events and we don't want to clutter the global audit log with them.

##### Recording Mode

SSH session recording can operate in synchronous or asynchronous mode, and can
be captured at the node or at the proxy, resulting in a total of 4 possible
configurations.

- `node`
- `node-sync`
- `proxy`
- `proxy-sync`
- `off`

Synchronous recording mode emits each audit event directly to the audit log, and
fails if an event can not be written. Asynchronous recording mode writes events
to a local filesystem log, and periodically uploads the audit events to external
storage. Asynchronous mode is both more efficient (due to making less API calls)
and more resilient (due to support for retries, backoff, stream recovery, etc).
For desktop recordings, Teleport will support sync or async modes.

Unlike SSH sessions, where the node itself is running Teleport and the user has
an option of where the recording will take place, Desktop Access sessions will
always be recorded by the Windows Desktop Service which has an RDP connection to
the Windows Host. This is because we only need to record certain types of
messages, and the Windows Desktop Service is the only part of the stack that is
TDP protocol-aware (the Teleport Proxy service simply passes data between the
browser and the Windows Desktop Service without attempting to decode or
interpret the data in any way.)

To summarize, desktop session recording will interpret the session recording
configuration as follows:

- `off` means don't record any desktop (or SSH) sessions under any circumstances
- Both `node` and `proxy` result in desktop sessions recorded in async mode
  (this is the recommended configuration). Sessions are only recorded if the
  user has a role enabling desktop session recording.
- Both `node-sync` and `proxy-sync` result in desktop sessions recorded
  in sync mode. Sessions are only recorded if the user has a role enabling
  desktop session recording.

#### Playback

The core component of the playback functionality is the `ProtoReader`, which reads
from a gzipped-stream of protobuf-encoded events and emits deserialized audit events
one at a time. This is what powers the `StreamSessionEvents` API call.

For desktop access, a new websocket endpoint will be added to the Teleport proxy
for session playback. The proxy will pull the session ID out of the URL, and use
`StreamSessionEvents` to start receiving AuditEvents for the session. Since the
desktop recording events contain a TDP message that is already encoded, the proxy
simply needs to look at the event's delay, determine how long to wait, and then
send the event on the websocket. This process repeats until the events channel
is closed and there are no more events to send.

Additionally, during playback mode, the Teleport proxy listens for JSON-encoded
commands from the browser, with the format:

```
{ "action": "play/pause" }
```

The only supported action for the initial implementation will be `play/pause`.
When told to pause, the Teleport proxy will stop streaming playback data to the
browser, while maintaining the current position in the stream. When a subsequent
action is received to resume playback, the proxy starts sending from where it
left off.

In the future, we may support additional actions to "seek" to a particular time
in the stream or adjust the playback speed.

### Security

Teleport 8.1 introduced
[RBAC for sessions](https://goteleport.com/docs/access-controls/reference/#rbac-for-sessions),
allowing users to control access to shared SSH sessions (resource kind
`ssh_session`) or session recordings (kind `session`) via Teleport's RBAC
system.

Desktop Access does not support shared sessions at this time, so we will *not*
add a new resource type for an active desktop session (kind `desktop_session`).

We will, however ensure that RBAC support for `session` resources is extended to
desktop sessions as well. This means that the canonical role for allowing a user
access to only the session recordings that they participated in will work for
desktop recordings without modification:

```
version: v4
kind: role
metadata:
  name: only-own-sessions
spec:
  allow:
    rules:
    # Users can only view session recordings for sessions in which they
    # participated.
    - resources: [session]
      verbs: [list, read]
      where: contains(session.participants, user.metadata.name)
```

In order for this to work, the set of `participants` for a desktop session will
be set to a list containing one element - the user who started the session.

This lays the groundwork for supporting shared desktop sessions in the future
while integrating with the RBAC system as it exists today.

### User Experience

Desktop session recordings will appear in the web UI in the same "Session
Recordings" page as SSH sessions. The existing search and filtering
functionality will be applied to both types of recordings.

There is no translation of screen size during recording, so sessions are
captured at the screen size of the live session. Since we capture the screen
spec message, the player will scale the size of the canvas to fit the size of
the screen needed for playback.

This means that sessions will be viewable when played back on in a window that
is larger or smaller than the original recorded screen size, and there are no
concerns about the recording overflowing the bounds of the playback screen. It
should be noted that smaller recordings played back in very large windows may
appear to be of poor quality, similar to scaling a low resolution image on a
high resolution display.

