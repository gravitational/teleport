---
authors: Zac Bergquist (zac.bergquist@goteleport.com)
state: draft
---

# RFD TBD - Teleport Cert Renewal Bot

## What

RFD TBD defines the high-level goals and architecture for a new agent that can
continuosly renew Teleport-issued certificates.

## Why

Teleport nodes running the SSH service maintain a connection to the cluster and
are able automatically renew their host certificates when a CA rotation takes
place or the certificate is approaching its expiration date.

Unfortunately, external systems that join a Teleport cluster (OpenSSH servers,
databases, applications, etc.) are not under our control, and therefore do not
have a built in mechanism to renew certificates under these circumstances. The
workflow for working with these types of external systems is to generate
certificates via `tctl auth sign`, copy them to the target, and manually
configure the target system to use these certificates. This is a process that
must be repeated whenever a certificate is about to expire. Additionally, this
approach is likely to incur downtime during
[CA rotations](https://goteleport.com/docs/architecture/authentication/#certificate-rotation)
as the renewal has to be performed within the rotation's grace period in order
to maintain connectivity.

This experience can incentivize users to make decisions that conflict with
security best practices, including:

- Requesting certificates with a very large time-to-live (TTL)
- Electing not to perform CA rotations

Both of these decisions are simply delaying the inevitable - eventually, the
certificates will need to be renewed and external systems will need to be
updated. The agent proposed in this RFD intends to automate the process of
renewing these certificates - requiring less manual work on behalf of our users,
and enabling them to use shorter TTLs and perform frequent CA rotations.

## Details

### Goals

The primary user this agent is designed for is the admin who wants to leverage
Teleport for as many systems as possible, but simply does not have the time to
keep track of all certificates and manually renew them.

While users with sophisticated automation and dedicated credential management
tools may benefit from this solution, they likely already have the resources to 
automate certificate renewals.

As a result, a primary goal for this tool is simplicity. We will optimize the user
experience for those looking to add systems to their cluster quickly and easily rather
than supporting a multitude of configuration options and hooks for customization.

### Overview

We will develop a new lightweight tool, `tbot`, which runs as an agent on
external systems that are part of a Teleport cluster (a VM running a Postgres
database, or an OpenSSH server, for example).

This agent will join the Teleport cluster, request certificates, store them in a
user-specified [destination](#destination), and then monitor the certificates
and renew them as necessary.

#### User Experience

First, a token will be generated for the bot to join the cluster.

```
$ tctl tokens add --type=bot
The invite token: 13b74f49d27536dd5c514073097c197b
This token will expire in 60 minutes

...
```

Next, the agent will be started using the generated token (full set of options
omitted here for brevity):

```
$ tbot start --name=foo --auth-server=proxy.example.com:3080 --token=TOKEN ...
```

Note that both direct dialing or connecting through a Teleport proxy are supported.

The bot joins the cluster and can be seen with the `tctl` command:

```
$ tctl bots ls
ID              NAME
0123-4567       foo
```

TODO: what else might we want to see? public addr? number (and kind) of certs?

##### Security

It is important to consider the security implications of a credential that can
be continuously renewed. In order to minimize the blast radius of an attack, we
need to minimize both the scope and duration that an attacker could leverage a
compromised credential.

We limit the scope by allowing users to define exactly which roles the certificate
should assume. We encourage all bot certificates to follow the principle of least
privilege and define only the minimum set of permissions necessary.
(TODO more on impersonation)

We limit the amount of time an attacker can act with these certificates by
setting an aggressive expiration time on renewable certificates and allowing
users to prevent a bot from renewing certificates with the new `tctl lock`
functionality. A locked bot is unable to renew its certificates, so an attacker
who compromises a bot can only use the certificate until it expires. With a
sufficiently small TTL, the window for a valid attack can be minimized. To
further minimize this window, an administrator can initiate a CA rotation with a
small (or zero) grace period immediately after locking the compromised bot.

TODO:

- this depends on someone noticing an anomoly and deciding to lock a bot - how
  do we make it easier to detect? more audit events?
- should probably generate a new keypair when we renew and not just bump the
  cert's TTL - this prevents an attack from holding on to a private key and
  attempting to use it to decrypt traffic in the future

Note that a bot can manage multiple sets of certificates, and locking a
particular bot will prevent it from renewing *any* of the certificates it
manages.

```
$ tctl lock --bot=0123-4567
```

The confirmation message when running a lock command can include a list of the
certificates the bot is managing.

```
This bot is managing the following certificates:

- x509 server certificate at /etc/ssl (expires in 7 days)
- SSH client certificate at /etc/ssh (expires in 8 hours)

Locking the bot will prevent the renewal of these certificates.
Are you sure you want to continue? (y/n)
```

#### Renewals

There are several scenarios under which the bot will initiate a renewal:

1. When a certificate is nearing expiry. A certificate is considered near
   expiration if 75% of its TTL has elapsed, or when there are 4 hours or less
   until expiration (whichever is sooner).
2. When a user and/or host CA rotation is taking place.
3. When a renewal is requested manually. For testing and debugging purposes, the
   bot will expose an API endpoint that can be used to trigger a renewal. This
   API will be accessible on the loopback interface only.

#### Certificate Specification


The bot will be configured with one or more certificate specs, which can be
supplied on the command line in the format of:

```
--cert=MODE,DESTINATION,RELOAD
```

*TODO*: how can a user specify DNS names to include in the certs?

##### Mode

The mode tells the bot which type of certificate to generate. 

There are a number of ways the bot can be used to acquire and renew certificates.

| Mode          | Certificate Type   | Signed By  |  Include User CA | Include Host CA |
|---------------|--------------------|------------|------------------|-----------------|
| `x509:client` | x509               | User CA    | no               | yes             |
| `x509:server` | x509               | Host CA    | yes              | no              |
| `ssh:client`  | SSH                | User CA    | no               | yes             |
| `ssh:server`  | SSH                | Host CA    | yes              | no              |

For example, for mode `x509:client`, the bot will issue x509 client certificates
signed by Teleport's user CA. It also writes Teleport's host CA to the
destination so that the client can be configured to trust the server.

The mode also controls what the output of `tbot config` looks like.

TODO:

- should we use `user` and `host` instead of `client`/`server` terminology?

##### Destination

The destination tells the bot where to store the certificates it generates. The
syntax for a destination specifier is `type:location`. Initially, the only
supported type will be `dir` which indicates that the certificates should be
placed in a directory on the local filesystem. In the future, we may support
additional destinations such as Kubernetes secrets, credendial managers, or
CI/CD systems. If multiple `--cert` specifiers are provided, the bot will check
for overlapping destinations on startup, and fail fast if this is the case.

##### Reload Commmand

The reload command is an optional command that `tbot` will execute after a
succesful renewal. This can be used, for example, to restart an OpenSSH server
after new certificates are written to disk. Note: this command is executed with
Go's `os/exec` package, meaning that a system shell is not invoked, so shell
features such as glob patterns, redirection, or environment variable expansion
are not supported.

We will list a few example reload commands for common use cases (OpenSSH, nginx)
in our docs.

##### Configuration Assist

One of the more time consuming aspects of setting up mutual TLS is configuring
both sides to present the correct certificate and trust certificates signed by a
specific CA. We aim to simplify this configuration process for common systems by
providing a `tbot config` command which spits out snippets that can be pasted 
directly into a configuration file.

For example, if `tbot` is running on an OpenSSH server and managing/renewing a
host certificate, `tbot config` would render the correct `/etc/ssh/ssh_config`
configuration for presenting the host certificate and trusting users signed by
Teleport's user CA.

Initially, `tbot config` will be limited to supporting OpenSSH configuration
(client and server).

##### Join Script

We intend to develop a bot join script, similar in functionailty to the node
join script that exists in Teleport enterprise. While not required, this will
make it even easier to install and run the bot as a systemd service on the
target machine.

### Implementation

Initially, the bot will be implemented as a standalone tool (ie under
`tool/tbot`) and _not_ as a service that runs in a `teleport` process.

The bulk of the new logic will be implemented in a new `renew` package, which
will enable us to start with a standalone bot, and incorporate this
functionality into other parts of Teleport (ie database access) in the future.

### tbot configuration

The goal is to keep tbot configuration to a minimum to make for a quick getting
started experience. Ideally, all configuration is provided via the command line
and there is no tbot-specific configuration file. This has several advantages:

- Our users are less likely to mix up a tbot configuration file and a
  teleport.yaml.
- The command used to invoke tbot is all that is needed to run the same
  configuration. This makes it easy to troubleshoot or recreate a customer
  issue.

#### Configuration Assist

- `tbot config` to call a simple REST API that listens on loopback interface
  only (it must be run on the same host as the tbot agent)
- if tbot is managing multiple `--cert`s, you can pass `mode:destination` to
  `tbot config` to tell it which cert to render configuration for

Note: `tctl auth sign` has a tiny amount of configuration templating already for
Postgres and MongoDB certificates. We should move this logic out into an
`extconfig` package that can render configuration snippets for a variety of
external systems. This work is out of scope for the purposes of this RFD.

#### Polling for expiration

In order to watch a certificate for expiration, the
`teleport/lib/utils/interval` package will be used. Note that tbot will only set
up polling for _user certs_, as host certificates issued by teleport do not
expire (host certificates *will* be re-issued during CA rotations).

#### Renewable Certificates

Teleport's current behavior when re-issuing a user certificate is maintain the
original expiration and refuse to bump out the TTL. We plan to preserve this
behavior by default, and only allow for extending the expiration date on
certificates that have been marked renewable.

In order to mark a certificate as renewable, we'll add an attribute to the
subject in the certificate when the `bot` system role is present in the
certificate identity.

#### RBAC

The new system role for the bot will need: (verify this)

```
types.NewRule(types.KindEvent, services.RW()),
types.NewRule(types.KindProxy, services.RO()),
types.NewRule(types.KindCertAuthority, services.ReadNoSecrets()),
types.NewRule(types.KindUser, services.RO()),
types.NewRule(types.KindNamespace, services.RO()),
types.NewRule(types.KindRole, services.RO()),
types.NewRule(types.KindAuthServer, services.RO()),
types.NewRule(types.KindReverseTunnel, services.RW()),
types.NewRule(types.KindTunnelConnection, services.RO()),
types.NewRule(types.KindClusterName, services.RO()),
types.NewRule(types.KindClusterConfig, services.RO()),
types.NewRule(types.KindLock, services.RO()),
```

The proxy system role will need to be updated to include:

```
types.NewRule(types.KindBot, services.RO()),
```

#### Caching

*TODO*: should the cache set up a watch on bots? how many openssh servers does a
large customer have in their clusters?

Is it even possible to skip the cache, or must it conform to the client interface?

#### Audit Log

The auth server will emit new events to the audit log when:

- a new renewable certificate is issued for the first time
- a certificate is renewed