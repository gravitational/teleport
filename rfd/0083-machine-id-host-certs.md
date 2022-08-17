---
authors: Tim Buckley (tim@goteleport.com)
state: draft
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
        where: ends_with(host_cert.principals, ".foo.example.com")
```

This involves:
1. Adding new comparison functions (like `matches()` for regexes or a simple 
   `ends_with()`) to better match against DNS names.

2. Ensuring these functions support lists of strings (or building separate
   functions specifically for matching against `[]string`).

   For example, as implemented today, `equals(host_cert.principals, "foo")`
   already iterates over all LHS arguments and ANDs the result. A hypothetical
   `ends_with()` should behave the same way.

3. Evaluating `where` clauses in [`GenerateHostCert()`]. These follow an
   entirely different codepath than regular Teleport resource access
   ([`CheckAccessToRule()`]), so we'll likely need to implement special handling
   for this special non-resource, since we don't have a single name / namespace /
   etc.

   [Session Access] has a custom predicate implementation that would mostly suit
   our needs, though we'll need to follow standard RBAC rule behavior.

[`GenerateHostCert()`]: https://github.com/gravitational/teleport/blob/82c520c8183553f310459c3b4a96b70065ee268a/lib/auth/auth_with_roles.go#L2139
[`CheckAccessToRule()`]: https://github.com/gravitational/teleport/blob/82c520c8183553f310459c3b4a96b70065ee268a/lib/services/role.go#L2309
[Session Access]: https://github.com/gravitational/teleport/blob/82c520c8183553f310459c3b4a96b70065ee268a/lib/auth/session_access.go#L126

### UX

We have several options for issuing host certificates. Factors to consider
include:
1. Certificate lifespan. Do these certificates need to expire every hour?
2. Does issuing them every hour create additional UX problems of its own?
3. Does `sshd` gracefully reload these certificates, and if so, how can users be
   expected to gracefully reload `sshd` when `tbot` is running as a user
   process?

Per (3), we have an open issue to help notify applications of changed
certificates (https://github.com/gravitational/teleport/issues/11264). It's
unclear if OpenSSH gracefully handles certificate rotations, though, and
disconnecting sessions every 20 minutes would not be ideal.

#### Option 0: No-op

Users can already use bot identities to generate host certs with the `host_cert`
permission granted, by passing the Machine ID `identity` file to `tctl`:

```bash
$ tctl -i path/to/identity auth sign \
    --host=foo.example.com \
    --format=openssh \
    --out=myhost
```

This UX works fine in tandem with secure certificate issuance, but users are
left to issue certs manually and to inform `sshd` of the change. If managed by
cron / systemd timers / Ansible / etc, this could arguably be more 
straightforward than `tbot` file change notifications (but complicates handling
of CA rotation events).

#### Option 1: Config template

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
format it appropriately, and write it to disk. A new certificate would be 
written every 20 minutes. 

It would be up to the end user to inform sshd of the new certificate.

#### Option 2: Out of band

If we deem that these certs should be renewed at a different interval than other
bot resources, we could add a new renewal loop for specifically these
certificates. It would be additionally triggered on CA rotation, as regular
renewals are today.

Open questions: bots still renew all certs at startup. Do we maintain that
behavior for these? If not, how do we know when to renew certs next?

