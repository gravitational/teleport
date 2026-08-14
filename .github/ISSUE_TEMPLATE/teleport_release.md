---
name: Teleport Release 
about: Checklist for a Teleport release 
labels: []
title: "Teleport Release: <version>"
---

1. **Prepare:**
    - [ ]  Create branch `release/X.Y.Z` off of appropriate release branch.
    - [ ]  Make sure you have correct version of `e` checked out.
    - [ ]  Bump the `e` reference. This should be included in the release PR.
2. **Update changelog:**
    - [ ]  `make changelog`
    - [ ]  Insert generated PR list to [CHANGELOG.md](https://github.com/gravitational/teleport/blob/v13.2.1/CHANGELOG.md).
    - [ ]  If this is the first full release build after a [cloud-only release](https://gravitational.slab.com/posts/cloud-only-releases-81v2wjfn), remove the “(unreleased)” label from the version section heading and add changelog entries to the existing section.
    - [ ]  Clean it up: remove irrelevant PRs (like docs-only), update descriptions to be human-readable, format similar to previous release notes.
    - [ ]  Decide if any Enterprise changelogs are relevant and should be inserted. Remove the PR number and links for entry into CHANGELOG.md.
    - [ ]  If this is a follow up to a private release, you should include the disclosure. You can find an example [here](https://github.com/gravitational/teleport/blob/fc5120487872545b1e676646c267b6fc7feb7445/CHANGELOG.md?plain=1#L508-L532)
```
### Security fixes

This release includes various security-related improvements and bug fixes.
We recommend that users on versions prior to vX.Y.Z upgrade to the latest release.
For Teleport Cloud customers, your control plane has already been upgraded to a patched release.

<Insert disclosures here>

### Other fixes and improvements

<Insert normal changelog entry here>
```
3. **Update version:**
    - [ ]  Update [VERSION](https://github.com/gravitational/teleport/blob/v13.2.1/Makefile#L14) in the Makefile.
    - [ ]  Run `make update-version`.
4. **Open PR:**
    - [ ]  Commit changes, e.g: `git commit -a -m "Release X.Y.Z"`
    - [ ]  `gh pr create --title "Release X.Y.Z" --label no-changelog --base branch/vX`
    - [ ]  Add the changelog for the release version to the PR body.
    - [ ]  Wait for the PR to be approved and merged.
5. **Update tag and Export**
    - [ ]  Once PR is merged, checkout/pull the release branch and make sure `e` is up-to-date.
    - [ ]  `make update-tag`
    - [ ]  `./e/scripts/export-pipeline/export-sanitize.sh --branch <branch> --create-pr`

> [!NOTE]
> While the PR is waiting to get merged for export, you can start the next section and build the tag

    - [ ] Get PR merged
    - [ ] Run export sync `./e/scripts/export-pipeline/export-sync.sh --branch <branch>`
6. **Build tag:**
    - [ ]  `make tag-build`
    - [ ]  Wait for the corresponding [GitHub Actions build](https://github.com/gravitational/teleport.e/actions/workflows/tag-build.yaml) to complete.
7. **Publish tag:**
    - [ ]  `./e/scripts/export-pipeline/export-tag.sh --branch <branch> --tag <tag>`
    - [ ]  `make tag-publish`
    - [ ]  Wait for the corresponding [GitHub Actions publish](https://github.com/gravitational/teleport.e/actions/workflows/tag-publish.yaml) to complete.

> [!NOTE]
> If this is a follow up to a private release you must add some security labels when creating the GitHub release below. Ensure that the latest cloud release is included in the `security-patch-alts` as tenants will already be on the patched release.

7. **Create release:**
    - [ ]  `make create-github-release`  or `make create-github-release LATEST=true` if releasing the latest release branch. 

    - [ ] If this is not a follow up to a private release and includes security fixes add `GITHUB_RELEASE_LABELS=security-patch=yes` to the `make` command
    - [ ] If this is a follow up to a private release adde `GITHUB_RELEASE_LABELS=security-patch=yes,security-patch-alts=v<last-private-version>` to the `make` command. More than one version can be defined in `security-patch-alts` by using `|` separator between them. Make sure to escape it so that the shell doesn't treat it as a pipe operator (ie `security-patch-alts=v18.7.4\|v18.7.5`)
    
    The `make` command should show the URL of the newly created release that you can verify and make any changes to.
8. **Verify Post-release workflow:**
    - [ ]  Verify [Post-release](https://github.com/gravitational/teleport/actions/workflows/post-release.yaml) GHA workflow executed successfully.
    - [ ]  Get the PRs created by the workflow merged.
