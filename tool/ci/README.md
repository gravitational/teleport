# Development Workflow Automation Bot 

## Purpose
This bot automates the workflow of the pull request process. It does this by automatically assigning reviewers to a pull request and checking pull request reviews for approvals.

## Prerequisites
### Set Up Secrets 

[Documentation for setting up secrets](https://docs.github.com/en/actions/reference/encrypted-secrets#creating-encrypted-secrets-for-a-repository)

#### Reviewers

Reviewers is a json object encoded as a string with authors mapped to their required reviewers. This map MUST contain an empty string key that maps to default reviewers. 

Example: 

```json
    {
        "author1": ["reviewer0", "reviewer1"],
        "author2": ["reviewer2", "reviewer3", "reviewer4"],
        "": ["defaultreviewer0", "defaultreviewer1"]
    }
```
### Set up workflow configuration files 

This bot supports the following events: 

- Pull Request 
    - `assigned`
    - `opened` 
    - `reopened` 
    - `ready_for_review` 
    - `synchronize`
- Pull Request Review 
    - `submitted`
    - `edited`
    - `dismissed`


The following subcommands are used in the workflow files: 

| Subcommand     | Description |
| ----------- | ----------- |
| `assign-reviewers`      | Assigns reviewers to a pull request.      |
| `check-reviewers`  | Checks pull request for required reviewers.       |
| `dismiss-runs`  | Dismisses stale workflow runs on an interval configurable in the workflow configuration file.|



Create the following workflow files in the master branch:

_Assigning Reviewers_

```yaml
name: Assign
on: 
  pull_request_target:
    types: [assigned, opened, reopened, ready_for_review]

jobs:
  auto-request-review:
    name: Auto Request Review
    runs-on: ubuntu-latest
    steps:
      - name: Checkout branch
        uses: actions/checkout@v2
        with: 
          ref: <name of branch where bot code will persist>      
      - name: Installing the latest version of Go.
        uses: actions/setup-go@v2
      # Running "assign-reviewers" subcommand on bot.
      - name: Assigning reviewers 
        run: cd tool/ci && go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }} --reviewers=${{ secrets.reviewers }} assign-reviewers

```

_Checking reviews_

```yaml
name: Check
on: 
  pull_request_review:
    type: [submitted, edited, dismissed]
  pull_request_target: 
    types: [assigned, opened, reopened, ready_for_review, synchronize]

jobs: 
  check-reviews:
    name: Checking reviewers 
    runs-on: ubuntu-latest
    steps:
      - name: Checkout branch 
        uses: actions/checkout@v2
        with:
          ref: <name of branch where bot code will persist>
      - name: Installing the latest version of Go.
        uses: actions/setup-go@v2
        # Running "check-reviewers" subcommand on bot.
      - name: Checking reviewers
        run: cd tool/ci && go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }} --reviewers=${{ secrets.reviewers }} check-reviewers

```

_Dimiss stale workflow runs_

```yaml
# Workflow will run every 30 minutes and dismiss stale runs on all open pull requests. 
name: Dismiss Stale Workflows Runs
on:
  schedule:
    # Runs every 30 minutes. You can configure this to any interval. 
    - cron:  '0,30 * * * *' 

jobs: 
  dismiss-stale-runs:
    name: Dismiss Stale Workflow Runs
    runs-on: ubuntu-latest
    steps:
      - name: Checkout master branch 
        uses: actions/checkout@master
      - name: Installing the latest version of Go.
        uses: actions/setup-go@v2
        # Run "dismiss-runs" subcommand on bot.
      - name: Dismiss
        run: cd tool/ci && go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }} dismiss-runs
```