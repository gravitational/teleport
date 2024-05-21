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
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport"
)

// TraceLevel is the logging level when set to Trace verbosity.
const TraceLevel = slog.LevelDebug - 1

// SlogTextHandler is a [slog.Handler] that outputs messages in a textual
// manner as configured by the Teleport configuration.
type SlogTextHandler struct {
	cfg SlogTextHandlerConfig
	// withCaller indicates whether the location the log was emitted from
	// should be included in the output message.
	withCaller bool
	// withTimestamp indicates whether the times that the log was emitted at
	// should be included in the output message.
	withTimestamp bool
	// component is the Teleport subcomponent that emitted the log.
	component string
	// preformatted data from previous calls to WithGroup and WithAttrs.
	preformatted []byte
	// groupPrefix is for the text handler only.
	// It holds the prefix for groups that were already pre-formatted.
	// A group will appear here when a call to WithGroup is followed by
	// a call to WithAttrs.
	groupPrefix buffer
	// groups passed in via WithGroup and WithAttrs.
	groups []string
	// nOpenGroups the number of groups opened in preformatted.
	nOpenGroups int

	// mu protects out - it needs to be a pointer so that all cloned
	// SlogTextHandler returned from WithAttrs and WithGroup share the
	// same mutex. Otherwise, output may be garbled since each clone
	// will use its own copy of the mutex to protect out. See
	// https://github.com/golang/go/issues/61321 for more details.
	mu  *sync.Mutex
	out io.Writer
}

// SlogTextHandlerConfig allow the SlogTextHandler functionality
// to be tweaked.
type SlogTextHandlerConfig struct {
	// Level is the minimum record level that will be logged.
	Level slog.Leveler
	// EnableColors allows the level to be printed in color.
	EnableColors bool
	// Padding to use for various components.
	Padding int
	// ConfiguredFields are fields explicitly set by users to be included in
	// the output message. If there are any entries configured, they will be honored.
	// If empty, the default fields will be populated and included in the output.
	ConfiguredFields []string
	// ReplaceAttr is called to rewrite each non-group attribute before
	// it is logged.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

// NewSlogTextHandler creates a SlogTextHandler that writes messages to w.
func NewSlogTextHandler(w io.Writer, cfg SlogTextHandlerConfig) *SlogTextHandler {
	if cfg.Padding == 0 {
		cfg.Padding = defaultComponentPadding
	}

	handler := SlogTextHandler{
		cfg:           cfg,
		withCaller:    len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, callerField),
		withTimestamp: len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, timestampField),
		out:           w,
		mu:            &sync.Mutex{},
	}

	if handler.cfg.ConfiguredFields == nil {
		handler.cfg.ConfiguredFields = defaultFormatFields
	}

	return &handler
}

// Enabled returns whether the provided level will be included in output.
func (s *SlogTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if s.cfg.Level != nil {
		minLevel = s.cfg.Level.Level()
	}
	return level >= minLevel
}

func (s *SlogTextHandler) appendAttr(buf []byte, a slog.Attr) []byte {
	if rep := s.cfg.ReplaceAttr; rep != nil && a.Value.Kind() != slog.KindGroup {
		var gs []string
		if s.groups != nil {
			gs = s.groups
		}
		// Resolve before calling ReplaceAttr, so the user doesn't have to.
		a.Value = a.Value.Resolve()
		a = rep(gs, a)
	}

	// Resolve the Attr's value before doing anything else.
	a.Value = a.Value.Resolve()
	// Ignore empty Attrs.
	if a.Equal(slog.Attr{}) {
		return buf
	}

	switch a.Value.Kind() {
	case slog.KindString:
		value := a.Value.String()
		if a.Key == slog.TimeKey {
			buf = fmt.Append(buf, value)
			break
		}

		if a.Key == teleport.ComponentFields {
			switch fields := a.Value.Any().(type) {
			case map[string]any:
				for k, v := range fields {
					buf = s.appendAttr(buf, slog.Any(k, v))
				}
			case logrus.Fields:
				for k, v := range fields {
					buf = s.appendAttr(buf, slog.Any(k, v))
				}
			}
		}

		if needsQuoting(value) {
			if a.Key == teleport.ComponentKey || a.Key == slog.LevelKey || a.Key == callerField || a.Key == slog.MessageKey {
				if len(buf) > 0 {
					buf = fmt.Append(buf, " ")
				}
			} else {
				if len(buf) > 0 {
					buf = fmt.Append(buf, " ")
				}
				buf = fmt.Appendf(buf, "%s%s:", s.groupPrefix, a.Key)
			}
			buf = strconv.AppendQuote(buf, value)
			break
		}

		if a.Key == teleport.ComponentKey || a.Key == slog.LevelKey || a.Key == callerField || a.Key == slog.MessageKey {
			if len(buf) > 0 {
				buf = fmt.Append(buf, " ")
			}
			buf = fmt.Appendf(buf, "%s", a.Value.String())
			break
		}

		buf = fmt.Appendf(buf, " %s%s:%s", s.groupPrefix, a.Key, a.Value.String())
	case slog.KindGroup:
		attrs := a.Value.Group()
		// Ignore empty groups.
		if len(attrs) == 0 {
			return buf
		}
		// If the key is non-empty, write it out and indent the rest of the attrs.
		// Otherwise, inline the attrs.
		if a.Key != "" {
			s.groupPrefix = fmt.Append(s.groupPrefix, a.Key)
			s.groupPrefix = fmt.Append(s.groupPrefix, ".")
		}
		for _, ga := range attrs {
			buf = s.appendAttr(buf, ga)
		}
		if a.Key != "" {
			s.groupPrefix = s.groupPrefix[:len(s.groupPrefix)-len(a.Key)-1 /* for keyComponentSep */]
			if s.groups != nil {
				s.groups = (s.groups)[:len(s.groups)-1]
			}
		}
	default:
		switch err := a.Value.Any().(type) {
		case trace.Error:
			buf = fmt.Appendf(buf, " error:[%v]", err.DebugReport())
		case error:
			buf = fmt.Appendf(buf, " error:[%v]", a.Value)
		default:
			buf = fmt.Appendf(buf, " %s:%s", a.Key, a.Value)
		}
	}
	return buf
}

