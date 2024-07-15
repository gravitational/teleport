//go:build !windows
// +build !windows

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

package utils

import (
	"io"
	"log/syslog"
	"os"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

// SwitchLoggingToSyslog configures the default logger to send output to syslog.
func SwitchLoggingToSyslog() error {
	logger := logrus.StandardLogger()

	w, err := NewSyslogWriter()
	if err != nil {
		logger.Errorf("Failed to switch logging to syslog: %v.", err)
		logger.SetOutput(os.Stderr)
		return trace.Wrap(err)
	}

	hook, err := NewSyslogHook(w)
	if err != nil {
		logger.Errorf("Failed to switch logging to syslog: %v.", err)
		logger.SetOutput(os.Stderr)
		return trace.Wrap(err)
	}

	logger.ReplaceHooks(make(logrus.LevelHooks))
	logger.AddHook(hook)
	logger.SetOutput(io.Discard)

	return nil
}

// NewSyslogHook provides a [logrus.Hook] that sends output to syslog.
func NewSyslogHook(w io.Writer) (logrus.Hook, error) {
	if w == nil {
		return nil, trace.BadParameter("syslog writer must not be nil")
	}

	sw, ok := w.(*syslog.Writer)
	if !ok {
		return nil, trace.BadParameter("expected a syslog writer, got %T", w)
	}

	return &logrusSyslog.SyslogHook{Writer: sw}, nil
}

// NewSyslogWriter creates a writer that outputs to the local machine syslog.
func NewSyslogWriter() (io.Writer, error) {
	writer, err := syslog.Dial("", "", syslog.LOG_WARNING, "")
	return writer, trace.Wrap(err)
}
