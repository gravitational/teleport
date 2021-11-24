---
authors: David Boslee (david@goteleport.com), Naji Obeid (naji@goteleport.com)
state: draft
---

# RFD XX - Proxy Tunnel Service

## What
This document describes an API that enables a proxy to dial the nodes connected to a neighboring proxy. This will allow node agents to connect to one proxy and be reachable through any other proxy in the cluster. This is one component of a larger design to make teleport more cloud friendly. See [RFD 48](https://github.com/gravitational/teleport/blob/master/rfd/0048-war-dialler-node-tracker.md) for more details.

## Why
The main goal is to remove the need for a node agent to create a reverse tunnel to every proxy. The problems caused by this behavior are outlined [here](https://github.com/gravitational/teleport/blob/master/rfd/0048-war-dialler-node-tracker.md#why).

## Details

### Proxy API

The following gRPC service will be added to proxy servers:

```protobuf
syntax = "proto3";

package api;

service TunnelService { rpc DialNode(stream Frame) returns (stream Frame); }

// Frame wraps different message types to be sent over a stream.
message Frame {
   oneof Message {
       DialRequest DialRequest = 1;
       Data Data = 2;
   }
}

// DialRequest contains details for connecting to a node.
message DialRequest {
   // ServerID is the {UUID}.{ClusterName} of a node.
   string ServerID = 1;
   // ConnType is the type of connection requested.
   // Examples: “node” or “app”
   string ConnType = 2;
   // Source is the source address of the connection.
   Addr Source = 3;
   // Destination is the destination address of the connection.
   Addr Destination = 4;
}

message Addr {
   // Network is the name of the network transport.
   string Network = 1;
   // String is the string form of the address.
   string String = 2;
}

// Data contains the raw bytes of a connection.
message Data { bytes Bytes = 1; }
```

### How it works

The following diagram shows a user connecting to a proxy, proxy1, and trying to reach a node connected to another proxy, proxy2. Using the DialNode rpc, proxy1 can create a bidirectional stream to the node through proxy2.
```
┌──────┐                       ┌──────┐
|client|──────connection──────>|proxy1|
└──────┘                       └──╥───┘
                                  ║
                             grpc stream
                                  ║
  ┌────┐                       ┌──╨───┐
  |node|────reverse-tunnel────>|proxy2|
  └────┘                       └──────┘
```

A call to the DialNode rpc will send an initial frame containing a `DialRequest`. All subsequent frames should contain `Data` messages. An appropriate error will be returned if the dial request fails for some reason.

To avoid duplicate work the user proxy will handle all typical proxy side logic like session recording. While the node proxy will bypass this and forward the connection directly to the node.

The DialNode rpc will be wrapped with a client library to return a net.Conn when called. This abstraction allows teleport to treat any underlying transport the same, whether it be a direct dial to the node, a reverse tunnel connected to the user proxy, or a connection over the DialNode rpc.

```go
type NodeTunnelClient interface {
    DialNode(
        ctx context.Context,
        proxyAddress string,
        src net.Addr,
        dst net.Addr,
        serverID string,
        connType string,
    ) (net.Conn, error)
}
```

### Security
The api will use mTLS to ensure that only other proxies are able to connect. This is done by checking certificates for the build-in role “Proxy”. This will prevent users from connecting to the service directly and bypassing proxy side logic.

### API Clients
Each proxy will need to manage multiple grpc clients, one to each neighboring proxy. These will be created as needed, or in other words the first time `DialNode` is called for that specific proxy. Once a client is created it will be reused for any future requests to the same neighboring proxy.

Testing should be done to ensure that a single grpc connection does not become a bottleneck for many concurrent streams. Some comments on [grpc#2412](https://github.com/grpc/grpc-go/issues/2412) could be useful. If this does become an issue additional connections to the same proxy will need to be created.

In dynamic environments where proxies are added and removed, grpc clients will need to be cleaned up periodically to avoid memory leaks. This can be implemented using [grpc connectivity states](https://github.com/grpc/grpc/blob/master/doc/connectivity-semantics-and-api.md). If a client connection enters a bad state for some period of time it can be cleaned up.

### Reverse Tunnel Agent Pool
Changes to the reverse tunnel agent and agent pool are required to support this design. The existing implementation which connects to all proxies will be named `MeshPool` and the new implementation which connects to a single proxy will be named `SinglePointPool`. Both will implement the following interface:
```go
type AgentPool interface {
    // Start begins creating agents and blocks until the pool is stopped or
    // the context is canceled or
    Start(ctx context.Context) error
    // Wait blocks until the agent pool is stopped or the context is canceled.
    Wait(ctx context.Context) error
    // Stop stops all agents in the pool and cleans up resources.
    Stop(ctx context.Context) error
}
```

The SinglePointAgent can take advantage of connection rotation and improved jitter/backoffs to help further mitigate the thundering herd problem. Node agents will reconnect periodically to take advantage of graceful shutdown periods at proxies and load balancers. This reconnect period must have jitter as well to distribute the reconnects evenly over time. Without reconnects if a load balancer or proxy were to shutdown many nodes would reconnect at the same time.

During connection rotation, we need to avoid closing open sessions and avoid having the node be unreachable. First the agent must create a new connection before closing the old connection. Next the agent will send a ssh request over the old connection to signal the connection is entering a shutdown state. Proxies should not send new sessions to connection in the shutdown state. Finally when all existing sessions have closed the old connection can be closed.
