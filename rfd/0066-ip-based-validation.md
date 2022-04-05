---
authors: Przemko Robakowski(przemko.robakowski@goteleport.com)
state: draft
---

# RFD 66 - IP-based validation

## What

Additional validation based on client IP for creating and using certificates. User can define which IP addresses can
create certificates and from where they can be used.

## Why

It provides additional security against leaked credentials, if adversary gets hold of certificate he won't be able to
use them outside defined set of machines. It also forms part of user identity.

Relevant issue: [#7081](https://github.com/gravitational/teleport/issues/7081)

## Details

### Configuration

New fields will be added to role allow/deny sections:

* `generator_ips` - list of IP addresses or subnets in CIDR notation that can or can't be used to create certificates
  using this role, defaults to `0.0.0.0/0` (all addresses) in `allow`, `0.0.0.0/32` (no addresses) in `deny`
* `user_ips` - list of IP addresses or subnets in CIDR notation that can or can't use certificates generated earlier
  defaults to `0.0.0.0/0` (all addresses) in `allow`, `0.0.0.0/32` (no addresses) in `deny`

Example configuration:

```yaml
kind: role
metadata:
  name: dev
spec:
  allow:
    # anyone from subnet 192.168.10.0/24 and 192.168.12.5 can generate certificates (i.e. using tsh login)
    generator_ips: [ 192.168.10.0/24, 192.168.12.5 ]
    # anyone from subnet 192.168.0.0/16 can use certificate created earlier by addresses above
    user_ips: [ 192.168.0.0/16 ]
  deny:
    # 192.168.10.5 can't be used to generate certificates
    generator_ip: [ 192.168.10.5 ]
```

### Rules for combining addresses from multiple roles

When user has multiple roles defined following apply:

* allowed IPs will be determined by intersection of ranges from all roles
* denied IPs will be determined by sum of ranges from all roles

This will guarantee that role can never be used outside set of addresses defined by the user.

### Implementation

Following definition will be added to `types.proto`:

```protobuf
message RoleConditions {

  // ...

  // GeneratorIPs specifies policies for IP addresses allowed to generate certificates
  repeated string GeneratorIPs = 21 [(gogoproto.jsontag) = "generator_ips,omitempty"];

  // UserIP specifies policies for IP addresses allowed to use certificates
  repeated string UserIPs = 22 [(gogoproto.jsontag) = "user_ips,omitempty"];
}
```

Ranges from `GeneratorIPs` will be validated in `lib/auth/auth.go#generateUserCert`.

Ranges from `UserIPs` will be encoded depending on certificate type:

* SSH certificate will encode IPs using `source-address` critical option as defined
  by [OpenSSH](https://cvsweb.openbsd.org/src/usr.bin/ssh/PROTOCOL.certkeys?annotate=HEAD). This option is also
  recognized by Go's [ssh package](https://pkg.go.dev/golang.org/x/crypto/ssh), so it will be enforced automatically in
  Teleport. Current `ClientIP` will be added as additional allowed address as `tsh login` uses the certificate it
  creates and will fail if current address is not allowed.
* TLS certificates (used by DB, Kubernetes, Application and Desktop access) will encode IPs in custom extension with OID
  from range `1.3.9999`, similar to `KubeUsers` and others in [tls/ca.go](tls/ca.go). They will be then decoded as part
  of `tlsca.Identity` and validated in authorization routines in respective services. They will be also stored in JWT
  tokens in Application access.

### UX

This change should be mostly transparent for the user. Administrator will add relevant sections to role definitions and
should work for all users.

New flag `--user-ips` will be added to `tsh login` that will let user to further narrow down allowed addresses.
Addresses defined in CLI will be treated as additional role with `allow` section, so rules about combining roles above
apply.

### Security

This proposal does not protect against IP spoofing, but it should provide at least the same level of security as we have
today (as this additional protection, not replacement for user authentication).

Encoding IPs in certificate can provide attacker insight into network topology used (and make it easier to target
high-profile hosts), but it's consistent with encoding used in the rest of the Teleport.

Care must be taken wherever we support Proxy protocol - both source address and proxy address (i.e. real address
connecting to Teleport cluster) should be validated, otherwise it'd be trivial to spoof address by simply prepending
request with `PROXY TCP4 <spoofed source> <target> <spoofed port> <target port>\r\n`
