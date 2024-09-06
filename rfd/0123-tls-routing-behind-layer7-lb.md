---
authors: STeve Huang (xin.huang@goteleport.com)
state: implemented
---

# RFD 0123 - TLS Routing behind load balancers

## Required Approvals

* Engineering: @smallinsky && @r0mant
* Product: @xinding33 || @klizhentas
* Security: @reedloden

## What

This RFD details how to support [TLS
Routing](https://github.com/gravitational/teleport/blob/master/rfd/0039-sni-alpn-teleport-proxy-routing.md) behind layer
7 load balancers for client protocols supported by Teleport.

## Why

Allows simple single-port Teleport deployment when Teleport needs to sit behind a layer 7 load balancer.

## Details

### The challenge

Here is a typical setup that we want to enable by this RFD:
```
 ┌───┐
 │tsh│
 └┬──┘
  │
  │SSH,DB,Kube,App,etc.
  │
 ┌▼────────────┐
 │Load Balancer◄──┐
 └┬────────────┘  │Reverse tunnel,
  │               │Join,
  │Web port       │etc.
  │               │
 ┌▼───────┐     ┌─┴──────┐
 │Teleport│     │Teleport│
 │Proxy(s)│     │Agent(s)│
 └────────┘     └────────┘
```

The "load balancer" here could be a cloud-hosted service like AWS ALB, a Kubernetes ingress controller, or any other
similar network solutions.

TLS Routing already combines all proxy ports into a single port. However, most load balancers do NOT support passthrough
of custom ALPN, SNI, or mTLS, which are required by TLS Routing, when the load balancers are configured to terminate
TLS.

### The WebSocket upgrade

The Teleport client make a WebSocket connection upgrade through a web API on the Teleport Proxy:

```
 ┌───┐                     ┌─────────────┐                 ┌──────────────┐
 │tsh│                     │Load Balancer│                 │Teleport Proxy│
 └─┬─┘                     └───────┬─────┘                 └───────┬──────┘
   │                               │                               │
   │ GET /webapi/connectionupgrade │                               │
   │ Upgrade: websocket            │                               │
   ├──────────────────────────────►│                               │
   │                               │  ┌───────────────┐            │
   │                               ├──┤TLS Termination│            │
   │                               │  └───────────────┘            │
   │                               │                               │
   │                               │ GET /webapi/connectionupgrade │
   │                               │ Upgrade: websocket            │
   │                               ├──────────────────────────────►│
   │                               │                               │
   │                               │ HTTP 101 Switching Protocols  │
   │                               │◄──────────────────────────────┤
   │ HTTP 101 Switching Protocols  │                               │
   │◄──────────────────────────────┤                               │
   │                               │                               │
   │ TLS Routing: teleport-mysql   │                               │
   │◄──────────────────────────────┼──────────────────────────────►│
   │                               │                               │
```

This allows the client to tunnel the "original TLS Routing" call through a Layer 7 (HTTP) load balancer.

In addition to the `Upgrade` header, the following headers are also set from the client:
- `Connection` header is set to `Upgrade` to meet [RFC spec](https://datatracker.ietf.org/doc/html/rfc2616#section-14.42)
- `Sec-WebSocket-Version` is set to 13.
- `Sec-WebSocket-Protocol` specifies the sub-protocol used for this upgrade: `alpn` or `alpn-ping`.
- `Sec-WebSocket-Key` is required by WebSocket standard. More details in security section.

A WebSocket frame can be one of 6 types: `text`, `binary`, `ping`, `pong`, `close` and `continuation`. The "original TLS
Routing" layer will be embedded as `binary` frames. When the sub-protocol is `alpn-ping`, Proxy Server will send native
WebSocket `ping` frames periodically and the client will respond with `pong`.

### The legacy "connection upgrade"

This section describes the legacy Teleport-custom connection upgrade used before v15.1.

Borrowed from the "WebSocket" design, the Teleport client can make a connection upgrade through a web API on the
Teleport Proxy:

```
 ┌───┐                     ┌─────────────┐                 ┌──────────────┐
 │tsh│                     │Load Balancer│                 │Teleport Proxy│
 └─┬─┘                     └───────┬─────┘                 └───────┬──────┘
   │                               │                               │
   │ GET /webapi/connectionupgrade │                               │
   │ Upgrade: alpn                 │                               │
   ├──────────────────────────────►│                               │
   │                               │  ┌───────────────┐            │
   │                               ├──┤TLS Termination│            │
   │                               │  └───────────────┘            │
   │                               │                               │
   │                               │ GET /webapi/connectionupgrade │
   │                               │ Upgrade: alpn                 │
   │                               ├──────────────────────────────►│
   │                               │                               │
   │                               │ HTTP 101 Switching Protocols  │
   │                               │◄──────────────────────────────┤
   │ HTTP 101 Switching Protocols  │                               │
   │◄──────────────────────────────┤                               │
   │                               │                               │
   │ TLS Routing: teleport-mysql   │                               │
   │◄──────────────────────────────┼──────────────────────────────►│
   │                               │                               │
```

This allows the client to tunnel the "original TLS Routing" call through a Layer 7 (HTTP) load balancer.

In addition to the `Upgrade` header, the following headers are also set from the client:
- `Connection` header is set to `Upgrade` to meet [RFC spec](https://datatracker.ietf.org/doc/html/rfc2616#section-14.42)
- `X-Teleport-Upgrade` header is set to the same value as the `Upgrade` header as some load balancers have seen
  dropping values from the `Upgrade` header

#### Migration from "legacy" to WebSocket

Both WebSocket and the "legacy" upgrade methods will be supported by the Teleport clients and servers for 1-2 major
releases for backwards compatibility. And the "legacy" upgrade is expected to be removed in a later version.

During the transition period, the client will send both upgrade types in the "Upgrade" headers, with "websocket" as the
first choice. Upon receiving 101 Switching Protocols, the client will pick the negotiated protocol based on the value of
the "Upgrade" header from the response ([RFC spec](https://datatracker.ietf.org/doc/html/rfc2616#section-14.42)).

On the server side, older servers ignore any upgrade types other than the "legacy" upgrade types. Therefore "websocket"
sent by newer clients will be skipped and "legacy" upgrades will be negotiated as long as the "legacy" types are also
present in the request headers.

During the transition period, newer servers prefer the "websocket" upgrade type if both WebSocket and "legacy" types are
provided.

### Teleport Proxy with self-signed certs

Load balancers usually terminate TLS with certificates signed by publicly trusted CAs. Since Teleport Proxy is no longer
the public entry point, it is no longer necessary for Teleport Proxy to serve a publicly trusted cert (e.g. using ACME).
Thus Teleport Proxy should be welcome to use self-signed certs when sitting behind load balancers.

To accommodate that, instead of serving the **Web** certs, the ALPN server on the Proxy will serve the **Host** certs
when receiving connections from the upgrade flow. And instead of using the SystemCertPool, the Teleport client should
use the **Host** CA for verifying the Proxy server when dialling TLS Routing inside an upgrade connection. Note that
this modification does NOT apply to Teleport ALPN protocols that already do mTLS with other certs.

### When to make a connection upgrade

Teleport clients should first perform a test to decide if a connection upgrade is necessary. The test is done by sending a
TLS handshake with a Teleport custom ALPN to the Proxy server:
```
                    ┌───────────────────────────┐
                    │     TLS Handshake         │
                    │ALPN teleport-reversetunnel│
                    └────┬─────────────────┬────┘
                         │                 │
                     ┌───▼─────┐        ┌──▼──────┐
                     │Handshake│        │Handshake├───────────────┐
 ┌───────────────────┤ Success │        │ Failed  │               │
 │                   └───┬─────┘        └┬────────┘               │
 │                       │               │                        │
 │Negotiated             │No ALPN        │remote error: tls:      │other
 │teleport-reversetunnel │Negotiated     │no application protocol │errors
 │                       │               │                        │
 │                   ┌───▼────┐          │                  ┌─────▼──┐
 │                   │Upgrade ◄──────────┘                  │  Not   │
 │                   │Required│                             │Required│
 │                   └────────┘                             └────▲───┘
 │                                                               │
 └───────────────────────────────────────────────────────────────┘
```
Connection upgrade for TLS Routing should be required when the client and the server fail to negotiate the ALPN
protocol, hinting a load balancer in the middle terminates TLS.

However, this test may not be bulletproof. In particular, the test explicitly looks for `tls: no application protocol`
from the remote when handshakes fail, but it is possible that a load balancer implementation decides to use a different
text. An environment variable is provided to overwrite the decision per client, just in case:
```
export TELEPORT_TLS_ROUTING_UPGRADE=false
export TELEPORT_TLS_ROUTING_UPGRADE=my-teleport.com=true;another-teleport.com=false
```

### Supporting all TLS Routing protocols

In general, in ALL existing places we are dialling the TLS Routing connection, common logic will be added to the
`ContextDialer` to automatically upgrade the connection when necessary.

However, not all connections are initiated by `tsh`, `teleport`, or a client using the API lib. In such cases, a [local
Teleport
proxy](https://github.com/gravitational/teleport/blob/master/rfd/0039-sni-alpn-teleport-proxy-routing.md#local-teleport-proxy)
is required to dial the TLS Routing connection with the upgrade.

Local proxy has already been implemented for Database Access and Application Access. Here are some details of how it can
be implemented for Kubernetes Access.

#### Local proxy for Kubernetes Access

`tsh` will serve a local HTTPS proxy server with a localhost self-signed CA. The self-signed CA should be generated per
`tsh login` session with matching TTL, on demand.

An ephemeral (per `tsh` command) KUBECONFIG will be provided to Kubernetes applications to connect to the local server.
Each Kubernetes cluster logged in through Teleport will have its own `cluster` entry with `proxy-url` field pointing to
the local proxy. The `tls-server-name` will be in the format of
`<hex-encoded-kube-cluster-name>.<teleport-cluster-name>` so the local proxy can identify the Kubernetes cluster name
and the Teleport cluster name. And the config will include static user credentials generated with the local self-signed
CA:
```yaml
apiVersion: v1
clusters:
- cluster:
    certificate-authority: <path-to-self-signed-CA>
    proxy-url: http://127.0.0.1:8888
    server: https://my-teleport.com
    tls-server-name: 6D792D6B756265.my-teleport.com
  name: my-teleport.com-my-kube
- cluster:
    certificate-authority: <path-to-self-signed-CA>
    proxy-url: http://127.0.0.1:8888
    server: https://my-teleport.com
    tls-server-name: 616E6F746865722D6B756265.my-teleport-leaf.com
  name: my-teleport-leaf.com-another-kube
contexts:
- context:
    cluster: my-teleport.com-my-kube
    user: my-teleport.com-my-kube
  name: my-teleport.com-my-kube
- context:
    cluster: my-teleport-leaf.com-another-kube
    user: my-teleport-leaf.com-another-kube
  name: my-teleport-leaf.com-another-kube
current-context: my-teleport.com-my-kube
users:
- name:: my-teleport.com-my-kube
  user:
    client-certificate: <path-to-user-cert>
    client-key: <path-to-user-key>
- name: my-teleport-leaf.com-another-kube
  user:
    client-certificate: <path-to-user-cert>
    client-key: <path-to-user-key>
```

Once the local server receives a request and verifies the request, the local server will proxy the request using TLS
Routing. And if connection upgrade is required, the TLS Routing request will be wrapped with the connection upgrade:
```
 ┌────┐                                           ┌─────────┐                         ┌───────────────┐
 │kube│                                           │tsh local│                         │Teleport Proxy/│
 │app │                                           │  proxy  │                         │ Load Balancer │
 └┬───┘                                           └─┬───────┘                         └────────────┬──┘
  │                                                 │                                              │
  │                                                 ├──┐                                           │
  │                                                 │  │generate:                                  │
  │                                                 │  │local CA                                   │
  │                                                 │  │local credentials                          │
  │                                                 │◄─┘ephemeral KUBECONFIG                       │
  │                                                 │                                              │
  │ server https://localhost:8888                   │                                              │
  │ sni my-kube.kube-teleport-local.my-teleport.com │                                              │
  │ local credentials                               │                                              │
  ├────────────────────────────────────────────────►│ server https://my-teleport.com:443           │
  │                                                 │ sni kube-teleport-proxy-alpn.my-teleport.com │
  │                                                 │ "real" user credentials                      │
  │                                                 ├─────────────────────────────────────────────►│
  │                                                 │                                              │
  │                                                 │◄─────────────────────────────────────────────┤
  │◄────────────────────────────────────────────────┤                                              │
  │                                                 │                                              │
```

The local proxy should manage/cache the Teleport certs used for routing Kubernetes requests.

Per-session-MFA TTL should be extended to `max_session_ttl` and cached in
memory by the local proxy, similar to [Database MFA
Sessions](https://github.com/gravitational/teleport/blob/master/rfd/0090-db-mfa-sessions.md).
See [RFD 0121 - Kubernetes MFA
sessions](https://github.com/gravitational/teleport/blob/master/rfd/0121-kube-mfa-sessions.md)
for more details.

### Client source IPs

Correct client source IPs must be reported for IP pinning and audit logs.

Unlike layer 4 load balancers, layer 7 load balancers usually CANNOT transparently forward original clients' IPs or add
standard [PROXY](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt) protocol headers to layer 7 payload. The
standard way for layer 7 load balancers to report correct client source IPs is through the
["X-Forwarded-For"](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For) (XFF) header.

The Proxy service will examine the values of the XFF headers when `proxy_service.use_x_forwarded_for` is enabled (more
details in security section).

ALL web APIs will take client source IPs from the XFF headers. This includes login APIs, Web UI APIs (including wss
sessions) and the ALPN connection upgrade API, to make sure a correct IP is always logged.

Teleport should support XFF header with both IPv4 and IPv6 addresses, with or without port numbers. In case no port is
reported in the XFF header, port can be taken from the observed TCP connection or use an arbitrary number, since the
port is usually trivial.

To keep things simple, only XFF header containing one IP address is accepted. Any requests with multiple XFF headers or
headers containing multiple IPs will be rejected.

### User Experience

Once the connection upgrade support to all protocols is implemented, users can be recommended to upgrade their Teleport
clusters to single port mode if the cluster is currently serving separate ports.

All scenarios discussed in this section assume Teleport sits behind a load balancer so connection upgrade for TLS
Routing is required.

#### 1 - Database Access UX

Common flows already use `tsh` local proxy (since v8) so they will automatically perform the connection upgrade:

- `tsh db connect` (single port mode)
- `tsh proxy db`

So no UX change to these.

In single port mode, using native database clients to connect to Teleport Proxy directly with `tsh db env/config` will
NOT work through load balancers. Users can use one of the above methods instead.

#### 2 - Application Access UX

Application Access through Teleport Webapp is NOT affected by the issue of interest of this RFD.

On the `tsh` side, TCP, AWS, Azure, and GCP apps always use a local proxy so they will automatically perform the
connection upgrade when needed. No UX change to these.

However, HTTP apps MUST use `tsh proxy app`. For example:
```
$ tsh apps login my-app
Logged into app my-app. Start the local proxy for it:

  tsh proxy app my-app -p 8080

Then connect to the application through this proxy:

  curl http://127.0.0.1:8080
```

#### 3 - Kubernetes Access UX

When the connection upgrade is required, vanilla Kubernetes clients cannot connect to Teleport Proxy directly.

`tsh kube login` should advertise users to run `tsh kubectl` and `tsh proxy kube` (instead of `kubectl version`). `tsh
kube credentials` should also error out and suggest using `tsh kubectl` and `tsh proxy kube`.

`tsh kubectl` will automatically start a local proxy and performs the connection upgrade when needed.

Vanilla Kubernetes clients can be used with `tsh proxy kube` running a local proxy:
```
$ tsh proxy kube cluster1 cluster2 -p 8888

Started local Kubernetes server at https://localhost:8888.

Please use the following KUBECONFIG for your Kubernetes applications:
export KUBECONFIG=<path to kubeconfig>
```

`tsh proxy kube` should support all flags supported by `tsh kube login` like cluster names, `--as` etc. and can be run
independently WITHOUT running `tsh kube login` first. If `tsh kube login` has been run perviously, `tsh proxy kube`
should inherit the flags from the login if they are not specified during `tsh proxy kube`.

In addition to the cluster names provided to `tsh proxy kube`, the ephemeral kubeconfig should also copy other
non-Teleport clusters in the default kubeconfig (or `$KUBECONFIG`), so the user does not need to switch between configs
for different clusters.

#### 4 - Server Access UX

UX is the same as when [TLS Routing is
enabled](https://github.com/gravitational/teleport/blob/master/rfd/0039-sni-alpn-teleport-proxy-routing.md#local-teleport-proxy).

#### 5 - Client source IPs

A new setting is added to the Proxy service for web APIs to take client IPs from the "X-Forwarded-For" headers:
```
proxy_service:
  # Enables the Proxy service to take client source IPs from the
  # "X-Forwarded-For" headers for web APIs received from layer 7 load balancers
  # or reverse proxies.
  # 
  # IMPORTANT: please make sure the service is behind a layer 7 load balancer or
  # reverse proxy that sets or appends client IPs in the "X-Forwarded-For"
  # headers.
  #
  # default: false
  use_x_forwarded_for: true
```

Note that this setting is NOT mutually exclusive with the `proxy_protocol` setting. For example, in a separate-port
setup, web APIs including the ALPN connection upgrade endpoint (e.g. called by `tsh proxy db`) can take client source
IPs from the XFF headers set by a layer 7 load balancer, whereas SSH, reverse tunnels and separate DB ports can take
client source IPs from the PROXY headers set by a layer 4 load balancer.

### Security

#### 1 - Connection upgrade

When upgrading the connection, the Teleport client verifies the load balancer's TLS cert using SystemCertPool. And as
mentioned early, at the TLS Routing request, the Teleport client will be configured with a Teleport CA for verifying the
Proxy server.

There is no authentication at the connection upgrade request. Authentication is deferred to the TLS Routing request so
authentication remains the same as if there is no connection upgrade.

#### 2 - Client source IPs

The premise of the IP Pinning feature is to protect against hackers using compromised credentials from different
locations.

A hacker can easily modify a Teleport client with a fake IP in the "X-Forwarded-For" (XFF) if the Proxy service trusts
this value without conditions. Remember the ALPN connection upgrade endpoint can embed all Teleport protocols in TLS
routing requests, and the endpoint is publicly available regardless of whether the Proxy is behind a load balancer or
not.

To minimize this risk, the web APIs will only take client source IPs from XFF headers when
`proxy_service.use_x_forwarded_for` is explicitly set to true. Users have the responsibility to ensure that the Proxy
service is indeed behind a layer 7 load balancer AND the load balancer is configured to set correct values in the XFF
headers.

Further more, requests with multiple XFF headers or headers containing multiple IPs will be rejected as it is difficult
to know which is the truth without extra knowledge of the setup.

#### 3 - `Sec-WebSocket-Key` and `Sec-WebSocket-Accept`

The Teleport clients generate a new random `Sec-WebSocket-Key` on each WebSocket upgrade request, following [RFC
6455](https://www.rfc-editor.org/rfc/rfc6455#section-11.3.1), and verify the value of `Sec-WebSocket-Accept` from the
server response. This ensures the middleman between Teleport clients and Proxy either understands the WebSocket protocol
or bypasses it without caching. The magic string `258EAFA5-E914-47DA-95CA-C5AB0DC85B11` is used as GUID for calculating
`Sec-WebSocket-Accept` as specified in the RFC spec.

### Performance

#### 1 - Performance on the connection upgrade

The downside of doing the connection upgrade is the performance penalty.

The connection is already double-encrypted with TLS Routing, and now it is triple-encrypted with a connection upgrade.
Modern processors should have no trouble doing the job, but concurrent TLS Routing requests with connection upgrades may
affect CPU usage.

The more noticeable impact is likely the latency. Each connection upgrade adds 1~2 round trips for TLS handshake and 1
round trip for the HTTP request/response, per connection. And the extra round trips can add up if there are a lot of new
connections. For example, let's say a `tsh login` makes 10 connections to the remote server and the ping RTT between the
client and the server is 30ms. Without the connection upgrades the login may take about 2~3 seconds (not accounting for
the time for user inputs). The connection upgrades will add about 10 * 2 * 30ms = 600ms total latency.

Some ideas for improving performance include implementing reusable upgraded connections and multiplexing concurrent TLS
Routing requests.

#### 2 - Performance on the load balancer detection test

The test performs a single TLS handshake without any payload. This should take only 1~2 round trips per test.

Since load balancers are usually stationary, clients can cache the test decision to avoid repeating the test. For
example, per `tsh login` can perform the test once and save it in the user profile.

### Keepalive

Some load balancers (e.g. AWS ALB) can drop a connection when the traffic is idle at Layer 7 for a period of time. In
such cases, using short TCP keepalive would not help maintain long-lived connections.

For now, the connection upgrade flow assumes the tunnelled Teleport ALPN protocol either handles keepalives on its own
or requests an `alpn-ping` version of the upgrade. For example, SSH and reverse tunnel connections use `alpn-ping`.

When `alpn-ping` is requested by the client, the Proxy server will send out pings periodically. For WebSocket upgrades,
native WebSocket `ping` frames are sent. For "legacy" upgrades, [TLS Routing
Ping](https://github.com/gravitational/teleport/blob/master/rfd/0081-tls-ping.md) is used to wrap the connection.

