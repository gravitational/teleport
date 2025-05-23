---
name: Documentation Release Plan
about: Documentation checks and changes to perform for major Teleport releases
title: "Teleport X Docs Test Plan"
labels: testplan
---

Perform the following tasks whenever we roll out a new major version of
Teleport. 

We need to make sure that the documentation site presents accurate information
to Teleport Enterprise (Cloud) users by default. Since we roll out a new major
Teleport version to Teleport Enterprise (Cloud) users several weeks after we
release the version, documentation release steps take place in two
phases:

- **Phase One:** We have released a new major version of Teleport but have not
  rolled it out to Teleport Enterprise (Cloud) customers.
- **Phase Two:** We have rolled out the new major version of Teleport to
  Teleport Enterprise (Cloud) customers.

Use `/docs/upcoming-releases` to determine the Teleport Enterprise (Cloud)
rollout date.

## Phase One tasks

Make sure these tasks are complete by the time we have released a new major
version of Teleport.

- [ ] Identify features within the new release that we want to include as topics
  in our measurement of documentation coverage. Update our internal
  documentation coverage record to include the new topics. See our internal
  knowledge base for the location of the coverage record.

- [ ] Update the submodule configuration in `gravitational/docs-website`.

  1. Remove the `content` directory for the EOL release. Create a directory for
     the next release using a command similar to the following:
  
     ```bash
     git submodule add https://github.com/gravitational/teleport content/<VERSION>.x
     ```

  1. Verify that `gravitational/docs-website/.gitmodules` contains the latest
     release and not the EOL release.

  1. In `gravitational/docs-website/.gitmodules`, make sure the major version
     we're releasing corresponds to the major version's release branch, not
     `master`.

  1. In `gravitational/docs-website/config.json`, ensure that the EOL version
     has the `deprecated` key set to `true`. Add the next version and update the
     `branch` field as needed. 

     **DO NOT** update the `isDefault` field, since we only change the default
     docs site version when we release the new major version on Teleport Cloud.

  1. Test that you have completed these steps successfully by building and
     running the docs site locally:

     ```bash
     rm -rf docs/* versioned_docs/* versioned_sidebars/*
     yarn build
     yarn serve
     ```

- [ ] Verify that Teleport version variables are correct and reflect the upcoming
  release. Check `docs/config.json` for this in all supported branches of
  `gravitational/teleport`.

- [ ] Remove version warnings in the docs that mention a version we no longer
  support _except_ for the last EOL version. E.g., if we no longer support
  version 10, remove messages saying "You need at least version n to use this
  feature" for all versions before 10, but keep warnings for version 10.

- [ ] Verify that all necessary documentation for the release was backported to
  the release branch:
  - [ ] Diff between `master` and the new release branch and make sure there are
    no missed PRs.
  - [ ] Ensure that the release branch's documentation content reflects all
    changes introduced by the release. If not, plan to update the docs ASAP and
    notify all relevant teams of the delay.

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

  - [ ] General [installation page](../../docs/pages/installation.mdx): ensure
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

- [ ] Verify that the [Upcoming Releases
  Page](../../docs/pages/upcoming-releases.mdx) only exists for the major
  version of Teleport we have rolled out. Ensure that this page contains the
  latest information:

  ```bash
  $ git checkout origin/branch/v<last_version> -- docs/pages/upcoming-releases.mdx
  ```
