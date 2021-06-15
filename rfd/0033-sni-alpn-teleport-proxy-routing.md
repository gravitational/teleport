---
authors: Marek Smoliński (marek@goteleport.com)
state: draft
---

# RFD 33 - SNI and ALPN teleport proxy routing

## What

Combine all teleport proxy ports into one by routing proxy incoming traffic based on SNI and ALPN values.

## Why

Simplification of teleport proxy configuration, ability to expose only one teleport proxy port
through a firewall/load balancer.


## Details

| Port | Service | Description |
| --- | --- | --- |
| 3026  | **Proxy**  | HTTPS Kubernetes proxy proxy_service.kube_listen_addr  |
| 3023  | **Proxy**  | SSH port clients connect to. A proxy will forward this connection to port #3022 on the destination node.  |
| 3024  | **Proxy**  |  SSH port used to create "reverse SSH tunnels" from behind-firewall environments into a trusted proxy server. |
| 3080  | **Proxy**  | HTTPS connection to authenticate tsh users and web users into the cluster. The same connection is used to serve a Web UI. |
| MySQL Port | **Proxy**  | Handles MySQL client connection. |
| Postgres Port| **Proxy**  | Handles Postgres client connection. By default Postgres is multiplexed with Proxy Web Port 3080 |
| 3022  | Node  | SSH port. This is Teleport's equivalent of port #22 for SSH.  |
| 3025  | Auth  | SSH  port used by the Auth Service to serve its API to other nodes in a cluster.  |
| 3027  | Kubernetes  | Kubernetes Service kubernetes_service.listen_addr  |

In order to maintain backward compatibility a new proxy listener address (proxy_listener_addr) will be added to the proxy service alongside with current proxy listeners:
to `teleport.yaml` configuration:
```yaml
proxy_service:
   proxy_listener_addr: 0.0.0.0:443
```

where TLS handler listening on proxy_listener_addr will be responsible for routing incoming traffic to appropriate proxy service based on SNI and ALPN values:
```
                                                      ┌───────────────┐
                                                      │Teleport Proxy │
                                                      │   Listener    │
                                                      │               │
                                                      └───────────────┘
                                                              │
                                                              │
                                                              │
                                                              │
                                            ┌─────────────────┴─────────────────┐
                                            │                                   │
                                            │                                   │
                                            │                                   │
                                            │                                   │
                                            ▼                                   ▼
                             ┌────────────────────────────┐    ┌─────────────────────────────────┐
                             │SNI:                        │    │SNI:                             │
                             │  proxy.example.teleport.com│    │ kube.proxy.example.teleport.com │
                             │  default                   │    │                                 │
                             └────────────────────────────┘    └─────────────────────────────────┘
                                            │                                   │
                                            │                                   │
                                            │                                   │
                                            │                                   │
                                            │                                   │
                                            │                                   │
                                            │                                 ALPN:
                                            │                         http/1.1, h2, default
           ┌────────────────┬───────────────┴──┬───────────────┐                │
           │                │                  │               │                │
           │                │                  │               │                │
         ALPN:              │                  │ALPN:          │                │
    teleport-mysql          │            teleport-proxy-ssh    │                │
           │                │                  │               │                │
           │                │ALPN:             │             ALPN:              │
           │           teleport-postgres       │     http/1.1, h2, default       │
           │                │                  │               │                │
           ▼                ▼                  ▼               ▼                ▼
    ┌────────────┐  ┌───────────────┐  ┌───────────────┐  ┌─────────┐  ┌────────────────┐
    │MySQL proxy │  │Postgres proxy │  │ssh proxy      │  │web proxy│  │kube proxy      │
    │service     │  │service        │  │service        │  │service  │  │service         │
    └────────────┘  └───────────────┘  └───────────────┘  └─────────┘  └────────────────┘ 

```


### Reducing the number of teleport proxy port by TLS ALPN Routing:

ALPN (Application-Layer Protocol Negotiation) is a TLS extension that allows the application layer to negotiate which protocol should be performed over a TLS connection. Teleport proxy will leverage ALPN routing in order to reduce the number of exposed ports.

Outgoing traffic send to the ssh-proxy, reverse tunnel, MySQL, Postgres and MongoDB proxy services will be wrapped in TLS protocol where the teleport proxy clients (tsh, teleport internal libs, local db proxy) will be responsible for setting one of the following teleport protocols:
- teleport-ssh-proxy
- teleport-reverse-tunnel
- teleport-mysql
- teleport-postgres
- teleport-mongodb

The proxy server will listen on port 443 and will terminate the incoming TLS connection and forward the downstream traffic to the appropriate service based on the TLS ALPN negotiated protocol.

A downside of this approach is that traffic will be double encrypted, once by the TLS layer, and again by SSH, MySQL and Postgres protocols. This seems like a reasonable tradeoff for the external simplicity of only needing to run a single port and protocol for all connectivity.

### Reducing the number of teleport port by TLS SNI routing (Server Name Indication)

In order to expose WEB, Kubernetes proxy services in one teleport proxy port, the teleport proxy server will route the incoming HTTPS traffic based on SNI TLS value either to web service or Kubernetes service.

#### Setting SNI value for kubectl CLI:

`tsh kube login` command generates a local kubeconfig.yaml file used during accessing teleport proxy by kubectl CLI. Additional to the current configuration the  tls-server-name  failed will be added with appropriate SNI value:

