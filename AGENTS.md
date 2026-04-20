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
- Reliability regressions: crash loops, panics, deadlocks, unbounded retries

### Documentation

When you are looking at a given product area find the relevant documentation in the docs/ directory to ensure you understand the context in which the code is used.
