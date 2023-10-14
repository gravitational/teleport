---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0150 - SSH connection resumption

## Required Approvers

* Engineering: TBD
* Security: @jentfoo

## What

This RFD defines a mechanism by which connections to the Teleport SSH service can be wrapped in a protocol that allows for disconnections and reconnections of the outer bytestream transport without affecting the actual SSH bytestream.

## Why

A major pain point of the user experience of Teleport Server Access is the fact that SSH connections are forwarded through at least one Proxy server (sometimes more, when trusted clusters or proxy peering are in play), and thus can be killed when components other than the actual server need to be restarted because of upgrades, or have some fault. If the actual bytestream can be resumed for some time after losing connectivity, we can avoid this issue and even improve the user experience beyond what is possible with a direct TCP connection to a traditional SSH server.

A secondary pain point compared to the user experience with traditional SSH servers is that we also kill sessions when the Teleport SSH agent needs to be upgraded. Connection resumption offers a path to fix that as well, but it's out of scope for this RFD, and will only be touched upon briefly.

## Goals and constraints

The first broad goal is "users of Server Access should not be able to notice that the Proxy service was restarted while they have a shell open". It wouldn't be feasible to change Teleport's connectivity model in such a way that TCP connections handled by the Proxy service can persist a Proxy restart, so that goal can be also reworded as "Server Access shells should be able to survive a connection drop".

To do this, we could augment the existing session joining mechanism to support replaying the terminal output from an exact byte offset, then whenever the connection between `tsh` and the server is disrupted, we resume the session. This approach would require changes in how we record sessions (as we'd need to keep track of the input by participant to each session, and how said input was merged together in the session) and has quite a few points of failure (the session might need to be replayed through the auth server in `node-sync` recording mode, for instance, which requires the auth server and the session recording backend to be available). Moreover, this would take care of interactive shells but not command execution (which isn't recorded or joinable) or port forwarding - goal 2.

