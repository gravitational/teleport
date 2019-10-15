## Overview

TODO: Need some new custom diagrams for this page.

[TOC]

## The Node Service

A regularnode becomes a Teleport Node when the node joins a cluster with an "join" token. Read about how nodes are issued certificates in the [Auth Guide](./auth/#issuing-node-certificates).

A Teleport Node runs the [`teleport`](../cli-docs/#teleport) daemon with the `node` role. This process handles incoming connection requests, authentication, and remote command execution on the node, similar to the function of OpenSSH's `sshd`.

All cluster Nodes keep the Auth Server updated with their status with  periodic ping messages. They report their IP addresses and values of their assigned labels. Nodes can access the list of all Nodes in their cluster via the [Auth Server API](./auth/#auth-api).

!!! tip "Tip"
    In lightweight environments, such as IoT deployments, it is possible to entirely replace `sshd` with the `node` service <!--other examples?-->.

The `node` service provides SSH access to every node with all of the following clients:

* [OpenSSH: `ssh`](../guides/openssh)
* [Teleport CLI client: `tsh ssh`](../cli-docs/#tsh-ssh)
* [Teleport Proxy UI](./proxy/#web-to-ssh-proxy) accessed via a web browser.

Each client is authenticated via the [Auth Service](./auth/#authentication-in-teleport) before being granted access to a Node.

## Node Identity on a Cluster

Node Identity is defined on the Cluster level by the certificate a node possesses.

This certificate contains information about the node including:

* The **host ID**, a generated UUID unique to a node
* A **nodename**, which defaults to `hostname` of the node, but can be [configured](../configuration)
* The **cluster_name**, which defaults to the `hostname` of the auth server, but can be [configured](../configuration)
* The node **role** (i.e. `node,proxy`) encoded as a certificate extension
* The cert **TTL** (time-to-live)

A Teleport Cluster is a set of one or more machines whose public keys are signed by the same certificate authority (CA) operating in the Auth Server. A certificate is issued to a node when it joins the cluster for the first time. Learn more about this process in the [Auth Guide](./auth/#authentication-in-teleport).

!!! warning "Single-Node Clusters are Clusters"
    Once a Node gets a signed certificate from the Node CA, the Node is considered a member of the cluster, even if that cluster has only one node.

## Connecting to Nodes

When a client requests access to a Node, authentication is always performed through a cluster proxy. When the proxy server receives a connection request from a client it validates the client's credentials with the Auth Service. Once the client is authenticated the proxy attempts to connect the client to the requested Node.

There is a detailed walkthrough of the steps needed to initiate a connection to a node in the [Architecture Guide](./architecture).

<!--Network connection diagram-->

[Session state](./auth/#auth-state) is stored on the Auth Server rather than on the Node. Each node is completely stateless and holds no secrets such as keys or passwords.

## Cluster State

Cluster state is stored in a central storage location configured by the Auth Server. This means that the Node Service is completely stateless.

The cluster state information stored includes:

* Node membership information and online/offline status for each node.
* List of active sessions.
* List of locally stored users
* RBAC configuration (roles and permissions).
* Dynamic configuration.

Read more about what is stored in the [Auth Guide](./auth/#auth-state)

## Trusted Clusters

Teleport Auth Service can allow 3rd party users or nodes to connect to cluster nodes if their public keys are signed by a trusted CA. A "trusted cluster" is a pair of public keys of the trusted CA. It can be configured via `teleport.yaml` file.

<!--Link to more docs on this-->

## More Concepts

* [Basics](./basics)
* [Teleport Users](./users)
* [Teleport Auth](./auth)
* [Teleport Proxy](./proxy)
* [Architecture](./architecture)
