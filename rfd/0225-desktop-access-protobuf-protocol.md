---
authors: Ryan Hammonds (rhammonds@goteleport.com)
state: draft
---

# RFD 225 - Desktop Access - Desktop Access Protobufs

## Required Approvals

- Engineering: @zmb3 && (@probakowski || @danielashare)

## What
Re-define Teleport Desktop Protocol (TDP) to make use of protocol buffers (TDPB). 

## Why
The motivation to revise TDP arises from two key shortcomings:
- Lack of message framing
- Message serialization/deserialization lacks forward and backwards compatibility

TDP does not define any mechanism for the framing of messages. Message boundaries are
determined implicitly by detecting the message type from the first byte received, then
deserializing the remainder of the message according to that message's schema. This means that:
- Encountering an unknown message type is an unrecoverable error since the implementation
    cannot possibly know how many remaining bytes to discard from the stream.
- It is impossible to change the schema of an existing message because older or newer
    peers with different schema definitions will incorrectly determine message boundaries, 
    resulting in misalignment within the stream.

Furthermore, even in the absence of framing concerns, individual messages schemas cannot be changed due to TDP's inflexible binary format. The omition or addition of a single field will result in decoding errors between peers running different TDP versions.


## Why Protocol Buffers?
#### Ease of conversion
RFD 37 defines the following types for use in TDP messages:
- `byte`
- `uint32`
- `int64`
- `string` (UTF-8 encoded text represented as `[]byte`)
- `[]byte` (variable length byte array)

Existing TDP messages using these types can be easily represented as protocol buffer messages, with the exception of `byte` which is typically used to represent boolean values anyhow.

#### Backward and Forward Compatibility
By following protocol buffer best practices, we get forward and backward compatibility "for free" at least with respect to serialization/deserialization of individual messages. This is one of the main strengths of protocol buffers.

#### Familiarity
Protocol buffers are used extensively at Teleport with existing automation for both Go and Typescript code generation.

#### Easy Framing
Protocol buffers are not self-delimiting, so framing concerns must still be addressed. However, this can be easily handled by wrapping all TDP protobuf messages in an envelope message. 
```
enum MessageType { ... }

message TDPFrame {
    MessageType type = 1;
    bytes message = 2;
}
```

With invariants:
- All TDP Protobuf messages must be wrapped in the TDPEnvelope before transmission
- The `TDPFrame` message structure cannot change

The envelope message takes care of our framing concerns. Unknown messages can be safely detected and discarded.

## Protocol Specification Updates
Unless noted otherwise in this section, all existing TDP messages will simply be modeled as equivalent protocol buffer message representations. The behavior of the protocol will be largely the same aside from the updates below.

### Protocol Selection via ALPN
 The current TDP specification provides no handshake mechanism that we can use to upgrade from TDP to TDPB. Instead, we'll utilize ALPN to attempt to negotiate the use of TDPB. Modern Desktop agents will successfully negotiate `teleport-tdpb-1.0`, while older agents will fail negotiation entirely. This will signal the Teleport Proxy to use legacy TDP messages for the connection.

 ALPN may also be useful if we ever need to make breaking changes to the protocol, such as changes to the envelope message structure.

### Translation/Compatibility layer
Once the Teleport Proxy is updated, the websocket connection between browser client and Proxy will utilize TDPB. However, until legacy Desktop agents are out of support, TDPB must maintain backwards compatibility with TDP. This can be achieved by implementing a simple translation layer that runs on the Teleport Proxy. The outcome of the ALPN exchange will determine the need for this translation layer on a per-connection basis. 

```
                                                                    
                                                       ┌─────────┐  
    ┌─────────┐                                        │ Desktop │  
    │ Browser │    TDPB                                │ Agent   │  
    │ Client  ├─────────────────┐                      │ (Legacy)│  
    └─────────┘                 │                      └────▲────┘  
                              ┌─▼───────┐    TDP            │       
                              │         ┼───────────────────┘       
                              │  Proxy  │                           
                              │         ┼───────────────────┐       
    ┌─────────┐               └─▲───────┘    TDPB           │       
    │ Browser │    TDPB         │                      ┌────▼────┐  
    │ Client  ┼─────────────────┘                      │ Desktop │  
    └─────────┘                                        │ Agent   │  
                                                       │ (Modern)│  
                                                       └─────────┘  
                                                                    
```

### New Connection Greeting
New Client and server hello messages will be exchanged at the start of the connection. 

```protobuf
message TDPClientHello {
    string username = 1;
    ClientScreenSpec screen_spec = 2;
    // Future initialization fields
    // or capability advertisements here
}

message TDPServerHello {
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


### Additional Message Updates

#### 2 - PNG frame and 27 - PNG Frame 2 
Messages 2 and 27, (PNG Frame and PNG Frame 2) will be consolidated into a single message. The optimization brought on by the PNG 2 mesage is obsoleted under protobufs.

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
The MFA message will no longer contain json. Instead, it will compose the existing `MFAAuthenticationChallenge` and `MFAAthenticateResponse` messages.



## Backwards Compatibility with Screen Recordings
RFD 48 defines a protocol buffer message `DesktopRecordingEvent` that captures a subset of TDP messages required for session playback.  We can simply add a new field, `ProtoMessage` to this message.

```protobuf
message DesktopRecordingEvent {
    // Metadata is a common event metadata
    Metadata Metadata = 1;

    // Message is the encoded TDP message. It is not marshaled to JSON format
    bytes Message = 2;

    // A TDPFrame message which contains
    // PNG, Screen Spec, ClipboardData, Mouse Move, and 
    // mouse button events
    bytes ProtoMessage = 3;

    // DelayMilliseconds is the delay in milliseconds from the start of the session
    int64 DelayMilliseconds = 3;
}
```

The existing session player will need a small update to handle desktop recording messages containing either a TDP message or TDPB message.