---
authors: Marek Smoliński (marek@goteleport.com)
state: draft
---

# RFD 33 - SNI and ALPN telepot proxy routing

## What

Combine all teleport proxy ports into one by routing proxy incoming traffic based on SNI and ALPN values.

## Why

Simplification of teleport proxy configuration, ability to expose ony one teleport proxy port 
through firewall/load balancer.


## Details

| Port | Service | Description |
| --- | --- | --- |
| 3026  | **Proxy**  | HTTPS Kubernetes proxy proxy_service.kube_listen_addr  |
| 3023  | **Proxy**  | SSH port clients connect to. A proxy will forward this connection to port #3022 on the destination node.  |
| 3024  | **Proxy**  |  SSH port used to create "reverse SSH tunnels" from behind-firewall environments into a trusted proxy server. |
| 3080  | **Proxy**  | HTTPS connection to authenticate tsh users and web users into the cluster. The same connection is used to serve a Web UI. |
| MySQL Port | **Proxy**  | Handles MySQL client connection. |
| Postgres Port| **Proxy**  | Handles Postgres client connection. |
| 3022  | Node  | SSH port. This is Teleport's equivalent of port #22 for SSH.  |
| 3025  | Auth  | SSH  port used by the Auth Service to serve its API to other nodes in a cluster.  |
| 3025  | Kubernetes  | Kubernetes Service kubernetes_service.listen_addr  |

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
                                            │                         http1.1, h2, default        
           ┌────────────────┬───────────────┴──┬───────────────┐                │                 
           │                │                  │               │                │                 
           │                │                  │               │                │                 
         ALPN:              │                  │ALPN:          │                │                 
    teleport-mysql          │            teleport-proxy-ssh    │                │                 
           │                │                  │               │                │                 
           │                │ALPN:             │             ALPN:              │                 
           │           teleport-postgres       │     http1.1, h2, default       │                 
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
  tls-server-name: kube.proxy.example.teleport.com
  name: proxy.example.teleport.com
```


### Local teleport proxy

To handle connections established to teleport proxy by external clients like mysql, psql CLIs where setting custom ALPN value is not possible we will run local teleport proxy. Traffic sent from external teleport clients will be forwarded through the local proxy where the proxy will be responsible for wrapping incoming connections in TLS protocol, setting appropriate ALPN protocol and establishing a connection to the remote teleport proxy.

For the tsh ssh and tsh scp commands scope forwarding traffic through local proxy seems to be superfluous. SSH traffic can be easily wrapped in TLS with custom teleport-ssh-proxy ALPN protocol inside tsh ssh handler after detecting that the teleport proxy_listener_addr listener was enabled but we should consider adding support for ssh proxy in order to enable external SSH and SCP clients like FileZilla, WinSCP and Putty.

Question:
- Do we want to support Unix sockets in local proxy  ? 

### CLI UX 

#### tsh proxy connect db-instance-name

Command will start an on-demand local proxy and connect to the database through the local proxy using one psql, mysql or mongo shells. When a user exits from the db cli shell a local proxy will be automatically terminated. 

```
                                                                                                                                            +------------+                                 
                                                                                                                                            |            |                                 
                                                                                                                                            |    Auth    |                                 
                                                                                                                                            |            |                                 
                                                                                                                                            +--^------|--+                                 
                                                                                                                                               |      |                                    
                                                                                                                                               |      |                                    
                                                                                                                                        5) Auth Req 6) Auth Resp                           
                                                                                                                                               |      |                                    
                                               teleport-db-protocol                                                                            |      |                                    
+-----------+  1) DB Protocol     +-----------+               +-----------------+  3) DB Protocol  +-------------------+   4) DB Protocol   +--|------v--+  7) DB Protocol+---------------+
|           ---------------------->           ---------------->                 ------------------->                   --------------------->            ----------------->               |
| DB Client |  12) DB Protocol    | Local DB  | 11) TLS       | Remote Teleport |  10) DB Protocol |  DB Proxy Service |   9) DB Protocol   | DB Service |  8) DB Protocol|  DB Instance  |
|           <----------------------   Proxy   <----------------     Proxy       <-------------------                   <---------------------            <-----------------               |
+-----------+                     +-----------+               +-----------------+                  +-------------------+                    +------------+                +---------------+
```

#### tsh proxy start db db-instance-name [-p port] 

Start a local DB proxy on a dedicated port allowing external DB clients to connect to the DB instance through the proxy. 
