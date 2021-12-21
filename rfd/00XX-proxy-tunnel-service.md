---
authors: David Boslee (david@goteleport.com), Naji Obeid (naji@goteleport.com)
state: draft
---

# RFD XX - Proxy Tunnel Service

## What
This document describes an API that enables a proxy to dial the nodes connected to a neighboring proxy. This is an optional feature that will allow node agents to connect to one proxy and be reachable through any other proxy in the cluster. This is one component of a larger design to make teleport more cloud friendly. See [RFD 48](https://github.com/gravitational/teleport/blob/master/rfd/0048-war-dialler-node-tracker.md) for more details.

## Why
The main goal is to remove the need for a node agent to create a reverse tunnel to every proxy. The problems caused by this behavior are outlined [here](https://github.com/gravitational/teleport/blob/master/rfd/0048-war-dialler-node-tracker.md#why).

## Terminology
**User-Proxy** - The proxy a user establishes a connection to.

**Node-Proxy** - The proxy a node establishes a reverse tunnel to.

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
   // NodeID is the {UUID}.{ClusterName} of the node to connect to.
   string NodeID = 1;
   // TunnelType is the type of service being accessed. This differentiates agents that
   // create multiple reverse tunnels for different services.
   string TunnelType = 2 [ (gogoproto.casttype) = "github.com/gravitational/teleport/api/types.TunnelType" ];
   // Source is the original source address of the client.
   Addr Source = 3;
   // Destination is the destination address to connect to over the reverse tunnel.
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

The following diagram shows a user connecting to a proxy, the user-proxy, and trying to reach a node connected to a different proxy, the node-proxy. Using the DialNode rpc, the user-proxy can create a bidirectional stream to the node through the node-proxy.
```
┌────┐                         ┌──────────┐
|user|──────connection────────>|user-proxy|
└────┘                         └────╥─────┘
                                    ║
                                grpc stream
                                    ║
┌────┐                         ┌────╨─────┐
|node|─────reverse-tunnel─────>|node-proxy|
└────┘                         └──────────┘
```

A call to the DialNode rpc will send an initial frame containing a `DialRequest`. All subsequent frames should contain `Data` messages. An appropriate error will be returned if the dial request fails for some reason.

To avoid duplicate work the user-proxy will handle all typical proxy side logic like authorization and session recording, while the node-proxy will forward the connection directly to the node.

The DialNode rpc will be wrapped with a client library to return a net.Conn when called. This abstraction allows teleport to treat any underlying transport the same, whether it be a direct dial to the node, a reverse tunnel connected to the user-proxy, or a connection over the DialNode rpc.

```go
type NodeTunnelClient interface {
    DialNode(
        ctx context.Context,
        proxyAddress string,
        src net.Addr,
        dst net.Addr,
        serverID string,
        tunnelType types.TunnelType,
    ) (net.Conn, error)
}
```

### Security
The api will use mTLS to ensure that only other proxies are able to connect. This is done by checking certificates for the build-in role “Proxy”. This will prevent users from connecting to the service directly without going through the user-proxy logic of authorization and session recording.

### API Clients
Each proxy will need to manage multiple grpc clients, one to each neighboring proxy. These will be created as needed, or in other words the first time `DialNode` is called for that specific proxy. Once a client is created it will be reused for any future requests to the same neighboring proxy.

Monitoring will be added to ensure that a single grpc connection does not become a bottleneck for many concurrent streams. The following metrics will be tracked:

1. Total throughput in bytes, this is the aggregate number of bytes sent over all grpc channels on a single connection.
2. Number of concurrent streams, this is the number of streams at any instant.
3. Total number of streams, this is the total number of streams including both current and past streams.

With these metrics we will be able to see if throughput begins to drop as more streams are being used. If this does become an issue additional tcp connections will need to be created.

In dynamic environments where proxies are added and removed, grpc clients will need to be cleaned up periodically to avoid memory leaks. This can be implemented using [grpc connectivity states](https://github.com/grpc/grpc/blob/master/doc/connectivity-semantics-and-api.md). If a client connection enters a bad state for some period of time it can be cleaned up.

### Reverse Tunnel Agent Pool
Changes to the reverse tunnel agent and agent pool are required to support this design. The existing implementation which connects to all proxies will be named `MeshPool` and the new implementation which connects to a single proxy will be named `SinglePointPool`. Both will implement the following interface:
```go
type AgentPool interface {
    // Start begins creating agents and blocks until the pool is stopped or
    // the context is canceled
    Start(ctx context.Context) error
    // Stop stops all agents in the pool and cleans up resources.
    Stop(ctx context.Context) error
}
```

The SinglePointAgent can take advantage of reconnects and improved jitter/backoffs to help further mitigate the thundering herd problem. Node agents can be signalled to reconnect periodically as well as when the proxy server is shutting down. The proxy server will evenly distribute reconnencts over time. Without reconnects, if a load balancer or proxy were to shutdown many nodes would reconnect at the same time.

A proxy initiated reconnect will work as follows:
1. Assume the agent has already established a connection to a proxy server.
2. The proxy server sends a request to the agent over ssh to begin a reconnect.
3. The agent establishes a new connection to a proxy server.
4. The agent sends an request over the old ssh connection to indicate the old connection is being drained. At this point the proxy will stop sending new requests to the old connection.
5. After all pre existing requests have been drained the agent closes the old connection.

### Trusted Clusters
Leaf clusters will continue to use the mesh agent pool, connecting to all proxies in the root cluster. Supporting trusted clusters would add a non-trivial amount of work and complexity to this design and provides diminishing returns. It is expected that trusted clusters will not be connected at the same scale as other resouces like ssh nodes and therefore will not be a big contributer to the problems we are trying to address here.

## Alternative Considerations
An alternative approach was considered to redirect clients to the corresponding node-proxy. This was ultimately disregarded for a couple of reasons. It increases the time to establish a session for the client as a client would need to dial and authenticate with two proxies. Proxies would need to be individually addressible by the client which makes them an easier targets for DDOS attacks.