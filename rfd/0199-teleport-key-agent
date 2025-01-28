---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 199 - Teleport Key Agent

## Required Approvers

* Engineering: @rosstimothy
* Product: @xinding33 || @klizhentas

## What

Introduce Teleport Key Agent to provide the ability for one Teleport client
to handle private key storage and operations for one or more other local
Teleport clients.

## Why

Teleport Key Agent will enable multiple use cases, including:

* Providing a shared key state necessary for specific features
  * Delegating Hardware Key touch/pin prompts to Teleport Connect for better UI
  * [hardware key pin caching](./XXXX-hardware-key-pin-caching.md) between
    multiple Teleport clients
  * Syncing Teleport Connect's front end login state with `tsh`
  * Exposing login state stored in memory, e.g. `tsh --add-keys-to-agent=only`
* Providing concurrent access to key storage with limiting properties
  * Example: Hardware Key Support holds an exclusive connection to the Hardware
    Key, preventing concurrent access from different clients

Note: the primary short-term goal of this RFD is to enable UX improvements in
Hardware Key PIN support, while the other use cases may or may not be
implemented in the future. I'll note these features below as *future work*.

## Details

The client running the Teleport Key Agent will be referred to as the "agent"
client. Any client interfacing with the agent will be called a "dependent"
client.

### UX

