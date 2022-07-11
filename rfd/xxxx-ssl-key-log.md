---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD X - SSL Key Logging

## What
Teleport will optionally log TLS session keys. See [curl](https://everything.curl.dev/usingcurl/tls/sslkeylogfile) or [Firefox](https://firefox-source-docs.mozilla.org/security/nss/legacy/key_log_format/index.html) for examples of this implemented elsewhere.

### Related issues
- [#12295](https://github.com/gravitational/teleport/issues/12995)

## Why
Logging session keys allows users to inspect Teleport traffic with tools such as Wireshark.

## Details
Key logging will apply in the following scenarios:
- All `tsh` clients
- All `tctl` clients
- Web API listener on proxies
- Auth API listener on auth servers

To enable keylogging, Teleport will use a new build flag, `sslkeylog`, enabled by setting the environment variable `ALLOW_SSLKEYLOGFILE` when compiling. When this build of Teleport is run and the `SSLKEYLOGFILE` environment variable is present, Teleport will open a file at `SSLKEYLOGFILE` and add it to the `tls.Config` of any client/listener than needs it (note: this means that the clients/listeners will be sharing a file handle. The key log file will be in append mode and calls to `os.File.Write` are atomic, so there should be no concurrency issues).

### Security
This feature is inherently insecure and should only be used to assist with debugging. Teleport should warn the user:
- when building via `make`
- when running the insecure build
- when creating the key log file