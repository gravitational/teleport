---
authors: Roman Tkachenko (roman@goteleport.com),
         Zac Bergquist (zac@goteleport.com),
         Jim Bishopp (jim@goteleport.com),
         RJ (rjones@goteleport.com),
         Sasha Klizhentas (sasha@goteleport.com)
state: draft
---

# RFD 134 - Cloud-first deployment

## Required approvers

- Engineering: @russjones, @jimbishopp, @zmb3
- Product: @klizhentas, @xinding33

## What

This RFD describes the Cloud-first deployment approach, as well as the release
process changes required to support it.

## Why

Our current feature delivery process is not compatible with the company's shift
to "SaaSify" Teleport. The new (or rather, extended) Cloud-focused process aims
to address shortcomings like:

- Long release cycles where marquee new features get delivered to Cloud users
  (roughly) once a quarter and are tied to major self-hosted releases.
- Slow and monolithic release pipeline that always builds all artifacts and
  does not allow for very fast Cloud rollouts.
- Engineers being disconnected from Cloud deployments and unaware of when their
  changes are getting rolled out.

## Self-hosted release process (current)

Before diving into the new process, let's set the stage with a high-level overview
of the current release process.

Current release versioning scheme follows semver, with quarterly major releases
and monthly minor releases.

Major releases are getting rolled onto Cloud quarterly, with a month's delay.
Major features are reserved for major releases, which leads to Cloud customers
getting major features with a month's delay.

The release process is triggered on a GitHub tag creation and builds the full
suite of artifacts that takes 2-3 hours so it's unrealistic to do multiple
deploys a day.

Cloud upgrade happens once a week to a release published the previous week, and
engineers oftentimes don't have a clear idea of when their change is being
deployed.

## Cloud release process

In the Cloud-first world, every release that's deployed on Cloud does not necessarily
have a corresponding self-hosted release tag. For this reason, we can't reuse
the current versioning scheme and apply it to both Cloud and self-hosted.

Instead, we want to get to a place where Cloud and self-hosted releases are
decoupled, with Cloud releases being regularly cut from the main development
branch and self-hosted releases cut from their respective release branches.

Although the concept of semantic versioning loses most of its meaning with the
continuous release train, we do need to keep it for Cloud releases to be able
to use automatic upgrades system we built.

With this in mind, the Cloud version will consist of:

- Major: "development" version number on master branch (latest stable self-hosted release + 1).
- Minor/patch: always 0.
- Prerelease: `-cloud.X.Y` where `X` will represent a timestamp of when the tag
  was cut and `Y` the corresponding commit hash: `v14.0.0-cloud.20230712T123633.aabbcc`.

Once it is time for the next self-hosted major release (every 3-4 months), the
new release branch will cut and the Cloud release version major component will
increment.

```
                                                   14.0.0-alpha.1     14.0.0     14.0.1
                                        branch/v14 |------------------|----------|---------....---->
                                                   |
                                                   |
            14.0.0-cloud.20230712T123633.aabbcc    |     15.0.0-cloud...  15.0.0-cloud...    16.0.0-cloud...    16.0.0-cloud...
master -----|-------....---------------------------|-----|----------------|-------....--|----|------------------|--------------....---->
                                                                                        |
                                                                                        |
                                                                             branch/v15 |-----------------|---------|---------...---->
                                                                                        15.0.0-alpha.1    15.0.0    15.0.1
```

Once a self-hosted release branch is cut, subsequent self-hosted minor/patch
releases are published without any changes to the current process, but on a
less frequent cadence than Cloud releases off of master. Changes to the release
branches will follow the same "develop on master, backport to release branch"
workflow we're using now.

### Cloud release artifacts

The self-hosted release and promotion process is slow and takes 2-3 hours
because it builds a full suite of artifacts. Cloud users need most of these
artifacts as well for the agents they run on their own infrastructure, however
Cloud control plane needs a very limited subset of all artifacts. For this
reason, the Cloud release and promotion process will be split in 2 parts:

- The workflow that builds artifacts required for the control plane: amd64
  binaries and Docker images.
