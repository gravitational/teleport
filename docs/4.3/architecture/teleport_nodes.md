## Teleport Nodes


## The Node Service

A regular node becomes a Teleport Node when the node joins a cluster with a
"join" token. Read about how nodes are issued certificates in the
[Auth Guide](teleport_auth.md#issuing-node-certificates).

![Node joins a cluster](../img/node_join.svg)

A Teleport Node runs the [`teleport`](../cli-docs.md#teleport) daemon with the
`node` role. This process handles incoming connection requests, authentication,
and remote command execution on the node, similar to the function of OpenSSH's
`sshd`.

![Node Service ping API](../img/node_service_api.svg)

All cluster Nodes keep the Auth Server updated on their status with periodic
ping messages. They report their IP addresses and values of their assigned
labels. Nodes can access the list of all Nodes in their cluster via the
[Auth Server API](teleport_auth.md#auth-api).

!!! tip "Tip"
    In most environments we advise replacing the OpenSSH daemon `sshd`
    with the Teleport Node Service unless there are existing workflows relying
    on `ssh` or in special cases such as embedded devices that can't run
    custom binaries.

The `node` service provides SSH access to every node with all of the following clients:

* [OpenSSH: `ssh`](../admin-guide.md#using-teleport-with-openssh)
* [Teleport CLI client: `tsh ssh`](../cli-docs.md#tsh-ssh)
* [Teleport Proxy UI](teleport_proxy.md#web-to-ssh-proxy) accessed via a web browser.

Each client is authenticated via the [Auth Service](teleport_auth.md#authentication-in-teleport) before being granted access to a Node.

## Node Identity on a Cluster

Node Identity is defined on the Cluster level by the certificate a node possesses.

![Node Identity](../img/node_identity.svg)

This certificate contains information about the node including:

* The **host ID**, a generated UUID unique to a node
* A **nodename**, which defaults to `hostname` of the node, but can be configured.
* The **cluster_name**, which defaults to the `hostname` of the auth server, but can be configured
* The node **role** (i.e. `node,proxy`) encoded as a certificate extension
* The cert **TTL** (time-to-live)

A Teleport Cluster is a set of one or more machines whose public keys are signed
by the same certificate authority (CA) operating in the Auth Server. A
certificate is issued to a node when it joins the cluster for the first time.
Learn more about this process in the [Auth
Guide](teleport_auth.md#authentication-in-teleport).

!!! warning "Single-Node Clusters are Clusters"

    Once a Node gets a signed certificate from the Node CA, the Node is considered a member of the cluster, even if that cluster has only one node.

## Connecting to Nodes

When a client requests access to a Node, authentication is always performed
through a cluster proxy. When the proxy server receives a connection request
from a client it validates the client's credentials with the Auth Service. Once
the client is authenticated the proxy attempts to connect the client to the
requested Node.

There is a detailed walk-through of the steps needed to initiate a connection to
a node in the [Architecture Overview](teleport_architecture_overview.md).

![Proxy Connection between client and node](../img/proxy_client_connect.svg)

## Cluster State

Cluster state is stored in a central storage location configured by the Auth
Server. This means that each node is completely stateless and holds no secrets
such as keys or passwords.

![Cluster State](../img/cluster_state.svg)

The cluster state information stored includes:

* Node membership information and online/offline status for each node.
* List of active sessions.
* List of locally stored users.
* RBAC configuration (roles and permissions).
* Dynamic configuration.

Read more about what is stored in the [Auth Guide](teleport_auth.md#auth-state)

## Session Recording

By default, nodes submit SSH session traffic to the Auth server
for storage. These recorded sessions can be replayed later via `tsh play`
command or in a web browser.

Some Teleport users mistakenly believe that audit and session recording happen
by default on the Teleport proxy server. This is not the case because a proxy
cannot see the encrypted traffic, it is encrypted end-to-end, i.e. from an SSH
client to an SSH server/node, see the diagram below:

![session-recording-diagram](../img/session-recording.svg)

However, starting from Teleport 2.4, it is possible to configure the
Teleport proxy to enable "recording proxy mode".

## Trusted Clusters

Teleport Auth Service can allow 3rd party users or nodes to connect to cluster
nodes if their public keys are signed by a trusted CA. A "trusted cluster" is a
pair of public keys of the trusted CA. It can be configured via `teleport.yaml`
file.

<!--TODO: incomplete, write more on this-->

## More Concepts

* [Architecture Overview](teleport_architecture_overview.md)
* [Teleport Users](teleport_users.md)
* [Teleport Auth](teleport_auth.md)
* [Teleport Proxy](teleport_proxy.md)
