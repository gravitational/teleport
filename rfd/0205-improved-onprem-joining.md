---
authors: Tim Buckley (<tim@goteleport.com>)
state: implemented
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
  possible resiliency to 24 hours before a bot can no longer recover without
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
  token is still valid, could a malicious bot purge its identity and recover?

- When a bot needs to recover, does it use the same token? If so, can that token
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
in a sense - inverting the token joining procedure: bots generate a keypair,
using the cluster signature algorithm (probably ED25519) and the public key is
copied to the server. The public key can be copied out-of-band, or bots can
provide their public key on first join using a one-time use shared secret to
authenticate the exchange, much like today's `token` method.

Once the public key has been shared, bots may then join by requesting a
challenge from the Teleport Auth service and complete it by signing it with
their private key. If successful, the bot is issued a nonrenewable identity
similar to our existing delegated join methods, and the bot will actively
refresh this identity for as long as possible, or until its backing token
expires.

If the identity refresh fails at any point, bots may attempt to recover, and the
Auth service can use predefined per-bot rules to decide if this specific bot is
allowed to recover, including a join counter and expiration date. If a join
attempt is rejected, the bot's keypair does not necessarily remain invalid: if
server-side rules are adjusted, for example by increasing the token's join
allowance, it can then recover without any client-side reconfiguration.

This has several important differences to existing join methods:

- Onboarding secrets are optional, and the secret exchange process may be
  skipped if the `ProvisionToken` is configured with a public key directly.
  Otherwise, joining bots authenticate with an onboarding secret to
  automatically share their public key with the server.

- When joining, Teleport issues a challenge that the client must solve. This is
  similar to TPM joining today, but backed by a local keypair rather than
  (necessarily) a hardware token.

- When a bot's identity expires, assuming it has a nonzero join allowance,
  it can simply repeat the joining process to receive a fresh certificate.

- If a bot exhausts its joining allowance, it will not be able to fetch new
  certificates, similar to today's behavior. However, this bot can be restored
  without needing to generate a new identity: an admin user can edit the backing
  `ProvisionToken` to increment `spec.bound_keypair.recovery.limit`.
  The failed `tbot` instance can then retry the joining process, and it will
  succeed.

It otherwise functions similarly to `token`-joined bots today, despite being
implemented as a delegated joining method:

- It is still fully infrastructure agnostic and works across operating systems.

- The joining UX is largely compatible with `token` joining and should still
  work great for experimentation and documentation examples.

- It proves its identity - either via an onboarding secret or public key - to
  receive a nonrenewable identity and refreshes it as usual for as long as
  possible by repeating the joining challenge.

- When the internal identity expires, the bot loses access to resources until it
  reauthenticates.

- The generation counter is still used to detect identity reuse.

#### Renewing and Recovering

> [!NOTE]
> We use the term "renew" fairly loosely in various parts of Teleport and
> especially Machine ID. For our purposes here, we'll try to stick to these
> definitions:
>
> - **refreshing**: fetching new identities at a regular interval, regardless of
>   method
> - **renewing**: refreshing a renewable identity without completing a joining
>   challenge, specific to token joining
> - **recovering**: in bound keypair joining, a recovery occurs when attempting
>   a refresh with no client certificates, or expired client certificates. This
>   triggers additional verifications, and consumes a recovery.

Today, Machine ID has two broad categories of joining:

- Delegated joining, where joining challenges are completed each time a bot
  requests new certificates. Regardless of whether a bot is joining for the
  first time or refreshing an existing valid identity, it must complete a
  challenge to receive certificates. The two cases are functionally equivalent,
  with only minor caveats (see below). This is all join methods except `token`.

- Non-delegated joining, where bots complete an initial joining challenge once
  and then use their existing valid identity as its own proof to fetch new
  certificates. This is exclusively used with `token` joining.

Bound keypair joining could be implemented using either of these strategies.
However, we'll opt to implement this as a delegated join method. This provides
several advantages:

- Standardized implementation, matching all other join methods - except `token`.

- Regular verification checks. With true renewable certificates, bots could last
  indefinitely without completing a challenge. This makes it harder to tell
  which bots are still alive, and could leave bots alive if their join token is
  deleted. When bots regularly interact with the join method, we can ensure they
  _stop_ working more rapidly once disabled (see lifecycle section below)

- If using hardware key storage backends, repeating the joining challenge helps
  ensure the identity can't be effectively exfiltrated.

