---
authors: Tim Buckley (tim@goteleport.com)
state: implemented (v10.3.0)
---

# RFD 83 - Host Certificate support for Machine ID

## What

Allow Machine ID bots to issue host certificates for OpenSSH servers, per our
documented [manual instructions].

This has two components:
1. Allow bots to securely issue host certificates for particular hosts via RBAC
   rules
2. Provide some bot UX for issuing and making use of issued host certificates 

[manual instructions]: https://goteleport.com/docs/server-access/guides/openssh/#step-24-configure-host-authentication

## Why

Issuing host certs today requires someone with admin-level permissions to
manually issue certificates via `tctl auth sign ...`. If these certificates
expire or if the cluster CAs are rotated, these certificates need to be manually
refreshed. This is inconvenient and encourages users to distribute long lived
credentials when onboarding OpenSSH nodes.

## Details

### Issuing host certificates securely

In Teleport today, host certificates are not modelled as a real resource.
However, we do expose a `host_cert` RBAC resource. Any user granted the `create`
verb on this (virtual) resource may call `GenerateHostCert()` with any
parameters they wish and receive a set of host certificates.

This is reasonable for human cluster administrators with short-lived credentials
protected by MFA, but problematic for long-lived machine credentials. Stolen
credentials, even if short lived, could be used to issue additional long-lived
certificates. While these host credentials don't provide a large level of access
to the Teleport cluster, they could be used to impersonate real nodes.

To mitigate this, we'd like to implement support for `where` clauses for
`host_cert` RBAC rules as we do for rules backed by real Teleport resources. For
example, this role would allow a user to issue host certs, but only for
principals ending in `foo.example.com`:

```yaml
kind: role
version: v5
metadata:
  name: example
spec:
  allow:
    rules:
      - resources: [host_cert]
        verbs: [create]
        where: all_end_with(host_cert.principals, ".foo.example.com")
```

This involves:
1. Adding new comparison functions to better match against DNS names. Minimally,
   we'll add `all_end_with(inputs, suffix)` and `all_equal(inputs, value)`, but
   may consider additional matchers in the future if there's demand. Methods
   like regexes may have problematic quoting and escaping requirements that may
   not be worth solving in the underlying `predicate` library, so we'll avoid
   implementing these until a meaningful use-case exists.

2. Ensuring these functions support lists of strings (or building separate
   functions specifically for matching against `[]string`).

   For example, a hypothetical `all_end_with(host_cert.principals, ".example.com")`
   should AND the result for each input principal. Luckily this does not require
   any changes to the underlying library, and we can confine this behavior to
   only new functions to avoid impact to existing rules.

3. Evaluating `where` clauses in [`GenerateHostCert()`]. As these are not
   proper Teleport resources, the `predicate` context doesn't include any
   useful values to compare against. We'll want to add a new optional
   `host_cert` field much like [`SSHSession`], then provide a custom context in
   [`GenerateHostCert()`].

   Additional context fields will be passed to the `predicate` parser to allow
   further restriction of certificate issuance, including the role, cluster
   name, host ID, and node name. While present, the TTL field may require
   additional custom predicate functions to compare durations. Most of these
   fields (other than principals) are normally unset for SSH nodes.

   We should provide (and document) a sane rule that ensures regular SSH host
   cert rules are followed, alongside users' own requirements (e.g. DNS suffix).

[`GenerateHostCert()`]: https://github.com/gravitational/teleport/blob/82c520c8183553f310459c3b4a96b70065ee268a/lib/auth/auth_with_roles.go#L2139
[`SSHSession`]: https://github.com/gravitational/teleport/blob/ab12ad33d9b3143baa5dc1a0c236cb6ed7645f10/lib/services/parser.go#L183

### Issuing short-lived certificates

SSH host certificates issued today via `tctl auth sign` [do not have an
expiration date][date]. While we could preserve this behavior, any process that
continually produces certificates with an infinite TTL seems problematic.