// writeTimeRFC3339 writes the time in [time.RFC3339Nano] to the buffer.
// This takes half the time of [time.Time.AppendFormat]. Adapted from
// go/src/log/slog/handler.go.
func writeTimeRFC3339(buf *buffer, t time.Time) {
	year, month, day := t.Date()
	buf.WritePosIntWidth(year, 4)
	buf.WriteByte('-')
	buf.WritePosIntWidth(int(month), 2)
	buf.WriteByte('-')
	buf.WritePosIntWidth(day, 2)
	buf.WriteByte('T')
	hour, min, sec := t.Clock()
	buf.WritePosIntWidth(hour, 2)
	buf.WriteByte(':')
	buf.WritePosIntWidth(min, 2)
	buf.WriteByte(':')
	buf.WritePosIntWidth(sec, 2)
	_, offsetSeconds := t.Zone()
	if offsetSeconds == 0 {
		buf.WriteByte('Z')
	} else {
		offsetMinutes := offsetSeconds / 60
		if offsetMinutes < 0 {
			buf.WriteByte('-')
			offsetMinutes = -offsetMinutes
		} else {
			buf.WriteByte('+')
		}
		buf.WritePosIntWidth(offsetMinutes/60, 2)
		buf.WriteByte(':')
		buf.WritePosIntWidth(offsetMinutes%60, 2)
	}
}

