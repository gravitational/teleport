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
are able automatically renew their certificates when a CA rotation takes
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

First, register a bot with the new `tctl bots add` subcommand, giving the bot a
name and one or more roles the bot is allowed to
[impersonate](https://goteleport.com/docs/access-controls/guides/impersonation/).

```
$ tctl bots add --name=jenkins --roles=ci
The invite token: 13b74f49d27536dd5c514073097c197b
This token will expire in 60 minutes

...
```

This command will:

- Create a role for the bot, allowing impersonation of the provided roles (`ci`
  in this case)
- Create a new user for the bot, and assign the new bot role to this user
- Generate a special invite token that bot must use to join the cluster. This
  token is scoped to the new user that was created for the bot.

Next, the `tbot` agent will be started using the generated token (full set of options
omitted here for brevity):

```
$ tbot start --name=foo --auth-server=proxy.example.com:3080 --token=13b74f49d27536dd5c514073097c197b ...
```

Note that both direct dialing or connecting through a Teleport proxy are supported.

The bot joins the cluster and can be seen with the `tctl` command:

```
$ tctl bots ls
ID            NAME       LOCKED    ROLES     CERTIFICATES
0123-4567     foo        false     dev,ssh   ssh (/home/ubuntu/.ssh/id_rsa-cert.pub)
```

When the bot starts, it generates a set of keys, and uses the join token that
was provided to request one or more signed certificates from the auth server.
These certificates are placed in the specified destination, and renew as necessary.

TODO: the following is pending based on whether we'll continue to allow the bot
to manage multiple sets of certs.

Note: the join token can only be used once, and is invalidated after first use
or when its TTL expires (whichever comes first). If the bot fails to renew its
certificates before they expire, it will be unable to authenticate with the
cluster and must re-join with a new token.

##### Security

It is important to consider the security implications of a credential that can
be continuously renewed. In order to minimize the blast radius of an attack, we
need to minimize both the scope and duration that an attacker could leverage a
compromised credential.

We limit the scope by allowing users to define exactly which roles the
certificate should assume. The `tctl` tool will create a dedicated user and role
for the bot, and this role will be able to impersonate the roles specified by
the user. We encourage users to follow the principle of least privilege and
allow impersonation for the minimum set of roles necessary.

We limit the amount of time an attacker can act with these certificates by
setting an aggressive expiration time on renewable certificates and allowing
users to prevent a bot from renewing certificates with the new `tctl lock`
functionality. A locked bot is unable to renew its certificates, so an attacker
who compromises a bot can only use the certificate until it expires. With a
sufficiently small TTL, the window for a valid attack can be minimized. To
further minimize this window, an administrator can initiate a CA rotation with a
small (or zero) grace period immediately after locking the compromised bot.

While out of scope for the initial implementation, the security of this approach
can be further improved in the future by pinning the certificates to a certain
machine.

#### Renewals

There are several scenarios under which the bot will initiate a renewal:

1. When a certificate is nearing expiry. A certificate is considered near
   expiration if 75% of its TTL has elapsed, or when there are 4 hours or less
   until expiration (whichever is sooner).
2. When a user and/or host CA rotation is taking place.
3. When a renewal is requested manually. For testing and debugging purposes, the
   bot will expose an API endpoint that can be used to trigger a renewal. This
   API will be accessible on the loopback interface only.

##### API Client Refresh

As of now, our [API client](./0010-api.md) is initialized with a set of TLS
credentials and expects those credentials be valid for the lifetime of the
client.

In order to continue communicating with the cluster, the bot will need to inform
the API client of the new credentials after a renewal. As part of this effort,
the client will be updated to allow for refreshing itself. This process
initializes a new client that attempts to connect with the new credentials, and
closes the original client when the new client is succesfully connected with the
cluster.

```
func (c *Client) Refresh(ctx context.Context) error {
    // use the existing config to generate a new client
    newClient, err := connect(ctx, c.c)

    c.Close() // close the original client

    *c = newClient
}
```

It will be the responsibility of the caller of `Refresh` to reinitialize any
watches or streams that the client may have been running prior to the refresh.

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

TODO: we always generate SSH and TLS certs, can we simplify the mode to just
user/host and always dump both types of certificates to the destination?

| Mode          | Certificate Type   | Signed By  |  Include User CA | Include Host CA |
|---------------|--------------------|------------|------------------|-----------------|
| `ssh:user`    | SSH                | User CA    | no               | yes             |
| `ssh:host`    | SSH                | Host CA    | yes              | no              |
| `x509:user`   | x509               | User CA    | no               | yes             |
| `x509:host`   | x509               | Host CA    | yes              | no              |

For example, for mode `x509:user`, the bot will issue x509 certificates signed
by Teleport's user CA. It also writes Teleport's host CA to the destination so
that the client can be configured to trust the server.

The mode also controls what the output of `tbot config` looks like.

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

We will list a few example reload commands for common use cases (OpenSSH, NGINX)
in our docs.

#### Configuration Assist

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

#### Join Script

We intend to develop a bot join script, similar in functionailty to the node
join script that exists in Teleport enterprise. While not required, this will
make it even easier to install and run the bot as a systemd service on the
target machine.

TODO: support for auto-join on AWS without token

### Implementation

Initially, the bot will be implemented as a standalone tool (ie under
`tool/tbot`) and _not_ as a service that runs in a `teleport` process.

The bulk of the new logic (parsing configuration, writing certificates to a
destination, etc.) will be implemented in a new `renew` package, which will
enable us to start with a standalone bot, and incorporate this functionality
into other parts of Teleport (ie database access) in the future.

#### API Objects

On the backend, registering a new bot (via `tctl`) creates a set of related API
objects:

- an object representing the bot itself
- a non-interactive User for the bot to act as (when user certs are requested)
- a role for the bot, which will contain the minimum set of permissions
  necessary to watch for CA rotations, as well as the ability to impersonate
  other roles
- a token that the bot can use to join the cluster for the first time

In order to discover related objects and ensure that no "zombie" items are left behind,
`tctl` will ensure that:

- the objects related to a bot all follow a standardized naming convention that
  includes the name of the bot
- a new `teleport.dev/owned-by` label is applied to all resources related to a
  particular bot

Note: end users should never need to manually create, modify, or delete these
objects. All operations on bots and their related objects is performed through
the `tctl bots` subcommand.

#### `tbot` configuration

The goal is to keep tbot configuration to a minimum to make for a quick getting
started experience. Ideally, all configuration is provided via the command line
and there is no tbot-specific configuration file. This has several advantages:

- Our users are less likely to mix up a tbot configuration file and a
  teleport.yaml.
- The command used to invoke tbot is all that is needed to run the same
  configuration. This makes it easy to troubleshoot or recreate a customer
  issue.

#### External Configuration Assist

In order to make it easy to consume certificates generated by `tbot`, the agent
will have a `tbot config` subcommand that can render configuration snippets
for common integration points.

For example:

```
$ tbot start ...
$ tbot config --ssh
To configure sshd to trust this host certificate, add the following to your sshd_config:

HostCertificate /foo/bar/tbot-cert.pub
```

(In the above, the instructional text will be written to stderr, so that the following
will be possible: `tbot config --ssh | sudo tee -a /etc/ssh/sshd_config`)

If `tbot` is managing multiple `--cert`s, you can pass `mode:destination`
to tell it which certificate to render configuration for.

Note: `tctl auth sign` has a tiny amount of configuration templating already for
Postgres and MongoDB certificates. We should aim to move this logic out into a
separate package that can render configuration snippets for a variety of
external systems. This work is out of scope for the purposes of this RFD.

#### Polling for expiration

In order to watch a certificate for expiration, the
`teleport/lib/utils/interval` package will be used. Note that tbot will only set
up polling for _user certs_, as host certificates issued by teleport do not
expire (host certificates *will* be re-issued during CA rotations).

#### Initial User Certificates

In order for the bot to obtain its first set of user certificates, a new
endpoint will be added to the API server at `tokens/register/user`. This
endpoint will mimic the host registration flow, accepting a pre-generated token
and a public key to sign, but will use user tokens instead of provisioning
tokens.

Today, user tokens can be used to initiate the user invite flow, handle password
resets, and manage the account recovery process. In order to generate initial
user certificates, we will introduce a new type of user token for certificate
renewal bots.

When the backend receives the request to generate new user credentials, it validates
the token, ensuring that the token is of the correct type and has not expired.

#### Renewable User Certificates

Teleport's current behavior when re-issuing a user certificate is maintain the
original expiration time and refuse to bump out the TTL. We plan to preserve
this behavior by default, and only allow for extending the expiration date on
certificates that have been marked renewable.

In order to mark a certificate as renewable, we'll include a new extension in
the certificate's subject field. This attribute will only be set by teleport
when the certificates are first issued using the new endpoint and user token.

#### Audit Log

The auth server will emit new events to the audit log when:

- a new renewable certificate is issued for the first time
- a certificate is renewed
- a bot is locked
- a bot is removed (likely due to failed heartbeating or expired certificates)