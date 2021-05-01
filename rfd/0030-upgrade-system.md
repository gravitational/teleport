---
authors: Forrest Marshall (forrest@goteleport.com)
state: draft
---

# RFD 30 - Upgrade System

## What

System for automated/assisted upgrading of teleport installations.

## Why

Teleport, like virtually all software, must be periodically updated in order to
maintain security.  Outdated teleport isntallations also impose additional burdens
both on us, and our users.  Currently, teleport does not assist with its
own upgrade process, and does not inform users when the current installation is
in need of upgrading due to being too old, or due to the existence of a relevant
security patch.

By making teleport upgrades easier (or automatic), we can improve the experience
of using teleport, reduce workload for us and our users, and improve the security
of the teleport ecosystem at large.

## Intro

### High Level Goals

- The upgrade system must be *secure*.  In the worst-case, a compromised upgrade
system could allow an attacker to install arbitrary malicious software across the
teleport ecosystem. With this in mind, the upgrade system will be designed with
the aspirational goal of being resilient to compromise of any single machine,
secret, account, etc (in this context, "resilient" just means "won't result in
successful execution of a malicious installation").  In practice, this level of
fault-tolerance is essentially impossible to achieve, but compartmentalization
and redundancy will be primary guiding principals.

- The upgrade system must be *reliable*.  Specifically, the upgrade system must
be resilient to intermittent failures and self-healing to the greatest extent possible.
With this in mind, the upgrade system will embrace a kubernetes-esque model of enacting
change; components will continually attempt to reconcile the current state of the system
with some desired final state.  Care will also be taken to maximize cross-version and
cross-implementation compatibility.

- The upgrade system should be *modular* and *extensible*.  It should be easy to extend
the behavior of the upgrade system both as a user and as a maintainer.  The individual
components of the upgrade system must have simple and well-defined responsibilities.
It should be easy to write new components, and to hook into external systems.  Components
should also be highly opinionated about their domain of responsibility, while maintaining
as little opinion as possible about other domains (this extends to mundane things, e.g. we
will prefer purpose-specific validation and selection of defaults for resources, rather than
a monolithic `CheckAndSetDefaults` operation as is common in much of teleport's internals).

### Attack Scenarios

#### Malicious Distribution

**Scenario**: An attacker successfully leverages the upgrade system to directly distribute
malicious non-teleport software (or a malicious fork of teleport), causing existing teleport
clusters to install said malicious software on one or more servers (aka: the doomsday scenario).

We're going to roughly divide the possible avenues of attack into two families, direct and
indirect compromise.  In the case of direct compromise, the attacker successfully causes
"legitimate" distribution infrastructure to perform functions that result in the installation
of malicious software.  This may be via compromise of supply chain, software/libraries,
accounts, or even individuals. In the case of indirect compromise, the attacker successfully
tricks clients into believing that they are interacting with, or received a package from,
some trusted source, when that is not the case.  This may occur due to the compromise of a
cryptographic key or CA, manipulation of a client into using insecure protocols, redirection
of a client to an illigitimate download point, injection of malicious public key into a root
of trust, etc.

Direct compromise will be mitigated via compartmentalization.  In particular, we will divide
the sources of truth that clusters interact with into two abstract subsystems that can serve as
checks agianst one another.  The index/manifest subsystem will be the "discovery" point for new
teleport releases and their checksums, as well as metadata about the nature of the releases
(e.g. whether or not the release contains a critical security patch).  The
repository/distribution subsystem will be isolated from the index/manifest subsystem, and serve
as the origin from which releases can be downloaded and installed (there are already a number of
teleport distribution systems, but some tweaks may be needed to meet all our requirements).
By keeping these two halves of the upgrade system isolated (separate code bases, secrets,
environments, etc) we can help compartmentalize security breaches.  Compromise of the index
cannot result in installation of unauthorized software if the repositories don't serve it.
Compromise of one or more repositories cannot result in the installation of unauthorized software
if the index does not publish matching checksums.

Indirect compromise will be partially mitigated by the split system described above, since the two
subsystems will rely on separate cryptographic identities.  For additional mitigation, we will also
require that compromise of any single store of trust (e.g. injection of a malicious TLS root cert)
or skipping of any single verification step (e.g. skipping package signature verification)
will not be sufficient to perform a compromised installation.  This is fairly straightforward, so
long as we rely on separate public keys/keystores for verifying package signatures and server
identities (most modern package managers already do this).


#### Malicious Downgrades

**Scenario**: Attacker successfully leverages the upgrade system to cause an outdated version of
teleport (presumably one with some vulnerability that the attacker would like to exploit) to be
installed.  This differs from the previous scenario in that the package has a valid signature and
was, at some point, considered a valid installation target.

Once again, we're going to subdivide this scenario into two families of attack: visible and masked
downgrades.  A visible downgrade is a downgrade where teleport "knows" that it is installing an older
version, but does so anyway.  This would most likely be caused by a compromised index/manifest system.
A masked downgrade is a downgrage that looks like an upgrade (i.e. teleport thinks it is installing
`v1.2.3` when it is actually installing `v0.1.2`).  This would most likely be caused by a compromised
distribution system.

The best way to prevent visible downgrades is to not allow downgrades! Unfortunately, this
may not be practical.  If the latest version turns out to have an unexpected issue, a downgrade may be
necessary.  The simplest solution (and the one we'll likely start with) is to require that downgrades
be triggered manually, but this isn't ideal.  A better solution would be to have custom policies for
downgrades, with a reasonable default being something like "previous patch release iff current release
was not a security patch".  Complementary to this would be including an ability to yank outdated/insecure
versions from distribution (or at least mark them as such).

Masked downgrades aren't a significant threat if only the distribution system is compromised, as the
index/manifest system will continue to provide checksums with correct versioning information.  Some
repository systems also resist tampering with version numbers, though not all do so.  That leaves us
with one final scenario: the index *and* at least one repository are compromised simultaneously (but
the compromise is not so severe as to put us in the previously discussed 'doomsday scenario').  This is
a bit niche, but lets exercise a little professional paranoia.  We should have a mechanism of validating
package version (prior to unpacking/installation) as part of package signature verification (i.e. on the
"distribution" side).  One appealing option is [minisign](https://jedisct1.github.io/minisign/) which
is a simple and modern signing utility that supports "trusted comments" (i.e. signed metadata) as part
of the signature file.


#### Compromised Build System

Out of scope for now, but I've heard all the cool kids are doing deterministic builds these days.


## Proposal

### Overview

High-level overview of system components, including discussion of MVP vs planned features.

#### Index Server

The index server, as discussed above, is responsible for serving metadata and checksums for the
current list of valid installation targets.  Part of the security model of the upgrade system is
that the index server remains isolated from the rest of the ugrade system stack, and provides only
abstract information about available releases (i.e. it will not provide things like download urls
or install scripts).

Key design considerations related to the index server are as follows:
- Index server should be trivially horizontally scalable.
- Index servers should always require TLS 1.3 unless started in development mode.
- Rollbacks of index server state or API version must not be detremental to its clients
(i.e. clients must be resilient to inconsistent values being returned).
- Index server must remain compatible with many past teleport versions (exact number TBD),
but we don't want to orphan older versions, so lets err on the side of caution here.

Outstanding design questions:
- Grpc or basic https API?  My preference leans toward grpc, but its hard to discount the possible
benefits of a basic https API (e.g. pulling down release info with `curl`).
- How 'smart' should the index server be exactly?  On one hand, if the index server is able to intelligently
select what releases/targets are relevant to a given client, it may be able to reduce the impact of bugs
in the upgrade system (easier to update the index server than to update all clients).  On the other hand,
the extensibility of the upgrade system (not to mention the simplicity of the index server) would likely
be benifited by keeping as much logic as possible in the client clusters.

Hypothetical teleport release descriptors returned by index server:

```yaml
targets:
  - tags: [oss]
    version: '100.1.0-alpha.2'
    arch: amd64
    os: linux
    flavor: oss
    stable: false
    security_patch: false
    deprecated: false
    sums:
      rpm-blake2-256: '...'
      tgz-blake2-256: '...'
      deb-blake2-256: '...'
  - tags: [ent]
    version: '100.0.1'
    arch: amd64
    flavor: ent
    os: linux
    stable: true
    security_patch: true
    deprecated: false
    sums:
      rpm-blake2-256: '...'
      tgz-blake2-256: '...'
      deb-blake2-256: '...'
  - tags: [ent,fips]
    version: '100.0.1'
    arch: amd64
    os: linux
    flavor: ent
    stable: true
    security_patch: true
    deprecated: false
    sums:
      rpm-blake2-256: '...'
      tgz-blake2-256: '...'
      deb-blake2-256: '...'
```

*note*: Since writing the above I'm leaning toward grouping targets by version number with a single set of user-assignable
tags/labels stored at the top-level of the version entry.  Will circle back and update.

#### Upgrade Policies

An upgrade policy will be responsible for matching releases to servers.  In the interest of
reliability and fault-taulerance, many upgrade policies may apply to a given server.  The
upgrade policy which maps to the most recent teleport version will be given priority for
a given server.

Policy evaluation (i.e. determining how releases map to servers) will be the responsibility
of the auth server.  The goal of this centralization is to ensure that if a logic bug in the
mapping results in requiring a manual upgrade, only the auth server(s) will need to be upgraded.
Some amount of hedging will be added to the API to make sure its easy to add custom (external)
policy evaluators later if the need exists, but I imagine that custom upgrade controllers and
manually managed release manifests will cover most usecases.

I have yet to work out the exact details of how policy syntax and evaluation should work, but
here's an example of one of the syntaxes I've been toying with in my head:

```yaml
target_selectors:
  - name: Latest oss
    flavors: [oss]
    permit_unstable: true
server_selectors:
  - name: Preview servers
    server_roles: ['proxy','node']
    filter: 'contains(server.labels['unstable-preview'],'yes')'
schedule:
  not_before: '2021-04-29T00:00:00Z'
  time_range: '01:00:00-06:00:00'
  day_range: Mon-Fri
```

Note that my current thinking on target/server matching is that policies should not actually
be able to cause a node to be upgraded to a target with different fundamental attributes (e.g. os,
flavor, arch, etc) due to the risk of catastrophic user error being too great. Instead, the
purpose of target selectors it to allow different requirements to be put in place *based* on
those fundamental attributes (e.g. only installing unstable versions on oss nodes).

Not all policies need be fancy.  In fact, something like this aught to be a valid policy:

```
target_selectors:
- version: '99.*'
```

read: "Keep all servers on the latest stable `99.X` release"


#### Upgrade Controllers

The "upgrade controller" will be the primary hook that is used to customize upgrade
behavior.  An upgrade controller takes in config objects, and `server => target`
mappings produced by upgrade policies, and attempts to reconcile the current state
of the various servers with their target states. 

In a perfect world, we'd launch the upgrade system with an in-house installer *so good*
that everyone would immediately drop their existing package management solutions and
wonder how they ever got by before.  In reality, this is a pretty tough sell, and
a huge amount of work if you want to build something meaningfully better than can
be achieved with a few lines of bash.  With this in mind, I'm of the opinion that
flexibility of upgrade mechanism matters more than having a single extremely robust
install mechanism, at least where MVP functionality is concerned.

Lets take a look at a hypothetical example of an input resource for a "local script"
upgrade controller:

```yaml
env:
  TELEPORT_VER: '${target.version}'
  TELEPORT_SUM: '${target.sums["rpm-blake2-256"]}'
install.sh: |
  #!/bin/bash
  set -euo pipefail

  tmp_dir=$(mktemp -d -t teleport-XXXXXX)

  yum install -y --downloadonly --destdir $tmp_dir teleport-ent-${TELEPORT_VER}

  package_file="$(ls $tmp_dir)"

  echo "$TELEPORT_SUM $tmp_dir/$package_file" | b2sum --check

  yum localinstall -y $tmp_dir/$package_file
```

Note that we don't just run a normal `yum install` in the above example.  This is
because we still need to maintain the security requirements that we came up with in
the previous attack scenario discussion. The checksum (distributed by the index) must
be used to verify the validity of the package pulled in by `yum`.  We don't care how
secure the yum repo is.  If we want to compartmentalize failure, then we must assume
that it is untrusted and verify the checksums of downloaded package against the expected
values from the index.

While the above example was intended to be run on the host being upgraded, there is no
reason we can't extend that model to, for example, running on a didicated node that manages
ansible or terraform scripts, or a kubernetes pod with the ability to modify image tags.

In addition to a simple builtin script-based controller (and various future builtins),
it should be possible to write external controllers and hook them in, in a similar manner
to the current access plugins.  Much like the current access plugins, such controllers
will likely require access to a `PluginData`-esque API for managing some per-server state
(in order to enforce things like locking and backoff).  I don't want to assume that the
`PluginData` API as it exists will be a perfect match, so my thinking here is that we should
write an example controller and let the challenges that arise during that process inform
the API (though, if we can re-use the existing `PluginData` implementation, we should).


#### Server & Upgrade State Management

Server heartbeats will need to be updated to include build information so that teleport
can reason about what servers are in need of an upgrade, and select the appropriate
installation target for said upgrade.  Ex:

```yaml
version: '98.7.6'
arch: amd64
os: linux
flavor: ent
stable: true
fips: false
```

We will also need a mechanism for synchronization between upgrade controllers (either
controllers of different kinds, or different instances of the same controller).  Multiple
controllers concurrently attempting to upgrade a node could be potentially disasterous (and
even if our builtin controllers are resilient to it, we don't want to force user-implemented
controllers to have to indipendently reason about this kind of thing).

My initial thinking on controller sync was that we would allow controllers to take out some
kind of short lived "lease" which grants them temporary exclusive operating rights for a
given server.  This, however, may not be as extensible as we would like, since it
wouldn't map efficiently to controllers which indirectly upgrade many nodes (e.g.
by changing a kubernetes deployment, or triggering an ansible playbook).

Another option for controller sync would be to have controllers publish a short-lived lease which
included a compiled list of the relevant `server_selectors`.  So long as the published leases
could be given a strong odering (i.e. they would all need to be published to the same resource),
then we could filter out targets that match earlier leases (i.e. if the first lease selects all
servers with label `foo=bar`, and the second lease selects all servers where `foo=bar` *or* `foo=bin`,
then the second controller would end up processing only servers with label `foo=bin`).  This would
be powerful, but possibly is more complex than necessary.

After a lot of back-and forth on this, I'm leaning towards starting as simple as possible.  Its
more important that upgrade systems be reliable than fast, and the simplest way to synchronize
controllers is to have a single short-lived *global* controller lease.  A controller publishes
what its working on, and how long it will be working.  Other controllers wait until after that
lease either expires, or is explicitly released (preferable).  After release, only nodes who
have heartbeated since the release time should be considered valid upgrade targets (so that the
version information in the backend matches the post-upgrade state of the world).

Regardless of the lease strategy, leases should include some basic information, including:

- Name of the controller that created the lease.
- Est. number of servers targeted for upgrade (this might be the number of servers which
match the `server_selector` and not necessarily reflective of the number of servers that
will actually get upgraded).
- Version being targeted.


#### Upgrade UX

At the cluster-level, upgrade flows should roughly fall into two camps.  Manual and automatic,
where manual upgrades require some kind of triggering from a user.  Other than needing a simple
manual trigger, the UX around these two modes of operation should be identical.  I think the
best way to do this is to have the process of applying policies to available releases produce
a compiled "target state" that is independent of the resources used to calculate it (i.e. the
upgrade policies and release manifests).

By keeping the target state indipendent of the resources used to calculate it, we are able to
store it as an indipendent resource.  Upgrade controllers will attempt to converge toward the stored
target state, meaning that they will not be directly affected by new releases becoming available
or upgrade policies being changed.  If the cluster is configured for automatic upgrades, then
periodic operations in the auth server can recompile the target state.  If the cluster is configured
for manual upgrades, the user can preview and apply new compilations of the target state via `tctl`.

This should allow us to implement a manual upgrade flow that looks something like this:

```
$ tctl upgrade
<diff of target state>
Proceed: y/n
```

Or, if we want to allow for manual modification of target state, something like:

```
$ tctl upgrade plan > target-state.yaml
# review the plan & make changes

$ tctl upgrade apply -f target-state.yaml
<diff>
Proceed: y/n
```

### Technical Notes


#### Crypto

We aren't in full control of the cryptography at play since we're integrating user scripts and
third-party installation mechanisms, but where possible we will be opinionated.  Where applicable
we will select algorithms that are relatively modern and targeted to meet or exceede a 128-bit
security level.

##### Package Signing

This is generally covered by existing package distribution solutions.  I'm leaning toward `minisign`
being the distributor-agnostic signing mechanism of choice.  Its dead simple, produces signatures
compatible with OpenBSD's `signify`, and the one piece of "bloat" it does have, trusted comments,
is actually useful to us.

##### Package Checksums

256-bit `BLAKE2` will be used for checksums (compatible with the `b2sum` utility from GNU coreutils).
Other checksums may be added as needed, but only if they are of similar quality (e.g. we're not adding `md5`
or `sha2). `BLAKE2` has similar properties to the `sha3`/`keccak` hashing algorithm (e.g. no length-extension),
but is more performant for software implementations. To my knowledge, the choice between `BLAKE2` and `sha3`/`keccak`
for performing infrequent file checksums is a tossup.  I chose `BLAKE2` due to its inclusion in coreutils, which
guarantees fairly broad accessibility.

##### TLS

New components will require TLS v1.3.  Not much to say here.  Its the latest version and its widely supported.


### Nice to Have

#### Pending Upgrade Plans

One simple way to open the door for all kinds of interesting workflows around manual upgrades
would be to allow teleport to store one or more "pending" upgrade states that would be kept
immutable and could be applied via a separate API call.  This would open the door for approval
workflows to develop around upgrades, much in the way that they currently exist for roles.

#### Local Auto-Rollback

Some upgrade controllers will rely on the correct functioning of the teleport installation in order
to apply an ugprade (e.g. `ssh`ing into a node in order to upgrade it).  This poses a problem since,
if the new version ends up being non-functional, it would be impossible to perform a rollback/patch
via the same mechanism.

One way to mitigate risk of this kind of failure would be to allow an upgrade controller to leave
behind a script that teleport can use to perform a rollback if teleport detects that it remains unhealthy
for some period of time after upgrade.  We'd need to be strict about file/directory permissions here
in order to prevent the behavior from becoming an avenue for privilege escalation (script would almost
certainly need to be run as root, so it would need to be impossible for a non-root user to create/modify
the script).  Definitely doable though.

#### Pre-Installation Self-Check Command

For the same reasons that local rollback would be nice, it would also be nice to be able to unpack
the new teleport binary prior to installation and have it run a series of self-checks to help catch
problems like this before they occur.

Simple self-check possibilities:
- Ping auth/proxy servers as appropriate.
- Validate config file, certs, cache contents, etc.

Advanced self-check possibilities:
- Check if able to read/write backend (auth only).
- Check if able to register a tunnel and dial into self (node only, would require updating proxy
logic to permit opening a "testing" tunnel that did not interfere with real tunnels for the same
host).
- Check if able to instantiate a healthy in-memory cache.

Note that doing this correctly requires getting access to the exact `teleport start` command being
used by the currently running teleport instance, which is something we can't put into a heartbeat
because it may contain secrets.  It would also require doing a better job of isolating state-changing
logic (e.g. migrations) so that we can confidently spin up non-trivial teleport process components
without worrying about modifying state for the main process.  All in all, this isn't practical in the
short-term, but it is a goal worth keeping in mind.


#### TODO

---

playing with some ideas, please ignore

---

```yaml
kind: version-control-directive
metadata:
  name: default
spec:
    # schedule constrains the *start time* of an upgrade (no guarantees are made
    # about when said upgrade completes, if it completes).
    schedule:
      not_before: '2021-04-29T00:00:00Z'
      time_range: '01:00:00-06:00:00'
      day_range: Mon-Fri

    # targets is the list of available installation targets.  targets are prioritized
    # by version.
    targets:
      - tags: [oss]
        version: '100.1.0-alpha.2'
        arch: amd64
        os: linux
        flavor: oss
        stable: false
        security_patch: false
        sums:
          rpm-blake2-256: '...'
          tgz-blake2-256: '...'
          deb-blake2-256: '...'
      - tags: [ent]
        version: '100.0.1'
        arch: amd64
        flavor: ent
        os: linux
        stable: true
        security_patch: true
        sums:
          rpm-blake2-256: '...'
          tgz-blake2-256: '...'
          deb-blake2-256: '...'
      - tags: [ent,fips]
        version: '100.0.1'
        arch: amd64
        os: linux
        flavor: ent
        stable: true
        security_patch: true
        sums:
          rpm-blake2-256: '...'
          tgz-blake2-256: '...'
          deb-blake2-256: '...'

    # installers describe mechanisms by which an installation target may be
    # applied to a server.  installation methods are prioritized by the target
    # versions that they match.  in the case that multiple installers match
    # a target, they are attempted in order until one succeeds.
    installers:
      - name: yummy-ent-installer
        kind: local-script
        target_selectors:
            - flavors: [ent]
        server_selectors:
            - name: Yummy servers
              server_roles: ['proxy','node']
              filter: 'contains(server.lables['pkg-manager'],'yum')
        env:
          TELEPORT_VER: '${target.version}'
          TELEPORT_SUM: '${target.sums["rpm-blake2-256"]}'
        install.sh: |
          #!/bin/bash
          set -euo pipefail

          tmp_dir=$(mktemp -d -t teleport-XXXXXX)

          yum install -y --downloadonly --destdir $tmp_dir teleport-ent-${TELEPORT_VER}

          package_file="$(ls $tmp_dir)"

          echo "$TELEPORT_SUM $tmp_dir/$package_file" | b2sum --check

          yum localinstall -y $tmp_dir/$package_file

      - name: curl-oss-installer
        kind: local-script
        target_selectors:
            - flavors: [oss]
        server_selectors:
            - name: Curly-whirly servers
              server_roles: ['proxy','node']
              filter: 'contains(server.lables['pkg-manager'],'kinda')'
            - name: Preview servers
              server_roles: ['proxy','node']
              permit_unstable: true
              filter: 'contains(server.lables['pkg-manager'],'kinda') && contains(server.labels['unstable-preview'],'yes')'
        env:
          TELEPORT_VER:  '${target.version}'
          TELEPORT_SUM:  '${target.sums["tgz-blake2-256"]}'
          TELEPORT_OS:   '${target.os}'
          TELEPORT_ARCH: '${target.arch}'
        install.sh: |
          #!/bin/bash
          set -euo pipefail

          tmp_dir=$(mktemp -d -t teleport-XXXXXX)

          pkg_name="teleport-v${TELEPORT_VER}-${TELEPORT_OS}-${TELEPORT_ARCH}-bin.tar.gz"

          cd $tmp_dir

          curl --tlsv1.3 -o $pkg_name "https://get.gravitational.com/${pkg_name}"

          echo "$TELEPORT_SUM $pkg_name" | b2sum --check

          tar -xf $pkg_name

          ./teleport/install

    # we're gonna be a little more agressive than usual where sync is concerned.
    internal: 
        nonce: 1
        written: '2021-04-29T00:00:00Z'
```
