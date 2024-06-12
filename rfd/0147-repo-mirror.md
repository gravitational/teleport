---
authors: Mike Jensen (mike.jensen@goteleport.com)
state: draft
---

# RFD 144 - Automatic Repo Mirroring

## Required approvers

* Engineering: @zmb3 || @codingllama || @rosstimothy
* Security: @reedloden || @wadells

## What

This RFD outlines a method for continuously and automatically syncing a GitHub repository. This can be used to create private copies of a repository.

## Why

We currently face a few frictions that could be addressed through a more robust repository sync mechanism:
1. Our internal `teleport-private` mirror of `teleport`, used for security development, currently requires manual updates to sync.
2. Certain tools, such as Dependabot, don't support scanning non-default branches, necessitating manual efforts to manage our release branches.
3. Due to the committing of CI and scanning configurations in the repository, we receive duplicate alerts when using a repository for testing or as an internal mirror/fork.

While addressing these immediate needs, the aim of this RFD is to establish a generic pattern for repository mirroring. Although this is a generic pattern, this is not flexible to the point of being suitable for syncing external forks. External forks face a couple issues with this strategy:
1. This process of rebasing rewrites the history on the default branch, that is not suitable for anything we depend on. Instead a process with controlled merge commits would be better.
2. Using an automatic merge commit would produce a very messy history to review. We likely only want to bring in upstream changes on an as-needed basis which can be controlled and reviewed.

## Details

The core requirements of this solution are:
* Automatic and regular updates to a repository to ensure its state matches the upstream.
* The ability to make additional changes to the mirror repository, separate from the upstream repository (e.g., disabling actions we don't want to run on the mirror, or enabling additional scanning/actions).

### Branch Structure

* `master` (configurable) - This will serve as the repository's default branch. It will contain both the upstream history and any commits necessary for the synchronization process or other custom changes made to the mirror repository.
* `sync/upstream-[UPSTREAM_BRANCH_NAME]` - This branch will be an exact copy of the upstream branch. Should there be a need to create a PR in this repository, the history should be based on this branch, not on `master`.
* `sync/rebase` - This is the branch where custom changes will be committed. It will be rebased onto the upstream changes before being merged into master.

#### Branch Protections

The only protected branch will be `sync/rebase`. Other branches will allow force pushes to enable forceful synchronization based on the upstream state.

### High Level Implementation

The implementation has two primary components:
1. A GitHub Action, scheduled and also triggered by changes to the `sync/rebase` branch. This workflow will handle the rebase operation and commit the results to the appropriate branches (described above).
2. A script invoked when the rebase cannot be applied cleanly. This will enable custom rebase logic, reducing the frequency with which the `sync/rebase` branch needs manual updates to avoid rebase conflicts. A simple example is that workflow automations not desired in the mirror repository might be removed in the `sync/rebase` branch. Any modifications to these workflows would then cause a conflict that can be resolved by simply removing the files. This logic can be changed to meet the specific needs of the repository.

At a high level the action will execute the following steps:
1. Set up authentication and any repository-specific configurations.
2. Check out the latest upstream contents.
3. Fetch our custom changes.
4. Push the exact copy branches `sync/upstream-[UPSTREAM_BRANCH_NAME]` to our mirror.
5. Rebase our `sync/rebase` branch onto the latest upstream changes, using the script for conflict resolutions when necessary and possible.
6. Force-push the rebased changes to the default branch of our repository mirror.

### Specific Implementation Details

#### teleport-private

The only custom changes required for `teleport-private` is the need to maintain multiple `sync/upstream` branches. Specifically, we will sync all release branches. This will be useful when producing backport PRs. Despite syncing multiple upstream branches, only the `master` branch will undergo rebasing (as generally documented above). The rebase is necessary only to incorporate changes related to GitHub Actions and Workflow automations required for syncing.

#### Teleport Security Scanning

This tooling will leveraged to address gaps in our release branch scanning in the following manner:
1. Create four repositories, one for each supported version (and one extra described below): `teleport-sec_scan-1`, `teleport-sec_scan-2`, `teleport-sec_scan-3`, and `teleport-sec_scan-4`. The 1 will refer to our latest release branch, determined by automation, with each subsequent repository covering a consecutively older version.
2. Set up sync automation as documented above.
3. Modify the Dependabot configuration to provide notifications only for security updates on these branches.
4. Disable any unecessary workflows.
5. Adjust the CodeQL configuration to focus scanning on these branches. This would centralize security reporting and minimize inconsistencies between master and the release branches, thereby reducing the frequency of `Fixed` and `Reopened` status changes.

It is important to minimize manual intervention in maintaining this process. Therefore, automating the detection of the current release branch is essential. One challenge is that all repositories will be updated as soon as the next version is created. To ensure constant coverage of supported versions, we maintain a fourth repository.

### `teleport-private` Developer Experience

The development experience within `teleport-private` will largely remain the same, with a few key highlights:
* The most significant change is the requirement to base your branch on one of the `sync/upstream-[UPSTREAM_BRANCH_NAME]` branches. Typically, you will use `sync/upstream-master`, but release branches will also be available for creating backport PRs.
* Manual updates of the repository will be unnecessary. You can start developing whenever you're ready, confident that you're working from an up-to-date branch.

