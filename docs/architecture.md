# Architecture

This document covers the underlying design principles of Teleport and a detailed 
description of Teleport architecture.

## High Level Overview

### Design Principles

Teleport was designed in accordance with the following design principles:

* **Off the Shelf Security**. Teleport does not re-implement any security primitives
  and uses well-established, popular implementations of the encryption and network protocols.

* **Open Standards**. There is no security through obscurity. Teleport is fully compatible
  with existing and open standards and other software, including OpenSSH.

* **Cluster-oriented Design**. Teleport is built for managing clusters, not individual
  servers. In practice this means that hosts and users have cluster memberships. Identity 
  management and authorization happen on a cluster level.

* **Built for Teams**. Teleport was created under the assumption of multiple teams operating
  on several disconnected clusters, for example production-vs-staging, or perhaps
  on a cluster-per-customer or cluster-per-application basis.

### Core Concepts

There are three types of services (roles) in a Teleport cluster. 

| Service(Role)  | Description
|----------------|------------------------------------------------------------------------
| node   | This role provides the SSH access to a node. Typically every machine in a cluster runs `teleport` with this role. It is stateless and lightweight.
| proxy  | The proxy accepts inbound connections from the clients and routes them to the appropriate nodes. The proxy also serves the Web UI.
| auth   | This service provides authentication and authorization service to proxies and nodes. It is the certificate authority (CA) of a cluster and the storage for audit logs. It is the only stateful component of a Teleport cluster.

Although `teleport` daemon is a single binary, it can provide any combination of these services 
via `--roles` command line flag or via the configuration file.

In addition to `teleport` daemon, there are three client tools you will use:

| Tool           | Description
|----------------|------------------------------------------------------------------------
| tctl    | Cluster administration tool used to invite nodes to a cluster and manage user accounts. `tctl` must be used on the same machine where `auth` is running.
| tsh     | Teleport client tool, similar in principle to OpenSSH's `ssh`. Use it to login into remote SSH nodes, list and search for nodes in a cluster, securely upload/download files, etc. `tsh` can work in conjunction with `ssh` by acting as an SSH agent.
| browser | You can use your web browser to login into any Teleport node, just open `https://<proxy-host>:3080` (`proxy-host` is one of the machines that has proxy service enabled).

### Cluster Overview

Lets explore how these services come together and interact with Teleport clients and with each other. 

**High Level Diagram of a Teleport cluster**

![Teleport Overview](img/overview.svg)

Notice that the Teleport Admin tool must be physically present on the same machine where
Teleport Auth is running. Adding new nodes or inviting new users to the cluster is only
possible using this tool.

Once nodes and users (clients) have been invited to the cluster, lets go over the sequence
of network calls performed by Teleport components when the client tries to connect to the 
node.

1. The client tries to establish an SSH connection to a proxy using either the CLI interface or a 
   web browser (via HTTPS). Clients must always connect through a proxy for two reasons:

   * Individual nodes may not always be reacheable from "the outside".
   * Proxies always record SSH sessions and keep track of active user sessions. This makes it possible
     for an SSH user to see if someone else is connected to a node she is about to work on.

   When establishing a connection, the client offers its public key.

