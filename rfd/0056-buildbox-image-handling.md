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

 2. The same process (and its resulting images) should be used by all parts 
    of the development cycle (development, test, CI, Release)

 3. Changes to the buildbox image must be reviewable and auditable.

 4. Using the buildbox images (at least in public-facing CI) must not require 
    the use of privileged containers (e.g. require Docker-In-Docker), as this 
    is a security threat.

 5. Creating an updated buildbox image once changes have been approved should 
    be automatic (or at least a single push-button operation)

 6. Where multiple build environments are required, they should all be 
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
   Goal #6 is met

On the other hand:

 * We do no validation of the components installed onto the buildbox image, so 
   the every image created (including at build time for releases) is vulnerable
   to downloaded artifacts being subverted.
 * We tag _every_ revision of the buildbox with the current go version, meaning
   that we cannot uniquely identify any revision of the buildbox image.

Repeatability

While the current process meets 

For release
Immutable tags