Our preference is to generate similarly short-lived certificates as we do for
other bot credentials; this has certain (minor) caveats as explored in the UX
section below.

[date]: https://github.com/gravitational/teleport/blob/ab12ad33d9b3143baa5dc1a0c236cb6ed7645f10/tool/tctl/common/auth_command.go#L426

### UX

We have several options for issuing host certificates. Factors to consider
include:
1. Certificate lifespan. Do these certificates need to expire every hour? Our
   documented advice today generates certificates with no expiration date
   specified.
2. Does issuing them every hour create additional UX problems of its own? We
   found this is largely safe with certain caveats (see below).
3. Does `sshd` gracefully reload these certificates? We found that sshd re-reads
   the certificate file for each incoming connection, so no reload is necessary
   unless the `sshd_config` file itself has changed (e.g. changed the path to
   the host cert / key).

#### Preferred implementation: Config Template

Config Templates in Machine ID are refreshed each iteration of the bots usual
renewal loop (20min by default). This would be the most straightforward
integration approach and we'd approach it like we do other special cert types
for apps / databases / k8s clusters.

Users would request a host cert via the config file:

```yaml
destinations:
  - directory: /opt/machine-id
    configs:
      - host_cert:
          principals: [foo.example.com]
```

While rendering the config template, the bot would call `GenerateHostCert()`,
format the resulting certificate and key appropriately, and write it to disk.
A new certificate would be written at startup, then approximately every 20
minutes while running normally.

Per testing, OpenSSH re-reads certificate files on demand, so no additional work
is needed to make use of these certificates once paths are configured.
Additionally, sshd does play nice with `tbot init`'s ACL implementation, and
does not care about file permissions unless they're owned by `root`.

A downside to this approach is that permissions errors (for example, if the
`where` predicate doesn't match) will be reported as errors but will not crash
the bot.

##### `sshd` caveat with short-lived certificates

When `sshd` host certificates expire (because `tbot` crashed, for instance),
users will see an error like the following:

```
Certificate invalid: expired
The authenticity of host '192.168.122.6.foo.example.com (<no hostip for proxy command>)' can't be established.
RSA key fingerprint is SHA256:CWqUJ7q3uPGX9gMoD7R76Hi8pJsoSL8SA0J1FIMmOc8.
This key is not known by any other names
Are you sure you want to continue connecting (yes/no/[fingerprint])?
```

This is nearly identical to the usual ssh TOFU message, save for the
easy-to-miss "Certificate invalid: expired" message. Users are likely
conditioned to accept this, and if that happens the expired or invalid host key
will be committed to their `known_hosts` permanently, after which the "expired"
message will not be shown again.

We'll need to document this caveat along with a workaround (e.g. a
`ssh-keygen -R` command to remove the old entry) to help users avoid connecting
to potentially untrusted hosts.

#### Alternative 1: No-op

Users can already use bot identities to generate host certs with the `host_cert`
permission granted, by passing the Machine ID `identity` file to `tctl`:

```bash
$ tctl -i path/to/identity auth sign \
    --host=foo.example.com \
    --format=openssh \
    --out=myhost
```

This UX works fine in tandem with secure certificate issuance, but users still
need to reissue certs when CAs are rotated or when compromised. Users could
potentially use cron / systemd timers / etc to automate this.

#### Alternative 2: Out of band

If we deem that these certs should be renewed at a different interval than other
bot resources, we could add a new renewal loop for specifically these
certificates. It would be additionally triggered on CA rotation, as regular
renewals are today.

Open questions: bots still renew all certs at startup. Do we maintain that
behavior for these? If not, how do we know when to renew certs next?

## Future Considerations

* Improved IP address support:
  * Automatically include client IP address as an allowed certificate principal
* Further predicate enhancements:
  * Set operations to better support "any of the following allowed values"
* Audit logging:
  * Consider writing audit log events for failed `GenerateHostCert()` calls
  * Consider an audit event predicate action?
