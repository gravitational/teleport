# Architecture

This document covers the underlying design principles of Teleport and a detailed 
description of Teleport architecture.

## High Level Overview

### Design Principles

Teleport was designed in accordance with the following design principles:

* **Off the Shelf Security**: Teleport does not re-implement any security primitives
  and uses well-established, popular implementations of the encryption and network protocols.

* **Open Standards**: There is no security through obscurity. Teleport is fully compatible
  with existing and open standards and other software, including OpenSSH.

* **Cluster-oriented Design**: Teleport is built for managing clusters, not individual
  servers. In practice this means that hosts and users have cluster memberships. Identity 
  management and authorization happen on a cluster level.

* **Built for Teams**: Teleport was created under the assumption of multiple teams operating
  on several disconnected clusters (production-vs-staging, or perhaps
  on a cluster-per-customer or cluster-per-application basis).

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
Teleport auth is running. Adding new nodes or inviting new users to the cluster is only
possible using this tool.

Once nodes and users (clients) have been invited to the cluster, lets go over the sequence
of network calls performed by Teleport components when the client tries to connect to the 
node.

1. The client tries to establish an SSH connection to a proxy using either the CLI interface or a 
   web browser (via HTTPS). Clients must always connect through a proxy for two reasons:

      * Individual nodes may not always be reachable from "the outside".
      * Proxies always record SSH sessions and keep track of active user sessions. This makes it possible
      for an SSH user to see if someone else is connected to a node she is about to work on.

      When establishing a connection, the client offers its public key.

