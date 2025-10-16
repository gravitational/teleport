---
authors: Tim Ross(tim.ross@goteleport.com)
state: draft
---

# RFD 225 - Teleport Extended Release Support

#### Required Approvers

# Required Approvers
* Engineering: @zmb3 && @russjones && @r0mant
* Product: @klizhentas

## What

Prolong the support window of major releases to reduce the upgrade burden faced by customers.

## Why

Teleport releases a new major version every ~4 months, and provides support for the
current and two previous major versions. This means each major release is supported for
~12 months. Upgrading major releases has to be performed in order - skipping releases
to get to the latest major is not supported.

These two factors can make it difficult for customers that have fallen behind from catching
up to the latest release. Other customers have security and compliance reasons that restrict
them from upgrading major versions outside of a few windows per year. This forces many
customers to run old, unpatched versions of Teleport that are vulnerable to security issues.

## Details

### Long Term Support Releases

A Long Term Support (LTS) release of Teleport has been a popular and long standing request.
This would be no different than any other major release, except it would be tagged LTS and
would be supported and patched beyond that of a typical release.

On the surface this sounds easy, add three extra letters to a release and ensure
that critical patches are backported to this release for its lifetime. However, in
reality this adds a lot of hidden complexity.

Teleport does not support skipping major versions when upgrading today. If an LTS release
would overlap with multiple major versions, then our upgrade process needs to be reevaluated
to support the possibility that a cluster may be upgrading from an LTS release to a non-LTS
release or vice versa. 

Bouncing between LTS and non-LTS or even upgrading from an LTS to a subsequent LTS version
brings about several compatibility concerns. Any backend changes, or changes to Teleport
resources will need special care to ensure resources aren't left in an unusable state.

Currently, the control plane only supports connections from agents and client tools (tsh, tctl, tbot, event-handler, etc.)
that are on the current or previous major version. Adding an LTS release to this mix would
require revisiting this policy to answer the following questions.

Are agents and client tools from a non-LTS release allowed to connect to an LTS control plane?
Are agents and client tools from any non-LTS release that overlaps with the LTS release of the control plane allowed to connect?

Customers already find our current support matrix confusing, adding in an LTS release is only going to
make the situation worse. Confusion aside, the implications of the answers above would also
require a significant change in our deprecation and compatibility philosophies.

When upgrading to LTS N from LTS N-1, the control plane will first be upgraded to LTS N,
and all agents and clients will still be on LTS N-1. Which means we need to ensure compatibility
for whatever the time span is between two subsequent releases.

In addition to the technical and philosophical challenges that an LTS release would introduce, it would
also have an impact on the tools team and developer experience. We would need to support an additional
number of release branches and build pipelines. 

### Extended Support

While customers might be asking for an LTS release explicitly, what they really want are two
things: stability and extended support. We can achieve these without introducing a Teleport LTS
release by changing our process slightly.

We can reduce the frequency of Teleport major releases from every ~4 months to once a year and cut
down the number of supported releases from N-2 to N-1. That would extend the support window for major
releases from ~12 months to ~24 months at the expense of supporting fewer major versions. Reducing
the number of versions also means it takes fewer upgrade cycles to get to the most recent version of Teleport.
No changes will be made to the upgrade guide. Users may upgrade their cluster from any N-1 version to any
N version.

This satisfies customer desire for major releases to be supported for a longer period of time
without impacting our current upgrade process or component compatibility guarantees. However, it
comes at the expense of reducing the frequency at which we can make breaking changes.

As dictated by semantic versioning, a major release should be reserved for making any backward-incompatible
changes to the public API. We've often relied on this four month release cycle to quickly iterate
on APIs that don't work or cause issues for one reason or another. With yearly major releases this
becomes a less viable solution and a stronger emphasis must be put both on backward and forward compatibility. 

Examples of forward compatibility issues that we've seen in the past:

- Using a boolean instead of a string or an enum - https://protobuf.dev/best-practices/dos-donts/#bad-bools
- Strict validation over a closed set of values prevents adding new ones
- Making new fields mandatory

Examples of backward compatibility issues that we've seen in the past:
- Removing a deprecated API too soon
- Changing the behavior of an existing API
- Altering validation of existing fields of a resource

Going forward, solutions to problems should attempt to preserve compatibility at all costs and
rely less on all customers adhering to our upgrade procedure. Instead of relying on versions of
clients, and that the Auth server will reject outdated clients we should put a stronger emphasis
on negotiating compatibility. If clients and servers in our control can inform each other about
what features they support in some manner then both can mutually agree on which features and APIs
can be safely used. An example of this can be seen with the capabilities exchanged during instance
heartbeat hello messages: https://github.com/gravitational/teleport/blob/6d4961aed1eef6b360e05facdbcd997fb95593d4/api/proto/teleport/legacy/client/proto/inventory.proto#L136-L181.

To address the stability concerns, all new features *MUST* only be added to the most recent major version.
The previous major version goes into maintenance mode the moment a new major version is released.
In other words, even though we will now support two major releases, N and N-1, new features *MUST* only
be included in version N. N-1 will continue to receive bug fixes and security patches, but not new features.

The Test Plan should also be performed more frequently to help ensure stability of releases. Additionally,
when the Cloud framework permits, release branches compatibility, performance, and stability should be
automatically tested on a periodic cadence (i.e. nightly, weekly).

### Toolchain Implications

Extending support of a Teleport release means that it may outlive releases of toolchains used to build Teleport.
Go releases are supported for ~12 months. They release a new major version every ~6 months, and are EOL
when two newer major releases exist.

While Go is typically good about preserving compatibility, new releases do occasionally alter behavior
that require changes to Teleport. We could align our release cycle to the Go release cadence to reduce
the number of times the toolchain would need to be upgraded but that doesn't buy us much.

This isn't an entirely new problem as our current N-2 and N-1 release outlive Go toolchains today.
We should continue to only bump the Go toolchain on branches _after_ it has gone through a test plan.

Rust releases a new stable version every ~6 weeks, and security updates are only issued for the current
release. We have historically been very behind on updating Rust toolchains due to build issues with
wasm-pack and GLIBC compatibility issues. Now that wasm-pack is removed, updating the Rust toolchain
more frequently should be achievable. The Desktop Access and tools teams will coordinate with
bumping Rust versions regularly to ensure builds don't break.


## Proposed Rollout

The currently documented supported releases are the following.

| Release | Release Date      | EOL            | Minimum `tsh` version |
|---------|-------------------|----------------|-----------------------|
| v18.x   | July 3, 2025      | August 2026    | v17.0.0               |
| v17.x   | November 16, 2024 | February 2026  | v16.0.0               |
| v16.x   | June 14, 2024     | October 2025   | v15.0.0               |

To migrate to the extended support described above, v16, the current
N-2 release, will become EOL at the end of October 2025 without the
introduction of a new major release. The lifetimes of v17 and v18 will
be extended to be the first Teleport releases supported according to the
process detailed above.

| Release | Release Date      | EOL               | Minimum `tsh` version |
|---------|-------------------|-------------------|-----------------------|
| v18.x   | July 3, 2025      | August 2027       | v17.0.0               |
| v17.x   | November 16, 2024 | August 2026       | v16.0.0               |
| v16.x   | June 14, 2024     | October 31 2025   | v15.0.0               |

This means that 19.0.0 would be released in August of 2026, at which time
v17 becomes EOL and v18 transitions to a maintenance only release.
