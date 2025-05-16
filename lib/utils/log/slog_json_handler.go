// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/teleport"
)

// SlogJSONHandlerConfig allows the SlogJSONHandler functionality
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
	withCaller := len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, CallerField)
	withComponent := len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, ComponentField)
	withTimestamp := len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, TimestampField)

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
					if a.Value.Kind() != slog.KindString {
						return a
					}

					a.Key = ComponentField
				case slog.LevelKey:
					// The slog.JSONHandler will inject "level" Attr.
					// However, this lib's consumer might add an Attr using the same key ("level") and we end up with two records named "level".
					// We must check its type before assuming this was injected by the slog.JSONHandler.
					lvl, ok := a.Value.Any().(slog.Level)
					if !ok {
						return a
					}

					var level string
					switch lvl {
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

					// The slog.JSONHandler will inject "time" Attr.
					// However, this lib's consumer might add an Attr using the same key ("time") and we end up with two records named "time".
					// We must check its type before assuming this was injected by the slog.JSONHandler.
					if a.Value.Kind() != slog.KindTime {
						return a
					}

					t := a.Value.Time()
					if t.IsZero() {
						return a
					}

					a.Key = TimestampField
					a.Value = slog.StringValue(t.Format(time.RFC3339))
				case slog.MessageKey:
					// The slog.JSONHandler will inject "msg" Attr.
					// However, this lib's consumer might add an Attr using the same key ("msg") and we end up with two records named "msg".
					// We must check its type before assuming this was injected by the slog.JSONHandler.
					if a.Value.Kind() != slog.KindString {
						return a
					}
					a.Key = messageField
				case slog.SourceKey:
					if !withCaller {
						return slog.Attr{}
					}

					// The slog.JSONHandler will inject "source" Attr when AddSource is true.
					// However, this lib's consumer might add an Attr using the same key ("source") and we end up with two records named "source".
					// We must check its type before assuming this was injected by the slog.JSONHandler.
					s, ok := a.Value.Any().(*slog.Source)
					if !ok {
						return a
					}

					file, line := getCaller(s)
					a = slog.String(CallerField, fmt.Sprintf("%s:%d", file, line))
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

func (j *SlogJSONHandler) Handle(ctx context.Context, r slog.Record) error {
	addTracingContextToRecord(ctx, &r)
	return j.JSONHandler.Handle(ctx, r)
}