// Handle formats the provided record and writes the output to the
// destination.
func (s *SlogTextHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := newBuffer()
	defer buf.Free()

	addTracingContextToRecord(ctx, &r)

	if s.withTimestamp && !r.Time.IsZero() {
		if s.cfg.ReplaceAttr != nil {
			*buf = s.appendAttr(*buf, slog.Time(slog.TimeKey, r.Time))
		} else {
			writeTimeRFC3339(buf, r.Time)
		}
	}

	// Processing fields in this manner allows users to
	// configure the level and component position in the output.
	// This matches the behavior of the original logrus. All other
	// fields location in the output message are static.
	for _, field := range s.cfg.ConfiguredFields {
		switch field {
		case levelField:
			var color int
			var level string
			switch r.Level {
			case TraceLevel:
				level = "TRACE"
				color = gray
			case slog.LevelDebug:
				level = "DEBUG"
				color = gray
			case slog.LevelInfo:
				level = "INFO"
				color = blue
			case slog.LevelWarn:
				level = "WARN"
				color = yellow
			case slog.LevelError:
				level = "ERROR"
				color = red
			case slog.LevelError + 1:
				level = "FATAL"
				color = red
			default:
				color = blue
				level = r.Level.String()
			}

			if !s.cfg.EnableColors {
				color = noColor
			}

			level = padMax(level, defaultLevelPadding)
			if color == noColor {
				*buf = s.appendAttr(*buf, slog.String(slog.LevelKey, level))
			} else {
				*buf = fmt.Appendf(*buf, " \u001B[%dm%s\u001B[0m", color, level)
			}
		case componentField:
			// If a component is provided with the attributes, it should be used instead of
			// the component set on the handler. Note that if there are multiple components
			// specified in the arguments, the one with the lowest index is used and the others are ignored.
			// In the example below, the resulting component in the message output would be "alpaca".
			//
			//	logger := logger.With(teleport.ComponentKey, "fish")
			//	logger.InfoContext(ctx, "llama llama llama", teleport.ComponentKey, "alpaca", "foo", 123, teleport.ComponentKey, "shark")
			component := s.component
			r.Attrs(func(attr slog.Attr) bool {
				if attr.Key == teleport.ComponentKey {
					component = fmt.Sprintf("[%v]", attr.Value)
					component = strings.ToUpper(padMax(component, s.cfg.Padding))
					if component[len(component)-1] != ' ' {
						component = component[:len(component)-1] + "]"
					}

					return false
				}

				return true
			})

			*buf = s.appendAttr(*buf, slog.String(teleport.ComponentKey, component))
		default:
			if _, ok := knownFormatFields[field]; !ok {
				return trace.BadParameter("invalid log format key: %v", field)
			}
		}
	}

	*buf = s.appendAttr(*buf, slog.String(slog.MessageKey, r.Message))

	// Insert preformatted attributes just after built-in ones.
	*buf = append(*buf, s.preformatted...)
	if r.NumAttrs() > 0 {
		if len(s.groups) > 0 {
			for _, n := range s.groups[s.nOpenGroups:] {
				s.groupPrefix = fmt.Append(s.groupPrefix, n)
				s.groupPrefix = fmt.Append(s.groupPrefix, ".")
			}
		}

		r.Attrs(func(a slog.Attr) bool {
			// Skip adding any component attrs since they are processed above.
			if a.Key == teleport.ComponentKey {
				return true
			}

			*buf = s.appendAttr(*buf, a)
			return true
		})
	}

	if r.PC != 0 && s.withCaller {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()

		src := &slog.Source{
			Function: f.Function,
			File:     f.File,
			Line:     f.Line,
		}

		file, line := getCaller(slog.Attr{Key: slog.SourceKey, Value: slog.AnyValue(src)})
		*buf = fmt.Appendf(*buf, " %s:%d", file, line)
	}

	buf.WriteByte('\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.out.Write(*buf)
	return err
}

// WithAttrs clones the current handler with the provided attributes
// added to any existing attributes. The values are preformatted here
// so that they do not need to be formatted per call to Handle.
func (s *SlogTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return s
	}
	s2 := *s
	// Force an append to copy the underlying arrays.
	s2.preformatted = slices.Clip(s.preformatted)
	s2.groups = slices.Clip(s.groups)

	// Add all groups from WithGroup that haven't already been added to the prefix.
	if len(s.groups) > 0 {
		for _, n := range s.groups[s.nOpenGroups:] {
			s2.groupPrefix = fmt.Append(s2.groupPrefix, n)
			s2.groupPrefix = fmt.Append(s2.groupPrefix, ".")
		}
	}

	// Now all groups have been opened.
	s2.nOpenGroups = len(s2.groups)

	component := s.component

	// Pre-format the attributes.
	for _, a := range attrs {
		switch a.Key {
		case teleport.ComponentKey:
			component = fmt.Sprintf("[%v]", a.Value.String())
			component = strings.ToUpper(padMax(component, s.cfg.Padding))
			if component[len(component)-1] != ' ' {
				component = component[:len(component)-1] + "]"
			}
		case teleport.ComponentFields:
			switch fields := a.Value.Any().(type) {
			case map[string]any:
				for k, v := range fields {
					s2.appendAttr(s2.preformatted, slog.Any(k, v))
				}
			case logrus.Fields:
				for k, v := range fields {
					s2.preformatted = s2.appendAttr(s2.preformatted, slog.Any(k, v))
				}
			}
		default:
			s2.preformatted = s2.appendAttr(s2.preformatted, a)
		}
	}

	s2.component = component
	// Remember how many opened groups are in preformattedAttrs,
	// so we don't open them again when we handle a Record.
	s2.nOpenGroups = len(s2.groups)
	return &s2
}

// WithGroup opens a new group.
func (s *SlogTextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return s
	}

	s2 := *s
	s2.groups = append(s2.groups, name)
	return &s2
}

