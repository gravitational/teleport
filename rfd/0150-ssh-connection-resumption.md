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

1. Resilience against connection drops should include SSH servers running in "direct dial" mode - i.e. Teleport SSH service agents that receive TCP connections from the Teleport proxies - not just servers connected to a Teleport cluster via reverse tunnel.

This RFD's proposal to fulfill these goals is a mechanism to establish a single uninterrupted bytestream across separate fallible bytestream connections  (both connections coming through a reverse tunnel, or direct TCP connections), only requiring end-to-end connectivity between a compliant client and server, whose wire protocol has a handshake that can degrade gracefully into a standard SSH version handshake. This approach allows for session persistence across not only restarts of the Proxy, but general loss of end-to-end connectivity.

## Description

A resumable connection is a userland implementation of a bytestream network connection (in Go, a `net.Conn`) that exchanges data with a remote resumable connection across multiple underlying connections (at most one at a time), keeping a record of the data that was sent and acknowledging the data that was received, so that it's possible to open a new underlying connection and resend all the data that was not received by the other end of the connection (because of some fault in the previous underlying connection, for example).

The wire protocol for the underlying connection is described in the following paragraphs.

### Version exchange

The client sends the bytestring `SSH-2.0-`, which is always part of the preamble of the client side of a SSH connection (see [RFC 4253 section 4.2](https://datatracker.ietf.org/doc/html/rfc4253#section-4.2)). As this is 8 bytes, it's enough to trigger the Teleport protocol multiplexer (which unfortunately blocks until the client sends 8 bytes), which will direct the connection to the resumption protocol handler.

The server generates a _resumption token_ to be used later if the connection leads to a new resumable connection; the resumption token is a random 8-byte bytestring. It will then send the bytestring `teleport-resume-v0 <hex-encoded resumption token> <Teleport host ID>\r\nSSH-2.0-<standard SSH version line>\r\n` (the latter part is going to be `SSH-2.0-Teleport` in our current servers, but the client shouldn't assume so). If the client receives such a string from the server, it will then send `\x00teleport-resume-v0` (the NUL is explicitly disallowed by the SSH version handshake) and both ends are in agreement about the use of the "resumption protocol v0" from that point of the connection.

If the first bytes sent by the server don't match the bytestring mentioned above then the client must assume that the server is a regular SSH server, and thus will abort the resumption protocol version exchange, and begin a normal SSH client connection; as `SSH-2.0-` was already sent in the connection, eight bytes should be skipped from the application side.

If the first bytes sent by the client don't match `SSH-2.0-\x00teleport-resume-v0` then the server must assume that the client is a regular SSH client, and thus will abort the resumption protocol version exchange, and begin a normal SSH server connection; as the SSH version string was already sent in the connection, an appropriate amount of data should be skipped from the application side. This either requires knowing what the version string for the server will be, or requires running the server side of the connection until the server-side version string is written by the server.

### Handshake (new connection)

The client sends the resumption token sent as part of the version exchange; this signals that the connection is brand new, that the server should be assumed to have sent `SSH-2.0-<standard SSH version line>\r\n`, and that those bytes have been acknowledged by the client. Data exchange can immediately follow.

### Handshake (reconnection)

The client sends the previously-received _resumption token_ followed by the count of how many bytes it has received from the server; the server will reply with `0xffff_ffff_ffff_ffff` (i.e. eight `\xff` bytes) to signal that the connection should be closed (because the token is unknown or because the connection was already closed - the two things are likely indistinguishable), or with its own count of how many bytes it has received from the client. In the former case, the client should stop further reconnection attempts and treat the connection as closed by the remote side.

These counts are sent as

### Data exchange

After both sides agree that the connection can proceed, data is exchanged in frames consisting of a variable-sized unsigned integer count of how many bytes are being acknowledged as successfully received (since the previous acknowledgement, or since the beginning of the connection, to reduce the size of the variable-length integer), then a length-prefixed chunk of data; the length is transmitted as a variable-sized unsigned integer. The size of the chunk must not exceed a size of 128KiB. If either side cannot proceed from the exchanged resume point, or if either side wants to explicitly close the connection, a value of `0xffff_ffff_ffff_ffff` should be sent (as the acknowledgement). Upon receiving such value, the connection should be considered closed by the remote side and the resumption token invalidated.

### State

Both sides need to maintain a log of data that has been generated by the application and potentially sent over the wire, but hasn't yet been acknowledged by the peer. Such a buffer should be limited in size to avoid unconstrained resource consumption - from some early tests, a buffer size of 2MiB seems to be sufficient to maintain near full bandwidth when transferring data across a single SSH channel, compared to a regular SSH connection that doesn't make use of the resumption protocol. The protocol does not mandate the use of a buffer with a fixed maximum size, so other approaches can be used in the future if need be.

This design allows for keepalive frames containing no acknowledgement and no data, similarly to the protocol used in [TLS routing Ping](https://github.com/gravitational/teleport/blob/master/rfd/0081-tls-ping.md); a frame consisting of two NULs should be sent if no data has been otherwise sent for some _keepalive interval_ (30 seconds?), and the underlying connection should be closed and considered dead if no data has been received for two or three times that interval.

The server side of the connection is tasked with keeping track of the existing resumable connections - a similar role to the TCP stack in the OS - and should take care not to let dead connections accumulate if they're unused. A connection that has no underlying transport for longer than a _grace period_ - we have an idle timeout of 15 minutes for SSH connections, but we can pick something slightly shorter, around 5 minutes - should be closed and then cleaned up by the server. Closed connections (as a result of an explicit close by the application, or as a result of the peer sending a close signal) should likewise be cleaned up.

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

The connection limit for a given source address should be reduced upon receiving a connection but should not be increased again until the connection is properly cleaned up by the server or until a new underlying connection replaces the old one. This should be documented, as it might cause issues if connection limits are deliberately kept very low and some connection ends up being lost or not terminated correctly.

## Future development

### Other protocols

The same resumption mechanism could be employed for other protocols, since losing a long-lived database connection or a desktop session due to a Teleport proxy restart can be annoying. This can only be done whenever a client in our total control is at play, however, and not when the proxy is terminating connections (that would be pointless); that means using `tsh proxy db`, `tsh proxy kube` or some yet-to-exist desktop client for our Desktop Access sessions.

This specific bespoke wire protocol, however, might not be suited to wrap completely arbitrary bytestreams: the SSH protocol works particularly well with it, since its internal multiplexing makes it so that one side of the connection can generally make the assumption that the other side is capable of reading all the data that is about to be written - anything based on HTTP/2 will probably work just as well, since the internal multiplexing is similar.

### Persisting SSH connections across Teleport node restarts

We currently re-execute Teleport to handle various user bookkeeping tasks, but the SSH connection itself is handled by the single monolithic `teleport` process; in direct dial mode it would be possible for a child process to handle the incoming connection on its own (similarly to how OpenSSH does it), but since the connection could not outlive the main process handling the reverse tunnel connection in tunnel mode, this was never explored.

Connection resumption lets us do that, however; as long as any Teleport agent is running and can forward connections to the child processes, it would be possible for the SSH connection to survive and be used across a Teleport agent restart or upgrade.

### Proxy-terminated connection resumption

This proposal defines connection resumption from a resumption-aware client to a resumption-aware server, intending for the latter to be a Teleport SSH service agent reached directly from a user's machine. If the SSH connection is terminated and forwarded on a Teleport proxy, we have the option to allow connection resumption on both sides, if possible.

Connection resumption from the client to the proxy is only possible if we introduce a method for a user machine to get a connection to a specific proxy - this, at a minimum, requires proxy peering - and only helps against network problems between the user and the proxy, since a restart of the proxy that's doing the termination and forward will still cause the connection to be killed.

Connection resumption from the proxy to the node can be useful (there might be other hops between the proxy doing the SSH termination and the agent, because of proxy peering or trusted cluster connectivity) but still requires the node to be a Teleport SSH server, and the main reason why a SSH connection might be terminated at the Proxy would be if the node is a registered OpenSSH/agentless node, which is not going to support connection resumption anyway (unless we provide some sort of frontend for it). In the very specific case of a Teleport cluster configured in proxy recording mode and an agent connected over proxy peering or an agent in a leaf cluster we might see some benefit from proxy-node connection resumption.
