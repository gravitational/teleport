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

#### Recording

In standard operation where we are recording sessions directly on the node (rather than through the proxy), each SSH [session](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L480-L534)
is given an [`AuditWriter`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L147-L166) as its [`recorder`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L679-L691), which is then [added as one of the `multiWriter`s of that `session`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L696). All input and output of the session will be [written to that `s.writer`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L799), which is a struct that registers `io.WriteCloser`'s and writes to all of them on `Write`. Therefore, along with any `io.WriteCloser`'s corresponding to each party (person) in the session, all input and output bytes are written by the [`AuditWriter`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L192-L228), which sets up part of a protobuf-generated struct named [`SessionPrint`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/api/types/events/events.pb.go#L540-L559) and then calls [`EmitAuditEvent`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L267-L359), which finalizes event setup by calling [setupEvent](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L555-L590) (which itself calls [checkAndSetEventFields](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/emitter.go#L175-L197)) before [writing it to the `AuditWriter`'s `eventsCh`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L291) (note that there is some complex backoff logic below that line in case of a bottleneck). By the time that channel is written to, your event will look something like the following:

```go
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

The `eventsCh` is read from a [`processEvents`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L65) loop, which calls [`EmitAuditEvent`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L421) of an [`AuditStream`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L43) wrapped in a [`CheckingStream`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/auditwriter.go#L52). In the codepath we're following (which traverses through [`startInteractive`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L675)), that `AuditStream` is a [`TeeStreamer`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L1074) which creates a `TeeStream` whose [`EmitAuditEvent`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/emitter.go#L553-L570) writes all events to a [`filesessions.NewStreamer`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L1067) object, which [creates a `ProtoStreamer`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L1067) that is ultimately responsible for uploading the audit stream [to a storage backend](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/stream.go#L116-L118). The `TeeStream` also writes events via [`t.emitter.EmitAuditEvent`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/emitter.go#L566), which is (presumably) another object asynchronously writing to the audit log on the auth server ([`ctx.srv`](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/srv/sess.go#L1074)), but is [skipping](https://github.com/gravitational/teleport/blob/bfe7f9878a05bce08129ce92ee8773bace7fd75b/lib/events/emitter.go#L563) the following events: `ResizeEvent, SessionDiskEvent, SessionPrintEvent, AppSessionRequestEvent, ""`.

Zooming out, as a session is being recorded the node creates the path

```bash
# TODO: What is $UID_1? Presumably something related to multi.
# TODO: what is $DATA_DIR/log/upload/sessions for?
# TODO: "default" is presumably the default namespace -- is this a vestigial feature at this point, or does teleport still support namespaces?
$DATA_DIR/log/upload/streaming/default/multi/$UID_1/$SESSION_ID
```

When the session ends, it appears that it's corresponding file is uploaded to the auth server (TODO: confirm its auth) under

```bash
# TODO: while the file is named .tar, it doesn't seem to be recognized by the system as such.
# When I try a `tar -tvf $SESSION_ID.tar`
# I get back
# tar: This does not look like a tar archive
# tar: Skipping to next header
# tar: Exiting with failure status due to previous errors
$DATA_DIR/log/records/$SESSION_ID.tar
```

TODO: figure out how file upload works. Is the file saved (what's its final file name? right now I've only discovered the path) and then a job periodically uploads saved files to the server, deleting upon success? Or else is the upload attempted immediately after the session ends?

There is also a

```bash
$DATA_DIR/log/records/multi
```

which presumably is going unused in my experiments because a multi-part upload wasn't required. TODO: if this was a multi-part upload, would the files be saved under

```bash
$DATA_DIR/log/records/multi/$UID_1/$SESSION_ID.tar
```

? if so, how would we later discover that for playback, which appears to be identified by `sessionId`?

#### Playback

When a session playback is requested, a new set of files are created in `$DATA_DIR/log/playbacks/sessions/default`:

```bash
$SESSION_ID.tar
$SESSION_ID.index
$SESSION_ID-0.chunks
$SESSION_ID-0.chunks.gz
$SESSION_ID-0.events.gz
```

`$SESSION_ID.tar` is the file in `$DATA_DIR/log/records/$SESSION_ID.tar`, copied over.

`$SESSION_ID.index` looks like

```
{"file_name":"$SESSION_ID.events.gz","type":"events","index":0,"offset":0}
{"file_name":"$SESSION_ID.chunks.gz","type":"chunks","index":0,"offset":0}
```

TODO: presumably this is useful in the case of a long playback that gets broken up into many chunks.

`$SESSION_ID-0.chunks` apppears to be the raw byte stream from the session

```
^[]0;root@localhost: ~^Groot@localhost:~# ls
^[]0;root@localhost: ~^Groot@localhost:~# echo "Something"
Something
^[]0;root@localhost: ~^Groot@localhost:~# ls
^[]0;root@localhost: ~^Groot@localhost:~# ^[[H^[[2J^[]0;root@localhost: ~^Groot@localhost:~# cd /
^[]0;root@localhost: /^Groot@localhost:/# ls
^[[0m^[[01;36mbin^[[0m  ^[[01;34mboot^[[0m  ^[[01;34mdev^[[0m  ^[[01;34metc^[[0m  ^[[01;34mhome^[[0m  ^[[01;36mlib^[[0m  ^[[01;36mlib32^[[0m  >
^[]0;root@localhost: /^Groot@localhost:/# tree
...
```

TODO: presumably this was pieced together by appending the `Data` fields in each `SessionPrint` event saved (in some unknown format) in the `$SESSION_ID.tar`

```go
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

`$SESSION_ID-0.chunks.gz` is just the LZ77 encoded `$SESSION_ID-0.chunks`, presumably used for sending over the wire. TODO: is there a reason an unzipped version of this persists in the directory, whereas there's no correlary unzipped version of `$SESSION_ID-0.events.gz`?

`$SESSION_ID-0.events.gz` is an LZ77 encoded file containing what appears to be a list of all the events in the session (including non-print events), in a format that ignores the actual data in each print event.

```
{"ei":0,"event":"session.start","uid":"c06685ff-1b59-4858-b3a3-5e9467a77616","code":"T2000I","time":"2021-11-18T15:33:59.792Z","cluster_name":"teleport.zacazure.grvpoc.com","user":"joe","login":"root","sid":"9b6ec731-80f7-4751-9154-4214ecdc812a","namespace":"default","server_id":"ade010d6-4e75-4ed8-9f92-886a4f49eb26","server_hostname":"localhost","server_addr":"173.230.128.45:45350","addr.remote":"173.160.69.53:56544","size":"80:25","session_recording":"node"}
{"ei":1,"event":"resize","uid":"c13d7ba5-42d0-4dbe-9c16-7fc772093055","code":"T2002I","time":"2021-11-18T15:33:59.862Z","cluster_name":"teleport.zacazure.grvpoc.com","user":"joe","login":"root","sid":"9b6ec731-80f7-4751-9154-4214ecdc812a","namespace":"default","server_id":"ade010d6-4e75-4ed8-9f92-886a4f49eb26","server_hostname":"localhost","server_addr":"173.230.128.45:45350","size":"168:44"}
{"ei":2,"event":"print","time":"2021-11-18T15:34:00.069Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":0,"bytes":40,"ms":0,"offset":0}
{"ei":3,"event":"print","time":"2021-11-18T15:34:00.769Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":1,"bytes":1,"ms":700,"offset":40}
{"ei":4,"event":"print","time":"2021-11-18T15:34:00.859Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":2,"bytes":1,"ms":790,"offset":41}
{"ei":5,"event":"print","time":"2021-11-18T15:34:01.025Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":3,"bytes":2,"ms":956,"offset":42}
{"ei":6,"event":"print","time":"2021-11-18T15:34:01.028Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":4,"bytes":40,"ms":959,"offset":44}
{"ei":7,"event":"print","time":"2021-11-18T15:34:02.239Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":5,"bytes":1,"ms":2170,"offset":84}
{"ei":8,"event":"print","time":"2021-11-18T15:34:02.556Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":6,"bytes":2,"ms":2487,"offset":85}
{"ei":9,"event":"print","time":"2021-11-18T15:34:02.584Z","cluster_name":"teleport.zacazure.grvpoc.com","ci":7,"bytes":1,"ms":2515,"offset":87}
```

TODO: My guess is that this file was generated from the `SESSION_ID.tar` file (which I guess contains the full version of the events, including the raw data stream). My first thouth is that its likely cross-referenced by the frontend to decide the time to display the bytes during playback. However it's unclear why this would need to be kept in a separate file, assuming the `SESSION_ID.tar` indeed includes some representation of the print events with the raw data.

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
