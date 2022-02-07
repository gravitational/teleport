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
   the every image created (including at build time for releases) is vulnerable
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

* Independently versioned buildboxes, so we can deal with things like Go and 
  Rust being updated independently
* Buildbox source extracted from Teleport repo into it's own space
* Images get build on a tag as per releases, after code review & CI as per
  general software.
* Images being pushed to a repository with immutable tags, so we have a guarantee
  that the image is what we think it is
* Everything (including releases) uses the same set of images
* Enforce strong validation of components on the buildbox as far as possible