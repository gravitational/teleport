
---
author: jane quintero (jane@goteleport.com) Russell Jones (rjones@goteleport.com)
state: draft
---

# Use GitHub Actions for CI and Automation

## What/Why

This RFD proposes using Github Actions for CI and automation purposes to improve the development process and velocity of Teleport. This includes, but is not limited to the following:

* Adding bots to automate parts of the development process.
* Running unit and integration tests on internal and external PRs.
* Running sanity tests against `master`.
* Tracking and reporting test failures.
* Tracking and reporting regressions.

The first iteration will lay out the security properties that we will maintained as well as two initial workflows (review assignment and approval).

## Details

### Security Properties

The following properties should be maintained.

* We need support internal and external contributors.
* Automation failures should support an override method for break-glass scenarios.
* Changes to workflows should require repository administrator approval.

To maintain these properties, the following is proposed.

* The `.github/workflows` directory should be [protected by `CODEROWNERS`](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners) requiring approval by repository administrator (like @klizhentas, @russjones, or @r0mant) before merge. This is to prevent an attacker from exploiting a bug in a approval workflow itself and using that to change the approval workflow and then make malicious commits to the repository.
* [Branch protection rules](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/) should be used to dismiss approvals on a PR when new changes are pushed to a PR after it has been approved. This is to prevent a user from submitting a non-malicious PR, waiting for approval, then updating it before merge.
* [PRs from an external fork should never have access to repository secrets](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/). This is to prevent an external user from submitting a malicous PR that ex-filtrates secrets.

### Workflows

Two initial workflows are proposed: automated assignment of reviewers and automated approval of PRs. Both will be paired with a small Go program with minimal external dependencies. This approach is taken as it's something we have a lot of (Go developers) as well as allowing us to write unit tests.

### Assign Reviewer

A assign reviewer workflow will be created to automatically assign reviewers to all PRs.

