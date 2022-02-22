---
authors: David Boslee (david@goteleport.com), Naji Obeid (naji@goteleport.com)
state: draft
---

# RFD XX - Proxy Peering

## What
This document describes an API that enables a proxy to dial the nodes connected to a peer proxy. This is an optional feature that will allow node agents to connect to a single proxy and be reachable through any other proxy in the cluster.

## Why
Currently node agents dialing over a reverse tunnel are required to connect to every proxy instance in the cluster. This allows a client to connect to a node through any proxy but it also causes other problems when running proxies behind a load balancer like in our cloud environment. These problems include:

- Ephemeral port exhaustion between a NAT gateway and load balancer. This limits the number of nodes that can be connected behind a single NAT gateway. As more proxies are added fewer nodes are able to connect.
- Thundering herd when adding, removing, or restarting a proxy. Node agents retry connecting until they randomly get balanced to the desired proxy. The more node agents connected the worse this problem becomes.

Both these issues are mitigated by changing the node agent behavior to connect to a single proxy. Ephemeral port exhaustion is no longer tied to the number of proxies and node agents no longer need to retry until they connect to a specific proxy.

## Terminology
**User-Proxy** - The proxy a user establishes a connection to.

**Node-Proxy** - The proxy a node establishes a reverse tunnel to.

## Details

### Proxy API

The following gRPC service will be added to proxy servers:

```protobuf
syntax = "proto3";

package api;

service ProxyService { rpc DialNode(stream Frame) returns (stream Frame); }

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

A call to the DialNode rpc will send an initial frame containing a `DialRequest`. All subsequent frames should contain `Data` messages. Failure scenarios are outlines [here](#failure-scenarios).

To avoid duplicate work the user-proxy will handle all the typical proxy side logic like authorization and session recording, while the node-proxy will forward the connection directly to the node.

The DialNode rpc will be wrapped with a client library to return a net.Conn when called. This abstraction allows teleport to treat any underlying transport the same, whether it be a direct dial to the node, a reverse tunnel connected to the user-proxy, or a connection over the DialNode rpc.

```go
type ProxyClient interface {
    DialNode(
        ctx context.Context,
        serverID string,
        tunnelType types.TunnelType,
        proxyID string,
    ) (net.Conn, error)
}
```

### Security
The api will use mTLS to ensure that only other proxies are able to connect. This is done by checking certificates for the built-in role “Proxy”. This will prevent users from connecting to the service directly without going through the user-proxy logic of authorization and session recording.

### Enabling Proxy Peering
This feature will need to be explicity configured to use it. The configuration will be set in the auth_service section of the teleport config and will update the `ClusterNetworkingConfig` stored in the backend.

The teleport config:
```yaml
auth_service:
  ...
  proxy_peering: enabled
  ...
```

The `ClusterNetworkingConfig`:
```
message ClusterNetworkingConfigSpecV2 {
    ...
    ProxyPeeringMode ProxyPeeringMode = 9 [ (gogoproto.jsontag) = "proxy_peering_mode,omitempty" ];
    ...
}

enum ProxyPeeringMode {
    Disabled = 0;
    Enabled = 1;
}

```

### Peer Address Configuration
The peer address is the address the `ProxyService` GRPC API will be exposed on. This will be configured under proxy_service in the configuration file. If the address is unspecified (`0.0.0.0`) then an address will be discovered using the `GuessHostIP` function in [lib/utils/addr.go](https://github.com/gravitational/teleport/blob/56c536e61f4b52c011b7d18dfaaf2b2c9ecac1cc/lib/utils/addr.go#L281). During startup the proxy will check the `ClusterNetworkingConfig` to see if `proxy_peering` is enabled before starting the `ProxyService`. A default value of `0.0.0.0:3021` will be used.
```yaml
proxy_service:
  ...
  peer_listen_addr: 0.0.0.0:3021
  ...
