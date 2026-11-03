# AGENTS.md

## Review Guidelines
- Focus only on critical security, reliability, performance, and scalability issues.
- Ignore style, performance micro-optimizations, and readability nits unless they are tied to a significant failure

### What to Look For
- Authentication/authorization bypasses
- Secret leakage, unsafe logging, or credential exposure
- Unsafe defaults in security-sensitive areas
- Injection risks (SQL, command, template, path traversal, SSRF)
- Insecure crypto usage or key handling
- Privilege escalation or sandbox escapes
- Data corruption, durability failures, or irreversible loss scenarios
- Concurrency hazards that can cause outages or data races
- Reliability regressions: crash loops, panics, deadlocks, unbounded retries, nil pointer dereferences
- Adherence to the guidelines defined in [RFD 153](https://github.com/gravitational/teleport/blob/8ff3d34581424302182c217e4d8d3be45bf2b0b7/rfd/0153-resource-guidelines.md) of Teleport resource definitions, gRPC, backend storage, and cache APIs.

### Flaky Tests

Treat flaky tests as a reliability issue. Report a problem only when you can explain how the test can fail.

Teleport reads are often eventually consistent. A successful write does not mean that every cache has processed the change. Check the source of each read after a resource is created, updated, or deleted.

Look for these problems

- A test writes through an authoritative client and then reads through a cache, proxy, separate service, or `AccessPoint`
- A test assumes that a watcher or cache has processed a write without waiting for it
- A test expects a cached `Get` or `List` to return a new value or `NotFound` right after a write
- A test uses `time.Sleep` to wait for work or cache updates
- A test checks a result before background work is done
- Test cleanup races with goroutines, servers, or watchers
- Parallel tests share ports, environment variables, global state, clocks, or fixtures

For cache problems name the write path and the read path. Explain why the read can be stale. Do not report a problem if the read API guarantees that it will see the write.

Prefer clear synchronization. Read from the authoritative client when testing a write. When testing cache updates start a watcher before the write and wait for the matching event or revision. Wait for goroutines to finish.

Prefer `testing/synctest` for tests that depend on time or goroutine scheduling. Use a fake clock only when `testing/synctest` cannot be used.

Bounded polling is acceptable when the test covers eventual consistency. Adding a fixed sleep or increasing a timeout is not a fix.

### Documentation

When you are looking at a given product area find the relevant documentation in the docs/ directory to ensure you understand the context in which the code is used.
