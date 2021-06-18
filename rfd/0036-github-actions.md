
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

### Assigning Reviewers 

We will assign reviewers when a PR is opened, ready for review, or reopened: 

```yaml
# Example workflow configuration 
on:
  pull_request:
    # Job will run on these types of PR events
    types: [opened, ready_for_review, reopened]

jobs:
  auto-request-review:
    name: Auto Request Review
    runs-on: ubuntu-latest
    steps:
        # Check out master 
      - uses: actions/checkout@master
        # Install Go  
      - uses: actions/setup-go@v1
        with:
          go-version: '1.16.5'
      - name: Building binary 
        run: go build .github/workflows/bot.go
        # Run command
      - name: Assigning reviewers 
        run: bot assign-reviewers
        env: 
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

```

This command will be run during the job: 

```
bot assign-reviewers
```

### Checking Reviews 

Every time a pull request review occurs event occurs, the bot will check if all the required reviewers have approved. 

```yaml
# Workflow will trigger on all pull request review event types
on: pull_request_review

job: 
    # steps...

```

This command will check the reviewers 

```
bot check-reviews
```

### Implementation 

This project will use [go-github](https://github.com/google/go-github) to access the Github API. 

Information about the pull request needs to be obtained in order to authenticate and use the client library to make API calls. Github actions allows you to [access execution context information](https://docs.github.com/en/enterprise-server@3.0/actions/reference/context-and-expression-syntax-for-github-actions). One of the default environment variables provided by Github actions is  `GITHUB_EVENT_PATH`, which is the path to file with the complete webhook event payload. With this, we can gather information about the pull requests to make the necessary API calls. 

#### Assignment

To know which reviewers to assign, a hardcoded JSON object will be used as a Github secret. Usernames will be the the name of the key and the value will be a list of required reviewers usernames. To access this object to use in the bot, we can use the `secrets` context.

```json
 // Example json object 
 {
    "russjones: ["quinqu", "r0mant"],  
    "r0mant": ["awly", "webvictim"],
 }
```

```yaml
 // Store in environment variable
 env: 
  ASSIGNMENTS: ${{ secrets.assignments }}
```

[Creating repository secrets](https://docs.github.com/en/actions/reference/encrypted-secrets#creating-encrypted-secrets-for-a-repository)


The [CODEOWNERS](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners) feature can be used to assign reviewers who are able to approve edits to the `.github/workflows` directory.


#### Security 

In order to prevent edits to the contents of the workflow directory after reviewers have approved, we need to invalidate approvals for following commits. This can be done by [creating a branch protection rule](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule#creating-a-branch-protection-rule). If a new commit is pushed, a [workflow re-run](https://docs.github.com/en/rest/reference/actions#re-run-a-workflow) will occur. 


#### Authentication & Permissions

For authentication, Github actions provides a token to use in workflow, saved as `GITHUB_TOKEN` in the secrets context, to authenticate on behalf of Github actions. 

The token expires when the job is finished. 

The `GITHUB_TOKEN` can be set with the following permissions and the scope is sufficient for all jobs: 

```
permissions:
  actions: read|write|none
  pull-requests: read|write|none
```

[Setting permissions](https://docs.github.com/en/actions/reference/workflow-syntax-for-github-actions#permissions)

#### Bot Failures 


Reviewers will need to be manually added to pull request. 

Bot runs in the context of master so, if the workflow does not succeed due to a bug, we need a way to make changes to it and allow it to be merged. We can do this by adding a tag on the PR that needs to make changes to `.github/workflows` and dismiss the workflow. 

CODEOWNERS will still need to approve these changes before the edits get merged. 

```yaml
# Example config that ignores PR's with `override` tag:
on:
  pull_request:
    types: [opened, ready_for_review, reopened]
    tags-ignore:
        - 'override' 
```



### Unit tests

- Adding reviewers to PR.
- Ensuring the correct reviewers are assigned to the author.
- Allow running merge command when required reviewers approve.
- Dont run merge command if all required reviewers haven't approved.

