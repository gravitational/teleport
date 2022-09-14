---
authors: Jeff Pihach (jeff.pihach@goteleport.com)
state: draft
---

# RFD 89 - Merge Webapps into Teleport

## What

- Merge the [webapps](https://github.com/gravitational/webapps) repo into the
  [teleport](https://github.com/gravitational/teleport) repo.
- Merge the [webapps.e](https://github.com/gravitational/webapps.e) repo into
  the [teleport.e](https://github.com/gravitational/teleport.e) repo.

## Why

The version of the web UI and the Teleport are tightly coupled but the
repositories are separate which means that complexities arrise when integrating
the two projects. We have to separately clone and build the web UI and commit
the webasset changes into the teleport repo whenever we want to represent an
update. We also have to maintain multiple build systems and ops procedures for
the web UI. Moving the web UI repositories into their respective Teleport repo
will save us time and confusion.

## Success Criteria

Both [Teleport](https://github.com/gravitational/teleport) and
[Teleport.e](https://github.com/gravitational/teleport.e) repos contain their
respective UI counterparts and the build systems correctly build all projects.

## Process

Below outlines the process for each repository in the order that the steps need
to be taken. All repositories will be prepped for the final archival and merging
but the final switch over steps will not be taken until v11 has been released.

#### Webapps repository

[Webapps repository](https://github.com/gravitational/webapps)

- [ ] Triage issues.
  - [ ] Close those that are no longer relevant.
  - [ ] [Transfer remaining issues](https://docs.github.com/en/issues/tracking-your-work-with-issues/transferring-an-issue-to-another-repository) to the [Teleport repo](https://github.com/gravitational/teleport)
  - [ ] Add the [ui label](https://github.com/gravitational/teleport/labels/ui) to
        those transferred issues.
- [ ] Triage PRs.
  - [ ] Close those that are no longer relevant.
  - [ ] Merge remaining PR's.
  - [ ] Apply the necessary backports.

#### Webapps.e repository

[Webapps.e](https://github.com/gravitational/webapps.e)

- [ ] Triage issues.
  - [ ] Close those that are no longer relevant.
  - [ ] [Transfer remaining issues](https://docs.github.com/en/issues/tracking-your-work-with-issues/transferring-an-issue-to-another-repository) to the [Teleport.e repo](https://github.com/gravitational/teleport.e)
  - [ ] Add the [ui label](https://github.com/gravitational/teleport.e/labels/ui)
- [ ] Triage PRs.
  - [ ] Close those that are no longer relevant.
  - [ ] Merge remaining PR's.
  - [ ] Apply the necessary backports.

#### Teleport repository

[Teleport repository](https://github.com/gravitational/teleport)

- Remove `webassets` submodule
  - This submodule is no longer required as the web UI will be built on demand.
- Clone the [Webapps repository](https://github.com/gravitational/webapps) into
  the Teleport root.
  - This will need to be done for every respective version branch (v8, v9, v10)
- update targets that use the `packages/webapps.e` submodule to point points to
  the correct version in the `teleport.e/webapps` folder.
- add the `webapps-build` and `webapps-test` build steps.
- only require teleport build processes to run on teleport paths and the webapp
  ones to run on the webapp paths
-
- Archive the [Webapps repository](https://github.com/gravitational/webapps).

#### Teleport.e repository

[Teleport.e repository](https://github.com/gravitational/teleport.e)

- Clone the [Webapps.e repository](https://github.com/gravitational/webapps.e)
  into the Teleport.e root.
  - This will need to be done for every respective version branch (v8, v9, v10)
- only require teleport build processes to run on teleport paths and the webapp
  ones to run on the webapp paths
-
- [ ] Archive webapps.e repository

#### CI job notes
