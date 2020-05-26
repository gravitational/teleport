# Quick Start Guide

Welcome to the Teleport Quick Start Guide!

The goal of this document is to show off the basic capabilities of Teleport.
There are three types of services Teleport nodes can run: `nodes`, `proxies`
and `auth servers`.

- **Auth servers** store user accounts
  and provide authentication and authorization services for every node and
  every user in a cluster.
- **Proxy servers** route client connection requests to the appropriate node
  and serve a Web UI which can also be used to log into SSH nodes. Every
  client-to-node connection in Teleport must be routed through the proxy.
- **Nodes** are regular SSH nodes, similar to the `sshd` daemon you may be
  familiar with. When a node receives a connection request, the request is
  authenticated through the cluster's auth server.

The `teleport` daemon runs all three of these services by default. This Quick Start Guide will
be using this default behavior to create a cluster and interact with it
using Teleport's client-side tools:

| Tool    | Description
|---------|-------------
| tctl    | Cluster administration tool used to invite nodes to a cluster and manage user accounts.
| tsh     | Similar in principle to OpenSSH's `ssh`. Used to login into remote SSH nodes, list and search for nodes in a cluster, securely upload/download files, etc.
| browser | You can use your web browser to login into any Teleport node by opening `https://<proxy-host>:3080`.

## Installing and Starting

