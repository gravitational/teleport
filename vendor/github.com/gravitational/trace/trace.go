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
package trace

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

var debug bool

func EnableDebug() {
	debug = true
}

// Wrap takes the original error and wraps it into the Trace struct
// memorizing the context of the error.
func Wrap(err error, args ...interface{}) Error {
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

// Fatalf. If debug is false Fatalf calls Errorf. If debug is
// true Fatalf calls panic
func Fatalf(format string, args ...interface{}) error {
	if debug {
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
				File: "unknown_file",
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
			File: filepath.Base(filePath),
			Path: filePath,
			Func: runtime.FuncForPC(pc).Name(),
			Line: line,
		},
		"",
	}
}

type Traces []Trace

func (s *Traces) SetTrace(t Trace) {
	*s = append(*s, t)
}

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

type Trace struct {
	File string
	Path string
	Func string
	Line int
}

func (t *Trace) String() string {
	return fmt.Sprintf("%v:%v", t.File, t.Line)
}

// TraceErr contains error message and some additional
// information about the error origin
type TraceErr struct {
	error
	Trace
	Message string
}

func (e *TraceErr) Error() string {
	return fmt.Sprintf("[%v:%v] %v %v", e.File, e.Line, e.Message, e.error)
}

func (e *TraceErr) OrigError() error {
	return e.error
}

// Error is an interface that helps to adapt usage of trace in the code
// When applications define new error types, they can implement the interface
// So error handlers can use OrigError() to retrieve error from the wrapper
type Error interface {
	error
	OrigError() error
}

type TraceSetter interface {
	Error
	SetTrace(Trace)
}
