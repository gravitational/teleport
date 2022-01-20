---
authors: Nic Klaassen <nic@goteleport.com>
state: Draft
---

# RFD 50 - Cluster Join Methods and Endpoints

## What

There are multiple ways a node can get credentials for a Teleport cluster. This
document describes the current node joining methods and details upcoming changes
for the IAM join method (RFD 41) and Certificate Bot (RFD YY).

## Why

This RFD can serve as a reference for all of the ways Teleport nodes can get
cluster credentials in order to facilitate discussion of the current methods and
new endpoints that will be added for the IAM join method and Certificate Bot.

## Details

When "joining" a cluster, a Teleport node must authenticate itself using either
a secret token or one of the new AWS join methods (EC2 or IAM method). If the
node can successfully authenticate, the Auth server will return signed SSH and
TLS certificates to the node for the requested role (Proxy, Node, Kubernetes,
etc).

The joining node also needs to be able to trust that the Teleport cluster is
authentic. It can do this by using CA pins configured on the node, in the case
of IoT nodes connecting to the proxy, the proxy TLS cert can be trusted through
PKI.

### Current Join Endpoints

#### Auth `POST /tokens/register`
Join methods: token, EC2
Server trust: CA pins
This is the main token register endpoint on the Auth server. Accepts tokens or
EC2 Identity Documents to authenticate the client.

#### Auth `POST /tokens/register/auth`
Join Methods: token
Server trust: CA pins
Looks like this is a legacy endpoint the just deletes the given token. Only
works for tokens with the `Auth` role. I can't find anything that calls this, so
it can probably be scheduled for deletion.

#### Auth `rpc GenerateHostCerts(HostCertsRequest) returns (Certs)`
Client trust: mTLS
Server trust: mTLS
This is an mTLS authenticated gRPC endpoint for cert renewal. The node must
first join the cluster using another endpoint to get its first client
certificate used for the mTLS connection.

#### Proxy `POST /webapi/host/credentials`
Join methods: token, EC2
Server trust: PKI
This is the proxy endpoint for registering IoT nodes, it basically forwards to
the Auth `/tokens/register` endpoint.

### New IAM Join Method
The IAM join method requires new gRPC methods because the design requires gRPC
streams to implement a challenge/response protocol. A new gRPC method will be
added on Auth and Proxy to support this. The client will be able to call the
gRPC method with either a Proxy or Auth address and it will "just work"
transparently.

#### Auth `rpc RegisterUsingIAM(stream RegisterUsingIAMRequest) returns (stream RegisterUsingIAMResponse)`
Join mathods: IAM
Server trust: CA pins
This is the Auth endpoint that will complete the IAM join request.

#### Proxy `rpc RegisterUsingIAM(stream RegisterUsingIAMRequest) returns (stream RegisterUsingIAMResponse)`
Join mathods: IAM
Server trust: PKI
Normally, authenticated gRPC calls from IoT nodes are tunnelled through the
proxy over SSH. This will not work for unauthenticated clients which don't yet
have an SSH certificate.

It would be possible to use TLS routing on the proxy to forward gRPC requests
directly to the Auth server, but this would open up the Auth server to DOS
attacks from unauthenticated clients.

Instead, this single gRPC method will be implemented on the proxy. This method
will first perform rate-limiting, then "forward" the request to the auth server
method at the application layer by calling the gRPC method through the Proxy's
own Auth client.

Since the Proxy currently does not expose any gRPC service of its own, a new
gRPC listener will be added to support this. gRPC connections will be
multiplexed on the web listener/port using the existing ALPN-based multiplexer
and a special ALPN ProtocolName of `teleport-proxy-grpc` that will be passed by
the client.

### Certificate Bot
The Certificate Bot needs to get an initial certificate for the cluster which
will be renawable. It will either provide a token or use the new EC2 or IAM
methods to get the initial certificate. This is very similar to a node joining a
cluster, except that the obtained certificate will be renewable.

We can re-use the existing `/tokens/register` (on Auth) and
`/webapi/host/credentials` (on Proxy) endpoints and extend the backend
implementations to provide renewable certs if allowed by the corresponding token
in the backend. This will also allow the client-side logic on the bot to reuse
the `auth.Register` function which handles CA pins and supports joining through
both Auth and Proxy.

The existing `ProvisionTokenV2` type can be extended to allow renewable certs
and to encode the allows `bot_user` and `bot_roles`.
