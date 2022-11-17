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
gRPC. Errors in Typescript are beyond the scope of this RFD.

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

### Error Creation

### Error Handling

When receiving an error from a call, we generally have two choices:

- Propagate the error upwards
- Handle the error

### Error Propagation in Go

#### Use `trace.Wrap`

When propagating an error from a call to the caller of your function, first wrap
that error using `trace.Wrap(err)`.

This is important because it attaches additional context to the error. This
context includes information such as a stack trace to identify exactly where
the error came from.

#### Consider providing a message when wrapping

`trace.Wrap` also accepts a message to attach to an error when wrapping it:

```go
trace.Wrap(err, "writing to config for crystal analyzer")
```

This allows you to provide additional context as to what the code was trying to
do at the time that the error occurred. This is especially important when your
code makes multiple calls that may return similar errors.

For simple functions, or where errors are likely to already be descriptive,
attaching this message is not a necessity. Use your discretion.

Bad:

```go
func foo() error {
    if err := os.WriteFile("/long/unspecific/config/file", []bytes("bar"), 0660); err != nil {
        return err
    }
    if err := os.WriteFile("/short/config", []bytes("buzz"), 0660); err != nil {
        return err
    }
    return nil
}
```

Good:

```go
func foo() error {
    if err := os.WriteFile("/long/unspecific/config/file", []bytes("bar"), 0660); err != nil {
        return trace.Wrap(err, "writing config for bar9000")
    }
    if err := os.WriteFile("/short/config", []bytes("buzz"), 0660); err != nil {
        return trace.Wrap(err, "writing config for buzz-lightyear")
    }
    return nil
}
```

### Testing and Errors

When writing automated tests for code, we want to ensure we cover more than
just the happy paths. We should also ensure that the correct errors are
returned when something is incorrect.

#### Be specific

Try to be more specific than just checking that an error has been returned.

This is important because when testing more complicated code there are often
multiple points where an error could be returned - less specific checks could
mean that the test case designed to check a piece of validation passes due to an
error from another part of the unit.

Bad:

```go
func TestFooValidationFail(t *testing.T) {
    err := foo("some really bad input")
    require.Error(t, err)
}
```

Good:

```go
func TestFooValidationFail(t *testing.T) {
    err := foo("some really bad input")
    require.True(t, trace.IsBadParameter(err))
    // TODO: Include a "best" which shows being even more specific about the
    // returned error.
}
```

## The Future

This section will be expanded in a future PR. It should cover:

- What we need from errors.
- Current pain points experienced with errors.
- Potential solutions for those pain points.