2. The proxy checks if the submitted public key has been previously signed by the auth server. 
   If there was no key previously offered (first time login) or if the key certificate has expired, the 
   proxy denies the connection and asks the client to login interactively using a password and a 
   2nd factor.

   Teleport uses [Google Authenticator](https://support.google.com/accounts/answer/1066447?hl=en) 
   for the second authentication.

   The password plus second factor are submitted to a proxy via HTTPS, therefore it is critical to install a 
   proper HTTPS certificate on the proxy for a secure configuration of Teleport. 
   **DO NOT** use the self-signed certificate installed by default.

   If the credentials are correct, the auth server generates and signs a new certificate and returns
   it to a client via the proxy. The client stores this key and will use it for subsequent 
   logins. The key will automatically expire after 22 hours. In the future, Teleport will support
   a configurable TTL of these temporary keys.

3. At this step, the proxy tries to locate the requested node in a cluster. There are three
   lookup mechanisms a proxy uses to find the node's IP address:

   * Tries to resolve the name requested by the client.
   * Asks the auth server if there is a node registered with this `nodename`.
   * Asks the auth server to find a node (or nodes) with a label that matches the requested name.

   If the node is located, the proxy establishes the connection between the client and the
   requested node and begins recording the session, sending the session history to the auth
   server to be stored.

4. When the node receives a connection request, it too checks with the auth server to validate 
   the submitted client certificate. The node also requests the auth server to provide a list
   of OS users (user mappings) for the connecting client, to make sure the client is authorized 
   to use the requested OS login.
   
   In other words, every connection is authenticated twice before being authorized to log in:

   * User's cluster membership is validated when connecting a proxy.
   * User's cluster membership is validated again when connecting to a node.
   * User's node-level permissions are validated before authorizing him to interact with SSH 
     subsystems.

**Detailed Diagram of a Teleport cluster**

![Teleport Everything](img/everything.svg)

## Teleport Services

Lets explore each of the Teleport services in detail.

### The Auth Service

The `auth server` is the core of the Teleport cluster. It acts as a sertificate authority (CA)
of the cluster.

On an initial connection the auth server generates a public / private keypair and stores it in the 
configurable key storage. The auth server also keeps the records of what has been happening
inside the cluster: it stores recordings of all SSH sessions in the configurable events 
storage.

![Teleport Auth](img/auth-server.svg)

#### Auth API

When a new node (server) joins the cluster, a new public / private keypair is generated for that node, 
signed by the CA. To invite a node, the auth server generates a disposable one-time token which
the new node must submit when requesting its certificate for the first time.

Teleport cluster members (servers) can interact with the auth server using the Auth API. The API is 
implemented as an HTTP REST service running over the SSH transport, authenticated using host 
certificates previously signed by the CA.

All node-members of the cluster send periodic ping messages to the auth server, reporting their
IP addresses, values of their assigned labels. The list of connected cluster nodes is accessible
to all members of the cluster via the API.

Clients cannot connect to the auth API directly (because client computers do not have valid 
cluster certificates). Clients can interact with the auth API only via Teleport proxy.

Cluster administration is performed using `tctl` command line tool.

**Production note:** For high availability a Teleport cluster can be serviced by multiple auth servers 
running in sync. Check [HA configuration]() in the Admin Guide.

### Storage Backends

The persistent state of a Teleport cluster is stored by the auth server. There are three kinds
of data the auth server stores:

* **Key storage**. As described above, a Teleport cluster is a set of machines whose public keys are 
  signed by the same certificate authority (CA), with the auth server acting as the CA of a cluster.
  The auth server stores its own keys in a key storage. Currently there are two storage backends
  for keys: [BoldDB](https://github.com/boltdb/bolt) for single-node auth server, and 
  [etcd](https://github.com/coreos/etcd) for HA configurations. Implementing another key storage 
  backend is simple, see `lib/backend` directory in Teleport source code.

* **Event storage**. As users login into a Teleport cluster, execute remote commands and logout,
  all that activity is recorded as a live event stream. Teleport uses its Events Storage backend
  for storing it. Currently only BoltDB backend is supported for storing events.
  
* **Recorded Sessions**. When Teleport users launch remote shells via `tsh ssh` command, their 
  interactive sessions are recorded and stored by the auth server in a session storage. Each recorded 
  session is a file which, by default, is saved in `/var/lib/teleport`.


### The Proxy Service

The proxy is a stateless service which performs two functions in a Teleport cluster: 

1. It serves a Web UI which is used by cluster users to sign up and configure thehir accounts, 
   explore nodes in a cluster, login into remote nodes, join existing SSH sessions or replay 
   recorded sessions.

2. It serves as an authentication gateway, asking for user credentials and forwarding them
   to the auth server via Auth API. When a user executes `tsh --proxy=p ssh node` command,
   trying to login into "node", the `tsh` tool will establish HTTPS connection to the proxy "p"
   and authenticate before it will be given access to "node".

All user interactions with the Teleport cluster are done via a proxy service. It is
recommended to have several of them running.

When you launch the Teleport Proxy for the first time, it will generate self-signed HTTPS 
certificate to make it easier to explore Teleport.

**WARNING:** It is absolutely crucial to properly configure TLS for HTTPS when you 
prepare to use Teleport Proxy in production.


## Teleport Tools

Teleport users two command line tools. `tsh` is a client tool used by the end users, while
`tctl` is used for cluster administration.

### TSH

`tsh` is similar in nature to OpenSSH `ssh` or `scp`. In fact, it has subcommands named after
them so you can call:

```
> tsh --proxy=p ssh -p 1522 user@host
> tsh --proxy=p scp -P example.txt user@host/destination/dir
```

Unlike `ssh`, `tsh` is very opinionated about authentication: it always uses auto-expiring
keys and it always connects to Teleport nodes via a proxy, it is a mandatory parameter.

When `tsh` logs in, the auto-expiring key is stored in `~/.tsh` and is valid for 20 hours by
default, unless you specify another interval via `--ttl` flag (still capped by a server-side
configuration).

You can learn more about `tsh` in the [User Manual](docs/user-manual.md).

### TCTL

`tctl` is used to administer a Teleport cluster. It connects to the `auth server` listening
on `127.0.0.1` and allows cluster administrator to manage nodes and users in the cluster.

You can learn more about `tctl` in the [Admin Manual](docs/admin-guide.md).
