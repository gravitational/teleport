# Teleport Enterprise Quick Start

Welcome to the Quick Start Guide to Teleport Enterprise! 

The goal of this document is to show off the basic capabilities of Teleport.
There are three types of services Teleport nodes can run: `nodes`, `proxies`
and `auth servers`.

- **Auth servers** are the core of a cluster. Auth servers store user accounts and
  provide authentication and authorization services for every node and every
  user in a cluster.
- **Proxy servers** route client connection requests to the appropriate node and serve a
  Web UI which can also be used to log into SSH nodes. Every client-to-node
  connection in Teleport must be routed via a proxy.
- **Nodes** are regular SSH servers, similar to the `sshd` daemon you may be familiar
  with. When a node receives a connection request, the request is authenticated
  through the cluster's auth server.

The `teleport` daemon runs all three of these services by default. This Quick
Start Guide will be using this default behavior to create a cluster and
interact with it using Teleport's client-side tools:

| Tool       | Description
|------------|------------
| tctl       | Cluster administration tool used to invite nodes to a cluster and manage user accounts.
| tsh        | Similar in principle to OpenSSH's `ssh`. Used to login into remote SSH nodes, list and search for nodes in a cluster, securely upload/download files, etc.
| browser    | You can use your web browser to login into any Teleport node by opening `https://<proxy-host>:3080`.

## Prerequisites

You will need to have access to the [customer portal](https://dashboard.gravitational.com) 
to download the software. You will also need three computers: two servers and
one client (probably a laptop) to complete this tutorial. Lets assume the servers will have
the following DNS names and IPs:

Server Name    |  IP Address    | Purpose
---------------|----------------|--------------
_"auth.example.com"_  | 10.1.1.10      | This server will be used to run all three Teleport services: auth, proxy and node.
_"node.example.com"_  | 10.1.1.11      | This server will only run the SSH service. Vast majority of servers in production will be nodes.

This Quick Start Guide assumes that the both servers are running a [systemd-based](https://www.freedesktop.org/wiki/Software/systemd/) 
Linux distribution such as Debian, Ubuntu or a RHEL deriviative.

## Installing

To start using Teleport Enterprise you will need to:

* Download the binaries
* Download the license file
* Read this documentation

You can download both the binaries and the license file from the [customer portal](https://dashboard.gravitational.com).
After downloading the binary tarball, run:

```bash
$ tar -xzf teleport-binary-release.tar.gz
$ cd teleport
```

* Copy `teleport` and `tctl` binaries to a bin directory (we suggest `/usr/local/bin`) on the auth server.
* Copy `teleport` binary to a bin directory on the node server.
* Copy `tsh` binary to a bin directory on the client computer.

### License File

The Teleport license file contains a X.509 certificate and the corresponding
private key in [PEM format](https://en.wikipedia.org/wiki/Privacy-enhanced_Electronic_Mail). 

Download the license file from the [customer portal](https://dashboard.gravitational.com) 
and save it as `/var/lib/teleport/license.pem` on the auth server.


### Configuration File

Save the following configuration file as `/etc/teleport.yaml` on the _node.example.com_:

```bash
teleport:
  auth_token: dogs-are-much-nicer-than-cats
  # you can also use auth server's IP, i.e. "10.1.1.10:3025"
  auth_servers: [ "auth.example.com:3025" ]

  # enable ssh service and disable auth and proxy:
ssh_service:
  enabled: true

auth_service:
  enabled: false
proxy_service:
  enabled: false
```

Now, save the following configuration file as `/etc/teleport.yaml` on the _auth.example.com_:

```bash
teleport:
  auth_token: dogs-are-much-nicer-than-cats
  auth_servers: [ "localhost:3025" ]

auth_service:
  # enable the auth service:
  enabled: true

  tokens:
  # this static token is used for other nodes to join this Teleport cluster
  - proxy,node:dogs-are-much-nicer-than-cats
  # this token is used to establish trust with other Teleport clusters
  - trusted_cluster:trains-are-superior-to-cars

  # by default, local authentication will be used with 2FA
  authentication:
      second_factor: otp

  # SSH is also enabled on this node:
ssh_service:
  enabled: "yes"
```

### Systemd Unit File

Next, download the systemd service unit file from [examples directory](https://github.com/gravitational/teleport/tree/master/examples/systemd) 
on Github and save it as `/etc/systemd/system/teleport.service` on both servers.

```bash
# run this on both servers:
$ sudo systemctl daemon-reload
$ sudo systemctl enable teleport
```

## Starting

```bash
# run this on both servers:
$ sudo systemctl start teleport
```

Teleport daemon should start and you can use `netstat -lptne` to make sure that
it's listening on [TCP/IP ports](admin-guide/#ports). On _auth.example.com_ it should 
look something like this:

```bash
$ auth.example.com ~: sudo netstat -lptne
Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address   State       User       PID/Program name    
tcp6       0      0 :::3024         LISTEN      0          337/teleport        
tcp6       0      0 :::3025         LISTEN      0          337/teleport        
tcp6       0      0 :::3080         LISTEN      0          337/teleport        
tcp6       0      0 :::3022         LISTEN      0          337/teleport        
tcp6       0      0 :::3023         LISTEN      0          337/teleport        
```

and _node.example.com_ should look something like this:

```bash
$ node.example.com ~: sudo netstat -lptne
Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address   State       User       PID/Program name    
tcp6       0      0 :::3022         LISTEN      0          337/teleport        
```

See [troubleshooting](#troubleshooting) section at the bottom if something is not working.

## Adding Users

[TBD]

## Troubleshooting

If Teleport services do not start, take a look at the syslog:

```
$ sudo journalctl -fu teleport
```

Usually the error will be reported there. Common reasons for failure are:

* Mismatched tokens, i.e. "auth_token" on the node does not match "tokens/node" value on the auth server.
* Network issues: port `3025` is closed via iptables.
* Network issues: ports `3025` or `3022` are occupied by another process.
* Disk issues: Teleport fails to create `/var/lib/teleport` because the volume is read-only or not accessible.

## Getting Help

If something is not working, please reach out to us by creating a ticket via [customer portal](https://dashboard.gravitational.com/).
Customers who have purchased the premium support package, or submit a message
to the Slack channel.
