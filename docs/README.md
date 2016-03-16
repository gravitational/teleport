# Overview

### Introduction

Gravitational Teleport is a tool for remotely accessing isolated clusters of 
Linux servers via SSH or HTTPS. Unlike traditional key-based access, Teleport 
enables teams to easily adopt the following practiecs:

- Avoid key distribution and [trust on first use](https://en.wikipedia.org/wiki/Trust_on_first_use) issues by using auto-expiring keys signed by a cluster certificate authority (CA).
- Enforce 2nd factor authentication.
- Connect to clusters located behind firewalls or without direct Internet access via SSH bastions.
- Record and replay SSH sessions for audit purposes.
- Collaboratively troubleshoot issues via built-in session sharing.
- Discover online servers and running Docker containers within a cluster with dynamic node labels.

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

Teleport is built by [Gravitational Inc](https://gravitational.com). We created Teleport by borrowing 
from our previous experiences at Rackspace. It has been extracted from the [Gravity](http://gravitational.com/vendors.html), 
our system for deploying and remotely managing SaaS applications running in many cloud regions or even on-premise.

Being a wonderful standalone tool, Teleport can be used as a software library enabling 
trust management in a complex multi-cluster, multi-region scenarios across many teams 
within multiple organizations.

# Quick Start

### Installing

Gravitational Teleport natively runs on any modern Linux distribution and OSX. You can
download prebuilt binaries from [here](https://github.com/gravitational/teleport/releases)
or you can [build it from source](BROKEN).

### Quick Start

To get a quick feel of Teleport lets start it on `localhost` and connect to via the command
line client and also via a browser. This quick start assumes you have root permissions.

Create a directory for Teleport to keep its data and start `teleport` daemon:

```bash
mkdir -p /var/lib/teleport
teleport start
```

At this point you should see Teleport print its services listening addresses into the console.
You are running a single-node Teleport cluster. Lets add a user to it:

```bash
tctl users add $USER
```

Teleport users are not the same as OS users on servers, but for convenience we've added
a Teleport user with the same name as your local login.

`tctl` will print a sign-up URL for you to visit and complete the user creation:

```
Signup token has been created. Share this URL with the user:
https://turing:3080/web/newuser/96c85ed60b47ad345525f03e1524ac95d78d94ffd2d0fb3c683ff9d6221747c2
```

You will have to open this link in your browser, install Google Authenticator on your phone,
set up 2nd factor authentication and pick a password.

Once you have done that, you will be presented with a Web UI where you will see your 
machine and will be able to log into it using web-based terminal.

Lets login using the command line too:

```bash
tsh --proxy=localhost localhost
```

You're in! Notice that `tsh` client always needs the `--proxy` flag because all client connections
in Teleport have to go via proxy, sometimes called an "SSH bastion".

Lets add another node to your cluster. Lets assume the other node can be reached by
hostname `luna`. 

`tctl` command below will create a single-use token for a node to join and will print instructions
for you to follow:

```bash
> tctl nodes add

The invite token: n92bb958ce97f761da978d08c35c54a5c
Run this on the new node to join the cluster:
teleport start --roles=node --token=n92bb958ce97f761da978d08c35c54a5c --auth-server=10.0.10.1
```

Start `teleport` daemon on "luna" as shown above, but make sure to use the proper `--auth-server` 
IP to point back to your localhost.

Once you do that, "luna" will join the cluster. To verify, type this on your localhost:

```bash
> tsh --proxy=localhost ls

Node Name     Node ID                     Address            Labels
---------     -------                     -------            ------
localhost     xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.1:3022     
luna          xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.2:3022     
```

Notice the "Labels" column which is currently empty. Labels in teleport allow you to apply static
or dynamic labels to your nodes so you can quickly find the right node when you have many.

Lets stop `teleport` on "luna" and start it with the following command:

```bash
teleport start --roles=node --auth-server=10.0.10.1 --nodename=db --labels "location=virginia,arch=[1h:/bin/uname -m]"
```

Notice a few things here:

* We did not use `--token` flag because "luna" is already a member of the cluster.
* We renamed "luna" to "db" because this machine is running a database. This name only exists within Teleport, the actual hostname has not changed.
* We assigned a static label "location" to this host and set it to "viriginia".
* We also assigned a dynamic label "arch" which will evaluate `/bin/uname -m` command once an hour and assign the output to this label value.

Lets take a look at our cluster now:

```bash
> tsh --proxy=localhost ls

Node Name     Node ID                     Address            Labels
---------     -------                     -------            ------
localhost     xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.1:3022     
db            xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.2:3022     location=virginia,arch=x86_64
```

Lets use the newly created labels to filter the output of `tsh ls` and ask to show only
nodes located in Virginia:

```
> tsh --proxy=localhost ls location=virginia

Node Name     Node ID                     Address            Labels
---------     -------                     -------            ------
db            xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.2:3022     location=virginia,arch=x86_64
```

Labels can be used with the regular `ssh` command too. This will execute `ls -l /` command
on all servers located in Virginia:

```
> tsh --proxy=localhost ssh location=virginia ls -l /
```

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

![Teleport Diagram](img/teleport.png)

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
