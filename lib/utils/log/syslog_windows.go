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
	"io"
	"log/slog"

	"github.com/gravitational/trace"
)

type SyslogWriter = io.Writer

// NewSyslogWriter always returns an error on Windows.
func NewSyslogWriter() (SyslogWriter, error) {
	return nil, trace.NotImplemented("cannot use syslog on Windows")
}

// NewSyslogTextLogger always returns a discard logger on Windows; syslog is not supported.
func NewSyslogTextLogger(w SyslogWriter, cfg SlogTextHandlerConfig) *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// NewSyslogJsonLogger always returns a discard logger on Windows; syslog is not supported.
func NewSyslogJsonLogger(w SyslogWriter, cfg SlogJSONHandlerConfig) *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
