---
authors: Zac Bergquist (zac.bergquist@goteleport.com), Tim Buckley (tim@goteleport.com)
state: draft
---

# RFD TBD - Teleport Cert Renewal Bot

## What

RFD TBD defines the high-level goals and architecture for a new agent that can
continuously renew Teleport-issued certificates.

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
database, an OpenSSH server, or a CI/CD worker, for example).

This agent will join the Teleport cluster, request certificates, store them in a
user-specified destination, and then monitor the certificates and renew them as
necessary.

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
$ tbot start \
  --name=foo \
  --directory=/home/ubuntu/tbot \
  --auth-server=proxy.example.com:3080 \
  --token=13b74f49d27536dd5c514073097c197b \
  --ca-pin=sha256:...
  ...
```

Both direct dialing or connecting through a Teleport proxy are supported.

The bot joins the cluster and can be seen with the `tctl` command:

```
$ tctl bots ls
ID            NAME       LOCKED    ROLES
0123-4567     foo        false     dev,ssh
```

When the bot starts, it generates a new set of keys (private key and TLS/SSH
public keys), then uses the provided join token to request a signed, renewable
certificate from the auth server. These certificates are stored in a restricted
location, such as `/var/lib/teleport/bot`, and renewed continuously. They are
exclusively used by the bot for renewal and to issue secondary end-user
certificates. Secondary certificates are written to the user-specified location
and are useful for connecting to resources.

Note: the join token can only be used once, and is invalidated after first use
or when its TTL expires (whichever comes first). If the bot fails to renew its
certificates before they expire, it will be unable to authenticate with the
cluster and must re-join with a new token.

In the above example, as no additional client configuration has been specified,
the bot will generate only a single set of end-user certificates with access to
the union of all roles specified. Users with additional security requirements
may opt to issue one or more sets of certificates to different locations and
with access to a different set of roles.

For example, given the following command to create a bot:

```
$ tctl bots add --name=example --roles=a,b,c
```

... users may create a configuration file, such as `/etc/tbot.yaml`:

```yaml
destinations:
  - directory: /home/alice/.tbot
    roles: [a, b, c]

  - directory: /home/bob/.tbot
    roles: [b, c]
