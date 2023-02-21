---
authors: STeve Huang (xin.huang@goteleport.com)
state: draft
---

# RFD XXX - TLS Routing behind load balancers

## Required Approvals

* Engineering: @smallinsky && @r0mant
* Product: @xin || @klizhentas
* Security: @reedloden

## What

This RFD details how to support [TLS
Routing](https://github.com/gravitational/teleport/blob/master/rfd/0039-sni-alpn-teleport-proxy-routing.md) behind load
balancers.

## Why

Allows simple single-port Teleport deployment when Telport needs to sit behind a load balancer.

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
 └┬────────────┘  │Reversse tunnel,
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

### The "connection upgrade"

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

### Teleport Proxy with self-signed certs

Load balancers usually terminate TLS with certificates signed by publicly trusted CAs. Since Teleport Proxy is no longer
the public entry point, it is no longer neccessary for Teleport Proxy to serve a publicly trusted cert (e.g. using
ACME). Thus Teleport Proxy should be welcome to use self-signed certs when sitting behind load balancers.

To accommodate that, instead of serving the **Web** certs, the ALPN server on the Proxy will serve the **Host** certs
when receiving connections from the upgrade flow. And instead of using the SystemCertPool, the Teleport client should
use the **Host** CA for verifying the Proxy server when dialing TLS Routing inside an upgrade connection. Note that this
modification does NOT apply to Teleport ALPN protocols that already do mTLS with other certs.

### When to make a connection upgrade

The client can performance a test to decide if a connection upgrade is necessary. The test is done by sending a TLS
handshake with a Teleport custom ALPN to the Proxy server:
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
Connection upgrade for TLS Routing should be required when the client and the server fail to neogiate the ALPN protocol,
hinting there is a load balancer in the middle that terminates TLS.

However, this test may not be bullet proof. Different load balancers (like the one million [Kubernetes Ingress
Controllers](https://docs.google.com/spreadsheets/d/191WWNpjJ2za6-nbG4ZoUMXMpUK8KlCIosvQB0f-oq3k/)) can behave
differently for the test. An environment variable (e.g. `TELEPORT_TLS_ROUTING_UPGRADE_MODE=off`) will be provided to
overwrite the mode per client, just in case.

### Supporting all TLS Routing protocols

In general, in ALL existing places we are dialing the TLS Routing connection, common logic will be added to the
`ContextDialer` to automatically upgrade the connection when necessary.

However, not all connections are initiated by `tsh`, `teleport`, or a client usingthe API lib. In such cases, a [local
Teleport
proxy](https://github.com/gravitational/teleport/blob/master/rfd/0039-sni-alpn-teleport-proxy-routing.md#local-teleport-proxy)
is required to dial the TLS Routing connection with the upgrade.

Local proxy has already been implemented for Database Access and Application Access. Here are some details how it can be
implemented for Kubernetes Access.

#### Local proxy for Kubernetes Access

`tsh` will serve a local HTTPS proxy server with a localhost self-signed CA. An ephemeral (per `tsh` command) KUBECONFIG
will be provided to Kubernetes applications to connect to the local server. This KUBECONFIG points to the local proxy
using the cluster `server` field, and the config will include static user credentials generated with the local
self-signed CA.

```
 ┌────┐                 ┌─────────┐             ┌────────┐
 │kube│                 │tsh local│             │Teleport│
 │app │                 │  proxy  │             │ Proxy  │
 └┬───┘                 └─┬───────┘             └──────┬─┘
  │                       │                            │
  │                       ├──┐                         │
  │                       │  │generate:                │
  │                       │  │local CA                 │
  │                       │  │local client credentials │
  │                       │◄─┘local KUBECONFIG         │
  │                       │                            │
  │ server localhost:8888 │                            │
  │ local credentials     │                            │
  ├──────────────────────►│  server kube-xxx.xxx.xxx   │
  │                       │  "real" credentials        │
  │                       ├───────────────────────────►│
  │                       │                            │
  │                       │◄───────────────────────────┤
  │◄──────────────────────┤                            │
  │                       │                            │
  │ another request       │                            │
  ├──────────────────────►│                            |
  │                       ├───────────────────────────►│
  ...                   
```

Once the local server receives a request and verifies the request, the local server will proxy the request using the TLS
Routing dialer to connect the Teleport Proxy. The local proxy should be in charge of managing the Teleport cert used for
routing Kubernetes requests.

Note that `kube-teleport-proxy-alpn.my-teleport-cluster.com` is not required to be resolvable (by DNS) in this case, but
it will be used internally for routing.

### User Experience

Once the connection upgrade support to all protocols are implemented, users can be recommended to upgrade their Teleport
clusters to single port mode if the cluster is currently serving separate ports.

All scenarios discussed in this section assumes Teleport sits behind a load balancer so connection upgrade for TLS
Routing is required.

#### 1 - Database Access UX

Common flows already use `tsh` local proxy (since v8) so they will automatically perform the connection upgrade:

- `tsh db connect` (single port mode)
- `tsh proxy db`

So no UX change to these.

In single port mode, using native clients to connect to Teleport Proxy directly with `tsh db env/config` will NOT work
through load balancers. Users can use one of the above methods instead.

#### 2 - Application Access UX

Application Access through Teleport Webapp is NOT affected by the issue of interest of this RFD.

On the `tsh` side, TCP, AWS, Azure, and GCP apps always use a local proxy so they will automatically perform the
connection upgrade when needed. No UX change to these.

However, HTTP apps MUST use `tsh proxy app`.

#### 3 - Kubernetes Access UX

Using native Kubernetes clients after `tsh kube login` will NOT work as the native clients cannot upgrade the
connection.

`tsh kubectl` will automatically starts a local proxy and performs the connection upgrade when needed.

A new `tsh proxy kube` command will be added to support other Kubernetes clients. The command will start a local proxy
and provide a config for Kubernetes clients to use:

```
$ tsh proxy kube -p 8888

Started local Kubernetes server at https://localhost:8888.

Please use the following KUBECONFIG for your Kubernetes applications:
export KUBECONFIG=<path to kubeconfig>
```

A `--exec` flag will be provided to execute a command backed by the local proxy:
```
$ tsh proxy kube --exec helm install my-chart
```

In addition, we can provide tips to users to improve their UX by utilizing `alias`:
```
alias kubectl=`tsh kubectl`
alias helm=`tsh proxy kube --exec helm`
```

or [`tsh` aliases](https://github.com/gravitational/teleport/blob/master/rfd/0061-tsh-aliases.md):
```
aliases:
    "helm": "$TSH proxy kube --exec helm"
```

#### 4 - Server Access UX

No UX change.

### Security

When upgrading the connection, Teleport client verifies the load balancer's TLS cert using SystemCertPool. And as
mentioned early, at the TLS Routing request, the Teleport client will be configured with a Teleport CA for verifying the
Proxy server.

There is no authentication at the connection upgrade request. Authentication is deferred to the TLS Routing request so
authentication remains the same as if there is no connection upgrade.

### Performance

The downside doing the connection upgrade is the performance penalty.

The connection is already double encrypted with TLS Routing, and now it is tripple encrypted with connection upgrade.
Mordern processors should have no trouble doing the job, but concurrent TLS Routing requests with connection upgrades
may affect CPU usage.

The more noticeable impact is likely the latency incurred by the **extra roundtrips** by the connection upgrade. A quick
API call throub connection upgrade by `tsh` may take double the time as before, since the latency between `tsh` and
Teleport Proxy is usually more significant than the latency within the Teleport agents.

Some ideas for improving performance includes implementing resuable upgraded connections and multiplexing concurrent TLS
Routing requests. This RFD should be updated once we have more detailed plans on performance improvements.

### Keepalive

Some load balancers (e.g. AWS ALB) can drop a connection when the traffic is idle at Layer 7 for a period of time. In
such cases, using short TCP keepalive would not help maintain long-lived connections.

For now, the connection upgrade flow assumes the tunneled Teleport ALPN protocol either handles keepalives on their own
or is not long-lived. [TLS Routing Ping](https://github.com/gravitational/teleport/blob/master/rfd/0081-tls-ping.md) has
been added to all database protocols to prevent idle timeouts.