For the purposes of differentiating a recovery from a regular refresh, we can
take advantage of optional authenticated joining added in
[RFD 0162](./0162-machine-id-token-join-method-bot-instance.md). This allows
clients to present an existing valid identity to preserve certain identity
parameters, like the bot instance UUID.

Using this mechanism, we can ensure that join attempts with an existing client
identity do not consume a join; attempts without one will consume a join.

#### Joining UX Flows

This join method creates two new joining flows:

1. **Static Binding**: A keypair is pregenerated on the client and the public
   key is directly included in the token resource by a Teleport admin.

   Example UX (subject to change):

   ```
   $ tbot keypair create --storage=./storage --proxy-server=example.teleport.sh:443
   Wrote id_ed25519
   Wrote id_ed25519.pub
   $ tctl bots add example --public-key id_ed25519.pub
   $ tbot start identity --token=bound-keypair:id_ed25519
   ```

   (In this example, `tctl bots add` creates a `bound-keypair` token
   automatically, much like a `token`-type token is created automatically
   today.)

   The public key can be copied as needed, similar to SSH `authorized_keys` and
   GitHub's SSH authentication. This is arguably more secure since no secret is
   ever copied.

   On startup, Auth issues a challenge to the bot which is solved with its
   private key, and it receives a standard bot identity.

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

#### Bot Lifecycle

Bots joined with bound-keypair tokens will have a meaningfully different
lifecycle than other types of bots. To summarize some key differences:

- The `Bot` and `ProvisionToken` resources are never expected expire
  automatically. Users could put an expiration on the token resource, but we
  will not recommend doing so.

- The bot keypair has no defined lifespan of its own.

- As long as a bot retains its keypair, it can always be recovered server-side.
  If it runs out of joins, the backend token can be reconfigured to allow
  more. If the backend token is deleted outright, it can be recreated with the
  public key.

- Each time a bot joins, it creates a new bot instance. The bot instance is
  tied to the valid client certificate, and we won't change this behavior. The
  new bot instance will contain a reference to the previous instance ID based on
  the content of `.status.bound_keypair.bound_bot_instance_id` at recovery
  time.

- When a new instance is generated as part of a recovery, refresh attempts using
  the old instance will be denied via a check against the currently bound
  `.status.bound_keypair.bound_bot_instance_id`.

Bots may stop refreshing under several conditions, triggering a recovery
attempt:

- The backing `ProvisionToken` resource has been deleted; in this case, the
  recovery attempt is unlikely to succeed

- The bot has been offline for longer than its certificate TTL

- A lock targeting the bot in any capacity (username, instance UUID, token name)
  is in place

Bots will be unable to recover under any of these conditions:

- The `ProvisionToken` resource has been deleted

- Loss of private key

- A lock is in place

- `.status.bound_keypair.recover_count` is greater than or equal to
  `.spec.bound_keypair.recovery.limit` (and mode is `standard`)

- `.spec.bound_keypair.onboarding.must_register_before` is set to a value before
  the current time

- Inability to provide valid join state document after first join attempt

#### Preventing Credential Duplication

Like `token` joining, `bound-keypair` aims to have relatively robust protections
against stolen or duplicated credentials, including both the long-lived keypair
and short-lived refreshed certificates.

Refreshed certificates will function like renewable certificates do today,
including generation counter checks. An attacker would need to exfiltrate both
the bot certificates and its keypair, and then also prevent the original bot
instance from attempting to refresh certificates, or else a generation counter
check will lock the bot on the next refresh attempt.

However, the long-lived keypair introduces a similar class of problem. If an
attacker exfiltrates the keypair, assuming additional joins are available, they
can attempt to recover and gain extended access. The original bot will still
compete with the imposter bot and each will be forced to recover on every
attempt, but this results in minimal real downtime for an attacker, at least
until the join allowance runs out.

To mitigate this, we'll need to create a mechanism similar to the generation
counter to protect recoveries and the long-lived keypair:

1. When joining, Auth will return a join state document (signed JWT) that
   includes the current join counter, alongside the usual Teleport
   certificates. Bots must store this document for the next join attempt.

2. On subsequent join attempts, bots must include this signed document in the
   join request.

3. On each join attempt, before creating a new bot instance, auth verifies the
   JWT, and compares the current join counter to the join counter in the JWT.

   If the values match, the join is allowed, and a new join state document is
   returned alongside the certificate bundle.

   If they do not match, the join is rejected, and a lock is generated against
   the affected `(bot, token)` pair.

