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
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

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
	groupPrefix string
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
		withCaller:    len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, CallerField),
		withTimestamp: len(cfg.ConfiguredFields) == 0 || slices.Contains(cfg.ConfiguredFields, TimestampField),
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

func (s *SlogTextHandler) newHandleState(buf *buffer, freeBuf bool) handleState {
	state := handleState{
		h:       s,
		buf:     buf,
		freeBuf: freeBuf,
		prefix:  newBuffer(),
	}
	if s.cfg.ReplaceAttr != nil {
		state.groups = groupPool.Get().(*[]string)
		*state.groups = append(*state.groups, s.groups[:s.nOpenGroups]...)
	}
	return state
}

// Handle formats the provided record and writes the output to the
// destination.
func (s *SlogTextHandler) Handle(ctx context.Context, r slog.Record) error {
	state := s.newHandleState(newBuffer(), true)
	defer state.free()

	addTracingContextToRecord(ctx, &r)

	// Built-in attributes. They are not in a group.
	stateGroups := state.groups
	state.groups = nil // So ReplaceAttrs sees no groups instead of the pre groups.
	rep := s.cfg.ReplaceAttr

	if s.withTimestamp && !r.Time.IsZero() {
		if rep == nil {
			state.appendKey(slog.TimeKey)
			state.appendTime(r.Time.Round(0))
		} else {
			state.appendAttr(slog.Time(slog.TimeKey, r.Time.Round(0)))
		}
	}

	// Processing fields in this manner allows users to
	// configure the level and component position in the output.
	// This matches the behavior of the original logrus formatter. All other
	// fields location in the output message are static.
	for _, field := range s.cfg.ConfiguredFields {
		switch field {
		case LevelField:
			level := formatLevel(r.Level, s.cfg.EnableColors)

			if rep == nil {
				state.appendKey(slog.LevelKey)
				// Write the level directly to stat to avoid quoting
				// color formatting that exists.
				state.buf.WriteString(level)
			} else {
				state.appendAttr(slog.String(slog.LevelKey, level))
			}
		case ComponentField:
			// If a component is provided with the attributes, it should be used instead of
			// the component set on the handler. Note that if there are multiple components
			// specified in the arguments, the one with the lowest index is used and the others are ignored.
			// In the example below, the resulting component in the message output would be "alpaca".
			//
			//	logger := logger.With(teleport.ComponentKey, "fish")
			//	logger.InfoContext(ctx, "llama llama llama", teleport.ComponentKey, "alpaca", "foo", 123, teleport.ComponentKey, "shark")
			component := s.component
			r.Attrs(func(attr slog.Attr) bool {
				if attr.Key != teleport.ComponentKey {
					return true
				}

				component = formatComponent(attr.Value, s.cfg.Padding)
				return false
			})

			if rep == nil {
				state.appendKey(teleport.ComponentKey)
				state.appendString(component)
			} else {
				state.appendAttr(slog.String(teleport.ComponentKey, component))
			}
		default:
			if _, ok := knownFormatFields[field]; !ok {
				return trace.BadParameter("invalid log format key: %v", field)
			}
		}
	}

	if rep == nil {
		state.appendKey(slog.MessageKey)
		state.appendString(r.Message)
	} else {
		state.appendAttr(slog.String(slog.MessageKey, r.Message))
	}

	state.groups = stateGroups // Restore groups passed to ReplaceAttrs.
	state.appendNonBuiltIns(r)

	if r.PC != 0 && s.withCaller {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()

		src := slog.Source{
			Function: f.Function,
			File:     f.File,
			Line:     f.Line,
		}
		src.File, src.Line = getCaller(&src)

		if rep == nil {
			state.appendKey(slog.SourceKey)
			state.appendString(fmt.Sprintf("%s:%d", src.File, src.Line))
		} else {
			state.appendAttr(slog.Any(slog.SourceKey, &src))
		}

	}

	state.buf.WriteByte('\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.out.Write(*state.buf)
	return err
}

func formatLevel(value slog.Level, enableColors bool) string {
	var color int
	var level string
	switch value {
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
		level = value.String()
	}

	if !enableColors {
		color = noColor
	}

	level = padMax(level, defaultLevelPadding)
	if color != noColor {
		level = fmt.Sprintf("\u001B[%dm%s\u001B[0m", color, level)
	}

	return level
}

func formatComponent(value slog.Value, padding int) string {
	component := fmt.Sprintf("[%v]", value)
	component = strings.ToUpper(padMax(component, padding))
	if component[len(component)-1] != ' ' {
		component = component[:len(component)-1] + "]"
	}

	return component
}

func (s *SlogTextHandler) clone() *SlogTextHandler {
	// We can't use assignment because we can't copy the mutex.
	return &SlogTextHandler{
		cfg:           s.cfg,
		withCaller:    s.withCaller,
		withTimestamp: s.withTimestamp,
		component:     s.component,
		preformatted:  slices.Clip(s.preformatted),
		groupPrefix:   s.groupPrefix,
		groups:        slices.Clip(s.groups),
		nOpenGroups:   s.nOpenGroups,
		out:           s.out,
		mu:            s.mu, // mutex shared among all clones of this handler
	}
}

// WithAttrs clones the current handler with the provided attributes
// added to any existing attributes. The values are preformatted here
// so that they do not need to be formatted per call to Handle.
func (s *SlogTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return s
	}

	s2 := s.clone()
	// Pre-format the attributes as an optimization.
	state := s2.newHandleState((*buffer)(&s2.preformatted), false)
	defer state.free()
	state.prefix.WriteString(s.groupPrefix)

	// Remember the position in the buffer, in case all attrs are empty.
	pos := state.buf.Len()
	state.openGroups()

	nonEmpty := false
	for _, a := range attrs {
		switch a.Key {
		case teleport.ComponentKey:
			component := fmt.Sprintf("[%v]", a.Value.String())
			component = strings.ToUpper(padMax(component, s.cfg.Padding))
			if component[len(component)-1] != ' ' {
				component = component[:len(component)-1] + "]"
			}
			s2.component = component
		case teleport.ComponentFields:
			switch fields := a.Value.Any().(type) {
			case map[string]any:
				for k, v := range fields {
					if state.appendAttr(slog.Any(k, v)) {
						nonEmpty = true
					}
				}
			}
		default:
			if state.appendAttr(a) {
				nonEmpty = true
			}
		}
	}

	if !nonEmpty {
		state.buf.SetLen(pos)
	} else {
		// Remember the new prefix for later keys.
		s2.groupPrefix = state.prefix.String()
		// Remember how many opened groups are in preformattedAttrs,
		// so we don't open them again when we handle a Record.
		s2.nOpenGroups = len(s2.groups)
	}

	return s2
}

// WithGroup opens a new group.
func (s *SlogTextHandler) WithGroup(name string) slog.Handler {
	s2 := s.clone()
	s2.groups = append(s2.groups, name)
	return s2
}
