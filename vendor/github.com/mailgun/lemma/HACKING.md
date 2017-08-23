# Hacking

Contributions to lemma from the community are welcome and encouraged! To ensure everything happens in an orderly fashion:

1. If you are contributing a feature, please create a GitHub issue first. This allows us to discuss the issue and agree on a implementation.
2. If you are contributing a bug fix, feel free to submit the PR directly.

When submitting a Pull Request, please include the purpose, implementation details, and any related commits. For example:

---

**Purpose**

This PR fixed a security critical issue: all keys and nonces are to 0! This allows an attacker to trivially decrypt all ciphertext!

**Implementation**

Changed the key generation algorithm to obtain random data from /dev/urandom instead of /dev/zero.

**Related Commit/PR**

N/A
