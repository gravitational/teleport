
---
author: jane quintero (jane@goteleport.com)   
state: draft
---


# Bot 

## What 

This RFD proposes the implementation of using Github Actions to better manage the Teleport repository's pull requests. The first iteration of this implementation will include:  
- Auto assigning reviewers to pull requests. 
- Checking approvals for pull requests. 

## Why 

To improve speed and quality of the current pull request process.

## Details

This project will use the [go-github](https://github.com/google/go-github) client library to access the Github API to assign and check reviewers. 

Information about the pull request needs to be obtained in order to authenticate and use the client library to make API calls. Github actions allows you to [access execution context information](https://docs.github.com/en/enterprise-server@3.0/actions/reference/context-and-expression-syntax-for-github-actions). One of the default environment variables provided by Github actions is  `GITHUB_EVENT_PATH`, which is the path to file with the complete event payload. With this, we can gather information about the pull requests to make the necessary API calls. 

### Assigning Reviewers 

Reviewers will be assigned when a pull request is opened, ready for review, or reopened. 

```yaml
# Example workflow configuration 
on: pull_request_target
  
jobs:
  auto-request-review:
    name: Auto Request Review
    runs-on: ubuntu-latest
    steps:
        # Install Go  
      - uses: actions/setup-go@v1
        with:
          go-version: '1.16.5'
        # Run command
      - name: Assigning reviewers 
        run: go run .github/workflows/bot.go assign-reviewers
        env: 
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

```

This arg will be passed to assign reviewers: 

```
assign-reviewers
```

#### Worflow 

To know which reviewers to assign, a hardcoded JSON object will be used as a Github secret. Usernames will be the the name of the key and the value will be a list of required reviewers usernames. 

```json
 // Example json object 
 {
    "russjones: ["quinqu", "r0mant"],  
    "r0mant": ["awly", "webvictim"],
 }
```
To access this, we can use the `secrets` context to store the assignments in an environment variable.

```yaml
 // Store in environment variable
 env: 
  ASSIGNMENTS: ${{ secrets.assignments }}
```
[Creating repository secrets](https://docs.github.com/en/actions/reference/encrypted-secrets#creating-encrypted-secrets-for-a-repository)


I'll obtain the reviewers from the secrets and use this Github API endpoint to [request reviewers for the pull request](https://docs.github.com/en/rest/reference/pulls#request-reviewers-for-a-pull-request).

### Checking Reviews 

Every time a pull request review occurs event occurs, the bot will check if all the required reviewers have approved. 

```yaml
# Workflow will trigger on all pull request review event types
on: pull_request_review

jobs: 
  check-reviews:
    name: Checking reviewers 
    runs-on: ubuntu-latest
    steps:
      - name: Installing jq 
        run: sudo apt install jq
      - name: Getting base branch...
        run: BASE_BRANCH=`cat $GITHUB_EVENT_PATH | jq '.base | .ref'`
       # Check out base branch 
      - name: Check out branch 
        uses: actions/checkout@v2
        with:
          ref: env.BASE_BRANCH
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

This arg will be passed to check the reviewers. 

```
check-reviews
```

#### Workflow

A list of the current reviews for the pull request needs to be obtained to see who has approved. I will obtain the PR number from the exection context and will then use the [list reviews for a pull request](https://docs.github.com/en/rest/reference/pulls#list-reviews-for-a-pull-request) endpoint to get the reviews, see which users in the list whose review state is "APPROVED" (will parse from response), and compare with the approvers with required reviewers stored in the hardcoded JSON object stored in the Github secrets object. 


At this point, we need to rerun the workflow. We can obtain the run ID from the [Github context](https://docs.github.com/en/enterprise-server@3.0/actions/reference/context-and-expression-syntax-for-github-actions#github-context)(github.run_id). The run id number does not change if you rerun the workflow. To run the workflow again, we can call the [re-run a workflow](https://docs.github.com/en/rest/reference/actions#re-run-a-workflow) endpoint. 


### Authentication & Permissions

For authentication, Github actions provides a token to use in workflow, saved as `GITHUB_TOKEN` in the secrets context, to authenticate on behalf of Github actions. 

The token expires when the job is finished. 

The `GITHUB_TOKEN` can be set with the following permissions and the scope is sufficient for all jobs: 

```
permissions:
  actions: read|write|none
  pull-requests: read|write|none
```

[Setting permissions](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions#permissions).


### Security 

In order to prevent edits to the contents of the workflow directory after reviewers have approved, we need to invalidate approvals for following commits. This can be done by [creating a branch protection rule](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule#creating-a-branch-protection-rule). If a new commit is pushed, a [workflow re-run](https://docs.github.com/en/rest/reference/actions#re-run-a-workflow) will occur. 


The [CODEOWNERS](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners) feature can be used to assign reviewers who are able to approve edits to the `.github/workflows` directory.

### Bot Failures 

Reviewers will need to be manually added to pull request. 

The workflow runs in the context of master so if the workflow does not succeed due to a failure/bug in the bot, we need a way to make changes to `.github/workflows` without running actions and allow it to be merged. We can do this by ignoring changes to the path. However, the workflow will still run if some changes occur in paths that do not match the patterns in `paths-ignore`. 

```yaml
on:
  pull_request:
    types: [opened, ready_for_review, reopened]
    paths-ignore: '.github/workflows/**'
```      

__CODEOWNERS will still need to approve these changes before the edits get merged.__ 


[Ignoring paths](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions#example-ignoring-paths).

### Unit tests

- Adding reviewers to PR.
- Ensuring the correct reviewers are assigned to the author.
- Allow running merge command when required reviewers approve.
- Dont run merge command if all required reviewers haven't approved.
