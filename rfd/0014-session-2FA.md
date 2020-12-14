---
authors: Andrew Lytvynov (andrew@gravitational.com)
state: draft
---

# RFD 14 - Per-session 2FA

## What

Require a 2FA check before starting a new user "session" for all protocols that
Teleport supports.

## Why

Client machines may be compromised (either physically stolen or remotely
controlled), along with Teleport credentials on those machines.

Since Teleport keys and certificates are stored on disk, an attacker can
download them to their own machine and have up to 12hrs of access via Teleport.

To mitigate this risk, a legitimate user needs to authenticate with a 2nd
factor (usually a U2F hardware token) for every session. This is in addition to
regular authentication during `tsh login`.

An attacker, who doesn't also have the 2nd factor, can't abuse Teleport
credentials and escalate to the rest of the infrastructure.

## Details

First, some definitions and justification:

### sessions

Session here means:

- **not** a `tsh login` session
- SSH: SSH connection from the same client to a single server, with potentially
  multiple SSH channel multiplexed on top
- Kubernetes: arbitrary number of k8s requests from the same client to a single
  k8s cluster within a short time window (seconds or minutes)
- Web app: arbitrary number of HTTPS requests from the same client to a single
  application within a short time window (seconds or minutes)
- DB: database connection from the same client to a single database, with
  potentially multiple queries executed on top

### 2nd factor

There are a variety of 2FA options available, but for this design we'll focus
on U2F hardware tokens, because:

- portability: U2F devices are supported on all major OSs and browsers (vs
  TouchID or Windows Hello)
- UX: tapping a USB token multiple times per day is low friction (vs typing in
  TOTP codes)
- availability: many engineers already own a U2F token (like a YubiKey), since
  they are usable on popular websites (vs HSMs or smartcards)

We may consider adding support for other 2FA options, if there's demand.

### authn protocol

The design leverages short-lived SSH and TLS certificates per session. Cert
expiry is used to limit the cert to a single "session".

For all protocols, the flow is roughly:

1. client requests a new certificate for the session
2. client and server perform the U2F challenge exchange, with user tapping the
   security token
3. server issues a short-lived certificate with encoded constraints
4. client uses the in-memory certificate to start the session and discards the
   certificate

The short-lived certificate is used for regular SSH or mTLS handshakes, with
server validating it using the presented constraints.

#### constraints

Each session has the following constraints, encoded in the TLS or SSH
certificate issued after 2FA and enforced server-side:

- cert expiry: each certificate is valid for 1min, during which the client can
  _establish_ a session
- session TTL: each session is terminated server-side after 30min, whether
  active or idle
  - this is important to prevent a compromised session from being artificially
    kept alive forever, with some simulated activity
- target: a specific server, k8s cluster, database or web app this session is for
- client IP: only connections from the same IP that passed a 2FA check can
  establish a session

### session UX

UX is the same for all protocols: initiate session -> tap security key -> proceed.
But the plumbing details are different:

#### ssh

The U2F handshake is performed by `tsh ssh`, before the actual SSH connection:

```
awly@localhost $ tsh ssh server1
please tap your security key... <tap>
awly@server1 #
```

For OpenSSH, `tsh ssh` can be injected using `ProxyCommand` option in the
config, with identical UX.

#### kubernetes

`kubectl` is configured to call `tsh kube credentials` as an exec plugin, since
5.0.0. This plugin returns a private key and cert to `kubectl`, which uses them
in mTLS handshake.
`tsh kube credentials` will handle the U2F handshake, and cache the resulting
certificate in `~/.tsh/` for its validity period.

```
$ kubectl get pods
please tap your security key... <tap>
... list of pods ...

$ kubectl get pods # no 2FA needed right after the previous command
... list of pods ...

$ sleep 1m && kubectl get pods # 2FA needed since the short-lived cert expired
please tap your security key... <tap>
... list of pods ...

```

#### web apps

TODO: native browser U2F on first request, via JS code from the proxy

#### DB

TODO: command to generate short-lived cert, and maybe a wrapper for psql

### API

TODO: new *streaming* gRPC endpoint, similar to `ReissueUserCerts` but for only
1 cert and with U2F exchange

### RBAC

TODO: new role options: 2fa_per_session and session_ttl

### U2F device management

TODO: support multiple keys per user, easier enrollment on the CLI

## Alternatives considered

### Private keys stored on hardware tokens

TODO: PKCS#11, HSMs, smartcards, yubikey PIV

### Forward proxy on the client machine

TODO