Just as with the generation counter, this procedure relies on an imposter bot
successfully joining once. When the original bot fails to refresh and attempts
to recover, it presents a valid but outdated join state document, we generate a
lock, and then deny further access to both bots.

As an additional level of protection, following a successful recovery attempt,
we can optionally insert a lock targeting the previous bot instance UUID. This
lock can have a modest expiration date to avoid resource leakage on the cluster
(max renewable cert TTL of the proposed 7 days).

Today, bots do not immediately notice if they have been locked. However, we can
investigate methods to ensure clients notice locks early and trigger a
rapid renewal, which would in turn fully lock the `(bot, token)` pair once the
original bot attempts to recover with an outdated join state document. This
would be an improvement over `token`-joined bots today, which will take up to a
full renewal interval to trigger a generation counter check.

#### The Join State Document

As discussed above, the join state document is a JWT signed by Auth included
alongside the regular Teleport user certificates after successfully completing
the challenge ceremony.

This contains fields that

An example join state document JWT payload:
```json
{
  "iat": 1234567890,
  "iss": "example.teleport.sh",
  "aud": "bot-name",
  "bot_instance_id": "aaaa-bbbb",
  "recovery_sequence": 10,
  "recovery_limit": 1,
  "recovery_mode": "standard",
}
```

Our unique claims include:

- `recovery_sequence`: The identity sequence number, analogous to the generation
  counter used for `token` joining. This is used to ensure the identity can't be
  renewed simultaneously by two different clients.

- `recovery_limit`: Used to inform clients of how many remaining join attempts
  they can make before expiring.

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
    # Fields related to the initial join attempt.
    onboarding:
      # If set to a public key in SSH authorized_keys format, the
      # joining client must have the corresponding private key to join. This
      # keypair may be created using `tbot keypair create`. If set,
      # `registration_secret` and `must_register_before` are ignored.
      initial_public_key: ""

      # If set to a secret string value, a client may use this secret to perform
      # the first join without pre-registering a public key in
      # `initial_public_key`. If unset and no `initial_public_key` is provided,
      # a random value will be generated automatically into
      # `.status.bound_keypair.registration_secret`.
      registration_secret: ""

      # If set to an RFC 3339 timestamp, attempts to register via
      # `registration_secret` will be denied once the timestamp has elapsed. If
      # more time is needed, this field can be edited to extend the registration
      # period.
      must_register_before: ""

    # Fields related to recovery after certificates have expired.
    recovery:
      # The maximum number of allowed recovery attempts. This value may
      # be raised or lowered after creation to allow additional recovery
      # attempts should the initial limit be exhausted. If `mode` is set to
      # `standard`, recovery attempts will only be allowed if
      # `.status.bound_keypair.recovery_count` is less than this limit. This
      # limit is not enforced if `mode` is set to `relaxed` or `insecure`. This
      # value must be at least 1 to allow for the initial join during
      # onboarding, which counts as a recovery.
      limit: 1

      # The recovery rule enforcement mode. Valid values:
      # - standard (or unset): all configured rules enforced. The recovery limit
      #   and client join state are required and verified. This is the most
      #   secure recovery mode.
      # - relaxed: recovery limit is not enforced, but client join state is
      #   still required. This effectively allows unlimited recovery attempts,
      #   but client join state still helps mitigate stolen credentials.
      # - insecure: neither the recovery limit nor client join state are
      #   enforced. This allows any client with the private key to join freely.
      #   This is less secure, but can be useful in certain situations, like in
      #   otherwise unsupported CI/CD providers. This mode should be used with
      #   care, and RBAC rules should be configured to heavily restrict which
      #   resources this identity can access.
      mode: "standard"

    # If set to an RFC 3339 timestamp, once elapsed, a keypair rotation will be
    # forced on next join if it has not already been rotated. The most recent
    # rotation is recorded in `.status.bound_keypair.last_rotated_at`.
    rotate_after: ""

status:
  bound_keypair:
    # If `spec.onboarding.initial_public_key` is unset, this value will be
    # generated server-side and made available here. If
    # `spec.onboarding.registration_secret` is set, its value will be copied
    # here.
    registration_secret: <random>

    # The public key of the bot associated with this token, set on first join.
    # This key is written in SSH public key format.
    bound_public_key: <data>

    # The current bot instance UUID. A new UUID is issued on recovery; the
    # previous UUID will be linked via a `previous_instance_id` in the bot
    # instance.
    bound_bot_instance_id: <uuid>

    # A count of the total number of recoveries performed using this token.
    recovery_count: 0

    # The timestamp of the last successful recovery attempt, if any.
    last_recovered_at: null

    # The timestamp of the last successful keypair rotation, if any.
    last_rotated_at: null
