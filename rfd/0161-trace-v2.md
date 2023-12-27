---
authors: Hugo Hervieux (hugo.hervieux@goteleport.com)
state: draft
---

# RFD 0161 - Trace v2

## Required Approvers

* Engineering @strideynet && ??
* Security: (@reedloden || @jentfoo)
* Product: (@xinding33 || @klizhentas)

## What

This RFD proposes a set of changes to create a version 2 of the
`gravitational/trace` library.

## Why

The main motivation is to align how `trace.Wrap()` behaves with the [Golang
standard library additions in 1.20](https://go.dev/blog/go1.20#standard-library-additions).

Currently `trace.Wrap()` implementation is wrapping only once, subsequent wraps
only append messages to the existing traced error.
Go's standard libraries are consecutively wrapping and creating an error
tree that supports both single error wrapping and error joining.

Implementing error wrapping like the rest of the go's ecosystem would allow us
to rely more on the standard `errors` library and build error trees easier
to represent and understand for end users, fixing issues such as:
https://github.com/gravitational/trace/issues/104.

## Details

The main change is to follow the standard `errors` library wrapping/unwrapping
mechanism by encapsulating errors instead of appending messages in the same
error each time `trace.Wrap` is invoked.

The `TraceErr` structure would hold a pointer to the wrapped error and look like:

```go
type TraceErr struct {
	Traces  *Traces
	Message string
	Err     error
	
	Fields map[string]interface{}
}
```

Combined with the `aggregate` error, this would represent an error tree.
This tree would represent more clearly the execution path and the various errors
that happened.

Retrieving the error message or error fields requires traversing the error tree.
The standard `errors` package offers functions able to traverse the tree:
`errors.Is` and `errors.As`.

### Interfaces

We keep the existing interfaces with the following modifications:
- Remove the `ErrorWrapper` interface
- Introduce the `TraceReporter` interface
- Remove the `Clone` function from the error interface

The resulting interfaces would look like
```go
type Error interface {
	error
	DebugReporter
	UserMessager
	TraceReporter

	GetFields() map[string]interface{}
}

type DebugReporter interface {
	DebugReport() string
}

type UserMessager interface {
	UserMessage() string
}

type TraceReporter interface {
	Trace() *Traces
}
```

### API

We keep the existing public functions with the following modifications:
- Remove `WrapWithMessage`, and `WithUserMessage` as `Wrap` can also take
  additional args and build a user message. Based on the current usage in the
  Teleport codebase, `Wrap(err, "doing X went wrong")` and `WrapWithMessage(err, "doix X went wrong")`
  are used interchangeably. From an implementation point of view, the only `Error`
  implementation puts everything in `e.Messages` list, regardless of the invoke
  function. The current `WrapWithUserMessage` behaviour is also error prone:
  wrapping a nil error returns a non-nil error.
- Introduce `WrapResult[T](result T, err error) (T, error)` as a helper to wrap
  an error when a function returns both an error and a result.
- Remove `Unwrap` as it is equivalent to `errors.Unwrap` and is also frequently
  misused. Most `Unwrap` invocations should use `errors.Is` or `errors.As` instead.
  The few legitimate `Unwrap` invocations can already be replaced by
  `errors.Unwrap`.

<details>
  <summary>API Preview</summary>

  ```go
  // SetDebug turns on/off debugging mode, that causes Fatalf to panic
  func SetDebug(enabled bool) {}
  
  // IsDebug returns true if debug mode is on
  func IsDebug() bool {}
  
  // Wrap takes the original error and wraps it into the Trace struct
  // memorizing the context of the error.
  func Wrap(err error, args ...interface{}) Error {}

  // WrapResult takes the original error and wraps it into the Trace struct
  // memorizing the context of the error. The result is passed as-is.
  func WrapResult[T any](result T, err error) (T, Error) {}
  
  // UserMessage returns user-friendly part of the error
  func UserMessage(err error) string {}
  
  // UserMessageWithFields returns user-friendly error with key-pairs as part of the message
  func UserMessageWithFields(err error) string {}
  
  // DebugReport returns debug report with all known information
  // about the error including stack trace if it was captured
  func DebugReport(err error) string {}
  
  // GetFields returns any fields that have been added to the error message
  func GetFields(err error) map[string]interface{} {}
  
  // Errorf is similar to fmt.Errorf except that it captures
  // more information about the origin of error, such as
  // callee, line number and function that simplifies debugging
  func Errorf(format string, args ...interface{}) (err error) {}
  
  // Fatalf - If debug is false Fatalf calls Errorf. If debug is
  // true Fatalf calls panic
  func Fatalf(format string, args ...interface{}) error {}
  
  // WithField adds additional field information to the error.
  func WithField(err Error, key string, value interface{}) *TraceErr {}
  
  // WithFields adds a map of additional fields to the error
  func WithFields(err Error, fields map[string]interface{}) *TraceErr {}
  
  // NewAggregate creates a new aggregate instance from the specified
  // list of errors
  func NewAggregate(errs ...error) error {}
  
  // NewAggregateFromChannel creates a new aggregate instance from the provided
  // errors channel.
  //
  // A context.Context can be passed in so the caller has the ability to cancel
  // the operation. If this is not desired, simply pass context.Background().
  func NewAggregateFromChannel(errCh chan error, ctx context.Context) error {}
  ```

  Note: all the `IsXXXError` and `NewXXXError` have been omitted for readability.
</details>


### Trace capture

Recursively wrapping errors raises a new question: when do we capture the traces?
Three models were considered, and the "capture on first wrap" was chosen.

#### Model 1: Always capture

Capture stack traces for every `trace.Wrap()` invocation when error is non-nil.
This approach seems very inefficient as most traces collected are superfluous.
To get the full trace of an error, one must traverse the error tree up to the
last `TraceReporter`.

#### Model 2: Capture on first wrap

Only capture stack traces when during the first `trace.Wrap()` invocation.
This is the closest behavior to what we are currently doing.

This can be achieved by introducing a `TraceReporter` interface:
```go
type TraceReporter interface {
	Trace() *Traces
}
```

and checking during wrapping if the wrapped error implements it:

```go
// trace checks if the error is already traced. If it is, it only wraps it.
// If it is not, it starts a new trace and collects call traces by parsing the
// call stack.
func trace(err error, message string) error {
	// wrap existing trace
	if e, ok := err.(TraceReporter); ok {
		return &TraceErr{e.Trace(), message, err}
	}
	// start new trace
	const depth = 2
	tr := internal.captureTraces(depth)
	return &TraceErr{&tr, message, err}
}
```

Aggregates should capture traces becauses not all aggregated errors might
be traced. This is also the current `trace.NewAggregate` behaviour.

#### Model 3: Never capture and trace when calling

This is the approach used by `errtrace`: https://github.com/bracesdev/errtrace

They enforce each caller to wrap the error. Wrapping adds the current location
into the error trace. This approach is radically different from regular stack
trace collection but can provide better results in more complex cases such as
reporting an error through multiple goroutines.

This approach is similar to calling `trace.Wrap(err, "call site information")`
on every error but is more systematic.

### Remote errors

Currently, remote errors are unpacked a new trace is started without capturing
the local stack trace. This makes troubleshooting harder as the boundary between
local and remote errors is not clearly defined.

To address this, we can wrap the error returned from `trail.FromGRPC()` in a
`RemoteErr` error that explicitely displays that the error is remote.

`RemoteErr` can optionally hold remote traces and `trace.Wrap` nested errors
when debug is `true` and a `RawTrace` was attached to the GRPC error.
However, `RemoteErr` must always `Unwrap` to the error returned by `FromGRPC`,
even when it unmarshalled a `RawTrace` from the dbeug data. This is done to
ensure consistency when walking the error tree with `errors.Is`. We don't want
to potentially have a different behaviour when running in debug mode.

Proposed interface and structure:

```go
type RemoteWrapper interface {
	UnwrapRemote() error
}

type RemoteErr struct {
	LocalErr  error
	RemoteErr error
}

func (r RemoteErr) Unwrap() error {
	return r.LocalErr
}

func (r RemoteErr) UnwrapRemote() error {
	if r.RemoteErr != nil {
		return r.RemoteErr
	}
	return r.LocalErr
}

func (r RemoteErr) UserMessage() string {
	return "@RemoteErr"
}

func (r RemoteErr) Error() string {
	return fmt.Sprintf("remote error: %s", r.LocalErr.Error())
}
```

#### A note about serialization

When in debug mode, some setups will propagate traces over the wire, this is done
by the `trail` package.

A client running `trace` and receiving a `trace/v2` trace won't be able to
deserialize the trace unless we implement a backward-compatible protocol, sending
both the `trace.RawTrace` and `tracev2.RawTrace` in different headers.

A `trace/v2` client should not receive a v1 `trace` as per Teleport
compatibility guarantees. We can implement unmarshalling v1 traces into v2 structs
if necessary.

### Error representation

We can represent the error tree YAML-like and put the emphasis on remote
or aggregated errors.

```
preparing dinner for my family:
  - dressing the table:
      pouring wine glasses in the living room:
        spilled the wine
  - making dinner:
      putting pizza in the oven:
        pizza is burning
  - inviting guests:
      - calling cousin:
          @RemoteErr:
            checking calendar:
              already booked
      - calling sister:
          @RemoteErr:
            checking calendar:
              already booked
```

<details>
<summary>Example program</summary>

```go
package main

import (
	"errors"
	"fmt"
	trace "tracev2"
)

var isDebug bool

type food string

func readInstructions() (string, error) {
	return "", nil
}

func makeDiner() (food, error) {
	instructions, err := readInstructions()
	if err != nil {
		return "", trace.Wrap(err, "reading instructions")
	}
	return trace.WrapResult(cookPizza(instructions))
}

func cookPizza(_ string) (food, error) {
	return "", trace.Wrap(errors.New("pizza is burning"), "putting pizza in the oven")
}

func dressTheTable(room string) error {
	return trace.Wrap(
		pourWine(room),
		"pouring wine glasses in the %s",
		room,
	)
}

func pourWine(_ string) error {
	return errors.New("spilled the wine")
}

func prepareEverything() error {
	invitationErr := trace.Wrap(
		inviteGuests([]string{"grandma", "parents", "sister", "cousin"}),
		"inviting guests",
	)
	
	_, err := makeDiner()
	dinnerErr := trace.Wrap(err, "making dinner")

	dressingError := trace.Wrap(dressTheTable("living room"), "dressing the table")
	return trace.NewAggregate(invitationErr, dinnerErr, dressingError)
}

func phoneCall(guest string) error {
	if guest != "sister" && guest != "cousin" {
		return nil
	}
	var remoteErr error
	// simulate when debug is enabled on the rmeote server
	if isDebug {
		remoteErr = trace.Wrap(
			errors.New("already booked"),
			"checking calendar",
		)
	}
	return trace.RemoteErr{
		LocalErr:  errors.New("not available"),
		RemoteErr: remoteErr,
	}
}

func inviteGuests(guests []string) error {
	var errs []error
	for _, guest := range guests {
		errs = append(errs, trace.Wrap(phoneCall(guest), "calling %s", guest))
	}
	return trace.NewAggregate(errs...)
}

func main() {
	isDebug = true
	err := trace.Wrap(prepareEverything(), "preparing dinner for my family")
	fmt.Println(err)
}
```
</details>

This error representation is user-friendly but might not render properly when
represented as a single string (e.g. in a JSON field). A more compact version
can be introduced for those cases, maybe more JSON-like.

### Debug reports

#### Single error

#### Aggregated errors

#### Remote error
