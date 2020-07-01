## Architecture Introduction

This guide is for those looking for a deeper understanding of Teleport. If you
are looking for hands-on instructions on how to set up Teleport for your team,
check out the [Admin Guide](../admin-guide.md)

## Design Principles

Teleport was designed in accordance with the following principles:

* **Off the Shelf Security**: Teleport does not re-implement any security
  primitives and uses well-established, popular implementations of the
  encryption and network protocols.

* **Open Standards**: There is no security through obscurity. Teleport is fully
  compatible with existing and open standards and other software, including
  [OpenSSH](../admin-guide.md#using-teleport-with-openssh).

* **Cluster-Oriented Design**: Teleport is built for managing clusters, not
  individual servers. In practice this means that hosts and [Users](../user-manual.md)
  have cluster memberships. Identity management and authorization happen on a
  cluster level.

* **Built for Teams**: Teleport was created under the assumption of multiple
  teams operating on several disconnected clusters. Example use cases might be
  production-vs-staging environment, or a cluster-per-customer or
  cluster-per-application basis.

This doc introduces the basic concepts of Teleport so you can get started
managing access!

## Definitions

Here are definitions of the key concepts you will use in Teleport.

|Concept                  | Description
|------------------|------------
| Node             | A node is a "server", "host" or "computer". Users can create shell sessions to access nodes remotely.
| User             | A user represents someone (a person) or something (a machine) who can perform a set of operations on a node.
| Cluster          | A cluster is a group of nodes that work together and can be considered a single system. Cluster nodes can create connections to each other, often over a private network. Cluster nodes often require TLS authentication to ensure that communication between nodes remains secure and comes from a trusted source.
| Certificate Authority (CA) | A Certificate Authority issues SSL certificates in the form of public/private keypairs.
| [Teleport Node](teleport_nodes.md)    | A Teleport Node is a regular node that is running the Teleport Node service. Teleport Nodes can be accessed by authorized Teleport Users. A Teleport Node is always considered a member of a Teleport Cluster, even if it's a single-node cluster.
| [Teleport User](teleport_users.md)    | A Teleport User represents someone who needs access to a Teleport Cluster. Users have stored usernames and passwords, and are mapped to OS users on each node. User data is stored locally or in an external store.
| Teleport Cluster | A Teleport Cluster is comprised of one or more nodes, each of which hold public keys signed by the same [Auth Server CA](teleport_auth.md). The CA cryptographically signs the public key of a node, establishing cluster membership.
| [Teleport CA](teleport_auth.md) | Teleport operates two internal CAs as a function of the Auth service. One is used to sign User public keys and the other signs Node public keys. Each certificate is used to prove identity, cluster membership and manage access.

## Teleport Services

Teleport uses three services which work together: [Nodes](teleport_nodes.md),
[Auth](teleport_auth.md), and [Proxy](teleport_proxy.md).

[**Teleport Nodes**](teleport_nodes.md) are servers which can be accessed remotely with
SSH. The Teleport Node service runs on a machine and is similar to the `sshd`
daemon you may be familiar with. Users can log in to a Teleport Node with all
of the following clients:

* [OpenSSH: `ssh` ](../admin-guide.md#using-teleport-with-openssh) (works on Linux, MacOS and Windows)
* [Teleport CLI client: `tsh ssh` ](../cli-docs.md#tsh-ssh) (works on Linux and MacOS)
* [Teleport Proxy UI](teleport_proxy.md#web-to-ssh-proxy) accessed via any modern web browser (including Safari on iOS and Chrome on Android)

[**Teleport Auth**](teleport_auth.md) authenticates Users and Nodes, authorizes User
access to Nodes, and acts as a CA by signing certificates issued to Users and
Nodes.

[**Teleport Proxy**](teleport_proxy.md) forwards User credentials to the [Auth
Service](teleport_auth.md), creates connections to a requested Node after successful
authentication, and serves a [Web UI](teleport_proxy.md#web-to-ssh-proxy).

## Basic Architecture Overview

The numbers correspond to the steps needed to connect a client to a node. These
steps are explained below the diagram.

!!! warning "Caution"
    The teleport daemon calls services "roles" in the CLI
    client. The `--roles` flag has no relationship to concept of User Roles or
    permissions.

![Teleport Overview](../img/overview.svg)

1. Initiate Client Connection
2. Authenticate Client
3. Connect to Node
4. Authorize Client Access to Node

!!! tip "Tip"


    In the diagram above we show each Teleport service separately for
    clarity, but Teleport services do not have to run on separate nodes.
    Teleport can be run as a binary on a single-node cluster with no external
    storage backend. We demonstrate this minimal setup in the [Quickstart
    Guide](../quickstart).

## Detailed Architecture Overview

Here is a detailed diagram of a Teleport Cluster.

The numbers correspond to the steps needed to connect a client to a node. These
steps are explained in detail below the diagram.

![Teleport Everything](../img/everything.svg)

!!! note "Caution"


    The Teleport Admin tool, `tctl` , must be physically present
    on the same machine where Teleport Auth is running. Adding new nodes or
    inviting new users to the cluster is only possible using this tool.

### 1: Initiate Client Connection

![Client offers certificate](../img/client_initiate.svg)

The client tries to establish an SSH connection to a proxy using the CLI
interface or a web browser. When establishing a connection, the client offers
its public key. Clients must always connect through a proxy for two reasons:

1. Individual nodes may not always be reachable from outside a secure network.
2. Proxies always record SSH sessions and keep track of active user sessions.

   This makes it possible for an SSH user to see if someone else is connected to
   a node she is about to work on.

### 2: Authenticate Client Certificate

![Client offers valid certificate](../img/cert_ok.svg)

The proxy checks if the submitted certificate has been previously signed by the
auth server.

![Client obtains new certificate](../img/cert_invalid.svg)

If there was no key previously offered (first time login) or if the certificate
has expired, the proxy denies the connection and asks the client to login
interactively using a password and a 2nd factor if enabled.

Teleport supports
[Google Authenticator](https://support.google.com/accounts/answer/1066447?hl=en),
[Authy](https://www.authy.com/), or another
[TOTP](https://en.wikipedia.org/wiki/Time-based_One-time_Password_algorithm)
generator. The password + 2nd factor are submitted to a proxy via HTTPS,
therefore it is critical for a secure configuration of Teleport to install a
proper HTTPS certificate on a proxy.

!!! warning "Warning"

    Do not use self-signed SSL/HTTPS certificates in production!

If the credentials are correct, the auth server generates and signs a new
certificate and returns it to the client via the proxy. The client stores this key
and will use it for subsequent logins. The key will automatically expire after
12 hours by default. This [TTL](https://en.wikipedia.org/wiki/Time_to_live) can be [configured](../cli-docs.md#tctl-users-add)
to another value by the cluster administrator.

### 3: Lookup Node

![Node lookup](../img/node_lookup.svg)

At this step, the proxy tries to locate the requested node in a cluster. There
are three lookup mechanisms a proxy uses to find the node's IP address:

1. Uses DNS to resolve the name requested by the client.
2. Asks the Auth Server if there is a Node registered with this `nodename` .
3. Asks the Auth Server to find a node (or nodes) with a label that matches the
   requested name.

If the node is located, the proxy establishes the connection between the client
and the requested node. The destination node then begins recording the session,
sending the session history to the auth server to be stored.

!!! note "Note"


    Teleport may also be configured to have the session recording
    occur on the proxy, see [Audit Log](../admin-guide.md#audit-log) for more
    information.

### 4: Authenticate Node Certificate

![Node Membership Authentication](../img/node_cluster_auth.svg)

When the node receives a connection request, it checks with the Auth Server to
validate the node's public key certificate and validate the Node's cluster
membership.

If the node certificate is valid, the node is allowed to access the Auth Server
API which provides access to information about nodes and users in the cluster.

### 5: Grant User Node Access

![User Granted Node Access](../img/user_node_access.svg)

The node requests the Auth Server to provide a list of [OS users (user
mappings)](../admin-guide.md#adding-and-deleting-users) for the connecting client, to make sure the client is
authorized to use the requested OS login.

Finally, the client is authorized to create an SSH connection to a node.

![Proxy Connection Established](../img/proxy_client_connect.svg)

## Teleport CLI Tools

Teleport offers two command line tools. `tsh` is a client tool used by the end
users, while `tctl` is used for cluster administration.

### TSH

`tsh` is similar in nature to OpenSSH `ssh` or `scp`. In fact, it has
subcommands named after them so you can call:

```bsh
$ tsh --proxy=p ssh -p 1522 user@host
$ tsh --proxy=p scp -P example.txt user@host/destination/dir
```

Unlike `ssh`, `tsh` is very opinionated about authentication: it always uses
auto-expiring keys and it always connects to Teleport nodes via a proxy.

When `tsh` logs in, the auto-expiring key is stored in `~/.tsh` and is valid for
12 hours by default, unless you specify another interval via the `--ttl` flag
(capped by the server-side configuration).

You can learn more about `tsh` in the [User Manual](../user-manual.md).

### TCTL

`tctl` is used to administer a Teleport cluster. It connects to the Auth
server listening on `127.0.0.1` and allows a cluster administrator to manage
nodes and users in the cluster.

`tctl` is also a tool which can be used to modify the dynamic configuration of
the cluster, like creating new user roles or connecting trusted clusters.

You can learn more about `tctl` in the [Admin Manual](../admin-guide.md).


## Next Steps

* If you haven't already, read the [Quickstart Guide](../quickstart.md) to run a
minimal setup of Teleport yourself.
* Set up Teleport for your team with the [Admin Guide](../admin-guide.md).

Read the rest of the Architecture Guides:

* [Teleport Users](teleport_users.md)
* [Teleport Nodes](teleport_nodes.md)
* [Teleport Auth](teleport_auth.md)
* [Teleport Proxy](teleport_proxy.md)