In order to utilize the Teleport Key Agent, users will need to launch an agent
client and login. See [running the agent](#running-the-agent-ux-continued)
for more details on running the agent.

So long as the agent remains running, dependent client commands will interface
with the agent for some or all login information, depending on the
[agent mode](#modes).

Dependent clients should retrieve login information seamlessly from the agent
so that there is no UX degradation when compared to a client solely using file
storage.

#### Running the Agent

##### Teleport Connect

Teleport Connect is an ideal runner for an agent client since it:

1. Is a long-lived process
2. Provides a UI for login state (login status, current cluster, etc.)
3. Provides a UI for touch/pin prompts (Hardware Key Support)
4. Has the ability to foreground itself for prompts (relogin, touch/pin prompts)

By default Teleport Connect uses a different Teleport home directory from `tsh`,
so we need to add a way to use the same Teleport home.

We will introduce two new config options:

| Property | Default | Description |
|----------|---------|-------------|
| `teleport.home` | `~/Library/Application\ Support/Teleport\ Connect/tsh` | Sets home directory for login data |
| `teleport.agent` | `false` | Runs the Teleport Key Agent |

**future work**

The Teleport Connect developers have tested with linking login storage with
a [symlink](https://github.com/gravitational/teleport/issues/25806), which is
essentially the same as changing Connect's Teleport home directory. Without any
additional changes, this testing resulted in some state drift and other errors.

These issues are expected to impact [signing mode](#signing-mode) which won't
offer sophisticated login syncing logic. [Login sync mode](#login-sync-mode-future-work),
which is detailed as *future work*, could be used instead to mitigate state drift
issues in Teleport Connect.

Furthermore, once we develop a reliable framework for login state syncing, whether
it's with [the login sync agent mode](#login-sync-mode) or otherwise, we may consider
defaulting the config values above to `~/.tsh` and `true` respectively.

##### `tsh agent`

Teleport Connect is the primary method for running the Teleport Key Agent as it
provides the best UX as a naturally long-lived process and UI components for
Hardware Key pin/touch prompts. However there are likely to be some scenarios
where running Teleport Connect is not an option. Therefore we will make
`tsh agent` available as well.

The Teleport Key Agent can be launched explicitly with `tsh agent`. This
command will prompt the user to login and remain running to serve the agent.

Note: `tsh agent` will also be a lightweight command useful for setting the
groundwork with the slightly more extensive Teleport Connect implementation.
It will definitely speed up my testing of the feature, so at the very least
we could make this a hidden command.

#### Agent already running

If either Teleport Connect or `tsh agent` finds that there is already an active
Teleport Key Agent for the same Teleport home directory, an error will be
returned.

```console
> tsh agent
Error: another instance of Teleport Key Agent is already running
```

In the case of Teleport Connect, the error will be displayed in the UI but will
not impact it from running. In fact, Teleport Connect will interface with the
running Teleport Key Agent the same as any other dependent client.

#### Agent stops running

If the agent stops unexpectedly during a dependent client's operation, it may
lead to an error for the dependent client. If the client is able to re-login
to fix the error, the client should do so, switching from key agent storage
to file storage.

In cases where the agent's login state was fully in file storage, the dependent
client can just change sources and continue without the need to re-login.

#### Prompts

The agent client is responsible for any prompts that occur within the key
storage and signing interface. Currently, this is limited to Hardware Key
pin/touch prompts, but there are some additional use cases we can explore.

##### Hardware Key pin/touch

Hardware Key pin or touch prompts occur when the user has any of the following
requirements on their cluster auth preference or role(s):

* `require_session_mfa: hardware_key_touch`
* `require_session_mfa: hardware_key_pin`
* `require_session_mfa: hardware_key_touch_and_pin`

These prompts occur any time the hardware key is employed to perform
cryptographical operations with the hardware-backed private key. Generally,
this occurs when a Teleport client forms a new connection to Auth, Proxy,
or any other Teleport service.

Since the agent will be responsible for these pin/touch prompts without any
context, it may look like this:

```console
> tsh agent
...
Tap your YubiKey
Enter your YubiKey PIV PIN:
Tap your YubiKey
Enter your YubiKey PIV PIN:
Enter your YubiKey PIV PIN:
Tap your YubiKey
```

Note: in the case of Teleport Connect, each tap/pin request is displayed in a
dialog which foregrounds the application.

Note: touch is cached for 15 seconds on the hardware key itself, while the PIN
is cached in the Teleport client for ~5 seconds or longer when
[Hardware Key pin caching](./0198-hardware-key-pin-caching) is enabled.

Dependent clients will also need to prompt the user to complete the touch/pin
prompt through the agent, especially if the agent doesn't have the ability to
foreground its prompts. From the dependent client's perspective, it does not
know if the agent is prompting pin, touch, or something else; it just sees its
sign request hang. Therefore, dependent clients will always output a generic
prompt if a sign request hangs:

```console
> tsh ssh server01
### hangs for ~1 seconds
Go to your Teleport Key Agent and complete any requested actions to continue.
```

##### Prompt notification (*future work*)

We can make the agent smarter when it needs to prompt for user action. Rather
than just outputting a prompt and waiting until it's completed, it can send a
notification back to the dependent client about the prompt. In some cases like
a touch prompt, the dependent client can mirror the agent's prompt:

```console
> tsh ssh server01
### sign through agent, agent notifies client of touch prompt requirement
Tap your YubiKey
```

In other cases, like pin entry, the dependent client can continue to output the
redirection prompt with additional context:

```console
> tsh ssh server01
Go to your Teleport Key Agent and enter your PIV PIN to continue.
```

##### Confirmation prompt (*future work*)

We can add a confirmation layer between the agent server and dependent clients.
In order to utilize the agent to query certs or sign with keys, the user would
need to confirm the dependent client's connection with one or more of the following:

* basic confirmation prompt in the key agent, similar to what we use for headless
* password prompt in the dependent client
  * like with ssh-agent, this would be a temporary password provided on agent
  startup
* touch prompt with temporary local MFA registration

Once the connection is authenticated through the local confirmation mechanism,
the dependent client could perform any queries or signatures until client is
closed. In practice, this means that each `tsh` command would require a single
confirmation prompt to complete.

Note: once again, we run into the issue of prompting on every `tsh` command
that we ran into with `hardware_key_pin`. Therefore we would only want to
enable this confirmation layer when the tradeoff of better security for worse
UX is justified. See the sections on [the risk of key agent forwarding](#key-agent-forwarding)
and [safely supporting key agent forwarding](#support-teleport-key-agent-forwarding-future-work)
for more details on how and why this confirmation layer would be included.

### Modes

The Teleport Key Agent can work in two different modes; `signing` and `login sync`.

Note: the Teleport Key Agent will be pre-released with `signing` mode only as
it will greatly improve the UX for Hardware Key pin support, especially when
paired with [Hardware Key pin caching](./xxxx-hardware-key-pin-caching).

#### Signing mode

In signing mode, the Teleport Key Agent will only be responsible for providing
a signing interface for Teleport Clients, similar to ssh-agent or gpg-agent.
All other login information (profile info, certificates, etc.) will be retrieved
from the standard file storage interface.

This signing mode provides the baseline feature set needed to introduce UX
improvements for features which limit access to private keys, including:

* Hardware Key Support
* In memory key storage, e.g. `tsh --add-keys-to-agent=only`

Since only private keys are synced between clients in this mode, there is a
possibility of state drift between clients.

For example, if the user is running Teleport Connect as the agent, but they
relogin with `tsh`, Teleport Connect would not know that the keys and certs
in their shared home directory has been replaced. Depending on what Teleport
Connect was trying to do at the time of the login, it might accidentally load
the new certs with the old key, leading to a new confusing error and in the
best case, another prompt to re-login.

Note: in signing mode, dependent clients can perform any login operations as
usual (e.g. `tsh login`, `tsh app login`) adding or removing certs directly in
file storage. The dependent client will not generate new private keys in file
storage and instead continue to use the same private keys offered by the agent
via its sign method. This will prevent the agent from becoming out of sync
with dependent clients.

#### Login sync mode (*future work*)

In login sync mode, the agent will be responsible for all login information.
Dependent clients will query it for profiles, current profiles, active certs,
in addition to interfacing with it for signing operations. Dependent clients
will also be able to add or remove certs through the agent when running
commands like `tsh app login`.

The login sync agent will also notify any listening clients of login state
changes. For example, if the user logs in with `tsh login`, the agent will
store the certs from the login and notify any listening clients that the
login certificates have been replaced.

By running all login state storage through the agent, dependent clients can
sync their login state without racing over file storage.

This mode would be very useful for Teleport Connect to avoid state drifts
reflected in the UI. In fact, the Teleport Connect UI would act as a dependent
client, listening through `tshd` for any login state changes to respond to.

This mode would also enabled `tsh` in restricted modes like `tsh --add-keys-to-agent-only`,
which require the user to re-login for every command to avoid adding keys/certs
to file storage. Instead, users could run `tsh agent` to login once and then
run any `tsh` commands to interface with the agent without any private keys
or public certs touch file storage.

Note: this mode is a stretch goal and is not scheduled to be completed, but
this RFD would be incomplete if it did not at least consider this future work.

### Client interface

In order to utilize the Teleport Key Agent, dependent clients will need to
interface with the agent as opposed to file storage or in-memory storage. Agent
storage will be added as a new type of storage, with a `Keystore` implementation
for [signing mode](#signing-mode) and a `ProfileStore` and `TrustedCertsStore`
implementation for [login sync mode](#login-sync-mode-future-work).

### Security

#### Local key agent

For the intended use case of using Teleport Key Agent as a local key agent,
there is not much of concern to consider:

* The Teleport Key Agent acts as a proxy for normal file storage, and as such
is protected through file permissions on the unix socket it's served on:
`$TELEPORT_HOME/agent.sock`.
* Even in login sync mode, the agent never exposes the underlying private key
material, only an interface to sign with.

#### Key agent forwarding

We must also consider the unintended use case of forwarding the agent over ssh
with unix domain socket forwarding. Like ssh agent forwarding, this is an
insecure use case which will be strongly advised against. However, it cannot
be entirely avoided, the same way that we can not stop a user from running
`tsh scp $HOME/.tsh server01:`.

As usual remote attacks are best mitigated through phish-proof authentication.
If the following features are enabled, the impact of a compromised forwarded
Teleport Key Agent will be largely mitigated:

* Per-session MFA or Hardware Key Support (pin/touch)
* MFA for Admin Actions

If deemed necessary, we could put the agent behind an extra
[confirmation layer](#confirmation-prompt-future-work). This confirmation layer
would require dependent clients to perform a confirmation action (y/N, pin, or
touch) to allow the dependent client to utilize the agent.

### Configuration

N/A

### Proto specification

TODO: Teleport Key Agent will be a new gRPC server with methods to query
available (public) keys, certs, profiles, and cas. It will also implement a
signing method in a similar way to `ssh-agent`.

### Backward Compatibility

Teleport Key Agent is purely a client-side feature with no backwards
compatibility concerns. However, there may be some compatibility concerns
between different client implementations of the Teleport Key Agent in the
future as new versions are released.

### Audit Events

N/A

## Additional Considerations

### Support Teleport Key Agent forwarding (*future work*)

In the [security section above](#key-agent-forwarding), I covered the risks of
forwarding the agent and how to mitigate those risks. In the end, we would have
a pretty robust Key Agent server with local authentication in place.

Since it is likely impossible to completely remove the possibility of key agent
forwarding, it may be better to support it directly with the mitigation
strategies in place instead of leaving ill advised users to forward the agent
insecurely.

This would be an opt-in feature available only to clusters with Per-session MFA
and MFA for Admin Actions enabled.

For context, we have considered this type of key agent forwarding with `ssh-agent`
[in the past](https://github.com/gravitational/teleport/pull/19421) and decided
it was too insecure. However, I believe that the primary issues with that proposal
have been addressed:

* The addition of a local confirmation layer, ideally employing phish-proof MFA
* The addition of MFA for Admin Actions to protect sensitive admin actions
* The additional requirement of Per-session MFA or Hardware Key support

Note: the confirmation layer is very similar to headless. In both cases, the user's
local client is used to approve a remote connection. The main differences are:

* The user's local agent client is responsible for issuing the confirmation
request rather than the Teleport Auth and Proxy servers
* The user's local agent client decides the confirmation mechanism (e.g. temporary
registered local mfa) rather than Auth registered MFA

Note: In order to support Per-session MFA remotely, the agent will also
need to support issuing mfa verified certs. The MFA prompt would occur locally
and the cert would be shared with the remote client over TLS, where it would
be kept securely in memory (similar to headless). Meanwhile, admin actions,
which require a fresh MFA response to complete, would not be supported remotely
at all.

### Why not utilize `ssh-agent` or `gpg-agent`

Early on in researching this feature, I worried I may be reinventing the wheel
when similar tools like `ssh-agent` and `gpg-agent` exist. Here's what I
determined:

* `ssh-agent` does not support generic signing, it uses a different signing
algorithm specific to SSH. In order to use `ssh-agent`, we would need to add
several extensions. It would be better to make our own feature-rich custom gRPC
agent designed with our own purposes in mind.
* `gpg-agent` is reportedly an old school, bloated agent which does all things
related to encryption. However, it does not ship with any popular OS's out of
the box and has been largely abandoned by users in search of more modern and
potentially more secure tools.

#### `ssh-askpass` with `tsh agent`

In this RFD I've mentioned that Teleport Connect provides the best UX for this
feature. However, we could improve the UX of `tsh agent` as well by integrating
something like `ssh-askpass` for any agent prompts, allowing the user to run
the agent in the background or even as a systemd process.

Since `ssh-askpass` does not ship with most OS's, it seems better to me to skip
this complication and rely on Teleport Connect since it would provide better UX
regardless. This is something we may want to consider in the future if there is
any demand for it.
