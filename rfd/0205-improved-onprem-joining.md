---
authors: Tim Buckley (<tim@goteleport.com>)
state: draft
---

# RFD 0205 - Improved On-Prem Bots with Bound Keypair Joining

## Required Approvers

- Engineering: @strideynet && @zmb3

- Product: @thedevelopnik

## What

This RFD proposes several improvements to better support non-delegated and
on-prem joining, particularly for Machine ID.

Primarily, we discuss a new `bound-keypair` join method intended to replace the
traditional `token` join method for many use cases, but also proposes a number
of related UX improvements to improve bot joining generally and `token` and
`bound-keypair` joining in particular.

## Why

Today, if some form of delegated joining is not available, bots must fall back
to the traditional `token` join method. This join method simple and universal:
it has effectively zero hardware or software requirements, works with any (or
no) cloud provider, and is perfect for demos and experimentation. It's also
relatively secure: single use tokens ensure a Teleport admin user is directly
involved with each bot join, and generation counter checks help ensure bot
identities are difficult to exfiltrate unnoticed.

Unfortunately, that's a fairly exhaustive list of positives, and when used in a
production environment, bot token joining has major operational problems:

- Onboarding scales poorly: joining a large fleet of bot instances means
  provisioning a token for each bot, which also means generating secrets
  yourself, and distributing them appropriately.

- Ongoing maintenance requires manual intervention, breaking IaC principles. We
  should assume `token` joined bots will inevitably fail at some point, and when
  this happens, a new token must be issued - manually.

- Internal bot identities have a hard 24 hour TTL limit, limiting the maximum
  possible resiliency to 24 hours before a bot can no longer rejoin without
  manual human intervention

- `token`-method tokens are themselves secret values and their names need to be
  treated carefully to avoid leaking secrets

- Bots occasionally trigger generation counter lockouts, killing themselves and
  any instances on the same bot

- When a bot instance's renewable identity expires, there's no clear way to tell
  that it stopped functioning other than checking the `tbot` process directly.

These limitations led to a surprisingly narrow set of use cases where token
joining was really a *good* experience, effectively just:

- Experimentation and development use. It's simple and comprehensible which
  makes it great for use in documentation - `tctl bots add` even gives you the
  command to run to start a bot immediately!

- Running very few, very reliable, long-lived systems. If you can reasonably
  expect your system to never go down for more than 24 hours, bots can happily
  run for months.

  (Ironically, Kubernetes is a great environment in which to run `token`-joined
  bots since it'll rapidly reschedule any bot deployments that fail... but we
  have a dedicated `kubernetes` delegated join method.)

In short, token joining has a complexity cliff. It's extremely easy to get
started, but it can feel like a false start when users learn token joining is
not suitable to their production use case. At best it's back to the docs to
learn about some more complicated join method; at worst, it's even more
disappointing when users learn there simply is _no_ good method for on-prem
joining. (Well, unless they have TPMs.)

End users willing to create their own automation around token issuance could
work around some of these limitations, but this creates an unnecessary barrier
to entry for use of Machine ID on-prem.

## Details

### Context: Priorities and Security Invariants

It's become clear through conversations with customers and previous attempts at
solving this problem that some of these pain points are contradictory. As a case
study, an ideal UX from an end user's perspective might be to create one token,
join all their bots with it, and leave them running indefinitely. This would
create several issues:

- The initial joining secret would have an ambiguous lifetime. At what point
  does this multi-use token expire?

- How many times will the token be used? Can we trust it's never been used
  improperly, and that each use actually originated from the infrastructure we
  intended to join?

- How can we tell joined bots apart, even over time? If the original joining
  token is still valid, could a malicious bot purge its identity and rejoin?

- When a bot needs to rejoin, does it use the same token? If so, can that token
  *ever* expire?

With this in mind, we need to strike some balance between effective UX and a
system we can trust to not allow unauthorized or unintended access. To that end,
we'll focus on these explicit compromises we believe improve today's UX while
making minimal security concessions:

- **There must be a 1:1 relationship between a `ProvisionToken` and a bot
  instance.** Allowing many joins on one token creates unnecessary backend
  contention and - depending on implementation - creates severe traceability
  problems.

  However, we can greatly improve today's automation story. Secret fulfillment
  can take place server side to reduce the number of resources to generate in
  Terraform, and `ProvisionToken` resources can be made reusable to fully enable
  IaC workflows.