```
This address will be added to the [ServerSpecV2](https://github.com/gravitational/teleport/blob/95c53ad90e68887778db8141238fee494028bbdf/api/types/types.proto#L364) and stored in the backend.
```protobuf
string PeerAddr = 11 [ (gogoproto.jsontag) = "peer_addr,omitempty" ];
```
### Agent Proxy Relationship

The ID of the proxy an agent is connected to will be added to the [ServerSpecV2](https://github.com/gravitational/teleport/blob/95c53ad90e68887778db8141238fee494028bbdf/api/types/types.proto#L364) along with a Nonce and NonceID to mitigate out of order updates.
```protobuf
string ProxyID = 12 [ (gogoproto.jsontag) = "proxy_id,omitempty" ];
int64 Nonce = 13 [ (gogoproto.jsontag) = "nonce,omitempty" ];
int64 NonceID = 14 [ (gogoproto.jsontag) = "nonce_id,omitempty" ];
```

Since each proxy already keeps a cache of `Servers` there will be no additional mechanism required to replicate this information.

Each agent will be responsible for updating the `ProxyID` as it connects and reconnects to proxy servers. This will be done over the existing periodic heartbeats to the auth server. If `proxy_peering` is not enabled in the `ClusterNetworkingConfig` the `ProxyID` should not be included.

The `Nonce` will start at 0 and be incremented with each update sent to the auth server. On each restart of the teleport agent a new `NonceID` will be randomly generated. The auth server will reject any updates where the `heartbeat.nonce < previous.nonce && heartbeat.nonce_id == previous.nonce_id`.

### API Clients
Each proxy will need to manage multiple grpc clients, one for each peer proxy. Client connections will be created as peer proxies are discovered. Similar to the agent pools current behavior, clients can be expired if the connection fails and the peer hasn't heartbeated to the backend for a certain amount of time.

Transient connection failures can be detected using [GRPC keepalives](https://pkg.go.dev/google.golang.org/grpc/keepalive) along with the client [`WaitForStateChange`](https://pkg.go.dev/google.golang.org/grpc#ClientConn.WaitForStateChange) API. The time it takes to detect a dead connection is determined by the keepalive `Timeout` parameter. The grpc client will automatically try to reconnect with an exponential backoff policy.

For future backwards compatibility the proxy teleport version will be included in the grpc client/server headers. This will allow either a client or server to downgrade messages accordingly.

Metrics will be added so we can monitor whether a single grpc connection becomes a bottleneck for many concurrent streams. The following metrics will be tracked:

1. Total throughput in bytes, this is the aggregate number of bytes sent over all grpc channels on a single connection.
2. Number of concurrent streams, this is the number of streams at any instant.
3. Total number of streams, this is the total number of streams including both current and past streams.

With these metrics we will be able to see if throughput begins to flatten as more streams are being used. If this does become an issue additional tcp connections will need to be created.

### Reverse Tunnel Agent Pool
Changes to the reverse tunnel agent and agent pool are required to support this design. The existing implementation creates a connection to every proxy. The new impelementation will decide how many connections to create dynamically based on the `ClusterNetworkingConfig`. If `proxy_peering` is enabled, a single connection will be created. If it is not enabled a connection to every proxy will be created. Old agents can continue connecting to every proxy regardless of the `ClusterNetworkingConfig`.

Having a single connection means that if that connection fails the agent will be unreachable until it is able to establish a new connection. To mitigate this the agent will support proxy initiated reconnects. This will work as follows:

1. Agent has an established connection to a proxy, connection-1.
2. Proxy sends reconnect request to the agent over connection-1.
3. Agent establishes a new connection connection-2.
4. Agent signals the auth server of a proxy change if applicable.
5. Proxies receive the updated proxyID and begin connecting to the agent through new proxy.
6. Proxy stops sending transport requests to connection-1 and closes it once drained.

This can be used during graceful shutdowns to migrate agents off of a proxy so they remain reachable at all times. In the future this could be used periodically to migrate connections off of intermediate infrastrucuture like load balancers that are trying to drain connections.

### Trusted Clusters
Leaf clusters will continue connecting to all proxies in the root cluster. Supporting trusted clusters would add a non-trivial amount of work and complexity to this design and provides diminishing returns. It is expected that trusted clusters will not be connected at the same scale as other resouces like ssh nodes and therefore will not be a big contributer to the problems we are trying to address here.

### Cluster Upgrade
Upgrading a cluster to support this feature will require reconfiguration the auth service as follows:
```yaml
auth_service:
...
    proxy_peering: true
...
```

Then each proxy will need to be restarted. This will allow the proxy to see the new `ClusterNetworkingConfig` and start the `ProxyService`.

When an agent reconnects it will discover the new `ClusterNetworkingConfig` and begin using a single connection back to the proxies. The agent may create a single connection to a proxy that has not yet been restart to expose the `ProxyService`. This means the node will only be reachable through that proxy until the proxy is restarted.

### Failure Scenarios
This design introduces several new points of failure on the path from a client to a node agent.

1. Failure to dial the node-proxy.
2. Node agent not connected to the expected node-proxy.
3. Proxy tunnel grpc client disconnects.
4. Node agent disconnects during dial/session over proxy tunnel.

These failures will be presented to the client as follows:

1 and 2 will display a message similar to what is returned [here](https://github.com/gravitational/teleport/blob/9edf72b86fd192ca965e65db60fb92c6858a314d/lib/reversetunnel/localsite.go#L314-L322) to indicate the node agent is offline or disconnected.

3 and 4 will have the same behavior as a node agent disconnecting unexpectedly with the current implementation. This results in an [ExitMissingError](https://pkg.go.dev/golang.org/x/crypto/ssh#ExitMissingError) being displayed client side.

### TLS Routing
Load balancers between the agent and proxy servers may want to diffentiate between old agents that need to connect to every proxy and the new agents described in this document. This is important for geo distributed deployments to ensure low latency routing.

The cluster must be configure with `proxy_listener_mode: multiplex` to enable TLS ALPN routing. New agents will add an additional protocol `teleport-reversetunnelv2` to the ALPN header field resulting in the following list: `["teleport-reversetunnelv2", "teleport-reversetunnel"]`.

Preserving `teleport-reversetunnel` in the list of protocols, ensures that new agents are able to connect to proxies running an older version of teleport.

## Alternative Considerations

### Node Tracker
The orginal proposal included a separate service for tracking which proxy each node was connected to. This was ultimately decided against. The service was proposed to target scalability goals that need to be addressed in other parts of the system first. Given these limitations a simpler design was chosen to benefit the most customers. Further discussions on the node tracker proposal can be found [here](https://github.com/gravitational/teleport/pull/9121).

### Client Redirect
An alternative approach was considered to redirect clients to the corresponding node-proxy. This was ultimately disregarded for a couple of reasons. It increases the time to establish a session for the client as a client would need to dial and authenticate with two proxies. Proxies would need to be individually addressible by the client which makes them an easier targets for DDOS attacks.
