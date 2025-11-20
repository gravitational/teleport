---
authors: Ryan Hammonds (rhammonds@goteleport.com)
state: draft
---

# RFD 232 - Desktop Access - Desktop Access Protobufs

## Required Approvers

- Engineering: @zmb3 && (@probakowski || @danielashare)

## What
Re-define Teleport Desktop Protocol (TDP) to make use of protocol buffers (TDPB). 

## Why
The motivation to revise TDP arises from two key shortcomings that make the protocol difficult to extend:
- Lack of message framing
- Message serialization/deserialization lacks forward and backwards compatibility

TDP does not define any mechanism for the framing of messages. Message boundaries
are determined implicitly by detecting the message type from the first byte received,
then deserializing the remainder of the message according to that message's schema.
This means that:
- Encountering an unknown message type is an unrecoverable error since the
  implementation cannot possibly know how many remaining bytes to discard from
  the stream.
- It is impossible to change the schema of an existing message because older or
  newer peers with different schema definitions will incorrectly determine message
  boundaries, resulting in misalignment within the stream.

Furthermore, even in the absence of framing concerns, individual messages schemas
cannot be changed due to TDP's inflexible binary format. The omission or addition
of a single field will result in decoding errors between peers running different
TDP versions.


## Why Protocol Buffers?
### Ease of conversion
RFD 37 defines the following types for use in TDP messages:
- `byte`
- `uint32`
- `int64`
- `string` (UTF-8 encoded text represented as `[]byte`)
- `[]byte` (variable length byte array)

Existing TDP messages using these types can be easily represented as protocol
buffer messages, with the exception of `byte` which is typically used to
represent boolean values anyhow.

### Backward and Forward Compatibility
By following protocol buffer best practices, we get forward and backward
compatibility "for free" at least with respect to serialization/deserialization
of individual messages. This is one of the main strengths of protocol buffers.

### Familiarity
Protocol buffers are used extensively at Teleport with existing automation for
both Go and Typescript code generation.

