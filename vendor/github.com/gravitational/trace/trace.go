/*
Copyright 2015 Gravitational, Inc.

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
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

var debug int32

// EnableDebug turns on debugging mode, that causes Fatalf to panic
func EnableDebug() {
	atomic.StoreInt32(&debug, 1)
}

// IsDebug returns true if debug mode is on, false otherwize
func IsDebug() bool {
	return atomic.LoadInt32(&debug) == 1
}

// Wrap takes the original error and wraps it into the Trace struct
// memorizing the context of the error.
func Wrap(err error, args ...interface{}) Error {
	if err == nil {
		return nil
	}

	t := newTrace(runtime.Caller(1))
	if s, ok := err.(TraceSetter); ok {
		s.SetTrace(t.Trace)
		return s
	}
	t.error = err
	if len(args) != 0 {
		t.Message = fmt.Sprintf(fmt.Sprintf("%v", args[0]), args[1:]...)
	}
	return t
}

// Errorf is similar to fmt.Errorf except that it captures
// more information about the origin of error, such as
// callee, line number and function that simplifies debugging
func Errorf(format string, args ...interface{}) error {
	t := newTrace(runtime.Caller(1))
	t.error = fmt.Errorf(format, args...)
	return t
}

// Fatalf - If debug is false Fatalf calls Errorf. If debug is
// true Fatalf calls panic
func Fatalf(format string, args ...interface{}) error {
	if IsDebug() {
		panic(fmt.Sprintf(format, args))
	} else {
		return Errorf(format, args)
	}
}

func newTrace(pc uintptr, filePath string, line int, ok bool) *TraceErr {
	if !ok {
		return &TraceErr{
			nil,
			Trace{
				Path: "unknown_path",
				Func: "unknown_func",
				Line: 0,
			},
			"",
		}
	}
	return &TraceErr{
		nil,
		Trace{
			Path: filePath,
			Func: runtime.FuncForPC(pc).Name(),
			Line: line,
		},
		"",
	}
}

// Traces is a list of trace entries
type Traces []Trace

// SetTrace adds a new entry to the list
func (s *Traces) SetTrace(t Trace) {
	*s = append(*s, t)
}

// String returns debug-friendly representaton of traces
func (s Traces) String() string {
	if len(s) == 0 {
		return ""
	}
	out := make([]string, len(s))
	for i, t := range s {
		out[i] = t.String()
	}
	return strings.Join(out, ",")
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
	error
	Trace
	Message string
}

func (e *TraceErr) Error() string {
	return fmt.Sprintf("[%v] %v %v", e.String(), e.Message, e.error)
}

// OrigError returns original wrapped error
func (e *TraceErr) OrigError() error {
	err := e.error
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

// maxHops is a max supported nested depth for errors
const maxHops = 50

// Error is an interface that helps to adapt usage of trace in the code
// When applications define new error types, they can implement the interface
// So error handlers can use OrigError() to retrieve error from the wrapper
type Error interface {
	error
	OrigError() error
}

// TraceSetter indicates that this error can store traces
type TraceSetter interface {
	Error
	SetTrace(Trace)
}
