# Development Workflow Automation Bot 

## Purpose
This bot automates the workflow of the pull request process. It does this by automatically assigning reviewers to a pull request and checking pull request reviews for approvals.

## Prerequisites
### Set Up Secrets 

[Documentation for setting up secrets](https://docs.github.com/en/actions/reference/encrypted-secrets#creating-encrypted-secrets-for-a-repository)

#### Reviewers

Reviewers is a string represented as a json object with authors mapped to their respective required reviewers. 

The secret name must be `reviewers`.

Example: 

```json
    {
        "author1": ["reviewer0", "reviewer1"],
        "author2": ["reviewer2", "reviewer3", "reviewer4"]
    }
```

#### Default Reviewers

Default reviewers is a string represented as a list. 

The secret name must be `defaultreviewers`.

```
["defaultreviewer1", "defaultreviewer2", "defaultreviewer3", "defaultreviewer4"]
```


### Workflow Run Credentials

You will need to create an access token named `WORKFLOW_RUN_CREDENTIALS` with write permissions to actions. This is used for dimissing stale workflow runs. 

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




Create the following workflow files in the master branch:   
Assigning Reviewers

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
        run: cd .github/workflows/ci && go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }} --default-reviewers=${{ secrets.defaultreviewers }} --reviewers=${{ secrets.reviewers }} assign-reviewers

```

Checking reviews
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
        run: cd .github/workflows/ci && go run cmd/bot.go --token=${{ secrets.GITHUB_TOKEN }} --default-reviewers=${{ secrets.defaultreviewers }} --reviewers=${{ secrets.reviewers }} check-reviewers

```
