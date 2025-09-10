---
authors: Tim Ross(tim.ross@goteleport.com)
state: draft
---

# RFD 225 - Teleport Release Support

#### Required Approvers

# Required Approvers
* Engineering: @zmb3 && @russjones && r0mant
* Product: (@klizhentas)

## What

Prolong the support for major releases to reduce the upgrade burden of customers.

## Why

Teleport releases a new major version every ~4 months, and provides support for the
current and two previous major versions. This means each major release is supported for
~12 months. Upgrading major releases has to be performed in order - skipping releases
to get the latest major is not supported. These two factors can make it difficult for
customers that have fallen behind on upgrades from catching up to the latest release.

This leaves customers running old and unsupported releases with unpatched security
vulnerabilities because they do not wish to keep pace with our upgrade cadence. 

## Details

A Long Term Support (LTS) release of Teleport has been a popular and long standing request.
This would be no different than any other major release, except it would be tagged LTS and
its support window would be extended beyond that of a typical release.

On the surface this sounds easy, add three extra letters to a release and ensure
that critical patches are backported to this release for its lifetime. However, in
reality this adds a lot of hidden complexity.

Teleport does not support skipping major versions when upgrading today. If an LTS release
is introduced that overlaps with multiple major versions, then our upgrade process needs
to be reevaluated to support the possibility that a cluster may be upgraded from an LTS
release to a non-LTS release or vice versa. 

Bouncing between LTS and non-LTS or even upgrading from an LTS to a subsequent LTS version
brings about several compatibility concerns. Any backend changes, or changes to Teleport
resources will need special care to ensure resources aren't left in an unusable state.

The control plane today only supports connections from agents and client tools (tsh, tctl, tbot, event-handler, etc.)
that are on the current or previous major version. Adding an LTS release to this mix would
require revisiting this policy.

Are agents and client tools from a non-LTS release allowed to connect to an LTS control plane?
Are agents and client tools from any non-LTS release that overlaps with the LTS release of the control plane allowed to connect?

Customers already find our current support matrix confusing, adding in an LTS release is only going to
make the situation worse.

This also prolongs the amount of time we need to maintain compatibility to ensure zero downtime
upgrades. When upgrading to LTS N from LTS N-1, the control plane will first be upgraded to LTS N,
and all agents and clients will still be on LTS N-1. This means our current process of marking
a feature or API as ok to remove in two major release after making a breaking change need to be
altered to marking for removal in two LTS releses.

While customers might be asking for an LTS release explicity, what they really want are two things:
stability and extended support. We can achieve these without introducing a Teleport LTS release by
changing our process slightly.

To achieve extended support we can reduce the frequency of Teleport major releases from every ~4 months
to once a year and cut down the number of supported releases from N-2 to N-1. That would extend the
support window for major releases from ~12 months to ~24 months at the expense of supporting fewer
major versions. However, reducing the number of versions also means it takes fewer upgrade cycles to
get to the most recent version of Teleport.

This satisfies customer desire for major releases to be supported for a longer period of time
without impacting our current upgrade process or component compatibilty guarantees. However, this
does come at the expense of reducing the frequency at which we can make breaking changes. Fewer
major releases mean fewer chances to introduce changes that affect compatibility - though that
might be an opportunity in disguise.

To address stability concerns, all new features *MUST* only be added to the most recent major version.
The previous major version goes into maintainence mode the moment a new major version is released.
In other words, even though we will now support two major releases, N and N-1, new features will only
be included in version N. N-1 will continue to receive bug fixes and security patches, but backports
for features will be limited to version N.

While this somewhat resembles how things operate today, there is no formal policy in place and backports
are often at the discretion of a PR author. For the most part this means that today major version N-2
receives updates only for bugs, however, if a customer requesting a new feature is still running N-2  
we will comply with their request to backport said feature to N-2 if possible.

This practice can no longer continue if we are to promote stability of releases. All _new_ features
are to be limited to the most recent major version only.



- "Calendar versioning"
--- Reduce frequency of majors to once per year
--- Support N and N-1, thus extending support of a release to 24 months instead of 12 months
--- New features only land in the latest major, the previous major is frozen in maintainence mode
--- Everything else just works as it did today
--- Fewer release gives users a chance to catch up to the latest major
--- Need to be rigorous in auditing backports
--- No confusion for customers, there is no LTS, all releases become LTS
--- Toolchain versioning
----- Go releases are EOL ~9 months, which means we will need to update the toolchain that we build with
----- Can we can stop tying the version in go.mod to the Go version used to build to eliminate issues?
--- More minor releases per year
--- More emphasis on compatibility
----- Major releases are primarily an excuse to break backwards compatibility
----- We should put a stronger emphasis on forward compat from the begining
----- We should stop relying on version checking and instead exchange features that are supported
----- We should have a single version of update API that never changes allowing you to always update

