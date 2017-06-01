# Admin Manual

This manual covers installation and configuration of Teleport and the ongoing 
management of a Teleport cluster. It assumes that the reader has good understanding 
of Linux administration.

## Installing

Gravitational Teleport is written in Go language. It requires Golang v1.7 or newer. 
If you have Go already installed, type:

```bash
$ go get github.com/gravitational/teleport
$ cd $GOPATH/src/github.com/gravitational/teleport
$ make release
```

You can also download binaries from [Github releases](https://github.com/gravitational/teleport/releases) or you can [build it from source](https://github.com/gravitational/teleport).

## Definitions

Before diving into configuring and running Teleport, it helps to take a look at the [Teleport Architecture](/architecture) 
and go over the key concepts this document will be referring to:

|Concept   | Description
|----------|------------
|Node      | Synonym to "server" or "computer", something one can "SSH to". A node must be running `teleport` daemon running with "node" role/service turned on.
|Certificate Authority (CA) | A pair of public/private keys Teleport uses to manage access. A CA can sign a public key of a user or node establishing their cluster membership.
|Teleport Cluster | A Teleport Auth Service contains two CAs. One is used to sign user keys and the other signs node keys. A collection of nodes connected to the same CA is called a "cluster". 
|Cluster Name | Every Teleport cluster must have a name. If a name is not supplied via `teleport.yaml` configuration file, a GUID will be generated. **IMPORTANT:** renaming a cluster invalidates its keys and all certificates it had created.
|Trusted Cluster | Teleport Auth Service can allow 3rd party users or nodes to connect if their public keys are signed by a trusted CA. A "trusted cluster" is a pair of public keys of the trusted CA. It can be configured via `teleport.yaml` file.

## Teleport Daemon

The Teleport daemon is called `teleport` and it supports the following commands:

|Command     | Description
|------------|-------------------------------------------------------
|start       | Starts the Teleport daemon.
|configure   | Dumps a sample configuration file in YAML format into standard output.
|version     | Shows the Teleport version.
|status      | Shows the status of a Teleport connection. This command is only available from inside of an active SSH session.
|help        | Shows help.

When experimenting you can quickly start `teleport` with verbose logging by typing 
`teleport start -d`. 

!!! danger "WARNING": 
    Teleport stores data in `/var/lib/teleport`. Make sure that regular/non-admin users do not 
    have access to this folder on the Auth server.

### Systemd Unit File

In production, we recommend starting teleport daemon via an 
init system like `systemd`.  Here's the example of a systemd unit file:

```
[Unit]
Description=Teleport SSH Service
After=network.target 

[Service]
Type=simple
Restart=always
ExecStart=/usr/local/bin/teleport start --config=/etc/teleport.yaml

[Install]
WantedBy=multi-user.target
```

### Ports

Teleport services listen on several ports. This table shows the default port numbers.

|Port      | Service    | Description
|----------|------------|-------------------------------------------
|3022      | Node       | SSH port. This is Teleport's equivalent of port `#22` for SSH.
|3023      | Proxy      | SSH port clients connect to. A proxy will forward this connection to port `#3022` on the destination node.
|3024      | Proxy      | SSH port used to create "reverse SSH tunnels" from behind-firewall environments into a trusted proxy server.
|3025      | Auth       | SSH port used by the Auth Service to serve its API to other nodes in a cluster.
|3080      | Proxy      | HTTPS connection to authenticate `tsh` users and web users into the cluster. The same connection is used to serve a Web UI.


## Configuration

You should use a configuration file to configure the `teleport` daemon. 
But for simpler experimentation you can use command line flags to 
`teleport start` command. To see the list of flags:

```
$ teleport start --help
usage: teleport start [<flags>]
Flags:
  -d, --debug            Enable verbose logging to stderr
  -r, --roles            Comma-separated list of roles to start with [proxy,node,auth]
      --pid-file         Full path to the PID file. By default no PID file will be created
      --advertise-ip     IP to advertise to clients if running behind NAT
  -l, --listen-ip        IP address to bind to [0.0.0.0]
      --auth-server      Address of the auth server [127.0.0.1:3025]
      --token            One-time token to register with an auth server [none]
      --nodename         Name of this node, defaults to hostname
  -c, --config           Path to a configuration file [/etc/teleport.yaml]
      --labels           List of labels for this node
      --permit-user-env  Enables reading of ~/.tsh/environment when creating a session
```

### Configuration Flags

Let's cover some of these flags in more detail:

* `--roles` flag tells Teleport which services to start. It is a comma-separated
  list of roles. The possible values are `auth`, `node` and `proxy`. The default 
  value is `auth,node,proxy`. These roles are explained in the 
  [Teleport Architecture](architecture.md) document.

* `--advertise-ip` flag can be used when Teleport nodes are running behind NAT and
  their externally routable IP cannot be automatically determined. 
  For example, assume that a host "foo" can be reached via `10.0.0.10` but there is
  no `A` DNS record for "foo", so you cannot connect to it via `tsh ssh foo`. If
  you start teleport on "foo" with `--advertise-ip=10.0.0.10`, it will automatically 
  tell Teleport proxy to use that IP when someone tries to connect
  to "foo". This is also useful when connecting to Teleport nodes using their labels.

* `--nodename` flag lets you assign an alternative name the node which can be used
  by clients to login. By default it's equal to the value returned by `hostname` 
  command.

* `--listen-ip` should be used to tell `teleport` daemon to bind to a specific network
  interface. By default it listens on all.

* `--labels` flag allows to assign a set of labels to a node. See the explanation
  of labeling mechanism in the [Labeling Nodes](#labeling-nodes) section below.

* `--pid-file` flag creates a PID file if a path is given.

* `--permit-user-env` flag reads in environment variables from `~/.tsh/environment`
  when creating a session.
  
### Configuration File

Teleport uses the YAML file format for configuration. A sample configuration file is shown
below. By default, it is stored in `/etc/teleport.yaml`

!!! note "IMPORTANT": 
    When editing YAML configuration, please pay attention to how your editor 
    handles white space. YAML requires consistent handling of tab characters.

```bash
# By default, this file should be stored in /etc/teleport.yaml

# This section of the configuration file applies to all teleport
# services.
teleport:
    # nodename allows to assign an alternative name this node can be reached by.
    # by default it's equal to hostname
    nodename: graviton

    # Data directory where Teleport keeps its data, like keys/users for 
    # authentication (if using the default BoltDB back-end)
    data_dir: /var/lib/teleport

    # one-time invitation token used to join a cluster. it is not used on 
    # subsequent starts
    auth_token: xxxx-token-xxxx

    # when running in multi-homed or NATed environments Teleport nodes need 
    # to know which IP it will be reachable at by other nodes
    advertise_ip: 10.1.0.5

    # list of auth servers in a cluster. you will have more than one auth server
    # if you configure teleport auth to run in HA configuration
    auth_servers: 
        - 10.1.0.5:3025
        - 10.1.0.6:3025

    # Teleport throttles all connections to avoid abuse. These settings allow
    # you to adjust the default limits
    connection_limits:
        max_connections: 1000
        max_users: 250

    # Logging configuration. Possible output values are 'stdout', 'stderr' and 
    # 'syslog'. Possible severity values are INFO, WARN and ERROR (default).
    log:
        output: stderr
        severity: ERROR

    # Type of storage used for keys. You need to configure this to use etcd
    # backend if you want to run Teleport in HA configuration.
    storage:
        type: bolt

# This section configures the 'auth service':
auth_service:
    # Turns 'auth' role on. Default is 'yes'
    enabled: yes

    # Turns on dynamic configuration. Dynamic configuration defines the source
    # for configuration information, configuration files on disk or what's
    # stored in the backend. Default is false if no backend is specified,
    # otherwise if backend is specified, it is assumed to be true.
    dynamic_config: false

    # defines the types and second factors the auth server supports
    authentication:
        # type can be local or oidc
        type: local
        # second_factor can be off, otp, or u2f
        second_factor: otp

        # this section is only used if using u2f
        u2f:
            # app_id should point to the Web UI.
            app_id: https://localhost:3080

            # facets should list all proxy servers.
            facets:
            - https://localhost
            - https://localhost:3080

    # IP and the port to bind to. Other Teleport nodes will be connecting to
    # this port (AKA "Auth API" or "Cluster API") to validate client 
    # certificates 
    listen_addr: 0.0.0.0:3025

    # Pre-defined tokens for adding new nodes to a cluster. Each token specifies
    # the role a new node will be allowed to assume. The more secure way to 
    # add nodes is to use `ttl node add --ttl` command to generate auto-expiring 
    # tokens. 
    #
    # We recommend to use tools like `pwgen` to generate sufficiently random
    # tokens of 32+ byte length.
    tokens:
        - "proxy,node:xxxxx"
        - "auth:yyyy"

    # Optional "cluster name" is needed when configuring trust between multiple
    # auth servers. A cluster name is used as part of a signature in certificates
    # generated by this CA.
    # 
    # By default an automatically generated GUID is used.
    #
    # IMPORTANT: if you change cluster_name, it will invalidate all generated 
    # certificates and keys (may need to wipe out /var/lib/teleport directory)
    cluster_name: "main"

# This section configures the 'node service':
ssh_service:
    # Turns 'ssh' role on. Default is 'yes'
    enabled: yes

    # IP and the port for SSH service to bind to. 
    listen_addr: 0.0.0.0:3022
    # See explanation of labels in "Labeling Nodes" section below
    labels:
        role: master
        type: postgres
    # List (YAML array) of commands to periodically execute and use
    # their output as labels. 
    # See explanation of how this works in "Labeling Nodes" section below
    commands:
    - name: hostname
      command: [/usr/bin/hostname]
      period: 1m0s
    - name: arch
      command: [/usr/bin/uname, -p]
      period: 1h0m0s

    # enables reading ~/.tsh/environment before creating a session. by default
    # set to false, can be set true here or as a command line flag.
    permit_user_env: false

# This section configures the 'proxy servie'
proxy_service:
    # Turns 'proxy' role on. Default is 'yes'
    enabled: yes

    # SSH forwarding/proxy address. Command line (CLI) clients always begin their
    # SSH sessions by connecting to this port
    listen_addr: 0.0.0.0:3023

    # Reverse tunnel listening address. An auth server (CA) can establish an 
    # outbound (from behind the firewall) connection to this address. 
    # This will allow users of the outside CA to connect to behind-the-firewall 
    # nodes.
    tunnel_listen_addr: 0.0.0.0:3024

    # The HTTPS listen address to serve the Web UI and also to authenticate the 
    # command line (CLI) users via password+HOTP
    web_listen_addr: 0.0.0.0:3080

    # TLS certificate for the HTTPS connection. Configuring these properly is 
    # critical for Teleport security.
    https_key_file: /etc/teleport/teleport.key
    https_cert_file: /etc/teleport/teleport.crt
```

## Authentication

Teleport supports two types of user accounts: 

* **Internal users** are created and stored in Teleport's own identitiy storage. A cluster
  administrator has to create account entries for every Teleport user. 
  Teleport also supports two factor authentication (2FA), which is turned on by default. 
  There are two types of 2FA supported:
    * [TOTP](https://en.wikipedia.org/wiki/Time-based_One-time_Password_Algorithm)
      is the default. You can use [Google Authenticator](https://en.wikipedia.org/wiki/Google_Authenticator) or 
      [Authy](https://www.authy.com/) or any other TOTP client.
    * [U2F](https://en.wikipedia.org/wiki/Universal_2nd_Factor) is the second.
* **External users** are users stored elsewhere else within an organization. Examples include
  Github, Active Directory (AD), LDAP server, OpenID/OAuth2 endpoint or behind SAML. 

## FIDO U2F

Teleport supports [FIDO U2F](https://www.yubico.com/about/background/fido/) 
hardware keys as a second authentication factor.

To start using U2F:

* Purchase a U2F hardware key: looks like a tiny USB drive.
* Enable U2F in Teleport configuration `teleport.yaml`.
* For CLI-based logins you have to install [u2f-host](https://developers.yubico.com/libu2f-host/) utility. 
* For web-based logins you have to use Google Chrome, as the only browser supporting U2F at this moment.

Lets look into each of these steps in detail.

### Getting U2F Keys

The following hardware keys have been tested with Teleport:
   * [Yubikey](https://www.yubico.com/products/yubikey-hardware)

### Enabling U2F

By default U2F is disabled. To enable U2F, add the following to the auth 
service configuration in `teleport.yaml`:

```yaml
authentication:
   type: local
   second_factor: u2f

   u2f:
      # app_id should point to the Web UI.
      app_id: https://localhost:3080

      # facets should list all proxy servers.
      facets:
         - https://localhost
         - https://localhost:3080
```

For single-proxy setups, the App ID can be equal to the domain name of the
proxy, but this will prevent you from adding more proxies without changing the
App ID.  For multi-proxy setups, the App ID should be an HTTPS URL pointing to
a JSON file that mirrors `facets` in the auth config.

The JSON file should be hosted on a domain you control and it should be
accessible anonymously. See the [official U2F specification](https://fidoallian
ce.org/specs/fido-u2f-v1.0-ps-20141009/fido-appid-and-facets-ps-20141009.html#p
rocessing-rules-for-appid-and-facetid-assertions) for the exact format of the
JSON file.

!!! warning "Warning": 
    The App ID must never change in the lifetime of the cluster. If the App ID
    changes, all existing U2F key registrations will become invalid and all users
    who use U2F as the second factor will need to re-register.

    When adding a new proxy server, make sure to add it to the list of "facets" 
    in the configuration file, but also to the JSON file referenced by `app_id`

### Logging in with U2F

For logging in via the CLI, you must first install [u2f-host](https://developers.yubico.com/libu2f-host/). 
Installing: 

```bash
# OSX:
$ brew install libu2f-host

# Ubuntu 16.04 LTS:
$ apt-get install u2f-host
```

Then invoke `tsh ssh` as usual to authenticate:

```
tsh --proxy <proxy-addr> ssh <hostname>
```

!!! tip "Version Warning": 
    External user identities are only supported in [Teleport Enterprise](/enterprise/). Please reach
    out to `sales@gravitational.com` for more information.

## Adding and Deleting Users

This section covers internal user identities, i.e. user accounts created and
stored in Teleport's internal storage.

A user identity in Teleport exists in the scope of a cluster. The member nodes
of a cluster have multiple OS users on them. A Teleport administrator assigns
allowed logins to every Teleport account, allowing it to login as one of the 
specified OS users.

Let's look at this table:

|Teleport User | Allowed OS Logins | Description
|------------------|---------------|-----------------------------
|joe    | joe,root | Teleport user 'joe' can login into member nodes as OS user 'joe' or 'root'
|bob    | bob      | Teleport user 'bob' can login into member nodes only as OS user 'bob'
|ross   |          | If no OS login is specified, it defaults to the same name as the Teleport user.

To add a new user to Teleport you have to use `tctl` tool on the same node where
the auth server is running, i.e. `teleport` was started with `--roles=auth`. 

```bash
$ tctl users add joe joe,root
```

Teleport generates an auto-expiring token (with a TTL of 1 hour) and prints the token 
URL which must be used before the TTL expires. 

```
Signup token has been created. Share this URL with the user:
https://<proxy>:3080/web/newuser/xxxxxxxxxxxx

NOTE: make sure the <proxy> host is accessible.
```

The user will complete registration by visiting this URL, picking a password and 
configuring the 2nd factor authentication. If the credentials are correct, the auth 
server generates and signs a new certificate and the client stores this key and will use 
it for subsequent logins. The key will automatically expire after 23 hours by default after which 
the user will need to log back in with her credentials. This TTL can be configured to a maximum
of 30 hours and a minimum of 1 minute. Once authenticated, the account will become visible via `tctl`:

```bash
$ tctl users ls

User           Allowed to Login as
----           -------------------
admin          admin,root
ross           ross
joe            joe,root 
```

Joe would need to use the `tsh` client tool to log in to member node "luna" via 
bastion "work" _as root_:

```bash
$ tsh --proxy=work --user=joe root@luna
```

To delete this user:

```bash
$ tctl users del joe
```

## Adding Nodes to the Cluster

Gravitational Teleport is a "clustered" SSH manager, meaning it only allows SSH
access to nodes that had been previously granted cluster membership. 

A cluster membership means that every node in a cluster has its own host
certificate signed by the cluster's auth server. 

!!! tip "Note": 
	If interoperability with
	OpenSSH is required, make sure the node name and DNS name match because OpenSSH
	clients validate the DNS name against the node name presented on the certificate
	when connecting to a Teleport node.

A new Teleport node needs an "invite token" to join a cluster. An invite token 
also defines which role a new node can assume within a cluster: `auth`, `proxy` or 
`node`. 

There are two ways to create invitation tokens:

* Static Tokens
* Dynamic, Short-lived Tokens

### Static Tokens

You can pick your own tokens and add them to the auth server's config file: 

```bash
# Config section in `/etc/teleport/teleport.yaml` file for the auth server
auth_service:
    enabled: true
    #
    # statically assigned token: obviously we recommend a much harder to guess
    # value than `xxxxx`, consider generating tokens using a tool like pwgen
    #
    tokens:
    - "proxy,node:xxxxxx"
```

Now you can start a new Teleport node by setting its invitation token via `--token`
flag to `xxxxxx`. This node will join the cluster as a regular node but also
as a proxy server:

```bash
$ teleport start --roles=node,proxy --token=xxxxx --auth-server=10.0.10.5
```

### Short-lived Tokens

A more secure way to add nodes to a cluster is to generate tokens as they are 
needed. Such token can be used multiple times until its time to live (TTL) 
expires.

Use `tctl` tool to invite a new node into the cluster with `node` and `auth` 
roles:

```bash
$ tctl nodes add --ttl=5m --roles=node,proxy
The invite token: 24be3e582c3805621658225f8c841d2002
Run this on the new node to join the cluster:
> teleport start --roles=node,proxy --token=24be3e582c3805621658225f8c841d2002 --auth-server=192.168.1.8:3025

Please note:
  - This invitation token will expire in 5 minutes
  - 192.168.1.8:3025 must be reachable from the new node, see --advertise-ip server flag
  - For tokens of type "trustedcluster", tctl needs to be used to create a TrustedCluster resource. See the Admin Guide for more details. 
```

As new nodes come online, they start sending ping requests every few seconds
to the CA of the cluster. This allows everyone to explore cluster membership
and size:

```bash
$ tctl nodes ls

Node Name     Node ID                                  Address            Labels
---------     -------                                  -------            ------
turing        d52527f9-b260-41d0-bb5a-e23b0cfe0f8f     10.1.0.5:3022      distro:ubuntu
dijkstra      c9s93fd9-3333-91d3-9999-c9s93fd98f43     10.1.0.6:3022      distro:debian
```

## Revoking Invitations

As you have seen above, Teleport uses tokens to invite users to a cluster (sign-up tokens) or 
to add new nodes to it (provisioning tokens).

Both types of tokens can be revoked before they can be used. To see a list of outstanding tokens,
run this command:

```bash
$ tctl tokens ls

Token                                Role       Expiry Time (UTC)
-----                                ----       -----------------
eoKoh0caiw6weoGupahgh6Wuo7jaTee2     Proxy      never
696c0471453e75882ff70a761c1a8bfa     Node       17 May 16 03:51 UTC
6fc5545ab78c2ea978caabef9dbd08a5     Signup     17 May 16 04:24 UTC
```

In this example, the first token with "never" expiry date is a static token configured via
a config file. It cannot be revoked. 

The 2nd token with "Node" role was generated to invite a new node to this cluster. And the
3rd token was generated to invite a new user.

The latter two tokens can be deleted (revoked) via `tctl tokens del` command:

```bash
$ tctl tokens del 696c0471453e75882ff70a761c1a8bfa
Token 696c0471453e75882ff70a761c1a8bfa has been deleted
```

## Labeling Nodes

In addition to specifying a custom nodename, Teleport also allows for the application of arbitrary
key:value pairs to each node. They are called labels. There are two kinds of labels:

1. `static labels` never change while the `teleport` process is running. You may want
   to label nodes with their physical location, the Linux distribution, etc.

2. `label commands` or "dynamic labels". Label commands allow you to execute an external
   command on a node at a configurable frequency. The output of that command becomes
   the value of such label. Examples include reporting a kernel version, load averages,
   time after reboot, etc.

Labels can be configured in a configuration file or via `--labels` flag as shown below:

```bash
$ teleport start --labels uptime=[1m:"uptime -p"],kernel=[1h:"uname -r"]
```

Obviously the kernel version is not going to change often, so this example runs
`uname` once an hour. When this node starts and reports its labels into the cluster,
users will see:

```bash
$ tctl nodes ls

Node Name     Node ID          Address         Labels
---------     -------          -------         ------
turing        d52527f9-b260    10.1.0.5:3022   kernel=3.19.0-56,uptime=up 1 hour, 15 minutes
```

## Audit Log

Teleport logs every SSH event into its audit log. The log is stored on the auth server(s) 
in the `data_dir` location, under `log` subdirectory.

There are two components of the audit log:

1. **SSH Events:** Teleport logs events like successful user logins along with 
   the metadata like remote IP address, time and the sesion ID.
2. **Recorded Sessions:** Every SSH shell session is recorded and can be replayed 
   later.

### SSH Events

The event log is stored in `data_dir` under `log` directory, usually it is `/var/lib/teleport/log`.
Each day is represented as a file:

```bash
$ ls -l /var/lib/teleport/log/
total 104
-rw-r----- 1 root root  31638 Jan 22 20:00 2017-01-23.00:00:00.log
-rw-r----- 1 root root  91256 Jan 31 21:00 2017-02-01.00:00:00.log
-rw-r----- 1 root root  15815 Feb 32 22:54 2017-02-03.00:00:00.log
```

The log files use JSON format. They are human-readable but can also be programmatically parsed.
Each line represents an event and has the following format:

```js
{
   // Event type. See below for the list of all possible event types
   "event"      : "session.start",
   // Teleport user name
   "user"       : "ekontsevoy",
   // OS login
   "login"      : "root",
   // Server namespace. This field is reserved for future use.
   "namespace"  : "default",
   // Unique server ID.
   "server_id"  : "f84f7386-5e22-45ff-8f7d-b8079742e63f",
   // Session ID. Can be used to replay the sesssion.
   "sid"        : "8d3895b6-e9dd-11e6-94de-40167e68e931",
   // Address of the SSH node
   "addr.local" : "10.5.l.15:3022",
   // Address of the connecting client (user)
   "addr.remote": "73.223.221.14:42146",
   // Terminal size
   "size"       : "80:25",
   // Timestamp
   "time"       : "2017-02-03T06:54:05Z"
}
```

The possible event types are:

Event Type      | Description
----------------|----------------
auth            | Authentication attempt. Adds the following fields: `{"success": "false", "error": "access denied"}`
session.start   | Started an interactive shell session.
session.end     | An interactive shell session has ended.
session.join    | A new user has joined the existing interactive shell session.
session.leave   | A user has left the session.
exec            | Remote command has been executed via SSH, like `tsh ssh root@node ls /`. The following fields will be logged: `{"command": "ls /", "exitCode": 0, "exitError": ""}`
scp             | Remote file copy has been executed. The following fields will be logged: `{"path": "/path/to/file.txt", "len": 32344, "action": "read" }`
resize          | Terminal has been resized.

!!! tip "Note":
    The commercial Teleport edition called "Teleport Enterprise" supports native
    audit log exporting into external systems like Splunk, AlertLogic and others.
    Take a look at [Teleport Enterprise](enterprise.md) section to learn more.

### Recorded Sessions

In addition to logging `session.start` and `session.end` events, Teleport also records the entire
stream of bytes going to/from standard input and standard output of an SSH session.

The recorded sessions are stored as raw bytes in the `sessions` directory under `log`.
Each session consists of two files, both are named after the session ID:

1. `.bytes` file represents the raw session bytes and is somewhat
    human-readable, although you are better off using `tsh play` or 
    the Web UI to replay it.
2. `.log` file contains the copies of the event log entries that are related 
   to this session.

```bash
$ ls /var/lib/teleport/log/sessions/default
-rw-r----- 1 root root 506192 Feb 4 00:46 4c146ec8-eab6-11e6-b1b3-40167e68e931.session.bytes
-rw-r----- 1 root root  44943 Feb 4 00:46 4c146ec8-eab6-11e6-b1b3-40167e68e931.session.log
```

To replay this session via CLI:

```bash
$ tsh --proxy=proxy play 4c146ec8-eab6-11e6-b1b3-40167e68e931
```

## Trusted Clusters

Teleport allows to partition your infrastructure into multiple clusters. Some clusters can be 
located behind firewalls without any open ports. They can also have their own restrictions on
which users have access.

As [explained above](#nomenclature), a Teleport Cluster has a name and is managed by a 
`teleport` daemon with "auth service" enabled.

Let's assume we need to place some servers behind a firewall and we only want Teleport 
user "john" to have access to them. We already have our primary Teleport cluster and our 
users set up. Say this primary cluster is called `main`, and the behind-the-firewall cluster
is called `cluster-b` as shown on this diagram:

![Tunels](img/tunnel.svg)

This setup works as follows:

0. `cluster-b` and `main` trust each other: they are "trusted clusters".
1. `cluster-b` creates an outbound reverse SSH tunnel to `main` and keeps it open.
2. Users of `main` should use `--cluster=cluster-b` flag of `tsh` tool if they want to connect to any nodes of `cluster-b`.
3. The `main` cluster uses the tunnel to connect back to any node of `cluster-b`.

### Example Configuration

To add behind-the-firewall machines and restrict access only to "john", we will have to do the following:

1. Add `cluster-b` to the list of trusted clusters of `main`.
2. Add `main` cluster to the list of trusted clusters of `cluster-b`.
3. Tell `cluster-b` to open a reverse tunnel to `main`. 
4. Tell `cluster-b` to only allow user "john" from the `main` cluster.

Let's look into the details of each step. First, let's configure two independent (at first) clusters: 

```
auth_service:
  enabled: yes
  cluster_name: main
```

And our behind-the-firewall cluster:

```
auth_service:
  enabled: yes
  cluster_name: cluster-b
```

Start both servers. At this point they do not know about each other.
Now, export their public CA keys:

On "main":

```
$ tctl auth export > main-cluster.ca
```

On "cluster-b":

```
$ tctl auth export > b-cluster.ca
```

!!! tip "NOTE":
    In Teleport 2.0 the format used when exporting Certificate Authorities (CAs)
    has changed to better support interoperability with OpenSSH. In Teleport
    1.0, all CAs were exported in the `known_hosts` format. Starting in Teleport
    2.0 we export host CAs in `known_hosts` format and user CAs in
    `authorized_keys` format. For compatibility with Teleport 1.0, you can still
    export user CAs in the `known_hosts` format with the following command:
    `tctl auth export --compat=1.0 > cluster.ca`.

Update the YAML configuration of both clusters to connect them. 

On `main`:

```yaml
auth_service:
  enabled: yes
  cluster_name: main
  trusted_clusters:
      - key_file: /path/to/b-cluster.ca
```

On `cluster-b` (notice the `tunnel_addr` - that should point to the address of `main` proxy node):

```yaml
auth_service:
  enabled: yes
  cluster_name: cluster-b
  trusted_clusters:
      - key_file: /path/to/main-cluster.ca
        # This line contains comma-separated list of OS logins allowed
        # to users from this trusted cluster
        allow_logins: john
        # This line establishes a reverse SSH tunnel from 
        # cluster-b to main:
        tunnel_addr: 62.28.10.1
```

Now, if you restart `teleport` auth service on both clusters, they should trust each
other. To verify, run this on "cluster-b":

```
$ tctl auth ls
CA keys for the local cluster cluster-b:

CA Type     Fingerprint
-------     -----------
user        xxxxxxxxxxxxxxx
host        zzzzzzzzzzzzzzz

CA Keys for Trusted Clusters:

Cluster Name     CA Type     Fingerprint                     Allowed Logins
------------     -------     -----------                     --------------
main             user        zzzzzzzzzzzzzzzzzzzzzzzzzzz     john
main             host        xxxxxxxxxxxxxxxxxxxxxxxxxxx     N/A
```

Notice that each cluster is shown as two CAs: one is used to establish trust between nodes,
and another one is for trusting users. 

Now, our sample user John, having direct access to a proxy server of cluster "main" (let's call it main.proxy), can use `tsh` command to see which clusters are online:

```
$ tsh --proxy=main.proxy clusters
```

John can also list all nodes in the cluster-b:

```
$ tsh --proxy=main.proxy --cluster=cluster-b ls
```

Similarly, by passing `--cluster=cluster-b` to `tsh` John can login into cluster-b nodes.

!!! tip "Note":
    Teleport Enterprise also supports adding and removing trusted clusters dynamically
    at runtime. See [this section](enterprise.md#dynamic-trusted-clusters) to learn more.


### Permissions with Trusted Clusters

As illustrated in the above example, when you make changes to the Trusted Cluster
configuration, you need to restart Teleport. In addition if you specify your
backend, you need to set `dynamic_config: false` to make sure your changes are
propagated to the Auth Server.

In the example below we are starting with just allowing `root` to login to
`cluster-b` then also allowing `jsmith`.

First update `teleport.yaml` to so that `dynamic_config: false` set under
`auth_service` for both clusters and `allowed_logins` has your new user.
Something like this:

```
auth_service:
  dynamic_config: false
  trusted_clusters:
    - key_file: /path/to/one.ca
      allow_logins: root, jsmith
      tunnel_addr: one
```

You’ll need to restart the Auth Server in `cluster-b`.

If you look at the roles on `cluster-b`, you will see that you are allowed to
login as `root` or `jsmith`.

```
$ tctl get roles
Role          Allowed to login as     Namespaces     Node Labels     Access to resources
----          -------------------     ----------     -----------     -------------------
ca:cluster-b  root,jsmith             default        <all nodes>     node:read,session:read,tunnel:read,auth_server:read,cert_authority:read
```

Now back on the main cluster, you need to make sure you are issued a certificate
that allows you to login as `root` or `jsmith`. The easiest way to do this would be to
delete the existing `jsmith` user and create them again but you can do the same
by creating a new role with `logins` set and assigning `jsmith` that role.

```
$ tctl users del jsmith
User 'jsmith' has been deleted
```
```
$ tctl users add jsmith root,jsmith
Signup token has been created and is valid for 3600 seconds. Share this URL with the user:
https://localhost:3080/web/newuser/20ca3354800bdd50f6df0b19818e9c0e
```

Now take a look at at the allowed logins on your main cluster:

```
$ tctl get roles
Role            Allowed to login as     Namespaces     Node Labels     Access to resources
----            -------------------     ----------     -----------     -------------------
ca:cluster-b                            default        <all nodes>     auth_server:read,cert_authority:read,node:read,session:read,tunnel:read
user:jsmith     root,jsmith             default        <all nodes>     auth_server:read,cert_authority:read,node:read,role:read,session:read,tunnel:read
```

You can now login as both `root` and `jsmith`. Login again and you will be able to see the same in the issued SSH certificate:

```
$ tsh --proxy=localhost --user=jsmith login
[...]
$ ssh-keygen -L -f ~/.tsh/keys/localhost/jsmith.cert 
/root/.tsh/keys/localhost/jsmith.cert:
        Type: ssh-rsa-cert-v01@openssh.com user certificate
        Public key: RSA-CERT 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00
        Signing CA: RSA 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00
        Key ID: "jsmith"
        Serial: 0
        Valid: before 2017-05-09T12:04:36
        Principals: 
                root
                jsmith
        Critical Options: (none)
        Extensions: 
                permit-port-forwarding
                permit-pty
```

### HTTP CONNECT Tunneling

Some networks funnel all connections through a proxy server where they can be
audited and access control rules applied. For these scenarios Teleport supports
HTTP CONNECT tunneling.

To use HTTP CONNECT tunneling, simply set either the `HTTPS_PROXY` or
`HTTP_PROXY` environment variables and when Teleport builds and establishes the
reverse tunnel to the main cluster, it will funnel all traffic though the proxy.
Specifically Teleport will tunnel ports `3024` (SSH, reverse tunnel) and `3080`
(HTTPS, establishing trust) through the proxy.

The value of `HTTPS_PROXY` or `HTTP_PROXY` should be in the format
`scheme://host:port` where scheme is either `https` or `http`. If the
value is `host:port`, Teleport will prepend `http`.

!!! tip "Note":
    `localhost` and `127.0.0.1` are invalid values for the proxy host. If for
    some reason your proxy runs locally, you'll need to provide some other DNS
    name or a private IP address for it.

## Using Teleport with OpenSSH

Teleport is a standards-compliant SSH proxy and it can work in environments with 
existing SSH implementations, such as OpenSSH. This section will cover:

* Configuring OpenSSH client `ssh` to login into nodes inside a Teleport cluster.
* Configuring OpenSSH server `sshd` to join a Teleport cluster.

### Using OpenSSH Client

It is possible to use the OpenSSH client `ssh` to connect to nodes within a Teleport
cluster. Teleport supports SSH subsystems and includes a `proxy` subsystem that
can be used like `netcat` is with `ProxyCommand` to connect through a jump host.

First, you need to export the public keys of cluster members. This has to be done 
on a node which runs Teleport auth server:

```bash
$ tctl auth --type=host export > cluster_node_keys
```

On your client machine, you need to import these keys. It will allow your OpenSSH client
to verify that host's certificates are signed by the trusted CA key:

```bash
$ cat cluster_node_keys >> ~/.ssh/known_hosts
```

Make sure you are logged in and then launch `tsh` in the SSH agent mode:

```bash
$ tsh --proxy=work.example.com agent
```

`tsh agent` will print environment variables into the console. Copy and paste
the output into the shell you will be using to connect to a Teleport node.
The output exports the `SSH_AUTH_SOCK` and `SSH_AGENT_PID` environment variables
that allow OpenSSH clients to find the SSH agent.

Lastly, configure the OpenSSH client to use the Teleport proxy when connecting
to nodes with matching names. Edit `~/.ssh/config` for your user or
`/etc/ssh/ssh_config` for global changes:

```
# work.example.com is the jump host (proxy). credentials will be obtained from the
# teleport agent.
Host work.example.com
    HostName 192.168.1.2
    Port 3023

# connect to nodes in the work.example.com cluster through the jump
# host (proxy) using the same. credentials will be obtained from the
# teleport agent.
Host *.work.example.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@work.example.com -s proxy:%h:%p

# when connecting to a node within a trusted cluster with name "remote-cluster",
# add the name of the cluster to the invocation of the proxy subsystem.
Host *.remote-cluster.example.com
   HostName %h
   Port 3022
   ProxyCommand ssh -p 3023 %r@work.example.com -s proxy:%h:%p@remote-cluster
```

When everything is configured properly, you can use ssh to connect to any node 
behind `work.example.com`:

```bash
$ ssh root@database.work.example.com
```

!!! tip "NOTE":
    Teleport uses OpenSSH certificates instead of keys which means you can not connect
    to a Teleport node by IP address. You have to connect by DNS name. This is because
    OpenSSH ensures the DNS name of the node you are connecting is listed under
    the `Principals` section of the OpenSSH certificate to verify you are connecting
    to the correct node.

### Integrating with OpenSSH Servers

Existing `sshd` servers can be added to a Teleport cluster. For that to work, you
have to configure `sshd` to trust the Teleport CA.

Export the Teleport CA certificate into a file:

```bash
$ tctl auth --type=user export > teleport-user-ca.pub
```

To allow access per-user, append the contents of `teleport-user-ca.pub` to
  `~/.ssh/authorized_keys`.

To allow access for all users:

  * Edit `teleport-user-ca.pub` and remove `cert-authority` from the start of line.
  * Copy `teleport-user-ca.pub` to `/etc/ssh/teleport-user-ca.pub`
  * Update `sshd` configuration (usually `/etc/ssh/sshd_config`) to point to this
  file: `TrustedUserCAKeys /etc/ssh/teleport-user-ca.pub`

## Integrating with Ansible

Ansible is uses the OpenSSH client by default. This makes it compatible with Teleport without any extra work, except configuring OpenSSH client to work with Teleport Proxy:

* configure your OpenSSH to connect to Teleport proxy and user `tsh agent` socket
* enable scp mode in the Ansible config file (default is `/etc/ansible/ansible.cfg`):
 
```bash
scp_if_ssh = True
```

## High Availability

Usually there are two ways to achieve high availability. You can "outsource"
this function to the infrastructure, for example by using a highly available
network-based disk volumes (similar to AWS EBS) and by migrating a failed VM to
a new host. In this scenario there's nothing Teleport-specific to be done.

But if high availability cannot be provided by the infrastructue (perhaps
you're running Teleport on a bare metal cluster), you can configure Teleport
to run in a highly available fashion. 

#### Run multiple instances of Teleport Auth Server 

  For this to work you must switch to a highly available secrets back-end first. 
  Also, you must tell each node in a cluster that there are
  more than one auth server available. The are two ways to do this:

  * Use a load balancer to create a single auth API access point (AP) and
    specify this AP in `auth_servers` section of Teleport configuration for
    all nodes in a cluster.
  * If a load balancer is not an option, you must specify each instance of an 
    auth server in `auth_servers` section of Teleport configuration.

#### Run multiple instances of Teleport Proxy 

The Teleport Proxy is stateless which makes running multiple instances trivial.
If using the [default configuration](#ports) configure your load balancer to
forward ports `3023` and `3080` to the servers that run the Teleport proxy. If
you have configured your proxy to use non-default ports, you will need to
configure your load balancer to forward the ports you specified for
`listen_addr` and `web_listen_addr` in `teleport.yaml`.

If your load balancer supports health checks, configure it to hit the
`/webapi/ping` endpoint on the proxy. This endpoint will reply `200 OK` if the
proxy is running without problems.

!!! tip "NOTE": 
    As the new auth servers get added to the cluster and the old servers get 
    decommissioned, nodes and proxies will refresh the list of available auth
    servers and store it in their local cache `/var/lib/teleport/authservers.json`. 
    The values from the cache file will take precedence over the configuration 
    file.

We'll cover how to use `etcd` and `DynamoDB` storage back-ends to make Teleport highly available below.

### Using etcd

Teleport can use [etcd](https://coreos.com/etcd/) as a storage backend to
achieve highly available deployments.  Obviously, you must take steps to
protect access to `etcd` in this configuration because that is where Teleport
secrets like keys and user records will be stored.

To configure Teleport for using etcd as a storage back-end:

* Install etcd and configure peer and client TLS authentication using
   [etcd security guide](https://coreos.com/etcd/docs/latest/security.html).

* Configure all Teleport Auth servers to use etcd in the "storage" section of
  the config file as shown below.
* Deploy several auth servers connected to etcd back-end.
* Deploy several proxy nodes that have `auth_servers` pointed to list of auth servers to connect to.

```yaml
teleport:
  storage:
     type: etcd
     # list of etcd peers to connect to:
     peers: ["https://172.17.0.1:4001", "https://172.17.0.2:4001"]

     # required path to TLS client certificate and key files to connect to etcd
     tls_cert_file: /var/lib/teleport/etcd-cert.pem
     tls_key_file: /var/lib/teleport/etcd-key.pem

     # optional file with trusted CA authority
     # file to authenticate etcd nodes
     tls_ca_file: /var/lib/teleport/etcd-ca.pem

     # etcd key (location) where teleport will be storing its state under:
     prefix: teleport

     # NOT RECOMMENDED: enables insecure etcd mode in which self-signed
     # certificate will be accepted
     insecure: false
```

### Using DynamoDB

If you are running Teleport on AWS, you can use [DynamoDB](https://aws.amazon.com/dynamodb/) 
as a storage back-end to achieve high availability.

To configure Teleport to use DynamoDB as a storage back-end:

* Make sure you have AWS access key and a secret key which give you access to
  DynamoDB account. If you're using (as recommended) an IAM role for this, the policy 
  with necessary permissions is listed below.
* Configure all Teleport Auth servers to use DynamoDB back-end in the "storage" section
  of `teleport.yaml` as shown below.
* Deploy several auth servers connected to DynamoDB storage back-end.
* Deploy several proxy nodes that have `auth_servers` pointed to list of Auth servers to connect to.

```yaml
teleport:
  storage:
    type: dynamodb
    region: eu-west-1
    table_name: teleport.state
    access_key: BKZA3H2LOKJ1QJ3YF21A
    secret_key: Oc20333k293SKwzraT3ah3Rv1G3/97POQb3eGziSZ
```

Replace `region` and `table_name` with your own settings. Teleport will create the table automatically.
Also, here's the example of the IAM policy to grant access to DynamoDB:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllAPIActionsOnTeleportAuth",
            "Effect": "Allow",
            "Action": "dynamodb:*",
            "Resource": "arn:aws:dynamodb:eu-west-1:123456789012:table/prod.teleport.auth"
        }
    ]
}
```

## Troubleshooting

To diagnose problems you can configure `teleport` to run with verbose logging enabled
by passing it `-d` flag.

!!! tip "NOTE": 
    It is not recommended to run Teleport in production with verbose logging
    as it generates substantial amount of data.

Sometimes you may want to reset `teleport` to a clean state. This can be accomplished
by erasing everything under `"data_dir"` directory. Assuming the default location, 
`rm -rf /var/lib/teleport/*` will do.

## Getting Help

Please open an [issue on Github](https://github.com/gravitational/teleport/issues).
Alternatively, you can reach through the contact form on our [website](https://gravitational.com/).

For commercial support, custom features or to try our commercial edition, [Teleport Enterprise](/enterprise/),
please reach out to us: `sales@gravitational.com`. 
