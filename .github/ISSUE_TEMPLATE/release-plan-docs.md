---
name: Documentation Release Plan
about: Docs-related steps to complete with each major release
title: "Teleport X Docs Release Plan"
labels: testplan
---

Perform the following actions on the Teleport documentation whenever we release a
new major version of Teleport.

## Update the docs site configuration

- [ ] Verify the latest version in `gravitational/docs/config.json`

- [ ] Verify that `gravitational/docs/.gitmodules` contains the latest release

- [ ] Ensure that submodule directories in `gravitational/docs` correspond to
    those in `.gitmodules`.

    Remove the directory of the EOL release and create one for the next release
    using a command similar to the following:
    
    ```bash 
    git submodule add https://github.com/gravitational/teleport content/<VERSION>.x 
    ```

## Ensure new feature docs have shipped

- [ ] Verify that all necessary documentation for the release was backported to
  the release branch:
  - [ ] Diff between master and release branch and make sure there are no missed
    PRs
  - [ ] Ensure that the release branch's documentation content reflects all
    changes introduced by the release. If not, plan to update the docs ASAP and
    notify all relevant teams of the delay (e.g., Developer Relations).

## Update versioned information in the docs site

- [ ] Verify that Teleport version variables are correct and reflect the upcoming
  release. Check `docs/config.json` for this.

- [ ] Remove version warnings in the docs that mention a version we no longer
  support _except_ for the last EOL version. E.g., if we no longer support
  version 10, remove messages saying "You need at least version n to use this
  feature" for all versions before 10, but keep warnings for version 10.

- [ ] Verify that the [changelog](../../CHANGELOG.md) is up to date and complete
  for the default docs version. If one release branch has a more complete
  changelog than others, copy that `CHANGELOG.md` to the latest release branch:

  ```bash
  $ git checkout origin/branch/v<release_version> -- CHANGELOG.md
  ```

- [ ] Verify the supported versions table in the FAQ
  (https://goteleport.com/docs/faq/#supported-versions)

- [ ] Verify that the [Upcoming Releases
  Page](../../docs/pages/upcoming-releases.mdx) is up to date for the major
  version of Teleport we are releasing.

  Run the following command on the latest release branch:

  ```bash
  $ git checkout origin/branch/v<last_version> -- docs/pages/upcoming-releases.mdx
  ```
