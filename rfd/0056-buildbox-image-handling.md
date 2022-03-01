---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# RFD 45 - Buidlbox image Handling 

## What

A "buildbox" image contains the tools environment for constructing executable 
software from source. 

## Why

## Details

### Goals

Any workable solution must satisfy the following goals 

 1. The build system must be able to create repeatable builds, from source, of
    any revision of the Teleport source at any time. 

 2. The same process (and its resulting images) should be used by all automated 
    builds (e.g. CI, test & Release)

 3. Changes to the buildbox image must be reviewable and auditable.

 4. Changes to the buildbox should be opt-in, in the sense that changes to a 
    buildbox should not be *forced* on a consumer - the consumer should deliberately 
    reference the updated buildbox image.

 5. Using the buildbox images (at least in public-facing CI) must *not* require 
    the use of privileged containers (e.g. require Docker-In-Docker), as this 
    is a security threat.

 6. Creating an updated buildbox image once changes have been approved should 
    be automatic (or at least a single push-button operation)

 7. Where multiple build environments are required, they should all be 
    produced by the same process.

### The current process

Currently the buildbox images are constructed by Drone in response to a commits
being pushed onto `gravitational/teleport:master`. The drone pipeline constructs 
updated buildbox images from the latest source in `build.assets/Dockerfile` (and 
it's siblings), tags them with the version of `go` installed on the image and 
then pushed them to the repository at `quay.io`. These builds are then used for CI.

For _releases_, the release pipelines in Drone construct a buildbox image at 
_build time_ from the same source in `build.assets`, and then execute the build 
inside it via Docker-in-Docker

This process meets several of the goals set above:

 * The buildbox for a release are constructed from source at build time, so the 
   current process (somewhat - I'll explain why below) meets Goal #1.
 * Changes to the buildbox are incorporated by the PR/code review process, so 
   Goal #3 is met
 * Images are automatically produced for both CI and Release, so Goal #4 is met
 * Images for all of the different build targets are created in the same way, so
   Goal #7 is met

On the other hand:

 * We do no validation of the components installed onto the buildbox image, so 
   **_every_** image created (including at build time for releases) is vulnerable
   to downloaded artifacts being subverted.
 * We tag _every_ revision of the buildbox with the current go version, meaning
   that we cannot uniquely identify any revision of the buildbox image.
 * Using the version of Go as the image tag means we have no mechanism to 
   independently upgrade other software in the image, or identify which images 
   contain what versions of, for example, Rust.
 * Updating the buildbox image automatically forces and CI jobs using the 
   buildbox to accept the new changes. This has recently caused issues where 
   updating the buildbox `go1.17.2` image to build Teleport 9 broke CI for 
   Teleport 8, as there is no way to differentiate the images they require.

### Suggested changes

#### 1. Immutable, independently-versioned buildboxes.

That is:
 * each buildbox image should be an immutable, versioned collection of tools required
   to build Teleport. 
 * The version of the overall  _collection_ should not be tied to the version of any
   of the compenents inside it.
 * Modifiying the contents of a buildbox implies creating a new version tag.

#### 2. CI and releases are configured to use an _exact_ version of the buildbox

CI & release proccesses refer to _exact_ versions of the buildbox they use, either 
by tag (if we can guarantee tag immutability) or by hash (if we can't).

 * This allows us to reproduce a build with the _exact same tooling_ that it was 
   built with previously. We _cannot_ make this guarantee with floating tags
 * We _certianly_ can't guarantee it with the build-at-time-of-use scheme currently
   in use.
 * Makes changes to the buildbox _opt-in_ for consuming tasks. For example: say 
   somebody breaks the buildbox? No big deal - CI on your branch just keeps using the
   old one until you're ready to upgrade.

Ideally we should _also_ have some sort of developer-friendly floating tags so that
people can experiment locally without having to sweat multiple release/update cycles.

#### 3. Images being pushed to a repository with immutable tags

This is in order to guarantee that the image is what we think it is. This is a developer
affordance, as all this can be accomplished with hashes, but it's cumbersome and 
unfriendly.

#### 4. Images get built on a tag update and after code review & CI

This imposes the same release mechanism on the tooling as on the main software. The idea 
is to turn the process of spinning a new buildbox version into a rare(-r) event, rather 
than happening on every merge.

#### 5. Everything (including releases) uses the same set of images

A release process should refernce an _exact_ buildbox image it will use and commit that
to `git` prior to building the release. We should be able to see which buildbox was used
for any given release just by examining the Teleport source.

#### 6. Enforce strong validation of components on the buildbox as far as possible

Currently we implicitly trust that _none_ of the following have been compromsed when
constructing our buildbox:
 - The Ubuntu debian package repositories
 - The `go` package download package
 - The `rustup` tool download package
 - The `etcd` download package
 - The Google `addlicense` repository
 - The `libbpf` download site
 - The Google Cloud Platform SDK download site
 - The `golangci-lint` download site
 - and a few others, but you get the idea...

At minimum, we should create SHAs of known-good versions of the downloaded artefacts 
and verify that they have not changed, to ensure that we are not surprosed by any 
changes after the fact.

#### 7. Buildbox source extracted from Teleport repo into it's own repo

(This is controversial)

The whole `build.assets` directory is a murky part of the Teleport repo that I don't
think people understand, so removing it from the teleport repo and having appropriate
READMEs & so forth wouldbring it out into the open

While this would require a 2-step shuffle where any buildbox image is built and released 
prior to use by CI and Releases, this is also true for _any_  scenario that requires the 
use of pre-built images for CI, as you can't build the image internally and then run a 
build on it without requiring DinD or the like.

Author's Note:  This is how I've done it in the past, so it feels natural to me. 
Reasonable people can differ on this, I accept that this rationale may not scale 
to the rest of the team:-)
