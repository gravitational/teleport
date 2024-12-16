---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: implemented
---

# RFD 0165 - SSH Connection Hand-over

## Required Approvers

Engineering: (`@rosstimothy` || `@zmb3`) && (`@fspmarshall` || `@hugoShaka`)
Security: `@jentfoo`

## What

This RFD defines a mechanism to forward SSH connections between different Teleport SSH service processes running on the same data directory, as it happens during graceful restart. Furthermore, this RFD proposes to change the automatic upgrade behavior for the systemd unit upgrader from a hard systemd-managed restart to a graceful restart.

## Why

To allow SSH sessions to continue unimpeded (including automatic resumption) during Teleport upgrades.

## Details

The connection resumption protocol described in [RFD 0150][rfd0150] allows SSH connections from the client to a Teleport SSH server to be automatically resumed if the network path is severed at any of the places between the two; this facilitates the work of Teleport cluster administrators, since it makes SSH sessions survive a control plane upgrade. Presently, however, upgrading the Teleport agent results in either the Teleport process (and thus the session) being terminated after some timeout, or with the connection being handled entirely by the existing terminating Teleport process, with any attempt to resume the connection resulting in a failure, since the Teleport proxy will direct new connections to the new agent process that doesn't have a way to resume SSH connections handled by the former agent process.

To solve this problem we are going to add an IPC layer to allow Teleport agent processes running on the same machine (and using the same data directory) to forward connection resumption attempts between each other, so that the client can resume connections just by reaching the same host ID, even if the connection reaches a different Teleport process than the one handling the SSH connection.

[rfd0150]: https://github.com/gravitational/teleport/blob/master/rfd/0150-ssh-connection-resumption.md

### Connection registry

Each new resumption-enabled connection (with a given _resumption token_) will be associated with a listener in the hand-over connection registry, implemented, on UNIX platforms, as the directory `<datadir>/handover/`, where each connection is associated with a UNIX stream listener at `<datadir>/handover/<truncated hash of resumption token in url-safe base 64>`; the socket should be unlinked when the resumable connection is closed, and each Teleport process will, during startup, clean up sockets potentially left over by Teleport processes that were terminated before they could clean up after themselves.

The `handover` directory will be created with permissions such that only the owner of the directory (and the superuser) will be allowed to connect to sockets in it. This is important, since allowing arbitrary connections to hand-over sockets will result in potentially untrusted client IP propagation and/or unwanted disruption to existing connections.

The name of the socket should be a hash of the resumption token, to harden this mechanism against potential local information leaks. It's sufficient to use a regular hash function (rather than something keyed by a secret like a HMAC) because the resumption token is a 128-bit value picked uniformly, and thus unfeasible to brute force directly - we'll use SHA-256 truncated to 128 bits, because UNIX domain sockets have a path length limitation of 108 bytes on Linux and 104 on macOS and other BSDs, and dedicating well over a third of that just for the socket name runs the risk of exceeding the path length limit if the path to the Teleport data dir is long enough. 128 bits still provide more than enough collision resistance, and result in an unpadded base64 encoding length of 22 bytes, so 32 bytes (including `/handover/`) over the length of the path to the Teleport data directory.

### Hand-over protocol

After reading the resumption token from the client, the server will first check if the token is associated with a local connection. If found, connection resumption proceeds as usual. If not, the server will attempt to connect to the hand-over listener for that token. If successful, it will send the client's IP address over the connection (as a 16 byte IPv6 address, mapping the IPv4 to IPv6 if needed), then it will start bidirectionally copying data between the client and the hand-over connection. If the connection to the hand-over listener is not successful because the socket doesn't exist, then the server will reply with a "connection not found" tag to the client.

The server-side of the hand-over connection will first read the remote IP address, then proceed operating with the connection as if a client with the received remote IP address had just attempted to resume a connection with the token associated to the hand-over connection: following the connection resumption protocol, it will reply with a "success" message tag if the remote address matches the one of the resumable connection, a "bad address" tag otherwise, or a "not found" tag if the connection had been closed in the meantime.

### Graceful restart by default

With this change we're able to treat concurrent instances of Teleport running on the same data directory (and thus "hand-over registry") almost interchangeably, with regards to SSH connections; that's effectively what happens during a graceful restart initiated by SIGHUP: Teleport executes a new Teleport process launched from disk, with the same configuration, passing it a copy of all the listeners (if any) used by the configured services, then waits for it to signal that Teleport has started successfully, then waits for existing connections to complete just like during a graceful shutdown. Similarly, SIGUSR2 process forking (which shouldn't be used) and internal process restart due to host CA rotation (which we should get rid of) also end up running multiple copies of Teleport with the same configuration and credentials. SSH connection hand-over allows connection resumption to work in all such cases.

The current behavior of the `systemd-unit-upgrader` script is to initiate a systemd service restart after upgrading the Teleport package on disk. A systemd service restart (by default) sends a termination signal to the "main process" of the unit, then after it exits (or after a timeout) the whole cgroup of the service is killed, and a new copy of the service is started. We're going to change the unit upgrader to only restart in the presence of upgrades marked "critical" (as those generally indicate security-sensitive upgrades, and we should not risk running vulnerable binaries for the sake of not having to reopen connections), and to reload the service (sending Teleport a SIGHUP) in other cases.

In addition, we will change the behavior of SIGHUP to have a generous timeout before the old process exits (tentatively 30 hours, matching the longest validity for login credentials), to avoid accidentally accumulating too many Teleport processes. With this change, a (non-critical) Teleport upgrade should result in unaffected SSH sessions, as well as allowing sessions for other protocols to run until completion (provided that they last less than 30 hours after the ). An optional flag can be set in the configuration for `systemd-unit-upgrader` to disable reloading and always do a clean restart.

### Alternatives

The initial plan for this feature involved a fairly significant restructuring of the SSH service code, to run each connection in a separate process. This results in fairly clean overall behavior (we could move each connection handling process in a separate cgroup, like OpenSSH does) but is a major undertaking, would likely result in increased resource consumption (Teleport is a fairly big binary, all other things notwithstanding), and would require each connection to either have its own cluster API client or some scheme to share API clients between different processes; graceful upgrading is a bit messier but is already implemented and is usable right now without any code changes.

On Linux, the hand-over registry could make use of the abstract socket namespace: it doesn't need any cleanup, and it doesn't touch the disk, but we're on our own for permission checking (as opposed to just relying on file-system ACLs) and it's not portable to macOS (or other UNIX flavors, in the future).