```yaml
apiVersion: v1
clusters:
- cluster:
  certificate-authority-data: ...
  server: public_proxy_ip:443
  tls-server-name: kube.proxy.example.teleport.sh
  name: proxy.example.teleport.sh
```


### Local teleport proxy

To handle connections established to teleport proxy by external clients like mysql, psql CLIs where setting custom ALPN value is not possible we will run local teleport proxy. Traffic sent from external teleport clients will be forwarded through the local proxy where the proxy will be responsible for wrapping incoming connections in TLS protocol, setting appropriate ALPN protocol and establishing a connection to the remote teleport proxy.

For the tsh ssh and tsh scp commands scope forwarding traffic through local proxy seems to be superfluous. SSH traffic can be easily wrapped in TLS with custom teleport-ssh-proxy ALPN protocol inside tsh ssh handler after detecting that the teleport proxy_listener_addr listener was enabled but we should consider adding support for ssh proxy in order to enable external SSH and SCP clients like FileZilla, WinSCP and Putty.

Question:
- Do we want to support Unix sockets in local proxy ?


#### OpenSSH ProxyCommand
In order to support connection from OpenSSH clients where traffic send to cluster proxy is wrapped in TLS protocol the tsh binary will provide a new `tsh proxy ssh` subcommand that can be injected to ProxyCommand like `ssh -o "ForwardAgent yes" -o "ProxyCommand tsh proxy ssh" alice@nodeone.example.com"`
The `tsh proxy ssh` command will be responsible for establishing a TLS connection to cluster proxy with ALPN: teleport-reverse-tunnel protocol, forwarding ssh-agent and proxying traffic between an openssh client and cluster proxy.


### Reverse tunnel
Currently, reverse tunnel connection uses SSH protocol, and a separate cluster proxy port.  Reverse tunnels connection established from proxy client will be  wrapped in TLS  protocol with `teleport-reverse-tunnel` SNI where cluster proxy TLS listener will be
responsible for TLS termination and forwarding incoming SSH traffic to the `reversetunnel` teleport service where further remains unchanged.

Internal reverse tunnel dialer() will be aligned and an established reverse tunnel cluster proxy ssh connection will be wrapped in TLS

### CLI UX

#### kubectl CLI
kubectl CLI reads the cluster config from the `KUBECONFIG` file thus, UX remains unchanged. The proper SNI value will be set automatically based on `tls-server-name` field that will be set by `tsh kube login` command.

#### APP curl access
The app access UX will remain unchanged, `curl` already sets `ALPN` protocols to `h2,http/1.1` and SNI to the destination URL host.
```
curl \
  --cacert /Users/marek/.tsh/keys/proxy.example.teleport/certs.pem \
  --cert /Users/marek/.tsh/keys/proxy.example.teleport/alice-app/root/grafana-x509.pem \
  --key /Users/marek/.tsh/keys/proxy.example.teleport/alice \
  https://grafana.example.teleport.sh
```

#### tsh ssh node && tsh scp
UX remains unchanged. The `onSSH` and `onSCP` tsh commands handlers will be extended, and the connection established to cluster proxy will be wrapped in TLS protocol inside commands handler.

#### OpenSSH client
A new `tsh proxy ssh` command will be introduced allowing for injection to `ProxyCommand` openssh client command:
```
ssh -o "ForwardAgent yes" \
    -o "ProxyCommand tsh proxy ssh" \
    alice@node.example.teleport.sh
```


#### tsh db connect db-instance
`tsh db connect` command introduced in [#7213]([https://github.com/gravitational/teleport/pull/7213) will be extended. After detecting that cluster proxy support SNI ALPN routing
the `db connect` will start on-demand local proxy and connect to the database through the local proxy using one psql, mysql or mongo shells. When a user exits from the db cli shell a local proxy will be automatically terminated.

```
                                                             
         ┌─────────────┐                         
         │   DB CLI    │                         
         └─┬────────▲──┘                         
           │        │                            
       1) DB     12) DB                          
      Protocol  Protocol                         
           │        │                            
         ┌─▼────────┴──┐                         
         │ Local Proxy │                         
         └──┬────────▲─┘                         
            │        │                           
       2) TLS SNI 11) TLS                        
            │        │                           
            │        │                           
         ┌──▼────────┴─┐                         
         │Cluster Proxy│                         
         └──┬────────▲─┘                         
            │        │                           
        3) DB     10) DB                         
       Protocol  Protocol                        
            │        │                           
         ┌──▼────────┴─┐                         
         │  DB Proxy   │                         
         │   Service   │                         
         └──┬────────▲─┘                         
            │        │                           
         4) DB    9) DB                          
        Protocol Protocol                        
            │        │   5) Auth                 
         ┌──▼────────┴─┐   Req    ┌─────────────┐
         │             ├──────────▶             │
         │ DB Service  │ 6) Auth  │    Auth     │
         │             ◀───Resp───┤             │
         └──┬────────▲─┘          └─────────────┘
            │        │                           
         7) DB      8) DB                        
        Protocol   Protocol                      
            │        │                           
         ┌──▼────────┴─┐                         
         │ DB Instance │                         
         │             │                         
         └─────────────┘
```

#### tsh proxy db start db-protocol [-p port]

In order to support connection for standalone DB clients the `tsh proxy db db-protocol` command will provide ability to start run local proxy manually allowing external DB clients to connect to cluster proxy TLS port through the local proxy.
