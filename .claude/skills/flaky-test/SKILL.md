---
name: flaky-test
description: Run when the user asks to investigate, troubleshoot, or fix a flaky Go test.
---

# Flaky Test

You are debugging a flaky test. In Teleport, flaky tests often pass locally on
powerful developer machines, but then fail on constrained CI/CD hardware.

Sometimes the problem is due to the way the test is written, but there are
plenty of occaisions where the flaky test is highlighting a problem in the
implementation itself.

## Steps

1. Try to reproduce the failure. Locate the flaky test and compile a test
   binary from the directory containing the test: `go test -c -o flaky.test .`
2. Run the test under the `stress` tool:
   `stress -failfast -p 30 ./flaky.test -test.run TestNameHere`.
   If `stress` is not installed it can be installed with
   `go install golang.org/x/tools/cmd/stress@latest`.
   You may need to increase the `-p` (parallelism) flag in order to reproduce
   the flake. Avoid increasing the parallelism too much. Anything above `-p 100`
   starts to induce failures in common setup logic rather than in the test itself.
3. Read the test to understand what is happening. Add print statements to test
   and debug your theories.
4. Attept to fix the flakiness.
5. Repeat step 2 to verify that the flakiness is no longer observed. If the
   flakiness still occurs, continue to iterate and re-run the test under
   `stress` until it passes consistently.
6. When complete, summarize the cause of the flakiness and the fix that you
   came up with. Delete the `flaky.test` binary. Do not make any commits,
   the developer will want to review your changes and make adjustments.

## Rules

Prefer deterministic changes like channel-based notifications or
`testing/synctest` over retry-based solutions like `require.Eventually`.
Extending the timeout of a test is usually an anti-pattern. Try to fix the bug
in the implementation or rewrite the test to run faster.

Another way to make tests more reliable is to make them do less. Look for
opportunities to improve the runtime of the test. This might mean optimizing the
underlying code, better isolating the feature under test, or disabling features
that are enabled by default but otherwise unnecessary for the feature being
tested.
