
---
author: jane quintero (jane@goteleport.com) Russell Jones (rjones@goteleport.com)
state: draft
---

# Continuous Integration (CI)

## What/Why

This RFD proposes changes to Continuous Integration (CI) at Teleport to increase security, reduce complexity, and build a base for future enhancements.

This includes, but is not limited to, the following functionality.

* Automating parts of the development process (approvals, assignments) with bots.
* Running tests (unit, integration) against internal and external PRs.
* Running scheduled workflows (nightly) against `master`.
* Tracking and reporting metrics (test failures, regressions) to external tooling (Slack).

## Details

### Security

To achieve these goals, the below is proposed.

* Continuous Integration (CI) and Continuous Delivery (CD) should be separate systems. Exploiting CI should not lead to a compromise of CD.
* We should not self-host CI, instead we should buy a CI service we trust. GitHub Actions is proposed in the RFD.
* User controlled input must always run in an unprivileged context.
* All business logic should be performed a small Go program called `workflow-bot`. This will let us write test coverage in the majority of the CI workflow.

To maintain the above security property, the following changes will be made.

#### Repository Settings

`CODEOWNERS` will be updated to only protect the [`.github/workflows` directory](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners). Any changes to GitHub Workflows will require repository administrators (like @klizhentas, @russjones, or @r0mant) to review and approve. Approvals for all other directories will be covered in `RFD 00XX: Assignment and Approval Workflows`.

Branch protection rules will be updated to [dismiss approvals](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule#creating-a-branch-protection-rule) to `master` and all release branches if new changes are pushed to the PR after approval. To prevent a slowdown to developer productivity, `Require branches to be up to date before merging` will be disabled. Any breaking changes caused by this will be caught by automated testing that will run against `master` and all release branches upon merge.

#### Workflow Permissions

Workflows will run either in the context of `master` or `HEAD` of the PR and [with access to secrets or without](https://docs.github.com/en/actions/reference/encrypted-secrets#accessing-your-secrets). Use cases are illustrated in the `Use Cases and Permissions` section.

### Use Cases and Permissions

#### PR Approval Bot

Context: `master`, Secrets: `GITHUB_TOKEN: READ`

A bot that approves PRs should not run in the context of the PR, otherwise an attacker could simply change the approval workflow itself, and approve their own PR. A PR approval bot should run in the context of `master`.

In addition, because an approval bot does not need to make changes to the repository, it should not have access to any additional secrets and should only have `GITHUB_TOKEN: READ` permissions.

#### PR Assignment Bot

Context `master`, Secrets: `GITHUB_TOKEN: WRITE`

A bot that approves PRs should not run in the context of the PR, otherwise an attacker could simply change the approval workflow itself, and approve their own PR. A PR approval bot should run in the context of `master`.

Because a PR assignment bot needs to update the repository (to assign reviewers), it must have `GITHUB_TOKEN: WRITE` access, however, this is safe because the only user controlled input would be the PR assigners GitHub login.

#### Posting Status to Slack

Context `master`, Secrets: `SLACK: WRITE`

A bot that posts an update to Slack should not in the context of a PR, otherwise an attacker could potentially gain access to the secret. A bot that needs a Slack token should run in the context of `master`. It should read from an artifact uploaded by a unprivileged workflow using [actions/upload-artifact](https://github.com/actions/upload-artifact)and post that to Slack.

#### Running Tests

Context `HEAD` of PR, Secrets: `GITHUB_TOKEN: READ`

Running tests must occur in the context of `HEAD` of the PR to test new changes. Due to this, this workflow should only have access to `GITHUB_TOKEN: READ` and no access to any additional secrets.

#### Running tests and updating the repository

Context `HEAD` of PR, Secrets: `GITHUB_TOKEN: WRITE`

This is an dangerous workflow that should should never be used!

To accomplish the same goal, test should run in a unprivileged `GITHUB_TOKEN: READ` and [should trigger a privileged workflow upon completion](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/) that does have access to secrets but running in the context of `master`. See `Posting Status to Slack` above.

### Break Glass Situations

In break glass situations, repository administrators can update branch protection rules to [lift restrictions on administrators](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule#creating-a-branch-protection-rule) and merge in changes to workflows.