```

#### Proto Draft

```protobuf
message RegisterUsingBoundKeypairInitialRequest {
  types.RegisterUsingTokenRequest join_request = 1;

  // If set, requests a rotation to the new public key. The joining challenge
  // must first be completed using the previous key, and upon completion a new
  // challenge will be issued for this key. Certificates will only be returned
  // after the second challenge is complete.
  bytes new_public_key = 2;

  // A document signed by Auth containing join state parameters from the
  // previous join attempt. Not required on initial join; required on all
  // subsequent joins.
  bytes previous_join_state = 3;
}

message RegisterUsingBoundKeypairChallengeResponse {
  bytes solution = 1;
}

message RegisterUsingBoundKeypairRequest {
  oneof payload {
    RegisterUsingBoundKeypairInitialRequest init = 1;
    RegisterUsingBoundKeypairChallengeResponse challenge_response = 2;
  }
}

message RegisterUsingBoundKeypairCertificates {
  // Signed Teleport certificates resulting from the join process.
  Certs certs = 1;

  // A signed join state document to be provided on the next join attempt.
  bytes join_state = 2;
}

message RegisterUsingBoundKeypairResponse {
  oneof response {
    // A challenge to sign. During keypair rotation, a second challenge will be
    // provided to verify the new keypair before certs are returned.
    string challenge = 1;
    RegisterUsingBoundKeypairCertificates certs = 2;
  }
}

service JoinService {
  // ...snip...

  rpc RegisterUsingBoundKeypair(stream RegisterUsingBoundKeypairRequest) returns (stream RegisterUsingBoundKeypairResponse);
}

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
performed simply by adjusting values in the resource. If a bot fails, its join
allowance can be incremented easily.

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

