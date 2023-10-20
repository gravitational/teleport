---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: implemented
---

# RFD 37 - Desktop Access - Desktop protocol

## What

RFD 33 defines the high-level goals and architecture for Teleport Desktop
Access.

This RFD specifies the Desktop protocol - wire protocol used between the
OS-specific Teleport desktop service (like `windows_desktop_service`) and the
web client.  OS-specific Teleport desktop services are responsible for
translating native protocols or APIs (like RDP or X11) into the protocol
described here.

## Details

### Goals

This custom protocol is created with several goals in mind:
- performance - the messages should be compact and fast to encode/decode
- portability - should map easily to standard protocols like RDP
- decoding simplicity - the parsing code should be simple enough for auditing,
  especially when written in dynamic languages like JavaScript
- extendability - new capabilities can be added in the future

### Overview

The protocol consists of if discrete messages, sent between a client and a
server. These messages are passed over a secure, authenticated and reliable
transport layer, like TLS or a websocket. The protocol leaves all security
concerns (authentication, integrity, etc) to the transport layer.

Messages are bi-directional and asynchronous - client and server can send any
message at any time to the other end. The expected sequence of messages for a
typical desktop connection is described [below](#message-sequence).

### Message sequence

Typical sequence of messages in a desktop session:

```
+--------+                                +--------+
| client |                                | server |
+--------+                                +--------+
     |           7 - client username          |
     |--------------------------------------->|
     |           1 - client screen spec       |
     |--------------------------------------->|
     |             2 - PNG frame              |
     |<---------------------------------------|
     |             2 - PNG frame              |
     |<---------------------------------------|
     |             3 - mouse move             |
     |--------------------------------------->|
     |             4 - mouse button           |
     |--------------------------------------->|
     |             2 - PNG frame              |
     |<---------------------------------------|
     |             5 - keyboard input         |
     |--------------------------------------->|
     |             2 - PNG frame              |
     |<---------------------------------------|
     |                ....                    |
```

Note that `client username` and `client screen spec` **must** be the first two
messages sent by the client, in that order. Any other incoming messages will be
discarded until those two are received.

### Message encoding

Each message consists of a sequence of fields.
Each field is either fixed size or variable size.
The first byte in each message is the message type and defines what fields are
expected after it.

#### Field types

Fields are all numbers, using Go-inspired names. Numbers are encoded in big
endian order.
For example:
- `byte` is a single byte
- `uint32` is an unsigned 32-bit integer
- `int64` is a signed 64-bit integer

Message definitions use the syntax `[]type` to declare a variable size field
with elements of type `type`. The length should be deducted from nearby fields.

Strings are encoded as UTF-8 in a `[]byte` field, with a `uint32` length
prefix.

### Message types

#### 1 - client screen spec

```
| message type (1) | width uint32 | height uint32 |
```

This message contains the dimensions of the client view - the dimensions used
for drawing the remote desktop image. Sent from client to server.

This message can be sent more than once per session, for example when client
resizes their window.

#### 2 - PNG frame

```
| message type (2) | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
```

This message contains new bitmap data for a region of the desktop screen. Sent
from server to client.

`left`, `top` and `right`, `bottom` contain the top-left and bottom-right
coordinates of the region, in pixels.
`data` contains the PNG-encoded bitmap.

#### 27 - PNG frame 2

```
| message type (27) | png_length uint32 | left uint32 | top uint32 | right uint32 | bottom uint32 | data []byte |
```

This is a newer version of the PNG frame message, which includes the length of the PNG data after
the message type. This allows for efficiently skipping over the PNG data without performing
a PNG decode.

#### 3 - mouse move

```
| message type (3) | x uint32 | y uint32 |
```

This message contains new mouse coordinates. Sent from client to server.

#### 4 - mouse button

```
| message type (4) | button byte | state byte |
```

This message contains a mouse button update. Sent from client to server.

`button` identifies which button was used:
- `0` is left button
- `1` is middle button
- `2` is right button

`state` identifies the button state:
- `0` is not pressed
- `1` is pressed

#### 5 - keyboard input

```
| message type (5) | key_code uint32 | state byte |
```

This message contains a keyboard update. Sent from client to server.

`key_code` is the keyboard code from https://developer.mozilla.org/en-US/docs/Web/API/KeyboardEvent/code/code_values#code_values_on_windows

`state` identifies the key state:
- `0` is not pressed
- `1` is pressed

Key combinations show up as a sequence of keys going into `state=1` and then
back to `state=0`.

#### 6 - clipboard data

```
| message type (6) | length uint32 | data []byte |
```

This message contains clipboard data. Sent in either direction.
When this message is sent from server to client, it's a "copy" action.
When this message is sent from client to server, it's a "paste" action.

#### 7 - client username

```
| message type (7) | username_length uint32 | username []byte |
```

This is the first message of the protocol and contains the username to login as
on the remote desktop.

#### 8 - mouse wheel scroll

```
| message type (8) | axis byte | delta int16 |
```

This message contains a mouse wheel update. Sent from client to server.

`axis` identifies which axis the scroll happened on:
- `0` is vertical scroll
- `1` is horizontal scroll

`delta` is the signed scroll distance in pixels.
- on vertical axis, positive `delta` is up, negative `delta` is down
- on horizontal axis, positive `delta` is left, negative `delta` is right

#### 9 - error

```
| message type (9) | message_length uint32 | message []byte |
```

This message indicates an error has occurred.

#### 28 - notification

```
| message type (28) | message_length uint32 | message []byte | severity byte |
```

This message sends a notification message with a severity level. Sent from server to client.

`message_length` denotes the length of the `message` byte array. It doesn't include the `severity` byte.

`severity` defines the severity of the `message`:
- `0` is for info
- `1` is for a warning
- `2` is for an error

An error (`2`) means that some fatal problem was encountered and the TDP connection is ending imminently.
A notification with `severity == 2` should be preferred to the `error` message above.

A warning (`1`) means some non-fatal problem was encountered but the TDP connection can still continue.

Info (`0`) can be used to communicate an arbitrary message back to the client without error semantics.

#### 10 - MFA

```
| message type (10) | mfa_type byte | length uint32 | JSON []byte |
```

This message is used to send the MFA challenge to the user when per-session MFA
is enabled. It is a container for a JSON-encoded MFA payload.

`mfa_type` is one of:

- `n` for Webauthn
- `u` for U2F

Per-session MFA for desktop access works the same way as it does for SSH
sessions. A JSON-encoded challenge is sent over websocket to the user's browser.
The only difference is that SSH sessions wrap the MFA JSON in a protobuf
encoding, where desktop sessions wrap the MFA JSON in a TDP message.
