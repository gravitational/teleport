---
authors: Zac Bergquist (zac@goteleport.com), Isaiah Becker-Mayer (isaiah@goteleport.com)
state: draft
---

# RFD 0047 - Desktop Access: Session Recording

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

#### Playback

TODO(zmb3)

### Overview

During a live session, desktop images are sent to the browser via a series of
PNG frames encoded in a TDP message (message #2, "PNG frame"). At a high level,
the session recording feature needs to archive these messages so that they can
be played back at a later time. During playback, the majority of the frontend
code works exactly as it does during a live session - the PNGs are rendered on
an HTML canvas.

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

In order to support a new type of session recording, a new audit event will be
defined for capturing desktop session data. The `DesktopRecordingEvent` will
implement the `AuditEvent` interface.

```protobuf
// DesktopRecordingEvent happens when a Teleport Desktop Protocol message
// is captured during a Desktop Access Session.
message DesktopRecordingEvent {
    // Metadata is a common event metadata
    Metadata Metadata = 1
        [ (gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = "" ];

    // ChunkIndex is a monotonicaly incremented index for ordering events
    int64 ChunkIndex = 2 [ (gogoproto.jsontag) = "ci" ];

    // Message is the encoded TDP message. It is not marshaled to JSON format
    bytes Message = 3 [ (gogoproto.nullable) = true, (gogoproto.jsontag) = "-" ];

    // Bytes says how many bytes have been written into the session
    int64 Bytes = 4 [ (gogoproto.jsontag) = "bytes" ];

    // DelayMilliseconds is the delay in milliseconds from the start of the session
    int64 DelayMilliseconds = 5 [ (gogoproto.jsontag) = "ms" ];

    // Offset is the offset in bytes in the session file
    int64 Offset = 6 [ (gogoproto.jsontag) = "offset" ];
}
```

The `Message` field contains an artibrary TDP-encoded message, as defined in
[RFD 0037](./0037-desktop-access-protocol.md). The vast majority of these events
will contain TDP message 2 (PNG frame), but we will also capture:

- message 1 - client screen spec: to know when the canvas needs to be resized
  during playback
- message 6 - clipboard data: to indicate when the clipboard was used in a
  session

Of note: we will not be capturing other user input events (keyboard or mouse
input), as they are not necessary for playback. The result of keyboard or mouse
input is captured in the session's PNG frames instead.

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
  these events and we don't want to clog the audit log.

#### Playback

TODO(zmb3)

### Security

TODO(zmb3)

- Do we need to make any updates to RBAC for access to desktop recordings?
- Is there an attack vector by running very long sessions? (especially when not
  using S3-compatible backend) - maybe when we have a better idea of session
  size we can emit warnings if disk space appears limited
- Are recordings encrypted at rest?
- We will leverage existing recording support which is known to be well-behaved
  in poor network conditions, supports backoff, can run in sync or async modes,
  etc.

### User Experience

Desktop session recordings will appear in the web UI in the same "Session
Recordings" page as SSH sessions. The existing search and filtering
functionality will be applied to both types of recordings.

One notable difference between SSH recordings and desktop recordings is that SSH
recordings can be played back in both the web UI or via the `tsh` command line
client. Desktop sessions are not text based and therefore can only be played
back via the web UI.

One interesting UX consideration is screen sizes. At this time we are aiming to
keep CPU intensive operations out of the critical path of recording, so we have
no plans to resize PNG frames during the recording process. This means that
attempting to view a session on a smaller screen than what it was recorded on
may overflow the bounds of the screen.

TODO(isaiah): brief description of playback functionality (play/pause, playback speed, permalink to specific time, etc.)

TODO(isaiah): investigate whether we can scale a recording down client-side
during playback

TODO(isaiah): any additional interface changes?
- icon do distinguish SSH vs desktop session?
- filter to show only SSH or only desktops in the list?