//go:build !windows
// +build !windows

/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