- **Bot credentials can be long-lived, but their *trust* must be controlled and
  renewed.** We can't allow any secret values to either remain both valid and
  useful for a long time, or allow currently valid identities to generate
  credentials that can be used for future, unchecked extension of access.

  However, we can create a new state where an identity is technically valid
  indefinitely, but useless unless explicitly allowed by the server. Existing
  controls like the generation counter can still effectively prevent unintended
  reuse of the bot identity.

### Bound Keypair Joining

We believe a new join method, `bound-keypair`, can meet our needs and provide
significantly more flexibility than today's `token` join method. This works by -
in a sense - inverting the token joining procedure: bots generate an ED25519
keypair, and the public key is copied to the server. The public key can be
copied out-of-band, or bots can provide their public key on first join using a
one-time use shared secret to authenticate the exchange, much like today's
`token` method.

Once the public key has been shared, bots may then join by requesting a
challenge from the Teleport Auth service and completing it by signing it with
their private key. If successful, the bot is issued a renewable identity just as
`token`-joined bots are today, and the bot will actively renew this identity for
as long as possible, or until its backing token expires.

If the identity renewal fails at any point, bots may attempt to reauthenticate,
and the Auth service can use predefined per-bot rules to decide if this specific
bot is allowed to rejoin, including a rejoin counter and expiration date. If a
rejoin is rejected, the bot's identity does not necessarily remain invalid: if
server-side rules are adjusted, for example by increasing the token's rejoin
limit, it can then rejoin without any client-side reconfiguration.

This has several important differences to existing join methods:

- Onboarding secrets are optional, and the secret exchange process may be
  skipped if the `ProvisionToken` is configured with a public key directly.
  Otherwise, joining bots authenticate with an onboarding secret to
  automatically share their public key with the server.

- When joining or rejoining, Teleport issues a challenge that the client must
  solve. This is similar to TPM joining today, but backed by a local keypair
  rather than (necessarily) a hardware token.

- When a bot's identity expires, assuming it has some rejoin allocations left,
  it can simply repeat the joining process to receive a fresh renewable
  certificate.

- If a bot exhausts its rejoining limit, it will not be able to fetch new
  certificates, similar to today's behavior. However, this bot can be restored
  without needing to generate a new identity: an admin user can edit the backing
  `ProvisionToken` to increment `spec.bound_keypair.rejoining.total_rejoins`.
  The failed `tbot` instance can then retry the joining process, and it will
  succeed.

It otherwise functions similarly to `token`-joined bots today:

- It is still fully infrastructure agnostic and works across operating systems.

- The joining UX is largely compatible with `token` joining and should still
  work great for experimentation and documentation examples.

- It proves its identity - either via an onboarding secret or public key - to
  receive a renewable identity and renews it as usual for as long as possible.

- When the internal identity expires, the bot loses access to resources until it
  reauthenticates.

- The generation counter is still used to detect identity reuse.

#### Joining UX Flows

This join method creates two new joining flows:

1. **Static Binding**: A keypair is pregenerated on the client and the public
   key is directly included in the token resource by a Teleport admin.

   Example UX (subject to change):

   ```
   $ tbot generate-keypair
   Wrote id_ed25519
   Wrote id_ed25519.pub
   $ tctl bots add example --public-key id_ed25519.pub
   $ tbot start identity --token=bound-keypair:id_ed25519
   ```

   (In this example, `tctl bots add` creates a `bound-keypair` token automatically,
   much like a `token`-type token is created automatically today.)

   The public key can be copied as needed, similar to SSH `authorized_keys` and
   GitHub's SSH authentication. This is arguably more secure since no secret is
   ever copied.

   On startup, Auth issues a challenge to the bot which is solved with its
   private key, and it receives a standard renewable identity.

