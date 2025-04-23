---
author: Noah Stride (noah@goteleport.com)
state: draft
---
 
# RFD 172 - Long-lived Local `tbot` SSH Proxy

## Required Approvers

- Engineering: @rosstimothy || @espadolini

## What

Implement an optional local long-lived SSH connection proxy as part of `tbot`.
This will be an optional feature that can be enabled via configuration and will
not replace the existing SSH features of `tbot`.

## Why

Today, SSH proxying with OpenSSH and Machine ID relies on a short-lived `tbot`
process being created for each SSH connection.

This has a few drawbacks:

- The `tbot` binary has a significant memory overhead, much of which is not
  necessary for proxying the SSH connection. When connecting to a large number
  of hosts, this overhead can become significant.
- A process per connection, with each process having a number of tasks, places
  the Linux scheduler under significant load.
- Each `tbot` process must perform a number of initial startup tasks
  individually, contributing to latency in establishing the SSH tunnel. Some of 
  these tasks could be shared, or their results cached.
- Monitoring a large number of short-lived `tbot` processes is difficult.
- There is no easy way to implement global rate limits and backoffs across all
  the short-lived `tbot` processes. This makes gracefully handling upstream
  outages problematic.

## Details

### Implementation

We will introduce a long-lived service in `tbot`: `ssh-local-proxy`.

Upon start, the service will write a SSH config to a configured destination.
This SSH config should be used by OpenSSH to leverage the service.

Once initialized, the service will listen on a Unix socket. Upon connection to this
socket, the client will write a short message containing the target host and
port. The service will then establish the connection, and then begin forwarding
data between the socket and the upstream connection.

The client will be configured as the `ProxyCommand`, and additionally
`ProxyUseFdpass` will be enabled. The `ProxyUseFdPass` allows the `ProxyCommand`
to return an FD via STDOUT using `sendmsg`, rather than the `ProxyCommand`
actively forwarding the connection. This means the client can exit after the
connection to the local proxy is established.

This solution means that whilst a process will be spun up for each SSH
connection, it will return after the connection is established. This reduces the
long-term overhead associated with each connection and the impact on the Linux
scheduler.

#### Protocol

A simple protocol will exist for interacting with the multiplexer:

1. The server will open a Unix domain socket named `v1.sock`.
2. Upon connection, the client will send a request indicating the target.
  a. The request will be encoded in JSON and be followed by a NUL (ASCII/Unicode 0) character.
  b. This JSON message will contain two fields:
    i. `host`: The target host.
    ii. `port`: The target host's port.
3. The server will establish a connection to the target, and then begin
   forwarding data between the local connection and the target.

Also considered was implementing the SOCKS5 protocol. Whilst this is more 
standardized, it is more complicated than necessary. In addition, it seems like
there's not many tools in the wild that can connect to a SOCKS server over a
UNIX domain socket, so any advantage gained by the use of a standard protocol
is not actually realized in practice.

#### Client Binary

As it stands today, the `tbot` binary has significant overhead associated with
it.

Due to this, we will introduce a new, extremely lightweight binary that will
act as the client.

The client will accept two arguments:

1. The path to the Unix socket.
2. The message to send to the `tbot` proxy service. This will have the host and
   port templated into it by OpenSSH with the `%h` and `%p` placeholders.

Comparing Rust and Go for this lightweight binary:

- Rust has a smaller binary size: ~400KiB vs ~1.8MiB.
- Rust has little to no overhead, whilst Go requires spinning up a runtime with multiple OS threads and a garbage collector before any user code gets to run.
- Tuning Go's GC behaviour requires also using `env` as part of the
  ProxyCommand. This is not necessary with Rust.

For this binary, we will use Rust.

#### Connection Re-use

The Go gRPC client is capable of multiplexing multiple streams over a single
connection, but using a single connection for a large number of streams is 
problematic for a few reasons:

- Contention of the upstream connection.
- TCP head of line blocking becomes more problematic as the amount of data being
  transferred over a single connection increases.
- The server enforces a maximum number of streams per connection.

To avoid this, a simple connection cycler will be implemented. After a number of
streams have been established, a new internal connection will be dialed. New
streams will use the new connection, and the old connection will be closed once
all streams using it have been closed.

### Initial Scope

For the initial version, we must include:

- The long-lived proxying background service within `tbot`
  - This will include a rudimentary connection cycler.
- The new lightweight client binary with support for ProxyUseFdPass.
- Prometheus metrics which exposes the health of the proxying.
  - Number of active proxied connections.
  - Number of attempts to establish proxied connections, the time this has
    taken, and the success status.
- OpenTelemetry trace spans at key points.
  - This will allow us to target future performance improvements.

### Future Scope

#### Caching

We can further improve performance by caching the results of operations such as
resolving the target host from labels.

This work is deferred as it is complex and introduces risks. Information from
OpenTelemetry tracing will allow us to target the most impactful areas at a 
later date.

#### Further improving re-use of connections

The initial version will use a rudimentary connection "cycler". This will cause
a new connection to be established after a certain number of downstream
connections have been established. It will then close the upstream connection
once all downstream connections using it have been closed.

This could be further improved by balancing incoming downstream connections
against a pool of upstream connections, because the upstream connections would
live longer and we'd avoid a scenario where an upstream connection remains open
but is only being utilized by one or two remaining downstream connections.

### UX

The user will configure `ssh-local-proxy` in the `tbot` configuration file:

```yaml
services:
  - type: ssh-local-proxy
    # Destination is where the ssh_config and UDS will be written to. It must be
    # a directory.
    destination:
      type: directory
      path: /opt/machine-id
```

### Security

#### Unix Domain Socket is unauthenticated

The Unix Domain Socket will be unauthenticated, and this means that any process
which is able to connect to it will be able to establish upstream connections.

Fortunately, existing Linux permissions mechanisms can be used to restrict 
access to the UDS and this is similar to how we handle the credentials
output by `tbot` today.

We should ensure we document the best practices to secure the UDS.
