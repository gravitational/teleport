/*
Copyright 2015-2019 Gravitational, Inc.

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

// Package trace implements utility functions for capturing debugging
// information about file and line in error reports and logs.
package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"golang.org/x/net/context"
)

var debug int32

// SetDebug turns on/off debugging mode, that causes Fatalf to panic
func SetDebug(enabled bool) {
	if enabled {
		atomic.StoreInt32(&debug, 1)
	} else {
		atomic.StoreInt32(&debug, 0)
	}
}

// IsDebug returns true if debug mode is on, false otherwise
func IsDebug() bool {
	return atomic.LoadInt32(&debug) == 1
}

// Wrap takes the original error and wraps it into the Trace struct
// memorizing the context of the error.
func Wrap(err error, args ...interface{}) Error {
	if len(args) > 0 {
		format := args[0]
		args = args[1:]
		return WrapWithMessage(err, format, args...)
	}
	return wrapWithDepth(err, 2)
}

// Unwrap unwraps error to it's original error
func Unwrap(err error) error {
	if terr, ok := err.(Error); ok {
		return terr.OrigError()
	}
	return err
}

// UserMessage returns user-friendly part of the error
func UserMessage(err error) string {
	if err == nil {
		return ""
	}
	if wrap, ok := err.(Error); ok {
		return wrap.UserMessage()
	}
	return err.Error()
}

// UserMessageWithFields returns user-friendly error with key-pairs as part of the message
func UserMessageWithFields(err error) string {
	if err == nil {
		return ""
	}
	if wrap, ok := err.(Error); ok {
		if len(wrap.GetFields()) == 0 {
			return wrap.UserMessage()
		}

		var kvps []string
		for k, v := range wrap.GetFields() {
			kvps = append(kvps, fmt.Sprintf("%v=%q", k, v))
		}
		return fmt.Sprintf("%v %v", strings.Join(kvps, " "), wrap.UserMessage())
	}
	return err.Error()
}

// DebugReport returns debug report with all known information
// about the error including stack trace if it was captured
func DebugReport(err error) string {
	if err == nil {
		return ""
	}
	if wrap, ok := err.(Error); ok {
		return wrap.DebugReport()
	}
	return err.Error()
}

// GetFields returns any fields that have been added to the error message
func GetFields(err error) map[string]interface{} {
	if err == nil {
		return map[string]interface{}{}
	}
	if wrap, ok := err.(Error); ok {
		return wrap.GetFields()
	}
	return map[string]interface{}{}
}

// WrapWithMessage wraps the original error into Error and adds user message if any
func WrapWithMessage(err error, message interface{}, args ...interface{}) Error {
	trace := wrapWithDepth(err, 3)
	if trace != nil {
		trace.AddUserMessage(message, args...)
	}
	return trace
}

func wrapWithDepth(err error, depth int) Error {
	if err == nil {
		return nil
	}
	var trace Error
	if wrapped, ok := err.(Error); ok {
		trace = wrapped
	} else {
		trace = newTrace(depth+1, err)
	}

	return trace
}

// Errorf is similar to fmt.Errorf except that it captures
// more information about the origin of error, such as
// callee, line number and function that simplifies debugging
func Errorf(format string, args ...interface{}) (err error) {
	err = fmt.Errorf(format, args...)
	trace := wrapWithDepth(err, 2)
	trace.AddUserMessage(format, args...)
	return trace
}

// Fatalf - If debug is false Fatalf calls Errorf. If debug is
// true Fatalf calls panic
func Fatalf(format string, args ...interface{}) error {
	if IsDebug() {
		panic(fmt.Sprintf(format, args...))
	} else {
		return Errorf(format, args...)
	}
}

func newTrace(depth int, err error) *TraceErr {
	var buf [32]uintptr
	n := runtime.Callers(depth+1, buf[:])
	pcs := buf[:n]
	frames := runtime.CallersFrames(pcs)
	cursor := frameCursor{
		rest: frames,
		n:    n,
	}
	return newTraceFromFrames(cursor, err)
}

func newTraceFromFrames(cursor frameCursor, err error) *TraceErr {
	traces := make(Traces, 0, cursor.n)
	if cursor.current != nil {
		traces = append(traces, frameToTrace(*cursor.current))
	}
	for {
		frame, more := cursor.rest.Next()
		traces = append(traces, frameToTrace(frame))
		if !more {
			break
		}
	}
	return &TraceErr{
		Err:    err,
		Traces: traces,
	}
}

func frameToTrace(frame runtime.Frame) Trace {
	return Trace{
		Func: frame.Function,
		Path: frame.File,
		Line: frame.Line,
	}
}

type frameCursor struct {
	// current specifies the current stack frame.
	// if omitted, rest contains the complete stack
	current *runtime.Frame
	// rest specifies the rest of stack frames to explore
	rest *runtime.Frames
	// n specifies the total number of stack frames
	n int
}

// Traces is a list of trace entries
type Traces []Trace

// SetTraces adds new traces to the list
func (s Traces) SetTraces(traces ...Trace) {
	s = append(s, traces...)
}

// Func returns first function in trace list
func (s Traces) Func() string {
	if len(s) == 0 {
		return ""
	}
	return s[0].Func
}

// Func returns just function name
func (s Traces) FuncName() string {
	if len(s) == 0 {
		return ""
	}
	fn := filepath.ToSlash(s[0].Func)
	idx := strings.LastIndex(fn, "/")
	if idx == -1 || idx == len(fn)-1 {
		return fn
	}
	return fn[idx+1:]
}

// Loc points to file/line location in the code
func (s Traces) Loc() string {
	if len(s) == 0 {
		return ""
	}
	return s[0].String()
}

// String returns debug-friendly representaton of trace stack
func (s Traces) String() string {
	if len(s) == 0 {
		return ""
	}
	out := make([]string, len(s))
	for i, t := range s {
		out[i] = fmt.Sprintf("\t%v:%v %v", t.Path, t.Line, t.Func)
	}
	return strings.Join(out, "\n")
}

// Trace stores structured trace entry, including file line and path
type Trace struct {
	// Path is a full file path
	Path string `json:"path"`
	// Func is a function name
	Func string `json:"func"`
	// Line is a code line number
	Line int `json:"line"`
}

// String returns debug-friendly representation of this trace
func (t *Trace) String() string {
	dir, file := filepath.Split(t.Path)
	dirs := strings.Split(filepath.ToSlash(filepath.Clean(dir)), "/")
	if len(dirs) != 0 {
		file = filepath.Join(dirs[len(dirs)-1], file)
	}
	return fmt.Sprintf("%v:%v", file, t.Line)
}

// TraceErr contains error message and some additional
// information about the error origin
type TraceErr struct {
	// Err is the underlying error that TraceErr wraps
	Err error `json:"error"`
	// Traces is a slice of stack trace entries for the error
	Traces `json:"traces"`
	// Message is an optional message that can be wrapped with the original error.
	//
	// This field is obsolete, replaced by messages list below.
	Message string `json:"message,omitempty"`
	// Messages is a list of user messages added to this error.
	Messages []string `json:"messages,omitempty"`
	// Fields is a list of key-value-pairs that can be wrapped with the error to give additional context
	Fields map[string]interface{} `json:"fields,omitempty`
}

// Fields maps arbitrary keys to values inside an error
type Fields map[string]interface{}

// RawTrace is the trace error that gets passed over the wire.
type RawTrace struct {
	// Err is the json-encoded error.
	Err json.RawMessage `json:"error"`
	// Traces represents the error callstack.
	Traces `json:"traces"`
	// Message is the user message.
	//
	// This field is obsolete, replaced by messages list below.
	Message string `json:"message"`
	// Messages is a list of user messages added to this error.
	Messages []string `json:"messages"`
}

// AddUserMessage adds user-friendly message describing the error nature
func (e *TraceErr) AddUserMessage(formatArg interface{}, rest ...interface{}) *TraceErr {
	newMessage := fmt.Sprintf(fmt.Sprintf("%v", formatArg), rest...)
	e.Messages = append(e.Messages, newMessage)
	return e
}

// AddFields adds the given map of fields to the error being reported
func (e *TraceErr) AddFields(fields map[string]interface{}) *TraceErr {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{}, len(fields))
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	return e
}

// AddField adds a single field to the error wrapper as context for the error
func (e *TraceErr) AddField(k string, v interface{}) *TraceErr {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{}, 1)
	}
	e.Fields[k] = v
	return e
}

// UserMessage returns user-friendly error message
func (e *TraceErr) UserMessage() string {
	if len(e.Messages) > 0 {
		// Format all collected messages in the reverse order, with each error
		// on its own line with appropriate indentation so they form a tree and
		// it's easy to see the cause and effect.
		result := e.Messages[len(e.Messages)-1]
		for index, indent := len(e.Messages)-1, 1; index > 0; index, indent = index-1, indent+1 {
			result = fmt.Sprintf("%v\n%v%v", result, strings.Repeat("\t", indent), e.Messages[index-1])
		}
		return result
	}
	if e.Message != "" {
		// For backwards compatibility return the old user message if it's present.
		return e.Message
	}
	return UserMessage(e.Err)
}

// DebugReport returns developer-friendly error report
func (e *TraceErr) DebugReport() string {
	var buffer bytes.Buffer
	err := reportTemplate.Execute(&buffer, struct {
		OrigErrType    string
		OrigErrMessage string
		Fields         map[string]interface{}
		StackTrace     string
		UserMessage    string
	}{
		OrigErrType:    fmt.Sprintf("%T", e.Err),
		OrigErrMessage: e.Err.Error(),
		Fields:         e.Fields,
		StackTrace:     e.Traces.String(),
		UserMessage:    e.UserMessage(),
	})
	if err != nil {
		return fmt.Sprint("error generating debug report: ", err.Error())
	}
	return buffer.String()
}

var reportTemplate = template.Must(template.New("debugReport").Parse(reportTemplateText))
var reportTemplateText = `
ERROR REPORT:
Original Error: {{.OrigErrType}} {{.OrigErrMessage}}
{{if .Fields}}Fields:
{{range $key, $value := .Fields}}  {{$key}}: {{$value}}
{{end}}{{end}}Stack Trace:
{{.StackTrace}}
User Message: {{.UserMessage}}
`

// Error returns user-friendly error message when not in debug mode
func (e *TraceErr) Error() string {
	return e.UserMessage()
}

func (e *TraceErr) GetFields() map[string]interface{} {
	return e.Fields
}

// OrigError returns original wrapped error
func (e *TraceErr) OrigError() error {
	err := e.Err
	// this is not an endless loop because I'm being
	// paranoid, this is a safe protection against endless
	// loops
	for i := 0; i < maxHops; i++ {
		newerr, ok := err.(Error)
		if !ok {
			break
		}
		if newerr.OrigError() != err {
			err = newerr.OrigError()
		}
	}
	return err
}

// GoString formats this trace object for use with
// with the "%#v" format string
func (e *TraceErr) GoString() string {
	return e.DebugReport()
}

// maxHops is a max supported nested depth for errors
const maxHops = 50

// Error is an interface that helps to adapt usage of trace in the code
// When applications define new error types, they can implement the interface
// So error handlers can use OrigError() to retrieve error from the wrapper
type Error interface {
	error
	// OrigError returns original error wrapped in this error
	OrigError() error
	// AddMessage adds formatted user-facing message
	// to the error, depends on the implementation,
	// usually works as fmt.Sprintf(formatArg, rest...)
	// but implementations can choose another way, e.g. treat
	// arguments as structured args
	AddUserMessage(formatArg interface{}, rest ...interface{}) *TraceErr

	// AddField adds additional field information to the error
	AddField(key string, value interface{}) *TraceErr

	// AddFields adds a map of additional fields to the error
	AddFields(fields map[string]interface{}) *TraceErr

	// UserMessage returns user-friendly error message
	UserMessage() string

	// DebugReport returns developer-friendly error report
	DebugReport() string

	// GetFields returns any fields that have been added to the error
	GetFields() map[string]interface{}
}

// NewAggregate creates a new aggregate instance from the specified
// list of errors
func NewAggregate(errs ...error) error {
	// filter out possible nil values
	var nonNils []error
	for _, err := range errs {
		if err != nil {
			nonNils = append(nonNils, err)
		}
	}
	if len(nonNils) == 0 {
		return nil
	}
	return wrapWithDepth(aggregate(nonNils), 2)
}

// NewAggregateFromChannel creates a new aggregate instance from the provided
// errors channel.
//
// A context.Context can be passed in so the caller has the ability to cancel
// the operation. If this is not desired, simply pass context.Background().
func NewAggregateFromChannel(errCh chan error, ctx context.Context) error {
	var errs []error

Loop:
	for {
		select {
		case err, ok := <-errCh:
			if !ok { // the channel is closed, time to exit
				break Loop
			}
			errs = append(errs, err)
		case <-ctx.Done():
			break Loop
		}
	}

	return NewAggregate(errs...)
}

// Aggregate interface combines several errors into one error
type Aggregate interface {
	error
	// Errors obtains the list of errors this aggregate combines
	Errors() []error
}

// aggregate implements Aggregate
type aggregate []error

// Error implements the error interface
func (r aggregate) Error() string {
	if len(r) == 0 {
		return ""
	}
	output := r[0].Error()
	for i := 1; i < len(r); i++ {
		output = fmt.Sprintf("%v, %v", output, r[i])
	}
	return output
}

// Errors obtains the list of errors this aggregate combines
func (r aggregate) Errors() []error {
	return []error(r)
}

// IsAggregate returns whether this error of Aggregate error type
func IsAggregate(err error) bool {
	_, ok := Unwrap(err).(Aggregate)
	return ok
}