2. **Bind-on-join**: The `tbot` client is given a joining secret.

   Example UX:
   ```
   $ tctl bots add example --join-method=bound-keypair
   The bot token: bound-keypair:04f0ceff1bd0589ba45c1832dfc8feaf
   This token will expire in 59 minutes.
   $ tbot start identity --token=bound-keypair:04f0ceff1bd0589ba45c1832dfc8feaf
   ```

   On `tbot` startup, a keypair is transparently generated and exchanged with
   Auth, after which the bot internally behaves as if flow 1 was used, and the
   now-bound keypair perform its first full join.

   From an end user's PoV, this process is nearly identical to traditional
   `token` joining. While a joining secret does need to be copied to the bot
   node, these secrets remain short-lived and one-time use.

We expect most users to use Flow 2: it's much easier to provision new nodes and
requires less back-and-forth between the admin's workstation and bot node. Flow
1 is particularly ill-suited to Terraform use since keypairs would need to be
pregenerated and copied to nodes, which is not ideal from a security PoV.

Flow 2 is also mostly equivalent to `token` joining. Current users will already
be conceptually familiar with the joining process, and documentation updates
will be minimal.

#### Token Resource Example

`bound-keypair`-type tokens differ from other types in that they are intended to
have no resource-level expiration (though that is allowed), are meant to have
their spec modified over time by users or automation tools, and publish
information about their current state in the immutable (to users) `status`
field.

``` yaml
kind: token
version: v2
metadata:
  name: my-join-token
spec:
  bot_name: example

  join_method: bound-keypair
  bound_keypair:

    # `onboarding` parameters control initial join behavior
    onboarding:
      # If set, no joining secret is generated; the secret exchange ceremony is
      # skipped and instance will directly prove its identity using its private
      # key. It is an error for a public key to be associated with more than one
      # token, and creation or update will fail if a public key is reused.
      public_key: null

      # If set, use an explicit initial joining secret; if both this and
      # `public_key` are unset, a value will be generated server-side and
      # stored in `.status.bound_keypair.initial_join_secret`
      initial_join_secret: ""

      # Initial joining must take place before this timestamp. May be
      # modified if bot has not yet joined.
      expires: "2025-03-01T21:45:40.104524Z"

    # Parameters to tune rejoining behavior when the regular bot identity has
    # expired
    rejoining:
      # If true, `total_rejoins` is ignored and bots may rejoin indefinitely;
      # must be opt-in.
      unlimited: false

      # Total number of allowed rejoins; this may be incremented to allow
      # additional rejoins, even if a bot identity has already expired. May
      # be decremented, but only by the current value of
      # `.status.bound_keypair.remaining_rejoins`.
      total_rejoins: 10

      # If set, rejoining is only valid before this timestamp; may be
      # incremented to extend bot lifespan.
      expires: ""

status:
  bound_keypair:
    # If `spec.onboarding.public_key` is unset, this value will be generated
    # server-side and made available here. If
    # `spec.onboarding.initial_join_secret` is set, its value will be copied
    # here.
    initial_join_secret: <random>

    # The public key of the bot associated with this token, set on first join.
    # The bound public key must be unique among all `bound-keypair` tokens;
    # token resource creation/update or bot joining will fail if a public key is
    # reused.
    bound_public_key: <data>

    # The current bot instance UUID. A new UUID is issued on rejoin; the previous
    # UUID will be linked via a `previous_instance_id` in the bot instance.
    bound_bot_instance_id: <uuid>

    # A count of remaining rejoins; if `.spec.bound_keypair.rejoining.total_rejoins`
    # is incremented, this value will be incremented by the same amount. If
    # decremented, this value cannot fall below zero.
    remaining_rejoins: 10
```

#### Terraform Example

