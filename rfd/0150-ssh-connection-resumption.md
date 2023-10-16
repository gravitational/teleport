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

The client blindly sends the bytestring `SSH-2.0-`. As this is 8 bytes, it's enough to trigger the Teleport protocol multiplexer (which unfortunately blocks until the client sends 8 bytes), which will direct the connection to the resumption protocol handler, which will send the bytestring `teleport-resume-v0\r\n`. If the client receives such a string from the server, it will then send `\x00teleport-resume-v0` (the NUL is explicitly disallowed by the SSH version handshake, see [RFC 4253 section 4.2](https://datatracker.ietf.org/doc/html/rfc4253#section-4.2)), and both ends are in agreement about the use of the "resumption protocol v0" from that point of the connection.

If the first bytes sent by the server don't match `teleport-resume-v0\r\n` then the client must assume that the server is a regular SSH server, and thus will abort the resumption protocol version exchange, and begin a normal SSH client connection; as `SSH-2.0-` was already sent in the connection, eight bytes must be skipped, assuming that the SSH client will send the same. Such an assumption is valid, as the client side of an SSH connection must send `SSH-2.0-<softwareversion>` at the very beginning of the connection.

If the first bytes sent by the client don't match `SSH-2.0-\x00teleport-resume-v0` then the server must assume that the client is a regular SSH client, and thus will abort the resumption protocol version exchange, and begin a normal SSH server connection. No special handling of the connection is required here, as the server side of the exchange is allowed to send CRLF-terminated lines of UTF-8 text that don't begin with `SSH-` and that don't contribute to the SSH connection state.

### Handshake (new connection)

The clients sends `\x00` (likely together with the previous data), and the server generates a _resumption token_ out of 16 random bytes, but with the highest bit of the first byte set. The server sends this resumption token, then its length-prefixed host ID (which is almost always a UUID in string form, but can also be an EC2 instance ID of varying length). The host ID of the server should be used in later reconnection attempts to ensure that the correct server is receiving the new connection, since the first connection might've been to a hostname or public address and thus might eventually resolve to a different server.

### Handshake (reconnection)

The client sends the previously-received _resumption token_, to which the server replies with a `\x01` to confirm that the token is for a connection that can be resumed or `\x00` if the token is unknown (or for a connection that's expired) followed by a connection close. In the latter case, the client should stop further reconnection attempts and treat the connection as closed by the remote side. The extra byte to confirm or deny the validity of the token is necessary to signal to the client that the connection should be treated as invalid - closing the connection will just trigger a new reconnection attempt.

After the acknowledgement, both sides will send a count of how many bytes they have received in the connection.

### Data exchange

After both sides agree that the connection can proceed, data is exchanged in frames consisting of an integer count of how many bytes are being acknowledged as successfully received (since the previous acknowledgement, or since the beginning of the connection, to reduce the size of the variable-length integer), then a length-prefixed chunk of data. The size of the chunk must not exceed a size of 128KiB. If either side cannot proceed from the exchanged resume point, or if either side wants to explicitly close the connection, a value of `0xffff_ffff_ffff_ffff` should be sent (as the acknowledgement). Upon receiving such value, the connection should be considered closed by the remote side.

### State

Both sides need to maintain a log of data that has been generated by the application and potentially sent over the wire, but hasn't yet been acknowledged by the peer. Such a buffer should be limited in size to avoid unconstrained resource consumption - from some early tests, a buffer size of 2MiB seems to be sufficient to maintain near full bandwidth when transferring data across a single SSH channel, compared to a regular SSH connection that doesn't make use of the resumption protocol. The protocol does not mandate the use of a buffer with a fixed maximum size, so other approaches can be used in the future if need be.

This design allows for keepalive frames containing no acknowledgement and no data, similarly to the protocol used in [TLS routing Ping](https://github.com/gravitational/teleport/blob/master/rfd/0081-tls-ping.md); a frame consisting of two NULs should be sent if no data has been otherwise sent for some _keepalive interval_ (30 seconds?), and the underlying connection should be closed and considered dead if no data has been received for two or three times that interval.

The server side of the connection is tasked with keeping track of the existing resumable connections, and should take care not to let dead connections accumulate if they're unused. A connection that has no underlying transport for longer than a _grace period_ - we have an idle timeout of 15 minutes for SSH connections, but we can pick something slightly shorter, around 5 minutes - should be closed and then cleaned up by the server. Closed connections (as a result of a SSH error, or as a result of the peer sending a close signal) should likewise be cleaned up.

The client side of the connection is tasked with reconnecting to the server; it should do so whenever the underlying connection has a fault, and it should also do so periodically, so that it's possible to rotate the Teleport proxy services forwarding the connection gracefully and without the connection being impacted. A _reconnection interval_ of 3 minutes works well for Teleport Cloud clusters, as the Teleport proxy is forcibly terminated 5 minutes after beginning its shutdown (which happens after new proxies are already available for connection routing). Similar operational recommendations should be documented for on-prem Teleport clusters.

## Security

The security of the application bytestream tunneled through the resumption protocol is delegated to the protocol used by the application; in this case the application protocol is SSH, which guarantees that all data is encrypted and authenticated, with a secure key exchange. This prevents any attack on the bytestream from becoming an attack on the application data, and the worst that can happen in that regard is that the connection ends up erroring out due to a MAC failure.

Other concerns are strictly about the additional surface provided by the resumption feature itself, however, and will be discussed here.

### Source address information

We propagate the source address of connections entering the Teleport cluster, and the address is stored whenever it's relevant to do so in the audit log. We offer the ability to require that client connections have the same source address as the one used to obtain credentials ([IP pinning](https://goteleport.com/docs/access-controls/guides/ip-pinning/)). As connection resumption intrinsically allows the same connection to roam between different source IP addresses (as the user changes their network configuration, for instance by switching from their wired connection to a mobile hotspot), we shall enforce that the source address of the connection stays the same until the SSH authentication is completed and the user is confirmed to have authenticated with credentials that do not require IP pinning. It's possible that a connection loses its underlying transport before the SSH authentication is complete, and a change in source address at that point will cause the connection to fail; this is acceptable, as the user couldn't have used the connection for anything before authentication anyway.

### Protocol

The protocol in its current form is entirely cleartext; the intended transport for the protocol, however, is secure between the client and the proxy, it's secure between proxies, and it's secure from the proxy to tunneled nodes. An attacker that's able to read traffic between the proxy and direct dial nodes will be able to learn the resumption token, and - with the ability to open connections to the node - the attacker will be able to disrupt the connection. An attacker knowing about a resumption token doesn't have the ability to communicate with the client but, by repeatedly resuming a pre-authentication connection, it's possible for an attacker to take the connection from the intended user and use it as its own. Nothing, however, can be realistically gained from this.

By examining the acknowledgements in the data frames it's possible to gain slightly more timing and traffic information than it would otherwise be available. We could employ some mitigations at the resumption protocol level, or just rely on the mitigations already included in the SSH protocol and implementations.

### Resource exhaustion

It's possible to fill the replay buffer on the remote side of a resumable connection just by not acknowledging any received data; such memory consumption can only be achieved by tricking the remote side into sending the data (which is not very realistic pre-SSH authentication, but should be kept in mind if this ends up being used to resume other protocols), and can only really last until some timeout that closes the connection anyway. As a big replay buffer is only required for throughput, it's also possible to limit the buffer size until the SSH handshake is successful.

The connection limit for a given source address should be reduced upon receiving a connection but should not be increased again until the connection is properly cleaned up by the server or until a new underlying connection replaces the old one. This should be documented, as it might cause issues if connection limits are deliberately kept very low.

## Future development
