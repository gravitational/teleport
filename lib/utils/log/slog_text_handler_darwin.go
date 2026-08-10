//go:build darwin && cgo

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"log/slog"
	"sync"

	"github.com/gravitational/teleport/lib/utils/log/oslog"
)

// NewSlogOSLogHandler creates a SlogTextHandler that writes messages to os_log as subsystem.
func NewSlogOSLogHandler(subsystem string, level slog.Leveler) (*SlogTextHandler, error) {
	handler := SlogTextHandler{
		cfg: SlogTextHandlerConfig{
			Level: level,
			// os_log doesn't support colors.
			EnableColors: false,
			// Pass only CallerField so that the logger does not include the level, component and
			// timestamp fields in the message. os_log has dedicated handling for this kind of metadata.
			ConfiguredFields: []string{CallerField},
		},
		out:        newOSLogWriter(subsystem),
		withCaller: true,
	}

	return &handler, nil
}

// osLogWriter is an [outputWriter] that writes to os_log, the
// unified logging system on macOS.
type osLogWriter struct {
	subsystem string
	mu        sync.Mutex
	loggers   map[string]*oslog.Logger
}

// newOSLogWriter creates a new output that writes to os_log. All oslog.Logger instances created by
// this output are going to use the given subsystem, whereas the category comes from the component
// passed to the Write method.
func newOSLogWriter(subsystem string) *osLogWriter {
	return &osLogWriter{
		subsystem: subsystem,
		loggers:   map[string]*oslog.Logger{},
	}
}

// Write sends the message from buf to os_log and maps level to a specific oslog.OsLogType.
// os_log truncates messages by default, see [oslog.Logger.Log] for more details.
func (o *osLogWriter) Write(bytes []byte, rawComponent string, level slog.Level) error {
	logger := o.getLogger(rawComponent)

	var osLogType oslog.OsLogType

	switch level {
	case TraceLevel, slog.LevelDebug:
		osLogType = oslog.OsLogTypeDebug
	case slog.LevelInfo:
		osLogType = oslog.OsLogTypeInfo
	case slog.LevelWarn:
		osLogType = oslog.OsLogTypeDefault
	case slog.LevelError:
		osLogType = oslog.OsLogTypeError
	default:
		osLogType = oslog.OsLogTypeDefault
	}

	logger.Log(osLogType, string(bytes))

	return nil
}

func (o *osLogWriter) getLogger(rawComponent string) *oslog.Logger {
	o.mu.Lock()
	defer o.mu.Unlock()

	logger, found := o.loggers[rawComponent]
	if found {
		return logger
	}

	logger = oslog.NewLogger(o.subsystem, rawComponent)
	o.loggers[rawComponent] = logger
	return logger
}