```

The user then starts the bot with the configuration file:

```
$ tbot start -c /etc/tbot.yaml
```

Once the bot fetches its own certificate, it then fetches additional
impersonated certificates for each specified certificate destinations and
writes them via the appropriate backend implementation. This may be
particularly useful for CI/CD use cases where untrusted workers run as a
particular user that should not be granted the full set of the bot's
privileges.

(TODO: how will we handle the advanced file permissions needed to make split
certificates useful? We can't recommend running the bot as root, but also need
each set of certificates to be scoped to just a single Unix user otherwise the
entire point is moot. Plus, destination files need appropriate Unix permissions
to keep tools like SSH happy. Perhaps Linux ACLs can help?)

### Security

It is important to consider the security implications of a credential that can
be continuously renewed. In order to minimize the blast radius of an attack, we
need to minimize both the scope and duration that an attacker could leverage a
compromised credential as well as encourage end users to deploy the bot in a
way that protects the certificates it generates.

We limit the scope by allowing users to define exactly which roles the
certificate should assume. The `tctl` tool will create a dedicated user and role
for the bot, and this role will be able to impersonate the roles specified by
the user. We encourage users to follow the principle of least privilege and
allow impersonation for the minimum set of roles necessary. Additionally, end
users may further limit the permissions granted to a particular set of
certificates should their workflow support this.

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

#### Impersonation

In a naive implementation, the bot might receive a single set of certificates
that would be both renewable and granted the union of all attached roles. If
these certificates were compromised, an attacker would have unfettered access
to cluster resources until either the certificates expire (assuming they don't
bother to renew them) or until a cluster admin notices and locks the bot.

We can mitigate these risks using impersonation. First, impersonation forces us
to split responsibilities for renewal and access. The bot would still be given
(and be responsible for continuously renewing) a user certificate, however this
certificate would be otherwise useless for end users as it would not provide 
access to any resources.

Next, users separately configure one or more certificate outputs. As part of
each renewal event, the bot generates an additional set of *non-*-renewable
certificates and stores them in the designated output. Each of these outputs
may optionally be scoped to a subset of all roles the bot is capable of
assuming.

This design is meant to encourage deployments where the bot's own data
directory, containing the renewable certificate, is separate and inaccessible
to other users on the system, via the native security methods of the output
destination (e.g. Unix permissions, Kubernetes RBAC, etc). The secondary
certificates may be written with relaxed permissions to enable use by end
users. If these secondary certificates are stolen, not only will they not be
renewable, but may also have only a subset of the permissions granted to the
bot.

Unfortunately, Teleport today does not fully support the impersonation UX we
would like to provide with the bot. As is, impersonation requires two `User`
resources: one for the impersonator, and one for the impersonatee. Today, we
could require that users manually create a Teleport user for each set of roles
the bot may assume, however this UX is undesirable. Therefore, we propose
adding the ability to, when requesting user certificates, allow the bot to
"impersonate" its own `User`, but with a set of additional roles as specified
in its certificate. This would enable use of the previously-described
`tctl bots add --roles=...` UX. If desired, this UX remains compatible with
other-user impersonation should we wish to add an additional `--users=...`
flag.

#### Renewals

There are several scenarios under which the bot will initiate a renewal:

1. When some fraction of a certificate's TTL has elapsed, for example 1/3 (and
   at most 1/2). A certificate valid for 3 hours should be renewed every hour
   to ensure at least one full additional renewal iteration can take place if
   the first fails.

   Tighter renewal intervals also help to ensure compromised certificates are
   discovered sooner (see "Preventing Certificate Propagation" below).
2. When a user and/or host CA rotation is taking place.
3. When a renewal is requested manually. For testing and debugging purposes, the
   bot trigger a renewal when it receives a `SIGUSR1` signal.

While the bot generates multiple sets of certificates - that is, one primary
renewable certificate and many secondary end-user certificates - they are all
renewed at roughly the same time, and secondary certificates have an expiration
matching the primary certificate. It may be possible in the future to have
secondary certificates with a shorter TTL than the primary (probably 1/n
durations) but this is out of scope for the initial implementation.

#### Preventing Certificate Propagation

Renewable certificates present an obvious security concern as they can be
renewed indefinitely, at least until a cluster admin decides to remove or lock
the bot user. Additionally, as renewing a certificate does not invalidate
previously-issued certificates, one compromised certificate can lead to an
unlimited number of indefinitely-renewable descendent certificates. To make
things worse, in Teleport today, this certificate propagation is somewhat
difficult to detect.

As a mitigation strategy, we propose adding a new certificate generation
counter (as a certificate extension) that increments each time a certificate
is renewed. This latest generation count is additionally stored on the auth
server as metadata for the bot [user? resource?].

On subsequent renewals, while generating user certificates on the auth server,
the current certificate is inspected and its current generation is compared to
the stored version on the auth server. If this generation counter is less than
the auth server's stored counter, this implies that multiple bots are competing
for renewals of the same certificate lineage.

Exact behavior for what to do next remains undetermined (TODO), but we have
several possible courses of action:
 * Immediately lock the bot. This has the downside of also breaking
   legitimate-but-accidental double-starts of a bot, if for example a user
   accidentally misconfigures their system. Users would then need to manually
   unlock the bot.
 * Simply deny the renewal. In practice, we can't know (short of some future
   HSM support) whether a particular renewal is malicious or not. This means
   that regardless of whether or not we lock the bot, the first client to renew
   the certificate gains access to the system for a fairly long period of time
   (and malicious users would of course do so immediately). One might speculate
   that a malicious user would not need a full hour to spawn a remote shell on
   every node the bot user can access.

   Presumably, legitimate users should notice that their certificate bot has
   failed to renew (this should be a very loud error). If they accidentally
   started the bot twice, there's at least a 50% chance that no further
   action is required on their end to resolve the issue (if the correct bot
   wins or if the start ordering didn't matter).

Another traceability enhancement would be to ensure renewable certificates
have a unique serial number. Upon renewal, we can embed the previous
certificate serial as another metadata / extension field. Should a certificate
be compromised, it will then be possible to trace the the tree of certificate
renewals. (*TODO (Tim): I suppose in practice the generation counter ensures
a flat hierarchy, so this may not be useful.*)

Lastly, regardless of the initial certificate's TTL, we can have the auth
server ensure that, when renewing, renewed certificate TTLs may only decrease
in length. This would prevent an attacker from stealing a renewable certificate
and immediately requesting a new certificate with the auth server's max
renewable TTL.

#### API Client Refresh

As of now, our [API client](./0010-api.md) is initialized with a set of TLS
credentials and expects those credentials be valid for the lifetime of the
client.

In order to continue communicating with the cluster, the bot will need to
inform the API client of the new credentials after a renewal. As part of this
effort, the client will be updated to allow for refreshing itself. This process
initializes a new client that attempts to connect with the new credentials, and
closes the original client when the new client is successfully connected with
the cluster.

```go
func (c *Client) Refresh(ctx context.Context) error {
    // use the existing config to generate a new client
    newClient, err := connect(ctx, c.c)

    c.Close() // close the original client

    *c = newClient
}
```

It will be the responsibility of the caller of `Refresh` to reinitialize any
watches or streams that the client may have been running prior to the refresh.

#### Artifacts

For internal use, the bot generates a set of renewable TLS certificates.
This set of credentials is only used certificate renewal, user certificate
generation, and establishing CA rotation watches. These artifacts should be
persisted to some private location to allow the bot to recover after a process
restart.

| Kind                   | Path         | Notes                            |
|------------------------|--------------|----------------------------------|
| TLS CA certificates    | `tlscacerts` | Teleport cluster CA certs (both) |
| TLS client certificate | `tlscert`    | Signed by Teleport `UserCA`      |
| Private key (TLS)      | `key`        |                                  |

The bot then generates a set of non-renewable credentials for each configured
credential destination. In the initial implementation, the following artifacts
may or may not be generated depending on the user's configuration (kind and
templates).

| Kind                   | Path          | Notes                                   | Kind | Config Template
| -----------------------|---------------|-----------------------------------------|------|---------
| Private key            | `key`         | Shared for both SSH and TLS uses        | Both | Always
| SSH public key         | `key.pub`     | Required for OpenSSH compatibility      | SSH  | Always
| SSH client certificate | `sshcert`     | Signed by Teleport `UserCA`             | SSH  | Always
| SSH known hosts        | `known_hosts` | Teleport `HostCA` certs, OpenSSH format | SSH  | `ssh-client`
| SSH client config      | `ssh_config`  | `Include`-ready SSH client config       | SSH  | `ssh-client`
| TLS CA certificates    | `tlscacerts`  | Teleport CA certificates (both)         | TLS  | Always
| TLS client certificate | `tlscert`     | Signed by Teleport `UserCA`             | TLS  | Always

Notes:
 * Users may optionally decide to disable persistence of the renewable
   certificate using the `memory` storage backend. This has security benefits
   but prevents the bot from recovering if stopped as the renewable credentials
   would only exist in memory.
 * The Teleport auth server's CA certificates are verified at first connect
   using CA pins, following Teleport's usual node joining procedure.
 * Unneeded certificate types/formats could be optionally disabled in
   `tbot.yaml` config if desired.
 * Additional artifacts may be written if users enable any app-specific config
   templates.
 * In the future, we could consider ephemeral certificate destinations for
   situations where certificates should not be written to disk (Webhooks or
   other IPC?)

#### Reload Command

The reload command is an optional command that `tbot` will execute after a
successful renewal. This can be used, for example, to restart an OpenSSH server
after new certificates are written to disk. Note: this command is executed with
Go's `os/exec` package, meaning that a system shell is not invoked, so shell
features such as glob patterns, redirection, or environment variable expansion
are not supported.

We will list a few example reload commands for common use cases (OpenSSH, NGINX)
in our docs.

*TODO (Tim)*: How will we ensure the bot is able to send signals / execute
commands in a way that doesn't require it to run as the target Unix user or as
root? Simple command exec is less useful than it appears when all the system 
components are deployed securely.

A few alternative / additional notification methods:
 * Filesystem watches (inotify, etc). Offloads complexity to the user and still
   requires solving the FS permission problem (Linux ACLs?).
 * Webhooks. Need to evaluate security.
 * Shell scripts. Will need to evaluate how useful this really is.
 * Unix signals. Require either same user, root, or `CAP_KILL`, or SUID, plus
   needs a way to query for the correct process. Not very practical for largely
   the same reason shell scripts aren't.
 * D-Bus? (grasping at straws)

We should implement as many of these as makes sense (minimally shell scripts,
and FS watches are a no-op on our end.)

#### Join Script

We intend to develop a bot join script, similar in functionality to the node
join script that exists in Teleport enterprise. While not required, this will
make it even easier to install and run the bot as a systemd service on the
target machine.

TODO: support for auto-join on AWS without token

### Implementation

The bot will be implemented as a standalone tool (ie under `tool/tbot`) and
_not_ as a service that runs in a `teleport` process.

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
started experience, however we recognize many use cases may quickly become
difficult to encode on the command line.

To that end, we'd like to follow the philosophy of "keep simple things simple
but make complex things possible": the simplest / most popular use cases should
ideally require minimal (zero?) configuration beyond a concise CLI, but more
complicated use cases can make use of a `tbot.yaml` configuration file.

An example `tbot.yaml` with all default / advanced options specified:

```yaml
# Storage for the bot's private data
storage:
  # can also support any backend supporting both store + fetch
  directory: /var/lib/teleport/bot

  # alternatively, we can store these certificates only in memory:
  # memory: {}

