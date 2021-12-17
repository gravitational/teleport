---
authors: Stephen Touset (stephen@squareup.com)
state: draft
---

# RFD 48 - Environment-Configured Custom HTTP Headers

## What

This RFD proposes that all HTTP clients should be configurable to add custom,
opaque HTTP headers to their requests using an environment variable.

## Why

At Square, we connect from corporate laptops to services inside of our cloud and
datacenter enviornments through a [BeyondCorp][https://cloud.google.com/beyondcorp]
proxy. Clients authenticate to the proxy by providing a specially-crafted HTTP
header that contains an opaque authentication token.

As such, we need a way to inject this header into requests made by the
webclient.

## Scope

These changes only affect the webclient (and roundtrip library), causing them to
look for user-provided headers in an environment variable and inserting those
headers into HTTP requests.

## UX

Teleport gains the power to be invoked with environment variables specifying
custom HTTP headers. Multiple headers may be separated with

```bash
env TELEPORT_WEBCLIENT_HEADERS="Authorization: Basic xxxx\nCookie: some_cookie=yyy" \
    tsh login --proxy teleport-proxy.example.com
```

## Security

If an attacker can control a user's environment when invoking the `tsh` command,
the user has already lost (e.g., `env LD_PRELOAD=/path/to/evil.so tsh`). So I do
not believe we need to whitelist or blacklist headers that we allow to be
provided with this mechanism.