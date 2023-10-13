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

## Security

## Future development
