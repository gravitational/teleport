---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: draft
---

# RFD 12 - Teleport versioning

## What

Versioning scheme for Teleport releases (post-5.0).

### Terminology

Quick note on terminology used below. I'll use naming from
[semver](https://semver.org):

```
vX.Y.Z
 ^ ^ ^
 | | *- "patch version Z"
 | *- "minor version Y"
 *- "major version X"
```

## Why

Teleport has ~4 big releases per year.

Up to v5.0.0, Teleport used a versioning scheme that looks like semver (but
actually isn't):
- a minor version is bumped for all regular releases (e.g. 4.3 -> 4.4)
- a major version is bumped when a release _feels_ particularly significant
  (e.g. 4.4 -> 5.0 with introduction of application access)
- a patch version is bumped for bug patches

Our compatibility promise is:

> When running multiple binaries of Teleport within a cluster (nodes, proxies, clients, etc), the following rules apply:
>
> * Patch versions are always compatible, for example any 4.0.1 component will work with any 4.0.3 component.
> * Other versions are always compatible with their previous release. This means you must not attempt to upgrade from 4.1 straight to 4.3. You must upgrade to 4.2 first.
> * Teleport clients tsh for users and tctl for admins may not be compatible with different versions of the teleport service.

The 2nd point is crucial: we **never** break compatibility with a previous
release (be it a major or a minor version bump).

The downsides of this versioning scheme are:
- the distinction between major and minor version bumps is largely driven by
  business/product and has no technical difference
- a user that doesn't read our upgrade docs carefully can assume semver
  semantics:
  - upgrading from `vX.A.B` to `vX.C.D` is safe - it's not if `C - A > 1`
  - upgrading from `vX.A.B` to `vY.C.D` is going to break things - it won't, as
    long as these are sequential releases

Therefore, I propose to switch to a more semver-like scheme, starting with 6.0.

## Details

- Major versions are for teleport releases.
- Minor versions are for regular, non-critical bugfix batches and important
  backported fixes for users.
- Patch versions are for quick followup regression and critical bug fixes.

As before, `vN.*.*` is compatible with `vN+1.*.*`.

The benefits are:
- Major version bumps clearly communicate to users to exercise caution when
  upgrading, and read release notes
- Minor/patch version bumps are a no-brainer and can be automated
- Users can intuitively understand versioning semantics, due to popularity of
  semver

### Example

If this RFD is approved, the next release of Teleport (e.g. the one containing
database access) will be `v6.0.0`.

If we find serious regressions or bugs that make the product unusable same week
as the release, `v6.0.1` will be cut with the fixes.

As we accumulate a batch of non-critical backported changes (e.g. usability and
performance fixes, backport requests by users), `v6.1.0` will be released.

Users are free to use any `v6.*.*` versions throughout their deployment, after
upgrading everything from `v5.*.*`.

### Why not use minor versions, like we do now?

An experienced reader might notice that this is not exactly semver. In semver,
major versions are for strictly breaking changes with no compatibility.

So why not keep using minor version bumps like we do now? And reserve major
version for when we really need to make a breaking change?

Because we want to keep our compatibility guarantee and avoid the Python 2/3
story. We never want a `vN+1` that has no migration path other than "rebuild
the cluster from scratch".

So we don't want to freeze the major version forever this way.
