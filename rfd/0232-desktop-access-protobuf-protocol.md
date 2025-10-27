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
two unsigned varints representing the message type and message length respectively. 

```
| message type (uvarint) | message_length (uvarint) | message_data []byte |
```

### Protocol Selection via ALPN
 The current TDP specification provides no handshake mechanism that we can use
to upgrade from TDP to TDPB. Instead, we'll utilize ALPN to attempt to negotiate
the use of TDPB. Modern Desktop agents will successfully negotiate
`teleport-tdpb-1.0`, while older agents will fail negotiation entirely. This will
signal the Teleport Proxy to use legacy TDP messages for the connection.

 ALPN may also be useful if we ever need to make breaking changes to the protocol,
such as changes to the envelope message structure.

### Translation/Compatibility layer
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


Teleport Connect also reaches the Desktop Agent via the Proxy service, however,
up-to-date Proxy's cannot assume that tsh speaks TDPB as it can with browser
clients. Similar to the Desktop Agent, we'll need some way to determine which
dialect to use for the connection. Since Teleport Connect uses a bi-directional
gRPC stream to tunnel TDP messages between tsh and the proxy, we can easily add
a field to the request message indicating that the tsh client wishes to embed TDPB
messages rather than legacy TDP. Legacy clients will omit this field, and the proxy
will assume that TDP translation is needed.

```protobuf
// TargetDesktop contains information about the destination desktop.
message TargetDesktop {
  // URI of the desktop to connect to.
  string desktop_uri = 1;
  // Login for the desktop session.
  string login = 2;
  // New field indicating that we wish to use TDPB
  bool use_tdpb = 3;
}
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
    // Future initialization fields
    // or capability advertisements here
}

message ServerHello {
    ConnectionActivated activation_data = 1;
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
    bytes ProtoMessage = 3;

    // DelayMilliseconds is the delay in milliseconds from the start of the session
    int64 DelayMilliseconds = 3;
}
```

The existing session player will need a small update to handle desktop recording
messages containing either a TDP message or TDPB message.

