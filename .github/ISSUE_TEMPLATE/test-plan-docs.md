---
name: Documentation Test Plan
about: Manual test plan for Teleport major releases
title: "Teleport X Docs Test Plan"
labels: testplan
---

Perform the following checks on the Teleport documentation whenever we release a
new major version of Teleport:

## Is the docs site configuration accurate?

- [ ] Verify the latest version in `gravitational/docs/config.json`

- [ ] Verify that `gravitational/docs/.gitmodules` contains the latest release

- [ ] Ensure that submodule directories in `gravitational/docs` correspond to
    those in `.gitmodules`.

    Remove the directory of the EOL release and create one for the next release
    using a command similar to the following:

    ```bash
    git submodule add https://github.com/gravitational/teleport content/<VERSION>.x
    ```

## Is the docs site up to date with the new release?

- [ ] Verify that Teleport version variables are correct and reflect the upcoming
  release. Check `docs/config.json` for this.

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

- [ ] Verify that the [changelog](../../CHANGELOG.md) is up to date and complete
  for the default docs version. If one release branch has a more complete
  changelog than others, copy that `CHANGELOG.md` to our other support release
  branches, e.g.,:

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

- [ ] [Community Edition](../../docs/pages/index.mdx)
- [ ] [Teleport Team](../../docs/pages/choose-an-edition/teleport-team.mdx)
  (this also serves as the getting started guide for Teleport Enterprise Cloud).
- [ ] [Teleport Enterprise with
  Helm](../../docs/pages/deploy-a-cluster/helm-deployments/kubernetes-cluster.mdx)
- [ ] [Teleport Enterprise with
  Terraform](../../docs/pages/deploy-a-cluster/deployments/aws-ha-autoscale-cluster-terraform.mdx)

### New feature docs

- [ ] Review the roadmap for the major version we are releasing and verify that
  you can complete all how-to guides for new features successfully. Consult the
  [Upcoming Releases Page](../../docs/pages/upcoming-releases.mdx) for a list of
  features in the next major release.
