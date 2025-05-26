---
authors: Grzegorz Zdunek (grzegorz.zdunek@goteleport.com)
state: implemented (17.5)
---

# RFD 208 - Add ProxyWindowsDesktopSession API to Teleport Proxy Service

## Required Approvals

- Engineering: @zmb3 && (@rosstimothy || @espadolini)

## What

Introduce a new API in the Teleport Proxy Service to support streaming desktop sessions to Teleport Connect.

## Why

Remote desktop sessions in Teleport rely on the Teleport Desktop Protocol (TDP) for communication.
To support interactive sessions, a bidirectional stream must be established between the client and the Windows Desktop Service to send and receive TDP messages in real time.

## Details

In the Web UI, the TDP stream is transmitted over a cookie-authenticated WebSocket. 

However, Teleport Connect (which uses tsh under the hood) does not rely on cookies for authentication—instead, it uses client certificates.
To support this scenario, a new gRPC-based API called `ProxyWindowsDesktopSession` will be added to the `TransportService` in the Teleport Proxy. 

### Architecture

```
          gRPC bidirectional stream   
+-----+  (ProxyWindowsDesktopSession)   +---------------+     TCP connection     +-------------------------+
| tsh | <============================> |  Proxy Service | <====================> | Windows Desktop Service |
+-----+                                 +---------------+                        +-------------------------+
        <======================================================================>
                                    TLS connection (end-to-end)
```

`ProxyWindowsDesktopSession` won't transmit TDP messages directly.
Instead, it will tunnel a TLS connection over gRPC, which will then carry the TDP messages.

This design simplifies per-session MFA by allowing the client to present an MFA-verified certificate over a single TLS connection that terminates at the Windows Desktop Service.

### Session Initialization Flow

1. The connection is initiated by tsh, which opens the `ProxyWindowsDesktopSession` gRPC stream.
   * The first message sent by tsh must contain the `dial_target`, specifying the desktop name and target cluster. 
   * If the first message does not include the `dial_target`, the Proxy will immediately terminate the connection.
2. The Teleport Proxy uses the provided `dial_target` to dial the Desktop Service, following the logic in a Web UI handler.
   * The Proxy queries the cluster's desktops looking for the ones that match the `desktop_name` in `dial_target`.
   * The returned desktops are then mapped to desktop services reporting them.
   * The Proxy attempts to connect to these services in a randomized order until one succeeds.
3. Data tunneling starts.
   * Once a Desktop Service is successfully reached, the Proxy begins forwarding raw data between tsh and the Desktop Service.
   * Meanwhile, tsh initiates a TLS connection over the gRPC stream using its MFA-verified client certificate.
   * The TLS connection is established when the Desktop Service accepts the handshake.

Note that Proxy won't enforce RBAC checks. It will attempt to connect to the Windows Desktop Service even if the user doesn’t have access to the target desktop.
The Desktop Service will enforce access control and return an error if the user isn’t allowed.

### Drawbacks and considered alternatives

Tunneling a TLS connection over a gRPC stream introduces some overhead due to the added TLS metadata in each message. 
This could impact performance slightly, although manual testing did not show any noticeable latency.

A potential alternative design would terminate the client's TLS connection in the Proxy, then open a separate TLS connection to the Windows Desktop Service. 
This would require the Proxy to detect whether per-session MFA is needed, issue an MFA challenge to the client and verify the response.
However, this approach is not currently feasible, since Proxy has no straightforward way to determine whether per-session MFA is required 
for a given user—it doesn’t interact with the Auth Service using a user's identity.

### gRPC Definitions

#### Service definition

```protobuf
// Bidirectional stream for proxying Windows desktop sessions.
rpc ProxyWindowsDesktopSession(stream ProxyWindowsDesktopSessionRequest) returns (stream ProxyWindowsDesktopSessionResponse);
```

#### Message definitions

```protobuf
// Request message for a proxied Windows desktop session.
message ProxyWindowsDesktopSessionRequest {
   // A chunk of data from the connection. Can be nonempty even in the first message, but it's also legal for it to be empty.
   bytes data = 1;
   // Target cluster and desktop. Must be set in the first message and unset in subsequent messages.
   TargetWindowsDesktop dial_target = 2;
}

// Response message for a proxied Windows desktop session.
message ProxyWindowsDesktopSessionResponse {
   // A chunk of data from the connection. Can be empty (for example, to send a message
   // signaling a successful connection even if there's no data available in the connection).
   bytes data = 1;
}

// Identifies the destination desktop within a specific cluster.
message TargetWindowsDesktop {
   // Name of the desktop to connect to.
   string desktop_name = 1;
   // Name of the cluster the desktop belongs to.
   string cluster = 2;
}
```