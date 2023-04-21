---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: implemented
---

# RFD 14 - Per-session MFA

## What

Require a MFA check before starting a new user "session" for all protocols that
Teleport supports.

## Why

Client machines may be compromised (either physically stolen or remotely
controlled), along with Teleport credentials on those machines.

Since Teleport keys and certificates are stored on disk, an attacker can
exfiltrate them to their own machine and have up to 12hrs of access via Teleport.

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
- Web app: the built-in session concept, with a shorter session expiry (minutes or hours)
- DB: database connection from the same client to a single database, with
  potentially multiple queries executed on top

### 2nd factor

There are a variety of MFA options available, but for this design we'll focus
on U2F hardware tokens, because:

- portability: U2F devices are supported on all major OSs and browsers (vs
  TouchID or Windows Hello)
- UX: tapping a USB token multiple times per day is low friction (vs typing in
  TOTP codes)
- availability: many engineers already own a U2F token (like a YubiKey), since
  they are usable on popular websites (vs HSMs or smartcards)
- compliance: hardware are available with FIPS certification. Helping strengthen Teleports current FedRAMP support. e.g. (YubiKey FIPS)[https://www.yubico.com/products/yubikey-fips/]

We may consider adding support for other MFA options, if there's demand.

### U2F device management

A prerequisite for usable MFA integration is solid MFA device management. This
work is tracked separately, as [RFD 15](0015-2fa-management.md), to keep
designs reasonably scoped and understandable.

For this RFD, we assume that:
- teleport MFA device management is separate from SSO MFA
- teleport supports MFA device management on CLI and web
- a user can have multiple MFA devices registered, including multiple security
  tokens
- a user can remove registered MFA devices

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
certificate issued after MFA and enforced server-side:

- cert expiry: each certificate is valid for 1min, during which the client can
  _establish_ a session
- session TTL: each session is terminated server-side after 30min, whether
  active or idle
  - this is important to prevent a compromised session from being artificially
    kept alive forever, with some simulated activity
- target: a specific server, k8s cluster, database or web app this session is for
- client IP: only connections from the same IP that passed a MFA check can
  establish a session

### session UX

UX is the same for all protocols: initiate session -> tap security key -> proceed.
But the plumbing details are different:

#### ssh

The U2F handshake is performed by `tsh ssh`, before the actual SSH connection:

```sh
awly@localhost $ tsh ssh server1
please tap your security key... <tap>
awly@server1 #
```

For OpenSSH, `tsh ssh` can be injected using `ProxyCommand` option in the
config, with identical UX.

For the Web UI, the U2F exchange happens over the existing websocket
connection, using JS messages (exact format TBD), before terminal traffic is
allowed.

#### kubernetes

`kubectl` is configured to call `tsh kube credentials` as an exec plugin, since
5.0.0. This plugin returns a private key and cert to `kubectl`, which uses them
in mTLS handshake.
`tsh kube credentials` will handle the U2F handshake, and cache the resulting
certificate in `~/.tsh/` for its validity period.

```sh
$ kubectl get pods
please tap your security key... <tap>
... list of pods ...

$ kubectl get pods # no MFA needed right after the previous command
... list of pods ...

$ sleep 1m && kubectl get pods # MFA needed since the short-lived cert expired
please tap your security key... <tap>
... list of pods ...

```

#### web apps

Web apps already have a session concept, with dedicated a login endpoint
(`/x-teleport-auth`). The application endpoint serves a bit of JS code to
redirect to the login endpoint.

This JS code will be modified to trigger browser's native U2F API, if the proxy
responds with a U2F challenge:

- user opens `app.example.com` (with an existing Teleport cookie)
- proxy serves a minimal JS page
- JS requests `app.example.com/x-teleport-auth`
- proxy responds with [407 Proxy Authentication
  Required](https://tools.ietf.org/html/rfc7235#section-3.2) and a U2F
  challenge in `Proxy-Authenticate` header
- JS triggers the browser U2F API
- browser shows a security key popup
- user taps the key
- JS requests `app.example.com/x-teleport-auth` with the signed U2F challenge
  in `Proxy-Authenticate` header
- proxy sends back an application-specific cookie and redirects to the
  application

#### DB

The initial integration for databases will be limited:

```sh
$ tsh db login prod
please tap your security key... <tap>

$ eval $(tsh db env)
$ psql -U awly prod
```

We'll also provide an example wrapper script:

```sh
$ cat teleport/examples/db/psql.sh
#!/bin/sh
# simplified version, without checking arguments

# Usage: psql.sh user dbname
tsh db login $2
eval $(tsh db env)
psql -U $1 $2
```

Users will need to adapt this for their DB clients. Teleport will always
generate short-lived key/cert in a predictable location under `~/.tsh/`.

### API

The protocol to obtain a new cert after a U2F check is:
```
client                               server
   |<-- mTLS using regular tsh cert -->|
   |--------- initiate U2F auth ------>|
   |<------------ challenge -----------|
   |---- u2f signature + metadata ---->|
   |<-------------- cert --------------|
```

This can be implemented as 2 request/response round-trips of the existing
`GenerateUserCerts` RPC, with some downsides:
- the server has to store state (challenge) in the backend
- extra latency (backend RTT and RPC overhead)
- complicating the existing RPC semantics

Instead, we'll use a single _streaming_ gRPC endpoint, using `oneof`
request/response messages.

```protobuf
rpc GenerateUserCertMFA(stream UserCertsMFARequest) returns (stream UserCertsMFAResponse);

message UserCertsMFARequest {
  // User sends UserCertsRequest initially, and MFAChallengeResponse after
  // getting MFAChallengeRequest from the server.
  oneof Request {
    UserCertsRequest Request = 1;
    MFAChallengeResponse MFAChallenge = 2;
  }
}

message UserCertsMFAResponse {
  // Server sends MFAChallengeRequest after receiving UserCertsRequest, and
  // UserCert after receiving (and validating) MFAChallengeResponse.
  oneof Response {
    MFAChallengeRequest MFAChallenge = 1;
    UserCert Cert = 2;
  }
}

message MFAChallengeResponse {
  // Extensible for other MFA protocols.
  oneof Response {
    U2FChallengeResponse U2F = 1;
  }
}

message MFAChallengeRequest {
  // Extensible for other MFA protocols.
  oneof Request {
    U2FChallengeRequest U2F = 1;
  }
}

message UserCert {
  // Only returns a single cert, specific to this session type.
  oneof Cert {
    bytes SSH = 1;
    bytes TLSKube = 2;
  }
}
```

The exchange is:

```
client                               server
   |<--------- gRPC over mTLS -------->|
   |---- start GenerateUserCertMFA --->|
   |-------- UserCertRequest --------->|
   |<------- MFAChallengeRequest ------|
   |------ MFAChallengeResponse ------>|
   |<------------- UserCert -----------|
```

### enforcement

MFA checks per session can be enforced per-role or globally.

#### per-role

This approach is for operators that want extra protection for some high-value
resources (like a prod DB VM or k8s cluster) but not others (like a test k8s
cluster), to reduce the friction for users.

A new field `require_session_mfa` in role `options` specifies whether MFA is
required. For example, the below privileged role enforces MFA per session:

```yaml
kind: role
version: v3
metadata:
  name: prod-admin
spec:
  options:
    require_session_mfa: true

  allow:
    logins: [root]
    node_labels:
      'environment': 'prod'
```

Assuming there exists node `A` with label `environment: prod` in the cluster.
User with role `prod-admin` is required to pass the MFA check before logging
into node `A`.

Now, if a user also has the role:

```yaml
kind: role
version: v3
metadata:
  name: dev
spec:
  allow:
    logins: [root]
    node_labels:
      'environment': 'dev'
```

And there exists node `B` with label `environment: dev` in the cluster.
Then they _don't_ need the MFA check before logging into `B`, because role
`dev` doesn't require it.

Generally, if at least one role that grants access to a resource (SSH node, k8s
cluster, etc.) sets `require_session_mfa: true`, then MFA check is required.
It's required even if another role grants access to the same resource without
MFA.

#### globally

This approach is for operators that want to enforce MFA usage org-wide, for all
sessions.

A new field `require_session_mfa` is available under `auth_service`:

```yaml
# teleport.yaml
auth_service:
  require_session_mfa: true
```

If this field is set to true, it overrides any values set in roles and always
requires MFA checks for all sessions.

### certificate changes

x509 and SSH certificates need 2 new pieces of information encoded:

- is this a short-lived cert issued after MFA?
- [constraints](#constraints) for the cert usage

When validating a certificate, the Teleport service will check RBAC to see if
MFA is required per session. If required, the MFA flag field must be set in the
certificate.

#### SSH

SSH certs will encode new data in extensions. New extensions are:

- `issued-with-mfa` - UUID of the MFA token used to issue the cert
- `client-ip` - IP of the client
- `session-deadline` - RFC3339 timestamp, hard deadline for the session, even
  when there's some activity
- `target-node` - UUID of the target node for the SSH session

#### x509

x509 certs will encode new data in the Subject extensions, similar to [the
other custom fields we
encode](https://github.com/gravitational/teleport/blob/103465ed5a8e20249275b48ac081ef9517ae5aa7/lib/tlsca/ca.go#L180-L260).

New extensions are:

- `IssuedWithMFA` (OID `1.3.9999.1.8`) - UUID of the MFA token used to issue the
  cert
- `ClientIP` (OID `1.3.9999.1.9`) - IP of the client
- `SessionTTL` (OID `1.3.9999.1.10`) - RFC3339 timestamp, hard deadline for the
  session, even when there's some activity
- `TargetName` (OID `1.3.9999.1.11`) - name of the target app, k8s cluster or
  database; the type of target is defined by the `identity.Usage` field (see
  below)
  - existing `KubernetesCluster`, `TeleportCluster`, `RouteToApp` extensions
    are kept for compatibility; enforcement happens based on `TargetName` if
    it's set, and the legacy fields otherwise

The `identity.Usage` field (encoded as `OrganizationalUnit` in the certificate
subject) will be enforced for MFA certs by `auth.Middleware` (even if
`identity.Usage` is empty, which is currently not blocked). The possible values
are:

- `usage:kube` (existing) - only k8s API
- `usage:apps` (existing) - only web apps
- `usage:db` (new) - only database connections

### audit log

All audit events related to session secured with MFA will include a `WithMFA`
field (under `SessionMetadata`) containing the UUID of the MFA token used to
start the session.

If this field is not set on a session event, the session was started without
MFA.

## Alternatives considered

### Private keys stored on hardware tokens

There's a range of hardware products that can store a private key and expose
low-level crypto operations (sign/verify/encrypt/decrypt). They are generally
accessible via a PKCS#11 module in userspace.

PKCS#11 is not well integrated in browsers (clunky UX at best) and not an
option at all for other client software (kubectl, psql, etc).

Apart from that, each kind has their own downsides:

#### HSMs

Hardware security modules (HSMs) are targeted at server use (e.g. storing a CA
private key) and way too expensive for an average user ($650 for YubiHSM, which
is _very_ cheap).

#### Smartcards

Smartcards are an obsolete technology, requiring a separate USB-connected
reader for the card, and targeted at multi-user cases (e.g. office access).

#### PIV

Personal Identity Verification (PIV) is a NIST standard and the closest thing
to generally-available PKCS#11 USB device. Unfortunately, it's only supported
in YubiKeys
(https://developers.yubico.com/yubico-piv-tool/YubiKey_PIV_introduction.html)
and future Solokeys
(https://solokeys.com/blogs/news/update-on-our-new-and-upcoming-security-keys).

All the non-Yubikey security keys out there don't support it and we still have
the UX problems in browsers.

#### CPU Enclaves

Enclaves are CPU-specific (bad compatibility) and have a bad track record with
vulnerabilities.

#### TPMs

Trusted Platform Modules (TPMs) are available on all Windows-compatible
motherboards, almost universal. They are used without human interaction
and only protect from key exfiltration (but not usage).

### Forward proxy on the client machine

Another option is running a forward proxy on the client machine. This means
running `tsh` as a daemon, with a local listening socket. All Teleport-bound
traffic goes to the local socket, through `tsh` and then out to the network.

This lets `tsh` perform any MFA exchanges before proxying the application
traffic:

```
# using TLS as an example
client                  local proxy                      teleport proxy
 |------- mTLS dial ------->|                                   |
 |                          |----------- mTLS dial ------------>|
 |                          |<-------- mTLS dial OK ------------|
 |                          |<-------- U2F challenge -----------|
 |                          |--------- U2F response ----------->|
 |                          |<-------- authenticated -----------|
 |<---- mTLS dial OK -------|                                   |
 |<--------------------- app traffic -------------------------->|
```

The local proxy can handle any authn customizations that we add. Local client
only needs to support a regular mTLS. This allows the U2F check to be
connection-bound (instead of time-bound), and can improve performance by
reusing a TLS connection (with periodic expiry to force U2F re-checks).

The downside is operational complexity - customers really don't want to manage
yet another system daemon. And we'll need to invent a custom U2F handshake
protocol on top of TLS.

Note: a daemon can be added later, working on top of short-lived certs
described in this doc, if there's a solid UX motivation.
