---
authors: Tim Ross (tim.ross@goteleport.com)
state: draft
---

# RFD 0100 - Use gRPC to proxy SSH connections to Nodes

# Required Approvers

* Engineering @zmb3 && (fspmarshall || espadolini)

## What

Add an alternate transport mechanism to the Proxy for proxying connections
to Nodes

## Why

One of the primary contributors to `tsh ssh` connection latency is the
time it takes to perform an SSH handshake. All connections to a Node via
`tsh` are proxied via a SSH session established with the Proxy. Which means 
that in order to connect to a Node `tsh` must perform at least two SSH handshakes,
one with the Proxy to setup the connection transport and another with the
target Node over the transport to establish the user's SSH connection.

## Details

`tsh ssh` needs to connect to the target Node via the Proxy, but it
doesn't have to use SSH for that communication. A new gRPC service exposed
by the Proxy could perform the same operations as the existing SSH server
but without as much overhead required to establish the session. To minimize
changes both in `tsh` and on Cluster admins, the existing SSH port can be multiplexed
to accept both SSH and gRPC by leveraging the TLS ALPN protocol `teleport-proxy-grpc-ssh`.
Any incoming requests on the SSH listener with said ALPN protocol will be routed
to the gRPC server and all other requests to the SSH server.

Note: a gRPC server is already exposed via the Proxy web address that users the ALPN protocol
`teleport-proxy-grpc`. In order to not conflict the new ALPN protocol is required. Reusing the
existing gRPC server is not an option since it is unauthenticated, has aggressive keep alive
parameters, and is only enabled when TLS Routing is enabled.

### Proto Definition

The specification is modeled after the [ProxyService](https://github.com/gravitational/teleport/blob/master/api/proto/teleport/legacy/client/proto/proxyservice.proto)
which is a similar transport mechanism leveraged for Proxy Peering.

```proto
service ConnectionProxyService {
  // GetClusterDetails provides cluster information from the perspective of the Proxy
  rpc GetClusterDetails(GetClusterDetailsRequest) returns (GetClusterDetailsResponse);
  // DialNode establishes a connection to the target host
  // 
  // The bidirectional stream is a transport mechanism to send raw data between
  // client and server. This allows other protocols(SSH) to be sent through the
  // tunnel.
  rpc DialNode(stream DialNodeRequest) returns (stream DialNodeResponse);
}

// Request for DialNode
//
// Connection flow
// -> Target (client)
// <- Connection (server)
// <-> Frame/AgentFrame (client/server)
// 
// Separating the Frame and AgentFrame allows
// SSH and the SSH Agent Protocol to be multiplexed
// over the same connection.
message DialNodeRequest {
  oneof payload {
    // Contains the information to identify and connect
    // to the target host
    Target dial_target = 1;
    // Raw data to transmit
    DataFrame frame = 2;
  }
}

// Response for DialNodeHost
message DialNodeResponse {
  oneof payload {
    // Cluster configuration details
    ClusterDetails details = 1;
    // Raw data to transmit
    DataFrame frame = 2;
  }
}

// Protocol represents the supported protocols
// that can be proxied.
enum Protocol {
  PROTOCOL_UNSPECIFIED = 0;
  // SSH
  PROTOCOL_SSH = 1;
  // SSH Agent
  PROTOCOL_SSH_AGENT = 2;
}

// A data frame carrying the payload and 
// procotol. Allows for multiplexing different
// protocols on the same stream.
message DataFrame {
  // The protocol that sent the data
  Protocol protocol = 1;
  // The raw packet of data 
  bytes payload = 2;
}

// Target indicates which server to connect to
message Target {
  // The hostname/ip/uuid of the remote host
  string host = 1;
  // The port to connect to on the remote host
  int port = 2;
  // The cluster the server is a member of
  string cluster = 3;
  // The OS login 
  string login = 4;
}

// Request for GetClusterDetails.
message GetClusterDetailsRequest {
  // The cluster to get details for
  string cluster = 1;
}

// Response for GetClusterDetails.
message GetClusterDetailsResponse {
  // Cluster configuration details
  ClusterDetails details = 1;
}

// ClusterDetails contains details about the cluster configuration
message ClusterDetails {
  // If proxy recording mode is enabled
  bool recording_proxy = 1;
  // If the cluster is running in FIPS mode
  bool fips_enabled = 2;
}
```

The `DialNode` RPC establishes a connection to a Node on behalf of the user.
There is an initial ceremony which must be completed by both sides of the stream
before raw data can be transported. The client must first send a `Target`
message which declares the target server that the connection is for. If the
target exists and session control allows, the server will establish the
connection and respond with a `ConnectionDetails` message.

After the initial ceremony, either side of the connection may transmit `Frame`s
until the connection terminates. Each `Frame` includes the raw payload and
the `Protocol` which generated the payload to allow for multiple protocols to
leverage the same stream as a transport.

The most pressing need for multiplexed protocols is to ensure that `tsh ssh` functions
properly in proxy recording mode. Since the Proxy creates an SSH connection to the Node
on behalf of the user in proxy recording mode the user *must* forward their agent to
facilitate the connection. Currently when `tsh` determines the Proxy is performing the
session recording it will forward the user's agent over a SSH channel. The Proxy then
communicates SSH Agent protocol over that channel to sign requests. `tsh` utilizes
`agent.ForwardToAgent` and `agent.RequestAgentForwarding` from `x/crypto/ssh/agent` to
set up the channel and serve the agent over the channel to the Proxy.

To achieve the same functionality using the gRPC stream proposed above, the SSH Agent protocol can
be multiplexed over the stream in addition to the SSH protocol. When `tsh` determines proxy
recording is in effect it can leverage `agent.ServeAgent` directly, passing in an `io.ReadWriter`
which sends and receives `Frame`s with `Protocol_SSH_AGENT`populated in the protocol field when
it is written to and read from. The server side can communicate with the local agent by using
`agent.NewClient` on a similar `io.ReadWriter`.

The end result is both SSH and SSH Agent protocol being transported across the same stream
to enable both the SSH connection to the target Node and allowing the Proxy to communicate
with the user's local SSH agent in a similar manner to way it works to date.

## Performance

Below are two traces captured with both Proxy transport mechanisms that illustrate the latency
reduction.

#### SSH
![SSH Transport](assets/0100-ssh-transport.png)
#### gRPC
![gRPC Transport](assets/0100-grpc-transport.png)


The existing SSH transport took 6.73s to execute `tsh user@foo uptime`, while the same
command via the gRPC transport took 5.36s resulting in a ~20% reduction in latency. 

## Security

The gRPC server will require mTLS for authentication and perform the same RBAC
and session control checks as the current SSH server does. Agent forwarding will
occur as it does today with the exception that the SSH Agent Protocol will use a
gRPC stream instead of an SSH channel for transport.

## UX

The behavior of `tsh ssh` should remain the same regardless of the configured
session recording mode. The time it takes to establish a session may be noticeably
faster depending on proximity of the client and the Proxy.
