# User Manual

This User Manual covers usage of the Teleport client tool `tsh`. In this 
document you will learn how to:

* Securely login into interactive shell on remote cluster nodes.
* Securely copy files to and from cluster nodes.
* Connect to SSH clusters behind firewals without any open ports using SSH reverse tunnels.
* Explore a cluster and execute commands on those nodes in a cluster that match your criteria.
* Share interactive shell sessions with colleagues or join someone else's session.
* Replay recorded interactive sessions.
* Use Teleport with OpenSSH client: `ssh` or with other tools that use SSH under the hood like Chef and Ansible.

In addition to this document, you can always type `tsh` into your terminal for the CLI reference.
```bash
> tsh
usage: tsh [<flags>] <command> [<command-args> ...]

Gravitational Teleport SSH tool

Commands:
  help         Show help.
  version      Print the version
  ssh          Run shell or execute a command on a remote SSH node
  join         Join the active SSH session
  play         Replay the recorded SSH session
  scp          Secure file copy
  ls           List remote SSH nodes
  clusters     List available Teleport clusters
  agent        Start SSH agent on unix socket
  login        Log in to the cluster and store the session certificate to avoid login prompts
  logout       Delete a cluster certificate

Notes:

  - Most of the flags can be set in a profile file ~/.tshconfig
  - Run `tsh help <command>` to get help for <command> like `tsh help ssh`
```

## Difference vs OpenSSH

There are a few differences between Teleport's `tsh` and OpenSSH's `ssh` but the 
most noticeable ones are:

* Teleport only uses certificate-based authentication. Teleport is about clusters
  centered around a certificate authority (CA). The concept of "cluster membership" 
  essential in Teleport.

* `tsh` always requires `--proxy` flag because `tsh` needs to know which cluster
  you are connecting to. 

* `tsh` needs _two_ usernames: one for the cluster and another for the node you
  are trying to login into. See "Teleport Identity" section below. For convenience, 
  `tsh` assumes `$USER` for both logins by default.

While it may appear less convenient than `ssh`, we hope that the default behavior
and techniques like bash aliases will help to minimize the amount of typing.

