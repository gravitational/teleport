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
	"bytes"
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/svc/eventlog"

	eventlogutils "github.com/gravitational/teleport/lib/utils/log/eventlog"
)

// NewSlogEventLogHandler creates a new slog handler that writes to the Windows Event Log as source.
// Requires registry entries to be set up first, see [eventlogutils.Install] and README in
// lib/utils/log/eventlog.
//
// The caller is expected to call the close function if the function does not return error.
func NewSlogEventLogHandler(source string, level slog.Leveler) (*SlogTextHandler, func() error, error) {
	writer, err := newEventLogWriter(source)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	handler := SlogTextHandler{
		cfg: SlogTextHandlerConfig{
			Level: level,
			// Event Log doesn't support colors.
			EnableColors: false,
			// Pass only CallerField so that the logger does not include the level and the timestamp
			// fields in the message. Event Log has dedicated handling for this kind of metadata.
			ConfiguredFields: []string{ComponentField, CallerField},
			// No padding since Event Log entries typically are viewed one by one, so adding padding
			// doesn't make them more readable.
			Padding: 0,
		},
		out:        writer,
		withCaller: true,
		// Event Log adds timestamps by itself.
		withTimestamp: false,
	}
	return &handler, writer.Close, nil
}

type eventLogWriter struct {
	log *eventlog.Log
}

// newEventLogWriter returns a writer for SlogTextHandler that writes events to Event Log as source.
// The caller is expected to close the writer after it's no longer needed.
func newEventLogWriter(source string) (*eventLogWriter, error) {
	log, err := eventlog.Open(source)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &eventLogWriter{
		log: log,
	}, nil
}

func (e *eventLogWriter) Write(bs []byte, rawComponent string, level slog.Level) error {
	// SlogTextHandler always adds a newline at the end and Event Log, unlike os_log, doesn't
	// automatically trim trailing newlines.
	bytesWithoutNewline := bytes.TrimRight(bs, " ")

	switch level {
	case slog.LevelWarn:
		return trace.Wrap(e.log.Warning(eventlogutils.EventID, string(bytesWithoutNewline)))
	case slog.LevelError:
		return trace.Wrap(e.log.Error(eventlogutils.EventID, string(bytesWithoutNewline)))
	default:
		// Event Log has no support for levels beyond info, warning, and error.
		return trace.Wrap(e.log.Info(eventlogutils.EventID, string(bytesWithoutNewline)))
	}
}

func (e *eventLogWriter) Close() error {
	return trace.Wrap(e.log.Close())
}