A different approach, as implemented by the Microsoft [Dev Tunnels SSH Library](https://github.com/microsoft/dev-tunnels-ssh) as the [session-reconnect@microsoft.com](https://github.com/microsoft/dev-tunnels-ssh/blob/main/ProtocolExtensions.md#extension-session-reconnection) extension, involves resuming the bidirectional stream of SSH packets after re-authenticating to the same server. This, unfortunately, would require the SSH library we use ([`golang.org/x/crypto/ssh`](https://golang.org/x/crypto/ssh)) to support this, meaning that we would need to fork the library, implement this nonstandard and fairly obscure extension, and then maintain our fork in the hopes that it would eventually get picked up upstream. In addition, this requires client support, meaning that we wouldn't be able to resume sessions opened via openssh or PuTTY (without some really nasty tricks like running a SSH server in `tsh proxy ssh`) - goal 3.

This leads to the final approach, which is what we propose in this RFD: resuming the actual bytestream used by the SSH connection.

Since we need some wire protocol anyway for the last leg of a connection to a direct dial node - we don't want resumption to be limited to tunnels, goal 4 - we might as well use the very same wire protocol directly from the client to the server. We initially thought of just relying on the tunneling protocol to let us carry the data stream as usual plus some needed metadata, but the way we currently do client-proxy and proxy-proxy connection forwarding (with frames sent over gRPC) doesn't actually allow for that without essentially employing the same multiplexing trickery that can be done end-to-end to begin with.

Such protocol doesn't have to be SSH-specific, but at least some initial handshake must be, as we intend to support direct dial servers and we must not break compatibility with existing clients or servers - goal 0.

Going over the goals again, we end up with this list:

0. Do not break compatibility with existing servers or clients.

1. Users of server access should not be able to notice that the Proxy service was restarted while they have a shell open, or Server Access shells should be able to survive a connection drop.

1. Command execution and port forwarding should be able to survive a connection drop.

1. OpenSSH compatibility via `tsh proxy ssh` should be able to survive a connection drop.

1. Resilience against connection drops should include SSH servers running in "direct dial" mode, not just servers connected to a Teleport cluster via reverse tunnel.

This RFD's proposal to fulfill these goals is an end-to-end protocol, whose early handshake degrades gracefully into a standard SSH version handshake, that establishes a single uninterrupted bytestream across separate fallible bytestream connections (both connections coming through a reverse tunnel, or direct TCP connections). This approach allows for session persistence across not only restarts of the Proxy, but general loss of end-to-end connectivity.

## Description of the protocol

In the following description, all numbers are sent as variable length unsigned integers, as per [Protobuf specs](https://protobuf.dev/programming-guides/encoding/#varints), or as implemented by the Go standard library in [`encoding/binary`](https://pkg.go.dev/encoding/binary).

### Version exchange

The client blindly sends the bytestring `SSH-2.0-`. As this is 8 bytes, it's enough to trigger the protocol multiplexer (which unfortunately blocks until the client sends 8 bytes), which will direct the connection to the resumption protocol handler, which will send the bytestring `teleport-resume-v0\r\n`. If the client receives such a string from the server, it will then send `\x00teleport-resume-v0` (the NUL byte is explicitly disallowed by the SSH version handshake, see [RFC 4253 section 4.2](https://datatracker.ietf.org/doc/html/rfc4253#section-4.2)), and both ends are in agreement about the use of the "resumption protocol v0" from that point of the connection.

If the first bytes sent by the server don't match `teleport-resume-v0\r\n` then the client must assume that the server is a regular SSH server, and thus will abort the resumption protocol version exchange, and begin a normal SSH client connection; as `SSH-2.0-` was already sent in the connection, eight bytes must be skipped, assuming that the SSH client will send the same. Such an assumption is valid, as the client side of an SSH connection must send `SSH-2.0-<softwareversion>` at the very beginning of the connection.

If the first bytes sent by the client don't match `SSH-2.0-\x00teleport-resume-v0` then the server must assume that the client is a regular SSH client, and thus will abort the resumption protocol version exchange, and begin a normal SSH server connection. No special handling of the connection is required here, as the server side of the exchange is allowed to send CRLF-terminated lines of UTF-8 text that don't begin with `SSH-` and that don't contribute to the SSH connection state.

### Handshake (new connection)

The clients sends `\x00` (likely together with the previous data), and the server generates a _resumption token_ out of 16 random bytes, but with the highest bit of the first byte set. The server sends this resumption token, then its length-prefixed host ID (which is almost always a UUID in string form, but can also be an EC2 instance ID of varying length). The host ID of the server should be used in later reconnection attempts to ensure that the correct server is receiving the new connection, since the first connection might've been to a hostname or public address and thus might eventually resolve to a different server. The size of the receive window is assumed to be 16 MiB for either peer.

### Handshake (reconnection)

The client sends the previously-received _resumption token_, to which the server replies with a `\x01` to confirm that the token is for a connection that can be resumed or `\x00` if the token is unknown (or for a connection that's expired) followed by a connection close. In the latter case, the client should stop further reconnection attempts and treat the connection as closed by the remote side. The extra byte to confirm or deny the validity of the token is necessary to signal to the client that the connection should be treated as invalid - closing the connection will just trigger a new reconnection attempt.

After the acknowledgement, both sides will send a count of how many bytes they have received in the connection, and the current size of the receive window.

### Data exchange

After both sides agree that the connection can proceed, data is exchanged in frames consisting of an integer that indicates how many bytes of data have been successfully received and consumed (and will thus increase the window size known to the peer), then a length-prefixed chunk of data being sent. The amount of data must not exceed the size of the receive window of the peer. If either side cannot proceed from the exchanged resume point, or if either side wants to explicitly close the connection, a value of `0xffff_ffff_ffff_ffff` should be sent (as the receive window increase). Upon receiving such value, the connection should be considered closed by the remote side.

## Security

## Future development