- The workflow that builds remaining artifacts such as deb/rpm packages and
  Helm charts.

This will allow us to produce rapid releases that affect only control plane
services (auth, proxy) while still having the ability to push remaining artifacts
when needed.

Both workflows will run in Github Actions and will reuse existing bits of the
full self-hosted release pipeline.

### Cloud promotion process

The Cloud release promotion process will be staged, with a different class of
tenants being upgraded at each stage:

1. Canary
2. Internal
3. Trials
4. Team & Enterprise

The canary tenants will be a set of precreated clusters with some data in them
which engineers will use for connecting their agents and testing their features.
Canary clusters will be updated automatically once a Cloud release is cut.

Before promoting the release further, engineers will test any significant change
in the cloud staging environment and then explicitly sign off that the change is
ready for deployment by applying a label (e.g. `cloud/verified`) on the master
PR corresponding to the change. The release manager then will approve the
further rollout.

Once the change is rolled out to production tenants, engineers will be expected
to monitor error rates and Cloud metrics relevant to their changes.

Let's now look at some of the edge case scenarios in the release process and
how we'll be handling them.

### Behavior-incompatible changes

Teleport's component compatibility guarantees become somewhat obsolete in a
Cloud-first world. Historically, we've been reserving behavior-incompatible
changes for major releases. The concept of a major release, however, becomes
non-existent in the Cloud-first world where any change, backwards compatible
or not, gets deployed to all users within days.

When applied to Cloud release model, and accounted for potential delays in
agent automatic upgrades, the compatibility requirement becomes:

| Any Cloud release must be always compatible with the oldest version of an
| agent that's connected to Cloud and enrolled in automatic upgrades.

Compatibility requirements affect control plane (proxy, auth and other services
we host for our users), agents and clients:

- Control plane. We have full control over it, deploy in tandem and make sure
  these are always compatible with each other.
- Agents deployed by users. We always keep backwards compatibilithy between
  adjacent releases and rely on automatic upgrades to make sure they don't drift.
- Clients. Clients require automatic upgrades to avoid breakage on behavior
  incompatible changes. This is [in progress](https://github.com/gravitational/cloud/issues/4880).

For self-hosted releases, the compatibility guarantees will stay the same:
https://goteleport.com/docs/faq/#version-compatibility.

### Rollbacks

Bad releases can happen even with the multi-staged rollout process. In that
situation the process will be:

- For the control plane, we will prefer rolling forward in most cases to a new
  release with the addressed issue or a rolled back change. The control plane
  release process described above will be fast enough to be able to push the
  new release quickly.
    - In a scenario where a major critical path is affected and a full "roll
      forward" process is not fast enough, the deployment team will manually
      perform release rollback using the same process that is used today.
- For the agents, the rollback mechanism is supported via automatic upgrade
  system's critical/version endpoints described in the [RFD109](https://github.com/gravitational/teleport/blob/master/rfd/0109-cloud-agent-upgrades.md).
- For the clients, which don't support automatic upgrades yet, the details are
  TBD as part of the [automatic upgrades](https://github.com/gravitational/cloud/issues/4880)
  work but we will likely need to build some automatic downgrade mechanism in
  case a client was auto-upgraded but a corresponding Github release was pulled.

### Security releases

Existing security release process is compatible with Cloud-first deployment.

Both `-cloud.X` and regular self-hosted tags will be published from teleport-private
repository and the process will be as follows:

- Cloud control plane is upgraded using promotion mechanism described above.
- Security release is promoted to Cloud repositories prompting agent auto-upgrades.
- Self-hosted private releases are published using existing process.

While the security embargo is in effect, if another Cloud release needs to be
published, it will be published from `teleport-private` repository to include
the security fix like in the current process.

No changes are expected in the `teleport-private` and `teleport` relationship
with regards to security release publishing in the short term.

### Prerequisites

Prerequisites for implementing the Cloud-first deployment process:

- Cloud-specific promotion workflows (control plane and full).
- Automation for release changeset sign-off via Github PR labels.
- Canary cluster(-s).
- Automatic upgrades for `tsh` and other clients.
- Metric and error rate monitoring dashboards.
