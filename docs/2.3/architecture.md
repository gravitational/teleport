# Architecture

This document covers the underlying design principles of Teleport and a detailed description of Teleport architecture.

## Design Principles

Teleport was designed in accordance with the following principles:

* **Off the Shelf Security**: Teleport does not re-implement any security primitives
  and uses well-established, popular implementations of the encryption and network protocols.

* **Open Standards**: There is no security through obscurity. Teleport is fully compatible
  with existing and open standards and other software, including OpenSSH.

* **Cluster-Oriented Design**: Teleport is built for managing clusters, not individual
  servers. In practice this means that hosts and users have cluster memberships. Identity 
  management and authorization happen on a cluster level.

* **Built for Teams**: Teleport was created under the assumption of multiple teams operating
  on several disconnected clusters (production-vs-staging, or perhaps
  on a cluster-per-customer or cluster-per-application basis).

## Core Concepts

The following core concepts are integral to understanding the Teleport architecture.

* **Cluster of Nodes**. Unlike traditional SSH service, Teleport operates on a _cluster_ of nodes.
   A cluster is a set of nodes (servers). There are several ramifications of this:
	* User identities and user permissions are defined and enforced on a cluster level.
	* A node must become a _cluster member_ before any user can connect to it via SSH.
	* SSH access to any cluster node is _always_ performed via a cluster proxy,
       sometimes called an "SSH bastion".

* **User Account**. Unlike traditional SSH, Teleport introduces the concept of a User Account.
   A User Account is not the same as SSH login. For example, there can be a Teleport user "johndoe" 
   who can be given permission to login as "root" to a specific subset of nodes.

* **Teleport Services**. A Teleport cluster consists of three separate services, also called 
   "node roles": `proxy`, `auth` and `node`. Each Teleport node can run any combination of them
   by passing `--role` flag to the `teleport` daemon.

* **User Roles**. Unlike traditional SSH, each Teleport user account is assigned a `role`.
   Having roles allows Teleport to implement role-based access control (RBAC), i.e. assign 
   users to groups (roles) and restrict each role to a subset of actions on a subset of
   nodes in a cluster.

* **Certificates**. Teleport uses SSH certificates to authenticate nodes and users within 
   a cluster. Teleport does not allow public key or password-based SSH authentication.

* **Dynamic Configuration**. Nearly everything in Teleport can be configured via the 
   configuration file, `/etc/teleport.yaml` by default. But some settings can be changed
   at runtime, by modifying the cluster state (eg., creating user roles or
   connecting to trusted clusters). These operations are called 'dynamic configuration'.

## User Accounts

Teleport supports two types of user accounts: 

* **Internal users** are created and stored in Teleport's own identitiy storage. A cluster
  administrator has to create account entries for every Teleport user. 
  Teleport supports second factor authentication (2FA) and it is enforced by default. 
  There are two types of 2FA supported:
    * [TOTP](https://en.wikipedia.org/wiki/Time-based_One-time_Password_Algorithm)
      is the default. You can use [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator) or 
      [Authy](https://www.authy.com/) or any other TOTP client.
    * [U2F](https://en.wikipedia.org/wiki/Universal_2nd_Factor).
* **External users** are users stored elsewhere else within an organization. Examples include
  Github, Active Directory (AD), OpenID/OAuth2 endpoint or behind SAML. 


!!! tip "Version Warning": 
    External user identities are only supported in Teleport Enterprise. Please
    take a look at [Teleport Enterprise](enterprise.md) chapter for more information.

## Teleport Cluster

Lets explore how these services come together and interact with Teleport clients and with each other. 

**High Level Diagram of a Teleport cluster**

![Teleport Overview](img/overview.svg)

Notice that the Teleport Admin tool must be physically present on the same machine where
Teleport auth is running. Adding new nodes or inviting new users to the cluster is only
possible using this tool.

Once nodes and users (clients) have been invited to the cluster, lets go over the sequence
of network calls performed by Teleport components when the client tries to connect to the 
node.

1) The client tries to establish an SSH connection to a proxy using either the CLI interface or a 
   web browser (via HTTPS). When establishing a connection, the client offers its public key. Clients must always connect through a proxy for two reasons:

* Individual nodes may not always be reachable from "the outside".
* Proxies always record SSH sessions and keep track of active user sessions. This makes it possible for an SSH user to see if someone else is connected to a node she is about to work on.

2) The proxy checks if the submitted certificate has been previously signed by the auth server. 
   If there was no key previously offered (first time login) or if the certificate has expired, the 
   proxy denies the connection and asks the client to login interactively using a password and a 
   2nd factor.
   
 Teleport uses [Google Authenticator](https://support.google.com/accounts/answer/1066447?hl=en) 
    for the two-step authentication. The password + 2nd factor are submitted to a proxy via HTTPS, therefore it is critical for a secure configuration of Teleport to install a proper HTTPS certificate on a proxy.
 
!!! warning "Warning": 
	Do not use a self-signed certificate in production!
    If the credentials are correct, the auth server generates and signs a new certificate and returns
    it to a client via the proxy. The client stores this key and will use it for subsequent 
    logins. The key will automatically expire after 23 hours by default. This TTL can be configured
    to a maximum of 30 hours and a minimum of 1 minute.

3) At this step, the proxy tries to locate the requested node in a cluster. There are three
   lookup mechanisms a proxy uses to find the node's IP address:

* Tries to resolve the name requested by the client.
* Asks the auth server if there is a node registered with this `nodename`.
* Asks the auth server to find a node (or nodes) with a label that matches the requested name.

If the node is located, the proxy establishes the connection between the client and the
requested node and begins recording the session, sending the session history to the auth
server to be stored.

4) When the node receives a connection request, it too checks with the auth server to validate 
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


