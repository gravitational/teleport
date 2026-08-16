//go:build !windows

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
	"bytes"
	"context"
	"log/slog"
	"log/syslog"
	"sync"

	"github.com/gravitational/trace"
)

type SyslogWriter = *syslog.Writer

// NewSyslogWriter creates a writer that outputs to the local machine syslog.
func NewSyslogWriter() (SyslogWriter, error) {
	writer, err := syslog.Dial("", "", syslog.LOG_INFO, "")
	return writer, trace.Wrap(err)
}

// NewSyslogTextLogger creates a logger that formats records as text and writes them to syslog at the appropriate severity level.
func NewSyslogTextLogger(w SyslogWriter, cfg SlogTextHandlerConfig) *slog.Logger {
	buf := bytes.Buffer{}
	return slog.New(&SyslogHandler{
		inner: NewSlogTextHandler(&buf, cfg),
		w:     w,
		buf:   &buf,
		mu:    &sync.Mutex{},
	})
}

// NewSyslogJsonLogger creates a logger that formats records as JSON and writes them to syslog at the appropriate severity level.
func NewSyslogJsonLogger(w SyslogWriter, cfg SlogJSONHandlerConfig) *slog.Logger {
	buf := bytes.Buffer{}
	return slog.New(&SyslogHandler{
		inner: NewSlogJSONHandler(&buf, cfg),
		w:     w,
		buf:   &buf,
		mu:    &sync.Mutex{},
	})
}

// SyslogHandler wraps a slog.Handler (text or JSON), formats each record into a
// buffer, then writes it to syslog at the severity matching the record level.
type SyslogHandler struct {
	inner slog.Handler // the text/json handler that formats into buf
	w     *syslog.Writer
	buf   *bytes.Buffer
	mu    *sync.Mutex
}

func (h *SyslogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *SyslogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	h.buf.Reset()
	if err := h.inner.Handle(ctx, r); err != nil {
		h.mu.Unlock()
		return err
	}
	// Trim the trailing newline the inner handler adds; syslog adds its own framing.
	msg := bytes.TrimRight(h.buf.Bytes(), "\n")
	s := string(msg)
	h.mu.Unlock()

	switch {
	case r.Level >= slog.LevelError:
		return h.w.Err(s)
	case r.Level >= slog.LevelWarn:
		return h.w.Warning(s)
	case r.Level >= slog.LevelInfo:
		return h.w.Info(s)
	default:
		return h.w.Debug(s)
	}
}

// WithAttrs and WithGroup must propagate to the inner handler while keeping the
// same buffer/mutex/writer, so all clones serialize through the same lock.
func (h *SyslogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SyslogHandler{
		inner: h.inner.WithAttrs(attrs),
		w:     h.w,
		buf:   h.buf,
		mu:    h.mu,
	}
}

func (h *SyslogHandler) WithGroup(name string) slog.Handler {
	return &SyslogHandler{
		inner: h.inner.WithGroup(name),
		w:     h.w,
		buf:   h.buf,
		mu:    h.mu,
	}
}
