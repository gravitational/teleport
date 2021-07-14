---
authors: Russell Jones (rjones@goteleport.com)
state: draft
---

# RFD 4 - Release Plan

## What/Why

While the Teleport team has grown, the release process has remained ad-hoc with varying release managers following varying steps with varying outcomes.

This RFD proposes a formalized process allows spreading the load of managing a release and de-silo knowledge and lead to a smoother release process.

## Details

### Release Manager Selection

The release manager will be rotated across team leads (or a delegated proxy) for every release.

The current list of rotation release managers is below.

* @awly for Security.
* @russjones for Scale.
* @russjones for SSH and Kubernetes Access.
* @r0mant for Application and Database Access.
* @russjones for Release.

### Release Manager Responsibilities

The release manager is responsible for shepherding the release before, during, and after a release in the areas outlined below.

Release managers will track their process by forking `.github/release.md` for each release and including it in the milestone. A sample `release.md` is included below and will act as a dashboard to view status of the release.

#### Track Approvals

Release managers will synchronize with and wait for approval of the following individuals responsible for the outlined areas before cutting a release.

* @awly for Security.
* @russjones for Scale.
* @russjones for SSH and Kubernetes Acccess.
* @r0mant for Application and Database Access.
* @russjones for Release.
* @klizhentas for Documentation.
* @klizhentas for Product.

#### Track Release Date

Release managers will synchronize with the approvers list above and update the release date accordingly.

To update the release date, release managers need approval from @klizhentas, after which the release manger will:

1. Submit a PR to update the [Upcoming Releases](https://goteleport.com/docs/preview/upcoming-releases/) to notify customers.
2. Send an email to Engineering, Customer Success, and Sales about the updated release date so they can inform customers or prospects waiting for a release appropriately.

#### Test Plan

Release managers will decide which parts of the test plan are appropriate for each release and round-robin assign them to individual team members.

In general, two weeks should be allocated for testing a major release and one week for minor releases.

#### Release Process

Release managers will follow the process outlined below for each release after testing is complete.

* Prepare CHANGELOG/Release Notes. Must be approved be @klizhentas.
* Block merging into release branches (needs investigation).
* Create release branch.
* TBD.
* Cut release.
* Organize release retrospective.

----

# Teleport x.y.z Release Plan

## Release Manager

Russell Jones

## Release Date

- [ ] First Beta: January 1st, 1970.
- [ ] First RC/Testing Start: January 1st, 1970.
- [ ] Release: January 1st, 1970.

## Product Approvals

- [ ] Security @awly
- [ ] Scale @russjones
- [ ] SSH and Kubernetes Access @russjones
- [ ] Application and Database Access @russjones
- [ ] Release @russjones
- [ ] Documentation @klizhentas
- [ ] Product @klizhentas

## Test Plan Approval

- [ ] Link to Teleport x.y.z Test Plan.

# Release Process

- [ ] Prepare CHANGELOG/Release Notes. Must be approved be @klizhentas.
- [ ] Block merging into release branches (needs investigation).
- [ ] Create release branch.
- [ ] TBD.
- [ ] Cut release.
- [ ] Organize release retrospective.

