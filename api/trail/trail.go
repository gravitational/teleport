/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package trail integrates trace errors with GRPC
//
// Example server that sends the GRPC error and attaches metadata:
//
//	func (s *server) Echo(ctx context.Context, message *gw.StringMessage) (*gw.StringMessage, error) {
//		trace.SetDebug(true) // to tell trace to start attaching metadata
//		// Send sends metadata via grpc header and converts error to GRPC compatible one
//		return nil, trail.Send(ctx, trace.AccessDenied("missing authorization"))
//	}
//
// Example client reading error and trace debug info:
//
//	var header metadata.MD
//	r, err := c.Echo(context.Background(), &gw.StringMessage{Value: message}, grpc.Header(&header))
//	if err != nil {
//		// FromGRPC reads error, converts it back to trace error and attaches debug metadata
//		// like stack trace of the error origin back to the error
//		err = trail.FromGRPC(err, header)
//
//		// this line will log original trace of the error
//		log.Errorf("error saying echo: %v", trace.DebugReport(err))
//		return
//	}
package trail

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// DebugReportMetadata is a key in metadata holding debug information
// about the error - stack traces and original error
const debugReportMetadata = "trace-debug-report"

// ToGRPC converts error to GRPC-compatible error
func ToGRPC(originalErr error) error {
	if originalErr == nil {
		return nil
	}

	// Avoid modifying top-level gRPC errors.
	if _, ok := status.FromError(originalErr); ok {
		return originalErr
	}

	code := codes.Unknown
	returnOriginal := false
	traverseErr(originalErr, func(err error) (ok bool) {
		if errors.Is(err, io.EOF) {
			// Keep legacy semantics and return the original error.
			returnOriginal = true
			return true
		}

		if s, ok := status.FromError(err); ok {
			code = s.Code()
			return true
		}

		// Duplicate check from trace.IsNotFound.
		if os.IsNotExist(err) {
			code = codes.NotFound
			return true
		}

		ok = true // Assume match

		var (
			accessDeniedErr      *trace.AccessDeniedError
			alreadyExistsErr     *trace.AlreadyExistsError
			badParameterErr      *trace.BadParameterError
			compareFailedErr     *trace.CompareFailedError
			connectionProblemErr *trace.ConnectionProblemError
			limitExceededErr     *trace.LimitExceededError
			notFoundErr          *trace.NotFoundError
			notImplementedErr    *trace.NotImplementedError
			oauthErr             *trace.OAuth2Error
		)
		if errors.As(err, &accessDeniedErr) {
			code = codes.PermissionDenied
		} else if errors.As(err, &alreadyExistsErr) {
			code = codes.AlreadyExists
		} else if errors.As(err, &badParameterErr) {
			code = codes.InvalidArgument
		} else if errors.As(err, &compareFailedErr) {
			code = codes.FailedPrecondition
		} else if errors.As(err, &connectionProblemErr) {
			code = codes.Unavailable
		} else if errors.As(err, &limitExceededErr) {
			code = codes.ResourceExhausted
		} else if errors.As(err, &notFoundErr) {
			code = codes.NotFound
		} else if errors.As(err, &notImplementedErr) {
			code = codes.Unimplemented
		} else if errors.As(err, &oauthErr) {
			code = codes.InvalidArgument
		} else {
			// *trace.RetryError not mapped.
			// *trace.TrustError not mapped.
			ok = false
		}

		return ok
	})
	if returnOriginal {
		return originalErr
	}

	return status.Error(code, trace.UserMessage(originalErr))
}