// SlogJSONHandlerConfig allow the SlogJSONHandler functionality
// to be tweaked.
type SlogJSONHandlerConfig struct {
	// Level is the minimum record level that will be logged.
	Level slog.Leveler
	// ConfiguredFields are fields explicitly set by users to be included in
	// the output message. If there are any entries configured, they will be honored.
	// If empty, the default fields will be populated and included in the output.
	ConfiguredFields []string
	// ReplaceAttr is called to rewrite each non-group attribute before
	// it is logged.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

// SlogJSONHandler is a [slog.Handler] that outputs messages in a json
// format per the config file.
type SlogJSONHandler struct {
	*slog.JSONHandler
}

// NewSlogJSONHandler creates a SlogJSONHandler that outputs to w.
func NewSlogJSONHandler(w io.Writer, cfg SlogJSONHandlerConfig) *SlogJSONHandler {
	withCaller := len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, callerField)
	withComponent := len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, componentField)
	withTimestamp := len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, timestampField)

	return &SlogJSONHandler{
		JSONHandler: slog.NewJSONHandler(w, &slog.HandlerOptions{
			AddSource: true,
			Level:     cfg.Level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				switch a.Key {
				case teleport.ComponentKey:
					if !withComponent {
						return slog.Attr{}
					}

					a.Key = componentField
				case slog.LevelKey:
					var level string
					switch lvl := a.Value.Any().(slog.Level); lvl {
					case TraceLevel:
						level = "trace"
					case slog.LevelDebug:
						level = "debug"
					case slog.LevelInfo:
						level = "info"
					case slog.LevelWarn:
						level = "warning"
					case slog.LevelError:
						level = "error"
					case slog.LevelError + 1:
						level = "fatal"
					default:
						level = strings.ToLower(lvl.String())
					}

					a.Value = slog.StringValue(level)
				case slog.TimeKey:
					if !withTimestamp {
						return slog.Attr{}
					}

					t := a.Value.Time()
					if t.IsZero() {
						return a
					}

					a.Key = timestampField
					a.Value = slog.StringValue(t.Format(time.RFC3339))
				case slog.MessageKey:
					a.Key = messageField
				case slog.SourceKey:
					if !withCaller {
						return slog.Attr{}
					}

					file, line := getCaller(a)
					a = slog.String(callerField, fmt.Sprintf("%s:%d", file, line))
				}

				// Convert [slog.KindAny] values that are backed by an [error] or [fmt.Stringer]
				// to strings so that only the message is output instead of a json object. The kind is
				// first checked to avoid allocating an interface for the values stored inline
				// in [slog.Attr].
				if a.Value.Kind() == slog.KindAny {
					if err, ok := a.Value.Any().(error); ok {
						a.Value = slog.StringValue(err.Error())
					}

					if stringer, ok := a.Value.Any().(fmt.Stringer); ok {
						a.Value = slog.StringValue(stringer.String())
					}
				}

				return a
			},
		}),
	}
}

const (
	traceID = "trace_id"
	spanID  = "span_id"
)

func addTracingContextToRecord(ctx context.Context, r *slog.Record) {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil {
		return
	}

	spanContext := span.SpanContext()
	if spanContext.HasTraceID() {
		r.AddAttrs(slog.String(traceID, spanContext.TraceID().String()))
	}

	if spanContext.HasSpanID() {
		r.AddAttrs(slog.String(spanID, spanContext.SpanID().String()))
	}
}

func (j *SlogJSONHandler) Handle(ctx context.Context, r slog.Record) error {
	addTracingContextToRecord(ctx, &r)
	return j.JSONHandler.Handle(ctx, r)
}

// getCaller retrieves source information from the attribute
// and returns the file and line of the caller. The file is
// truncated from the absolute path to package/filename.
func getCaller(a slog.Attr) (file string, line int) {
	s := a.Value.Any().(*slog.Source)
	count := 0
	idx := strings.LastIndexFunc(s.File, func(r rune) bool {
		if r == '/' {
			count++
		}

		return count == 2
	})
	file = s.File[idx+1:]
	line = s.Line

	return file, line
}

type stringerAttr struct {
	fmt.Stringer
}

// StringerAttr creates a [slog.LogValuer] that will defer to
// the provided [fmt.Stringer]. All slog attributes are always evaluated,
// even if the log event is discarded due to the configured log level.
// A text [slog.Handler] will try to defer evaluation if the attribute is a
// [fmt.Stringer], however, the JSON [slog.Handler] only defers to [json.Marshaler].
// This means that to defer evaluation and creation of the string representation,
// the object must implement [fmt.Stringer] and [json.Marshaler], otherwise additional
// and unwanted values may be emitted if the logger is configured to use JSON
// instead of text. This wrapping mechanism allows a value that implements [fmt.Stringer],
// to be guaranteed to be lazily constructed and always output the same
// content regardless of the output format.
func StringerAttr(s fmt.Stringer) slog.LogValuer {
	return stringerAttr{Stringer: s}
}

func (s stringerAttr) LogValue() slog.Value {
	return slog.StringValue(s.Stringer.String())
}

type typeAttr struct {
	val any
}

// TypeAttr creates a lazily evaluated log value that presents the pretty type name of a value
// as a string. It is roughly equivalent to the '%T' format option, and should only perform
// reflection in the event that logs are actually being generated.
func TypeAttr(val any) slog.LogValuer {
	return typeAttr{val}
}

func (a typeAttr) LogValue() slog.Value {
	if t := reflect.TypeOf(a.val); t != nil {
		return slog.StringValue(t.String())
	}
	return slog.StringValue("nil")
}
