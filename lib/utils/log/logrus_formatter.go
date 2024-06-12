/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package log

import (
	"fmt"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
)

// TextFormatter is a [logrus.Formatter] that outputs messages in
// a textual format.
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
	b *buffer
}

func newWriter() *writer {
	return &writer{b: &buffer{}}
}

func (w *writer) Len() int {
	return len(*w.b)
}

func (w *writer) WriteString(s string) (int, error) {
	return w.b.WriteString(s)
}

func (w *writer) WriteByte(c byte) error {
	return w.b.WriteByte(c)
}

func (w *writer) Bytes() []byte {
	return *w.b
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
	// defaultComponentPadding is a default padding for component field
	defaultComponentPadding = 11
	// defaultLevelPadding is a default padding for level field
	defaultLevelPadding = 4
)

// NewDefaultTextFormatter creates a TextFormatter with
// the default options set.
func NewDefaultTextFormatter(enableColors bool) *TextFormatter {
	return &TextFormatter{
		ComponentPadding: defaultComponentPadding,
		FormatCaller:     formatCallerWithPathAndLine,
		ExtraFields:      defaultFormatFields,
		EnableColors:     enableColors,
		callerEnabled:    true,
		timestampEnabled: false,
	}
}

// CheckAndSetDefaults checks and sets log format configuration.
func (tf *TextFormatter) CheckAndSetDefaults() error {
	// set padding
	if tf.ComponentPadding == 0 {
		tf.ComponentPadding = defaultComponentPadding
	}
	// set caller
	tf.FormatCaller = formatCallerWithPathAndLine

	// set log formatting
	if tf.ExtraFields == nil {
		tf.timestampEnabled = true
		tf.callerEnabled = true
		tf.ExtraFields = defaultFormatFields
		return nil
	}

	if slices.Contains(tf.ExtraFields, timestampField) {
		tf.timestampEnabled = true
	}

	if slices.Contains(tf.ExtraFields, callerField) {
		tf.callerEnabled = true
	}

	return nil
}

// Format formats each log line as configured in teleport config file.
func (tf *TextFormatter) Format(e *logrus.Entry) ([]byte, error) {
	caller := tf.FormatCaller()
	w := newWriter()

	// write timestamp first if enabled
	if tf.timestampEnabled {
		writeTimeRFC3339(w.b, e.Time)
	}

	for _, field := range tf.ExtraFields {
		switch field {
		case levelField:
			var color int
			var level string
			switch e.Level {
			case logrus.TraceLevel:
				level = "TRACE"
				color = gray
			case logrus.DebugLevel:
				level = "DEBUG"
				color = gray
			case logrus.InfoLevel:
				level = "INFO"
				color = blue
			case logrus.WarnLevel:
				level = "WARN"
				color = yellow
			case logrus.ErrorLevel:
				level = "ERROR"
				color = red
			case logrus.FatalLevel:
				level = "FATAL"
				color = red
			default:
				color = blue
				level = strings.ToUpper(e.Level.String())
			}

			if !tf.EnableColors {
				color = noColor
			}

			w.writeField(padMax(level, defaultLevelPadding), color)
		case componentField:
			padding := defaultComponentPadding
			if tf.ComponentPadding != 0 {
				padding = tf.ComponentPadding
			}
			if w.Len() > 0 {
				w.WriteByte(' ')
			}
			component, ok := e.Data[teleport.ComponentKey].(string)
			if ok && component != "" {
				component = fmt.Sprintf("[%v]", component)
			}
			component = strings.ToUpper(padMax(component, padding))
			if component[len(component)-1] != ' ' {
				component = component[:len(component)-1] + "]"
			}

			w.WriteString(component)
		default:
			if _, ok := knownFormatFields[field]; !ok {
				return nil, trace.BadParameter("invalid log format key: %v", field)
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
	return w.Bytes(), nil
}

// JSONFormatter implements the [logrus.Formatter] interface and adds extra
// fields to log entries.
type JSONFormatter struct {
	logrus.JSONFormatter

	ExtraFields []string
	// FormatCaller is a function to return (part) of source file path for output.
	// Defaults to filePathAndLine() if unspecified
	FormatCaller func() (caller string)

	callerEnabled    bool
	componentEnabled bool
}

// CheckAndSetDefaults checks and sets log format configuration.
func (j *JSONFormatter) CheckAndSetDefaults() error {
	// set log formatting
	if j.ExtraFields == nil {
		j.ExtraFields = defaultFormatFields
	}
	// set caller
	j.FormatCaller = formatCallerWithPathAndLine

	if slices.Contains(j.ExtraFields, callerField) {
		j.callerEnabled = true
	}

	if slices.Contains(j.ExtraFields, componentField) {
		j.componentEnabled = true
	}

	// rename default fields
	j.JSONFormatter = logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  timestampField,
			logrus.FieldKeyLevel: levelField,
			logrus.FieldKeyMsg:   messageField,
		},
		DisableTimestamp: !slices.Contains(j.ExtraFields, timestampField),
	}

	return nil
}

// Format formats each log line as configured in teleport config file.
func (j *JSONFormatter) Format(e *logrus.Entry) ([]byte, error) {
	if j.callerEnabled {
		path := j.FormatCaller()
		e.Data[callerField] = path
	}

	if j.componentEnabled {
		e.Data[componentField] = e.Data[teleport.ComponentKey]
	}

	delete(e.Data, teleport.ComponentKey)

	return j.JSONFormatter.Format(e)
}

// NewTestJSONFormatter creates a JSONFormatter that is
// configured for output in tests.
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
		*w.b = fmt.Appendf(*w.b, "[%v]", err.DebugReport())
	default:
		*w.b = fmt.Appendf(*w.b, "[%v]", value)
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
	if key == logrus.ErrorKey {
		w.writeError(value)
		return
	}
	w.writeValue(value, noColor)
}

func (w *writer) writeValue(value interface{}, color int) {
	if s, ok := value.(string); ok {
		if color != noColor {
			*w.b = fmt.Appendf(*w.b, "\u001B[%dm", color)
		}

		if needsQuoting(s) {
			*w.b = strconv.AppendQuote(*w.b, s)
		} else {
			*w.b = fmt.Append(*w.b, s)
		}

		if color != noColor {
			*w.b = fmt.Append(*w.b, "\u001B[0m")
		}
		return
	}

	if color != noColor {
		*w.b = fmt.Appendf(*w.b, "\x1b[%dm%v\x1b[0m", color, value)
		return
	}

	*w.b = fmt.Appendf(*w.b, "%v", value)
}

func (w *writer) writeMap(m map[string]any) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if key == teleport.ComponentKey {
			continue
		}
		switch value := m[key].(type) {
		case map[string]any:
			w.writeMap(value)
		case logrus.Fields:
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

var defaultFormatFields = []string{levelField, componentField, callerField, timestampField}

var knownFormatFields = map[string]struct{}{
	levelField:     {},
	componentField: {},
	callerField:    {},
	timestampField: {},
}

func ValidateFields(formatInput []string) (result []string, err error) {
	for _, component := range formatInput {
		component = strings.TrimSpace(component)
		if _, ok := knownFormatFields[component]; !ok {
			return nil, trace.BadParameter("invalid log format key: %q", component)
		}
		result = append(result, component)
	}
	return result, nil
}

// needsQuoting returns true if any non-printable characters are found.
func needsQuoting(text string) bool {
	for _, r := range text {
		if !unicode.IsPrint(r) {
			return true
		}
	}
	return false
}
