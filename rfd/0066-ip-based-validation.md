---
authors: Przemko Robakowski(przemko.robakowski@goteleport.com)
state: implemented (Teleport 10.0.0)
---

# RFD 66 - IP-based validation

Note: this approach was refined in [RFD 110](./0110-ip-pinning-and-propagation.md)

# Required Approvers

* Engineering @zmb3 && (@codingllama || @nklaassen )
* Product: (@xinding33 || @klizhentas )

## What

IP-based validation is an enterprise-only feature that embeds the source IP
in SSH certificates. When enabled, SSH certificates can only be used to connect
to resources from the same source IP that was issued the certificate.

## Why

It provides additional security against leaked credentials, if adversary gets hold of certificate he won't be able to
use them outside machine that created it. It also forms part of user identity.

Relevant issue: [#7081](https://github.com/gravitational/teleport/issues/7081)

## Details

### Configuration

New field will be added to role options definition:

* `pin_source_ip` - defines if certificate should be pinned to the IP of the client requesting it. User won't be able to
  use the certificate from different IP for example if one wants to move certificate to different machine without doing
  login there or if one uses mobile internet that changes IP addresses frequently or network with multiple exit nodes
  with different IPs.

Example configuration:

```yaml
kind: role
metadata:
  name: dev
spec:
  options:
    pin_source_ip: true
```

### Implementation

Following definition will be added to `types.proto`:

```protobuf
message RoleOptions {
  // ...

  // PinSourceIP defines if certificate should be pinned to the IP of the client requesting it.
  bool PinSourceIP = 18 [(gogoproto.jsontag) = "pin_source_ip", (gogoproto.casttype) = "Bool"];
}
```

If any role has `PinSourceIP` set to `true` then IP of the client requesting certificate will be encoded depending on
certificate type:

* SSH certificate will encode IP using `source-address` critical option as defined
  by [OpenSSH](https://cvsweb.openbsd.org/src/usr.bin/ssh/PROTOCOL.certkeys?annotate=HEAD). This option is recognized
  by `sshd` from OpenSSH and also by Go's [ssh package](https://pkg.go.dev/golang.org/x/crypto/ssh), so it will be
  enforced automatically in Teleport.
* TLS certificates (used by DB, Kubernetes, Application and Desktop access) already encode IP in custom extension with
  OID
  `1.3.9999.1.9` in [tls/ca.go](tls/ca.go). It is then decoded as part of `tlsca.Identity` and will be validated
  in `*authorizer.Authorize` method `lib/auth/permissions.go`

Encoding above will happen in all places we generate certificates:

* `lib/auth/auth.go#generateUserCert`
* `lib/auth/join.go#generateCerts` (Machine ID)
* `lib/auth/auth_with_roles.go#generateUserCerts()` (renewals, impersonation etc)

Implementation must ensure that all calls `*authorizer.Authorize` provide valid client IP (there are at least HTTP, gRPC
and databases protocols to handle).

### UX

This change should be mostly transparent for the user. Administrator will add relevant option to role definitions and
should work for all users.

If user tries to use certificate on other machine (different IP) `tsh` will force relogin as it currently does when
certificate expires:

```shell
~$ ./teleport/tsh ssh node1.cluster.local
Enter password for Teleport user admin:
```

### Security

This proposal does not protect against IP spoofing, but it should provide at least the same level of security as we have
today (as this additional protection, not replacement for user authentication). Likewise, it won't prevent attacks that
use certificates from original machine as malware, worms etc.
