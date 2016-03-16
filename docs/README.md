# Overview

### Introduction

Gravitational Teleport is a tool for remotely accessing isolated clusters of 
Linux servers via SSH or HTTPS. Unlike traditional key-based access, Teleport 
enables teams to easily adopt the following practiecs:

- Avoid key distribution problem by using auto-expiring keys signed by cluster certificate authority (CA).
- Integrate SSH into the existing identity management in your organization.
- Enforce 2nd factor authentication.
- Connect to clusters located behind firewalls that block all inbound connections.
- Record and replay SSH sessions for audit purposes.
- Collaboratively troubleshoot issues via built-in session sharing.
- Discover cluster nodes and Docker containers via logical names or dynamic labels.
- Connect to servers without direct Interenet connection via SSH bastions.
- Avoid [trust on first use](https://en.wikipedia.org/wiki/Trust_on_first_use) problem.

Take a look at [Quick Start]() page to get a taste of using Teleport, or read the 
[Design Document]() to get a full understanding of how Teleport works.


### Why?

Mature tech companies with significant infrastructure footprints tend to implement most
of these patterns internally. Gravitational Teleport allows smaller companies without 
significant in-house SSH epxertise to easily do the same. It comes with a beautiful
Web UI and a very permissive [Apache 2.0](https://github.com/gravitational/teleport/blob/master/LICENSE)
license.

Teleport is built on top of the high-quality [Golang SSH](https://godoc.org/golang.org/x/crypto/ssh) 
implementation and it is fully compatible with OpenSSH.

### Who Built Teleport?

Teleport is built by Gravitational Inc](https://gravitational.com). We created Teleport by borrowing 
from our previous experiences at Rackspace. It has been extracted from the [Gravity](http://gravitational.com/vendors.html), 
our system for deploying and remotely managing SaaS applications running in many cloud regions or
even on-premise.

Being a wonderful standalone tool, Teleport can be used as a software library enabling 
trust management in a complex multi-cluster, multi-region scenarios across many teams 
within multiple organizations.

# Quick Start

### Installing

Gravitational Teleport natively runs on any modern Linux distribution and OSX. You can
download prebuilt binaries from [here](https://github.com/gravitational/teleport/releases)
or you can [build it from source](BROKEN).

### Quick Start

TBD

# Architecture

This document covers the underlying design principles of Teleport and offers the detailed 
description of Teleport architecture.

### Design Principles

Teleport was designed in accordance with the following design principles:

* **Off the shelf security**. Teleport does not re-implement any security primitieves
  and uses well-established, popular implementations of the encryption and network protocols.
* **Open standards**. There is no security through obscurity. Teleport is fully compatible
  with existing and open standards.

### Core Concepts

There are three types of services (roles) in a Teleport cluster. 

| Service(Role)  | Description
|----------------|------------------------------------------------------------------------
| node   | This role provides the SSH access to a node. Typically every machine in a cluster runs `teleport` with this role. It is stateless and lightweight.
| proxy  | The proxy accepts inbound connections from the clients and routes them to the appropriate nodes. The proxy also serves the Web UI.
| auth   | This service provides authentication and authorization service to proxies and nodes. It is the certificate authority (CA) of a cluster and the storage for audit logs. It is the only stateful component of a Teleport cluster.

Although `teleport` daemon is a single binary, it can provide any combination of these services 
via `--roles` command line flag or via the configuration file.

Lets explore how these services interact with Teleport clients and with each other. Consider the diagram:

[!img/teleport.png]


# Admin Guide

### Building

Gravitational Teleport is written in Go and requires Golang v1.5 or newer. If you have Go
already installed, building is easy:

```bash
> git clone https://github.com/gravitational/teleport && cd teleport
> make
```

If you do not have Go but you have Docker installed and running, you can build Teleport
this way:

```bash
> git clone https://github.com/gravitational/teleport
> make -C build.assets
```

### Installing

TBD

- Configuration
- Adding users to the cluster
- Adding nodes to the cluster
- Controlling access

FAQ
---

0. Can I use Teleport instead of OpenSSH in production today?

1. Can I use OpenSSH client's `ssh` command with Teleport?

2. Which TCP ports does Teleport uses?

3. Do you offer commercial support for Teleport?
