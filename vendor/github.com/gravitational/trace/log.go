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

// Package trace implements utility functions for capturing logs
package trace

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"runtime"
	rundebug "runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	// FileField is a field with code file added to structured traces
	FileField = "file"
	// FunctionField is a field with function name
	FunctionField = "func"
	// LevelField returns logging level as set by logrus
	LevelField = "level"
	// Component is a field that represents component - e.g. service or
	// function
	Component = "trace.component"
	// ComponentFields is a fields component
	ComponentFields = "trace.fields"
	// DefaultComponentPadding is a default padding for component field
	DefaultComponentPadding = 11
	// DefaultLevelPadding is a default padding for level field
	DefaultLevelPadding = 4
)

// IsTerminal checks whether writer is a terminal
func IsTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return terminal.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

// TextFormatter is logrus-compatible formatter and adds
// file and line details to every logged entry.
type TextFormatter struct {
	// DisableTimestamp disables timestamp output (useful when outputting to
	// systemd logs)
	DisableTimestamp bool
	// ComponentPadding is a padding to pick when displaying
	// and formatting component field, defaults to DefaultComponentPadding
	ComponentPadding int
	// EnableColors enables colored output
	EnableColors bool
	// FormatCaller is a function to return (part) of source file path for output.
	// Defaults to filePathAndLine() if unspecified
	FormatCaller func() (caller string)
}

// Format implements logrus.Formatter interface and adds file and line
func (tf *TextFormatter) Format(e *log.Entry) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			data = append([]byte("panic in log formatter\n"), rundebug.Stack()...)
			return
		}
	}()

	formatCaller := tf.FormatCaller
	if formatCaller == nil {
		formatCaller = formatCallerWithPathAndLine
	}

	caller := formatCaller()
	w := &writer{}

	// time
	if !tf.DisableTimestamp {
		w.writeField(e.Time.Format(time.RFC3339), noColor)
	}

	// level
	color := noColor
	if tf.EnableColors {
		switch e.Level {
		case log.DebugLevel:
			color = gray
		case log.WarnLevel:
			color = yellow
		case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
			color = red
		default:
			color = blue
		}
	}
	w.writeField(strings.ToUpper(padMax(e.Level.String(), DefaultLevelPadding)), color)

	// always output the component field if available
	padding := DefaultComponentPadding
	if tf.ComponentPadding != 0 {
		padding = tf.ComponentPadding
	}
	if w.Len() > 0 {
		w.WriteByte(' ')
	}
	value := e.Data[Component]
	var component string
	if reflect.ValueOf(value).IsValid() {
		component = fmt.Sprintf("[%v]", value)
	}
	component = strings.ToUpper(padMax(component, padding))
	if component[len(component)-1] != ' ' {
		component = component[:len(component)-1] + "]"
	}
	w.WriteString(component)

	// message
	if e.Message != "" {
		w.writeField(e.Message, noColor)
	}

	// rest of the fields
	if len(e.Data) > 0 {
		w.writeMap(e.Data)
	}

	// caller, if present, always last
	if caller != "" {
		w.writeField(caller, noColor)
	}

	w.WriteByte('\n')
	data = w.Bytes()
	return
}

// JSONFormatter implements logrus.Formatter interface and adds file and line
// properties to JSON entries
type JSONFormatter struct {
	log.JSONFormatter
}

// Format implements logrus.Formatter interface
func (j *JSONFormatter) Format(e *log.Entry) ([]byte, error) {
	if cursor := findFrame(); cursor != nil {
		t := newTraceFromFrames(*cursor, nil)
		new := e.WithFields(log.Fields{
			FileField:     t.Loc(),
			FunctionField: t.FuncName(),
		})
		new.Time = e.Time
		new.Level = e.Level
		new.Message = e.Message
		e = new
	}
	return j.JSONFormatter.Format(e)
}

// formatCallerWithPathAndLine formats the caller in the form path/segment:<line number>
// for output in the log
func formatCallerWithPathAndLine() (path string) {
	if cursor := findFrame(); cursor != nil {
		t := newTraceFromFrames(*cursor, nil)
		return t.Loc()
	}
	return ""
}

var frameIgnorePattern = regexp.MustCompile(`github\.com/(S|s)irupsen/logrus`)

// findFrames positions the stack pointer to the first
// function that does not match the frameIngorePattern
// and returns the rest of the stack frames
func findFrame() *frameCursor {
	var buf [32]uintptr
	// Skip enough frames to start at user code.
	// This number is a mere hint to the following loop
	// to start as close to user code as possible and getting it right is not mandatory.
	// The skip count might need to get updated if the call to findFrame is
	// moved up/down the call stack
	n := runtime.Callers(4, buf[:])
	pcs := buf[:n]
	frames := runtime.CallersFrames(pcs)
	for i := 0; i < n; i++ {
		frame, _ := frames.Next()
		if !frameIgnorePattern.MatchString(frame.File) {
			return &frameCursor{
				current: &frame,
				rest:    frames,
				n:       n,
			}
		}
	}
	return nil
}

const (
	noColor = -1
	red     = 31
	yellow  = 33
	blue    = 36
	gray    = 37
)

type writer struct {
	bytes.Buffer
}

func (w *writer) writeField(value interface{}, color int) {
	if w.Len() > 0 {
		w.WriteByte(' ')
	}
	w.writeValue(value, color)
}

func (w *writer) writeValue(value interface{}, color int) {
	var s string
	switch v := value.(type) {
	case string:
		s = v
		if needsQuoting(s) {
			s = fmt.Sprintf("%q", v)
		}
	default:
		s = fmt.Sprintf("%v", v)
	}
	if color != noColor {
		s = fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, s)
	}
	w.WriteString(s)
}

func (w *writer) writeError(value interface{}) {
	switch err := value.(type) {
	case Error:
		w.WriteString(fmt.Sprintf("[%v]", err.DebugReport()))
	default:
		w.WriteString(fmt.Sprintf("[%v]", value))
	}
}

func (w *writer) writeKeyValue(key string, value interface{}) {
	if w.Len() > 0 {
		w.WriteByte(' ')
	}
	w.WriteString(key)
	w.WriteByte(':')
	if key == log.ErrorKey {
		w.writeError(value)
		return
	}
	w.writeValue(value, noColor)
}

func (w *writer) writeMap(m map[string]interface{}) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == Component {
			continue
		}
		switch value := m[key].(type) {
		case log.Fields:
			w.writeMap(value)
		default:
			w.writeKeyValue(key, value)
		}
	}
}

func needsQuoting(text string) bool {
	for _, r := range text {
		if !strconv.IsPrint(r) {
			return true
		}
	}
	return false
}

func padMax(in string, chars int) string {
	switch {
	case len(in) < chars:
		return in + strings.Repeat(" ", chars-len(in))
	default:
		return in[:chars]
	}
}
