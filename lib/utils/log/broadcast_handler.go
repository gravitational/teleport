// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package log

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
)

// BroadcastHandler is a slog.Handler that wraps an inner handler and
// broadcasts structured log records to subscribers as *debugpb.LogEntry.
// When no subscribers are active, the overhead is a single atomic load
// in Enabled.
type BroadcastHandler struct {
	inner       slog.Handler
	broadcaster *LogBroadcaster
	preAttrs    []groupedAttr
	groups      []string
}

// groupedAttr stores an attribute together with the group path that was
// active when WithAttrs was called.
type groupedAttr struct {
	groups []string
	attr   slog.Attr
}

// NewBroadcastHandler wraps inner with broadcast capability.
func NewBroadcastHandler(inner slog.Handler, broadcaster *LogBroadcaster) *BroadcastHandler {
	return &BroadcastHandler{
		inner:       inner,
		broadcaster: broadcaster,
	}
}

// Enabled returns true if either the inner handler or any subscriber
// wants records at this level.
func (h *BroadcastHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.inner.Enabled(ctx, level) {
		return true
	}
	return level >= h.broadcaster.MinLevel()
}

// Handle forwards the record to the inner handler (if enabled) and
// builds a *debugpb.LogEntry for broadcast to subscribers.
func (h *BroadcastHandler) Handle(ctx context.Context, r slog.Record) error {
	var innerErr error
	if h.inner.Enabled(ctx, r.Level) {
		innerErr = h.inner.Handle(ctx, r)
	}

	if r.Level >= h.broadcaster.MinLevel() {
		entry := h.toLogEntry(ctx, r)
		h.broadcaster.Broadcast(entry, r.Level)
	}

	return innerErr
}

// WithAttrs returns a new BroadcastHandler with the given attrs appended,
// tagged with the current group path.
func (h *BroadcastHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	groupsCopy := make([]string, len(h.groups))
	copy(groupsCopy, h.groups)

	newPreAttrs := make([]groupedAttr, len(h.preAttrs), len(h.preAttrs)+len(attrs))
	copy(newPreAttrs, h.preAttrs)
	for _, a := range attrs {
		newPreAttrs = append(newPreAttrs, groupedAttr{
			groups: groupsCopy,
			attr:   a,
		})
	}

	return &BroadcastHandler{
		inner:       h.inner.WithAttrs(attrs),
		broadcaster: h.broadcaster,
		preAttrs:    newPreAttrs,
		groups:      groupsCopy,
	}
}

// WithGroup returns a new BroadcastHandler with the given group appended
// to the group path.
func (h *BroadcastHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &BroadcastHandler{
		inner:       h.inner.WithGroup(name),
		broadcaster: h.broadcaster,
		preAttrs:    h.preAttrs,
		groups:      newGroups,
	}
}

func (h *BroadcastHandler) toLogEntry(ctx context.Context, r slog.Record) *debugpb.LogEntry {
	entry := &debugpb.LogEntry{
		Timestamp: timestamppb.New(r.Time.UTC()),
		Level:     levelToString(r.Level),
		Message:   r.Message,
	}

	if r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		src := &slog.Source{Function: f.Function, File: f.File, Line: f.Line}
		file, line := getCaller(src)
		entry.Caller = fmt.Sprintf("%s:%d", file, line)
	}

	span := oteltrace.SpanFromContext(ctx)
	if span != nil {
		sc := span.SpanContext()
		if sc.HasTraceID() {
			entry.TraceId = sc.TraceID().String()
		}
		if sc.HasSpanID() {
			entry.SpanId = sc.SpanID().String()
		}
	}

	attrs := make(map[string]string)
	for _, ga := range h.preAttrs {
		flattenAttr(entry, attrs, ga.groups, ga.attr)
	}
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr(entry, attrs, h.groups, a)
		return true
	})
	if len(attrs) > 0 {
		entry.Attributes = attrs
	}

	return entry
}

// levelToString converts slog.Level to the Teleport-standard string.
func levelToString(level slog.Level) string {
	switch level {
	case TraceLevel:
		return TraceLevelText
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return level.String()
	}
}

// flattenAttr flattens an slog.Attr into dot-separated string keys in the
// attributes map. The component key is handled specially by setting the
// Component field on the LogEntry directly.
func flattenAttr(entry *debugpb.LogEntry, m map[string]string, groups []string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}

	key := a.Key
	if key == teleport.ComponentKey {
		entry.Component = fmt.Sprintf("%v", resolveValue(a.Value))
		return
	}

	if a.Value.Kind() == slog.KindGroup {
		subAttrs := a.Value.Group()
		subGroups := groups
		if a.Key != "" {
			subGroups = append(append([]string{}, groups...), a.Key)
		}
		for _, ga := range subAttrs {
			flattenAttr(entry, m, subGroups, ga)
		}
		return
	}

	prefix := ""
	if len(groups) > 0 {
		prefix = strings.Join(groups, ".") + "."
	}
	m[prefix+key] = fmt.Sprintf("%v", resolveValue(a.Value))
}

// resolveValue converts an slog.Value to a JSON-friendly Go value.
func resolveValue(v slog.Value) any {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindBool:
		return v.Bool()
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().UTC().Format(time.RFC3339Nano)
	case slog.KindAny:
		switch val := v.Any().(type) {
		case error:
			return val.Error()
		case fmt.Stringer:
			return val.String()
		default:
			return fmt.Sprintf("%+v", val)
		}
	default:
		return v.String()
	}
}

var _ slog.Handler = (*BroadcastHandler)(nil)