Teleport runs on Linux, MacOS and Teleport 3.0 now supports Windows for running `tsh login`.
You can download pre-built binaries from [here](https://gravitational.com/teleport/download/)
or you can [build it from source](https://github.com/gravitational/teleport).

After downloading the binary tarball, run:

```bash
$ tar -xzf teleport-binary-release.tar.gz
$ cd teleport
$ sudo ./install
```

This will copy Teleport binaries to `/usr/local/bin`.

Let's start Teleport on a single-node. First, create a directory for Teleport
to keep its data. By default it's `/var/lib/teleport`. Then start `teleport` daemon:

```bash
$ mkdir -p /var/lib/teleport
$ sudo teleport start

[AUTH]  Auth service is starting on 0.0.0.0:3025
[PROXY] Reverse tunnel service is starting on 0.0.0.0:3024
[PROXY] Web proxy service is starting on 0.0.0.0:3080
[PROXY] SSH proxy service is starting on 0.0.0.0:3023
[SSH]   Service is starting on 0.0.0.0:3022
```

At this point you should see Teleport print listening IPs of all 3 services into the console.

Congratulations - you are now running Teleport!

If you wish to deploy Teleport inside a Docker container:

```
# This command will pull the Teleport container image for version 4.0.0-rc.2
# Replace 2.7.3 with the version you want to run:
$ docker pull quay.io/gravitational/teleport:4.0.0-rc.2
```

## Creating Users

Teleport users are defined on a cluster level and every Teleport user must be associated with
a list of machine-level OS usernames it can authenticate as during a login. This list is
called "user mappings".

If you do not specify the mappings, the new Teleport user will be assigned a mapping with
the same name. Let's create a Teleport user with the same name as the OS user:

```bash
$ sudo tctl users add $USER

Signup token has been created. Share this URL with the user:
https://localhost:3080/web/newuser/96c85ed60b47ad345525f03e1524ac95d78d94ffd2d0fb3c683ff9d6221747c2
```

`tctl` prints a sign-up URL for you to open in your browser and complete registration:

![teleport login](/img/login.png?style=grv-image-center-md)

Teleport enforces two-factor authentication. If you do not already have Google
Authenticator (or another 2FA client), you will have to install it on your
smart phone. Then you can scan the bar code on the Teleport login web page,
pick a password and enter in the two factor token.

The default TTL for a login is 12 hours but this can be configured to be another value.

Having done that, you will be presented with a Web UI where you will see your
machine and will be able to log in to it using web-based terminal.

![teleport ui](/img/firstpage.png?style=grv-image-center-md)

## Logging in Through CLI

Let's login using the `tsh` command line tool:

```bash
$ tsh --proxy=localhost --insecure login
```

Notice that `tsh` client always needs `--proxy` flag because all client connections
in Teleport must to go through a proxy, sometimes called a "bastion".

!!! warning "Warning":
    For the purposes of this quickstart we are using the `--insecure` flag which allows
    us to skip configuring the HTTP/TLS certificate for Teleport proxy. Your browser will
    throw a warning **Your connection is not private**. Click Advanced, and **Proceed to 0.0.0.0 (unsafe)**
    to preview the Teleport UI.

    Never use `--insecure` in production unless you terminate SSL at a load balancer. This will
    apply to most cloud providers (AWS, GCP and Azure). You must configure a HTTP/TLS certificate for the Proxy.
    This process has been made easier with Let's Encrypt. [See instructions here](https://gravitational.com/blog/letsencrypt-teleport-ssh/).

If successful, `tsh login` command will receive a user certificate for a given proxy
and will store it in `~/.tsh/keys/<proxy>` directory.

With a certificate in place, a user can SSH into any host behind the proxy:

```
$ tsh ssh localhost
```

!!! tip "Tip":
    To avoid typing "tsh ssh" a user may rename `tsh` binary to `ssh` and use the familiar syntax as in `ssh localhost`.

## Adding Nodes to Cluster

Let's add another node to the cluster. The `tctl` command below will create a single-use
token for a node to join and will print instructions for you to follow:

```bash
$ sudo tctl nodes add

The invite token: n92bb958ce97f761da978d08c35c54a5c
Run this on the new node to join the cluster:
teleport start --roles=node --token=n92bb958ce97f761da978d08c35c54a5c --auth-server=10.0.10.1
```

Start `teleport` daemon on a new node as shown above, but make sure to use the proper
`--auth-server` IP to point back to your localhost.

Once you do that, verify that the new node has joined the cluster:

```bash
$ tsh --proxy=localhost ls

Node Name     Node ID                     Address            Labels
---------     -------                     -------            ------
localhost     xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.1:3022
new-node      xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.2:3022
```

!!! tip "NOTE":
    Teleport also supports static pre-defined invitation tokens which can be set in the [configuration file](admin-guide.md#adding-nodes-to-the-cluster)

## Adding a node located behind NAT
With the current setup you've only been able to add nodes that have direct access to the
auth server and within the internal IP range of the cluster. We recommend
setting up a [Trusted Cluster](/trustedclusters) if you have workloads split
across different networks / clouds.

Teleport Node Tunneling lets you add a node to an existing Teleport Cluster. This can be
useful for IoT applications or for managing a couple of servers in a different network.

Similar to [Adding Nodes to Cluster](/quickstart/#adding-nodes-to-cluster), use `tctl` to
create a single-use token for a node, but this time you'll replace the auth server IP with
the URL of the Proxy Server. In the Example below, we've replaced the auth server IP with the Proxy
web endpoint `teleport.example.com`.

```bash
$ sudo tctl nodes add

The invite token: n92bb958ce97f761da978d08c35c54a5c
Run this on the new node to join the cluster:
teleport start --roles=node --token=n92bb958ce97f761da978d08c35c54a5c --auth-server=teleport-proxy.example.com
```

Using the ports in the default configuration, the node needs to be able to talk to ports 3080
and 3024 on the proxy. Port 3080 is used to initially fetch the credentials (SSH and TLS certificates)
and for discovery (where is the reverse tunnel running, in this case 3024). Port 3024 is used to
establish a connection to the Auth Server through the proxy.

## Using Node Labels

Notice the "Labels" column in the output above. It is currently not populated. Teleport lets
you apply static or dynamic labels to your nodes. As the cluster grows and nodes assume different
roles, labels will help to find the right node quickly.

Let's see labels in action. Stop Teleport `ctrl-c` on the node we just added and restart it with the following command:

```bash
$ sudo teleport start --roles=node --auth-server=10.0.10.1 --nodename=db --labels "location=virginia,arch=[1h:/bin/uname -m]"
```

Notice a few things here:

* We did not use `--token` flag this time, because this node is already a member of the cluster.
* We explicitly named this node as "db" because this machine is running a database. This name only exists within Teleport, the actual hostname has not changed.
* We assigned a static label "location" to this host and set it to "virginia".
* We also assigned a dynamic label "arch" which will evaluate `/bin/uname -m` command once an hour and assign the output to this label value.

Let's take a look at our cluster now:

```bash
$ tsh --proxy=localhost ls

Node Name     Node ID                     Address            Labels
---------     -------                     -------            ------
localhost     xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.1:3022
db            xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.2:3022     location=virginia,arch=x86_64
```

Let's use the newly created labels to filter the output of `tsh ls` and ask to show only
nodes located in Virginia:

```bash
$ tsh --proxy=localhost ls location=virginia

Node Name     Node ID                     Address            Labels
---------     -------                     -------            ------
db            xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.2:3022     location=virginia,arch=x86_64
```

Labels can be used with the regular `ssh` command too. This will execute `ls -l /` command
on all servers located in Virginia:

```
$ tsh --proxy=localhost ssh location=virginia ls -l /
```

## Sharing SSH Sessions

Suppose you are trying to troubleshoot a problem on a node. Sometimes it makes sense to ask
another team member for help. Traditionally this could be done by letting them know which
node you're on, having them SSH in, start a terminal multiplexer like `screen` and join a
session there.

Teleport makes this a bit more convenient. Let's log into "db" and ask Teleport for your
current session status:

```bash
$ tsh --proxy=teleport.example.com ssh db
db > teleport status

User ID    : joe, logged in as joe from 10.0.10.1 43026 3022
Session ID : 7645d523-60cb-436d-b732-99c5df14b7c4
Session URL: https://teleport.example.com:3080/web/sessions/7645d523-60cb-436d-b732-99c5df14b7c4
```

You can share the Session URL with a colleague in your organization. Assuming that your colleague has access to `teleport.example.com` proxy, she will be able to join and help you troubleshoot the problem on "db" in her browser.

Also, people can join your session via terminal assuming they have Teleport installed and running. They just have to run:

```bash
$ tsh --proxy=teleport.example.com join 7645d523-60cb-436d-b732-99c5df14b7c4
```

!!! tip "NOTE":
    For this to work, both of you must have proper user mappings allowing you access `db` under the same OS user.

## Running in Production

We hope this quickstart guide has helped you to quickly set up and play with Teleport. For production environments, we strongly recommend the following:

- Install HTTPS certificates for every Teleport proxy. If setting up in a HA config
  on AWS, GCP or another cloud you'll likely terminate TLS at a Load Balancer. In
  this case you'll need to start teleport with `--insecure`
- Run Teleport `auth` on isolated servers. The auth service can run in a
  highly available (HA) configuration.
- Use a configuration file instead of command line flags because it gives you
  more flexibility.
- Review the [Architecture Overview](architecture.md), [Admin Manual](admin-guide.md) and [User Manual](user-manual.md) for a better understanding of Teleport.