This join method is explicitly designed to be used with Terraform and IaC
workflows. Today, it's possible to generate one or more secret values, compute
an expiration time, provision a token for each secret, and spawn many VM
instances with that token passed along via user data. While a bit verbose, this
initial deployment workflow is mostly sane. For an example, refer to [this
documentation
snapshot](https://github.com/gravitational/teleport/blob/cb0a69d09550e45c2c327ab7dcc6a023e3bb162a/docs/pages/reference/terraform-provider/resources/bot.mdx#example-usage)
with a working example.

However, the critical issue in this workflow is in maintenance: if any one of
these bots expires, there is no reasonable method to restore it short of
manually issuing a new join token (`tctl bots instance add foo`), connecting to
the node, and manually copying the new token into the bot config.

The new `bound-keypair` method improves this situation in two primary ways: one
fewer resource needs to be generated (the secret value), and maintenance can be
performed simply by adjusting values in the resource. If a bot fails, its rejoin
counter can be incremented easily.

As an example, we can consider provisioning several bots. We'll need to account
for future overrides so we can fix a single bot in the future if needed:

``` hcl
locals {
  nodes = toset(["foo", "bar", "baz"])
}

resource "teleport_bot" "example" {
  name = "example"
  roles = ["access"]
}

variable "bot_rejoin_overrides" {
  type = map(number)
  default = {
    foo = 5
  }
}

resource "teleport_provision_token" "example" {
  for_each = local.nodes

  version = "v2"
  metadata = {
    name = "example-${each.key}"
  }
  spec = {
    roles = ["Bot"]
    bot_name = teleport_bot.example.name
    join_method = "bound-keypair"

    bound_keypair = {
      rejoining = {
        # look up node-specific count in the rejoin overrides map, default to 2
        total_rejoins = lookup(var.bot_rejoin_overrides, each.key, 2)
      }
    }
  }
}

resource "aws_instance" "example" {
  for_each = teleport_provision_token.example

  ami           = "ami-12345678"
  instance_type = "t2.micro"

  user_data = templatefile("${path.module}/user-data.tpl", {
    initial_join_token = each.value.status.bound_keypair.initial_join_secret
  })
}
```

In this example, if node `bar` uses its 2 renewals, we can add a new entry for
it in `bot_rejoin_overrides` and its `ProvisionToken` will be updated to allow
additional renewals.

#### Challenge Ceremony

The challenge ceremony will take inspiration from several existing join methods.
We can create an interactive challenge similar to TPM joining and present a
challenge containing a nonce. To avoid completely implementing our own
authentication ceremony, clients can use `go-jose` to marshal and sign a JWT
which can then be verified easily on the server.

TODO: This needs significant further elaboration and feedback.

#### Client-Side Changes in `tbot`

Bots should be informed of their number of remaining rejoins. There's a few
methods by which we could inform bots of their remaining rejoins:

1. (Recommended) Heartbeats: bots submit heartbeats at startup and on a regular
   interval. It would be trivial to include a remaining rejoin counter in the
   (currently empty) heartbeat response.

2. Certificate field: we could include the number of remaining rejoins in a
   certificate field.

3. New RPC: we could add a new RPC for bots to fetch this, alongside any other
   potentially useful information.

4. We could grant bots permission to view their own join tokens. There is
   precedent here as bots can view e.g. their own roles without explicitly
   having RBAC permissions to do so.

The remaining rejoin counter should then be exposed as a Prometheus metric to
allow for alerting if a bot drops below some threshold.

Importantly, this is a potentially lagging indicator. The design allows for the
rejoin counter to be decreased (to zero) at any time, so a rejoin attempt may
still fail at any time. This should be acceptable since it can also be increased
after the fact to restore access if desired.

#### Keystore Storage Backends

We should support abstract keystore storage backends to enable storage methods
beyond plain file storage.

For example:

- HSM storage, for hardware supporting PKCS#11. This includes many TPM 1.2 / 2.0
  implementations. We have some prior art here in Teleport's [HSM
  support](https://goteleport.com/docs/admin-guides/deploy-a-cluster/hsm/).

- [Apple Secure Enclave key storage][enclave]. This would require additional
  changes to our release process, as access to this functionality requires app
  signing. We again have prior art here with `tsh`.

Further evaluation will be necessary to ensure these backends support our
challenge process and key types. Libraries like [`sks`] provide compatibility
across TPM 2.0 (Windows, Linux) and Apple's Secure Enclave, and should be able
to sign our challenges appropriately, and using our desired key types.

[enclave]: https://developer.apple.com/documentation/security/protecting-keys-with-the-secure-enclave
[`sks`]: https://github.com/facebookincubator/sks

#### Non-Terraform UX

This proposal also aims to improve the non-Terraform UX, particularly when
automating with `tctl`. All regular token management workflows with
`tctl create -f` will continue to work; upserting resources to modify runtime
values will, for example, properly increase
`status.bound_keypair.remaining_rejoins` while preserving other token fields
like `status.bound_keypair.bound_public_key`.

Additional `tctl` changes will include:

- Once satisfied with behavior of the join method, replacing the default
  automatically generated join tokens for `tctl bots add` and
  `tctl bots instances add` to use this new join method.

- Adding a column for "rejoins remaining" in `tctl bots instances ls` (where
  relevant).

- Adding support for updating `total_rejoins` in `tctl bots update`

#### Expiration Alerting UX

A major deficiency in `token` joining is that bots fail silently. Their status
can be partially monitored via `tctl get bot_instance` but this does not
effectively notify administrators when something has gone wrong.

We should take steps to improve visibility of bots at or near expiry, including:

- Configurable cluster alerts when the number of available renewals has crossed
  some threshold

- Exposing the number of available renewals in the web UI and `tctl bot ls`

- Exposing per-token renewal counts as Prometheus metrics, both on the Auth
  Service and via `tbot`'s metrics endpoint.

#### Outstanding Issue: Soft Bot Expiration

The `spec.rejoining.expires` field can be used to prevent rejoining after a
certain time, in tandem with the rejoin count limit. This has the - likely
confusing - downside that bots will still be able to renew their certificates
indefinitely past the expiration date, assuming their certs were valid at the
time of expiration.

Also, as with all Teleport resources, the `metadata.expires` field can also
remove the token resource after a set time. Bots will also continue to renew
certs as long as possible until they are either locked or otherwise fail to
renew their certs on time.

These two expirations create some confusion, and do not allow for an obvious
method to deny a bot access, aside from creating a lock.

We would like to solve two expiration use cases:
1. We should be able to prevent all bot resource access after a certain date,
   including renewals, in a way that allows the bot to be resumed later if
   desired. (I.e. the token resource must still exist.)

   Locks may accomplish this, but some centralized management in the token
   resource would be convenient.

2. We should be able to prevent bot rejoins after a certain date, to control
   rejoining conditions in tandem with the rejoin counter. The
   `spec.rejoining.expires` field accomplishes this, but does have a naming
   collision with `metadata.expires`.

TODO: Ensure this is solved and not confusing.

#### Keypair Rotation

Given the long-lived nature of the keypair credential, it's important to support
rotation without bot downtime. Ideally, it should be possible to initiate a
rotation from either the server (e.g. by setting a `rotate_on_next_renewal` flag
on the token/bot instance) or `tbot` client.

TODO: Expand this.

#### Remaining Downsides

- Repairing a bot that has exhausted all of its rejoins is still a semi-manual
  process. It is significantly easier, and does not necessarily require any
  changes on the impacted bot node itself, but is still annoying. Users can opt
  out of this by setting `.spec.rejoining.unlimited=true`, but this has obvious
  security implications.

- Effort required to configure IaC / Terraform is still fairly high, even if
  reduced.

### Other Supporting Improvements

Alongside `bound-keypair` joining, we have several UX proposals to further
improve the usability of non-delegated joining.

#### Longer-Lived Bots

The renewable identity's 24 hour maximum TTL is too restrictive and should be
lengthened. We propose raising this limit to 7 days, but keeping the default
(1hr) the same.

#### Bot Instance Locking

When we introduced bot instances in
[RFD0162](0162-machine-id-token-join-method-bot-instance.md), it allowed many
bot instances to join under a single bot (and Teleport user). Generation counter
checks were moved out of a bot's user and into bot instances, however when a
generation counter mismatch occurs, the resulting lock is still filed against
the user as a whole. This means a generation counter lockout in one instance can
easily impact all other instances under the same bot.

We can introduce several new lock targets to address this:

- Bot instance UUID locking: prevent access by a particular instance. This will
  not lock bots that have since reauthenticated and received a new bot instance
  UUID.

- Join token locking: locks all bot instances that joined with a particular
  token. This may require introduction of a new certificate field to track the
  exact join token used.

- Public key locking: locks bots joining with a particular public key. A
  compromised bot could theoretically generate a fresh keypair so a join token
  lock is the primary locking solution, however this will prevent joining with
  another token.

#### Bot Joining URIs

Instead of separate flags for proxy, join method, token value, and other
potential join-method specific parameters, we propose adopting joining URIs.
These would provide a single value users can copy to pre-fill various
configuration fields and would greatly improve the onboarding experience.

The URI syntax might look like this:
```
tbot+[auth|proxy]://[join method]:[token value]@[addr]:[port]?key=val&foo=bar
```

Consider these two equivalent commands:
```
$ tbot start identity --proxy-server example.teleport.sh:443 --join-method bound-keypair --token example

$ tbot start identity tbot+proxy://bound-keypair:example@example.teleport.sh:443
```

Joining URIs can greatly simplify the regular onboarding experience by providing
a single value to copy when onboarding a bot:

```
$ tctl bots add example --roles=access
The bot token: tbot+proxy://bound-keypair:example@example.teleport.sh:443
This token will expire in 59 minutes.

[...snip...]

$ tbot start identity tbot+proxy://bound-keypair:example@example.teleport.sh:443 ...
```

Given the CLI now supports many operational modes, it's much easier for users to
write their given starting command (e.g. `tbot start app`) and paste the joining
URI to get started immediately.

URL paths and query parameters may also provide options for future extension if
desired.

## Future Extensions and Alternatives

### Agent Joining Support

We should explore expanding this join method to cover regular Teleport agent
joining as well as bots, as a more secure alternative to static or long-lived
join tokens.

### Additional Keypair Protections

We should investigate supporting additional layers of protection for the private
key. There are several avenues for this, depending on storage backend:

- Filesystem storage can be encrypted at rest and require a private key to be
  entered to unlock it, similar to SSH keys without an agent.

- Secure Enclave storage can require that the device be unlocked, or require
  biometric verification.

- Other HSM-stored keys may support various types of human presence
  verification. For example, YubiHSM has a touch sensor that can be required for
  access on a key-by-key basis.

#### Tightly Scoped Token RBAC

To better support use cases where central administrators vend bot tokens for
teams, we can add scoped RBAC support for `ProvisionToken` CRUD operations.

For example, this would allow a designated team to update a `bound-keypair`
token to increase the rejoin counter without needing to reach out to the central
administrator.

This is likely dependent on [Scoped RBAC](https://github.com/gravitational/teleport/pull/38078),
which is still in the planning stage.

### Explicitly Insecure Token Joining

There are perfectly valid use cases for allowing relatively insecure access to
resources that do not have strict trust requirements, and Teleport's RBAC system
is robust enough to only allow these bots access to an acceptable subset of
resources. It may be worthwhile to add an `insecure-shared-secret` join method
that allows for arbitrary joining in use cases that still fall through the
cracks, so long as end users understand the security implications.

### Client-side multi-token support

A simpler variant of N-Token Resiliency, this would allow `tbot` clients to
accept an ordered list of joining token strings which could be used
sequentially. If the internal identity expires, the next token in the list will
be used to attempt a rejoin.

This may be interesting for users with workload-critical bots wishing to hedge
against in outage in a delegated join method's IdP. With Workload ID being used
to authenticate e.g. database connections, this might be a worthwhile future
addition.

### Alternative: State in Bot Instances

We could alternatively store state in bot instances, rather than the token
resource.

To some extent this better matches current Teleport behavior today. Bot instance
resources already manage quite a bit of backend state and track recent
authentications, and there isn't much precedent for state to be actively managed
in provision tokens themselves.

On the other hand, bot instances are created automatically and are not generally
edited by user - though there's no compelling reason this can't be the case.

In practice, the best argument for keeping state in the provision token is
probably that we may wish to enable node joining with this method in the future.

## Rejected Alternatives

### N-Token Resiliency

This alternative built on top of the existing `token` join method by providing
bots with additional secrets they could use if their identity expired. Users
could select their desired level of resiliency by selecting the number of backup
tokens a bot would receive, thus the name.

This idea still has some merit but we realized this can largely be simplified
into the bind-on-join flow described above. Multiple secrets mainly served to
constrain credential reuse by limiting the number of possible rejoins until a
human has to take some action.

Bound keypair joining replaces the secrets with a rejoin counter, and allows for
(among other things) resuscitation of dead bots since their credentials remain
available even once expired.

A lighter weight alternative here could be client-side multi-token support as
described in the alternatives above.
