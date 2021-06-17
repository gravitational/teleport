
---
author: jane quintero (jane@goteleport.com)   
state: draft
---


# Github Actions Bot 

## What 

This RFD proposes the implementation of using Github Actions to better manage the Teleport repository's pull requests. The first iteration of this implementation will include:  
- Assigning reviewers to pull requests. 
- Merge pull requests once all of the assigned reviewers have approved.  

## Why 

To improve speed and quality of the current pull request process.

## Details
 ### Assigning Reviewers 

We will assign reviewers when one of the following events happen:

| Event | Definition  |
|---|---|  
|  `ready_for_review` |  when pull request is ready for review, from draft  |   
|  `opened` | when a  pull request opens |   
| `reopened`  | when a pull request is reopened |   
  

This command will be run during the job: 

```
bot assign-reviewers
```

### Checking Reviews 

Every time a pull request review occurs, the bot will get all reviews and check if all the assigned reviewers have approved with:

```
bot check-reviews
```

### Merging Pull Requests 

In order to merge a pull request there are a few conditions that need to be met: 
- All reviewers need to approve the pull request.
- All the steps of the job need to pass in the context of master 
    - This is important because if they are run in the context of their own PR, they can change the way the workflow runs and make their PR pass requirements. 

When a PR meets all of the conditions it needs, the job wil run: 

```
bot merge
```

### Implementation 

This project will use [go-github](https://github.com/google/go-github) to access the Github API. 

Information about the pull request needs to be obtained in order to authenticate and use the client library to make API calls. Github actions allows you to [access execution context information](https://docs.github.com/en/enterprise-server@3.0/actions/reference/context-and-expression-syntax-for-github-actions), we can save needed values as environment variables via job configuration and use in the workflow.


| Name  | Environment Variable  | Expression |    
|---|---|---|   
|  Author | `PULL_REQUEST_AUTHOR`  |  github.event.pull_request.user.login |   
|  Pull Request Number |  `PULL_REQUEST_NUMBER` |  github.event.number  |     
| Repository  | `REPOSITORY_NAME`  |   github.repository  |    
| Owner | `REPOSITORY_OWNER` |  github.repository_owner |  
| Token   | `GITHUB_TOKEN` | secrets.GITHUB_TOKEN |

#### Assignment

To store which reviewers map to an author, a hardcoded JSON object can be used as a Github secret. To access this object to use in our bot, we can use the `secrets` context.

```json
 // Example json object 
 {
    "russjones: ["quinqu", "r0mant"],  
    "r0mant": ["awly", "webvictim"],
 }
```

```yaml
 // Store in env variable
 env: 
  ASSIGNMENTS: ${{ secrets.assignments }}
```

#### Approvals 

Every time a pull request review event occurs, a job will compare the Github secret values the author has to the PR reviews that are in an approved state and check if the required reviewers have approved.  

The [CODEOWNERS](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners) feature can be used to define who can approve edits to the `.github/workflows` directory.

In order to prevent edits to the contents of the workflow directory after reviewers have approved, we need to invalidate approvals for following commits. This can be done by [creating a branch protection rule](https://docs.github.com/en/github/administering-a-repository/defining-the-mergeability-of-pull-requests/managing-a-branch-protection-rule#creating-a-branch-protection-rule). If a new commit is pushed, a [workflow re-run](https://docs.github.com/en/rest/reference/actions#re-run-a-workflow) needs to occur. 


In the case of an update branch event, it is not needed invalidate approvals. 

#### Auto Updating Branches

In the case that a branch needs to be updated, we can use this [action](https://github.com/marketplace/actions/auto-update) on the Github marketplace to auto update. If this event occurs, we will not re-run workflow.  

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

In the case of a bot failure: 
- Reviewers will need to be manually added to pull request. 
- CODEOWNERS can override and approve or merge changes to `.github/workflows`. 
 
### Testing 

In order to run tests and not expose any sensitive information, we can use [act](https://github.com/nektos/act) which runs in a docker container that is set up the way Github actions sets up its instance. We will need to install docker on the actions instance. 

```
        ┌────────────────────────────┐  
        │  ┌─────────────────────┐   │  
        │  │                     │   │  
        │  │      act            │   │  
        │  │                     │   │   
        │  │      docker instance│   │   
        │  └─────────────────────┘   │   
        │   gh instance running linux│   
        └────────────────────────────┘   
```

To run act with secrets, we supply them as environment variables. 

Unit tests
- Adding reviewers to PR.
- Ensuring the correct reviewers are assigned to the author.
- Allow running merge command when required reviewers approve.
- Dont run merge command if all required reviewers haven't approved.


Integration tests (used with act)
- Context values are saved as environment variables. 
- Ensuring pull requests with new commits invalidate approvals. 
 
