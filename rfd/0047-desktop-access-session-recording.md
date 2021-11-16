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

- performance - session playback should as be at least as fluid as the live session
- consistency - the user experience of viewing a desktop session should feel
  familiar to those who have experience browsing and playing back SSH sessions
- extendability - new capabilities can be added in the future:
  - playback speed
  - export to video file for viewing outside browser
  - identifying key "events" in the playback (file transfer, clipboard action, etc.)

### Non-Goals

While we will not rule these features out in a future update, the following
items are not a high priority for the initial implementation of session
recording:

- Video Export. In the initial implementation, we will only support viewing
  desktop sessions in the browser.
- File Size Optimization. TDP sends a lot of PNG frames over the wire. We expect
  that session recordings may consume large amounts of disk space.

### Prior Art

<!-- TODO: brief overview of how SSH recordings work, so we can keep that context in
mind for the rest of the doc -->

Teleport SSH connections each have their own [session](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/session/session.go#L82-L109) (distinct from [web sessions](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/api/types/session.go#L53-L91); we currently do not have anything analogous for desktop sessions).

TODO: how are the above session id's connected to those of `session` below, if at all?

In standard operation where we are recording sessions directly on the node (rather than through the proxy), each SSH [session](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L480-L534)
is given an [`AuditWriter`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L147-L166) as its [`recorder`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L679-L691), which is then [added as one of the `multiWriter`s of that `session`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L696). All input and output of the session will be [written to that `s.writer`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L799), which is a struct that registers `io.WriteCloser`'s and writes to all of them on `Write`. Therefore, along with any `io.WriteCloser`'s corresponding to each party (person) in the session, all input and output bytes are written by the [`AuditWriter`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L192-L228), which sets up part of a protobuf-generated struct named [`SessionPrint`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/api/types/events/events.pb.go#L540-L559) and then calls [`EmitAuditEvent`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L267-L359), which finalizes event setup by calling [setupEvent](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L555-L590) (which itself calls [checkAndSetEventFields](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/emitter.go#L175-L197)) before [writing it to the `AuditWriter`'s `eventsCh`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L291) (note that there is some complex backoff logic below that line in case of a bottleneck). By the time that channel is written to, your event will look something like the following:

```go
// TODO: How do we know which session this event belongs to?
// It seems as though the session ID may be registered in the filepath,
// as when I look in the audit log in the UI I find:
// {
//   "cluster_name": "teleport.zacazure.grvpoc.com",
//   "code": "T2005I",
//   "ei": 2147483647,
//   "event": "session.upload",
//   "sid": "f9cc2921-c655-47f9-9179-6ae546fc04a8",
//   "time": "2021-11-16T21:30:26.865Z",
//   "url": "file:///home/zmb/datadir/log/records/multi/f9cc2921-c655-47f9-9179-6ae546fc04a8"
// }
// However, the home/zmb/datadir/log/records/multi directory is empty. I do see a
// home/zmb/datadir/log/records/f9cc2921-c655-47f9-9179-6ae546fc04a8.tar, however when I try a
// `tar -tvf f9cc2921-c655-47f9-9179-6ae546fc04a8.tar`
// I get back
// tar: This does not look like a tar archive
// tar: Skipping to next header
// tar: Exiting with failure status due to previous errors
printEvent := SessionPrint {
	// Metadata is a common event metadata
	Metadata: {
    // Index is a monotonicaly incremented index in the event sequence
    Index: 10 // AuditWriter.eventIndex++
    // Type is the event type
    Type: "print" // SessionPrintEvent
    // ID is a unique event identifier
    ID: "" // empty for SessionPrintEvent, see checkAndSetEventFields
    // Code is a unique event code
    Code "" // empty for SessionPrintEvent events, see checkAndSetEventFields
    // Time is event time
    Time: time.Now().UTC().Round(time.Millisecond)
    // ClusterName identifies the originating teleport cluster
    ClusterName: "cluster-name"
  }
	// ChunkIndex is a monotonicaly incremented index for ordering print events
	ChunkIndex: 0
	// Data is data transferred, it is not marshaled to JSON format
	Data: []byte{0, 1, 0, 6, 4, 3}
	// Bytes says how many bytes have been written into the session
	// during "print" event
	Bytes: 6
	// DelayMilliseconds is the delay in milliseconds from the start of the session
	DelayMilliseconds: 5000
	// Offset is the offset in bytes in the session file
	Offset: 100
}
```

The `eventsCh` is read from a [`processEvents`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L65) loop, which calls [`EmitAuditEvent`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L421) of an [`AuditStream`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L43) wrapped in a [`CheckingStream`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L52) (Note that the `CheckingStream` is superfluous in the code path described here, though may not be in some other path we're not following). In the codepath we're following (which traverses through [`startInteractive`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L675)), that `AuditStream` is a [`TeeStreamer`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L1074) which creates a `TeeStream` whose [`EmitAuditEvent`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/emitter.go#L553-L570) writes all events to a [`filesessions.NewStreamer`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L1067) object, which [creates a `ProtoStreamer`](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/srv/sess.go#L1067) that is ultimately responsible for uploading the audit stream [to a storage backend](https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/stream.go#L116-L118).


<!-- AuditWriter (https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/events/auditwriter.go#L37-L69) ultimately gets called by https://github.com/gravitational/teleport/blob/dbc032a31fd49cdd2f72636075a84d97557f62b3/lib/sshutils/server.go#L407-L544, see the comment
// HandleConnection is called every time an SSH server accepts a new
// connection from a client.
//
// this is the foundation of all SSH connections in Teleport (between clients
// and proxies, proxies and servers, servers and auth, etc).
//
 -->

<!-- NOTE: the ssh session-id is a url param -->

<!-- TODO: almost certainly these are used to record sessions. confirm that, then -- do we need ours to be analogous in the sense that users should be able to rejoin existing sessions? how are we handling the fact that multiple different users could log in to the same machine with the same account? How are we handling rdp sessions? -->

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

### Considerations

#### Data Storage / Format

TODO:

- where will we store the data
- how often will we write to the backend
- what will the format be? just the raw TDP messages?
- do we need any other metadata? (length of session, etc?)
- TDP currently just sends a bunch of small squares in PNG format - they are not
  in any way "grouped" by frame. do we need this functionality? it might allow for
  more compact storage and would eliminate the "painting" effect, even if the live
  session was over a poor/slow network connection

#### Playback

TODO:

- do we need to also store a timestamp for accurate playback?
- screen size: the raw TDP messages captured during a live session are based on
  the size of the browser window when the client started the session. should we
  convert all recordings to a specific screen size?
- we don't yet support resize during a session - how does this impact session
  recording? would it be better to get resize supported first?
- what playback controls are required vs. nice-to-have?
  - play/pause seems required
  - would be nice to see progress of where you are in playback of the current session
  - for long sessions, being able to increase the playback speed or "seek" to a particular
    time in the playback would be nice
  - another nice to have would be to generate a permalink that drops you to a particular
    timestamp in a session playback
- how should we handle idle time during a session? do we want playback to simply
  skip over it, or identify that the session was open and sitting idle (which is
  a more accurate representation of what happened)