## Protocol Specification Updates
Unless otherwise noted in this section, all existing TDP messages will simply
be modeled as equivalent protocol buffer message representations. The behavior
of the protocol will be largely the same aside from the updates below.
See [all message definitions](#appendix-a-example-message-definitions) below.

### Framing
All TDPB messages will be preceded by a simple framing header consisting of
two, unsigned, 32-bit, big-endian encoded integers representing the message
type and message length respectively.

```
| message type (uint32) | message_length (uint32) | message_data []byte |
```

### Proxy/Agent Protocol Selection via ALPN
The current TDP specification provides no handshake mechanism that we can use
to upgrade from TDP to TDPB. Instead, we'll utilize ALPN to attempt to negotiate
the use of TDPB. Modern Desktop agents will successfully negotiate
`teleport-tdpb-1.0`, while older agents will fail negotiation entirely. This will
signal the Teleport Proxy to use legacy TDP messages for the connection.

ALPN may also be useful if we ever need to make breaking changes to the protocol,
such as changes to the envelope message structure.

### Web Client/Proxy Protocol Selection
Typically, Teleport Agents and Clients are expected to connect to a Proxy instance whose
version is greater than or equal to its own, however, the Desktop Access web client is a
special case. During the rollout of a Proxy upgrade, "modern" Proxy instances will serve
"modern" web clients who may in turn establish websocket connections to "legacy" Proxies.
Likewise, "modern" Proxy instances may receive connections from "legacy" web clients.
In order to gracefully handle this pathological upgrade scenario, the web client will need
to support both TDP and TDPB implementations.

By default, both the Proxy and web client will default to TDP for their leg of the session.
A new "TDP upgrade" message will be added to the legacy TDP protocol, allowing the proxy
to initiate an upgrade to TDPB. The Proxy will check for a new query parameter, `tdpb=<version>`,
on the incoming HTTP upgrade request. If present, the Proxy knows that the client is capable
of using TDPB and will issue the upgrade message. Any previously received messages are discarded,
and the connection begins anew with TDPB. "Modern" web clients will supply this new
query parameter and listen for the new upgrade, but continue with TDP as usual.

### Teleport Connect/Proxy Protocol Selection
Unlike the web client, the Teleport Connect should never connect to a Teleport Proxy whose
version is lower than its own. This eliminates the need for the Connect client to support
backwards compatibility with legacy TDP, as the Proxy can handle translation. Since
Teleport Connect uses a bi-directional gRPC stream to tunnel TDP messages between tsh and
the proxy, we can easily add a field to the request message indicating that the tsh client
wishes to embed TDPB messages rather than legacy TDP. Legacy clients will omit this field,
and the proxy will assume that TDP translation is needed.

```protobuf
// TargetDesktop contains information about the destination desktop.
message TargetDesktop {
  // URI of the desktop to connect to.
  string desktop_uri = 1;
  // Login for the desktop session.
  string login = 2;
  // New field indicating that we wish to use TDPB
  // Ex, "teleport-tdpb-1.0"
  string protocol = 3;
}
```

### Translation/Compatibility layer
Desktop Clients (both the web client and Teleport Connect) do not have any explicit
version compatibility rules with Desktop Agents. However, clients as well as agents *do*
have compatibility rules with respect to the Teleport Proxy. Since the Proxy is expected
to be the most up-to-date participant in a given desktop session, it makes sense to build
a compatibility/translation layer into the proxy to facilitate connections between "legacy"
and "modern" endpoints. This approach allows us to isolate backwards compatibility concerns
to the Proxy, rather than maintain backwards compatible TDPB/TDP implementations on the
web client, Teleport Connect client, and Desktop Agent<sup>*</sup>.

<sup>*</sup> As mentioned in the previous section, web clients will need to support legacy
TDP. However, this approach eliminates the need to build backwards compatibility into
both the Teleport Connect client and Desktop Agent.

Once the Teleport Proxy is updated, the websocket connection between browser
client and Proxy will utilize TDPB. However, until legacy Desktop agents are
out of support, TDPB must maintain backwards compatibility with TDP. This can
be achieved by implementing a simple translation layer that runs on the Teleport
Proxy. The outcome of the ALPN exchange will determine the need for this translation
layer on a per-connection basis. 

```                                                                    
                                                   ┌─────────┐ 
                                                   │ Desktop │ 
┌─────────┐      TDPB                              │ Agent   │ 
│ Browser │  (Proxy Translation)                   │ (Legacy)│ 
│ Client  │◄────────────────┐                      └─────────┘ 
└─────────┘                 ▼          ALPN: (None)    ▲       
                          ┌─────────┐      TDP         │       
                          │         │◄─────────────────┘       
                          │  Proxy  │                          
                          │         │◄─────────────────┐       
┌─────────┐               └─────────┘      TDPB        ▼       
│ Browser │    TDPB (Native)▲          ALPN: (TDPB)┌─────────┐ 
│ Client  │◄────────────────┘                      │ Desktop │ 
└─────────┘                                        │  Agent  │ 
                                                   │ (Modern)│ 
                                                   └─────────┘ 
```


### New Connection Greeting
New Client and server hello messages will be exchanged at the start of the
connection. These handshakes will make it easier to implement small changes
to the TDPB protocol. It is much easier to have the implementation advertise
its capabilities rather than infer it from the agent's version number. 

```protobuf
message ClientHello {
    string username = 1;
    ClientScreenSpec screen_spec = 2;
    uint32 keyboard_layout = 3;
    // Future initialization fields
    // or capability advertisements here
}

message ServerHello {
    ConnectionActivated activation_data = 1;
    bool clipboard_enabled = 2;
    // Future initialization fields
    // or capability advertisements here
}
```

These messages will be exchanged at the start of the connection.

```
+--------+                                +--------+
| client |                                | server |
+--------+                                +--------+
     |           Client Hello                 |
     |--------------------------------------->|
     |           Server Hello                 |
     |<---------------------------------------|
     |             PNG frame                  |
     |<---------------------------------------|
     |             Mouse Move                 |
     |--------------------------------------->|
```


### Notable Message Updates

#### 2 - PNG frame and 27 - PNG Frame 2 
Messages 2 and 27, (PNG Frame and PNG Frame 2) will be consolidated into a
single message. The optimization brought on by the PNG 2 message is obsoleted
under protobufs.

```protobuf
message Rectangle {
    uint32 left = 1;
    uint32 top = 2;
    uint32 right = 3;
    uint32 bottom = 4;
}

message PNG frame {
    Rectangle coordinates = 1;
    bytes data = 2;
}
```

#### Message 10 - MFA 
The MFA message will no longer contain json. Instead, it will compose the
existing `MFAAuthenticationChallenge` and `MFAAuthenticateResponse` messages.

## Backwards Compatibility with Screen Recordings
RFD 48 defines a protocol buffer message `DesktopRecordingEvent` that captures
a subset of TDP messages required for session playback.  We can simply add a
new field, `ProtoMessage` to this message.

```protobuf
message DesktopRecordingEvent {
    // Metadata is a common event metadata
    Metadata Metadata = 1;

    // Message is the encoded TDP message. It is not marshaled to JSON format
    bytes Message = 2;

    // A TDPFrame message which contains PNG, Screen Spec,
    // ClipboardData, Mouse Move, and mouse button events
    bytes ProtoMessage = 4;

    // DelayMilliseconds is the delay in milliseconds from the start of the session
    int64 DelayMilliseconds = 3;
}
```

The existing session player will need a small update to handle desktop recording
messages containing either a TDP message or TDPB message.

### Appendix A: Example Message Definitions
```protobuf
extend google.protobuf.MessageOptions {
  optional MessageType tdp_type_option = 61111;
}

// Types of messages that TDPB implementations will exchange
enum MessageType {
  MESSAGE_TYPE_UNSPECIFIED = 0;
  MESSAGE_TYPE_PNG_FRAME = 1;
  MESSAGE_TYPE_FASTPATH_PDU = 2;
  MESSAGE_TYPE_RDP_RESPONSE_PDU = 3;
  MESSAGE_TYPE_SYNC_KEYS = 4;
  MESSAGE_TYPE_MOUSE_MOVE = 5;
  MESSAGE_TYPE_MOUSE_BUTTON = 6;
  MESSAGE_TYPE_KEYBOARD_BUTTON = 7;
  MESSAGE_TYPE_ALERT = 8;
  MESSAGE_TYPE_MOUSE_WHEEL = 9;
  MESSAGE_TYPE_CLIPBOARD_DATA = 10;
  MESSAGE_TYPE_MFA = 11;
  MESSAGE_TYPE_SHARED_DIRECTORY_REQUEST = 12;
  MESSAGE_TYPE_SHARED_DIRECTORY_RESPONSE = 13;
  MESSAGE_TYPE_SHARED_DIRECTORY_ANNOUNCE = 14;
  MESSAGE_TYPE_SHARED_DIRECTORY_ACKNOWLEDGE = 15;
  MESSAGE_TYPE_LATENCY_STATS = 16;
  MESSAGE_TYPE_PING = 17;
  MESSAGE_TYPE_CLIENT_HELLO = 18;
  MESSAGE_TYPE_SERVER_HELLO = 19;
  MESSAGE_TYPE_CLIENT_SCREEN_SPEC = 20;
}

// Sent by client to begin a TDPB connection and advertise capabilities.
message ClientHello {
  option (tdp_type_option) = MESSAGE_TYPE_CLIENT_HELLO;
  string username = 1;
  ClientScreenSpec screen_spec = 2;
  uint32 keyboard_layout = 3;
}

// Sent by server in response to a 'Client Hello'. Advertises server capabilities.
message ServerHello {
  ConnectionActivated activation_spec = 1;
  bool clipboard_enabled = 2;
  option (tdp_type_option) = MESSAGE_TYPE_SERVER_HELLO;
}

// Defines the boundaries that PNG frame will update.
// Used for composition on PNG frame messages only.
message Rectangle {
  uint32 left = 1;
  uint32 top = 2;
  uint32 right = 3;
  uint32 bottom = 4;
}

// Contains updated image data to be displayed.
message PNGFrame {
  option (tdp_type_option) = MESSAGE_TYPE_PNG_FRAME;
  Rectangle coordinates = 1;
  bytes data = 2;
}

// Contains a raw RDP FastPath message to by interpreted by the client.
message FastPathPDU {
  option (tdp_type_option) = MESSAGE_TYPE_FASTPATH_PDU;
  bytes pdu = 1;
}

// Contains a raw RDP response PDU to be interpreted by the server.
message RDPResponsePDU {
  option (tdp_type_option) = MESSAGE_TYPE_RDP_RESPONSE_PDU;
  bytes response = 1;
}

// Internal message sent by the server after establishing a connection
// to the RDP host.
message ConnectionActivated {
  uint32 io_channel_id = 1;
  uint32 user_channel_id = 2;
  uint32 screen_width = 3;
  uint32 screen_height = 4;
}

// Conveys the current state of keyboard buttons with persistent state.
message SyncKeys {
  option (tdp_type_option) = MESSAGE_TYPE_SYNC_KEYS;
  bool scroll_lock_pressed = 1;
  bool num_lock_state = 2;
  bool caps_lock_state = 3;
  bool kana_lock_state = 4;
}

// Represents the current position of the cursor on the client.
message MouseMove {
  option (tdp_type_option) = MESSAGE_TYPE_MOUSE_MOVE;
  uint32 x = 1;
  uint32 y = 2;
}

// Specifies which mount button was pressed.
enum MouseButtonType {
  MOUSE_BUTTON_TYPE_UNSPECIFIED = 0;
  MOUSE_BUTTON_TYPE_LEFT = 1;
  MOUSE_BUTTON_TYPE_MIDDLE = 2;
  MOUSE_BUTTON_TYPE_RIGHT = 3;
}

// Informs the server of a mouse button press.
message MouseButton {
  option (tdp_type_option) = MESSAGE_TYPE_MOUSE_BUTTON;
  MouseButtonType button = 1;
  bool pressed = 2;
}

// Informs the server of a keyboard button press.
message KeyboardButton {
  option (tdp_type_option) = MESSAGE_TYPE_KEYBOARD_BUTTON;
  uint32 key_code = 1;
  bool pressed = 2;
}

// Composed in Client Hello to inform the server of the client's screen size.
// May also be sent during a desktop session as the client resizes its display.
// These messages are captured for session recordings in order to replay
// resizing events.
message ClientScreenSpec {
  option (tdp_type_option) = MESSAGE_TYPE_CLIENT_SCREEN_SPEC;
  uint32 width = 1;
  uint32 height = 2;
}

// Severity of an alert contained in an Alert message.
enum AlertSeverity {
  ALERT_SEVERITY_UNSPECIFIED = 0;
  ALERT_SEVERITY_INFO = 1;
  ALERT_SEVERITY_WARNING = 2;
  ALERT_SEVERITY_ERROR = 3;
}

// Represents an Alert to be displayed by the client.
message Alert {
  option (tdp_type_option) = MESSAGE_TYPE_ALERT;
  string message = 1;
  AlertSeverity severity = 2;
}

// Represents the axis on which a scroll wheel acts.
enum MouseWheelAxis {
  MOUSE_WHEEL_AXIS_UNSPECIFIED = 0;
  MOUSE_WHEEL_AXIS_VERTICAL = 1;
  MOUSE_WHEEL_AXIS_HORIZONTAL = 2;
}

// Mouse wheel event
message MouseWheel {
  option (tdp_type_option) = MESSAGE_TYPE_MOUSE_WHEEL;
  MouseWheelAxis axis = 1;
  uint32 delta = 2;
}

// Represents shared clipboard data.
message ClipboardData {
  option (tdp_type_option) = MESSAGE_TYPE_CLIPBOARD_DATA;
  bytes data = 1;
}

// MFA challenge type
enum MFAType {
  MFA_TYPE_UNSPECIFIED = 0;
  MFA_TYPE_WEBAUTHN = 1;
  MFA_TYPE_U2F = 2;
}

// Contains an MFA challenge or response
// The client implicitly expects a non-empty challenge while the server
// expects a non-empty response.
message MFA {
  option (tdp_type_option) = MESSAGE_TYPE_MFA;
  MFAType type = 1;
  proto.MFAAuthenticateChallenge challenge = 2;
  proto.MFAAuthenticateResponse authentication_response = 3;
}

// Sent by client to announce a new shared directory.
message SharedDirectoryAnnounce {
  option (tdp_type_option) = MESSAGE_TYPE_SHARED_DIRECTORY_ANNOUNCE;
  uint32 directory_id = 1;
  string name = 2;
}

// Sent by server to acknowledge a new shared directory.
message SharedDirectoryAcknowledge {
  option (tdp_type_option) = MESSAGE_TYPE_SHARED_DIRECTORY_ACKNOWLEDGE;
  uint32 directory_id = 1;
  uint32 error_code = 2;
}

// Represents an operation on a shared directory.
enum DirectoryOperation {
  DIRECTORY_OPERATION_UNSPECIFIED = 0;
  DIRECTORY_OPERATION_INFO = 1;
  DIRECTORY_OPERATION_CREATE = 2;
  DIRECTORY_OPERATION_DELETE = 3;
  DIRECTORY_OPERATION_LIST = 4;
  DIRECTORY_OPERATION_READ = 5;
  DIRECTORY_OPERATION_WRITE = 6;
  DIRECTORY_OPERATION_MOVE = 7;
  DIRECTORY_OPERATION_TRUNCATE = 8;
  DIRECTORY_OPERATION_ANNOUNCE = 9;
  DIRECTORY_OPERATION_ACKNOWLEDGE = 10;
}

// Contains data necessary for shared directory operations.
// Not all operation types make use of all fields.
message SharedDirectoryRequest {
  option (tdp_type_option) = MESSAGE_TYPE_SHARED_DIRECTORY_REQUEST;
  // Operation type
  DirectoryOperation operation_code = 1;
  // Common fields
  uint32 directory_id = 2;
  uint32 completion_id = 3;
  // Subject path for Info and Move requests.
  string path = 4;
  // Destination path for Move requests.
  string new_path = 9;
  // Specifies file type in CREATE requests.
  uint32 file_type = 5;
  // Specifies offset where the read/write should start
  // in READ and WRITE requests.
  uint64 offset = 6;
  // Requested length to read for READ requests.
  uint32 length = 7;
  // For TRUNCATE requests.
  uint32 end_of_file = 8;
  // Data to be written in WRITE requests.
  bytes data = 10;
  // Defines the shared directory name in
  // ANNOUNCE operations.
  string name = 11;
}

// Represents a file object in a shared directory.
message FileSystemObject {
  uint64 last_modified = 1;
  uint64 size = 2;
  uint32 file_type = 3;
  bool is_empty = 4;
  string path = 5;
}

// Contains data necessary for responses to shared directory operations.
// Not all operation types make use of all fields.
message SharedDirectoryResponse {
  option (tdp_type_option) = MESSAGE_TYPE_SHARED_DIRECTORY_RESPONSE;
  // Common response fields
  uint32 completion_id = 1;
  uint32 error_code = 2;
  // Returned FileSystemObject(s) for INFO and LIST responses.
  repeated FileSystemObject fso_list = 3;
  // Data returned in READ responses.
  bytes data = 4;
}



// Contains latency metrics between the proxy and RDP host
// as well as between the proxy and client.
message LatencyStats {
  option (tdp_type_option) = MESSAGE_TYPE_LATENCY_STATS;
  uint32 client_latency = 1;
  uint32 server_latency = 2;
}

// A ping message used to time latency between the web client and proxy.
message Ping {
  option (tdp_type_option) = MESSAGE_TYPE_PING;
  // UUID is used to correlate message send by proxy and received from the Windows Desktop Service
  bytes uuid = 1;
}
```