### Appendix A: Example Message Definitions
```protobuf
enum MessageType {
  MESSAGE_UNKNOWN = 0;
  MESSAGE_CLIENT_HELLO = 1;
  MESSAGE_SERVER_HELLO = 2;
  MESSAGE_RECTANGLE = 3;
  MESSAGE_PNG_FRAME = 4;
  MESSAGE_FASTPATH_PDU = 5;
  MESSAGE_RDP_RESPONSE_PDU = 6;
  MESSAGE_CONNECTION_ACTIVATED = 7;
  MESSAGE_SYNC_KEYS = 8;
  MESSAGE_MOUSE_MOVE = 9;
  MESSAGE_MOUSE_BUTTON = 10;
  MESSAGE_KEYBOARD_BUTTON = 11;
  MESSAGE_CLIENT_SCREEN_SPEC = 12;
  MESSAGE_CLIENT_USERNAME = 13;
  MESSAGE_ERROR = 14;
  MESSAGE_ALERT = 15;
  MESSAGE_MOUSE_WHEEL = 16;
  MESSAGE_CLIPBOARD_DATA = 17;
  MESSAGE_MFA = 18;
  MESSAGE_SHARED_DIRECTORY_ANNOUNCE = 19;
  MESSAGE_SHARED_DIRECTORY_ACKNOWLEDGE = 20;
  MESSAGE_SHARED_DIRECTORY_INFO_REQUEST = 21;
  MESSAGE_SHARED_DIRECTORY_INFO_RESPONSE = 22;
  MESSAGE_SHARED_DIRECTORY_CREATE_REQUEST = 23;
  MESSAGE_SHARED_DIRECTORY_CREATE_RESPONSE = 24;
  MESSAGE_SHARED_DIRECTORY_DELETE_REQUEST = 25;
  MESSAGE_SHARED_DIRECTORY_DELETE_RESPONSE = 26;
  MESSAGE_SHARED_DIRECTORY_LIST_REQUEST = 27;
  MESSAGE_SHARED_DIRECTORY_LIST_RESPONSE = 28;
  MESSAGE_SHARED_DIRECTORY_READ_REQUEST = 29;
  MESSAGE_SHARED_DIRECTORY_READ_RESPONSE = 30;
  MESSAGE_SHARED_DIRECTORY_WRITE_REQUEST = 31;
  MESSAGE_SHARED_DIRECTORY_WRITE_RESPONSE = 32;
  MESSAGE_SHARED_DIRECTORY_MOVE_REQUEST = 33;
  MESSAGE_SHARED_DIRECTORY_MOVE_RESPONSE = 34;
  MESSAGE_SHARED_DIRECTORY_TRUNCATE_REQUEST = 35;
  MESSAGE_SHARED_DIRECTORY_TRUNCATE_RESPONSE = 36;
  MESSAGE_LATENCY_STATS = 37;
  MESSAGE_PING = 38;
  MESSAGE_CLIENT_KEYBOARD_LAYOUT = 39;
}

extend google.protobuf.MessageOptions {
  optional MessageType tdp_type_option = 50000;
}


message ClientHello {
    option (tdp_type_option) = MESSAGE_CLIENT_HELLO;
    string username = 1;
    ClientScreenSpec screen_spec = 2;
    // Future initialization fields
    // or capability advertisements here
}

message ServerHello {
    option (tdp_type_option) = MESSAGE_SERVER_HELLO;
    // Future initialization fields
    // or capability advertisements here
}


message Rectangle {
  option (tdp_type_option) = MESSAGE_RECTANGLE;
  uint32 left = 1;
  uint32 top = 2;
  uint32 right = 3;
  uint32 bottom = 4;
}

message PNGFrame {
  option (tdp_type_option) = MESSAGE_PNG_FRAME;
  Rectangle coordinates = 1;
  bytes data = 2;
}

message FastPathPDU {
  option (tdp_type_option) = MESSAGE_FASTPATH_PDU;
  bytes pdu = 1;
}

message RDPResponsePDU {
  option (tdp_type_option) = MESSAGE_RDP_RESPONSE_PDU;
  bytes response = 1;
}

message ConnectionActivated {
  option (tdp_type_option) = MESSAGE_CONNECTION_ACTIVATED;
  uint32 io_channel_activated = 1;
  uint32 user_channel_id = 2;
  uint32 screen_width = 3;
  uint32 screen_height = 4;
}

message SyncKeys {
  option (tdp_type_option) = MESSAGE_SYNC_KEYS;
  bool scroll_lock_pressed = 1;
  bool num_lock_state = 2;
  bool caps_lock_state = 3;
  bool kana_lock_state = 4;
}

message MouseMove {
  option (tdp_type_option) = MESSAGE_MOUSE_MOVE;
  uint32 x = 1;
  uint32 y = 2;
}

enum MouseButtonType {
  MOUSE_BUTTON_TYPE_UNKNOWN = 0;
  MOUSE_BUTTON_TYPE_LEFT = 1;
  MOUSE_BUTTON_TYPE_MIDDLE = 2;
  MOUSE_BUTTON_TYPE_RIGHT = 3;
}

message MouseButton {
  option (tdp_type_option) = MESSAGE_MOUSE_BUTTON;
  MouseButtonType button = 1;
  bool pressed = 2;
}

message KeyboardButton {
  option (tdp_type_option) = MESSAGE_KEYBOARD_BUTTON;
  uint32 key_code = 1;
  bool pressed = 2;
}

message ClientScreenSpec {
  option (tdp_type_option) = MESSAGE_CLIENT_SCREEN_SPEC;
  uint32 width = 1;
  uint32 height = 2;
}

message ClientUsername {
  option (tdp_type_option) = MESSAGE_CLIENT_USERNAME;
  string username = 1;
}

message Error {
  option (tdp_type_option) = MESSAGE_ERROR;
  string message = 1;
}

enum AlertSeverity {
  ALERT_SEVERITY_INFO = 0;
  ALERT_SEVERITY_WARNING = 1;
  ALERT_SEVERITY_ERROR = 2;
}

message Alert {
  option (tdp_type_option) = MESSAGE_ALERT;
  string message = 1;
  AlertSeverity severity = 2;
}

enum MouseWheelAxis {
  MOUSE_WHEEL_AXIS_UNKNOWN = 0;
  MOUSE_WHEEL_AXIS_VERTICAL = 1;
  MOUSE_WHEEL_AXIS_HORIZONTAL = 2;
}

message MouseWheel {
  option (tdp_type_option) = MESSAGE_MOUSE_WHEEL;
  MouseWheelAxis axis = 1;
  uint32 delta = 2;
}

message ClipboardData {
  option (tdp_type_option) = MESSAGE_CLIPBOARD_DATA;
  bytes data = 1;
}

enum MFAType {
  MFA_TYPE_UNKNOWN = 0;
  MFA_TYPE_WEBAUTHN = 1;
  MFA_TYPE_U2F = 2;
}

message MFA {
  option (tdp_type_option) = MESSAGE_MFA;
  MFAType type = 1;
  proto.MFAAuthenticateChallenge challenge = 2;
  proto.MFAAuthenticateResponse authentication_response = 3;
}

message SharedDirectoryAnnounce {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_ANNOUNCE;
  uint32 directory_id = 1;
  string name = 2;
}

message SharedDirectoryAcknowledge {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_ACKNOWLEDGE;
  uint32 directory_id = 1;
  uint32 error_code = 2;
}

message SharedDirectoryInfoRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_INFO_REQUEST;
  uint32 directory_id = 1;
  uint32 completion_id = 2;
  string path = 3;
}

message FileSystemObject {
  uint64 last_modified = 1;
  uint64 size = 2;
  uint32 file_type = 3;
  bool is_empty = 4;
  string path = 5;
}

message SharedDirectoryInfoResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_INFO_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
  FileSystemObject fso = 3;
}

message SharedDirectoryCreateRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_CREATE_REQUEST;
  uint32 completion_id = 1;
  uint32 directory_id = 2;
  uint32 file_type = 3;
  string path = 4;
}

message SharedDirectoryCreateResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_CREATE_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
  FileSystemObject fso = 3;
}

message SharedDirectoryDeleteRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_DELETE_REQUEST;
  uint32 directory_id = 1;
  uint32 completion_id = 2;
  string path = 3;
}

message SharedDirectoryDeleteResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_DELETE_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
}

message SharedDirectoryListRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_LIST_REQUEST;
  uint32 directory_id = 1;
  uint32 completion_id = 2;
  string path = 3;
}

message SharedDirectoryListResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_LIST_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
  repeated FileSystemObject fso_list = 3;
}

message SharedDirectoryReadRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_READ_REQUEST;
  uint32 completion_id = 1;
  uint32 directory_id = 2;
  string path = 3;
  uint64 offset = 4;
  uint32 length = 5;
}

message SharedDirectoryReadResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_READ_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
  uint32 read_data_length = 3;
  bytes read_data = 4;
}

message SharedDirectoryWriteRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_WRITE_REQUEST;
  uint32 completion_id = 1;
  uint32 directory_id = 2;
  uint64 offset = 3;
  string path = 4;
  uint32 write_data_length = 5;
  bytes write_data = 6;
}

message SharedDirectoryWriteResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_WRITE_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
  uint32 bytes_written = 3;
}

message SharedDirectoryMoveRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_MOVE_REQUEST;
  uint32 completion_id = 1;
  uint32 directory_id = 2;
  string original_path = 3;
  string new_path = 4;
}

message SharedDirectoryMoveResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_MOVE_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
}

message SharedDirectoryTruncateRequest {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_TRUNCATE_REQUEST;
  uint32 completion_id = 1;
  uint32 directory_id = 2;
  string path = 3;
  uint32 end_of_file = 4;
}

message SharedDirectoryTruncateResponse {
  option (tdp_type_option) = MESSAGE_SHARED_DIRECTORY_TRUNCATE_RESPONSE;
  uint32 completion_id = 1;
  uint32 error_code = 2;
}

message LatencyStats {
  option (tdp_type_option) = MESSAGE_LATENCY_STATS;
  uint32 client_latency = 1;
  uint32 server_latency = 2;
}

message Ping {
  option (tdp_type_option) = MESSAGE_PING;
  // UUID is used to correlate message send by proxy and received from the Windows Desktop Service
  bytes uuid = 1;
}

message ClientKeyboardLayout {
  option (tdp_type_option) = MESSAGE_CLIENT_KEYBOARD_LAYOUT;
  uint32 keyboard_layout = 1;
}

```