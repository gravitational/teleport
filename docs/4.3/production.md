# Production Guide

This guide provides more in-depth details for running Teleport in Production.

<!-- TODO: Minimal Config example -->

## Prerequisites

* Read the [Architecture Overview](architecture/teleport_architecture_overview.md)
* Read through the [Installation Guide](installation.md) to see the available packages and binaries available.
* Read the CLI Docs for [`teleport`](cli-docs.md#teleport)

## Designing Your Cluster

Before installing anything there are a few things you should think about.

* Where will you host Teleport?
    * On-premises
    * Cloud VMs such as [AWS EC2](aws_oss_guide.md) or [GCE](gcp_guide.md)
    * An existing Kubernetes Cluster
* What does your existing network configuration look like?
    * Are you able to administer the network firewall rules yourself or do you need to work with a network admin?
    * Are these nodes accessible to the public Internet or behind NAT?
* Which users, ([Roles or ClusterRoles](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) on k8s) are set up on the existing system?
   * Can you add new users or Roles yourself or do you need to work with a system administrator?

## Firewall Configuration

Teleport services listen on several ports. This table shows the default port numbers.

|Port      | Service    | Description | Ingress | Egress
|----------|------------|-------------|---------|----------
| 3080      | Proxy      | HTTPS port clients connect to. Used to authenticate `tsh` users and web users into the cluster. | Allow inbound connections from HTTP and SSH clients. | Allow outbound connections to HTTP and SSH clients.
| 3023      | Proxy      | SSH port clients connect to after authentication. A proxy will forward this connection to port `3022` on the destination node. | Allow inbound traffic from SSH clients. | Allow outbound traffic to SSH clients.
| 3022      | Node       | SSH port to the Node Service. This is Teleport's equivalent of port `22` for SSH. | Allow inbound traffic from proxy host. | Allow outbound traffic to the proxy host.
| 3025      | Auth       | SSH port used by the Auth Service to serve its Auth API to other nodes in a cluster. | Allow inbound connections from all cluster nodes. | Allow outbound traffic to cluster nodes.
| 3024      | Proxy      | SSH port used to create "reverse SSH tunnels" from behind-firewall environments into a trusted proxy server. | <TODO> | <TODO>

<!--TODO: Add several diagrams of firewall config examples-->

## Installation

We have a detailed [installation guide](installation.md) which shows how to
install all available binaries or [install from
source](installation.md#installing-from-source). Reference that guide to learn
the best way to install Teleport for your system and the come back here to
finish your production install.

### Filesystem Layout

By default a Teleport node has the following files present. The location of all
of them is configurable.

| Default path                 | Purpose |
|------------------------------|---------|
| `/etc/teleport.yaml` | Teleport configuration file. |
| `/usr/local/bin/teleport` | Teleport daemon binary. |
| `/usr/local/bin/tctl` | Teleport admin tool. It is only needed for auth servers. |
| `/usr/local/bin/tsh` | Teleport CLI client tool. It is needed on any node that needs to connect to the cluster. |
| `/var/lib/teleport` | Teleport data directory. Nodes keep their keys and certificates there. Auth servers store the audit log and the cluster keys there, but the audit log storage can be further configured via `auth_service` section in the config file. |

## Running Teleport in Production

### Systemd Unit File

In production, we recommend starting teleport daemon via an init system like
`systemd`. If systemd and unit files are new to you, check out [this helpful guide](https://www.digitalocean.com/community/tutorials/understanding-systemd-units-and-unit-files). Here's an example systemd unit file for the Teleport [Proxy, Node and Auth Service](https://github.com/gravitational/teleport/tree/master/examples/systemd/production).

There are a couple of important things to notice about this file:

1. The start command in the unit file specifies `--config` as a file and there
   are very few flags passed to the `teleport` binary. Most of the configuration
   for Teleport should be done in the [configuration file](admin-guide.md#configuration).

2. The **ExecReload** command allows admins to run `systemctl reload teleport`.
   This will attempt to perform a graceful restart of Teleport _*but it only works if
   network-based backend storage like [DynamoDB](admin-guide.md#using-dynamodb) or
   [etc 3.3](admin-guide.md#using-etcd) is configured*_. Graceful Restarts will
   fork a new process to handle new incoming requests and leave the old daemon
   process running until existing clients disconnect.

### Start the Teleport Service

You can start Teleport via systemd unit by enabling the `.service` file
with the `systemctl` tool.

```bash
$ cd /etc/systemd/system

# Use your text editor of choice to create the .service file
# Here we use vim
$ vi teleport.service

# use the file linked above as is, or customize as needed
# save the file
$ systemctl enable teleport
$ systemctl start teleport

# show the status of the unit
$ systemctl status teleport

# follow tail of service logs
$ journalctl -fu teleport

# If you modify teleport.service later you will need to
# reload the systemctl daemon and reload teleport
# to apply your changes
$ systemctl daemon-reload
$ systemctl reload teleport
```

You can also perform restarts or upgrades by sending `kill` signals
to a Teleport daemon manually.

| Signal                  | Teleport Daemon Behavior |
|-------------------------|--------------------------|
| `USR1`                  | Dumps diagnostics/debugging information into syslog. |
| `TERM`, `INT` or `KILL` | Immediate non-graceful shutdown. All existing connections will be dropped. |
| `USR2`                  | Forks a new Teleport daemon to serve new connections. |
| `HUP`                   | Forks a new Teleport daemon to serve new connections **and** initiates the graceful shutdown of the existing process when there are no more clients connected to it. This is the signal sent to trigger a graceful restart. |

### Adding Nodes to the Cluster

We've written a dedicated guide on [Adding Nodes to your
Cluster](admin-guide.md#adding-nodes-to-the-cluster) which shows how to generate or set join tokens and
use them to add nodes.

## Security Considerations

### SSL/TLS for Teleport Proxy

TLS stands for Transport Layer Security (TLS), and its now-deprecated predecessor,
Secure Sockets Layer (SSL).  Teleport requires TLS authentication to ensure that
communication between nodes, clients and web proxy remains secure and comes from
a trusted source.

During our [quickstart](quickstart.md) guide we skip over setting up TLS so that you can quickly try Teleport.
Obtaining a TLS certificate is easy and is free with thanks to [Let's Encrypt](https://letsencrypt.org/).

If you use [certbot](https://certbot.eff.org/), you get this list of files provided:

```
README
cert.pem
chain.pem
fullchain.pem
privkey.pem
```

The files that are needed for Teleport are these:

```
https_key_file: /path/to/certs/privkey.pem
https_cert_file: /path/to/certs/fullchain.pem
```

If you already have a certificate these should be uploaded to the Teleport Proxy and
can be set via `https_key_file` and `https_cert_file`. Make sure any certificates
files uploaded contain a full certificate chain, complete with any intermediate
certificates required - this [guide](https://www.digicert.com/ssl-support/pem-ssl-creation.htm) may help.

```yaml
# This section configures the 'proxy service'
proxy_service:
    # Turns 'proxy' role on. Default is 'yes'
    enabled: yes

    # The DNS name the proxy HTTPS endpoint as accessible by cluster users.
    # Defaults to the proxy's hostname if not specified. If running multiple
    # proxies behind a load balancer, this name must point to the load balancer
    # (see public_addr section below)
    public_addr: proxy.example.com:3080

    # TLS certificate for the HTTPS connection. Configuring these properly is
    # critical for Teleport security.
    https_key_file: /var/lib/teleport/webproxy_key.pem
    https_cert_file: /var/lib/teleport/webproxy_cert.pem
```

When setting up on Teleport on AWS or GCP, we recommend leveraging their certificate
managers.

- [ACM](https://gravitational.com/teleport/docs/aws_oss_guide/#acm) on AWS
- [Google-managed SSL certificates](https://cloud.google.com/load-balancing/docs/ssl-certificates) on GCP

When setting up Teleport with a Cloud Provider, it can be common to terminate
TLS at the load balancer, then use an autoscaling group for the proxy nodes. When
setting up the proxy nodes start Teleport with:

`teleport start --insecure --roles=proxy --config=/etc/teleport.yaml`

See [Teleport Proxy HA](admin-guide.md#teleport-proxy-ha) for more info.


<!-- TODO Address vulns in quay? -->

<!-- TODO SSL for Webproxy & Auth Section -->

### CA Pinning

Teleport nodes use the HTTPS protocol to offer the join tokens to the auth
server. In a zero-trust environment, you must assume that an attacker can
hijack the IP address of the auth server.

To prevent this from happening, you need to supply every new node with an
additional bit of information about the auth server. This technique is called
"CA pinning". It works by asking the auth server to produce a "CA pin", which
is a hashed value of its private key, i.e. it cannot be forged by an attacker.

To get the current CA pin run this on the auth server:

```bash
$ tctl status
Cluster  staging.example.com
User CA  never updated
Host CA  never updated
CA pin   sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1
```

The CA pin at the bottom needs to be passed to the new nodes when they're starting
for the first time, i.e. when they join a cluster:

Via CLI:

```bash
$ teleport start \
   --roles=node \
   --token=1ac590d36493acdaa2387bc1c492db1a \
   --ca-pin=sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1 \
   --auth-server=10.12.0.6:3025
```

or via `/etc/teleport.yaml` on a node:

```yaml
teleport:
  auth_token: "1ac590d36493acdaa2387bc1c492db1a"
  ca_pin: "sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1"
  auth_servers:
    - "10.12.0.6:3025"
```

!!! warning "Warning"

    If a CA pin is not provided, Teleport node will join a cluster but it will
    print a `WARN` message (warning) into its standard error output.

!!! warning "Warning"

    The CA pin becomes invalid if a Teleport administrator performs the CA
    rotation by executing `tctl auth rotate`.

### Secure Data Storage

By default the `teleport` daemon uses the local directory `/var/lib/teleport` to
store its data. This applies to any role or service including Auth, Node, or Proxy.
While an Auth node hosts the most sensitive data you will want to prevent
unauthorized access to this directory. Make sure that regular/non-admin users
do not have access to this folder, particularly on the Auth server. Change
the ownership of the directory with [`chown`](https://linuxize.com/post/linux-chown-command/)

```bash
$ sudo teleport start
```

If you are logged in as `root` you may want to create a new OS-level user first.
On Linux, create a new user called `<username>` with the following commands:

```bash
$ adduser <username>
$ su <username>
```

<!--  Security considerations on installing tctl under root or not -->
