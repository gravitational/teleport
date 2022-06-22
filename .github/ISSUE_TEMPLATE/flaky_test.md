---
name: Flaky Test ❄
about: Report a flaky unit or integration test
title: '`TestName` flakiness'
labels: flaky tests
---

## Before submitting a new issue

Before filing a new issue, search https://github.com/gravitational/teleport/issues?q=is%3Aissue+is%3Aopen+label%3A%22flaky+tests%22 for
the flaky test's name to see if the same issue has already been reported. If so, double check the `Relevant snippet` to confirm that
the specific failure is the same. If it is, please react 👍 to this issue (so we can sort issues by reactions). If you have a link to the
full log of the test run, please add it to the `Link(s) to logs` section below. If instead you've encountered a new failure, edit the issue
and copy paste in a new `Failure` section below the existing one(s).

If there's no existing issue for this test, continue to file a new one:

1. Change `TestName` in the title to the name of the test that failed.
2. Use `git blame` to try and figure out who is most responsible for the test, and assign them to this issue.
3. **Delete this entire section and continue**.

## Failure

#### Link(s) to logs

- Update the link below with the similar link to the logs of the test run that failed, **then delete this line**.
- https://console.cloud.google.com/cloud-build/builds/f3a2add5-a72a-4332-8f5e-76134492da9d?project=ci-account

#### Relevant snippet

**replace everything below here with your snippet**

Be extra helpful by adding the relevant snippet of the logs that show where the failure takes place.
See the examples below.

##### `check` style test, ctrl-F for "FAIL:"

```
FAIL: services_test.go:159: ServicesSuite.TestSemaphoreContention

/workspace/lib/services/suite/suite.go:1222:
    c.Assert(err, check.IsNil)
... value *trace.TraceErr =
ERROR REPORT:
Original Error: *trace.LimitExceededError too much contention on semaphore connection/alice
Stack Trace:
	/workspace/lib/services/local/presence.go:760 github.com/gravitational/teleport/lib/services/local.(*PresenceService).AcquireSemaphore
	/workspace/lib/services/semaphore.go:245 github.com/gravitational/teleport/lib/services.AcquireSemaphoreLock
	/workspace/lib/services/suite/suite.go:1221 github.com/gravitational/teleport/lib/services/suite.(*ServicesTestSuite).SemaphoreContention.func1
	/opt/go/src/runtime/asm_amd64.s:1571 runtime.goexit
User Message: too much contention on semaphore connection/alice ("too much contention on semaphore connection/alice")
```

##### `testing` style test, ctrl-F for "Test:"

```
    tsh_test.go:667:
        	Error Trace:	tsh_test.go:667
        	Error:      	Received unexpected error:
        	            	exit code 1
        	Test:       	TestSSHAccessRequest
```

## Update the Flaky Test Tracker meta-issue

After submitting this issue, add a comment to our `Flaky Test Tracker` issue (https://github.com/gravitational/teleport/issues/9492) in the format:

`TestName`: https://github.com/gravitational/teleport/issues/<issue-number>

See https://github.com/gravitational/teleport/issues/9492#issuecomment-1158937319 for an example.

**Delete this section, submit the issue, and add the comment**