variable "bot_join_overrides" {
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
      joining = {
        # look up node-specific count in the join overrides map, default to 2
        total_joins = lookup(var.bot_join_overrides, each.key, 2)
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

In this example, if node `bar` uses its 2 joins, we can add a new entry for
it in `bot_join_overrides` and its `ProvisionToken` will be updated to allow
additional joins.

#### Challenge Ceremony

The challenge ceremony will take inspiration from several existing join methods.
We can create an interactive challenge similar to TPM joining and present a
challenge containing a nonce. To avoid completely implementing our own
authentication ceremony, clients can use `go-jose` to marshal and sign a JWT
which can then be verified easily on the server.

A rough outline of the joining procedure:

```mermaid
sequenceDiagram
	participant keystore as Local Keystore
	participant bot as Bot
	participant auth as Auth Server

	opt first join & has initial join token
		bot->>+keystore: Generate new keypair
		keystore-->>-bot: Public key

		bot->>auth: Bind public key with token
		auth->>auth: Verify token<br>& record public key
	end

	bot->>auth: Request joining challenge
	auth-->>bot: Sends joining challenge
	bot->>keystore: Request signed document
	keystore-->>bot: Signed challenge document
	bot->>auth: Signed challenge document<br>& previous join state
	auth->>auth: Validate signed document<br>against bound public key

	opt no valid client certificate
		auth->>auth: Validate join state document
		auth->>auth: decrement remaining join counter
	end

	auth-->>bot: Signed TLS certificates<br>& new join state
```

To avoid use of traditional renewable certificates, this takes advantage of
recent support for optionally authenticated joining added as part of
[RFD 0162](./0162-machine-id-token-join-method-bot-instance.md). If a bot still
has a valid client certificate from a previous authentication attempt, it uses
it to open an mTLS session.

When Auth validates the join attempt, clients that presented an existing valid
identity are considered to be requesting a refresh rather than recovering,
leaving the recovery counter untouched. Clients that do not present a valid
client certificate are considered to be recovering and the token associated with
this public key must have `.status.bound_keypair.recovery_count` less than
`.spec.bound_keypair.recovery.limit`.

#### Client-Side Changes in `tbot`

Bots should be informed of various status metrics, including number of remaining
joins and whether or not a keypair rotation has been requested. There's a few
methods by which we could inform bots of their remaining join allowance:

1. Join State Document (recommended): we include this information as part of the
   join state document which is returned alongside the certificate bundle
   following a successful join.

2. Heartbeats: bots submit heartbeats at startup and on a regular interval. It
   would be trivial to include a remaining join counter in the (currently empty)
   heartbeat response.

3. Certificate field: we could include the number of remaining joins in a
   certificate field.

4. New RPC: we could add a new RPC for bots to fetch this, alongside any other
   potentially useful information.

5. We could grant bots permission to view their own join tokens. There is
   precedent here as bots can view e.g. their own roles without explicitly
   having RBAC permissions to do so.

The remaining join counter should then be exposed as a Prometheus metric to
allow for alerting if a bot drops below some threshold.

Importantly, this is a potentially lagging indicator. The design allows for the
join counter to be decreased (to zero) at any time, so a recovery attempt may
still fail. This should be acceptable since it can also be increased after the
fact to restore access if desired.

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
`tctl edit` will continue to work; upserting resources to modify runtime
values will not interfere with bots as this is expected to be a regular part of
bot / token lifecycle.

Additional `tctl` changes will include:

- Once satisfied with behavior of the join method, replacing the default
  automatically generated join tokens for `tctl bots add` and
  `tctl bots instances add` to use this new join method.

- Adding a column for "joins remaining" in `tctl bots instances ls` (where
  relevant).

- Adding support for updating `total_joins` in `tctl bots update`

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

In the future, we might also consider configurable cluster alerts when a bot
recovers or has used its last attempt. This should be opt-in as this type of
alert may not scale well with lots of bots.

#### Keypair Rotation

Given the long-lived nature of the keypair credential, it's important to support
rotation without bot downtime. Ideally, it should be possible to initiate a
rotation from either the server (e.g. by setting a `rotate_on_next_renewal` flag
on the token/bot instance) or `tbot` client.

To trigger a rotation, an admin can set `.spec.bound_keypair.rotate_after=$timestamp`
on the bound keypair token. On its next refresh attempt, the server will require
a keypair rotation as part of the usual challenge process. Once the previous
keypair has been validated, the client will be asked to generate a new keypair,
then repeats the challenge process to validate ownership of the new private key,
and is only then issued certificates.

To perform the rotation, additional steps are taken as part of the challenge
ceremony:

```mermaid
sequenceDiagram
  participant keystore as Local Keystore
  participant bot as Bot
  participant auth as Auth Server

  Note over keystore,auth: Initial onboarding not shown

  bot->>auth: Request joining challenge,<br>provides previous join state
  auth-->>bot: Sends joining challenge<br>against original public key

  bot->>keystore: Request signed document<br>with original public key
  keystore-->>bot: Signed challenge document
  bot->>auth: Signed challenge document
  auth->>auth: Validate signed document<br>against bound public key

  auth->>bot: Requests new public key
  bot->>keystore: Request new keypair
  keystore-->>bot: New public key
  bot->>auth: New public key

  auth-->>bot: Sends new joining challenge<br>for new public key
  bot->>keystore: Request signed document<br>with new public key
  keystore-->>bot: Signed challenge document

  bot->>auth: Signed challenge document
  auth->>auth: Validate signed document<br>against new public key
  auth->>auth: Commit new public key to backend

  auth-->>bot: Signed TLS certificates<br>& new join state
```

As shown above, the join service will return a second challenge rather than
certs if a rotation was requested server side. Certs will only be returned on
completion of the second challenge using the new public key, and the new key
will only be committed to the backend at this point.

Other bot parameters remain unchanged. No new bot instance is created, the
generation counter is not reset and is checked and incremented as usual.

#### Remaining Downsides

- Repairing a bot that has exhausted all of its joins is still a semi-manual
  process. It is significantly easier, and does not necessarily require any
  changes on the impacted bot node itself, but is still annoying. Users can opt
  out of this by setting `.spec.recover.mode="relaxed"`, but this has obvious
  security implications, which may be tolerable for some use cases.

- Effort required to configure IaC / Terraform is still fairly high, even if
  reduced.

### Other Supporting Improvements

Alongside `bound-keypair` joining, we have several UX proposals to further
improve the usability of non-delegated joining.

#### Longer-Lived Bots

The bot identity's 24 hour maximum TTL is too restrictive and should be
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
tbot+[auth|proxy]+[join method]://[token name]:[optional parameter]@[addr]:[port]?key=val&foo=bar
```

The "optional parameter" is a relatively new concept to support specifying the
token name and secret value, as they are now decoupled. Join methods like Azure
that require an additional parameter can take advantage of this as well;
previously this method required a `tbot.yaml` in order to specify a `ClientID`.

Consider these two equivalent commands:
```
$ tbot start identity --proxy-server example.teleport.sh:443 --join-method bound-keypair:initial-join-secret --token example

$ tbot start identity tbot+proxy+bound-keypair://example:initial-join-secret@example.teleport.sh:443
```

(We will also need to introduce `method:parameter` syntax for the traditional
`--join-method` flag.)

Joining URIs can greatly simplify the regular onboarding experience by providing
a single value to copy when onboarding a bot:

```
$ tctl bots add example --roles=access
The bot token: tbot+proxy+bound-keypair://example:initial-join-secret@example.teleport.sh:443
This token will expire in 59 minutes.

[...snip...]

$ tbot start identity tbot+proxy+bound-keypair://example:initial-join-secret@example.teleport.sh:443 ...
```

Given the CLI now supports many operational modes, it's much easier for users to
write their given starting command (e.g. `tbot start app`) and paste the joining
URI to get started immediately.

URL paths and query parameters may also provide options for future extension if
desired.

Additional examples for other join methods:
```
# Traditional token joining, connecting to auth
$ tbot start identity tbot+auth+token://abcde12345@teleport.example.com:3025

# Azure joining via proxy with client ID specified
$ tbot start identity tbot+proxy+azure://bot-token:22222222-2222-2222-2222-222222222222@example.teleport.sh:443/
```

## Future Extensions and Alternatives

### Future: Agent Joining Support

We should explore expanding this join method to cover regular Teleport agent
joining as well as bots, as a more secure alternative to static or long-lived
join tokens.

### Future: Additional Keypair Protections

We should investigate supporting additional layers of protection for the private
key. There are several avenues for this, depending on storage backend:

- Filesystem storage can be encrypted at rest and require a private key to be
  entered to unlock it, similar to SSH keys without an agent.

- Secure Enclave storage can require that the device be unlocked, or require
  biometric verification.

- Other HSM-stored keys may support various types of human presence
  verification. For example, YubiHSM has a touch sensor that can be required for
  access on a key-by-key basis.

Note that as a consequence of implementing this as a delegated joining method,
bots are expected to complete joining challenges at regular intervals. This
could be varying levels of impractical depending on keystore backend. Keys
stored encrypted at rest on the filesystem could be decrypted and kept in
memory, but HSMs with presence requirements may not be practical.

#### Future: Tightly Scoped Token RBAC

To better support use cases where central administrators vend bot tokens for
teams, we can add scoped RBAC support for `ProvisionToken` CRUD operations.

For example, this would allow a designated team to update a `bound-keypair`
token to increase the join counter without needing to reach out to the central
administrator.

This is likely dependent on [Scoped RBAC](https://github.com/gravitational/teleport/pull/38078),
which is still in the planning stage.

### Future: Explicitly Insecure Token Joining

There are perfectly valid use cases for allowing relatively insecure access to
resources that do not have strict trust requirements, and Teleport's RBAC system
is robust enough to only allow these bots access to an acceptable subset of
resources.

Initially, we will provide a minimal version of this via the
`.spec.bound_keypair.joining.insecure` flag, which bypasses the generation
counter check. When combined with the `unlimited` flag, this allows for
effectively unlimited joining, provided a keypair can still be provided. This
should be enough to enable basic support on otherwise-unsupported CI/CD
provides, at least provided a keypair can be stored in a platform secret.

Alternatively, it may be worthwhile to add an `insecure-shared-secret` join
method that further reduces security enforcement, and allows for arbitrary
joining in use cases that still fall through the cracks, so long as end users
understand the security implications.

### Alternative: Client-side multi-token support

A simpler variant of N-Token Resiliency, this would allow `tbot` clients to
accept an ordered list of joining token strings which could be used
sequentially. If the internal identity expires, the next token in the list will
be used to attempt a recovery.

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
constrain credential reuse by limiting the number of possible recoveries until a
human has to take some action.

Bound keypair joining replaces the secrets with a join counter, and allows for
(among other things) resuscitation of dead bots since their credentials remain
available even once expired.

A lighter weight alternative here could be client-side multi-token support as
described in the alternatives above.
