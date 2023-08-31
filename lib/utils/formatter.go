/*

 Copyright 2022 Gravitational, Inc.

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

package utils

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type TextFormatter struct {
	// ComponentPadding is a padding to pick when displaying
	// and formatting component field, defaults to DefaultComponentPadding
	ComponentPadding int
	// EnableColors enables colored output
	EnableColors bool
	// FormatCaller is a function to return (part) of source file path for output.
	// Defaults to filePathAndLine() if unspecified
	FormatCaller func() (caller string)
	// ExtraFields represent the extra fields that will be added to the log message
	ExtraFields []string
	// TimestampEnabled specifies if timestamp is enabled in logs
	timestampEnabled bool
	// CallerEnabled specifies if caller is enabled in logs
	callerEnabled bool
}

type writer struct {
	bytes.Buffer
}

const (
	noColor        = -1
	red            = 31
	yellow         = 33
	blue           = 36
	gray           = 37
	levelField     = "level"
	componentField = "component"
	callerField    = "caller"
	timestampField = "timestamp"
	messageField   = "message"
)

func NewDefaultTextFormatter(enableColors bool) *TextFormatter {
	return &TextFormatter{
		ComponentPadding: trace.DefaultComponentPadding,
		FormatCaller:     formatCallerWithPathAndLine,
		ExtraFields:      KnownFormatFields.names(),
		EnableColors:     enableColors,
		callerEnabled:    true,
		timestampEnabled: false,
	}
}

// CheckAndSetDefaults checks and sets log format configuration
func (tf *TextFormatter) CheckAndSetDefaults() error {
	// set padding
	if tf.ComponentPadding == 0 {
		tf.ComponentPadding = trace.DefaultComponentPadding
	}
	// set caller
	tf.FormatCaller = formatCallerWithPathAndLine

	// set log formatting
	if tf.ExtraFields == nil {
		tf.timestampEnabled = true
		tf.callerEnabled = true
		tf.ExtraFields = KnownFormatFields.names()
		return nil
	}
	// parse input
	res, err := parseInputFormat(tf.ExtraFields)
	if err != nil {
		return trace.Wrap(err)
	}

	if slices.Contains(res, timestampField) {
		tf.timestampEnabled = true
	}

	if slices.Contains(res, callerField) {
		tf.callerEnabled = true
	}

	tf.ExtraFields = res
	return nil
}

// Format formats each log line as configured in teleport config file
func (tf *TextFormatter) Format(e *log.Entry) ([]byte, error) {
	var data []byte
	caller := tf.FormatCaller()
	w := &writer{}

	// write timestamp first if enabled
	if tf.timestampEnabled {
		w.writeField(e.Time.Format(time.RFC3339), noColor)
	}

	for _, match := range tf.ExtraFields {
		switch match {
		case "level":
			color := noColor
			if tf.EnableColors {
				switch e.Level {
				case log.DebugLevel, log.TraceLevel:
					color = gray
				case log.WarnLevel:
					color = yellow
				case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
					color = red
				default:
					color = blue
				}
			}
			w.writeField(strings.ToUpper(padMax(e.Level.String(), trace.DefaultLevelPadding)), color)
		case "component":
			padding := trace.DefaultComponentPadding
			if tf.ComponentPadding != 0 {
				padding = tf.ComponentPadding
			}
			if w.Len() > 0 {
				w.WriteByte(' ')
			}
			value := e.Data[trace.Component]
			var component string
			if reflect.ValueOf(value).IsValid() {
				component = fmt.Sprintf("[%v]", value)
			}
			component = strings.ToUpper(padMax(component, padding))
			if component[len(component)-1] != ' ' {
				component = component[:len(component)-1] + "]"
			}
			w.WriteString(component)
		default:
			if !KnownFormatFields.has(match) {
				return nil, trace.BadParameter("invalid log format key: %v", match)
			}
		}
	}

	// always use message
	if e.Message != "" {
		w.writeField(e.Message, noColor)
	}

	if len(e.Data) > 0 {
		w.writeMap(e.Data)
	}

	// write caller last if enabled
	if tf.callerEnabled && caller != "" {
		w.writeField(caller, noColor)
	}

	w.WriteByte('\n')
	data = w.Bytes()
	return data, nil
}

// JSONFormatter implements the logrus.Formatter interface and adds extra
// fields to log entries
type JSONFormatter struct {
	log.JSONFormatter

	ExtraFields []string

	callerEnabled    bool
	componentEnabled bool
}

// CheckAndSetDefaults checks and sets log format configuration
func (j *JSONFormatter) CheckAndSetDefaults() error {
	// set log formatting
	if j.ExtraFields == nil {
		j.ExtraFields = KnownFormatFields.names()
	}

	// parse input
	res, err := parseInputFormat(j.ExtraFields)
	if err != nil {
		return trace.Wrap(err)
	}

	if slices.Contains(res, timestampField) {
		j.JSONFormatter.DisableTimestamp = true
	}

	if slices.Contains(res, callerField) {
		j.callerEnabled = true
	}

	if slices.Contains(res, componentField) {
		j.componentEnabled = true
	}

	// rename default fields
	j.JSONFormatter = log.JSONFormatter{
		FieldMap: log.FieldMap{
			log.FieldKeyTime:  timestampField,
			log.FieldKeyLevel: levelField,
			log.FieldKeyMsg:   messageField,
		},
	}

	return nil
}

// Format implements logrus.Formatter interface
func (j *JSONFormatter) Format(e *log.Entry) ([]byte, error) {
	if j.callerEnabled {
		path := formatCallerWithPathAndLine()
		e.Data[callerField] = path
	}

	if j.componentEnabled {
		e.Data[componentField] = e.Data[trace.Component]
	}

	delete(e.Data, trace.Component)

	return j.JSONFormatter.Format(e)
}

func NewTestJSONFormatter() *JSONFormatter {
	formatter := &JSONFormatter{}
	if err := formatter.CheckAndSetDefaults(); err != nil {
		panic(err)
	}
	return formatter
}

func (w *writer) writeError(value interface{}) {
	switch err := value.(type) {
	case trace.Error:
		w.WriteString(fmt.Sprintf("[%v]", err.DebugReport()))
	default:
		w.WriteString(fmt.Sprintf("[%v]", value))
	}
}

func padMax(in string, chars int) string {
	switch {
	case len(in) < chars:
		return in + strings.Repeat(" ", chars-len(in))
	default:
		return in[:chars]
	}
}

func (w *writer) writeField(value interface{}, color int) {
	if w.Len() > 0 {
		w.WriteByte(' ')
	}
	w.writeValue(value, color)
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

func (w *writer) writeMap(m map[string]any) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == trace.Component {
			continue
		}
		switch value := m[key].(type) {
		case map[string]any:
			w.writeMap(value)
		case log.Fields:
			w.writeMap(value)
		default:
			w.writeKeyValue(key, value)
		}
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

// formatCallerWithPathAndLine formats the caller in the form path/segment:<line number>
// for output in the log
func formatCallerWithPathAndLine() (path string) {
	if cursor := findFrame(); cursor != nil {
		t := newTraceFromFrames(*cursor, nil)
		return t.Loc()
	}
	return ""
}

var frameIgnorePattern = regexp.MustCompile(`github\.com/sirupsen/logrus`)

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
		if !frameIgnorePattern.MatchString(frame.Function) {
			return &frameCursor{
				current: &frame,
				rest:    frames,
				n:       n,
			}
		}
	}
	return nil
}

func newTraceFromFrames(cursor frameCursor, err error) *trace.TraceErr {
	traces := make(trace.Traces, 0, cursor.n)
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
	return &trace.TraceErr{
		Err:    err,
		Traces: traces,
	}
}

func frameToTrace(frame runtime.Frame) trace.Trace {
	return trace.Trace{
		Func: frame.Function,
		Path: frame.File,
		Line: frame.Line,
	}
}

func (r knownFormatFieldsMap) has(name string) bool {
	_, ok := r[name]
	return ok
}

func (r knownFormatFieldsMap) names() (result []string) {
	for k := range r {
		result = append(result, k)
	}
	return result
}

type knownFormatFieldsMap map[string]struct{}

// KnownFormatFields are the known fields for log entries
var KnownFormatFields = knownFormatFieldsMap{
	levelField:     {},
	componentField: {},
	callerField:    {},
	timestampField: {},
}

func parseInputFormat(formatInput []string) (result []string, err error) {
	for _, component := range formatInput {
		component = strings.TrimSpace(component)
		if !KnownFormatFields.has(component) {
			return nil, trace.BadParameter("invalid log format key: %q", component)
		}
		result = append(result, component)
	}
	return result, nil
}
