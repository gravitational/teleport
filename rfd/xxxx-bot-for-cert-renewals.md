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
$ tbot start \
  --name=foo \
  --output=file:/home/ubuntu/.tbot \
  --auth-server=proxy.example.com:3080 \
  --token=13b74f49d27536dd5c514073097c197b \
  --ca-pin=sha256:...
  ...
```

Note that both direct dialing or connecting through a Teleport proxy are
supported.

The bot joins the cluster and can be seen with the `tctl` command:

```
$ tctl bots ls
ID            NAME       LOCKED    ROLES
0123-4567     foo        false     dev,ssh
```

When the bot starts, it generates a set of keys, and uses the provided join
token to request a signed, renewable certificate from the auth server. These
certificates are stored in a restricted location, such as `/var/lib/tbot`, and
renewed continuously. They are exclusively used by the bot for renewal and to
issue secondary end-user certificates. Secondary certificates are written to
the user-specified location and are useful for connecting to resources.

Note: the join token can only be used once, and is invalidated after first use
or when its TTL expires (whichever comes first). If the bot fails to renew its
certificates before they expire, it will be unable to authenticate with the
cluster and must re-join with a new token.

In the above example, as no additional client configuration has been specified,
the bot will generate only a single set of end-user certificates with access to
the sum of all roles specified. Users with additional security requirements may
opt to issue one or more sets of certificates to different locations and with
access to a different set of roles.

For example, given the following command to create a bot:

```
$ tctl bots add --name=example --roles=a,b,c
```

... users may create a configuration file, such as `/etc/tbot.yaml`:

```yaml
sinks:
  - directory: /home/alice/.tbot
    roles: [a, b, c]

  - directory: /home/bob/.tbot
    roles: [b, c]

  - kubernetesSecret:
      kubeconfig: /home/user/kubeconfig
      namespace: example
      secretName: example-secret
    roles: [c]

  - webhook: http://localhost:8001
    roles: [a]
```

The user then starts the bot with the configuration file:

```
$ tbot start -c /etc/tbot.yaml
```

Once the bot fetches its own certificate, it then fetches additional
impersonated certificates for each specified certificate sink and writes them
via the appropriate backend implementation. This may be particularly useful for
CI/CD use cases where untrusted workers run as a particular user, or as
Kubernetes tasks.

(TODO: how will we handle the advanced file permissions needed to make split
certificates useful? We can't recommend running the bot as root, but also need
each set of certificates to be scoped to just a single Unix user otherwise the
entire point is moot, and with appropriate Unix permissions to keep tools like
SSH happy. Perhaps Linux FACLs can help?)

### Security

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

#### Impersonation

In a naive implementation, the bot might receive a single set of certificates
that would be both renewable and granted the sum of all attached roles. If
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
sink (e.g. Unix permissions, Kubernetes RBAC, etc). The secondary certificates
may be written with relaxed permissions to enable use by end users. If these
certificates are stolen, not only will they not be renewable, but may also have
only a subset of the permissions granted to the bot.

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

1. When a certificate is nearing expiry. A certificate is considered near
   expiration if 75% of its TTL has elapsed, or when there are 4 hours or less
   until expiration (whichever is sooner).
2. When a user and/or host CA rotation is taking place.
3. When a renewal is requested manually. For testing and debugging purposes, the
   bot will expose an API endpoint that can be used to trigger a renewal. This
   API will be accessible on the loopback interface only.

While the bot generates multiple sets of certificates - that is, one primary
renewable certificate and many secondary end-user certificates - they are all
renewed at roughly the same time, and secondary certificates have an expiration
matching the primary certificate. It may be possible in the future to have
secondary certificates with a shorter TTL than the primary (probably 1/n
durations) but this is out of scope for the initial implementation.

*TODO (Tim):* I think the renewal interval should be some fraction of the total
TTL (at most 1/2) rather than 75%. For example, given a 4 hour TTL, we should
attempt to renew certificates every 1 or 2 hours. This allows at least 1 full
renewal interval's worth of time in the event that renewals fail.

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
for renewals.

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

### Certificate Specification

The bot will be configured with one or more certificate specs, which can be
supplied on the command line in the format of:

```
--cert=MODE,DESTINATION,RELOAD
```

*TODO*: how can a user specify DNS names to include in the certs?

*TODO* (Tim): Non-filesystem destinations are likely to be very difficult to
encode as CLI flags (e.g. k8s secrets). Perhaps the CLI should only support
local FS and other sinks should defer to `tbot.yaml`?

#### Mode

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

#### Destination

The destination tells the bot where to store the certificates it generates. The
syntax for a destination specifier is `type:location`. Initially, the only
supported type will be `dir` which indicates that the certificates should be
placed in a directory on the local filesystem. In the future, we may support
additional destinations such as Kubernetes secrets, credential managers, or
CI/CD systems. If multiple `--cert` specifiers are provided, the bot will check
for overlapping destinations on startup, and fail fast if this is the case.

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
components are deployed securely. Perhaps users can rely on FS change
notifications (e.g. inotify) and we can provide examples using them.
Alternatively, we can deliver webhooks to another running service on the
system.

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

*TODO (Tim):* Per learnings from the demo, generating config may require an
active API client (auth, proxy, etc). E.g. SSH does to generate `known_hosts`
and to fetch the proxy address/port/etc. Perhaps it might make sense (where
it makes sense) to generate these during the renew loop and configure them via
`tbot.yaml`?

#### Join Script

We intend to develop a bot join script, similar in functionality to the node
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
started experience, however we recognize many use cases may quickly become
difficult to encode on the command line.

To that end, we'd like to follow the philosophy of "keep simple things simple
but make complex things possible": the simplest / most popular use cases should
ideally require minimal (zero?) configuration beyond a concise CLI, but more
complicated use cases can make use of a `tbot.yaml` configuration file.

An example `tbot.yaml` with all advanced options specified:

```yaml
# Storage for the bot's private data
storage:
  # can also support any backend supporting both store + fetch
  directory: /var/lib/tbot

sinks:
  - directory: /home/alice/.tbot
    roles: [a, b, c]

  # TODO: probably need to specify permissions (this will be ugly)
  - directory: /home/bob/.tbot
    roles: [b, c]

  # k8s secrets support store and fetch
  - kubernetesSecret:
      kubeconfig: /home/user/kubeconfig
      namespace: example
      secretName: example-secret
    roles: [c]

  # Webhooks support only store
  - webhook: http://localhost:8001
    roles: [a]
```

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
- a certificate generation counter conflict is detected (certificate possibly compromised)
