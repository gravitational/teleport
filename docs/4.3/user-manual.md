# Teleport User Manual

This User Manual covers usage of the Teleport client tool, `tsh` and Teleport's Web interface.

In this document you will learn how to:

* Log into an interactive shell on remote cluster nodes.
* Copy files to and from cluster nodes.
* Connect to SSH clusters behind firewalls without any open ports, using SSH
  reverse tunnels.
* Explore a cluster and execute commands on specific nodes in a cluster.
* Share interactive shell sessions with colleagues or join someone else's
  session.
* Replay recorded interactive sessions.

In addition to this document, you can always simply type `tsh` into your
terminal for the [CLI reference](cli-docs.md).


## Introduction

For the impatient, here's an example of how a user would typically use
[`tsh`](cli-docs.md#tsh):

```bash
# Login into a Teleport cluster. This command retrieves user's certificates
# and saves them into ~/.tsh/teleport.example.com
$ tsh login --proxy=teleport.example.com

# SSH into a node, as usual:
$ tsh ssh user@node

# `tsh ssh` takes the same arguments as OpenSSH client:
$ tsh ssh -o ForwardAgent=yes user@node
$ tsh ssh -o AddKeysToAgent=yes user@node

# you can even create a convenient symlink:
$ ln -s /path/to/tsh /path/to/ssh

# ... and now your 'ssh' command is calling Teleport's `tsh ssh`
$ ssh user@host

# This command removes SSH certificates from a user's machine:
$ tsh logout
```

In other words, Teleport was designed to be fully compatible with existing
SSH-based workflows and does not require users to learn anything new, other than
to call [`tsh login`](cli-docs.md#tsh-login) in the beginning.

## Installing tsh

- Windows Users: [Download tsh Binary](https://gravitational.com/teleport/download?os=windows)
- Mac Users: [Download Mac Teleport Binary. Includes tsh](https://gravitational.com/teleport/download?os=macos)
- Mac Users with [Brew](https://brew.sh/): `brew install teleport`
- Linux Users: [Download Mac Teleport Binary. Includes tsh](https://gravitational.com/teleport/download?os=linux)

## User Identities

A user identity in Teleport exists in the scope of a cluster. The member nodes
of a cluster may have multiple OS users on them. A Teleport administrator
assigns allowed logins to every Teleport user account.

When logging into a remote node, you will have to specify both logins. Teleport
identity will have to be passed as `--user` flag, while the node login will be
passed as `login@host`, using syntax compatible with traditional `ssh`.

```bash
# Authenticate against the "work" cluster as joe and then
# log into the node as root:
$ tsh ssh --proxy=work.example.com --user=joe root@node
```
[CLI Docs - tsh ssh](cli-docs.md#tsh-ssh)

## Logging In

To retrieve a user's certificate, execute:

```bash
# Full form:
$ tsh login --proxy=proxy_host:<https_proxy_port>,<ssh_proxy_port>

# Using default ports:
$ tsh login --proxy=work.example.com

# Using custom HTTPS port:
$ tsh login --proxy=work.example.com:5000

# Using custom SSH proxy port, which is set on the Auth Server:
$ tsh login --proxy=work.example.com:2002
```
[CLI Docs - tsh login](cli-docs.md#tsh-login)

Port               | Description
-------------------|-------------------------------------
https_proxy_port   | the HTTPS port the proxy host is listening to (defaults to 3080).
ssh_proxy_port     | the SSH port the proxy is listening to (defaults to 3023).


The login command retrieves a user's certificate and stores it in `~/.tsh`
directory as well as in the [ssh
agent](https://en.wikipedia.org/wiki/Ssh-agent), if there is one running.

This allows you to authenticate just once, maybe at the beginning of the day.
Subsequent `tsh ssh` commands will run without asking for credentials until the
temporary certificate expires. By default, Teleport issues user certificates
with a TTL (time to live) of 12 hours.

!!! tip "Tip"

    It is recommended to always use [`tsh login`](cli-docs.md#tsh-login) before
    using any other `tsh` commands. This
    allows users to omit `--proxy` flag in subsequent tsh commands. For example
    `tsh ssh user@host` will work.

A Teleport cluster can be configured for multiple user identity sources. For
example, a cluster may have a local user called "admin" while regular users
should [authenticate via Github](admin-guide.md#github-oauth-20). In this case,
you have to pass `--auth` flag to `tsh login` to specify which identity storage
to use:

```bash
# Login using the local Teleport 'admin' user:
$ tsh --proxy=proxy.example.com --auth=local --user=admin login

# Login using Github as an SSO provider, assuming the Github connector is called "github"
$ tsh --proxy=proxy.example.com --auth=github --user=admin login
```

When using an external identity provider to log in, `tsh` will need to open a web browser to
complete the authentication flow. By default, `tsh` will use your system's default browser to open
such links. If you wish to suppress this behaviour, you can use the `--browser=none` flag:

```
# Don't open the system default browser when logging in
$ tsh login --proxy=work.example.com --browser=none
```

In this situation, a link will be printed to the screen. You can copy and paste this link into
a browser of your choice to continue the login flow.

[CLI Docs - tsh login](cli-docs.md#tsh-login)


### Inspecting SSH Certificate

To inspect the SSH certificates in `~/.tsh`, a user may execute the following
command:

```bash
$ tsh status

> Profile URL:  https://proxy.example.com:3080
  Logged in as: johndoe
  Roles:        admin*
  Logins:       root, admin, guest
  Valid until:  2017-04-25 15:02:30 -0700 PDT [valid for 1h0m0s]
  Extensions:   permit-agent-forwarding, permit-port-forwarding, permit-pty
```
[CLI Docs - tsh status](cli-docs.md#tsh-status)

### SSH Agent Support

If there is an [ssh agent](https://en.wikipedia.org/wiki/Ssh-agent) running,
`tsh login` will store the user certificate in the agent. This can be verified
via:

```bash
$ ssh-add -L
```

SSH agent can be used to feed the certificate to other SSH clients, for example
to OpenSSH `ssh`.

If you wish to disable SSH agent integration, pass `--no-use-local-ssh-agent`
to `tsh`. You can also set the `TELEPORT_USE_LOCAL_SSH_AGENT` environment
variable to `false` in your shell profile to make this permanent.

### Identity Files

[`tsh login`](cli-docs.md#tsh-login) can also save the user certificate into a
file:

```bash
# Authenticate user against proxy.example.com and save the user
# certificate into joe.pem file
$ tsh login --proxy=proxy.example.com --out=joe

# Use joe.pem to login into a server 'db'
$ tsh ssh --proxy=proxy.example.com -i joe joe@db
```

By default, `--out` flag will create an identity file suitable for `tsh -i` but
if compatibility with OpenSSH is needed, `--format=openssh` must be specified.
In this case the identity will be saved into two files: `joe` and
`joe-cert.pub`:

```bash
$ tsh login --proxy=proxy.example.com --out=joe --format=openssh
$ ls -lh
total 8.0K
-rw------- 1 joe staff 1.7K Aug 10 16:16 joe
-rw------- 1 joe staff 1.5K Aug 10 16:16 joe-cert.pub
```

### SSH Certificates for Automation

<!--This seems more like an admin task-->

Regular users of Teleport must request an auto-expiring SSH certificate, usually
every day. This doesn't work for non-interactive scripts, like cron jobs or
CI/CD pipeline.

For such automation, it is recommended to create a separate Teleport user for
bots and request a certificate for them with a long time to live (TTL).

In this example we're creating a certificate with a TTL of 10 years for the
jenkins user and storing it in jenkins.pem file, which can be later used with
`-i` (identity) flag for `tsh`.

```bash
# to be executed on a Teleport auth server
$ tctl auth sign --ttl=87600h --user=jenkins --out=jenkins.pem
```
[CLI Docs - tctl auth sign](cli-docs.md#tctl-auth-sign)

Now `jenkins.pem` can be copied to the jenkins server and passed to `-i`
(identity file) flag of `tsh`. Essentially `tctl auth sign` is an admin's
equivalent of `tsh login --out` and allows for unrestricted certificate TTL
values.

## Exploring the Cluster

In a Teleport cluster, all nodes periodically ping the cluster's auth server and
update their status. This allows Teleport users to see which nodes are online
with the `tsh ls` command:

```bash
# This command lists all nodes in the cluster which you previously logged in via "tsh login":
$ tsh ls

# Output:
Node Name     Node ID                Address            Labels
---------     -------                -------            ------
turing        11111111-dddd-4132     10.1.0.5:3022      os:linux
turing        22222222-cccc-8274     10.1.0.6:3022      os:linux
graviton      33333333-aaaa-1284     10.1.0.7:3022      os:osx
```
[CLI Docs - tsh ls](cli-docs.md#tsh-ls)

`tsh ls` can apply a filter based on the node labels.

```bash
# only show nodes with os label set to 'osx':
$ tsh ls os=osx

Node Name     Node ID                Address            Labels
---------     -------                -------            ------
graviton      33333333-aaaa-1284     10.1.0.7:3022      os:osx
```
[CLI Docs -tsh ls](cli-docs.md#tsh-ls)

## Interactive Shell

To launch an interactive shell on a remote node or to execute a command, use
`tsh ssh`.

`tsh` tries to mimic the `ssh` experience as much as possible, so it supports
the most popular `ssh` flags like `-p`, `-l` or `-L`. For example, if you have
the following alias defined in your ~/.bashrc: `alias ssh="tsh ssh"` then you
can continue using familiar SSH syntax:

```bash
# Have this alias configured, perhaps via ~/.bashrc
$ alias ssh=/usr/local/bin/tsh

# Login into a cluster and retrieve your SSH certificate:
$ tsh --proxy=proxy.example.com login

# These commands execute `tsh ssh` under the hood:
$ ssh user@node
$ ssh -p 6122 user@node ls
$ ssh -o ForwardAgent=yes user@node
$ ssh -o AddKeysToAgent=yes user@node
```

### Proxy Ports

A Teleport proxy uses two ports: `3080` for HTTPS and `3023` for proxying SSH
connections. The HTTPS port is used to serve Web UI and also to implement 2nd
factor auth for the `tsh` client.

If a Teleport proxy is configured to listen on non-default ports, they must be
specified via `--proxy` flag as shown:

```
tsh --proxy=proxy.example.com:5000,5001 <subcommand>
```

This means _use port `5000` for HTTPS and `5001` for SSH_.

### Port Forwarding

`tsh ssh` supports the OpenSSH `-L` flag which forwards incoming
connections from localhost to the specified remote host:port. The syntax of `-L`
flag is as follows, where "bind_ip" defaults to `127.0.0.1`:

```bash
-L [bind_ip]:listen_port:remote_host:remote_port
```

Example:

```bash
$ tsh ssh -L 5000:web.remote:80 node
```

This will connect to remote server `node` via `proxy.example.com`, then it will
open a listening socket on `localhost:5000` and will forward all incoming
connections to `web.remote:80` via this SSH tunnel.

It is often convenient to establish port forwarding, execute a local command
which uses the connection, and then disconnect. You can do this with the `--local`
flag.

Example:

```bash
$ tsh ssh -L 5000:google.com:80 --local node curl http://localhost:5000
```

This command:

1. Connects to `node`
2. Binds the local port 5000 to port 80 on google.com
3. Executes `curl` command locally, which results in `curl` hitting
   google.com:80 via `node`


### SSH Jumphost

While implementing ProxyJump for Teleport, we have extended the feature to `tsh`.

```bash
$ tsh ssh -J proxy.example.com telenode
```

Known limits:

* Only one jump host is supported (`-J` supports chaining that Teleport does not utilise)
and tsh will return with error in case of two jumphosts: `-J` proxy-1.example.com,proxy-2.example.com
will not work.
* Only one jump host is supported (`-J` supports chaining that Teleport does not utilise) and `tsh` will return with error in the case of two jumphosts, i.e. `-J proxy-1.example.com,proxy-2.example.com` will not work.
* When `tsh ssh -J user@proxy` is used, it overrides the SSH proxy defined in the tsh profile and port forwarding is used instead of the existing Teleport proxy subsystem.


### Resolving Node Names

`tsh` supports multiple methods to resolve remote node names.

1. Traditional: by IP address or via DNS.
2. Nodename setting: teleport daemon supports `nodename` flag, which allows
   Teleport administrators to assign alternative node names.
3. Labels: you can address a node by `name=value` pair.

If we have two nodes, one with `os:linux` label and one node with `os:osx`, we
can log into the OSX node with:

```bash
$ tsh ssh os=osx
```

This only works if there is only one remote node with the `os:osx` label, but
you can still execute commands via SSH on multiple nodes using labels as a
selector. This command will update all system packages on machines that run
Linux:

```bash
$ tsh ssh os=ubuntu apt-get update -y
```

### Short-lived Sessions

The default TTL of a Teleport user certificate is 12 hours. This can be modified
at login with the `--ttl` flag. This command logs you into the cluster with a
very short-lived (1 minute) temporary certificate:

```bash
$ tsh --ttl=1 login
```

You will be logged out after one minute, but if you want to log out immediately,
you can always do:

```bash
$ tsh logout
```

## Copying Files

To securely copy files to and from cluster nodes, use the `tsh scp` command. It
is designed to mimic traditional `scp` as much as possible:

```bash
$ tsh scp example.txt root@node:/path/to/dest
```

Again, you may want to create a bash alias like `alias scp="tsh --proxy=work
scp"` and use the familiar syntax:

```bash
$ scp -P 61122 -r files root@node:/path/to/dest
```

## Sharing Sessions

Suppose you are trying to troubleshoot a problem on a remote server. Sometimes
it makes sense to ask another team member for help. Traditionally, this could be
done by letting them know which node you're on, having them SSH in, start a
terminal multiplexer like `screen` and join a session there.

Teleport makes this more convenient. Let's log into a server named "luna"
and ask Teleport for our current session status:

```bash
$ tsh ssh luna
>luna $ teleport status

User ID    : joe, logged in as joe from 10.0.10.1 43026 3022
Session ID : 7645d523-60cb-436d-b732-99c5df14b7c4
Session URL: https://work:3080/web/sessions/7645d523-60cb-436d-b732-99c5df14b7c4
```

Now you can invite another user account to the "work" cluster. You can share the
URL for access through a web browser, or you can share the session ID and she
can join you through her terminal by typing:

```bash
$ tsh join <session_ID>
```

!!! note

    Joining sessions is not supported in recording proxy mode (where `session_recording` is set to `proxy`).

## Connecting to SSH Clusters behind Firewalls

Teleport supports creating clusters of servers located behind firewalls
**without any open listening TCP ports**.  This works by creating reverse SSH
tunnels from behind-firewall environments into a Teleport proxy you have access to.

This feature is called ["Trusted Clusters"](https://gravitational.com/teleport/docs/trustedclusters/). Refer to
[the admin manual](admin-guide.md#trusted-clusters) to learn how a trusted cluster can be configured.

Assuming the "work" Teleport proxy server is configured with a few trusted
clusters, a user may use the `tsh clusters` command to see a list of all clusters on the server:

```bash
$ tsh --proxy=work clusters

Cluster Name     Status
------------     ------
staging          online
production       offline
```
[CLI Docs - tsh clusters](cli-docs.md#tsh-clusters)

Now you can use the `--cluster` flag with any `tsh` command. For example, to list SSH nodes that are members of the "production" cluster, simply do:

```bash
$ tsh --proxy=work ls --cluster=production
Node Name     Node ID       Address            Labels
---------     -------       -------            ------
db-1          xxxxxxxxx     10.0.20.31:3022    kernel:4.4
db-2          xxxxxxxxx     10.0.20.41:3022    kernel:4.2
```

Similarly, if you want to SSH into `db-1` inside the "production" cluster:

```bash
$ tsh --proxy=work ssh --cluster=production db-1
```

This is possible even if nodes in the "production" cluster are located behind a
firewall without open ports. This works because the "production" cluster
establishes a reverse SSH tunnel back into "work" proxy, and this tunnel is used
to establish inbound SSH connections.

## Web UI

Teleport proxy serves the web UI on `https://proxyhost:3080`. The UI allows you
to see the list of online nodes in a cluster, open a web-based terminal to them,
see recorded sessions, and replay them. You can also join other users in active
sessions.

## Using OpenSSH Client

There are a few differences between Teleport's `tsh` and OpenSSH's `ssh` but
most of them can be mitigated.

1. `tsh` always requires the `--proxy` flag because `tsh` needs to know which
    cluster you are connecting to. But if you execute `tsh --proxy=xxx login`,
    the current proxy will be saved in your `~/.tsh` profile and won't be needed
    for other `tsh` commands.

2. `tsh ssh` operates _two_ usernames: one for the cluster and another for the
   node you are trying to log into. See [User Identities](#user-identities)
   section below. For convenience, `tsh` assumes `$USER` for both by default.
   But again, if you use `tsh login` before `tsh ssh`, your Teleport username
   will be stored in `~/.tsh`.

If you'd like to set the login name that should be used by default on the remote host, you can set the `TELEPORT_LOGIN` environment variable.

!!! tip "Tip"

    To avoid typing `tsh ssh user@host` when logging into servers,
    you can create a symlink `ssh -> tsh` and execute the symlink. It will
    behave exactly like a standard `ssh` command, i.e. `ssh login@host`. This is
    helpful with other tools that expect `ssh` to just work.

Teleport is built using standard SSH constructs: keys, certificates and
protocols. This means that a Teleport system is 100% compatible with both
OpenSSH clients and servers.

For an OpenSSH client (`ssh`) to work with a Teleport proxy, two conditions must
be met:

1. `ssh` must be configured to connect through a Teleport proxy.
2. `ssh` needs to be given the SSH certificate issued by the `tsh login` command.

### SSH Proxy Configuration

To configure `ssh` to use a Teleport proxy on `proxy.example.com`, a user must
update the `/etc/ssh/ssh_config` or `~/.ssh/config`. A few examples are shown
below:

```bash
# When "ssh db" is executed, OpenSSH will connect to proxy.example.com on port 3023
# and will request a proxied connection to "db" on port 3022 (default Teleport SSH port)
Host db
    Port 3022
    ProxyJump proxy.example.com:3023
```

The configuration above is all you need to `ssh root@db` if there's an
SSH agent running on a client computer. You can verify it by executing `ssh-add -L`
right after `tsh login`. If the SSH agent is running, the cluster certificates will
be printed to stdout.

If there is no ssh-agent available, the certificate must be passed to the OpenSSH client explicitly.

When proxy is in ["Recording mode"](https://gravitational.com/blog/how-to-record-ssh-sessions/) the following will happen with SSH:

```bash
$ ssh -J user@teleport.proxy:3023 -p 3022 user@target -F ./forward.config
```

Where `forward.config` enables agent forwarding:

```bash
Host teleport.proxy
  ForwardAgent yes
```

### Passing Teleport SSH Certificate to OpenSSH Client

If a user does not want to use an SSH agent or if the agent is not available,
the certificate must be passed to `ssh` via `IdentityFile` option (see `man
ssh_config`). Consider this example: the Teleport user "joe" wants to login into
the proxy named "lab.example.com".

He executes the [`tsh login`](cli-docs.md#tsh-login) command:

```bash
$ tsh --proxy=lab.example.com login --user=joe
```

His identity is now stored in `~/.tsh/keys/lab.example.com`, so his `~/.ssh/config` needs to look like this:

```bash
# ~/.ssh/config file:
Host *.lab.example.com
    Port 3022
    IdentityFile ~/.tsh/keys/lab.example.com/joe
    ProxyCommand ssh -i ~/.tsh/keys/lab.example.com/joe -p 3023 %r@lab.example.com -s proxy:%h:%p
```

Now he can SSH into any machine behind `lab.example.com` using the OpenSSH
client:

```bash
$ ssh jenkins.lab.example.com
```


## Troubleshooting

If you encounter strange behaviour, you may want to try to solve it by enabling
the verbose logging by specifying `-d` flag when launching `tsh`. Also, you may
want to reset it to a clean state by deleting temporary keys and other data from
`~/.tsh`

## Getting Help

If you need help, please ask on our [community forum](https://community.gravitational.com/). You can also open an [issue on Github](https://github.com/gravitational/teleport/issues).

For commercial support, you can create a ticket through the [customer dashboard](https://dashboard.gravitational.com/).

For more information about custom features, or to try our [Enterprise edition](enterprise/index.md) of Teleport, please reach out to us at [sales@gravitational.com](mailto:sales@gravitational.com).