// FromGRPC converts error from GRPC error back to trace.Error
// Debug information will be retrieved from the metadata if specified in args
func FromGRPC(err error, args ...interface{}) error {
	if err == nil {
		return nil
	}

	statusErr := status.Convert(err)
	code := statusErr.Code()
	message := statusErr.Message()

	var e error
	switch code {
	case codes.OK:
		return nil
	case codes.NotFound:
		e = &trace.NotFoundError{Message: message}
	case codes.AlreadyExists:
		e = &trace.AlreadyExistsError{Message: message}
	case codes.PermissionDenied:
		e = &trace.AccessDeniedError{Message: message}
	case codes.FailedPrecondition:
		e = &trace.CompareFailedError{Message: message}
	case codes.InvalidArgument:
		e = &trace.BadParameterError{Message: message}
	case codes.ResourceExhausted:
		e = &trace.LimitExceededError{Message: message}
	case codes.Unavailable:
		e = &trace.ConnectionProblemError{
			Message: message,
			Err:     err,
		}
	case codes.Unimplemented:
		e = &trace.NotImplementedError{Message: message}
	default:
		e = err
	}
	if len(args) != 0 {
		if meta, ok := args[0].(metadata.MD); ok {
			e = decodeDebugInfo(e, meta)
			// We return here because if it's a trace.Error then
			// frames was already extracted from metadata so
			// there's no need to capture frames once again.
			var traceErr trace.Error
			if errors.As(e, &traceErr) {
				return e
			}
		}
	}
	traces := captureTraces(1)
	return &trace.TraceErr{Err: e, Traces: traces}
}

// setDebugInfo adds debug metadata about error (traces, original error)
// to request metadata as encoded property
func setDebugInfo(err error, meta metadata.MD) {
	var traceErr trace.Error
	if !errors.As(err, &traceErr) {
		return
	}

	out, err := json.Marshal(err)
	if err != nil {
		return
	}
	meta[debugReportMetadata] = []string{
		base64.StdEncoding.EncodeToString(out),
	}
}

// decodeDebugInfo decodes debug information about error
// from the metadata and returns error with enriched metadata about it
func decodeDebugInfo(err error, meta metadata.MD) error {
	if len(meta) == 0 {
		return err
	}
	encoded, ok := meta[debugReportMetadata]
	if !ok || len(encoded) != 1 {
		return err
	}
	data, decodeErr := base64.StdEncoding.DecodeString(encoded[0])
	if decodeErr != nil {
		return err
	}
	var raw trace.RawTrace
	if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
		return err
	}
	if len(raw.Traces) != 0 && len(raw.Err) != 0 {
		return &trace.TraceErr{Traces: raw.Traces, Err: err, Message: raw.Message}
	}
	return err
}

// traverseErr traverses the err error chain until fn returns true.
// Traversal stops on nil errors, fn(nil) is never called.
// Returns true if fn matched, false otherwise.
func traverseErr(err error, fn func(error) (ok bool)) (ok bool) {
	if err == nil {
		return false
	}

	if fn(err) {
		return true
	}

	type singleUnwrap interface {
		Unwrap() error
	}

	type aggregateUnwrap interface {
		Unwrap() []error
	}

	var singleErr singleUnwrap
	var aggregateErr aggregateUnwrap

	if errors.As(err, &singleErr) {
		return traverseErr(singleErr.Unwrap(), fn)
	}

	if errors.As(err, &aggregateErr) {
		for _, err2 := range aggregateErr.Unwrap() {
			if traverseErr(err2, fn) {
				return true
			}
		}
	}

	return false
}

// FrameCursor stores the position in a call stack
type frameCursor struct {
	// Current specifies the current stack frame.
	// if omitted, rest contains the complete stack
	Current *runtime.Frame
	// Rest specifies the rest of stack frames to explore
	Rest *runtime.Frames
	// N specifies the total number of stack frames
	N int
}

// CaptureTraces gets the current stack trace with some deep frames skipped
func captureTraces(skip int) trace.Traces {
	var buf [32]uintptr
	// +2 means that we also skip `CaptureTraces` and `runtime.Callers` frames.
	n := runtime.Callers(skip+2, buf[:])
	pcs := buf[:n]
	frames := runtime.CallersFrames(pcs)
	cursor := frameCursor{
		Rest: frames,
		N:    n,
	}
	return getTracesFromCursor(cursor)
}

// GetTracesFromCursor gets the current stack trace from a given cursor
func getTracesFromCursor(cursor frameCursor) trace.Traces {
	traces := make(trace.Traces, 0, cursor.N)
	if cursor.Current != nil {
		traces = append(traces, frameToTrace(*cursor.Current))
	}
	for i := 0; i < cursor.N; i++ {
		frame, more := cursor.Rest.Next()
		traces = append(traces, frameToTrace(frame))
		if !more {
			break
		}
	}
	return traces
}

func frameToTrace(frame runtime.Frame) trace.Trace {
	return trace.Trace{
		Func: frame.Function,
		Path: frame.File,
		Line: frame.Line,
	}
}
