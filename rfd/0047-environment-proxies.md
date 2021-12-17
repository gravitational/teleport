---
authors: Stephen Touset (stephen@squareup.com)
state: draft
---

# RFD 47 - Environment-Configured Proxies

## What

This RFD proposes that all HTTP clients should be configurable to perform
requests through intermediate proxies, using the standard `HTTP_PROXY`,
`HTTPS_PROXY`, and `NOPROXY` enviroment variables.

## Why

Client environments sometimes need to issue requests through proxy servers,
either to get out of restricted corporate networks or to get into
[BeyondCorp][https://cloud.google.com/beyondcorp] production environments.

## Scope

The changes should be relatively limited in scope, requiring only that HTTP
clients are initialized with a transport that respects these environment
variables. The golang `http` library ships a feature that transparently enables
this behavior.

## UX

Teleport gains the power to be invoked with environment variables specifying the
locations of HTTP proxies.

```bash
env HTTPS_PROXY=http://proxy.example.com:80 tsh login --proxy teleport-proxy.example.com
```