The initial version of the assign reviewer workflow will use a hard coded list of reviewers that are stored as a [repository secret](https://docs.github.com/en/actions/reference/encrypted-secrets). 

#### Workflow

```yaml
# This workflow is run whenever a Pull Request is opened, re-opened, or taken
# out of draft (ready for review).
#
# NOTE: Due to the sensitive nature of this workflow, it must always be run
# against master AND with minimal permissions. These properties must always
# be maintained!
# 
# While it's tempting to use "pull_request_target" which automatically runs
# on the base of the PR, that target also grants the GITHUB_TOKEN write access
# to the repository.
on: pull_request
  
jobs:
  auto-request-review:
    name: Auto Request Review
    runs-on: ubuntu-latest

    steps:
      # Checkout master branch of Teleport repository. This is to prevent an
      # attacker from submitting their own review assignment logic.
      - uses: actions/checkout@v2
        with:
          ref: master
      
      # Install the latest version of Go.
      - uses: actions/setup-go@v2
      
      # Run "assign-reviewers" subcommand on bot.
      - name: Assign Reviews 
        run: go run .github/workflows/bot.go assign-reviewers
        env: 
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          REVIEWERS: ${{ secrets.REVIEWERS }}
```

#### Implementation

The Go program will have a `assign-reviewers` subcommand that reads in the `REVIEWERS` environment variable. This variable will be a JSON object with a hard coded list of reviewers. An example `REVIEWERS` object is below.

```json
{
   "russjones: ["quinqu", "r0mant"],  
   "r0mant": ["quinqu", "russjones"],
   "quinqu": ["russjones", "r0mant"]
}
```

If the PR author is not in listed in the JSON object (for example, a new team member or external contributor), the PR will be assigned to @russjones, @r0mant, and @awly.

The [Request reviewers for a pull request](https://docs.github.com/en/rest/reference/pulls#request-reviewers-for-a-pull-request) API will be used to assign reviewers to the Pull Request.

### Checking Reviews 

Every time a pull request review event occurs, the bot will check if all the required reviewers have approved. 

```yaml
# Workflow will trigger on all pull request review event types
on: pull_request_review

jobs: 
  check-reviews:
    name: Checking reviewers 
    runs-on: ubuntu-latest
    steps:
       # Check out base branch 
      - name: Check out branch 
        uses: actions/checkout@master
        # Install Go  
      - uses: actions/setup-go@v1
        with:
          go-version: '1.16.5'
        # Run command
      - name: Assigning reviewers 
        run: go run github/workflows/bot.go check-reviewers
        env: 
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```






###




  * Auto assigning reviewers to pull requests. 
  * Checking approvals for pull requests. 


This project will use the [go-github](https://github.com/google/go-github) client library to access the Github API to assign and check reviewers. 

Information about the pull request needs to be obtained in order to authenticate and use the client library to make API calls. Github actions allows you to [access execution context information](https://docs.github.com/en/enterprise-server@3.0/actions/reference/context-and-expression-syntax-for-github-actions). One of the default environment variables provided by Github actions is  `GITHUB_EVENT_PATH`, which is the path to file with the complete event payload. With this, we can gather information about the pull requests to make the necessary API calls. 



#### Workflow

A list of the current reviews for the pull request needs to be obtained to see who has approved. The PR number will be obtained from the execution context and will then use the [list reviews for a pull request](https://docs.github.com/en/rest/reference/pulls#list-reviews-for-a-pull-request) endpoint to get the reviews, see which users in the list whose review state is "APPROVED" (will parse from response), and compare with the approvers with required reviewers stored in the hardcoded JSON object stored in the Github secrets object. 


At this point, we need to rerun the workflow. We can obtain the run ID from the [Github context](https://docs.github.com/en/enterprise-server@3.0/actions/reference/context-and-expression-syntax-for-github-actions#github-context). The run id number does not change if you rerun the workflow. To run the workflow again, we can call the [re-run a workflow](https://docs.github.com/en/rest/reference/actions#re-run-a-workflow) endpoint. 


### Authentication & Permissions

For authentication, Github actions provides a token to use in workflow, saved as `GITHUB_TOKEN` in the secrets context, to authenticate on behalf of Github actions. 

The token expires when the job is finished. 

The `GITHUB_TOKEN` can be set with the following permissions and the scope is sufficient for all jobs: 

```yaml
permissions:
  # to re-run workflow 
  actions: read|write|none
  # to assign and check reviewers
  pull-requests: read|write|none
```

[Setting permissions](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions#permissions).


### Bot Changes/Failures 

The [CODEOWNERS](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners) feature will be used to assign reviewers who are able to approve edits to the `.github/workflows` directory if there is change to `.github/workflows`.

The workflow runs in the context of master so if the workflow does not succeed due to a failure/bug in the bot, we need a way to make changes to `.github/workflows` without running actions and allow it to be merged. We can do this by ignoring changes to the path. However, the workflow will still run if some changes occur in paths that do not match the patterns in `paths-ignore`. 

```yaml
on:
  pull_request:
    types: [opened, ready_for_review, reopened]
    paths-ignore: '.github/workflows/**'
```      

__CODEOWNERS will need to approve these changes before the edits get merged.__ 


[Ignoring paths](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions#example-ignoring-paths).

#### Security 

In order to prevent edits to the contents of the workflow directory after CODEOWNERS have approved for external contributors, we need to invalidate approvals for following commits. This can be done by [dismissing the reviews](https://docs.github.com/en/rest/reference/pulls#dismiss-a-review-for-a-pull-request) when there is a push event to the pull request that is making changes to `.github/workflows`. The process is the same for internal contributors making changes to `.github/workflows`, except additional commits will not invalidate approvals. 

The bot responsible for checking for stale approvals will use a JSON list populated with internal contributor usernames stored as a secret and check if the author's username is in that list. 

Once all reviewers have approved, a workflow in the context of the pull request will be run to ensure the changes run. 
```
 External pull request to make changes in `.github/workflows` process

                   ┌───────────────┐
                   │  External     │    Workflow does not initally run if
                   │  contributor  │    changes are made to
                   │  pull request │    `.github/workflows`
                   └───────┬───────┘
                           │
                           │
                           │
                 ┌─────────┴────────┐             ┌────────────────────┐
                 │                  │             │                    │     - invalidates existing approvals
                 │                  │   true      │                    │     - runs workflow in ctx of master/base
                 │  makes changes?  ├──────────►  │   run workflow     │
                 │ (pushes commit)  │             │                    ├────┐
                 │                  │             │                    │    │
                 └─────────┬────────┘             └────────────────────┘    │
                           │                             ▲                  │
                           │                             │        ┌─────────┴─────┐
                   false   │                        true └────────┤               │
                           │                                      │    more       │
                           ▼                                      │   changes?    │
                    ┌───────────────┐                             │               |
     workflow will  │               │                             └──┬────────────┘
trigger on PR review│  wait for     │             false              │
       event        │  approvals    │  ◄─────────────────────────────┘
              ┌─────┤               │
              │     └───────┬───▲───┘
              │             │   │
    approved  │             └───┘  not yet approved
              │
          ┌───┴──────────────┐
          │  run actions in  │
          │  context of PR   │
          └──────────────────┘
```

Notes: 

There is no way GitHub can invalidate commits on a particular type of contributor. Invalidating stale commits seems to only be repository wide rule.

If there ever is a major bug and a PR needs to be merged then we can specify certain people who can dismiss all PR reviews and repository admins can disable the workflows.

[Invalidating stale commits and specifying who can dismiss reviews](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule).

[Disabling workflows](https://docs.github.com/en/actions/managing-workflow-runs/disabling-and-enabling-a-workflow).
### Unit tests

- Adding reviewers to PR.
- Ensuring the correct reviewers are assigned to the author.
- Allow running merge command when required reviewers approve.
- Dont run merge command if all required reviewers haven't approved.
