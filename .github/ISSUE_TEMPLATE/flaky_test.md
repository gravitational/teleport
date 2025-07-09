---
name: Flaky Test ❄
about: Report a flaky unit or integration test
title: "`TestName` flakiness"
labels: flaky tests
---

## Before submitting a new issue

Before filing a new issue, search https://github.com/gravitational/teleport/issues?q=is%3Aissue+label%3A%22flaky+tests%22+ for
the flaky test's name to see if the same issue has already been reported.

If so, add a comment with the `Link(s) to logs` url (see section below). Double check the `Relevant snippet` to confirm that the specific failure is the same.
If you've encountered a new failure for the same test, also copy-paste the `Relevant snippet` (see section below).

If there's no existing issue for this test, continue to file a new one:

1. Change `TestName` in the title to the name of the test that failed.
2. Use `git blame` to try and figure out who is most responsible for the test, and assign them to this issue.
3. **Delete this entire section and continue**.

## Failure

#### Link(s) to logs

- Update the link below with the similar link to the logs of the test run that failed, **then delete this line**.
- https://github.com/gravitational/teleport/actions/runs/<run-id>/jobs/<job-id>

#### Relevant snippet

**replace everything below here with your snippet**

Be extra helpful by adding the relevant snippet of the logs that show where the failure takes place. Typically ctrl+F searching for "Test:" will bring you to the right spot.
If "Test:" doesn't work, try ctrl+F "Fail:".

The example below shows what a typical failure looks like (though sometimes no such log will exist, for example in the case of a timeout).

```
    tsh_test.go:667:
        	Error Trace:	tsh_test.go:667
        	Error:      	Received unexpected error:
        	            	exit code 1
        	Test:       	TestSSHAccessRequest
```
