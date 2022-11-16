---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 0 - Errors at Teleport

## What

Errors, and how errors are handled, are an important part of building robust
software. Robust software is able to gracefully handle error states itself, and
where it is unable to gracefully handle an error, provides enough information
to the user that they are able to rectify the situation themselves.

The first section of this RFD will codify existing practices around errors in
the codebase, and should act as a reference when writing and reviewing code.

The second section will determine what functionality we are currently missing
from our errors in Teleport, and set out a plan for improvements that can be
made.

This RFD primarily focusses on errors in Go and also on their propagation over
gRPC.

## The Present

This section codifies existing practices around errors in the codebase, and
should act as a reference when writing and reviewing code.

Useful external resources:

- <https://go.dev/doc/effective_go#errors>
- <https://github.com/golang/go/wiki/CodeReviewComments#error-strings>
- <https://go.dev/blog/go1.13-errors>

Tooling in use:

- <https://pkg.go.dev/errors>
- <https://github.com/gravitational/trace>
- <https://pkg.go.dev/google.golang.org/grpc/status>

### Error creation

### Error handling

When receiving an error from a call, we generally have two choices:

- Propagate the error upwards
- Handle the error

### Error propagation in Go

Error propagation refers to returnin

When returning an error from a call to the caller of a function, wrap that error
using `trace.Wrap`.

Why?

In the special case where the error is being returned from a gRPC call, ensure
`trail.FromGRPC`.

## The Future

What we need from errors:

- ?
- ?
