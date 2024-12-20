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
	"log/slog"
	"reflect"
	"strings"

	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	// TraceLevel is the logging level when set to Trace verbosity.
	TraceLevel = slog.LevelDebug - 1

	// TraceLevelText is the text representation of Trace verbosity.
	TraceLevelText = "TRACE"
)

// DiscardHandler is a [slog.Handler] that discards all messages. It
// is more efficient than a [slog.Handler] which outputs to [io.Discard] since
// it performs zero formatting.
// TODO(tross): Use slog.DiscardHandler once upgraded to Go 1.24.
type DiscardHandler struct{}

func (dh DiscardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (dh DiscardHandler) Handle(context.Context, slog.Record) error { return nil }
func (dh DiscardHandler) WithAttrs(attrs []slog.Attr) slog.Handler  { return dh }
func (dh DiscardHandler) WithGroup(name string) slog.Handler        { return dh }

func addTracingContextToRecord(ctx context.Context, r *slog.Record) {
	const (
		traceID = "trace_id"
		spanID  = "span_id"
	)

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

// getCaller retrieves source information from the attribute
// and returns the file and line of the caller. The file is
// truncated from the absolute path to package/filename.
func getCaller(s *slog.Source) (file string, line int) {
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
	if s.Stringer == nil {
		return slog.StringValue("")
	}
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
