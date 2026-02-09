# Teleport API

Public APIs (primarily gRPC, with some HTTP) and client libraries for connecting
to Teleport backends. Used by Teleport binaries and available to external consumers.

See the [API documentation](https://goteleport.com/docs/zero-trust-access/api/)
for usage guides and reference.

- This module has its own `go.mod` and is licensed under Apache 2.0 (separate
  from the main Teleport repo which is AGPL).
- Public APIs and exported functions aim to maintain backward compatibility with
  the previous major version, with migration paths to avoid breaking changes.
