# Quick Start

### Introduction

Welcome to the Teleport Quick Start Guide. The goal of this document is to show off the basic
capabilities of Teleport.

There are three types of services Teleport nodes can run: `nodes`, `proxies` and `auth servers`.

- An auth server is the core of a cluster. Auth servers store user accounts and offer  
  authentication and authorization service for every node and every user in a cluster.
- Nodes are regular SSH nodes, similar to the `sshd` daemon you are probably used to. When a node receives
  a connection request, it authenticates it via the cluster's auth server.
- Proxies route client connection requests to the appropriate node and serve a Web UI 
  which can also be used to login into SSH nodes. Every client-to-node connection in 
  Teleport must be routed via a proxy.

The `teleport` daemon runs all 3 of these services by default. This Quick Start Guide will
be using this default behavior to create a cluster and interact with it
using Teleport's client-side tools:

| Tool           | Description
|----------------|------------------------------------------------------------------------
| tctl    | Cluster administration tool used to invite nodes to a cluster and manage user accounts.
| tsh     | Similar in principle to OpenSSH's `ssh`. Used to login into remote SSH nodes, list and search for nodes in a cluster, securely upload/download files, etc.
| browser | You can use your web browser to login into any Teleport node by opening `https://<proxy-host>:3080`.

### Installing

Gravitational Teleport natively runs on any modern Linux distribution and OSX. You can
download pre-built binaries from [here](https://github.com/gravitational/teleport/releases)
or you can [build it from source](https://github.com/gravitational/teleport).

### Starting Teleport

Let's start Teleport on a single-node. First, create a directory for Teleport 
to keep its data. By default it's `/var/lib/teleport`. Then start `teleport` daemon:

```bash
mkdir -p /var/lib/teleport
teleport start
```

At this point you should see Teleport print listening IPs of all 3 services into the console.
Congratulations - You are running Teleport! 

### Creating Users

Teleport users are defined on a cluster level, and every Teleport user must be associated with
a list of machine-level OS usernames it can authenticate as during a login. This list is 
called "user mappings".

If you do not specify the mappings, the new Teleport user will be assigned a mapping with
the same name. Let's create a Teleport user with the same name as the OS user:

```bash
> tctl users add $USER

Signup token has been created. Share this URL with the user:
https://localhost:3080/web/newuser/96c85ed60b47ad345525f03e1524ac95d78d94ffd2d0fb3c683ff9d6221747c2
```

`tctl` prints a sign-up URL for you to visit and complete registration. Open this link in a 
browser, install Google Authenticator on your phone, set up 2nd factor authentication and 
pick a password. The default TTL for a login is 12 hours but this can be configured to a
maximum of 30 hours and a minimum of 1 minute.

Having done that, you will be presented with a Web UI where you will see your machine and 
will be able to log into it using web-based terminal.

### Login

Let's login using the `tsh` command line tool:

```bash
tsh --proxy=localhost localhost
```

Notice that `tsh` client always needs `--proxy` flag because all client connections
in Teleport have to go via a proxy sometimes called an "SSH bastion".

### Adding Nodes to Cluster

Let's add another node to your cluster. Let's assume the other node can be reached by
hostname "luna". `tctl` command below will create a single-use token for a node to 
join and will print instructions for you to follow:

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

### Using Node Labels

Notice the "Labels" column in the output above. It is currently not populated. Teleport lets 
you apply static or dynamic labels to your nodes. As the cluster grows and nodes assume different 
roles, labels will help to find the right node quickly.

Let's see labels in action. Stop `teleport` on "luna" and restart it with the following command:

```bash
teleport start --roles=node --auth-server=10.0.10.1 --nodename=db --labels "location=virginia,arch=[1h:/bin/uname -m]"
```

Notice a few things here:

* We did not use `--token` flag this time, because "luna" is already a member of the cluster.
* We renamed "luna" to "db" because this machine is running a database. This name only exists within Teleport, the actual hostname has not changed.
* We assigned a static label "location" to this host and set it to "virginia".
* We also assigned a dynamic label "arch" which will evaluate `/bin/uname -m` command once an hour and assign the output to this label value.

Let's take a look at our cluster now:

```bash
> tsh --proxy=localhost ls

Node Name     Node ID                     Address            Labels
---------     -------                     -------            ------
localhost     xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.1:3022     
db            xxxxx-xxxx-xxxx-xxxxxxx     10.0.10.2:3022     location=virginia,arch=x86_64
```

Let's use the newly created labels to filter the output of `tsh ls` and ask to show only
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

### Sharing SSH Sessions with Colleagues

Suppose you are trying to troubleshoot a problem on a node. Sometimes it makes sense to ask 
another team member for help. Traditionally this could be done by letting them know which 
node you're on, having them SSH in, start a terminal multiplexer like `screen` and join a 
session there.

Teleport makes this a bit more convenient. Let's login into "db" and ask Teleport for your 
current session status:

```bash
> tsh --proxy=teleport.example.com ssh db
db > teleport status

User ID    : joe, logged in as joe from 10.0.10.1 43026 3022
Session ID : 7645d523-60cb-436d-b732-99c5df14b7c4
Session URL: https://teleport.example.com:3080/web/sessions/7645d523-60cb-436d-b732-99c5df14b7c4
```

You can share the Session URL with a colleague in your organization. Assuming that `teleport.example.com`
is your company's Teleport proxy, she will be able to join and help you troubleshoot the
problem on "db" in her browser.

Also, people can join your session via CLI assuming they have Teleport installed. They just have to run:

```bash
> tsh --proxy=teleport.example.com join 7645d523-60cb-436d-b732-99c5df14b7c4
```

NOTE: for this to work, both of you must have proper user mappings allowing you 
access `db` under the same OS user.

### Inviting Colleagues to your Laptop

Sometimes you may want to temporarily share the terminal on your own laptop (if you
trust your guests, of course). First, you will have to start teleport with `--roles=node` in
a separate Terminal:

```bash
> teleport start --proxy=teleport.example.com
```

... then you will need to start a local SSH session by logging into localhost and
asking for a session ID:

```bash
> tsh --proxy=teleport.example.com ssh localhost
localhost> teleport status
```

Now you can invite someone into your localhost session. They will need to have a proper
user mapping, of course, to be allowed to join your session. To disconnect, shut down 
`teleport` daemon or simply exit the `tsh` session.

### Running in Production

We hope this Guide helped you to quickly set up Teleport to play with on
localhost. For production environments we strongly recommend the following:

- Install HTTPS certificates for every Teleport proxy.
- Run Teleport `auth` on isolated servers. The auth service can run in a 
  highly available (HA) configuration.
- Use a configuration file instead of command line flags because it gives you 
  more flexibility, for example for configuring HA clusters.
