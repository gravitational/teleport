# Teleport Nodes

## Prerequisites

* Read [Teleport Basics](./basics.md) first to get a brief overview of Teleport

## Overview

Teleport Nodes are servers which can be accessed remotely via Teleport Auth. A regular node becomes a Teleport Node when you start the `teleport` daemon with `--roles=node`. The `node` service provides SSH access to every node either with [OpenSSH](../guides/openssh) or with the [`tsh` client](../cli-docs).

## Cluster Membership

Teleport Nodes are always members of a [cluster](./clusters), even if that cluster is only one node. Teleport can be run as a single



## More Concepts

* Cluster