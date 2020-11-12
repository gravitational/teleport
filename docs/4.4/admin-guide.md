---
title: Teleport Admin Manual
description: Admin manual for how to configure identity-aware SSH, certificate-based SSH authentication, set up SSO for SSH, SSO for Kubernetes, and more.
---

# Teleport Admin Manual

This manual covers the installation and configuration of Teleport and the
ongoing management of a Teleport cluster. It assumes that the reader has good
understanding of Linux administration.

## Installing

Please visit our [installation page](installation.md) for instructions on downloading and installing Teleport.

## Definitions

Before diving into configuring and running Teleport, it helps to take a look at
the [Teleport Architecture](architecture/overview.md) and review the key concepts this
document will be referring to:

|Concept   | Description
|----------|------------
|Node      | Synonym to "server" or "computer", something one can "SSH to". A node must be running the [ `teleport` ](cli-docs.md#teleport) daemon with "node" role/service turned on.
|Certificate Authority (CA) | A pair of public/private keys Teleport uses to manage access. A CA can sign a public key of a user or node, establishing their cluster membership.
|Teleport Cluster | A Teleport Auth Service contains two CAs. One is used to sign user keys and the other signs node keys. A collection of nodes connected to the same CA is called a "cluster".
|Cluster Name | Every Teleport cluster must have a name. If a name is not supplied via `teleport.yaml` configuration file, a GUID will be generated.**IMPORTANT:** renaming a cluster invalidates its keys and all certificates it had created.
|Trusted Cluster | Teleport Auth Service can allow 3rd party users or nodes to connect if their public keys are signed by a trusted CA. A "trusted cluster" is a pair of public keys of the trusted CA. It can be configured via `teleport.yaml` file.

## Teleport Daemon

The Teleport daemon is called [ `teleport` ](cli-docs.md#teleport) and it supports
the following commands:

|Command     | Description
|------------|-------------------------------------------------------
|start       | Starts the Teleport daemon.
|configure   | Dumps a sample configuration file in YAML format into standard output.
|version     | Shows the Teleport version.
|status      | Shows the status of a Teleport connection. This command is only available from inside of an active SSH session.
|help        | Shows help.

When experimenting, you can quickly start [ `teleport` ](cli-docs.md#teleport)
with verbose logging by typing [ `teleport start -d` ](cli-docs.md#teleport-start)
.

!!! danger "WARNING"

    Teleport stores data in `/var/lib/teleport` . Make sure that
    regular/non-admin users do not have access to this folder on the Auth
    server.

### Systemd Unit File

In production, we recommend starting teleport daemon via an init system like
`systemd` . Here's the recommended Teleport service unit file for systemd:

``` systemd
[Unit]
Description=Teleport SSH Service
After=network.target

[Service]
Type=simple
Restart=on-failure
ExecStart=/usr/local/bin/teleport start --config=/etc/teleport.yaml --pid-file=/run/teleport.pid
ExecReload=/bin/kill -HUP $MAINPID
PIDFile=/run/teleport.pid

[Install]
WantedBy=multi-user.target
```

### Graceful Restarts

If using the systemd service unit file above, executing `systemctl reload
teleport` will perform a graceful restart, i.e.the Teleport daemon will fork a
new process to handle new incoming requests, leaving the old daemon process
running until existing clients disconnect.

!!! warning "Version warning"

    Graceful restarts only work if Teleport is
    deployed using network-based storage like DynamoDB or etcd 3.3+. Future
    versions of Teleport will not have this limitation.

You can also perform restarts/upgrades by sending `kill` signals to a Teleport
daemon manually.

| Signal                  | Teleport Daemon Behavior
|-------------------------|---------------------------------------
| `USR1` | Dumps diagnostics/debugging information into syslog.
| `TERM` , `INT` or `KILL` | Immediate non-graceful shutdown. All existing connections will be dropped.
| `USR2` | Forks a new Teleport daemon to serve new connections.
| `HUP` | Forks a new Teleport daemon to serve new connections **and** initiates the graceful shutdown of the existing process when there are no more clients connected to it.

### Ports

Teleport services listen on several ports. This table shows the default port
numbers.

|Port      | Service    | Description
|----------|------------|-------------------------------------------
|3022      | Node       | SSH port. This is Teleport's equivalent of port `#22` for SSH.
|3023      | Proxy      | SSH port clients connect to. A proxy will forward this connection to port `#3022` on the destination node.
|3024      | Proxy      | SSH port used to create "reverse SSH tunnels" from behind-firewall environments into a trusted proxy server.
|3025      | Auth       | SSH port used by the Auth Service to serve its API to other nodes in a cluster.
|3080      | Proxy      | HTTPS connection to authenticate `tsh` users and web users into the cluster. The same connection is used to serve a Web UI.
|3026      | Kubernetes Proxy      | HTTPS Kubernetes proxy (if enabled)

### Filesystem Layout

By default, a Teleport node has the following files present. The location of all
of them is configurable.

| Full path                 | Purpose               |
|---------------------------|-----------------------|
| `/etc/teleport.yaml` | Teleport configuration file (optional).|
| `/usr/local/bin/teleport` | Teleport daemon binary.|
| `/usr/local/bin/tctl` | Teleport admin tool. It is only needed for auth servers.|
| `/var/lib/teleport` | Teleport data directory. Nodes keep their keys and certificates there. Auth servers store the audit log and the cluster keys there, but the audit log storage can be further configured via `auth_service` section in the config file.|

## Configuration

You should use a [configuration file](#configuration-file) to configure the
[ `teleport` ](cli-docs.md#teleport) daemon. For simple experimentation, you can
use command line flags with the [ `teleport start` ](cli-docs.md#teleport-start)
command. Read about all the allowed flags in the [CLI
Docs](cli-docs.md#teleport-start) or run `teleport start --help`

### Configuration File

Teleport uses the YAML file format for configuration. A sample configuration
file is shown below. By default, it is stored in `/etc/teleport.yaml`, below is
an expanded and commented version from `teleport configure`.

The default path Teleport uses to look for a config file is `/etc/teleport.yaml`. You can override
this path and set it explicitly using the `-c` or `--config` flag to `teleport start`:

```bash
$ teleport start --config=/etc/teleport.yaml
```

For a complete reference, see our [Configuration Reference - teleport.yaml](config-reference.md#teleportyaml)

!!! note "IMPORTANT"

    When editing YAML configuration, please pay attention to how your
    editor handles white space. YAML requires consistent handling of
    tab characters.

``` yaml
#
# Sample Teleport configuration file
# Creates a single proxy, auth and node server.
#
# Things to update:
#  1. ca_pin: Obtain the CA pin hash for joining more nodes by running 'tctl status'
#     on the auth server once Teleport is running.
#  2. license-if-using-teleport-enterprise.pem: If you are an Enterprise customer,
#     obtain this from https://dashboard.gravitational.com/web/login
#
teleport:
  # nodename allows to assign an alternative name this node can be reached by.
  # by default it's equal to hostname
  nodename: NODE_NAME
  data_dir: /var/lib/teleport

  # Invitation token used to join a cluster. it is not used on
  # subsequent starts
  auth_token: xxxx-token-xxxx

  # Optional CA pin of the auth server. This enables more secure way of adding new
  # nodes to a cluster. See "Adding Nodes" section above.
  ca_pin: "sha256:ca-pin-hash-goes-here"

  # list of auth servers in a cluster. you will have more than one auth server
  # if you configure teleport auth to run in HA configuration.
  # If adding a node located behind NAT, use the Proxy URL. e.g.
  #  auth_servers:
  #     - teleport-proxy.example.com:3080
  auth_servers:
      - 10.1.0.5:3025
      - 10.1.0.6:3025

  # Logging configuration. Possible output values to disk via '/var/lib/teleport/teleport.log',
  # 'stdout', 'stderr' and 'syslog'. Possible severity values are INFO, WARN
  # and ERROR (default).
  log:
    output: stderr
    severity: INFO

auth_service:
  enabled: "yes"
  # A cluster name is used as part of a signature in certificates
  # generated by this CA.
  #
  # We strongly recommend to explicitly set it to something meaningful as it
  # becomes important when configuring trust between multiple clusters.
  #
  # By default an automatically generated name is used (not recommended)
  #
  # IMPORTANT: if you change cluster_name, it will invalidate all generated
  # certificates and keys (may need to wipe out /var/lib/teleport directory)
  cluster_name: "teleport-aws-us-east-1"

  # IP and the port to bind to. Other Teleport nodes will be connecting to
  # this port (AKA "Auth API" or "Cluster API") to validate client
  # certificates
  listen_addr: 0.0.0.0:3025

  tokens:
  - proxy,node:xxxx-token-xxxx
  # license_file: /path/to/license-if-using-teleport-enterprise.pem

  authentication:
    # default authentication type. possible values are 'local' and 'github' for OSS
    #  and 'oidc', 'saml' and 'false' for Enterprise.
    type: local
    # second_factor can be off, otp, or u2f
    second_factor: otp
ssh_service:
  enabled: "yes"
  labels:
    teleport: static-label-example
  commands:
  - name: hostname
    command: [/usr/bin/hostname]
    period: 1m0s
  - name: arch
    command: [/usr/bin/uname, -p]
    period: 1h0m0s
proxy_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3023
  web_listen_addr: 0.0.0.0:3080
  tunnel_listen_addr: 0.0.0.0:3024

  # The DNS name of the proxy HTTPS endpoint as accessible by cluster users.
  # Defaults to the proxy's hostname if not specified. If running multiple
  # proxies behind a load balancer, this name must point to the load balancer
  # (see public_addr section below)
  public_addr: TELEPORT_PUBLIC_DNS_NAME:3080

  # TLS certificate for the HTTPS connection. Configuring these properly is
  # critical for Teleport security.
  https_key_file: /etc/letsencrypt/live/TELEPORT_PUBLIC_DNS_NAME/privkey.pem
  https_cert_file: /etc/letsencrypt/live/TELEPORT_PUBLIC_DNS_NAME/fullchain.pem
```

#### Public Addr

Notice that all three Teleport services (proxy, auth, node) have an optional
`public_addr` property. The public address can take an IP or a DNS name. It can
also be a list of values:

``` yaml
public_addr: ["proxy-one.example.com", "proxy-two.example.com"]
```

Specifying a public address for a Teleport service may be useful in the
following use cases:

* You have multiple identical services, like proxies, behind a load balancer.
* You want Teleport to issue SSH certificate for the service with the additional
  principals, e.g.host names.

## Authentication

Teleport uses the concept of "authentication connectors" to authenticate users
when they execute [ `tsh login` ](cli-docs.md#tsh-login) command. There are three
types of authentication connectors:

### Local Connector

Local authentication is used to authenticate against a local Teleport user
database. This database is managed by [ `tctl users` ](cli-docs.md#tctl-users-add)
command. Teleport also supports second factor authentication (2FA) for the local
connector. There are three possible values (types) of 2FA:

  + `otp` is the default. It implements [TOTP](https://en.wikipedia.org/wiki/Time-based_One-time_Password_Algorithm)
     standard. You can use [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator)
     or [Authy](https://www.authy.com/) or any other TOTP client.

  + `u2f` implements [U2F](https://en.wikipedia.org/wiki/Universal_2nd_Factor)
    standard for utilizing hardware (USB) keys for second factor. You can use [YubiKeys](https://www.yubico.com/),
   [SoloKeys](https://solokeys.com/) or any other hardware token which implements the FIDO U2F standard.

  + `off` turns off second factor authentication.

Here is an example of this setting in the `teleport.yaml` :

``` yaml
auth_service:
  authentication:
    type: local
    second_factor: off
```

### Github OAuth 2.0 Connector

This connector implements Github OAuth 2.0 authentication flow. Please refer to
Github documentation on [Creating an OAuth App](https://developer.github.com/apps/building-oauth-apps/creating-an-oauth-app/)
to learn how to create and register an OAuth app.

Here is an example of this setting in the `teleport.yaml` :

``` yaml
auth_service:
  authentication:
    type: github
```

See [Github OAuth 2.0](#github-oauth-20) for details on how to configure it.

### SAML

This connector type implements SAML authentication. It can be configured against
any external identity manager like Okta or Auth0. This feature is only available
for Teleport Enterprise.

Here is an example of this setting in the `teleport.yaml` :

``` yaml
auth_service:
  authentication:
    type: saml
```

### OIDC

Teleport implements OpenID Connect (OIDC) authentication, which is similar to
SAML in principle. This feature is only available for Teleport Enterprise.

Here is an example of this setting in the `teleport.yaml` :

``` yaml
auth_service:
  authentication:
    type: oidc
```

### Hardware Keys - YubiKey FIDO U2F

Teleport supports [FIDO U2F](https://www.yubico.com/about/background/fido/)
hardware keys as a second authentication factor. By default U2F is disabled. To
start using U2F:

* Enable U2F in Teleport configuration `/etc/teleport.yaml` .

* For CLI-based logins you have to install [u2f-host](https://developers.yubico.com/libu2f-host/) utility.

* For web-based logins you have to use Google Chrome and Firefox 67 or greater, are the only
   supported U2F browsers at this time.

``` yaml
# snippet from /etc/teleport.yaml to show an example configuration of U2F:
auth_service:
  authentication:
    type: local
    second_factor: u2f
    # this section is needed only if second_factor is set to 'u2f'
    u2f:
       # app_id must point to the URL of the Teleport Web UI (proxy) accessible
       # by the end users
       app_id: https://localhost:3080
       # facets must list all proxy servers if there are more than one deployed
       facets:
       - https://localhost:3080
```

For single-proxy setups, the `app_id` setting can be equal to the domain name of
the proxy, but this will prevent you from adding more proxies without changing
the `app_id` . For multi-proxy setups, the `app_id` should be an HTTPS URL
pointing to a JSON file that mirrors `facets` in the auth config.

!!! warning "Warning"

    The `app_id` must never change in the lifetime of the
    cluster. If the App ID changes, all existing U2F key registrations will
    become invalid and all users who use U2F as the second factor will need to
    re-register. When adding a new proxy server, make sure to add it to the list
    of "facets" in the configuration file, but also to the JSON file referenced
    by `app_id`

**Logging in with U2F**

For logging in via the CLI, you must first install
[u2f-host](https://developers.yubico.com/libu2f-host/). Installing:

``` bash
# OSX:
$ brew install libu2f-host

# Ubuntu 16.04 LTS:
$ apt-get install u2f-host
```

Then invoke `tsh ssh` as usual to authenticate:

``` bash
$ tsh --proxy <proxy-addr> ssh <hostname>
```

!!! tip "Version Warning"

    External user identities are only supported in [Teleport Enterprise](enterprise/introduction.md).

    Please reach out to [sales@gravitational.com](mailto:sales@gravitational.com) for more information.

## Adding and Deleting Users

This section covers internal user identities, i.e. user accounts created and
stored in Teleport's internal storage. Most production users of Teleport use
_external_ users via [Github](#github-oauth-20) or [Okta](enterprise/sso/ssh-okta.md) or any other
SSO provider (Teleport Enterprise supports any SAML or OIDC compliant identity
provider).

A user identity in Teleport exists in the scope of a cluster. The member nodes
of a cluster have multiple OS users on them. A Teleport administrator creates
Teleport user accounts and maps them to the allowed OS user logins they can use.

Let's look at this table:

|Teleport User | Allowed OS Logins | Description
|------------------|---------------|-----------------------------
|joe    | joe, root | Teleport user 'joe' can login into member nodes as OS user 'joe' or 'root'
|bob    | bob      | Teleport user 'bob' can login into member nodes only as OS user 'bob'
|ross   |          | If no OS login is specified, it defaults to the same name as the Teleport user - 'ross'.

To add a new user to Teleport, you have to use the [ `tctl` ](cli-docs.md#tctl)
tool on the same node where the auth server is running, i.e.
[ `teleport` ](cli-docs.md#teleport) was started with `--roles=auth` .

``` bash
$ tctl users add joe joe,root
```

Teleport generates an auto-expiring token (with a TTL of 1 hour) and prints the
token URL which must be used before the TTL expires.

``` bash
Signup token has been created. Share this URL with the user:
https://<proxy>:3080/web/newuser/xxxxxxxxxxxx

NOTE: make sure the <proxy> host is accessible.
```

The user completes registration by visiting this URL in their web browser,
picking a password and configuring the 2nd factor authentication. If the
credentials are correct, the auth server generates and signs a new certificate
and the client stores this key and will use it for subsequent logins. The key
will automatically expire after 12 hours by default after which the user will
need to log back in with her credentials. This TTL can be configured to a
different value. Once authenticated, the account will become visible via `tctl`
:

``` bash
$ tctl users ls

User           Allowed Logins
----           --------------
admin          admin,root
ross           ross
joe            joe,root
```

Joe would then use the `tsh` client tool to log in to member node "luna" via
bastion "work" _as root_:

``` bash
$ tsh --proxy=work --user=joe root@luna
```

To delete this user:

``` bash
$ tctl users rm joe
```

## Editing Users

Users entries can be manipulated using the generic [resource
commands](#resources) via [ `tctl` ](cli-docs.md#tctl) . For example, to see the
full list of user records, an administrator can execute:

``` bash
$ tctl get users
```

To edit the user "joe":

``` bash
# dump the user definition into a file:
$ tctl get user/joe > joe.yaml
# ... edit the contents of joe.yaml

# update the user record:
$ tctl create -f joe.yaml
```

Some fields in the user record are reserved for internal use. Some of them will
be finalized and documented in the future versions. Fields like `is_locked` or
`traits/logins` can be used starting in version 2.3

## Adding Nodes to the Cluster

Teleport is a "clustered" system, meaning it only allows access to nodes
(servers) that had been previously granted cluster membership.

A cluster membership means that a node receives its own host certificate signed
by the cluster's auth server. To receive a host certificate upon joining a
cluster, a new Teleport host must present an "invite token". An invite token
also defines which role a new host can assume within a cluster: `auth` , `proxy`
or `node` .

There are two ways to create invitation tokens:

* **Static Tokens** are easy to use and somewhat less secure.
* **Dynamic Tokens** are more secure but require more planning.

### Static Tokens

Static tokens are defined ahead of time by an administrator and stored in the
auth server's config file:

``` yaml
# Config section in `/etc/teleport.yaml` file for the auth server
auth_service:
    enabled: true
    tokens:
    # This static token allows new hosts to join the cluster as "proxy" or "node"
    - "proxy,node:secret-token-value"
    # A token can also be stored in a file. In this example the token for adding
    # new auth servers is stored in /path/to/tokenfile
    - "auth:/path/to/tokenfile"
```

### Short-lived Tokens

A more secure way to add nodes to a cluster is to generate tokens as they are
needed. Such token can be used multiple times until its time to live (TTL)
expires.

Use the [ `tctl` ](cli-docs.md#tctl) tool to register a new invitation token (or
it can also generate a new token for you). In the following example a new token
is created with a TTL of 5 minutes:

``` bash
$ tctl nodes add --ttl=5m --roles=node,proxy --token=secret-value
The invite token: secret-value
```

If `--token` is not provided, [ `tctl` ](cli-docs.md#tctl) will generate one:

``` bash
# generate a short-lived invitation token for a new node:
$ tctl nodes add --ttl=5m --roles=node,proxy
The invite token: e94d68a8a1e5821dbd79d03a960644f0

# you can also list all generated non-expired tokens:
$ tctl tokens ls
Token                            Type            Expiry Time
---------------                  -----------     ---------------
e94d68a8a1e5821dbd79d03a960644f0 Node            25 Sep 18 00:21 UTC

# ... or revoke an invitation before it's used:
$ tctl tokens rm e94d68a8a1e5821dbd79d03a960644f0
```

### Using Node Invitation Tokens

Both static and short-lived tokens are used the same way. Execute the following
command on a new node to add it to a cluster:

``` bash
# adding a new regular SSH node to the cluster:
$ teleport start --roles=node --token=secret-token-value --auth-server=10.0.10.5

# adding a new regular SSH node using Teleport Node Tunneling:
$ teleport start --roles=node --token=secret-token-value --auth-server=teleport-proxy.example.com:3080

# adding a new proxy service on the cluster:
$ teleport start --roles=proxy --token=secret-token-value --auth-server=10.0.10.5
```

As new nodes come online, they start sending ping requests every few seconds to
the CA of the cluster. This allows users to explore cluster membership and size:

``` bash
$ tctl nodes ls

Node Name     Node ID                                  Address            Labels
---------     -------                                  -------            ------
turing        d52527f9-b260-41d0-bb5a-e23b0cfe0f8f     10.1.0.5:3022      distro:ubuntu
dijkstra      c9s93fd9-3333-91d3-9999-c9s93fd98f43     10.1.0.6:3022      distro:debian
```

### Untrusted Auth Servers

Teleport nodes use the HTTPS protocol to offer the join tokens to the auth
server running on `10.0.10.5` in the example above. In a zero-trust environment,
you must assume that an attacker can hijack the IP address of the auth server
e.g. `10.0.10.5` .

To prevent this from happening, you need to supply every new node with an
additional bit of information about the auth server. This technique is called
"CA Pinning". It works by asking the auth server to produce a "CA Pin", which
is a hashed value of its public key, i.e. for which an attacker can't forge a
matching private key.

On the auth server:

``` bash
$ tctl status
Cluster  staging.example.com
User CA  never updated
Host CA  never updated
CA pin   sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1
```

The "CA pin" at the bottom needs to be passed to the new nodes when they're
starting for the first time, i.e. when they join a cluster:

Via CLI:

``` bash
$ teleport start \
   --roles=node \
   --token=1ac590d36493acdaa2387bc1c492db1a \
   --ca-pin=sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1 \
   --auth-server=10.12.0.6:3025
```

or via `/etc/teleport.yaml` on a node:

``` yaml
teleport:
  auth_token: "1ac590d36493acdaa2387bc1c492db1a"
  ca_pin: "sha256:7e12c17c20d9cb504bbcb3f0236be3f446861f1396dcbb44425fe28ec1c108f1"
  auth_servers:
    - "10.12.0.6:3025"
```

!!! warning "Warning"

    If a CA pin is not provided, Teleport node will join a
    cluster but it will print a `WARN` message (warning) into its standard
    error output.

!!! warning "Warning"

    The CA pin becomes invalid if a Teleport administrator
    performs the CA rotation by executing
    [ `tctl auth rotate` ](cli-docs.md#tctl-auth-rotate) .

## Revoking Invitations

As you have seen above, Teleport uses tokens to invite users to a cluster
(sign-up tokens) or to add new nodes to it (provisioning tokens).

Both types of tokens can be revoked before they can be used. To see a list of
outstanding tokens, run this command:

``` bash
$ tctl tokens ls

Token                                Role       Expiry Time (UTC)
-----                                ----       -----------------
eoKoh0caiw6weoGupahgh6Wuo7jaTee2     Proxy      never
696c0471453e75882ff70a761c1a8bfa     Node       17 May 16 03:51 UTC
6fc5545ab78c2ea978caabef9dbd08a5     Signup     17 May 16 04:24 UTC
```

In this example, the first token has a "never" expiry date because it is a
static token configured via a config file.

The 2nd token with "Node" role was generated to invite a new node to this
cluster. And the 3rd token was generated to invite a new user.

The latter two tokens can be deleted (revoked) via [`tctl tokens
del`](cli-docs.md#tctl-tokens-rm) command:

``` bash
$ tctl tokens del 696c0471453e75882ff70a761c1a8bfa
Token 696c0471453e75882ff70a761c1a8bfa has been deleted
```

## Adding a node located behind NAT

!!! note
    This feature is sometimes called "Teleport IoT" or node tunneling.

With the current setup, you've only been able to add nodes that have direct access to the
auth server and within the internal IP range of the cluster. We recommend
setting up a [Trusted Cluster](trustedclusters.md) if you have workloads split
across different networks/clouds.

Teleport Node Tunneling lets you add a remote node to an existing Teleport Cluster via tunnel.
This can be useful for IoT applications, or for managing a couple of servers in a different network.

Similar to [Adding Nodes to the Cluster](#adding-nodes-to-the-cluster), use `tctl` to
create a single-use token for a node, but this time you'll replace the auth
server IP with the URL of the proxy server. In the example below, we've
replaced the auth server IP with the proxy web endpoint `teleport-proxy.example.com:3080`.

``` bash
$ sudo tctl nodes add

The invite token: n92bb958ce97f761da978d08c35c54a5c
Run this on the new node to join the cluster:
teleport start --roles=node --token=n92bb958ce97f761da978d08c35c54a5c --auth-server=teleport-proxy.example.com:3080
```

Using the ports in the default configuration, the node needs to be able to talk to ports 3080
and 3024 on the proxy. Port 3080 is used to initially fetch the credentials (SSH and TLS certificates)
and for discovery (where is the reverse tunnel running, in this case 3024). Port 3024 is used to
establish a connection to the auth server through the proxy.

To enable multiplexing so only one port is used, simply set the `tunnel_listen_addr` the same as the
`web_listen_addr` respectively within the `proxy_service`.  Teleport will automatically recognize using the same port and enable multiplexing. If the log setting is set to DEBUG you will see multiplexing enabled in the server log.

```bash
DEBU [PROC:1]    Setup Proxy: Reverse tunnel proxy and web proxy listen on the same port, multiplexing is on. service/service.go:1944
```

!!! tip "Load Balancers"

    The setup above also works even if the cluster uses multiple proxies behind
    a load balancer (LB) or a DNS entry with multiple values.  This works by
    the node establishing a tunnel to _every_ proxy. This requires that an LB
    uses round-robin or a similar balancing algorithm. Do not use sticky load
    balancing algorithms (a.k.a. "session affinity") with Teleport proxies.

## Labeling Nodes

In addition to specifying a custom nodename, Teleport also allows for the
application of arbitrary key:value pairs to each node, called labels. There are
two kinds of labels:

1. `static labels` do not change over time, while [ `teleport` ](cli-docs.md#teleport)
    process is running.  Examples of static labels are physical location of nodes,
    name of the environment (staging vs production), etc.

2. `dynamic labels` also known as "label commands" allow to generate labels at
   runtime. Teleport will execute an external command on a node at a configurable
   frequency and the output of a command becomes the label value. Examples include
    reporting load averages, presence of a process, time after last reboot, etc.

There are two ways to configure node labels.

1. Via command line, by using `--labels` flag to `teleport start` command.
2. Using `/etc/teleport.yaml` configuration file on the nodes.

To define labels as command line arguments, use `--labels` flag like shown
below. This method works well for static labels or simple commands:

``` bash
$ teleport start --labels uptime=[1m:"uptime -p"],kernel=[1h:"uname -r"]
```

Alternatively, you can update `labels` via a configuration file:

``` yaml
ssh_service:
  enabled: "yes"
  # Static labels are simple key/value pairs:
  labels:
    environment: test
```

To configure dynamic labels via a configuration file, define a `commands` array
as shown below:

``` yaml
ssh_service:
  enabled: "yes"
  # Dynamic labels AKA "commands":
  commands:
  - name: hostname
    command: [hostname]
    period: 1m0s
  - name: arch
    command: [uname, -p]
    # this setting tells teleport to execute the command above
    # once an hour. this value cannot be less than one minute.
    period: 1h0m0s
```

`/path/to/executable` must be a valid executable command (i.e. executable bit
must be set) which also includes shell scripts with a proper [shebang
line](https://en.wikipedia.org/wiki/Shebang_(Unix)).

**Important:** notice that `command` setting is an array where the first element
is a valid executable and each subsequent element is an argument, i.e:

``` yaml
# valid syntax:
command: ["/bin/uname", "-m"]

# INVALID syntax:
command: ["/bin/uname -m"]

# if you want to pipe several bash commands together, here's how to do it:
# notice how ' and " are interchangeable and you can use it for quoting:
command: ["/bin/sh", "-c", "uname -a | egrep -o '[0-9]+\\.[0-9]+\\.[0-9]+'"]
```

## Audit Log

Teleport logs every SSH event into its audit log. There are two components of
the audit log:

1. **SSH Events:** Teleport logs events like successful user logins along with
   the metadata like remote IP address, time and the session ID.

2. **Recorded Sessions:** Every SSH shell session is recorded and can be
   replayed later. The recording is done by the nodes themselves, by default,
   but can be configured to be done by the proxy.

3. **Optional: [Enhanced Session Recording](features/enhanced-session-recording.md)**

Refer to the ["Audit Log" chapter in the Teleport
Architecture](architecture/authentication.md#audit-log) to learn more about how the audit log and
session recording are designed.

### SSH Events

Teleport supports multiple storage back-ends for storing the SSH events. The
section below uses the `dir` backend as an example. `dir` backend uses the local
filesystem of an auth server using the configurable `data_dir` directory.

For highly available (HA) configurations, users can refer to our
[DynamoDB](#using-dynamodb) or [Firestore](#using-firestore) chapters for information
on how to configure the SSH events and recorded sessions to be stored on
network storage. It is even possible to store the audit log in multiple places at the
same time - see `audit_events_uri` setting in the sample configuration file above for
how to do that.

Let's examine the Teleport audit log using the `dir` backend. The event log is
stored in `data_dir` under `log` directory, usually `/var/lib/teleport/log` .
Each day is represented as a file:

``` bash
$ ls -l /var/lib/teleport/log/
total 104
-rw-r----- 1 root root  31638 Jan 22 20:00 2017-01-23.00:00:00.log
-rw-r----- 1 root root  91256 Jan 31 21:00 2017-02-01.00:00:00.log
-rw-r----- 1 root root  15815 Feb 32 22:54 2017-02-03.00:00:00.log
```

The log files use JSON format. They are human-readable but can also be
programmatically parsed. Each line represents an event and has the following
format:

``` js
{
    // Event type. See below for the list of all possible event types
    "event": "session.start",
    // uid: A unique ID for the event log. Useful for  deduplication.
    "uid": "59cf8d1b-7b36-4894-8e90-9d9713b6b9ef",
    // Teleport user name
    "user": "ekontsevoy",
    // OS login
    "login": "root",
    // Server namespace. This field is reserved for future use.
    "namespace": "default",
    // Unique server ID.
    "server_id": "f84f7386-5e22-45ff-8f7d-b8079742e63f",
    // Server Labels.
    "server_labels": {
      "datacenter": "us-east-1",
      "label-b": "x"
    }
    // Session ID. Can be used to replay the session.
    "sid": "8d3895b6-e9dd-11e6-94de-40167e68e931",
    // Address of the SSH node
    "addr.local": "10.5.l.15:3022",
    // Address of the connecting client (user)
    "addr.remote": "73.223.221.14:42146",
    // Terminal size
    "size": "80:25",
    // Timestamp
    "time": "2017-02-03T06:54:05Z"
}
```

The possible event types are:

| Event Type    | Description                                                                                                                                                         |
|---------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| auth          | Authentication attempt. Adds the following fields: `{"success": "false", "error": "access denied"}` |
| session.start | Started an interactive shell session.|
| session.end   | An interactive shell session has ended.|
| session.join  | A new user has joined the existing interactive shell session.|
| session.leave | A user has left the session.|
| session.disk  | A list of files opened during the session. *Requires Enhanced Session Recording*. |
| session.network | A list of network connections made during the session.  *Requires Enhanced Session Recording*. |
| session.command | A list of commands ran during the session.  *Requires Enhanced Session Recording*. |
| exec          | Remote command has been executed via SSH, like `tsh ssh root@node ls /` . The following fields will be logged: `{"command": "ls /", "exitCode": 0, "exitError": ""}` |
| scp           | Remote file copy has been executed. The following fields will be logged: `{"path": "/path/to/file.txt", "len": 32344, "action": "read" }` |
| resize        | Terminal has been resized.|
| user.login    | A user logged into web UI or via tsh. The following fields will be logged: `{"user": "alice@example.com", "method": "local"}` .|


### Recorded Sessions

In addition to logging `session.start` and `session.end` events, Teleport also
records the entire stream of bytes going to/from standard input and standard
output of an SSH session.

Teleport can store the recorded sessions in an [AWS S3 bucket](#using-dynamodb)
or in a local filesystem (including NFS).

The recorded sessions are stored as raw bytes in the `sessions` directory under
`log` . Each session consists of two files, both are named after the session ID:

1. `.bytes` file or `.chunks.gz` compressed format represents the raw session bytes and is somewhat
    human-readable, although you are better off using [`tsh
    play`](cli-docs.md#tsh-play) or the Web UI to replay it.

2. `.log` file or `.events.gz` compressed file contains the copies of the event log entries that are            related to this session.

``` bash
$ ls /var/lib/teleport/log/sessions/default
-rw-r----- 1 root root 506192 Feb 4 00:46 4c146ec8-eab6-11e6-b1b3-40167e68e931.session.bytes
-rw-r----- 1 root root  44943 Feb 4 00:46 4c146ec8-eab6-11e6-b1b3-40167e68e931.session.log
```

To replay this session via CLI:

``` bash
$ tsh --proxy=proxy play 4c146ec8-eab6-11e6-b1b3-40167e68e931
```

## Resources

A Teleport administrator has two tools to configure a Teleport cluster:

* The [configuration file](#configuration) is used for static configuration like
  the cluster name.

* The [ `tctl` ](cli-docs.md#tctl) admin tool is used for manipulating dynamic
  records like Teleport
  users.

[ `tctl` ](cli-docs.md#tctl) has convenient subcommands for dynamic
configuration, like `tctl users` or `tctl nodes` . However, for dealing with
more advanced topics, like connecting clusters together or troubleshooting
trust, [ `tctl` ](cli-docs.md#tctl) offers the more powerful, although
lower-level CLI interface called `resources` .

The concept is borrowed from the REST programming pattern. A cluster is composed
of different objects (aka, resources) and there are just three common operations
that can be performed on them: `get` , `create` , `remove` .

A resource is defined as a [YAML](https://en.wikipedia.org/wiki/YAML) file.
Every resource in Teleport has three required fields:

* `Kind` - The type of resource
* `Name` - A required field in the `metadata` to uniquely identify the resource
* `Version` - The version of the resource format

Everything else is resource-specific and any component of a Teleport cluster can
be manipulated with just 3 CLI commands:

| Command       | Description                                                           | Examples                                |
|---------------|-----------------------------------------------------------------------|-----------------------------------------|
| [ `tctl get` ](cli-docs.md#tctl-get) | Get one or multiple resources                                         | `tctl get users` or `tctl get user/joe` |
| [ `tctl rm` ](cli-docs.md#tctl-rm) | Delete a resource by type/name                                        | `tctl rm user/joe` |
| [ `tctl create` ](cli-docs.md#tctl-create) | Create a new resource from a YAML file. Use `-f` to override / update | `tctl create -f joe.yaml` |

!!! warning "YAML Format"

    By default Teleport uses [YAML format](https://en.wikipedia.org/wiki/YAML)
    to describe resources. YAML is a
    wonderful and very human-readable alternative to JSON or XML, but it's
    sensitive to white space. Pay attention to spaces vs tabs!

Here's an example how the YAML resource definition for a user Joe might look
like. It can be retrieved by executing [`tctl get
user/joe`](cli-docs.md#tctl-get)

``` yaml
kind: user
version: v2
metadata:
  name: joe
spec:
  roles: admin
  status:
    # users can be temporarily locked in a Teleport system, but this
    # functionality is reserved for internal use for now.
    is_locked: false
    lock_expires: 0001-01-01T00:00:00Z
    locked_time: 0001-01-01T00:00:00Z
  traits:
    # these are "allowed logins" which are usually specified as the
    # last argument to `tctl users add`
    logins:
    - joe
    - root
  # any resource in Teleport can automatically expire.
  expires: 0001-01-01T00:00:00Z
  # for internal use only
  created_by:
    time: 0001-01-01T00:00:00Z
    user:
      name: builtin-Admin
```

!!! tip "Note"

    Some of the fields you will see when printing resources are used
    only internally and are not meant to be changed.  Others are reserved for
    future use.

Here's the list of resources currently exposed via [ `tctl` ](cli-docs.md#tctl) :

| Resource Kind | Description                                                                                                                                  |
|---------------|----------------------------------------------------------------------------------------------------------------------------------------------|
| user          | A user record in the internal Teleport user DB.|
| node          | A registered SSH node. The same record is displayed via `tctl nodes ls` |
| cluster       | A trusted cluster. See [here](#trusted-clusters) for more details on connecting clusters together.|
| role          | A role assumed by users. The open source Teleport only includes one role: "admin", but Enterprise teleport users can define their own roles.|
| connector     | Authentication connectors for [single sign-on](enterprise/sso/ssh-sso.md) (SSO) for SAML, OIDC and Github.|

**Examples:**

```bash
# list all connectors:
$ tctl get connectors

# dump a SAML connector called "okta":
$ tctl get saml/okta

# delete a SAML connector called "okta":
$ tctl rm saml/okta

# delete an OIDC connector called "gsuite":
$ tctl rm oidc/gsuite

# delete a github connector called "myteam":
$ tctl rm github/myteam

# delete a local user called "admin":
$ tctl rm users/admin
```

!!! note
    Although `tctl get connectors` will show you every connector, when working with an individual
    connector you must use the correct `kind`, such as `saml` or `oidc`. You can see each
    connector's `kind` at the top of its YAML output from `tctl get connectors`.

## Trusted Clusters

As explained in the [architecture document](architecture/overview.md#design-principles),
Teleport can partition compute infrastructure into multiple clusters. A cluster
is a group of nodes connected to the cluster's auth server, acting as a
certificate authority (CA) for all users and nodes.

To retrieve an SSH certificate, users must authenticate with a cluster through a
proxy server. So, if users want to connect to nodes belonging to different
clusters, they would normally have to use a different `--proxy` flag for each
cluster. This is not always convenient.

The concept of trusted clusters allows Teleport administrators to connect
multiple clusters together and establish trust between them. Trusted clusters
allow users of one cluster to seamlessly SSH into the nodes of another cluster
without having to "hop" between proxy servers. Moreover, users don't even need
to have a direct connection to other clusters' proxy servers. Trusted clusters
also have their own restrictions on user access.

To learn more about Trusted Clusters please visit our [Trusted Cluster Guide](trustedclusters.md)

## Github OAuth 2.0

Teleport supports authentication and authorization via external identity
providers such as Github. You can watch the video for how to configure
[Github as an SSO provider](https://gravitational.com/resources/guides/github-sso-provider-kubernetes-ssh/),
or you can follow the documentation below.

First, the Teleport auth service must be configured to use Github for
authentication:

``` yaml
# snippet from /etc/teleport.yaml
auth_service:
  authentication:
      type: github
```

Next step is to define a Github connector:

``` yaml
# Create a file called github.yaml:
kind: github
version: v3
metadata:
  # connector name that will be used with `tsh --auth=github login`
  name: github
spec:
  # client ID of Github OAuth app
  client_id: <client-id>
  # client secret of Github OAuth app
  client_secret: <client-secret>
  # connector display name that will be shown on web UI login screen
  display: Github
  # callback URL that will be called after successful authentication
  redirect_url: https://<proxy-address>/v1/webapi/github/callback
  # mapping of org/team memberships onto allowed logins and roles
  teams_to_logins:
    - organization: octocats # Github organization name
      team: admins # Github team name within that organization
      # allowed logins for users in this org/team
      logins:
        - root
      # List of Kubernetes groups this Github team is allowed to connect to
      # (see Kubernetes integration for more information)
      kubernetes_groups: ["system:masters"]
```

!!! note

    For open-source Teleport the `logins` field contains a list of allowed
    OS logins. For the commercial Teleport Enterprise offering, which supports
    role-based access control, the same field is treated as a list of _roles_
    that users from the matching org/team assume after going through the
    authorization flow.

To obtain client ID and client secret, please follow Github documentation on
how to [create and register an OAuth
app](https://developer.github.com/apps/building-oauth-apps/creating-an-oauth-app/).
Be sure to set the "Authorization callback URL" to the same value as
`redirect_url` in the resource spec. Teleport will request only the `read:org`
OAuth scope, you can read more about [Github OAuth scopes](https://developer.github.com/apps/building-oauth-apps/understanding-scopes-for-oauth-apps/).

Finally, create the connector using [ `tctl` ](cli-docs.md#tctl)
[resource](#resources) management command:

``` bash
$ tctl create github.yaml
```

!!! tip

    When going through the Github authentication flow for the first time,
    the application must be granted the access to all organizations that are
    present in the "teams to logins" mapping, otherwise Teleport will not be
    able to determine team memberships for these orgs.

## HTTP CONNECT Proxies

Some networks funnel all connections through a proxy server where they can be
audited and access control rules are applied. For these scenarios Teleport
supports HTTP CONNECT tunneling.

To use HTTP CONNECT tunneling, simply set either the `HTTPS_PROXY` or
`HTTP_PROXY` environment variables and when Teleport builds and establishes the
reverse tunnel to the main cluster, it will funnel all traffic though the proxy.
Specifically, if using the default configuration, Teleport will tunnel ports
`3024` (SSH, reverse tunnel) and `3080` (HTTPS, establishing trust) through the
proxy.

The value of `HTTPS_PROXY` or `HTTP_PROXY` should be in the format
`scheme://host:port` where scheme is either `https` or `http` . If the value is
`host:port` , Teleport will prepend `http` .

It's important to note that in order for Teleport to use HTTP CONNECT
tunnelling, the `HTTP_PROXY` and `HTTPS_PROXY` environment variables must be set
within Teleport's environment. You can also optionally set the `NO_PROXY`
environment variable to avoid use of the proxy when accessing specified
hosts/netmasks. When launching Teleport with systemd, this will probably involve
adding some lines to your systemd unit file:

```
[Service]
Environment="HTTP_PROXY=http://proxy.example.com:8080/"
Environment="HTTPS_PROXY=http://proxy.example.com:8080/"
Environment="NO_PROXY=localhost,127.0.0.1,192.168.0.0/16,172.16.0.0/12,10.0.0.0/8"
```

!!! tip "Note"

    `localhost` and `127.0.0.1` are invalid values for the proxy
    host. If for some reason your proxy runs locally, you'll need to provide
    some other DNS name or a private IP address for it.

## PAM Integration

Teleport node service can be configured to integrate with
[PAM](https://en.wikipedia.org/wiki/Linux_PAM). This allows Teleport to create
user sessions using PAM session profiles.

To enable PAM on a given Linux machine, update `/etc/teleport.yaml` with:

```yaml
teleport:
   ssh_service:
      pam:
         # "no" by default
         enabled: yes
         # use /etc/pam.d/sshd configuration (the default)
         service_name: "sshd"
```

Please note that most Linux distributions come with a number of PAM services in
`/etc/pam.d` and Teleport will try to use `sshd` by default, which will be
removed if you uninstall `openssh-server` package. We recommend creating your
own PAM service file like `/etc/pam.d/teleport` and specifying it as
`service_name` above.

!!! tip "Note"

    Teleport only supports the `account` and `session` stack. The `auth` PAM module is currently not supported with Teleport.

## Using Teleport with OpenSSH

Review our dedicated [Using Teleport with OpenSSH](openssh-teleport.md) guide.

## Certificate Rotation

Take a look at the [Certificates chapter](architecture/authentication.md#authentication-in-teleport) in the
architecture document to learn how the certificate rotation works. This section
will show you how to implement certificate rotation in practice.

The easiest way to start the rotation is to execute this command on a cluster's
_auth server_:

``` bash
$ tctl auth rotate
```

This will trigger a rotation process for both hosts and users with a _grace
period_ of 48 hours.

This can be customized, i.e.

``` bash
# rotate only user certificates with a grace period of 200 hours:
$ tctl auth rotate --type=user --grace-period=200h

# rotate only host certificates with a grace period of 8 hours:
$ tctl auth rotate --type=host --grace-period=8h
```

The rotation takes time, especially for hosts, because each node in a cluster
needs to be notified that a rotation is taking place and request a new
certificate for itself before the grace period ends.

!!! warning "Warning"

    Be careful when choosing a grace period when rotating
    host certificates. The grace period needs to be long enough for all nodes in
    a cluster to request a new certificate. If some nodes go offline during the
    rotation and come back only after the grace period has ended, they will be
    forced to leave the cluster, i.e. users will no longer be allowed to SSH
    into them.

To check the status of certificate rotation:

``` bash
$ tctl status
```

!!! warning "CA Pinning Warning"

    If you are using [CA Pinning](#untrusted-auth-servers) when adding new
    nodes, the CA pin will changes after the rotation. Make sure you use the
    _new_ CA pin when adding nodes after rotation.

## Ansible Integration

Ansible uses the OpenSSH client by default. This makes it compatible with
Teleport without any extra work, except configuring OpenSSH client to work with
Teleport Proxy:

* configure your OpenSSH to connect to Teleport proxy and use `ssh-agent` socket
* enable scp mode in the Ansible config file (default is `/etc/ansible/ansible.cfg` ):

```bash
scp_if_ssh = True
```

## Kubernetes Integration

Teleport can be configured as a compliance gateway for Kubernetes clusters.
This allows users to authenticate against a Teleport proxy using [`tsh
login`](cli-docs.md#tsh) command to retrieve credentials for both SSH and
Kubernetes API.

Follow our [Kubernetes guide](kubernetes-ssh.md) which contains some more specific
examples and instructions.

## High Availability

!!! tip "Tip"

    Before continuing, please make sure to take a look at the
    [Cluster State section](architecture/nodes.md#cluster-state) in the Teleport
    Architecture documentation.

Usually there are two ways to achieve high availability. You can "outsource"
this function to the infrastructure. For example, using a highly available
network-based disk volumes (similar to AWS EBS) and by migrating a failed VM to
a new host. In this scenario, there's nothing Teleport-specific to be done.

If high availability cannot be provided by the infrastructure (perhaps you're
running Teleport on a bare metal cluster), you can still configure Teleport to
run in a highly available fashion.

### Auth Server HA

In order to run multiple instances of Teleport Auth Server, you must switch to a
highly available secrets back-end first. Also, you must tell each node in a
cluster that there is more than one auth server available. There are two ways to
do this:

  * Use a load balancer to create a single auth API access point (AP) and
    specify this AP in `auth_servers` section of Teleport configuration for all
    nodes in a cluster. This load balancer should do TCP level forwarding.

  + If a load balancer is not an option, you must specify each instance of an
    auth server in `auth_servers` section of Teleport configuration.

**IMPORTANT:** with multiple instances of the auth servers running, special
attention needs to be paid to keeping their configuration identical. Settings
like `cluster_name` , `tokens` , `storage` , etc must be the same.

### Teleport Proxy HA

The Teleport Proxy is stateless which makes running multiple instances trivial.
If using the [default configuration](#ports), configure your load balancer to
forward ports `3023` and `3080` to the servers that run the Teleport proxy. If
you have configured your proxy to use non-default ports, you will need to
configure your load balancer to forward the ports you specified for
`listen_addr` and `web_listen_addr` in `teleport.yaml` . The load balancer for
`web_listen_addr` can terminate TLS with your own certificate that is valid for
your users, while the remaining ports should do TCP level forwarding, since
Teleport will handle its own SSL on top of that with its own certificates.

!!! tip "NOTE"

    If you terminate TLS with your own certificate at a load
    balancer you'll need to run Teleport with `--insecure-no-tls`

If your load balancer supports HTTP health checks, configure it to hit the
`/readyz` [diagnostics endpoint](metrics-logs-reference.md) on machines running Teleport. This endpoint
must be enabled by using the `--diag-addr` flag to teleport start: `teleport start --diag-addr=127.0.0.1:3000`
The http://127.0.0.1:3000/readyz endpoint will reply `{"status":"ok"}` if the Teleport service
is running without problems.

!!! tip "NOTE"

    As the new auth servers get added to the cluster and the old
    servers get decommissioned, nodes and proxies will refresh the list of
    available auth servers and store it in their local cache
    `/var/lib/teleport/authservers.json` - the values from the cache file will take
    precedence over the configuration file.

We'll cover how to use `etcd`, DynamoDB and Firestore storage back-ends to make Teleport
highly available below.

### Teleport Scalability Tweaks

When running Teleport at scale (for example in the case where there are 10,000+ nodes connected
to a cluster via [node tunnelling mode](#adding-a-node-located-behind-nat), the following settings
should be set on Teleport auth and proxies:

#### Proxy Servers
These settings alter Teleport's [default connection limit](https://github.com/gravitational/teleport/blob/5cd212fecda63ec6790cc5ffe508a626c56e2b2c/lib/defaults/defaults.go#L385) from 15000 to 65000.

```yaml
# Teleport Proxy
teleport:
  cache:
    # use an in-memory cache to speed up the connection of many teleport nodes
    # back to proxy
    type: in-memory
  # set up connection limits to prevent throttling of many IoT nodes connecting to proxies
  connection_limits:
    max_connections: 65000
    max_users: 1000
```
#### Auth Servers

```yaml
# Teleport Auth
teleport:
  connection_limits:
    max_connections: 65000
    max_users: 1000
```

### Using etcd

Teleport can use [etcd](https://etcd.io/) as a storage backend to
achieve highly available deployments. You must take steps to protect access to
`etcd` in this configuration because that is where Teleport secrets like keys
and user records will be stored.

!!! warning "IMPORTANT"

    `etcd` can only currently be used to store Teleport's internal database in a highly-available
    way. This will allow you to have multiple auth servers in your cluster for an HA deployment,
    but it will not also store Teleport audit events for you in the same way that
    [DynamoDB](#using-dynamodb) or [Firestore](#using-firestore) will.

To configure Teleport for using etcd as a storage back-end:

* Make sure you are using **etcd version 3.3** or newer.
* Install etcd and configure peer and client TLS authentication using the [etcd
  security guide](https://etcd.io/docs/v3.4.0/op-guide/security/).
    * You can use [this script provided by
      etcd](https://github.com/etcd-io/etcd/tree/master/hack/tls-setup) if you
      don't already have a TLS setup.
* Configure all Teleport Auth servers to use etcd in the "storage" section of the config file as shown below.
* Deploy several auth servers connected to etcd back-end.
* Deploy several proxy nodes that have `auth_servers` pointed to list of auth
  servers to connect to.

``` yaml
teleport:
  storage:
     type: etcd

     # list of etcd peers to connect to:
     peers: ["https://172.17.0.1:4001", "https://172.17.0.2:4001"]

     # required path to TLS client certificate and key files to connect to etcd
     #
     # to create these, follow
     # https://coreos.com/os/docs/latest/generate-self-signed-certificates.html
     # or use the etcd-provided script
     # https://github.com/etcd-io/etcd/tree/master/hack/tls-setup
     tls_cert_file: /var/lib/teleport/etcd-cert.pem
     tls_key_file: /var/lib/teleport/etcd-key.pem

     # optional file with trusted CA authority
     # file to authenticate etcd nodes
     #
     # if you used the script above to generate the client TLS certificate,
     # this CA certificate should be one of the other generated files
     tls_ca_file: /var/lib/teleport/etcd-ca.pem

     # alternative password based authentication, if not using TLS client
     # certificate
     #
     # See https://etcd.io/docs/v3.4.0/op-guide/authentication/ for setting
     # up a new user
     username: username
     password_file: /mnt/secrets/etcd-pass

     # etcd key (location) where teleport will be storing its state under.
     # make sure it ends with a '/'!
     prefix: /teleport/

     # NOT RECOMMENDED: enables insecure etcd mode in which self-signed
     # certificate will be accepted
     insecure: false

     # Optionally sets the limit on the client message size.
     # This is usually used to increase the default which is 2MiB
     # (1.5MiB server's default + gRPC overhead bytes).
     # Make sure this does not exceed the value for the etcd
     # server specified with `--max-request-bytes` (1.5MiB by default).
     # Keep the two values in sync.
     #
     # See https://etcd.io/docs/v3.4.0/dev-guide/limit/ for details
     max_client_msg_size_bytes: 15728640 # 15MiB
```

### Using Amazon S3

!!! tip "Tip"

    Before continuing, please make sure to take a look at the
    [cluster state section](architecture/nodes.md#cluster-state) in Teleport
    Architecture documentation.

!!! tip "AWS Authentication"

    The configuration examples below contain AWS
    access keys and secret keys. They are optional, they exist for your
    convenience but we DO NOT RECOMMEND using them in production. If Teleport is
    running on an AWS instance it will automatically use the instance IAM role.
    Teleport also will pick up AWS credentials from the `~/.aws` folder, just
    like the AWS CLI tool.

S3 buckets can only be used as a storage for the recorded sessions. S3 cannot
store the audit log or the cluster state. Below is an example of how to
configure a Teleport auth server to store the recorded sessions in an S3 bucket.

``` yaml
teleport:
  storage:
      # The region setting sets the default AWS region for all AWS services
      # Teleport may consume (DynamoDB, S3)
      region: us-east-1

      # Path to S3 bucket to store the recorded sessions in.
      audit_sessions_uri: "s3://Example_TELEPORT_S3_BUCKET/records"

      # Teleport assumes credentials. Using provider chains, assuming IAM role or
      # standard .aws/credentials in the home folder.
```

The AWS authentication settings above can be omitted if the machine itself is
running on an EC2 instance with an IAM role.

### Using DynamoDB

!!! tip "Tip"

    Before continuing, please make sure to take a look at the
    [cluster state section](architecture/nodes.md#cluster-state) in Teleport Architecture documentation.

If you are running Teleport on AWS, you can use
[DynamoDB](https://aws.amazon.com/dynamodb/) as a storage back-end to achieve
high availability. DynamoDB back-end supports two types of Teleport data:

* Cluster state
* Audit log events

DynamoDB cannot store the recorded sessions. You are advised to use AWS S3 for
that as shown above. To configure Teleport to use DynamoDB:

* Make sure you have AWS access key and a secret key which give you access to
  DynamoDB account. If you're using (as recommended) an IAM role for this, the
  policy with necessary permissions is listed below.
* Configure all Teleport Auth servers to use DynamoDB back-end in the "storage"
  section of `teleport.yaml` as shown below.
* Deploy several auth servers connected to DynamoDB storage back-end.
* Deploy several proxy nodes.
* Make sure that all Teleport nodes have `auth_servers` configuration setting
  populated with the auth servers.

``` yaml
teleport:
  storage:
    type: dynamodb
    # Region location of dynamodb instance, https://docs.aws.amazon.com/en_pv/general/latest/gr/rande.html#ddb_region
    region: us-east-1

    # Name of the DynamoDB table. If it does not exist, Teleport will create it.
    table_name: Example_TELEPORT_DYNAMO_TABLE_NAME

    # This setting configures Teleport to send the audit events to three places:
    # To keep a copy in DynamoDB, a copy on a local filesystem, and also output the events to stdout.
    # NOTE: The DynamoDB events table has a different schema to the regular Teleport
    # database table, so attempting to use same table for both will result in errors.
    # When using highly available storage like DynamoDB, you should make sure that the list always specifies
    # the HA storage method first, as this is what the Teleport web UI uses as its source of events to display.
    audit_events_uri:  ['dynamodb://events_table_name', 'file:///var/lib/teleport/audit/events', 'stdout://']

    # This setting configures Teleport to save the recorded sessions in an S3 bucket:
    audit_sessions_uri: s3://Example_TELEPORT_S3_BUCKET/records
```

* Replace `us-east-1` and `Example_TELEPORT_DYNAMO_TABLE_NAME`
  with your own settings.  Teleport will create the table automatically.
* `Example_TELEPORT_DYNAMO_TABLE_NAME` and `events_table_name` **must** be different
  DynamoDB tables. The schema is different for each. Using the same table name for both
  will result in errors.
* The AWS authentication setting above can be omitted if the machine itself is
  running on an EC2 instance with an IAM role.
* Audit log settings above are optional. If specified, Teleport will store the
  audit log in DynamoDB and the session recordings **must** be stored in an S3
  bucket, i.e. both `audit_xxx` settings must be present. If they are not set,
  Teleport will default to a local file system for the audit log, i.e.
`/var/lib/teleport/log` on an auth server.
* If DynamoDB is used for the audit log, the logged events will be stored with a
  TTL of 1 year. Currently this TTL is not configurable.

!!! warning "Access to DynamoDB"

    Make sure that the IAM role assigned to
    Teleport is configured with the sufficient access to DynamoDB. Below is the
    example of the IAM policy you can use:

``` js
{
    "Version": "2012-10-17",
    "Statement": [{
            "Sid": "AllAPIActionsOnTeleportAuth",
            "Effect": "Allow",
            "Action": "dynamodb:*",
            "Resource": "arn:aws:dynamodb:eu-west-1:123456789012:table/prod.teleport.auth"
        },
        {
            "Sid": "AllAPIActionsOnTeleportStreams",
            "Effect": "Allow",
            "Action": "dynamodb:*",
            "Resource": "arn:aws:dynamodb:eu-west-1:123456789012:table/prod.teleport.auth/stream/*"
        }
    ]
}
```

### Using GCS

!!! tip "Tip"

    Before continuing, please make sure to take a look at the
    [cluster state section](architecture/nodes.md#cluster-state) in Teleport
    Architecture documentation.


Google Cloud Storage (GCS) can only be used as a storage for the recorded
sessions. GCS cannot store the audit log or the cluster state. Below is an
example of how to configure a Teleport auth server to store the recorded
sessions in a GCS bucket.

``` yaml
teleport:
  storage:
      # Path to GCS to store the recorded sessions in.
      audit_sessions_uri: "gs://Example_TELEPORT_STORAGE/records"
      credentials_path: /var/lib/teleport/gcs_creds
```


### Using Firestore

!!! tip "Tip"

    Before continuing, please make sure to take a look at the
    [cluster state section](architecture/nodes.md#cluster-state) in Teleport Architecture documentation.

If you are running Teleport on GCP, you can use
[Firestore](https://cloud.google.com/firestore/) as a storage back-end to achieve
high availability. Firestore back-end supports two types of Teleport data:

* Cluster state
* Audit log events

Firestore cannot store the recorded sessions. You are advised to use Google
Cloud Storage (GCS) for that as shown above. To configure Teleport to use
Firestore:

* Configure all Teleport Auth servers to use Firestore back-end in the "storage"
  section of `teleport.yaml` as shown below.
* Deploy several auth servers connected to Firestore storage back-end.
* Deploy several proxy nodes.
* Make sure that all Teleport nodes have `auth_servers` configuration setting
  populated with the auth servers or use a load balancer for the auth servers in
  high availability mode.

```yaml
teleport:
  storage:
    type: firestore
    # Project ID https://support.google.com/googleapi/answer/7014113?hl=en
    project_id: Example_GCP_Project_Name

    # Name of the Firestore table. If it does not exist, Teleport won't start
    collection_name: Example_TELEPORT_FIRESTORE_TABLE_NAME

    credentials_path: /var/lib/teleport/gcs_creds

    # This setting configures Teleport to send the audit events to three places:
    # To keep a copy in Firestore, a copy on a local filesystem, and also write the events to stdout.
    # NOTE: The Firestore events table has a different schema to the regular Teleport
    # database table, so attempting to use same table for both will result in errors.
    # When using highly available storage like Firestore, you should make sure that the list always specifies
    # the HA storage method first, as this is what the Teleport web UI uses as its source of events to display.
    audit_events_uri:  ['firestore://Example_TELEPORT_FIRESTORE_EVENTS_TABLE_NAME', 'file:///var/lib/teleport/audit/events', 'stdout://']

    # This setting configures Teleport to save the recorded sessions in GCP storage:
    audit_sessions_uri: gs://Example_TELEPORT_S3_BUCKET/records
```

* Replace `Example_GCP_Project_Name` and `Example_TELEPORT_FIRESTORE_TABLE_NAME`
  with your own settings. Teleport will create the table automatically.

* `Example_TELEPORT_FIRESTORE_TABLE_NAME` and `Example_TELEPORT_FIRESTORE_EVENTS_TABLE_NAME`
  **must** be different Firestore tables. The schema is different for each. Using the same
  table name for both will result in errors.

* The GCP authentication setting above can be omitted if the machine itself is
  running on a GCE instance with a Service Account that has access to the
  Firestore table.

* Audit log settings above are optional. If specified, Teleport will store the
  audit log in Firestore and the session recordings **must** be stored in a GCP
  bucket, i.e.both `audit_xxx` settings must be present. If they are not set,
  Teleport will default to a local file  system for the audit log, i.e.
  `/var/lib/teleport/log` on an auth server.


## Upgrading Teleport

Teleport is always a critical component of the infrastructure it runs on. This
is why upgrading to a new version must be performed with caution.

Teleport is a much more capable system than a bare bones SSH server. While it
offers significant benefits on a cluster level, it also adds some complexity to
cluster upgrades. To ensure robust operation Teleport administrators must follow
the upgrade rules listed below.

### Production Releases

First of all, avoid running pre-releases (release candidates) in production
environments. Teleport development team uses [Semantic
Versioning](https://semver.org/) which makes it easy to tell if a specific
version is recommended for production use.

### Component Compatibility

When running multiple binaries of Teleport within a cluster (nodes, proxies,
clients, etc), the following rules apply:

* Patch versions are always compatible, for example any 4.0.1 component will
  work with any 4.0.3 component.

* Other versions are always compatible with their **previous** release. This
  means you must not attempt to upgrade from 4.1 straight to 4.3. You must
  upgrade to 4.2 first.

* Teleport clients [`tsh`](cli-docs.md#tsh) for users and [`tctl`](cli-docs.md#tctl) for admins
  may not be compatible with different versions of the `teleport` service.

As an extra precaution you might want to backup your application prior to upgrading. We provide more instructions in [Backup before upgrading](#backup-before-upgrading).

!!! warning "Upgrading to Teleport 4.0+"

    Teleport 4.0+ switched to GRPC and HTTP/2 as an API protocol. The HTTP/2 spec bans
    two previously recommended ciphers. `tls-rsa-with-aes-128-gcm-sha256` & `tls-rsa-with-aes-256-gcm-sha384`, make sure these are removed from `teleport.yaml`
    [Visit our community for more details](https://community.gravitational.com/t/drop-ciphersuites-blacklisted-by-http-2-spec/446)

    If upgrading you might want to consider rotating CA to SHA-256 or SHA-512 for RSA
    SSH certificate signatures. The previous default was SHA-1, which is now considered
    weak against brute-force attacks. SHA-1 certificate signatures are also no longer
    accepted by OpenSSH versions 8.2 and above. All new Teleport clusters will default
    to SHA-512 based signatures. To upgrade an existing cluster, set the following in
    your teleport.yaml:

    ```bash
    teleport:
      ca_signature_algo: "rsa-sha2-512"
    ```

    After updating to 4.3+ rotate the cluster CA [following these docs](#certificate-rotation).

### Backup Before Upgrading

As an extra precaution you might want to backup your application prior to upgrading. We have more
instructions in [Backing up Teleport](#backing-up-teleport).

### Upgrade Sequence

When upgrading a single Teleport cluster:

1. **Upgrade the auth server first**. The auth server keeps the cluster state
    and if there are data format changes introduced in the new version this will
    perform necessary migrations.

2. Then, upgrade the proxy servers. The proxy servers are stateless and can be
   upgraded in any sequence or at the same time.

3. Finally, upgrade the SSH nodes in any sequence or at the same time.

!!! warning "Warning"

    If several auth servers are running in HA configuration
    (for example, in AWS auto-scaling group) you have to shrink the group to
    **just one auth server** prior to performing an upgrade. While Teleport
    will attempt to perform any necessary migrations, we recommend users
    create a backup of their backend before upgrading the Auth Server, as a
    precaution. This allows for a safe rollback in case the migration itself
    fails.

When upgrading multiple clusters:

1. First, upgrade the main cluster, i.e. the one which other clusters trust.
2. Upgrade the trusted clusters.

## Backing Up Teleport

When planning a backup of Teleport, it's important to know what is where and the
importance of each component. Teleport's Proxies and Nodes are stateless, and thus
only `teleport.yaml` should be backed up.

The Auth server is Teleport's brains, and depending on the backend should be backed up
regularly.

For example a customer running Teleport on AWS with DynamoDB have these key items of data:

| What | Where ( Example AWS Customer ) |
|-|-|
| Local Users ( not SSO )  | DynamoDB |
| Certificate Authorities | DynamoDB |
| Trusted Clusters | DynamoDB |
| Connectors: SSO | DynamoDB / File System  |
| RBAC | DynamoDB / File System |
| teleport.yaml | File System |
| teleport.service | File System |
| license.pem | File System |
| TLS key/certificate | ( File System / Outside Scope )  |
| Audit log | DynamoDB  |
| Session recordings| S3  |


For this customer, we would recommend using [AWS best practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/BackupRestore.html) for backing up DynamoDB. If DynamoDB is used for
the audit log, logged events have a TTL of 1 year.

| Backend | Recommended backup strategy  |
|-|-|
| dir ( local filesystem )   | Backup `/var/lib/teleport/storage` directory and the output of `tctl get all`. |
| DynamoDB | [Follow AWS Guidelines for Backup & Restore](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/BackupRestore.html) |
| etcd | [Follow etcD Guidleines for Disaster Recovery ](https://etcd.io/docs/v2/admin_guide) |
| Firestore | [Follow GCP Guidlines for Automated Backups](https://firebase.google.com/docs/database/backups) |

### Teleport Resources

Teleport uses YAML resources for roles, trusted clusters, local users and auth connectors.
These could be created via `tctl` or via the UI.

## GitOps

If running Teleport at scale, it's important for teams to have an automated way to
restore Teleport. At a high level, this is our recommended approach:

- Persist and backup your backend
- Share that backend among auth servers
- Store your configs as discrete files in VCS
- Have your CI run `tctl create -f *.yaml` from that git directory

## Migrating Backends.

As of version v4.1 you can now quickly export a collection of resources from
Teleport. This feature was designed to help customers migrate from local storage
to etcd.

Using `tctl get all` will retrieve the below items:

- Users
- Certificate Authorities
- Trusted Clusters
- Connectors:
    - Github
    - SAML [Teleport Enterprise]
    - OIDC [Teleport Enterprise]
    - Roles [Teleport Enterprise]

When migrating backends, you should back up your auth server's `data_dir/storage` directly.

**Example of backing up and restoring a cluster.**

``` bash
# export dynamic configuration state from old cluster
$ tctl get all > state.yaml

# prepare a new uninitialized backend (make sure to port
# any non-default config values from the old config file)
$ mkdir fresh && cat > fresh.yaml << EOF
teleport:
  data_dir: fresh
EOF

# bootstrap fresh server (kill the old one first!)
$ teleport start --config fresh.yaml --bootstrap state.yaml

# from another terminal, verify state transferred correctly
$ tctl --config fresh.yaml get all
# <your state here!>
```

The `--bootstrap` flag has no effect, except during backend initialization (performed
by auth server on first start), so it is safe for use in supervised/HA contexts.

**Limitations**

- All the same limitations around modifying the config file of an existing cluster also apply to a new cluster being bootstrapped from the state of an old cluster. Of particular note:
    - Changing cluster name will break your CAs (this will be caught and teleport will refuse to start).
    - Some user authentication mechanisms (e.g. u2f) require that the public endpoint of the web ui remains the same (this can't be caught by teleport, be careful!).
- Any node whose invite token is defined statically (in the config file of the auth server) will be able to join automatically, but nodes that were added dynamically will need to be re-invited


### Daemon Restarts

As covered in the [Graceful Restarts](#graceful-restarts) section, Teleport
supports graceful restarts. To upgrade a host to a newer Teleport version, an
administrator must:

1. Replace the Teleport binaries, usually [ `teleport` ](cli-docs.md#teleport)
   and [ `tctl` ](cli-docs.md#tctl)

2. Execute `systemctl restart teleport`

This will perform a graceful restart, i.e.the Teleport daemon will fork a new
process to handle new incoming requests, leaving the old daemon process running
until existing clients disconnect.

## License File

Commercial Teleport subscriptions require a valid license. The license file can
be downloaded from the [Teleport Customer
Portal](https://dashboard.gravitational.com/web/login).

The Teleport license file contains a X.509 certificate and the corresponding
private key in PEM format. Place the downloaded file on Auth servers and set the
`license_file` configuration parameter of your `teleport.yaml` to point to the
file location:

``` yaml
auth_service:
    license_file: /var/lib/teleport/license.pem
```

The `license_file` path can be either absolute or relative to the configured
`data_dir` . If license file path is not set, Teleport will look for the
`license.pem` file in the configured `data_dir` .

!!! tip "NOTE"

    Only Auth servers require the license. Proxies and Nodes that do
    not also have Auth role enabled do not need the license.

## Troubleshooting

To diagnose problems you can configure [ `teleport` ](cli-docs.md#teleport) to
run with verbose logging enabled by passing it `-d` flag.

!!! tip "NOTE"

    It is not recommended to run Teleport in production with verbose
    logging as it generates a substantial amount of data.

Sometimes you may want to reset [`teleport`](cli-docs.md#teleport) to a clean
state. This can be accomplished by erasing everything under `"data_dir"`
directory. Assuming the default location, `rm -rf /var/lib/teleport/*` will do.

Teleport also supports HTTP endpoints for monitoring purposes. They are disabled
by default, but you can enable them:

``` bash
$ teleport start --diag-addr=127.0.0.1:3000
```

Now you can see the monitoring information by visiting several endpoints:

* `http://127.0.0.1:3000/metrics` is the list of internal metrics Teleport is
   tracking. It is compatible with [Prometheus](https://prometheus.io/)
   collectors. For a full list of metrics review our [metrics reference](metrics-logs-reference.md).

* `http://127.0.0.1:3000/healthz` returns "OK" if the process is healthy or
  `503` otherwise.

* `http://127.0.0.1:3000/readyz` is similar to `/healthz` , but it returns "OK"
  _only after_ the node successfully joined the cluster, i.e.it draws the
  difference between "healthy" and "ready".

* `http://127.0.0.1:3000/debug/pprof/` is Golang's standard profiler. It's only
  available when `-d` flag is given in addition to `--diag-addr`

## Getting Help

If you need help, please ask on our [community forum](https://community.gravitational.com/). You can also open an [issue on Github](https://github.com/gravitational/teleport/issues).

For commercial support, you can create a ticket through the [customer dashboard](https://dashboard.gravitational.com/web/login).

For more information about custom features, or to try our [Enterprise edition](enterprise/introduction.md) of Teleport, please reach out to us at [sales@gravitational.com](mailto:sales@gravitational.com).