2. The proxy checks if the submitted certificate has been previously signed by the auth server. 
   If there was no key previously offered (first time login) or if the certificate has expired, the 
   proxy denies the connection and asks the client to login interactively using a password and a 
   2nd factor.
   
      Teleport uses [Google Authenticator](https://support.google.com/accounts/answer/1066447?hl=en) 
      for the two-step authentication.
   
      The password + 2nd factor are submitted to a proxy via HTTPS, therefore it is critical for 
      a secure configuration of Teleport to install a proper HTTPS certificate on a proxy.
     
    **WARNING:** Do not use a self-signed certificate in production!
  
    If the credentials are correct, the auth server generates and signs a new certificate and returns
    it to a client via the proxy. The client stores this key and will use it for subsequent 
    logins. The key will automatically expire after 23 hours by default. This TTL can be configured
    to a maximum of 30 hours and a minimum of 1 minute.

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
      * User's node-level permissions are validated before authorizing her to interact with SSH 
      subsystems.

**Detailed Diagram of a Teleport cluster**

![Teleport Everything](img/everything.svg)

## Teleport Services

Lets explore each of the Teleport services in detail.

### The Auth Service

The `auth server` acts as a certificate authority (CA) of the cluster. Teleport security is 
based on SSH certificates and every certificate must be signed by the cluster auth server.

There are two types of certificates the auth server can sign:

* **Host certificates** are used to add new nodes to a cluster.
* **User certificates** are used to authenticate users when they try to login into a cluster node.

Upon initialization the auth server generates a public / private keypair and stores it in the 
configurable key storage. The auth server also keeps the records of what has been happening
inside the cluster: it stores recordings of all SSH sessions in the configurable events 
storage.

![Teleport Auth](img/auth-server.svg)

When a new node joins the cluster, the auth server generates a new public / private keypair for 
the node and signs its certificate.

To join a cluster for the first time, a node must present a "join token" to the auth server.
The token can be static (configured via a config file) or a dynamic, single-use token.

!!! tip "NOTE": 
    When using dynamic tokens, their default time to live (TTL) is 15 minutes, but it can be 
    reduced (not increased) via `tctl` flag.

Nodes that are members of a Teleport cluster can interact with the auth server using the auth API. 
The API is implemented as an HTTP REST service running over the SSH tunnel, authenticated using host 
or user certificates previously signed by the auth server which acts as a certificate authority for 
the cluster.

All nodes of the cluster send periodic ping messages to the auth server, reporting their
IP addresses and values of their assigned labels. The list of connected cluster nodes is accessible
to all members of the cluster via the API.

Clients can also connect to the auth API through Teleport proxy to use a limited subset of the API to 
discover the member nodes of the cluster.

Cluster administration is performed using `tctl` command line tool.

!!! tip "NOTE": 
    For high availability in production, a Teleport cluster can be serviced by multiple auth servers 
    running in sync. Check [HA configuration](admin-guide.md#high-availability-and-clustering) in the 
    Admin Guide.

### Cluster State

Each cluster node is completely stateless and holds no secrets such as keys, passwords, etc. 
The persistent state of a Teleport cluster is kept by the auth server. There are three types
of data stored by the auth server:

* **Key storage**. As described above, a Teleport cluster is a set of machines whose public keys are 
  signed by the same certificate authority (CA), with the auth server acting as the CA of a cluster.
  The auth server stores its own keys in a key storage. Teleport supports multiple storage back-ends
  to store secrets, including the file-based storage or databases like [BoltDB](https://github.com/boltdb/bolt), 
  [DynamoDB](https://aws.amazon.com/dynamodb/) or [etcd](https://github.com/coreos/etcd).  
  Implementing another key storage backend is simple, see `lib/backend` directory in Teleport source code.

* **Audit Log**. As users login into a Teleport cluster, execute remote commands and logout,
  all that activity is recorded in the audit log. See [Audit Log](admin-guide.md#audit-log) 
  for more details.
  
* **Recorded Sessions**. When Teleport users launch remote shells via `tsh ssh` command, their 
  interactive sessions are recorded and stored by the auth server in a session storage. Each recorded 
  session is a file which, by default, is saved in `/var/lib/teleport`.


### The Proxy Service

The proxy is a stateless service which performs two functions in a Teleport cluster: 

1. It serves a Web UI which is used by cluster users to sign up and configure their accounts, 
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

.. warning:: It is absolutely crucial to properly configure TLS for HTTPS when you 
prepare to use Teleport Proxy in production.


#### Web to SSH Proxy

In this mode Teleport Proxy implements WSS (secure web sockets) to SSH proxy:

![Teleport Proxy Web](img/proxy-web.svg)

1. User logs in using username, password and 2nd factor token to the proxy.
2. Proxy passes credentials to the auth server's API
3. If auth server accepts credentials, it generates a new web session and generates a special
   ssh keypair associated with this web session.
   Auth server starts serving [OpenSSH ssh-agent protocol](https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.agent)
   to the proxy.
4. From the SSH node's perspective it's a regular SSH client connection that is authenticated using
   OpenSSH certificate, so no special logic is needed.

!!! tip "NOTE": 
    Unlike in SSH proxying, in web mode Teleport Proxy terminates the traffic and re-encodes for SSH client connection.

#### SSH Proxy

**1. Getting signed short lived certificate**

Teleport Proxy implements a special method to let clients get short lived certificates signed by auth's host certificate authority:

![Teleport Proxy SSH](img/proxy-ssh-1.svg)

1. TSH client or TSH agent generate OpenSSH keypair and forward generated public key and username, password and second factor token that are entered by user to the proxy.
2. Proxy forwards request to the auth server.
3. If auth server accepts credentials, it generates a new certificate signed by its user CA and sends it back to the proxy.
4. Proxy returns the user certificate to the client and client stores it in `~/.tsh/keys`

!!! tip "NOTE": 
    Unlike in SSH proxying, in web mode Teleport Proxy terminates the traffic and re-encodes for SSH client connection.

**2. Connecting to the nodes**

![Teleport Proxy Web](img/proxy-ssh-2.svg)

Once client has obtained short lived certificate, it can use it to authenticate with any node in the cluster. Users can use
the certificate using standard OpenSSH client (and get it using ssh-agent socket served by `tsh agent`) or using `tsh` directly:

1. SSH client connects to proxy and executes `proxy` subsystem of the proxy's SSH server providing target node's host and port location.
2. Proxy dials to the target TCP address and starts forwarding the traffic to the client.
3. SSH client uses established SSH tunnel to open new SSH connection and authenticate with the target node using its client certificate.

!!! tip "NOTE": 
    Teleport's proxy command makes it compatible with [SSH jump host](https://wiki.gentoo.org/wiki/SSH_jump_host) pattern implemented using OpenSSH's `ProxyCommand`


## Certificates

Teleport uses standard Open SSH certificates for client and host authentication.

### Host certificates and roles

Nodes, proxy and auth servers use certificates to authenticate with auth server and user's client connections.
Users should check if host's certificate is signed by the trusted authority.

Each role `proxy`, `auth` or `node` is encoded in the generated certificate using certificate extensions (opaque signed string).
All nodes in the cluster can connect to auth server's HTTP API via SSH tunnel that checks each connecting client's certificate and role
to enforce access control (e.g. client connection using node's certificate won't be able to add and delete users, and
can only get auth servers registered in the cluster).

### User certificates and allowed logins

When auth server generates a user certificate, it uses information provided by administrator about allowed linux logins
to populate the list of "valid principals". This list is used by OpenSSH and Teleport to check if the user has option
to log in as a certain OS user.

Teleport's user name is stored as a OpenSSH key id field.

User's certificates do not use any cert extensions as a workaround to the [bug](https://bugzilla.mindrot.org/show_bug.cgi?id=2387)
 that treats any extension as a critical one, breaking access to the cluster.
    

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

When `tsh` logs in, the auto-expiring key is stored in `~/.tsh` and is valid for 23 hours by
default, unless you specify another interval via `--ttl` flag (max of 30 hours and minimum of 1 minute and capped by the server-side configuration).

You can learn more about `tsh` in the [User Manual](user-manual.md).

### TCTL

`tctl` is used to administer a Teleport cluster. It connects to the `auth server` listening
on `127.0.0.1` and allows cluster administrator to manage nodes and users in the cluster.

You can learn more about `tctl` in the [Admin Manual](admin-guide.md).