# Destinations specify one or more impersonated certificates sets to generate.
destinations:
  # `directory` is the only destination supported in the initial iteration;
  # the tbot process must be given permissions to write to this directory.
  - directory: /home/alice/tbot

    # One of more roles to grant to the bot. It must have been granted (at
    # least) these roles with `tctl bots add --roles=...`
    # By default, all possible roles are included.
    roles: [a, b, c]

    # Which types of certificates to generate. `ssh` is the default.
    kinds: [ssh]

    # A list of configuration templates to generate and write to the
    # destination. Defaults are decided based on the certificate kinds:
    # `ssh` implies `ssh-client` unless manually (not) configured.
    configs:
      # ssh-client generates known_hosts and an ssh_config that can be
      # included. We can ensure the correct certificate kinds are generated
      # while generating the config templates.
      - ssh-client

  - directory: /home/bob/tbot
    roles: [b, c]

    # TLS certificates can be generated for app / db access.
    kinds: [ssh, tls]

    configs:
      # Some future configuration templates may require parameters.
      # (This is an example; ssh-server may not need this or any parameters)
      - ssh-server:
          hostname: example.com
```

In future iterations, additional output destination backends may be considered.
A few ideas include:
 * Kubernetes secrets (to run a bot safely in k8s without requiring a PV)
 * Webhook delivery (or other local IPC methods) to keep certificates in memory
 * Ansible vault / Hashicorp vault / AWS secrets manager / other secret stores

*TODO*: how can a user specify DNS names to include in the certs?

#### External Configuration Assist

One of the more time consuming aspects of setting up mutual TLS is configuring
both sides to present the correct certificate and trust certificates signed by a
specific CA. We aim to simplify this configuration process in two ways:

1. Generating additional configuration files from a template as part of the
   renewal loop.

   Many applications require that certificates be formatted in non-standard
   ways, depend on additional parameters that require a Teleport API client
   to fetch, may change over time, or may require multiple files. OpenSSH
   client configuration alone meets most of these criteria.

   Users can specify which additional configuration files to generate in
   `tbot.yaml` (initially only `ssh-client` will be provided). Templates that
   require no additional configuration can be specified on the command-line.
2. Providing a `tbot config` helper command for "last-mile" configuration and
   instructions.

   For applications that can consume the bot's generated credentials directly,
   this outputs a minimal configuration example with brief instructions.

   For applications supported by a config template as described above, this
   generates a snippet to make use of the generated file, e.g.
   `Include <path to tbot ssh_config>` for SSH config.

For example, to configure an SSH client, first users will configure `tbot` to
generate an `ssh_config` file:

```yaml
destinations:
  - directory: /home/alice/tbot
    configs: [ssh-client]
