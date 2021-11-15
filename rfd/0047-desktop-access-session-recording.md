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

TODO: brief overview of how SSH recordings work, so we can keep that context in
mind for the rest of the doc

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

