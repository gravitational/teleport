
---
author: jane quintero (jane@goteleport.com)   
state: implemented
---


# Repository Management with Github Actions 


## What and Why 

To inform repository management contributors of practices, vulnerabilities, security, and settings in place with Github Actions.


## Bot Edits and Overrides

The [CODEOWNERS](https://docs.github.com/en/github/creating-cloning-and-archiving-repositories/creating-a-repository-on-github/about-code-owners) feature is used to assign reviewers to `.github/workflows`. The status checks passing alone on a pull request will not be sufficient to get a pull request in with changes to `.github/workflows`, CODEOWNERS will need to approve the pull request for mergeability. 


If the bot has a bug or fails, repository admins can bypass the failed checks and force merge any pull request (this includes any bot changes). 

## Security 

### Authentication 

For authentication, Github Actions provides a token to use in workflow, saved as `GITHUB_TOKEN` in the `secrets` context, to authenticate on behalf of Github actions. The token expires when the job is finished. Personal access tokens (PATs) are NOT used because they do not expire at the end of a job. If an attacker were to get the `GITHUB_TOKEN`, they would only have the length of the run to do anything malicious, whereas an attacker that has a PAT could do a lot more because the token would have a longer token validity time or even no expiration. 

### Token Permissions

It is important to keep the `GITHUB_TOKEN` scope as small as possible by only granting the token the least possible permissions we can to run workflows. 

The max scope we have for all workflows in the Teleport repository are:
- `pull-requests:write` 
- `actions:write`
- `contents:write`

If additional permissions are needed in the future, here are some helpful resources to look through: 

- [Github Apps Permissions](https://docs.github.com/en/rest/reference/permissions-required-for-github-apps)
- [Control Permissions for the Github Token](https://github.blog/changelog/2021-04-20-github-actions-control-permissions-for-github_token/#:~:text=The%20GITHUB_TOKEN%20is%20an%20automatically,token%20when%20a%20job%20completes.) 

### External Contributions

To prevent edits to the contents of the workflow directory after CODEOWNERS have approved for external contributors, we invalidate approvals for the following commits (we also do this for all pull requests regardless of the directory). This is done via hitting the [dismiss a review](https://docs.github.com/en/rest/reference/pulls#dismiss-a-review-for-a-pull-request) endpoint for all reviews in an approved state. 


## Vulnerabilities

### Bypassing a Required Reviewer


The Github actions token is granted write permissions to pull requests and with this, an attacker can run a command that approves the pull request with malicious code. An attacker could only merge this pull request if the repository settings didn't require 2+ reviewers or if status checks weren't required. Each pull request requires 2 reviewers and status checks are required to pass before merging (Check and Assign). 

See the [Bypassing Required Reviewers using Github Actions](https://medium.com/cider-sec/bypassing-required-reviews-using-github-actions-6e1b29135cc7) article. 

### User Obtains the Github Token

If an attacker were to somehow obtain the Github token, the following is what they would have access to: 

- [Permissions](https://docs.github.com/en/rest/reference/permissions-required-for-github-apps#permission-on-actions) on Actions
- [Permissions](https://docs.github.com/en/rest/reference/permissions-required-for-github-apps#permission-on-pull-requests) on Pull Requests
- [Permissions](https://docs.github.com/en/rest/reference/permissions-required-for-github-apps#permission-on-contents) on Contents

### Attack Table  

| Action     | Scenario/Mitigation  |
| ----------- | ----------- |
| Cancels a run     |   An attacker could get all approvals on a pull request, trigger a `pull_request_target` event (such as `synchronize` which is a pushed commit to PR), and cancel a run in that commit with malicious code. If only internal contributors/CODEOWNERS have the ability to merge a PR from a fork, the security in place should be enough.     |
| Delete/edit comments on issues or pull requests.   | An attacker could edit or delete important comments. There doesn't seem to be a way to get a malicious commit in master this way.  |
| Delete logs of a workflow. | An attacker could delete a logs of a workflow run. The metadata for the run will still persist, including the status check. There doesn't seem to be a way to get a malicious commit in master this way. |
| Re-run a workflow.  | An attacker could re-run a workflow though there wouldn't be any benefit to them even if code was changes. Workflow would just run against the new code and pass/fail accordingly. | 
| Update an issue or a pull request. | An attacker could update the contents of a pull request or issue.  There doesn't seem to be a way to get a malicious commit in master this way. NOTE: Changing the contents of a pull request does not mean an attacker could push a commit to a pull request, hence changing the overall code (the token would need `contents:write`). Changing the contents of a pull request include, editing the title, description, or comments| 
| Merge a commit or pull request | An attacker can merge a malicious commit or pull request. |
| Get/Delete/Edit secrets | An attacker could steal, edit, or delete secrets. |

## Conclusion 

Anyone who is contributing to Teleport's repository management bot must be aware of the ways things are set up, from repository settings to general practices with Github Actions. Please review this RFD before making edits to `.github/workflows` and contribute to it as repository management evolves. 