### Cluster State

Each cluster node is completely stateless and holds no secrets such as keys, passwords, etc. 
The persistent state of a Teleport cluster is kept by the auth server. There are three types
of data stored by the auth server:

* **Key storage**. As described above, a Teleport cluster is a set of machines whose public keys are 
  signed by the same certificate authority (CA), with the auth server acting as the CA of a cluster.
  The auth server stores its own keys in a key storage. Teleport supports multiple storage back-ends
  to store secrets, including the file-based storage or databases like [BoltDB](https://github.com/boltdb/bolt), 
  [DynamoDB](https://aws.amazon.com/dynamodb/) or [etcd](https://github.com/coreos/etcd). Implementing another key storage backend is simple, see `lib/backend` directory in Teleport source code.

* **Audit Log**. When users log into a Teleport cluster, execute remote commands and logout,
  that activity is recorded in the audit log. See [Audit Log](admin-guide.md#audit-log) 
  for more details.
  
* **Recorded Sessions**. When Teleport users launch remote shells via `tsh ssh` command, their 
  interactive sessions are recorded and stored by the auth server. Each recorded 
  session is a file which is saved in `/var/lib/teleport`, by default.


## Teleport Services

There are three types of services (roles) in a Teleport cluster. 

| Service (Node Role)  | Description
|----------------|------------------------------------------------------------------------
| node   | This role provides the SSH access to a node. Typically every machine in a cluster runs this role. It is stateless and lightweight.
| proxy  | The proxy accepts inbound connections from the clients and routes them to the appropriate nodes. The proxy also serves the Web UI.
| auth   | This service provides authentication and authorization service to proxies and nodes. It is the certificate authority (CA) of a cluster and the storage for audit logs. It is the only stateful component of a Teleport cluster.

Although `teleport` daemon is a single binary, it can provide any combination of these services 
via `--roles` command line flag or via the configuration file.

In addition to `teleport` daemon, there are three client tools you will use:

| Tool           | Description
|----------------|------------------------------------------------------------------------
| tctl    | Cluster administration tool used to invite nodes to a cluster and manage user accounts. `tctl` must be used on the same machine where `auth` is running.
| tsh     | Teleport client tool, similar in principle to OpenSSH's `ssh`. Use it to log into remote SSH nodes, list and search for nodes in a cluster, securely upload/download files, etc. `tsh` can work in conjunction with `ssh` by acting as an SSH agent.
| Web browser | You can use your web browser to log into any Teleport node, just open `https://<proxy-host>:3080` (`proxy-host` is one of the machines that has proxy service enabled).

Let's explore each of the Teleport services in detail.

### The Auth Service

The auth server acts as a certificate authority (CA) of the cluster. Teleport security is 
based on SSH certificates and every certificate must be signed by the cluster auth server.

There are two types of certificates the auth server can sign:

* **Host certificates** are used to add new nodes to a cluster.
* **User certificates** are used to authenticate users when they try to log into a cluster node.

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
or user certificates previously signed by the auth server.

All nodes of the cluster send periodic ping messages to the auth server, reporting their
IP addresses and values of their assigned labels. The list of connected cluster nodes is accessible
to all members of the cluster via the API.

Clients can also connect to the auth API through Teleport proxy to use a limited subset of the API to 
discover the member nodes of the cluster.

Cluster administration is performed using `tctl` command line tool.

!!! tip "NOTE": 
    For high availability in production, a Teleport cluster can be serviced by multiple auth servers 
    running in sync. Check [HA configuration](admin-guide.md#high-availability) in the 
    Admin Guide.


### The Proxy Service

The proxy is a stateless service which performs two functions in a Teleport cluster: 

1. It serves a Web UI which is used by cluster users to sign up and configure their accounts, 
   explore nodes in a cluster, log into remote nodes, join existing SSH sessions or replay 
   recorded sessions.

2. It serves as an authentication gateway, asking for user credentials and forwarding them
   to the auth server via Auth API. When a user executes `tsh --proxy=p ssh node` command,
   trying to log into "node", the `tsh` tool will establish HTTPS connection to the proxy "p"
   and authenticate before it will be given access to "node".

All user interactions with the Teleport cluster are done via a proxy service. It is
recommended to have several of them running.

When you launch the Teleport Proxy for the first time, it will generate a self-signed HTTPS 
certificate to make it easier to explore Teleport.

!!! warning "Warning": 
	It is absolutely crucial to properly configure TLS for HTTPS when you use Teleport Proxy in production.


### Web to SSH Proxy

In this mode, Teleport Proxy implements WSS (secure web sockets) to SSH proxy:

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

### SSH Proxy

#### Getting signed short lived certificate

Teleport Proxy implements a special method to let clients get short lived certificates signed by auth's host certificate authority:

![Teleport Proxy SSH](img/proxy-ssh-1.svg)

1. TSH client or TSH agent generate OpenSSH keypair and forward generated public key and username, password and second factor token that are entered by user to the proxy.
2. Proxy forwards request to the auth server.
3. If auth server accepts credentials, it generates a new certificate signed by its user CA and sends it back to the proxy.
4. Proxy 

#### Connecting to the nodes

Once the client has obtained a short lived certificate, it can use it to authenticate with any node in the cluster. Users can use the certificate using standard OpenSSH client (and get it using ssh-agent socket served by `tsh agent`) or using `tsh` directly:

![Teleport Proxy Web](img/proxy-ssh-2.svg)

1. SSH client connects to proxy and executes `proxy` subsystem of the proxy's SSH server, providing target node's host and port location.
2. Proxy dials to the target TCP address and starts forwarding the traffic to the client.
3. SSH client uses established SSH tunnel to open a new SSH connection and authenticate with the target node using its client certificate.

!!! tip "NOTE": 
    Teleport's proxy command makes it compatible with [SSH jump hosts](https://wiki.gentoo.org/wiki/SSH_jump_host) implemented using OpenSSH's `ProxyCommand`


## Certificates

Teleport uses standard Open SSH certificates for client and host authentication.

### Node Certificates

Nodes, proxies and auth servers use certificates signed by the cluster's auth server
to authenticate when joining the cluster. Teleport does not allow SSH sessions into nodes 
that are not cluster members.

A node certificate contains the node's role (like `proxy`, `auth` or `node`) as
a certificate extension (opaque signed string). All nodes in the cluster can
connect to auth server's HTTP API via SSH tunnel that checks each connecting
client's certificate and role to enforce access control (e.g. client connection
using node's certificate won't be able to add and delete users, and can only
get auth servers registered in the cluster).

### User Certificates

When an auth server generates a user certificate, it uses the information provided
by the cluster administrator about the role assigned to this user.

A user role can restrict user logins to specific OS logins or to a subset of 
cluster nodes (or any other restrictions enforced by the role). Teleport's user name is stored in a certificate's key id field. User's certificates do not use any cert extensions as a workaround to the 
[bug](https://bugzilla.mindrot.org/show_bug.cgi?id=2387) that treats any extension 
as a critical one, breaking access to the cluster.
    
## Teleport CLI Tools

Teleport offers two command line tools. `tsh` is a client tool used by the end users, while
`tctl` is used for cluster administration.

### TSH

`tsh` is similar in nature to OpenSSH `ssh` or `scp`. In fact, it has subcommands named after
them so you can call:

```
$ tsh --proxy=p ssh -p 1522 user@host
$ tsh --proxy=p scp -P example.txt user@host/destination/dir
```

Unlike `ssh`, `tsh` is very opinionated about authentication: it always uses auto-expiring
keys and it always connects to Teleport nodes via a proxy. It is a mandatory parameter.

When `tsh` logs in, the auto-expiring key is stored in `~/.tsh` and is valid for 23 hours by
default, unless you specify another interval via `--ttl` flag (max of 30 hours and minimum of 1 minute and capped by the server-side configuration).

You can learn more about `tsh` in the [User Manual](user-manual.md).

### TCTL

`tctl` is used to administer a Teleport cluster. It connects to the `auth server` listening
on `127.0.0.1` and allows cluster administrator to manage nodes and users in the cluster.

`tctl` is also a tool which can be used to modify the dynamic configuration of the 
cluster, like creating new user roles or connecting trusted clusters.

You can learn more about `tctl` in the [Admin Manual](admin-guide.md).
