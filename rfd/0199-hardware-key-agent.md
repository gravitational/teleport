---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 199 - Hardware Key PIN Caching

## Required Approvers

* Engineering: @rosstimothy && @ravicious
* Product: @xinding33 || @klizhentas

## What

Implement a hardware key PIN caching mechanism so that a user is not prompted
for PIN more than once within a configured span of time.

In order to cache the PIN across process boundaries (Teleport Connect, separate
`tsh commands`), this RFD also introduces a client-side hardware key agent.

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

### Hardware key PIN caching

When enabled at the cluster-level, Teleport clients (`tsh`, `tctl`, and
Teleport connect) will cache the user's hardware key PIN in memory for a
specified duration of time. When the PIN is cached, the Teleport client will
provide the PIN to the hardware key without prompting the user again.

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

The hardware key agent will provide two functionalities:

1. Fetching info for a specific hardware private key, including the public
key, private key policy, and attestation statement.
1. Signing with a specific hardware private key

Note: this is the base functionality needed to implement the `crypto.Signer`
interface and Teleport's `keys.HardwareSigner` interface.

The agent will be served as a [gRPC](#hardwarekeyagentservice) service on a unix
socket, `$TEMP/.Teleport-PIV/agent.sock`, with [basic TLS](#security).

#### Terminology

An "agent client" is a Teleport client process serving the hardware key agent.

A "dependent client" is a Teleport client process interfacing with the hardware key agent.

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
it is prompted and handled by the agent client. Therefore, Teleport Connect
will foreground these prompts to maintain seamless UX.

Note: touch is cached for 15 seconds on the hardware key itself and PIN is
optionally cached when [Hardware Key pin caching](#hardware-key-pin-caching)
is enabled. Like any normal Teleport client, the agent client will only prompt
for PIN or touch when it isn't cached.

#### `hardwareKeyAgentService` pseudo-code implementation

```go
// Pseudo-code

// hardwareKeyRef uniquely identifies a hardware key
type hardwareKeyRef struct {
  serialNumber uint32
  pivSlot      uint32
}

type hardwareKeyInfo struct {
  publicKey            crypto.PublicKey
  privateKeyPolicy     keys.PrivateKeyPolicy
  attestationStatement keys.AttestationStatement
}

// hardwareKeyAgentService has two implementations:
//  - direct implementation with piv-go, adapted slightly from our existing implementation
//  - hardware key agent gRPC service implementation
type hardwareKeyAgentService interface {
  GetInfo(ref hardwareKeyRef) hardwareKeyInfo
  Sign(ref hardwareKeyRef, digest []byte, opts SignerOpts) (signature []byte, err error)
}

// Implements [crypto.Signer] and [keys.HardwareSigner].
type hardwareKeyAgentKey struct {
  agent  keyAgentService
  keyRef hardwareKeyRef
  // hardwareKeyInfo is cached after the first call
  hardwareKeyInfo hardwareKeyInfo
}

// Implement [crypto.Signer]
func (s *agentSigner) Public() crypto.PublicKey {
  return s.hardwareKeyInfo.publicKey
}

// Implement [crypto.Signer]
func (s *agentSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
  return s.agent.Sign(as.KeyRef, digest, opts)
}

// Implement [keys.HardwareSigner]
func (s *agentSigner) GetAttestationStatement() *AttestationStatement {
  return s.hardwareKeyInfo.attestationStatement
}

// Implement [keys.HardwareSigner]
func (s *agentSigner) GetPrivateKeyPolicy() PrivateKeyPolicy {
  return s.hardwareKeyInfo.privateKeyPolicy
}

func (s *agentSigner) fetchHardwareKeyInfo() (hardwareKeyInfo, error) {
  if s.hardwareKeyInfo == nil {
    s.hardwareKeyInfo = s.agent.GetInfo(s.ref)
  }
  return s.hardwareKeyInfo
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

For example, if a user primarily wants to use `tsh`, but get PIN caching and
PIN prompts in Teleport Connect, they could just launch Teleport Connect without
logging in. Teleport Connect would just foreground itself with hardware key
prompts for the user as needed without adding additional overhead.

Alternative: By default, set `teleport.agent=false`. Once the user logs into
Connect with a hardware key requirement for the first time, flip the flag to
true indefinitely. The benefit with this approach is that we won't run an
unused agent by default for users not using hardware key support. On the other
hand, it requires the user to log in at least once for the feature to work as
described in the example above.

If desired, the agent can be disabled manually with a config option:

| Property | Default | Description |
|----------|---------|-------------|
| `hardwareKeyAgent.enabled` | `false` | Starts the hardware key agent automatically |

##### `tsh agent`

`tsh agent` will be made available as a hidden command, primarily for
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

Note: this should be a very uncommon edge case, as it would only occur if the
user already has `tsh agent` running or another instance of Teleport connect.

Note: with Teleport Connect, this error would be displayed when it attempts
to start the agent, but Teleport Connect would not fail to start. The error
would be shown in Teleport Connect's debug logs.

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

#### Hardware key prompts

The agent is [responsible for prompting hardware key PIN and touch](#pin-and-touch-prompts)
on behalf of dependent clients.

The dependent client will include its full command to the agent `Sign` request
in order for the agent to relay to the user which dependent client is making
the signature request. The agent will then include this command in the existing
touch and PIN prompts.

Teleport connect:

```text
# normal touch prompt
Verify your identity to on root.example.com
 
# agent touch prompt
Verify your identity to continue with command "tsh ssh server01"
 
# normal pin prompt
Unlock hardware key to access root.example.com
 
# agent pin prompt
Unlock hardware key to continue with command "tsh ssh server01"
```

`tsh agent`:

```text
# normal touch prompt
Tap your YubiKey
 
# agent touch prompt
Tap your YubiKey to continue with command "tsh ssh server01"
 
# normal pin prompt
Enter your YubiKey PIV PIN:
 
# agent pin prompt
Enter your YubiKey PIV PIN to continue with command "tsh ssh server01":
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
// each other, due to the exclusive nature of PIV connections. This also enables shared
// hardware key states, such as a custom PIN cache shared across Teleport clients.
service HardwareKeyAgentService {
  // Ping the agent service to check if it is active.
  rpc Ping(PingRequest) {PingResponse}
  // Sign produces a signature with the provided options for the specified hardware private key
  //
  // This rpc implements Go's crypto.Signer interface.
  rpc Sign(SignRequest) returns (Signature) {}
  // GetInfo gets info for the specified hardware private key, including the public key,
  // attestation statement, and supported pin and touch policies.
  rpc GetInfo(GetInfoRequest) returns (GetInfoResponse) {}
}

// PingRequest is a request to Ping.
message PingRequest {}

// PingResponse is a response to Ping.
message PingResponse {
  // PID is the PID of the client process running the agent.
  PID uint32 = 1;
}

// SignRequest is a request to perform a signature with a specific hardware private key.
message SignRequest {
  // KeyRef references a specific hardware private key.
  KeyRef key_ref = 1;
  // PublicKeyDer is a public key encoded in PKIX, ASN.1 DER form.
  // This is provided to verify that the hardware private key has not
  // been swapped out against the public key expected by the client.
  bytes public_key_der = 2;
  // Digest is a hashed message to sign.
  bytes digest = 3;
  // Hash is the hash function used to prepare the digest.
  Hash hash = 4;
  // SaltLength specifies the length of the salt added to the digest before a signature.
  // This salt length is precomputed by the client, following the crypto/rsa implementation.
  // Only used, and required, for PSS RSA signatures.
  uint32 salt_length = 5;
  // CommandName is the name of the command or action requiring a signature.
  // e.g. "tsh ssh server01". The agent can include this detail in PIN and touch
  // prompts to show the origin of the signature request to the user.
  string command_name = 6;
}

// Signature is a private key signature.
message Signature {
  // For an (EC)DSA key, the default key algorithm for hardware private keys, this
  // will be a DER-serialised, ASN.1 signature structure.
  //
  // When the client is using a manually generated RSA key, this can be either a
  // PKCS #1 v1.5, or if the cluster is on the legacy signature algorithm suite,
  // a PSS signature,
  bytes signature = 1;
}

// KeyRef references a specific hardware private key.
message KeyRef {
  // SerialNumber is the serial number of the hardware key.
  uint32 serial_number = 1;
  // PivSlot is a specific PIV slot on the hardware key.
  PIVSlot piv_slot = 2;
}

// GetInfoRequest is a response to GetInfo
message GetInfoRequest {
  // KeyRef references a specific hardware private key.
  KeyRef key_ref = 1;
}

// GetInfoResponse is a response to GetInfo.
message GetInfoResponse {
  // PublicKey is the hardware public key encoded in PKIX, ASN.1 DER form.
  bytes public_key_der = 1;
  // AttestationStatement is a hardware key attestation statement corresponding
  // to a hardware private key.
  teleport.attestation.v1.AttestationStatement attestation_statement = 2;
  // PinRequired indicates whether the hardware private key requires PIN.
  bool pin_required = 3;
  // TouchRequired indicates whether the hardware private key requires touch.
  bool touch_required = 4;
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
  HASH_NAME_UNSPECIFIED = 0;
  HASH_NAME_SHA256 = 1;
  HASH_NAME_SHA512 = 2;
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
there is not much of concern to consider. The agent merely serves as a proxy
for the normal PC/SC (Personal Computer/Smart Card) interface. The only notable
difference is that the agent can [cache the hardware key PIN](#pin-caching)
directly when configured.

Still, the hardware key agent will implement some sensible restrictions to
increase security:

* Basic TLS for end-to-end encryption. The agent service will generate a key in
memory and a self-signed certificate next to the unix socket at `$TEMP/.Teleport-PIV/ca.pem`
where local Teleport clients can access it.
* The hardware key agent will not allow access to hardware private keys on PIV slots
that were not generated for a Teleport client, which can be identified by the presence
of a [self-signed metadata certificate](./0080-hardware-key-support.md#piv-slot-logic)
on the PIV slot.

Note: the agent will only serve hardware private keys managed by a Teleport
client, which can be identified by the [self-signed metadata certificate](./0080-hardware-key-support.md#piv-slot-logic)
stored on the PIV slot by Teleport clients. This is intended to reduce the chance
of the hardware key agent being misused for non Teleport use cases.

#### Hardware key agent forwarding

We must also consider the unintended use case of forwarding the agent over ssh
with unix domain socket forwarding. Like ssh agent forwarding, this is an
insecure use case which will be strongly advised against. However, it is not
possible to entirely avoid the possibility of a user misusing the agent in this
way, the same way that we can not stop a user from running `tsh scp $HOME/.tsh server01:`.

Note: any concerns here can be largely ignore in the case where the hardware
private key has a PIN or touch policy.

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
