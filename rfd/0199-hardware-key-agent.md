---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 199 - Hardware Key PIN Caching

## Required Approvers

* Engineering: @rosstimothy && @ravicious
* Product: @klizhentas

## What

Implement a hardware key PIN caching mechanism so that a user is not prompted
for PIN more than once within a configured span of time.

In order to cache the PIN across process boundaries (Teleport Connect, separate
`tsh commands`), this RFD also introduces a client-side hardware key agent which
will automatically run within Teleport Connect.

## Why

Currently, when a user has hardware key PIN enforced with `require_session_mfa: hardware_key_pin|hardware_key_touch_and_pin`,
they are required to enter their PIN for every action (e.g. `tsh` command,
`tsh proxy` connection). This is very disruptive when running several commands
in short succession, especially when:

* running several `kubectl` commands, database queries, or app requests through
a Teleport local proxy (`tsh proxy kube|db|app`).
* using automated scripts which run `tsh` commands in bulk.

This UX concern has turned out to be a significant impediment to the adoption
of Hardware Key PIN Support, and unfortunately, the PIN caching built in to the
hardware key has proven [unreliable for Teleport use cases](#problems-with-built-in-piv-pin-caching).

## Details

This RFD covers both hardware key PIN caching and the new hardware key agent.
These features are separate, but the combination of the two features ultimately
provides the desired behaviors:

* The hardware key agent prompts for PIN/touch on behalf of other clients
* The hardware key agent caches the PIN on behalf of other clients

If only one of the features is enabled, users would still get that feature's
distinct benefits.

Hardware key PIN caching: Teleport clients prompt for PIN and cache it for
the duration of the command. This is useful for commands which may usually
prompt for PIN multiple times during its lifetime, such as `tsh proxy ...`
commands.

Hardware key agent: The hardware key agent prompts for PIN/touch on behalf of
other clients. When using Teleport Connect as the agent, Teleport Connect will
provide a UI for these prompts and foreground them. This provides better UX
than the CLI prompts which can be easy to miss, especially in commands with
verbose output or commands running in the background like `tsh proxy ...`.

### Hardware key PIN caching

When enabled at the cluster-level, Teleport clients (`tsh`, `tctl`, and
Teleport connect) will cache the user's hardware key PIN in memory for a
specified duration of time. When the PIN is cached, the Teleport client will
provide the PIN to the hardware key without prompting the user again.

Note: the PIN will remain cached within the Teleport client so long as it
remains running. There is no additional manual or automatic mechanism to reset
the cache before the cache duration has elapsed.

#### Cluster Auth Preference

To enable PIN caching for Teleport clients, set `cap.hardware_key.pin_cache_timeout`
to the desired timeout duration:

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  name: cluster-auth-preference
spec:
  ...
  hardware_key:
    # pin_cache_timeout is the amount of time that Teleport clients will cache
    # the user's hardware key (PIV) PIN. The timeout countdown is started when 
    # the PIN is stored and is not extended by subsequent accesses. This timeout
    # can not exceed 1 hour. When empty or 0, the pin will not be cached.
    pin_cache_timeout: 15m
```

Teleport clients will retrieve this setting through `/webapi/ping`, which is
cached by the client alongside other cluster settings.

#### `PinCachingPrompt` pseudo-code

PIN caching will be implemented at the level of the PIN prompt itself, holding
the PIN in memory and only prompting for PIN once the cache timeout duration
has elapsed.

```go
// Pseudo-code

// PinCachingPrompt is a HardwareKeyPrompt wrapped with PIN caching.
type PinCachingPrompt struct {
  keys.HardwareKeyPrompt

  // cluster-configurable timeout duration
  PinCacheTimeout time.Duration

  cachedPIN       string
  cachedPINExpiry time.Time
}

func (p *PinCachingPrompt) AskPIN(ctx context.Context, requirement keys.PINPromptRequirement) (string, error) {
  pin := p.getCachedPIN()
  if pin != "" {
    return pin, nil
  }

  pin = p.HardwareKeyPrompt.AskPIN()
  p.setCachedPIN(pin)
  return pin, nil
}

func (p *PinCachingPrompt) getCachedPIN() (string) {
  if p.cachedPIN == "" {
    return ""
  }
  
  if time.Now().Before(p.cachedPINExpiry) {
    return p.cachedPIN
  }

  if time.Now().After(p.cachedPINExpiry) {
    p.cachedPIN = ""
    return ""
  }
}

func (p *PinCachingPrompt) setCachedPIN(pin string) {
  p.cachedPIN = pin
  p.cachedPINExpiry = time.Now().Add(p.PinCacheTimeout)
}
```

### Hardware Key Agent

The hardware key agent provides access to the hardware key to other Teleport
clients running on the same host. This allows separate clients to share the
agent's PIN/touch prompts, PIN cache, and open hardware key connections.

The hardware key agent can be started in two ways:

* By launching [Teleport Connect](#teleport-connect) which automatically starts
the hardware key agent within the background daemon process.
* By running [`tsh piv agent`](#tsh-piv-agent) explicitly.

#### Terminology

An "agent client" is a Teleport client process serving the hardware key agent.

A "dependent client" is a Teleport client process interfacing with the hardware key agent.

#### Signing interface

The hardware key agent will provide the ability to sign with a hardware private
key, specified by hardware key serial number, PIV slot, and known public key.

The agent will be served as a [gRPC](#hardwarekeyagentservice) service on a unix
socket, `$TEMP/.Teleport-PIV/agent.sock`, with [basic TLS](#security).

#### `$TEMP/.Teleport-PIV/agent.sock`

`$TEMP` here depends on the client OS. We will use `os.TempDir` to get the
correct temp directory for the OS.

We use a temp directory so that it is easy for other Teleport clients to connect
to the shared socket, regardless of each individual client's Teleport home
directory, as long as they have file permissions to do so (`700` for the folder,
`600` for the socket).

Note: Teleport clients with different Teleport home directories share the same
underlying hardware private keys, so the agent client's own Teleport home
directory is not relevant to its ability to serve the hardware private keys.
In practice, this means that Teleport Connect can serve the hardware key
agent without needing to sync its Teleport home directory with `tsh`.

#### PIN and touch prompts

Since the agent client interfaces directly with the hardware key, it is
responsible for any hardware key PIN/touch prompts. When a dependent client
makes a `Sign` request through the agent, the agent client will prompt for
PIN or touch if required.

As a result, when PIN/touch is required, the dependent client will hang until
it is prompted and handled by the agent client. When Teleport Connect acts as
the agent client, it will automatically foreground itself to present these prompts
to the user.

Note: touch is cached for 15 seconds on the hardware key itself and PIN is
optionally cached when [Hardware Key pin caching](#hardware-key-pin-caching)
is enabled. Like any normal Teleport client, the agent client will only prompt
for PIN or touch when it isn't cached.

### Dependent client changes

In order for dependent clients to utilize the hardware key agent fully, without
the need to connect to the hardware key directly outside of login, the changes
below are needed.

#### Enrich hardware private key PEM encoded file

During hardware key login, we store a [PEM encoded file](./0080-hardware-key-support.md#private-key-interface)
to represent the hardware backed private key. Instead of holding the actual
private key PEM, this file holds information necessary to retrieve the PIV
handler for the hardware private key. Currently, this is just the YubiKey
serial number and PIV slot.

However, in order to use the key, the client must also know the public key,
the private key policy to determine when to include touch/pin prompts, and the
attestation statement to include in any re-login features (e.g. Per-session MFA).

This additional information is retrieved directly from the hardware key by each
call of the client process. This results in two problems:

* Connecting to the hardware key and retrieving this information has some
performance cost.
  * In particular, re-attesting the key against the Yubico CA to derive the
  private key policy takes upwards of 100ms.
* Retrieving the information requires a mutually exclusive connection directly
to the hardware key.
  * The hardware key agent will hold open connections for at least 1 second in
  order to improve performance for back-to-back signature requests, meaning
  dependent clients could be locked out for this duration when trying to make
  a direct connection.
  * It would make more sense not to have dependent clients switching between
  direct hardware key connections and the hardware key agent, especially from
  an implementation perspective.

Since all this information is known at time of login, we will instead store
this information in the pem-encoded file:

```diff
-----BEGIN PIV YUBIKEY PRIVATE KEY-----
######## PEM encoded #########
{
  "serial_number": "12345678",
  "slot_key": "9e"
+ "public_key_der": "...",
+ "private_key_policy": "hardware_key_pin",
+ "attestation_statement": {...}
}
##############################
-----END PIV YUBIKEY PRIVATE KEY-----
```

Note: expanding the PEM encoded file is the chosen solution as it also improves
performance in the base, agentless case. For posterity, I originally planned on
adding a hardware key agent rpc like `GetAdditionalInfo` for dependent clients
to retrieve this info.

Note: for backwards compatibility, clients will continue to retrieve this
information directly from the hardware key if it is missing from the PEM
encoded file.

#### `hardwareKeyAgentService` pseudo-code implementation

Teleport clients will use a `hardwareKeyAgentService` interface to interact
with hardware private keys, rather than interacting directly with the PIV
interface.

Note: `HardwarePrivateKeyRef` is marshalled to/from the PEM encoded file described above.

```go
// Pseudo-code

// hardwareKeyAgentService has two implementations:
//  - direct implementation with piv-go, adapted slightly from our existing implementation
//  - hardware key agent gRPC service implementation
type hardwareKeyAgentService interface {
  Sign(ref HardwarePrivateKeyRef, rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error)
}

// HardwarePrivateKeyRef references a specific hardware private key.
type HardwarePrivateKeyRef struct {
  // SerialNumber is the hardware key's serial number.
  SerialNumber uint32 `json:"serial_number"`
  // SlotKey is the key name for the hardware key PIV slot, e.g. 0x9a.
  SlotKey uint32 `json:"slot_key"`
  // PublicKey is the public key.
  PublicKey crypto.PublicKey `json:"-"` // below we use custom marshaling in PKIX, ASN.1 DER form
  // PrivateKeyPolicy is the private key policy satisfied by the hardware private key.
  PrivateKeyPolicy PrivateKeyPolicy `json:"private_key_policy"`
  // AttestationStatement contains the hardware private key's attestation
  // statement, which is used by the Teleport server to attest the touch
  // and pin requirements for this hardware private key during login.
  AttestationStatement *AttestationStatement `json:"attestation_statement"`
}

// Implements [crypto.Signer] and [keys.HardwareSigner].
type hardwareKeyAgentKey struct {
  agent   keyAgentService
  keyInfo hardwareKeyInfo
}

// Implements [crypto.Signer].
func (h *HardwarePrivateKey) Public() crypto.PublicKey {
  return h.ref.PublicKey
}

// Implements [crypto.Signer].
func (h *HardwarePrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
  return h.service.Sign(h.ref, rand, digest, opts)
}

// Implements [crypto.HardwareSigner].
func (h *HardwarePrivateKey) GetAttestationStatement() *AttestationStatement {
  return h.ref.AttestationStatement
}

// Implements [crypto.HardwareSigner].
func (h *HardwarePrivateKey) GetPrivateKeyPolicy() PrivateKeyPolicy {
  return h.ref.PrivateKeyPolicy
}
```

### UX

#### PIN caching

The PIN caching portion of this RFD does not present any additional UX concerns.
It is purely a UX benefitting change, removing the need for back-to-back PIN
prompts.

#### Agent clients

##### Teleport Connect

Teleport Connect is an ideal runner for the key agent because it:

1. Is a long-lived process
1. Provides a UI for touch/PIN prompts, and can foreground itself for these prompts

By default, if Teleport Connect detects a hardware key plugged in, it will automatically
serve the hardware key agent service. This way, the agent will be served
regardless of Teleport Connect's [login state](#running-the-agent-before-login).

Note: the plugged in hardware key is detected by
[listing smart cards available via the PC/SC interface](https://pkg.go.dev/github.com/go-piv/piv-go/piv#Cards).

For example, if a user primarily wants to use `tsh`, but get PIN caching and
PIN prompts in Teleport Connect, they could just launch Teleport Connect without
logging in. Teleport Connect would just foreground itself with hardware key
prompts for the user as needed without adding additional overhead.

If desired, the agent can be disabled manually with a config option:

| Property | Default | Description |
|----------|---------|-------------|
| `hardwareKeyAgent.enabled` | `true` | Starts the hardware key agent automatically |

Note: In dev mode, `hardwareKeyAgent.enabled` will default to `false` to prevent
the dev instance from claiming the hardware key agent socket over a "prod"
instance. This will be achieved by deciding which default to use based on the
location of the config file - `~/Library/Application\ Support/Teleport\ Connect/app_config.json`
for prod vs. `~/Library/Application\ Support/Electron/app_config.json` for dev.

Alternative: By default, set `hardwareKeyAgent.enabled: true`. Once the user logs into
Connect with a hardware key requirement for the first time, flip the flag to
true indefinitely. The benefit with this approach is that we won't run an
unused agent by default for users not using hardware key support. On the other
hand, it requires the user to log in at least once for the feature to work as
described in the example above.

##### `tsh piv agent`

`tsh piv agent` will be made available as a hidden command, primarily for
development. If in the future we get requests to fully support this command,
we may make it a public command and make any necessary improvements.

For now, we will not put exorbitant effort into providing a good UX with this command
(e.g. stealing focus for hardware key prompts, re-login on cert expiration),
but it should be fully functional.

##### Agent already running

If there is already a hardware key agent running at `$TEMP/.Teleport-PIV/agent.sock`,
a newly started agent client would fail to open a new listener on that same
socket. Instead of failing, the new client will ping the running agent to check
if it's active.

If the ping request fails, the socket will be treated as abandoned and
automatically replaced by the new agent.

If the ping request succeeds, an error will be returned:

```console
> tsh piv agent
Error: another agent instance is already running. PID: 86946.
```

Note: this should be a very uncommon edge case outside of development. The only
client that starts a hardware key agent automatically by default is the official
Teleport connect app bundle(s), and there is a
[single app lock](https://github.com/gravitational/teleport/blob/ec89c43d574c7e514ff4922e5b281c12da9d2189/web/packages/teleterm/src/main.ts#L54-L61)
designed to prevent multiple app bundles from running at one time. This issue
will only occur when the user starts a hardware key agent with the hidden
`tsh piv agent` command or sets `hardwareKeyAgent.enabled: true` for a Teleport
Connect dev mode instance.

Note: with Teleport Connect, this error would be displayed when it attempts
to start the agent, but Teleport Connect would not fail to start. The error
would be shown in Teleport Connect's warn logs.

##### Running the agent before login

The hardware key agent does not depend on Teleport login state, meaning a user
can run it before logging in.

For the login itself, Teleport clients will interface directly with the
hardware key to check/generate the private key. Then, the client will interface
with the key through the hardware key agent that is already running.

#### Dependent clients

Dependent clients should interact with the hardware key agent seamlessly so
that there is limited UX degradation when compared to the a client connecting
directly to the hardware key.

##### Agent stops running

If the agent stops unexpectedly during a dependent client's operation, it may
lead to an error for the dependent client. Rather than returning this error,
the client will log the error as debug and try again by connecting directly to
the hardware key.

Note: Teleport Connect and `tsh piv agent` will both attempt to restart the
agent if it runs into an error, with backoff.

#### Hardware key prompts

The agent is [responsible for prompting hardware key PIN and touch](#pin-and-touch-prompts)
on behalf of dependent clients.

Note: sometimes, Teleport connect uses `tsh` commands directly, which prompts
for PIN/touch via the CLI within a Teleport Connect console (e.g. `tsh ssh`).
A side effect of this change is that the teleport key agent will now prompt via
Teleport Connect on behalf of these `tsh` commands within Teleport Connect.
This is ultimately a benefit as the previous CLI prompts will be replaced with
the UI prompts of Teleport Connect.

The dependent client will include its full command to the agent `Sign` request
in order for the agent to relay to the user which dependent client is making
the signature request. The agent will then include this command in the existing
touch and PIN prompts.

Teleport Connect:

```diff
# touch prompt
Verify your identity on root.example.com

+Hardware key touch is required to continue with command "tsh ssh server01"
 
# pin prompt
-Unlock hardware key to access root.example.com
+Verify your identity on root.example.com

+Hardware key PIN is required to continue with command "tsh ssh server01"
```

`tsh piv agent`:

```diff
# touch prompt
+Hardware key touch is required to continue with command "tsh ssh server01"
Tap your YubiKey
 
# pin prompt
+Hardware key PIN is required to continue with command "tsh ssh server01"
Enter your YubiKey PIV PIN:
```

While the agent client prompts the user, the dependent client will hang until
the prompt is handled. Teleport Connect will foreground these prompts, so it
should be clear to the user how to complete the prompts and proceed with the
dependent client.

Future improvement: instead of leaving the prompt purely up to the agent client,
the agent client could propagate the prompt to the dependent client requesting
a signature through a bi-directional, streaming version of the `Sign` rpc. The
dependent could then prompt for PIN or touch like normal, e.g. in the terminal.
The user could then choose between the terminal and agent prompt depending on
which is more convenient. For example, the user could enter their PIN in the
terminal for a basic `tsh ssh` command, but enter their PIN in Teleport Connect
for `tsh proxy` commands. However, this adds a lot of complexity and could
provide no benefit if Teleport Connect is always foregrounding the prompts.

### Proto

#### `HardwareKeyAgentService`

```proto
// HardwareKeyAgentService provides an agent service for hardware key (PIV) signatures.
// This allows multiple Teleport clients to share a PIV connection rather than blocking
// each other, due to the exclusive nature of PIV connections. This also enabled shared
// hardware key states, such as a custom PIN cache shared across Teleport clients.
service HardwareKeyAgentService {
  // Ping the agent service to check if it is active.
  rpc Ping(PingRequest) returns (PingResponse) {}
  // Sign produces a signature with the provided options for the specified hardware private key
  //
  // This rpc implements Go's crypto.Signer interface.
  rpc Sign(SignRequest) returns (Signature) {}
}

// PingRequest is a request to Ping.
message PingRequest {}

// PingResponse is a response to Ping.
message PingResponse {
  // PID is the PID of the client process running the agent.
  uint32 pid = 1;
}

// SignRequest is a request to perform a signature with a specific hardware private key.
message SignRequest {
  // Digest is a hashed message to sign.
  bytes digest = 1;
  // Hash is the hash function used to prepare the digest.
  Hash hash = 2;
  // SaltLength specifies the length of the salt added to the digest before a signature.
  // This salt length is precomputed by the client, following the crypto/rsa implementation.
  // Only used, and required, for PSS RSA signatures.
  uint32 salt_length = 3;
  // KeyRef references a specific hardware private key.
  KeyRef key_ref = 4;
  // KeyInfo contains additional, optional key info which generally will improve UX by
  // giving the agent context about the key, such as whether PIN/touch prompts are
  // expected, or what cluster login is trying to interface with the key.
  KeyInfo key_info = 5;
  // CommandName is the name of the command or action requiring a signature.
  // e.g. "tsh ssh server01". The agent can include this detail in PIN/touch
  // prompts to show the origin of the signature request to the user.
  string command_name = 6;
}

// Signature is a private key signature.
message Signature {
  // For an RSA key, signature should be either a PKCS #1 v1.5 or PSS signature,
  // depending on the hash and salt chosen. For an (EC)DSA key, it should be a
  // DER-serialized, ASN.1 signature structure.
  bytes signature = 1;
}

// KeyRef references a specific hardware private key.
message KeyRef {
  // SerialNumber is the serial number of the hardware key.
  uint32 serial_number = 1;
  // PivSlot is a specific PIV slot on the hardware key.
  PIVSlot piv_slot = 2;
  // PublicKey is the public key encoded in PKIX, ASN.1 DER form. If the public key does
  // not match the private key currently in the hardware key's PIV slot, the signature
  // will fail early.
  bytes public_key_der = 3;
}

// KeyInfo contains additional information about a hardware private key.
message KeyInfo {
  // TouchRequired is a client hint as to whether the hardware private key requires touch.
  // The agent will use this to provide the ideal UX for the touch prompt. If this client
  // hint is incorrect, touch will still be prompted.
  bool touch_required = 1;
  // PinRequired is a client hint as to whether the hardware private key requires PIN.
  // The agent will use this to provide the ideal UX for the PIN prompt. If this client
  // hint is incorrect, PIN will still be prompted for YubiKey versions >= 4.3.0, and
  // failing with an auth error otherwise.
  bool pin_required = 2;
  // ProxyHost is a Teleport proxy hostname that the key is associated with.
  // May be used to add context to PIN/touch prompts.
  string proxy_host = 3;
  // Username is a Teleport username that the key is associated with.
  // May be used to add context to PIN/touch prompts.
  string username = 4;
  // ClusterName is a Teleport cluster name that the key is associated with.
  // May be used to add context to PIN/touch prompts.
  string cluster_name = 5;
}

// PIVSlot is a specific PIV slot on a hardware key.
enum PIVSlot {
  // PIV slot not specified.
  PIV_SLOT_UNSPECIFIED = 0;
  // PIV slot 9a. This is the default slot for pin_policy=never, touch_policy=never.
  PIV_SLOT_9A = 1;
  // PIV slot 9c. This is the default slot for pin_policy=never, touch_policy=cached.
  PIV_SLOT_9C = 2;
  // PIV slot 9d. This is the default slot for pin_policy=once, touch_policy=cached.
  PIV_SLOT_9D = 3;
  // PIV slot 9e. This is the default slot for pin_policy=once, touch_policy=never.
  PIV_SLOT_9E = 4;
}

// Hash refers to a specific hash function used during signing.
enum Hash {
  HASH_UNSPECIFIED = 0;
  HASH_NONE = 1;
  HASH_SHA256 = 2;
  HASH_SHA512 = 3;
}
```

#### `PinCacheTimeoutNanoseconds`

```diff
### types.proto
message HardwareKey {
  ...
+  // PinCacheTimeoutNanoseconds is the amount of time in nanoseconds that Teleport
+  // clients will cache a user's PIV PIN when hardware key PIN policy is enabled.
+  // This timeout can not exceed 1 hour. When empty or 0, the pin will not be cached.
+  int64 PinCacheTimeoutNanoseconds = 3 [
+    (gogoproto.jsontag) = "pin_cache_timeout_nano_seconds,omitempty",
+    (gogoproto.casttype) = "Duration"
+  ];
}
```

### Security

#### Local hardware key agent

For the intended use case of using the hardware key agent as a local agent,
there is not much of concern. The agent serves as a proxy for the normal PC/SC
(Personal Computer/Smart Card) interface, which does not provide any privilege
(e.g. root) to utilize.

For a modicum level of security, the agent will use basic TLS for end-to-end
encryption. The agent service will generate a key in memory and a self-signed
certificate saved next to the unix socket at `$TEMP/.Teleport-PIV/ca.pem` where
local Teleport clients can access it.

#### Hardware key agent forwarding

We must also consider the unintended use case of forwarding the agent over ssh
with unix domain socket forwarding. Like ssh agent forwarding, this is an
insecure use case which will be strongly advised against. However, it is not
possible to entirely avoid the possibility of a user misusing the agent in this
way, the same way that we can not stop a user from running `tsh scp $HOME/.tsh server01:`.

Note: any concerns here can be largely ignore in the case where the hardware
private key has a PIN or touch policy.

#### PIN caching in memory

Since the hardware key PIN is cached in process memory, this introduces a risk
of the PIN being extracted from a swap file, core dump, cold boot, or other
similar attacks. These attacks generally are not guaranteed to succeed and
require system compromise with root privileges.

The [memguard](github.com/awnumar/memguard) library, or something similar, could
be used to further reduce the effectiveness of these attacks. We have decided
[not to pursue such mitigations](https://github.com/gravitational/teleport/pull/51537#discussion_r1953488171)
as the risk appears to be negligible, especially since this feature is intended
for local use only.

#### Hardware key agent PIN caching

When combined with PIN caching, the hardware key agent effectively caches the
PIN for any process that interfaces with it, potentially including non official
clients.

To some extent, enabling PIN caching is inherently a reduction in security for
this reason, and we don't seek to impose restrictions that may reduce the
use-fullness of the feature. For example, it may be useful for a user to create
a custom API client which performs some actions with PIN caching enabled.

However, the hardware key agent should prevent third party clients from using
the hardware key agent for non Teleport client keys. e.g PIV keys traditionally
used for email encryption.

The hardware key agent will determine whether a key in a PIV slot is a Teleport
client key by checking for a recent login session matching the key or checking
for the [self-signed metadata certificate](./0080-hardware-key-support.md#piv-slot-logic)
generated by Teleport clients on hardware key login.

Note: When user hardware keys are externally managed, administrators are currently
only required to generate a key in the PIV slot befitting their requirements.
However, they don't currently need to generate the metadata certificate that
a Teleport client would usually create, leaving no way for the hardware key
agent to determine whether the key is meant for Teleport or some other PIV
application. Therefore, admins will need to create the metadata certificate
in order for their users to utilize the hardware key agent.

Alternative: we could add some cross-process validation so clients can confirm
the hardware key agent is being served by a legitimate, signed Teleport client
and vice versa. However, this goes above and beyond what is necessary while
also limiting legitimate possibilities, e.g. using PIN caching for a custom
Teleport API client.

### Backward Compatibility

The hardware key agent is purely a client-side feature with no backwards
compatibility concerns. However, there may be some compatibility concerns
between different implementations of the hardware key agent API in the
future as new versions are released.

### Audit Events

N/A

### Additional Considerations

#### Hardware key agent mTLS

The initial design above lays out using basic TLS and file permissions to give
basic security coverage to the key agent. For the most part this is sufficient,
but when considering edge cases like [the unix socket being forwarded](#hardware-key-agent-forwarding),
mTLS would surely be nice to have.

In order for the hardware key agent to share a client keypair with each
independent Teleport client accessing the agent, the keypair would need to be
stored in a shared location. The first location that comes to mind is disk,
directly next the unix socket. Since the unix socket is protected by the same
file permissions that the client keypair would be, this does not provide any
security benefit.

Instead, we could utilize one of the hardware key's extra PIV slots to store the
client keypair. This keypair would be generated by the agent on startup, and
each Teleport client would access the hardware key directly to use this keypair
to perform an mTLS handshake with the hardware key agent.

This key would not require PIN or touch, just a direct connection to the
hardware key, ensuring that unintended remote forwarding use cases are all but
impossible.

While this would be nice touch for security, it introduces additional concerns,
particularly about claiming an additional PIV slot when the PIV specification
only guarantees 4 slots. We would need to add an option to specify a specific
piv slot, as well as fallback to basic TLS in the case where no PIV slot is
available.

Since mTLS is not crucial for the MVP of this feature, it may instead be
considered as a follow up measure. The specifics of the follow up can be
detailed and discussed in an implementation PR.

#### Problems with built-in PIV PIN caching

Currently, we use the built-in PIN caching mechanism detailed in the PIV
standard and implemented by Yubico. Unfortunately, this mechanism is quite
limited, inconsistent, and in some cases outright [bugged](https://github.com/gravitational/teleport/pull/36427).

In short, the PIN is not cached directly on the hardware key for a set duration
of time like touch is. Instead, it is cached within the PC/SC (Personal Computer/Smart
Card) transaction and wiped once the transaction is closed. This leaves room for
inconsistencies between different PIV implementations or versions.

For example, the developers and collaborators of the piv-go library found that
[depending on the YubiKey firmware version](https://github.com/go-piv/piv-go/issues/47#issuecomment-1368247984),
the PIN may or may not be cached across PC/SC transactions and even different
processes. In my own testing, I found that the PC/SC transaction has a few
seconds of grace period before it releases its resources, allowing for any
process to claim that transaction before it is released, with the PIN cache
still intact.

Lastly, the PIN is only cached so long as the PC/SC transaction is exclusive,
meaning long running Teleport clients like `tsh proxy` commands or Teleport
Connect can't hold open the PC/SC transaction to keep the PIN cached without
locking out all other Teleport/PIV clients in the meantime.

Suffice it to say, the inconsistencies of this PIN caching mechanism make it
poorly suited for Teleport clients. The resulting UX from this approach has
been workable at best and unusable at worst.