```

When the user next starts the bot using this config, it will write an
`ssh_config` and a `known_hosts`, per the template:
```
$ tbot start -c tbot.yaml
```

Next, the user configures their SSH client to use the generated config:
```
$ tbot config ssh -c tbot.yaml
To configure your SSH client, add the following to your local SSH configuration
(~/.ssh/config):

Include /home/alice/tbot/ssh_config
```

Where possible, the informative text is written to stderr to allow easy
appending:

```
$ tbot config ssh -c tbot.yaml >> ~/.ssh/config
The following SSH config snippet was written to the pipe:

Include /home/alice/tbot/ssh_config
```

Notes:
 * This is a contrived example as the `ssh-client` template is always generated
   by default for `ssh`-typed certificates, which are themselves the default
   In other words, `ssh_config` and `known_hosts` are generated if no
   `tbot.yaml` is specified.
 * `tctl auth sign` has a tiny amount of configuration templating already for
   Postgres and MongoDB certificates. We should aim to move this logic out into
   a separate package that can render configuration snippets for a variety of
   external systems. This work is out of scope for the purposes of this RFD.

#### Polling for expiration

To watch a certificate for expiration, the `teleport/lib/utils/interval`
package will be used. The tbot will only set up polling for _user certs_, as
host certificates issued by teleport do not expire (host certificates *will* be
re-issued during CA rotations).

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

*TODO (Tim): Investigate adding a new gRPC endpoint for this functionality instead
of introducing a new endpoint on the deprecated HTTP API.*

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
- a certificate generation counter conflict is detected (certificate possibly compromised)
