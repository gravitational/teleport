# Teleport Architecture

This document covers the underlying design principles of Teleport and a
detailed description of Teleport architecture.

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

* **Cluster of Nodes**. Unlike the traditional SSH service, Teleport operates on a _cluster_ of nodes.
   A cluster is a set of nodes (servers). There are several ramifications of this:
	* User identities and user permissions are defined and enforced on a cluster level.
	* A node must become a _cluster member_ before any user can connect to it via SSH.
	* SSH access to any cluster node is _always_ performed through a cluster proxy,
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

* **SSH Certificates**. Teleport uses SSH certificates to authenticate nodes and users within
   a cluster. Teleport does not allow public key or password-based SSH authentication.

* **Dynamic Configuration**. Nearly everything in Teleport can be configured via the
   configuration file, `/etc/teleport.yaml` by default. But some settings can be changed
   at runtime, by modifying the cluster state (eg., creating user roles or
   connecting to trusted clusters). These operations are called 'dynamic configuration'.

## User Accounts

Teleport supports two types of user accounts:

* **Local users** are created and stored in Teleport's own identity storage. A cluster
  administrator has to create account entries for every Teleport user.
  Teleport supports second factor authentication (2FA) and it is enforced by default.
  There are two types of 2FA supported:
    * [TOTP](https://en.wikipedia.org/wiki/Time-based_One-time_Password_Algorithm)
      is the default. You can use [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator) or
      [Authy](https://www.authy.com/) or any other TOTP client.
    * [U2F](https://en.wikipedia.org/wiki/Universal_2nd_Factor).
* **External users** are users stored elsewhere within an organization. Examples include
  Github, Active Directory (AD) or any identity store with an OpenID/OAuth2 or SAML endpoint.


!!! tip "Version Warning":
    SAML, OIDC and Active Directory are only supported in Teleport Enterprise. Please
    take a look at the [Teleport Enterprise](/enterprise.md) chapter for more information.

It is possible to have multiple identity sources configured for a Teleport cluster. In this
case, an identity source (called a "connector") will have to be passed to `tsh --auth=connector_name login`.
Local (aka, internal) users connector can be specified via `tsh --auth=local login`.

## Teleport Cluster

Let's explore how these services come together and interact with Teleport clients and with each other.

**High Level Diagram of a Teleport cluster**

![Teleport Overview](/img/overview.svg)

Notice that the Teleport Admin tool, `tctl`, must be physically present on the same machine where
Teleport auth is running. Adding new nodes or inviting new users to the cluster is only
possible using this tool.

Once nodes and users (clients) have been invited to the cluster, here is the sequence
of network calls performed by Teleport components when the client tries to connect to the
node:

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
	Do not use a self-signed SSL/HTTPS certificates when creating production!

If the credentials are correct, the auth server generates and signs a new certificate and returns
it to a client via the proxy. The client stores this key and will use it for subsequent
logins. The key will automatically expire after 12 hours by default. This TTL can be configured to another value by the cluster administrator.

3) At this step, the proxy tries to locate the requested node in a cluster. There are three
   lookup mechanisms a proxy uses to find the node's IP address:

* Tries to resolve the name requested by the client.
* Asks the auth server if there is a node registered with this `nodename`.
* Asks the auth server to find a node (or nodes) with a label that matches the requested name.

If the node is located, the proxy establishes the connection between the client and the
requested node. The destination node then begins recording the session, sending the session history to the auth server to be stored.

!!! note "Note":
    Teleport may also be configured to have the session recording occur on the proxy, see [Session Recording](architecture/#session-recording) for more information.

4) When the node receives a connection request, it also checks with the auth server to validate
   the submitted client certificate. The node requests the auth server to provide a list
   of OS users (user mappings) for the connecting client, to make sure the client is authorized
   to use the requested OS login.

   In other words, every connection is authenticated twice before being authorized to log in:

* User's cluster membership is validated when connecting a proxy.
* User's cluster membership is validated again when connecting to a node.
* User's node-level permissions are validated before authorizing her to interact with SSH
      subsystems.

**Detailed Diagram of a Teleport cluster**

![Teleport Everything](/img/everything.svg)


## Cluster State

Each cluster node is completely stateless and holds no secrets such as keys, passwords, etc.
The persistent state of a Teleport cluster is kept by the auth server. There are three types
of data stored by the auth server:

* **Cluster State**. A Teleport cluster is a set of machines whose public keys
  are signed by the same certificate authority (CA), with the auth server
  acting as the CA of a cluster. The auth server stores its own keys in a cluster state
  storage. All of cluster dynamic configuration is stored there as well, including:
    * Node membership information and online/offline status for each node.
    * List of active sessions.
    * List of locally stored users.
    * [RBAC](ssh_rbac) configuration (roles and permissions).
    * Other dynamic configuration.

* **Audit Log**. When users log into a Teleport cluster, execute remote commands and logout,
  that activity is recorded in the audit log. See [Audit Log](/admin-guide.md#audit-log)
  for more details.

* **Recorded Sessions**. When Teleport users launch remote shells via `tsh ssh` command, their
  interactive sessions are recorded and stored by the auth server. Each recorded
  session is a file which is saved in `/var/lib/teleport` by default, but can also be
  saved in external storage, like an AWS S3 bucket.

### Storage Back-Ends

Different types of cluster data can be configured with different storage back-ends as shown
in the table below:

Data Type        | Supported Back-ends       | Notes
-----------------|---------------------------|---------
Cluster state    | `dir`, `etcd`, `dynamodb` | Multi-server (HA) configuration is only supported using `etcd` and `dynamodb` back-ends.
Audit Log Events | `dir`, `dynamodb`         | If `dynamodb` is used for the audit log events, `s3` back-end **must** be used for the recorded sessions.
Recorded Sessions| `dir`, `s3`               | `s3` is mandatory if `dynamodb` is used for the audit log.

!!! tip "Note":
    The reason Teleport designers split the audit log events and the recorded
    sessions into different back-ends is because of the nature of the data. A
    recorded session is a compressed binary stream (blob) while the event is a
    well-defined JSON structure. `dir` works well enough for both in small
    deployments, but large clusters require specialized data stores: S3 is
    perfect for uploading session blobs, while DynamoDB or `etcd` are better
    suited to store the cluster state.

The combination of DynamoDB + S3 is especially popular among AWS users because it allows them to
run Teleport clusters completely devoid of local state.

## Teleport Services

There are three types of services (roles) in a Teleport cluster.

| Service (Node Role)  | Description
|----------------|------------------------------------------------------------------------
| node   | This role provides the SSH access to a node. Typically every machine in a cluster runs this role. It is stateless and lightweight.
| proxy  | The proxy accepts inbound connections from the clients and routes them to the appropriate nodes. The proxy also serves the Web UI.
| auth   | This service provides authentication and authorization service to proxies and nodes. It is the certificate authority (CA) of a cluster and the storage for audit logs. It is the only stateful component of a Teleport cluster.

Although `teleport` daemon is a single binary, it can provide any combination of these services
via `--roles` command line flag or via the configuration file.

In addition to `teleport` daemon, there are three client tools:

| Tool           | Description
|----------------|------------------------------------------------------------------------
| tctl    | Cluster administration tool used to invite nodes to a cluster and manage user accounts. `tctl` must be used on the same machine where `auth` is running.
| tsh     | Teleport client tool, similar in principle to OpenSSH's `ssh`. It is used to log into remote SSH nodes, list and search for nodes in a cluster, securely upload/download files, etc. `tsh` can work in conjunction with `ssh` by acting as an SSH agent.
| Web browser | You can use your web browser to log into any Teleport node. Just open `https://<proxy-host>:3080` (`proxy-host` is one of the machines that has proxy service enabled).

Let's explore each of the Teleport services in detail.

### The Auth Service

The auth server acts as a certificate authority (CA) for the cluster. Teleport security is
based on SSH certificates and every certificate must be signed by the cluster auth server.

There are two types of [certificates](#ssh-certificates) the auth server can sign:

* **Host certificates** are used to add new nodes to a cluster.
* **User certificates** are used to authenticate users when they try to log into a cluster node.

Upon initialization, the auth server generates a public / private keypair and stores it in the
configurable key storage. The auth server also keeps the records of what has been happening
inside the cluster, including the audit log and session recordings.

![Teleport Auth](/img/auth-server.svg?style=grv-image-center-lg)

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

Clients can also connect to the auth API through the Teleport proxy to use a limited subset of the API to
discover the member nodes of the cluster.

Cluster administration is performed using `tctl` command line tool.

!!! tip "NOTE":
    For high availability in production, a Teleport cluster can be serviced by multiple auth servers
    running in sync. Check [HA configuration](/admin-guide.md#high-availability) in the
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

All user interactions with the Teleport cluster are done though a proxy service. It is
recommended to have several of them running.

When you launch the Teleport Proxy for the first time, it will generate a self-signed HTTPS
certificate to make it easier to explore Teleport.

!!! warning "Warning":
	It is absolutely crucial to properly configure TLS for HTTPS when you use Teleport Proxy in production.


### Web to SSH Proxy

In this mode, Teleport Proxy implements WSS (secure web sockets) to SSH proxy:

![Teleport Proxy Web](/img/proxy-web.svg)

1. User logs in using username, password and 2nd factor token to the proxy.
2. Proxy passes credentials to the auth server's API
3. If auth server accepts credentials, it generates a new web session and generates a special
   ssh keypair associated with this web session.
   Auth server starts serving [OpenSSH ssh-agent protocol](https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.agent)
   to the proxy.
4. From the SSH node's perspective, it's a regular SSH client connection that is authenticated using an
   OpenSSH certificate, so no special logic is needed.

!!! tip "NOTE":
    When using the web UI, Teleport Proxy terminates the traffic and re-encodes for SSH client connection.

### SSH Proxy

#### Getting signed short lived certificate

Teleport Proxy implements a special method to let clients get short lived certificates signed by auth's host certificate authority:

![Teleport Proxy SSH](/img/proxy-ssh-1.svg)

1. TSH client or TSH agent generate OpenSSH keypair and forward generated public key and username, password and second factor token that are entered by user to the proxy.
2. Proxy forwards request to the auth server.
3. If auth server accepts credentials, it generates a new certificate signed by its user CA and sends it back to the proxy.
4. Proxy returns the user certificate to the client and client stores it in `~/.tsh/keys`

#### Connecting to the nodes

Once the client has obtained a short lived certificate, it can use it to authenticate with any node in the cluster. Users can use the certificate using standard OpenSSH client (and get it using ssh-agent socket served by `tsh agent`) or using `tsh` directly:

![Teleport Proxy Web](/img/proxy-ssh-2.svg)

1. SSH client connects to proxy and executes `proxy` subsystem of the proxy's SSH server, providing target node's host and port location.
2. Proxy dials to the target TCP address and starts forwarding the traffic to the client.
3. SSH client uses established SSH tunnel to open a new SSH connection and authenticate with the target node using its client certificate.

!!! tip "NOTE":
    Teleport's proxy command makes it compatible with [SSH jump hosts](https://wiki.gentoo.org/wiki/SSH_jump_host) implemented using OpenSSH's `ProxyCommand`


## SSH Certificates

Teleport uses standard SSH certificates for client and host authentication.

One can think of an SSH certificate as a "permit" issued and time-stamped by a
certificate authority. A certificate contains four important pieces of data:

* List of principals (identities) this certificate belongs to.
* Signature of the certificate authority who issued it, i.e. the _auth server_.
* The expiration date, also known as "time to live" or simply TTL.
* Additional data, often stored as a certificate extension.

One can think of a Teleport _auth server_ as a certificate authority (CA) of a
cluster which issues certificates. The reality is a bit more complicated
because cluster hosts need their own certificates too. Therefore, there are two
CAs inside the _auth server_ per cluster:

* **Host CA** issues host certificates.
* **User CA** issues user certificates.

### Host Certificates

All hosts (i.e. nodes, proxies and auth servers) always use host certificates
signed by the auth server's host CA to authenticate to join the cluster.
Teleport does not allow SSH sessions into nodes that are not cluster members.

A host certificate contains the node's role (like `proxy`, `auth` or `node`) as
a certificate extension (opaque signed string). All hosts in a cluster can
connect to auth server's HTTP API via SSH tunnel that checks each connecting
client's certificate and role to enforce access control (e.g. client connection
using node's certificate won't be able to add and delete users and can only
get auth servers registered in the cluster).

### User Certificates

The _auth server_ uses its _user CA_ to issue user certificates. In addition to
user's identity, user certificates also contain user roles and SSH options,
like "permit-agent-forwarding".

This additional data is stored as certificate extensions and is protected by the CA
signature.

### Certificate Rotation

By default, all user certificates have an expiration date, also known as time to
live (TTL). This TTL can be configured by a Teleport administrator. But the
host certificates issued by an _auth server_ are valid indefinitely by default.

Teleport supports certificate rotation, i.e. the process of invalidating _all_
previously issued certificates regardless of their TTL. Certificate rotation is
triggered by `tctl auth rotate` command. When this command is invoked by a Teleport
administrator on one of cluster's _auth servers_, the following happens:

* A new certificate authority (CA) key is generated.
* The old CA will be considered valid _alongside_ the new CA for some period of
  time. This period of time is called a _grace period_.
* During the grace period, all previously issued certificates will be considered
  valid, assuming their TTL isn't expired.
* After the grace period is over, the certificates issued by the old CA are no
  longer accepted.

This process is repeated twice, for both the host CA and the user CA. Take a
look at the [admin guide](/admin-guide.md#certificate-rotation) to learn how to
use certificate rotation in practice.

## Audit Log

The Teleport auth server keeps the audit log of SSH-related events that take
place on any node with a Teleport cluster. It is important to understand that
the SSH nodes emit audit events and submit them to the auth server.

!!! warning "Compatibility Warning":
    Because all SSH events like `exec` or `session_start` are reported by the
    Teleport node service, they will not be logged if you are using OpenSSH
    `sshd` daemon on your nodes.

Only an SSH server can report what's happening to the Teleport auth server.
The audit log is a JSON file which is by default stored on the auth server's
filesystem under `/var/lib/teleport/log`. The format of the file is documented
in the [Admin Manual](/admin-guide/#audit-log).

Teleport users are encouraged to export the events into external, long term
storage.

!!! info "Deployment Considerations":
    If multiple Teleport auth servers are used to service the same cluster (HA mode)
    a network file system must be used for `/var/lib/teleport/log` to allow them
    to combine all audit events into the same audit log.

### Session Recording

By default, destination nodes submit SSH session traffic to the auth server
for storage. These recorded sessions can be replayed later via `tsh play`
command or in a web browser.

Some Teleport users mistakenly believe that audit and session recording happen by default
on the Teleport proxy server. This is not the case because a proxy cannot see
the encrypted traffic, it is encrypted end-to-end, i.e. from an SSH client to
an SSH server/node, see the diagram below:

![session-recording-diagram](/img/session-recording.svg?style=grv-image-center-lg)

However, starting from Teleport 2.4, it is possible to configure the
Teleport proxy to enable "recording proxy mode". In this mode, the proxy
terminates (decrypts) the SSH connection using the certificate supplied by the
client via SSH agent forwarding and then establishes it's own SSH connection
to the final destination server, effectively becoming an authorized "man in the
middle". This allows the proxy server to forward SSH session data to the auth
server to be recorded, as shown below:

![recording-proxy](/img/recording-proxy.svg?style=grv-image-center-lg)

The recording proxy mode, although _less secure_, was added to allow Teleport
users to enable session recording for OpenSSH's servers running `sshd`, which is
helpful when gradually transitioning large server fleets to Teleport.

We consider the "recording proxy mode" to be less secure for two reasons:

1. It grants additional privileges to the Teleport proxy. In the default mode, the
   proxy stores no secrets and cannot "see" the decrypted data. This makes a proxy
   less critical to the security of the overall cluster. But if an attacker gains
   physical access to a proxy node running in the "recording" mode, they will be
   able to see the decrypted traffic and client keys stored in proxy's process memory.
2. Recording proxy mode requires the SSH agent forwarding. Agent forwarding is required
   because without it, a proxy will not be able to establish the 2nd connection to the
   destination node.

However, there are advantages of proxy-based session recording too. When sessions are recorded
at the nodes, a root user can add iptables rules to prevent sessions logs from reaching
the Auth Server. With sessions recorded at the proxy, users with root privileges on nodes
have no way of disabling the audit.

See the [admin guide](/admin-guide#recorded-sessions) to learn how to turn on the
recording proxy mode.

## Teleport CLI Tools

Teleport offers two command line tools. `tsh` is a client tool used by the end users, while
`tctl` is used for cluster administration.

### TSH

`tsh` is similar in nature to OpenSSH `ssh` or `scp`. In fact, it has subcommands named after
them so you can call:

```bsh
$ tsh --proxy=p ssh -p 1522 user@host
$ tsh --proxy=p scp -P example.txt user@host/destination/dir
```

Unlike `ssh`, `tsh` is very opinionated about authentication: it always uses auto-expiring
keys and it always connects to Teleport nodes via a proxy.

When `tsh` logs in, the auto-expiring key is stored in `~/.tsh` and is valid for 12 hours by
default, unless you specify another interval via `--ttl` flag (capped by the server-side configuration).

You can learn more about `tsh` in the [User Manual](/user-manual.md).

### TCTL

`tctl` is used to administer a Teleport cluster. It connects to the `auth server` listening
on `127.0.0.1` and allows a cluster administrator to manage nodes and users in the cluster.

`tctl` is also a tool which can be used to modify the dynamic configuration of the
cluster, like creating new user roles or connecting trusted clusters.

You can learn more about `tctl` in the [Admin Manual](/admin-guide.md).