On the other hand, Teleport is built using solely standard SSH constructus: keys,
certificates, protocols. This means that Teleport is 100% compatible with OpenSSH
clients and servers. See [Using Teleport with OpenSSH](admin-guide#using-teleport-with-openssh) 
chapter in the Admin Guide for more information.

## User Identities

A user identity in Teleport exists in the scope of a cluster. The member nodes
of a cluster may have multiple OS users on them. A Teleport administrator assigns
allowed logins to every Teleport user account.

When logging into a remote node, you will have to specify both logins. Teleport
identity will have to be passed as `--user` flag, while the node login will be
passed as `login@host`, using syntax compatible with traditional `ssh`.

These examples assume your localhost username is 'joe':

```bash
# Authenticate against cluster 'work' as 'joe' and then login into 'node'
# as root:
> tsh ssh --proxy=work.example.com --user=joe root@node

# Authenticate against cluster 'work' as 'joe' and then login into 'node'
# as joe (by default tsh uses $USER for both):
> tsh ssh --proxy=work.example.com node
```

`tsh` allows to login into the cluster without connecting to any master nodes:

```
> tsh login --proxy=work.example.com
```

This allows you to supply your password and the 2nd factor authentication
at the beginning of the day. Subsequent `tsh ssh` commands will run without
asking for your credentials until the temporary certificate expires (by default 12 hours).

## Exploring the Cluster

In a Teleport cluster all nodes periodically ping the cluster's auth server and
update their statuses. This allows Teleport users to see which nodes are online:

```bash
# Connect to cluster 'work' as $USER and list all nodes in 
# a cluster:
> tsh --proxy=work ls

# Output:
Node Name     Node ID                Address            Labels
---------     -------                -------            ------
turing        11111111-dddd-4132     10.1.0.5:3022     os:linux
turing        22222222-cccc-8274     10.1.0.6:3022     os:linux
graviton      33333333-aaaa-1284     10.1.0.7:3022     os:osx
```

You can filter out nodes based on their labels. Let's only list OSX machines:

```
> tsh --proxy=work ls os=osx

Node Name     Node ID                Address            Labels
---------     -------                -------            ------
graviton      33333333-aaaa-1284     10.1.0.7:3022     os:osx
```

## Interactive Shell

To launch an interactive shell on a remote node or to execute a command, use `tsh ssh` 
command:

```bash
> tsh ssh --help

usage: t ssh [<flags>] <[user@]host> [<command>...]
Run shell or execute a command on a remote SSH node.

Flags:
      --user      SSH proxy user [ekontsevoy]
      --proxy     SSH proxy host or IP address
      --ttl       Minutes to live for a SSH session 
      --insecure  Do not verify server certificate and host name. Use only in test environments
  -d, --debug     Verbose logging to stdout
  -p, --port      SSH port on a remote host
  -l, --login     Remote host login
  -L, --forward   Forward localhost connections to remote server
      --local     Execute command on localhost after connecting to SSH node

Args:
  <[user@]host>  Remote hostname and the login to use
  [<command>]    Command to execute on a remote host
```

`tsh` tries to mimic `ssh` experience as much as possible, so it supports the most popular `ssh`
flags like `-p`, `-l` or `-L`. For example if you have the following alias defined in your 
`~/.bashrc`: `alias ssh="tsh --proxy=work.example.com --user=myname"` then you can continue
using familiar SSH syntax:

```bash
> ssh root@host
> ssh -p 6122 root@host ls
```

### Port Forwarding

`tsh ssh` supports OpenSSH `-L` flag which allows to forward incoming connections from localhost
to the specified remote host:port. The syntax of `-L` flag is:

```
-L [bind_interface]:listen_port:remote_host:remote_port
```

where "bind_interface" defaults to `127.0.0.1`.

Exmaple:
```
> tsh --proxy=work ssh -L 5000:web.remote:80 -d node
```

Will connect to remote server `node` via `work` proxy, then it will open a listening socket on
`localhost:5000` and will forward all incoming connections to `web.remote:80` via this SSH 
tunnel.

It is often convenient to establish port forwarding, execute a local command which uses such 
connection and disconnect. Yon can do this via `--local` flag.

Example:
```
> tsh --proxy=work ssh -L 5000:google.com:80 --local node curl http://localhost:5000
```

This forwards just one curl request for `localhost:5000` to `google:80` via "node" server located
behind "work" proxy and terminates.

### Resolving Node Names

`tsh` supports multiple methods to resolve remote node names. 

1. Traditional: by IP address or via DNS.
2. Nodename setting: teleport daemon supports `nodename` flag, which allows Teleport administrators to assign alternative node names.
3. Labels: you can address a node by `name=value` pair.

In the example above, we have two nodes with `os:linux` label and one node with `os:osx`.
Lets login into the OSX node:

```bash
> tsh --proxy=work ssh os=osx
```

This only works if there is only one remote node with `os:osx` label, but you can still execute
commands via SSH on multiple nodes using labels as a selector. This command will update all
system packages on machines that run Linux:

```bash
> tsh --proxy=work ssh os=linux apt-get update -y
```

### Temporary Logins

Suppose you are borrowing someone else's computer to login into a cluster. You probably don't 
want to stay authenticated on this computer for 12 hours (Teleport default). This is where `--ttl`
flag can help.

This command logs you into the cluster with a very short-lived (1 minute) temporary certificate:

```bash
tsh --proxy=work --ttl=1 ssh
```

You will be logged out after one minute, but if you want to log out immediately, you can 
always do:

```bash
tsh --proxy=work logout
```

## Copying Files

To securely copy files to and from cluster nodes use `tsh scp` command. It is designed to mimic
traditional `scp` as much as possible:

```bash
> tsh scp --help

usage: tsh scp [<flags>] <from, to>...
Secure file copy

Flags:
      --user       SSH proxy user [ekontsevoy]
      --proxy      SSH proxy host or IP address
      --ttl        Minutes to live for a SSH session
      --insecure   Do not verify server certificate and host name. Use only in test environments
  -P, --debug      Verbose logging to stdout
  -d, --debug      Verbose logging to stdout
  -r, --recursive  Recursive copy of subdirectories

Args:
  <from, to>       Source and the destination
```

Examples:

```bash
> tsh --proxy=work scp example.txt root@node:/path/to/dest
```

Again, you may want to create a bash alias like `alias scp="tsh --proxy=work scp"` and use
the familiar sytanx:

```bash
> scp -P 61122 -r files root@node:/path/to/dest
```

## Sharing Sessions

Suppose you are trying to troubleshoot a problem on a remote server. Sometimes it makes sense 
to ask another team member for help. Traditionally this could be done by letting them know which 
node you're on, having them SSH in, start a terminal multiplexer like `screen` and join a 
session there.

Teleport makes this a bit more convenient. Let's login into "luna" and ask Teleport for your 
current session status:

```bash
> tsh --proxy=work ssh luna
luna > teleport status

User ID    : joe, logged in as joe from 10.0.10.1 43026 3022
Session ID : 7645d523-60cb-436d-b732-99c5df14b7c4
Session URL: https://work:3080/web/sessions/7645d523-60cb-436d-b732-99c5df14b7c4
```

Now you can invite another user account in the "work" cluster. You can share the URL for access through a web browser. 
Or you can share the session ID and she can join you through her terminal by typing:

```bash
> tsh --proxy=work join 7645d523-60cb-436d-b732-99c5df14b7c4
```

## Connecting to SSH Clusters behind Firewalls

Teleport supports creating clusters of servers located behind firewalls without any open ports.
This works by creating reverse SSH tunnels from behind-firewall environments into a Teleport
proxy you have access to. This feature is called "Trusted Clusters". 

Assuming your "work" Teleport server is configured with a few trusted clusters, this is how you can
see a list of them:

```bash
> tsh --proxy=work clusters

Cluster Name     Status
------------     ------
staging          online
production       offline
```

Now you can use `--cluster` flag with any `tsh` command. For example, to list SSH nodes that
are members of "production" cluster, simply do:

```bash
> tsh --proxy=work --cluster=production ls
Node Name     Node ID       Address            Labels
---------     -------       -------            ------
db-1          xxxxxxxxx     10.0.20.31:3022    kernel:4.4
db-2          xxxxxxxxx     10.0.20.41:3022    kernel:4.2
```

Similarly, if you want to SSH into `db-1` inside "production" cluster:

```bash
> tsh --proxy=work --cluster=production ssh db-1
```

This is possible even if nodes of the "production" cluster are located behind a firewall
without open ports. This works because "production" cluster establishes a reverse
SSH tunnel back into "work" proxy, and this tunnels is used to establish inbound SSH
connections.

For more details on configuring Trusted Clusters please look at [that section in the Admin Guide](admin-guide.md#trusted-clusters).

## Troubleshooting

If you encounter strange behaviour, you may want to try to solve it by enabling
the verbose logging by specifying `-d` flag when launching `tsh`.

Also you may want to reset it to a clean state by deleting temporary keys and 
other data from `~/.tsh`

## Getting Help

Please open an [issue on Github](https://github.com/gravitational/teleport/issues).
Alternatively, you can reach through the contact form on our [website](http://gravitational.com/).

For commercial support, custom features or to try our multi-cluster edition of Teleport,
please reach out to us: `sales@gravitational.com`.  
