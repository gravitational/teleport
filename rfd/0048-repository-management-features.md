# Repository Management Features


## What 

Proposes the implementation of repository management features. The features include:

- Auto-assign reviewers. 
- Checking pull requests for approvals from required reviewers. 
- Backporting docs. 

## Why

To improve the development workflow process and manage the repository. 

---
## Auto-Assign Reviewers 

**Status:** _Implemented_


### What 

Automatically assign reviewers to pull requests. 

### Why 

To even out the review workload evenly amongst the team and to avoid having engineers assign reviewers to their own pull requests.  

### Implementation

Command: `assign-reviewers` 

The auto-assign reviewers command is called upon a pull request event by the workflow.  

```yaml
# Example workflow configuration 
name: Assign
on: 
  # Types of events the workflows will trigger on
  pull_request_target:
    types: [assigned, opened, reopened, ready_for_review]

jobs:
  auto-request-review:
    name: Auto Request Review
    runs-on: ubuntu-latest
    steps:
      - name: Checkout master branch
        uses: actions/checkout@master        
      - name: Installing the latest version of Go.
        uses: actions/setup-go@v2
       # Runs "assign-reviewers" subcommand on bot as shown below.
      - name: Assigning reviewers 
        run: go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }}  --reviewers=${{ secrets.reviewers }} assign-reviewers

```
### Finding reviewers to Assign 


#### General Assignment 

The authors/required reviewers are stored as a map in the bot source code. This is how the bot determines who to assign to the pull request. 
For the Teleport core team, there are designated reviewers, Teleport overall requires reviews from two repository admins, and external contributors also require reviews from two admins. 

When the assign bot is triggered to run, the author of the pull request is found from the event payload. It then sees if the author's username exists in the map. If it does, the value is their required reviewers. If the username is not in the map, there is a wildcard symbol key (*) that maps to a list of reviewers for external contributors. 

Example map: 

```go
 var Reviewers = map[string][]string {
    // Internal
    "baz":     {"bar", "fizz"},
	  "car":     {"baz", "foo"},
	  "fizz":    {"car", "baz"},
	  // External
	  "*": {"foo", "bar"},
 }

```

#### Docs Assignment 

There is a special case where people that work with docs need to be assigned. The bot checks if the pull request has docs changes which are:

- changes to `docs/`
- changes to `rfd/` 
- changes to files that end in `.md` or `.mdx`

The following table shows who is assigned to a pull request based on the file type changes. 

| Pull Request Files    | Reviewers |
| ----------- | ----------- |
| Docs and Code Changes      | Required and Docs Reviewers     |
| Only Docs Changes   | Docs Reviewers        |
| Only Code Changes | Required Reviewers| 
| No Code or Docs Changes | Required Reviewers | 

---


## Checking Pull Requests For Approvals

**Status:** _Implemeted_


### What 

To check if a pull request is mergeable by ensuring required reviewers have approved a pull request. 

### Why 

This ensures the pull request has the required reviewers without using `CODEOWNERS` and overall speeds up the development workflow because engineers do not need to check if they have approvals from their designated reviewers before merging.

### Implementation 

Command: `check-reviewers`

The `check-reviewers` subcommand is called by the workflow that gets triggered by pull request events and pull request review events. 


```yaml
# Example Check workflow
name: Check
on: 
  # Types of events the workflows will trigger on
  pull_request_review:
    type: [submitted, edited, dismissed]
  pull_request_target: 
    types: [assigned, opened, reopened, ready_for_review, synchronize]
jobs: 
  check-reviews:
    name: Checking reviewers 
    runs-on: ubuntu-latest
    steps:
      - name: Check out the master branch 
        uses: actions/checkout@master
      - name: Installing the latest version of Go.
        uses: actions/setup-go@v2
        # Runs "check-reviewers" subcommand on bot here.
      - name: Checking reviewers
        run: go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }}  --reviewers=${{ secrets.reviewers }} check-reviewers
```

_Note: `check-reviewers` is triggered on pull request events is because it is used in conjunction with the  `assign-reviewers` part of the bot. The assign portion will always pass (if reviewers were properly requested), but because of that, we need `check-reviewers` to fail as soon as a pull request is open_ 

`check-reviewers` uses the same method as `assign-reviewers` to determine which reviewers to check for approval from. See [Finding Reviewers to Assign](#finding-reviewers-to-assign).

--- 

## Dismissing Stale Runs 

Status: _Implemented_

### What

Dismiss stale runs for external contributors' pull requests. 

### Why 

The Github token in workflows triggered by pull request review events for external contributors pull requests is not granted the correct permissions to dismiss stale workflow runs. 

### Implementation

Command: `dimiss-runs`

Stale workflow runs are runs that are no longer up-to-date due to a new event triggering a new/more recently run workflow. They need to be dismissed so the pull request reflects status checks are correct. The bot simply gets all the pull requests in the repository and iterates through all of them, finding old workflow runs in each (old being not the most recent one) and deletes those runs. 

This portion of the bot runs as a CRON job on an interval of 30 minutes. 

```yaml
  #  Example dismissing stale runs workflow 
  name: Dismiss Stale Workflows Runs
on:
  schedule:
    - cron:  '0,30 * * * *'
     
jobs: 
  dismiss-stale-runs:
    name: Dismiss Stale Workflow Runs
    runs-on: ubuntu-latest
    steps:
      - name: Check out the master branch 
        uses: actions/checkout@master
      - name: Installing the latest version of Go.
        uses: actions/setup-go@v2
        # Run "dismiss-runs" subcommand on bot.
      - name: Dismiss
        run: cd .github/workflows/teleport-ci && go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }} dismiss-runs
```
