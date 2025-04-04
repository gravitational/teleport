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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/log/oslog"
)

// SlogTextHandlerOutputOSLog represents an output for SlogTextHandler that writes to os_log, the
// unified logging system on macOS.
type SlogTextHandlerOutputOSLog struct {
	subsystem string
	mu        sync.Mutex
	isClosed  bool
	loggers   map[string]*oslog.Logger
}

// NewSlogTextHandlerOutputOSLog creates a new output that writes to os_log. All oslog.Logger
// instances created by this output are going to use the given subsystem, whereas the category comes
// from the component passed to the Write method.
func NewSlogTextHandlerOutputOSLog(subsystem string) *SlogTextHandlerOutputOSLog {
	return &SlogTextHandlerOutputOSLog{
		subsystem: subsystem,
		loggers:   map[string]*oslog.Logger{},
	}
}

// Write sends the message from buf to os_log and maps level to a specific oslog.OsLogType.
// os_log truncates messages by default, see [oslog.Logger.Log] for more details.
func (o *SlogTextHandlerOutputOSLog) Write(buf *buffer, rawComponent string, level slog.Level) error {
	logger, err := o.getLogger(rawComponent)
	if err != nil {
		return trace.Wrap(err)
	}

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
	case slog.LevelError + 1:
		osLogType = oslog.OsLogTypeFault
	default:
		osLogType = oslog.OsLogTypeDefault
	}

	logger.Log(osLogType, buf.String())

	return nil
}

func (o *SlogTextHandlerOutputOSLog) getLogger(rawComponent string) (*oslog.Logger, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.isClosed {
		return nil, trace.Errorf("OutputOSLog is closed")
	}

	logger, found := o.loggers[rawComponent]
	if found {
		return logger, nil
	}

	logger = oslog.NewLogger(o.subsystem, rawComponent)
	o.loggers[rawComponent] = logger
	return logger, nil
}

// Close releases objects that back all loggers created thus far and makes all further calls to Write
// fail.
func (o *SlogTextHandlerOutputOSLog) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	var errs []error
	for _, logger := range o.loggers {
		if err := logger.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	o.isClosed = true

	return trace.NewAggregate(errs...)
}
