---
authors: Zac Bergquist <zac@goteleport.com>
state: implemented (Teleport 15)
---

# RFD 91 - Streaming Session Playback

## Required Approvers

* Engineering: @kimlisa && (@ibeckermayer || @ryanclark)
* Product: @xinding33

## What

Update Teleport's session playback to use the `StreamSessionEvents` API.

Teleport's session recordings are stored as an ordered stream of gzipped and
protobuf-encoded events. See [RFD 2](./0002-streaming.md) for details.

Prior versions of Teleport stored recordings in a different format where the
session content (referred to as "chunks") was stored separately from the
metadata and timing information about those chunks.

When RFD 2 was implemented and session streaming was added, the players
(both the web UI and `tsh play`) saw minimal changes and we opted to convert
the new protobuf event stream into the legacy metadata/chunks format for
playback.

This RFD outlines the steps necessary to remove this legacy conversion step
and support streaming for session playback directly.

## Why

There are several reasons why we'd like to make this change.

### Removal of Legacy Code

The `lib/events` package has a long history of [issues](https://github.com/gravitational/teleport/issues?q=is%3Aopen+is%3Aissue+label%3Aaudit-log)
and dead code that
[should](https://github.com/gravitational/teleport/pull/13217)
[be](https://github.com/gravitational/teleport/pull/12395)
[removed](https://github.com/gravitational/teleport/pull/11380).

While we've succeeded in removing a large amount of old code, there is still
a non-trivial amount of code left behind due to the legacy playback format
that can be removed once we support the newer streaming API.

Reducing the surface area of the packages related to the audit log and session
recordings will make it easier to understand and modify as we continue to work
on new efforts such as an encrypted audit log.

### User Experience

The required conversion to the old playback format results in an experience that
is suboptimal for users. Since this process requires the entire session be
downloaded and converted before playback can begin, users experience significant
latency when attempting to play back a large session.

A prototype of streaming playback with a large session has shown a > 10x
improvement in "time to first byte" of playback (11s to < 1s).

In addition to latency, the current approach requires the entire session to be
loaded into memory. This is inefficient, and consuming large amounts of system
memory can impact the overall responsiveness of the system.

In the web UI, this can also result in failure to play the session at all,
as observed in [#10578](https://github.com/gravitational/teleport/issues/10578).

### Consistency with Desktop Access

When session recording and playback for desktop access was implemented
(see [RFD 48](./0048-desktop-access-session-recording.md) for details),
we needed to build a new graphical session player, so there was no reason
to convert to the legacy format in order to reuse the text-based SSH player.

As a result, desktop access has supported streaming playback since its
inception. Implementing this RFD for other types of sessions will ensure
greater consistency across the codebase, and will address the long-standing
request to implement seek functionality for desktop playback.

## Details

In all cases, we will be leveraging the `StreamSessionEvents` API to do session
playback. This API presents the following Go interface:

```
func StreamSessionEvents(
    ctx context.Context,
    sessionID session.ID,
    startIndex int64,
  ) (chan apievents.AuditEvent, chan error) {
```

A gRPC wrapper is also available:

```
  rpc StreamSessionEvents(StreamSessionEventsRequest) returns (stream events.OneOf);
```

Given a session ID and a start index, a channel of `AuditEvent`s is returned.
Consumers can control the speed of playback by limiting how fast they consume
from this channel. Each event has a timestamp indicating the number of
milliseconds that have elapsed since the beginning of the session, so consumers
can implement "real time" playback by sleeping for the correct amount of time
before consuming the next event. Speeding up playback can be accomplished by
sleeping for less time than required.

Skipping forward in a session can be done efficiently by playing back events as
fast as possible with no timing delay in between events until the desired
position is reached.

Skipping backward in a session is the most difficult function to  implement, as
this API offers no way to go back. Once an event is consumed from
the channel it is gone. While it is possible to keep some sort of buffer for the
last N events, this approach is not suitable here for several reasons:

- There is not an obvious buffer size to use. Select too small of a buffer, and
  you end up needing to start from the beginning anyway. Select too large of a
  buffer and you end up in the same state we are in today
  (too much memory usage, high latency, etc)
- The state of the terminal at any point in time is a function of all of the
  events that have occurred up until that time, so you always need to play from
  the beginning to accurately depict the session. This is true for both TTY
  sessions and desktop sessions.

Here we propose the naive approach of rewinding by:

1. restarting the stream from the beginning
2. "fast forwarding" up to the desired rewind point by playing back events
   without inserting a timing delay
3. resuming playback from this point at regular speed

### `tsh`

For `tsh` we can leverage the gRPC call that wraps `StreamSessionEvents`.
`tsh` will perform playback in a background goroutine, and listen for keypresses
to control playback (play, pause, skip forward, etc).

`tsh` supports the following playback commands:

- `ctrl+c` / `ctrl+d`: stop playing
- `space`: play/pause
- `left` / `down`: rewind
- `right` / `up`: fast-forward

This will be a relatively isolated change, as the `tsh` player code already
ranges over a slice of events. The loop will need to range over a channel of
events instead.

### Web UI

Today, the web UI performs an API call to
`/webapi/sites/:site/sessions/:sid/stream`. Unlike other APIs, this endpoint
does not return JSON - it returns the text contents directly. This API call
is synchronous and does not support streaming. The web UI fetches all of the
data (using multiple API calls if the session is large) before playback can
begin.

In order to better support streaming, we will need a new endpoint with better
support for a streaming API. The two options considered are server-sent events
(SSE) and websockets.

While SSE is often a good fit for streaming applications, it suffers from two
major limitations. First, most browsers implement a relatively small limit (6)
on the number of SSE subscriptions (player tabs, in our case) that can be open
to a single site. Second, while SSE is a convenient mechanism for a server to
stream data to a client, we actually need a bidirectional stream so that the
client can stream commands (play, pause, etc.) to the server.

This leaves websockets as the best choice for web UI playback. In addition to
supporting bidirectional streaming, websockets are already used throughout
Teleport and will not require additional server-side dependencies.

To implement this, we can leverage the desktop session player for inspiration.
We will add a new endpoint, `/webapi/sites/:site/ttyplayback/:sid`, which
upgrades the connection to websockets and streams events to the browser while
listening for playback commands. The commands sent from the browser will be
JSON-encoded, though the websocket will use binary encoding since there will be
a large amount of binary playback data as compared to the number of commands.

Since Desktop Access playback already uses JSON commands over websocket, we will
reuse the same set of commands.

- Play/pause: `{ "action": "play/pause" }`
- Set playback speed: `{ "action": "speed", "speed": 2.0 }`

In addition, a new command for "seeking" to a specific position in the recording will be added (for both desktop and SSH playback):

`{ "action": "seek", "ms": 25 }`

In this command, the `ms` field specifies the number of milliseconds into the
recording. Negative values are not accepted, and values greater than the length
of the recording will cause the playback to immediately jump to the end and
effectively stop playing.

### Deprecation/Implementation Plan

We will leverage a two-phased approach to ensure `tsh` and API client
compatibility for one major version.

*Phase 1:* Add new endpoint and introduce deprecation warning

In the first step, which we target for release in Teleport major release N.0.0,
we will:

- introduce the new websocket-based streaming endpoint in the web UI
- update the web UI player to leverage the new endpoint
- remove the `/webapi/sites/:site/sessions/:sid/stream` endpoint
- update `tsh play` to use streaming
- remove the playback conversion code
- mark `GetSessionEvents` and `GetSessionChunk` as deprecated in our API client

*Phase 2:* Fully remove old APIs

In the next major release, N+1.0.0, we will:

- remove the `GetSessionEvents` and `GetSessionChunk` from the gRPC API client
- remove `EventFields` from the `lib/events` package. This was the legacy way
  to refer to a generic event as key-value pairs before we introduced the
  common interface that all events must implement (`events.AuditEvent`)

### Security

There are no *new* security implications to consider here, since the streaming
APIs already exist and are already protected by Teleport's RBAC system.

### UX

From a user experience perspective, implementing this RFD will:

- reduce the initial latency when starting to play a recorded sessions
  (this will be more noticeable with larger sessions than smaller sessions)
- improve the experience of playing back sessions on resource constrained
  systems with smaller amounts of memory
- *increase* the latency of "rewind" operations, since the session stream
  needs to be restarted from the beginning to go backwards in time

This tradeoff is considered acceptable since users always experience the
initial latency when playing back a session, and only sometimes have a need
to rewind a session.