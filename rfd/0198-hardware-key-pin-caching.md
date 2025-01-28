---
authors: Brian Joerger (bjoerger@goteleport.com)
state: draft
---

# RFD 198 - Hardware Key Pin Caching

## Required Approvers

* Engineering: @rosstimothy
* Product: @xinding33 || @klizhentas

## What

Teleport offers the option to enforce the use of a hardware-backed private key
to connect to Teleport servers and services, with an additional option to
require the user's hardware key (PIV) pin for every operation
(`hardware_key_pin`).

This RFD proposes the implementation of a pin caching mechanism to improve UX
when using the `hardware_key_pin` option.

See [RFD 80](./0080-hardware-key-support.md) for more details on Hardware Key
Support.

## Why

Although `hardware_key_pin` provides a great security option, it currently
requires users to enter their pin once for every single action
(e.g. `tsh ssh`, `tsh ls`). This is very disruptive when running several
commands in short succession, especially when:

* running several kubernetes or database commands through a Teleport local proxy.
* using automated scripts which runs commands in bulk.

## Details

### UX

Pin caching will significantly improve the UX of Hardware Key pin support by
reducing the frequency of cumbersome pin prompts in the middle of active usage.
Specifically, a user will be prompted at most once per configured timeout
duration, rather than the the worst case scenario of being prompted every few
seconds.

Note that the Teleport client caching the pin will not prompt for pin
immediately after the cache times out. Instead, it will prompt for pin the next
time the user performs an action that would require pin.

UX implications of the Teleport Key Agent will be covered in the Teleport Key
Agent RFD.

### Single Process Solution

Caching the pin for a single process is not complicated. For example, we can
easily enable pin caching to make commands like `tsh proxy db` cache the pin
for any incoming db connections coming through the local proxy:

```console
> tsh proxy db --tunnel --port=60000
Enter your YubiKey PIV PIN:
Started authenticated tunnel for the PostgreSQL database "postgres" in cluster "root.example.com" on 127.0.0.1:60649.

Use the following command to connect to the database or to the address above using other database GUI/CLI clients:
  $ psql postgres://postgres@localhost:60000/postgres

# User can open several postgres connections over the local proxy without
# being re-prompted for pin, until the pin cache times out.
#
# Note that the user is not re-prompted for pin immediately after the cache times
# out. Instead they are prompted the next time they try to create a psql connection
# after the timeout.
Enter your YubiKey PIV PIN:
```

### Multi Process Solution

In order to cache the pin across multiple Teleport client processes (`tsh`,
`tctl`, Teleport Connect), we can use the new [Teleport Key Agent](./0199-teleport-key-agent).

Teleport Key Agent is responsible for providing an interface to signing with
the user's private keys. When `hardware_key_pin` is required, the the user's
PIV pin must be entered in order to perform a signature. Therefore, the agent
is responsible for prompting and caching the pin.

Any clients depending on the agent will share the benefits of the agent client's
pin prompts and pin cache. For example, if one dependent client requests a
signature and the user enters their pin into the agent, all dependent clients
can continue with the cached pin for their signatures, until the pin cache
times out.

This solution will have far better UX when paired with Teleport Connect
because Teleport Connect has the ability to pop into the foreground when the
user needs to enter their PIV pin.

### Security

Caching the pin in process memory introduces a risk for the pin to be
compromised. Fortunately, this risk can be almost entirely mitigated with
secure memory practices implemented in [memguard](github.com/awnumar/memguard).

Using [memguard](github.com/awnumar/memguard), Teleport clients will store the
cached pin in a secure enclave.

### Configuration

#### Cluster Auth Preference

To enable pin caching for Teleport clients, set `cap.hardware_key.pin_cache_timeout`
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
    # the user's PIV pin. The timeout countdown is started when the pin is
    # stored and is not extended by subsequent accesses.
    pin_cache_timeout: 15m
```

Teleport Clients will retrieve this setting through `/webapi/ping`.

### Proto

```diff
### types.proto
message HardwareKey {
  ...
+  // PinCacheTimeout is the amount of time in nanoseconds that Teleport clients
+  // will cache the user's PIV PIN when hardware key PIN policy is enabled.
+  int64 PinCacheTimeout = 3 [
+    (gogoproto.jsontag) = "pin_cache_timeout,omitempty",
+    (gogoproto.casttype) = "Duration"
+  ];
}
```

### Backward Compatibility

Pin caching is purely a client-side feature with no backwards compatibility
concerns.

### Audit Events

N/A

### Additional considerations

#### Utilize internal PIV pin caching

Since we are using the PIV pin policy `once`, the pin only needs to be provided
once per PC/SC transaction. This means that we could avoid caching the pin
explicitly by just holding open the PC/SC transaction for however long we want
the pin to be cached. Unfortunately, this built-in pin caching only works when
claiming an exclusive transaction on the PIV key, locking out any other PIV
applications from connecting to the key until the transaction is released.

Reportedly, it may be possible to detect when another process is trying to claim
a PC/SC transaction with the `SCardGetStatusChange` function. Theoretically, this
means that a Teleport client, or Teleport Key Agent, could hold the PC/SC
transaction for an extended period of time, dropping the transaction whenever
another process needs to access it. The other process can take advantage of
the cached pin, and then the original Teleport Client can reclaim the transaction
to maintain the cached pin further.

While this solution may work, it has the following drawbacks compared to the
explicit pin caching solution above.

* The implementation is more complex and would require upstream changes to piv-go
* Competing over PC/SC transactions is not as efficient as sharing one through the
Teleport Key Agent
* The duration of the PIN cache would not be as consistent, since the PC/SC transaction
could be reclaimed at any time to prolong the internal pin cache.

#### Corollary - `memguard`

Since we are introducing [memguard](github.com/awnumar/memguard) as a dependency
with this change, we should consider utilizing it to secure private keys and
restricted certificates (e.g. MFA verified certs) stored in memory.

This is particularly important for [Headless Authentication](./0105-headless-authentication.md),
which takes place on a remote host. Currently we only protect against the
possibility of memory swaps using [`unix.Mlockall`](https://pkg.go.dev/golang.org/x/sys/unix#Mlockall)
and print a warning for non linux environments.
