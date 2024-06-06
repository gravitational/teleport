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
- There is no easy way to implement global ratelimits and backoffs across all
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

The Unix socket will implement a simple protocol:

1. Upon connection, the client will send a message indicating the target.
  a. The message will be encoded in JSON. This will allow more fields to be 
    added in the future without breaking compatability.
  b. The `host_port` field will be a string in the format `host:port`.
2. The server will establish a connection to the target, and then begin
   forwarding data between the local connection and the target.

Also considered was implementing the SOCKS5 protocol. Whilst this is more 
standardized, it is more complicated than necessary. In addition, it seems like
there's not many tools in the wild that can connect to a SOCKS server over a
unix domain socket, so any advantage gained by the use of a standard protocol
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
- Rust executes faster.
- Tuning Go's GC behaviour requires also using `env` as part of the
  ProxyCommand. This is not necessary with Rust.

For this binary, we will use Rust.

### Initial Scope

For the initial version, we must include:

- The long-lived proxying background service within `tbot`
- The new lightweight client binary with support for ProxyUseFdPass.
- A configuration option to control the maximum number of proxied connections.
  - When this maximum is reached, the service will wait until the number of
    active connections drops below the maximum.
  - This will allow a sane limit to be provided to prevent the long-lived `tbot`
    process from being overwhelmed.
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

#### Upstream Connection Pooling

The initial version will not share the upstream connections to the Teleport
Proxy.

Sharing upstream connections will allow us to reduce the number of connections
and reduce the latency and overhead associated with establishing a new
connection.

This work is deferred as it is complex and introduces risks. We should first
ensure that the initial version is stable and to take measurements which can
inform the design of the connection pooling.

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
    # The maximum number of connections that can be proxied at once.
    max_connections: 100
```

### Security

#### Unix Domain Socket is unauthenticated

The Unix Domain Socket will be unauthenticated, and this means that any process
which is able to connect to it will be able to establish upstream connections.

Fortunately, existing Linux permissions mechanisms can be used to restrict 
access to the UDS and this is similar to how we handle the credentials
output by `tbot` today.

We should ensure we document the best practices to secure the UDS.