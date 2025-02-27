---
authors: Tim Buckley (<tim@goteleport.com>)
state: draft
---

# RFD 0205 - Improved On-Prem Bot Joining

## Required Approvers

- Engineering: @strideynet && @zmb3

- Product: @thedevelopnik

## What

This RFD proposes serveral improvements to better support non-delegated and
on-prem joining, particularly for Machine ID.

Primarily, we discuss a new `challenge` join method intended to replace the
traditional `token` join method for many use cases, but also proposes a number
of UX improvements to improve bot joining generally and `token` or
`challenge-response` joining in particular.

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

- How many bots will join?

- How can we tell joined bots apart? Can we trust a bot identity if it can be
  thrown away and regenerated?

- When a bot needs to rejoin, does it use the same token? Can that token *ever*
  expire?

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

### Challenge-Response Joining

TODO: Consider alternative join method names?

We believe a new join method, `challenge`, can meet our needs and provide
significantly more flexibility than today's `token` join method. This works by -
in a sense - inverting the token joining procedure: bots generate an ED25519
keypair, and the public key is copied to the server. The public key can be
copied out-of-band, or bots can provide their public key on first join using a
one-time use shared secret, much like today's `token` method.

Once the public key has been shared, bots may then join by requesting a
challenge from the Teleport Auth service and complete it by signing it with
their private key. If successful, the bot is issued a renewable identity just as
`token`-joined bots are today, and the bot will actively renew this identity for
as long as possible.

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
  `ProvisionToken` to increment `spec.challenge.rejoining.total_rejoins`. The
  failed `tbot` instance can then retry the joining process, and it will
  succeed.

It otherwise functions similarly to `token`-joined bots today. It proves its
identity - either via an onboarding secret or public key - to receive a
renewable identity and renews it as usual for as long as possible. The
generation counter is still used to detect identity reuse. When the internal
identity expires, the bot loses access to resources (until it reauthenticates).

#### Token Resource Example

`challenge`-type tokens differ from other types in that they are intended to
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

  join_method: challenge
	challenge:

	  # `onboarding` parameters control initial join behavior
  	onboarding:
  	  # If set, no joining secret is generated; the secret exchange ceremony is
  	  # skipped and instance will directly prove its identity using its private
  	  # key.
  	  public_key: null

  	  # If set, use an explicit initial joining secret; if both this and
  	  # `public_key` are unset, a value will be generated server-side and
  	  # stored in `.status.challenge.initial_join_secret`
  	  initial_join_secret: ""

  	  # Initial joining must take place before this timestamp. May be
  	  # modified if bot has not yet joined.
  	  expires: "2025-03-01T21:45:40.104524Z"

  	# Parameteres to tune rejoining behavior when the regular bot identity has
    # expired
  	rejoining:
  	  # If true, `total_rejoins` is ignored and bots may rejoin indefinitely;
  	  # must be opt-in.
  	  unlimited: false

  	  # Total number of allowed rejoins; this may be incremented to allow
  	  # additional rejoins, even if a bot identity has already expired. May
  	  # be decremented, but only by the current value of
  	  # `.status.challenge.remaining_rejoins`.
  	  total_rejoins: 10

  	  # If set, rejoining is only valid before this timestamp; may be
  	  # incremented to extend bot lifespan.
  	  expires: ""

status:
  challenge:
    # If `public_key` is unset, this value will be generated server-side and
    # made available here.
    initial_join_secret: <random>

    # The public key of the bot associated with this token, set on first join.
    bound_public_key: <data>

    # The current bot instance UUID. A new UUID is issued on rejoin; the previous
    # UUID will be linked via a `previous_instance_id` in the bot instance.
    bound_bot_instance_id: <uuid>

    # A count of remaining rejoins; if `.spec.challenge.rejoining.total_rejoins`
    # is incremented
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

The new `challenge` method improves this situation in two primary ways: one
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
    join_method = "challenge"

    challenge = {
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
    initial_join_token = each.value.status.challenge.initial_join_secret
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

Bots should be informed of their number of remaining rejoins. We can give bots
permission to view their own join token, or include the number of remaining
rejoins as an informational field in the bot's current user certificate. We
should then expose this as a Prometheus metric to allow for alerting if a bot

We should also investigate hardware key storage backends using HSMs / PKCS#11.
We have some prior art here in Teleport's [HSM
support](https://goteleport.com/docs/admin-guides/deploy-a-cluster/hsm/).

#### Non-Terraform UX

TODO

#### Remaining Downsides

- Repairing a bot that has exhausted all of its rejoins is still a semi-manual
  process. It is significantly easier, and does not necessarily require any
  changes on the impacted bot node itself, but is still annoying. Users can opt
  out of this by setting `.spec.rejoining.unlimited=true`, but this has obvious
  security implications.

- Effort required to configure IaC / Terraform is still fairly high, even if
  reduced.

### Other Supporting Improvements

Alongside `challenge` joining, we have several UX proposals to further improve
the usability of non-delegated joining.

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

We should make this more granular and make locks that can target a bot instance
UUID, and as this RFD introduces a method for bots to rejoin as a new instance,
a lock target for specific join tokens. We may need to introduce join token
tracking, e.g. by introducing a join token certificate field.

#### Token CLI UX Improvements

Instead of separating join method and token name, we propose combining the two
into a single CLI flag and referring to tokens as a (method, value) tuple. For
example:

``` 
$ tctl bots add example --roles=access
The bot token: challenge:04f0ceff1bd0589ba45c1832dfc8feaf
This token will expire in 59 minutes.

[...snip...]

$ tbot start --token=challenge:04f0ceff1bd0589ba45c1832dfc8feaf ...
```

## Alternatives and Future Extensions

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

## Rejected Alternatives

### N-Token Resiliency
