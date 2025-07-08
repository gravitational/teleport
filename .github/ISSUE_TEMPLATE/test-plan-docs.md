---
name: Documentation Test Plan
about: Manual test plan for Teleport major releases
title: "Teleport X Docs Test Plan"
labels: testplan
---

Perform the following checks on the Teleport documentation whenever we roll out
a new major version of Teleport on Teleport Cloud. Use `/docs/upcoming-releases`
to determine the rollout date.

## Is the internal documentation coverage record up to date?

- [ ] Identify features within the new release that we want to include as topics
  in our measurement of documentation coverage. Update our internal
  documentation coverage record to include the new topics. See our internal
  knowledge base for the location of the coverage record.

## Is the docs site configuration accurate?

> [!IMPORTANT] 
> **Do not merge the new docs site configuration** before we roll out a new
> major version to Teleport Enterprise (Cloud).

- [ ] Verify the latest version in `gravitational/docs/config.json`

- [ ] Verify that `gravitational/docs/.gitmodules` contains the latest release

- [ ] Ensure that submodule directories in `gravitational/docs` correspond to
    those in `.gitmodules`.

    Remove the directory of the EOL release and create one for the next release
    using a command similar to the following:

    ```bash
    git submodule add https://github.com/gravitational/teleport content/<VERSION>.x
    ```

## Is the docs site content up to date with the new release?

- [ ] Verify that Teleport version variables are correct and reflect the upcoming
  release. Check `docs/config.json` for this.

- [ ] Ensure that redirects (as configured in `docs/config.json`) only exist for
  the default version of the docs site, and have been removed from other
  versions.

- [ ] Remove version warnings in the docs that mention a version we no longer
  support _except_ for the last EOL version. E.g., if we no longer support
  version 10, remove messages saying "You need at least version n to use this
  feature" for all versions before 10, but keep warnings for version 10.

- [ ] Verify that all necessary documentation for the release was backported to
  the release branch:
  - [ ] Diff between master and release branch and make sure there are no missed
    PRs
  - [ ] Ensure that the release branch's documentation content reflects all
    changes introduced by the release. If not, plan to update the docs ASAP and
    notify all relevant teams of the delay (e.g., Developer Relations).

- [ ] Verify that the [changelog](../../CHANGELOG.md) is up to date. Each
  version of the docs (i.e., each `gravitational/teleport` release branch shown
  on the docs website) must include a `CHANGELOG.md` file in which the most
  recent major version is the one that corresponds to its release branch. 

  On `master`, edit `CHANGELOG.md` to include a heading for the next major
  version. We can add notes for features in development under this heading on
  `master`.

  For example, if we cut `branch/v20` from `master`, the `CHANGELOG.md` on
  `branch/v20` must include `v20` release notes at the top. `master` must begin
  with a heading for `v21` development notes, e.g.:

  ```markdown
  ## 21.0.0 (xx/xx/xx)
  ```

- [ ] Verify the accuracy of critical docs pages. Follow the docs guides below
  and verify their accuracy while using the newly released major version of
  Teleport.

  - [ ] General [installation page](../../docs/pages/installation/installation.mdx): ensure
    that installation methods support the new release candidate.
  - [ ] [Teleport Community
    Edition](../../docs/pages/admin-guides/deploy-a-cluster/linux-demo.mdx) demo
    guide.
  - [ ] [Teleport Enterprise (Cloud)](../../docs/pages/get-started.mdx) getting
    started guide.
  - [ ] [Teleport Enterprise (Self-Hosted) with
    Helm](../../docs/pages/admin-guides/deploy-a-cluster/helm-deployments/kubernetes-cluster.mdx)
  - [ ] [Teleport Enterprise (Self-Hosted) with
    Terraform](../../docs/pages/admin-guides/deploy-a-cluster/deployments/aws-ha-autoscale-cluster-terraform.mdx)

## Phase Two changes

Make sure these tasks are complete by the time we have rolled out a new major
version of Teleport to Teleport Enterprise (Cloud) customers.

- [ ] Update the docs site configuration in
  `gravitational/docs-website/config.json`: ensure that the EOL version has
  `"deprecated": true` assigned and the newly rolled out version has
  `"isDefault" true`. Remove the `"isDefault": true` assignment from the
  previous version.

- [ ] Copy the changelog from the previous default branch to the new one:

  ```bash
  $ git checkout origin/branch/v<release_version> -- CHANGELOG.md
  ```

- [ ] Verify the supported versions table in the FAQ
  (https://goteleport.com/docs/faq/#supported-versions)

- [ ] Verify that the [Upcoming Releases
  Page](../../docs/pages/upcoming-releases.mdx) only exists for the major
  version of Teleport we are releasing. Ensure that this page contains the
  latest information:

  ```bash
  $ git checkout origin/branch/v<last_version> -- docs/pages/upcoming-releases.mdx
  ```

## Verify the accuracy of critical docs pages

Follow the docs guides below and verify their accuracy. To do so, open the
version of the docs site that corresponds to the major release we're testing
for. For example, for Teleport 12 release use `branch/v12` branch and make sure
to select "Version 12.0" in the documentation version switcher.

### Installation

- [ ] General [installation page](../../docs/pages/installation.mdx): ensure that
  installation methods support the new release candidate.
- [ ] Enterprise Cloud [downloads
  page](../../docs/pages/choose-an-edition/teleport-cloud/downloads.mdx): ensure that
  the release cnadidate is available at the repositories we link to.

### Getting started

- [ ] [Teleport Community Edition](../../docs/pages/index.mdx)
- [ ] [Teleport Enterprise (Cloud)](../../docs/pages/choose-an-edition/teleport-cloud/get-started.mdx).
- [ ] [Teleport Enterprise (Self-Hosted) with
  Helm](../../docs/pages/deploy-a-cluster/helm-deployments/kubernetes-cluster.mdx)
- [ ] [Teleport Enterprise (Self-Hosted) with
  Terraform](../../docs/pages/deploy-a-cluster/deployments/aws-ha-autoscale-cluster-terraform.mdx)
