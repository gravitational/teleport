## Overview

TODO: This doc is in-progress not at reviewable stage
TODO: Create Diagrams

[TOC]

## Nodes

Teleport Nodes are servers which can be accessed remotely via Teleport Auth. A regular node becomes a Teleport Node when you start the `teleport` daemon with `--roles=node`. The `node` service provides SSH access to every node either with [OpenSSH](../guides/openssh) or with the [`tsh` client](../cli-docs).

In some setups it might make sense to replace `sshd` entirely. <!--TODO: Expand upon this use case-->

Teleport Nodes are always members of a cluster, even if that cluster is only one node. Teleport can be run as a single binary <!--TODO: Expand this use case -->. Teleport does not allow SSH sessions into nodes that are not cluster members.

Each node is completely stateless and holds no secrets such as keys, passwords, etc.
The persistent state of a Teleport cluster is kept by the [auth server](./auth/#auth-state).

## Cluster

Unlike the traditional SSH service, Teleport operates on a Cluster of nodes. A Teleport cluster is a set of machines whose public keys are signed by the same certificate authority (CA), with the auth server acting as the CA of a cluster.

A cluster is a set of nodes (servers). There are several implication of this:

* User identities and user permissions are defined and enforced on a cluster level.
* A node must become a Cluster Member before any user can connect to it via SSH.
* SSH access to any cluster node is _always_ performed through a cluster proxy.

## Cluster State

The auth server stores its own keys in a cluster state
  storage. All of cluster dynamic configuration is stored there as well, including:
    * Node membership information and online/offline status for each node.
    * List of active sessions.
    * List of locally stored users.
    * [RBAC](ssh_rbac) configuration (roles and permissions).
    * Other dynamic configuration.

<!--| Cluster Name     | Every Teleport cluster must have a name. If a name is not supplied via `teleport.yaml` configuration file, a GUID will be generated. **IMPORTANT:** renaming a cluster invalidates its keys and all certificates it had created.
| Trusted Cluster | Teleport Auth Service can allow 3rd party users or nodes to connect if their public keys are signed by a trusted CA. A "trusted cluster" is a pair of public keys of the trusted CA. It can be configured via `teleport.yaml` file.-->

## More Concepts

* [Basics](./basics)
* [Users](./users)
* [Auth](./auth)
* [Proxy](./proxy)
* [Architecture](./architecture)