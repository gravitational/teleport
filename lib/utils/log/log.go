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
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// Config configures teleport logging
type Config struct {
	// Output defines where logs go. It can be one of the following: "stderr", "stdout" or
	// a path to a log file
	Output string
	// Severity defines how verbose the log will be. Possible values are "error", "info", "warn"
	Severity string
	// Format defines the output format. Possible values are 'text' and 'json'.
	Format string
	// ExtraFields lists the output fields from KnownFormatFields. Example format: [timestamp, component, caller]
	ExtraFields []string
	// EnableColors dictates if output should be colored.
	EnableColors bool
	// Padding to use for various components.
	Padding int
}

// Initialize configures the default global logger based on the
// provided configuration. The [slog.Logger] and [slog.LevelVar]
func Initialize(loggerConfig Config) (*slog.Logger, *slog.LevelVar, error) {
	const (
		// logFileDefaultMode is the preferred permissions mode for log file.
		logFileDefaultMode fs.FileMode = 0o644
		// logFileDefaultFlag is the preferred flags set to log file.
		logFileDefaultFlag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	)

	var w io.Writer
	level := new(slog.LevelVar)
	switch loggerConfig.Output {
	case "":
		w = os.Stderr
	case "stderr", "error", "2":
		w = os.Stderr
	case "stdout", "out", "1":
		w = os.Stdout
	case teleport.Syslog:
		var err error
		w, err = NewSyslogWriter()
		if err != nil {
			slog.ErrorContext(context.Background(), "Failed to switch logging to syslog", "error", err)
			slog.SetDefault(slog.New(DiscardHandler{}))
			return slog.Default(), level, nil
		}
	default:
		// Assume a file path for all other provided output values.
		sharedWriter, err := NewFileSharedWriter(loggerConfig.Output, logFileDefaultFlag, logFileDefaultMode)
		if err != nil {
			return nil, nil, trace.Wrap(err, "failed to init the log file shared writer")
		}
		w = NewWriterFinalizer(sharedWriter)
		if err := sharedWriter.RunWatcherReopen(context.Background()); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	switch strings.ToLower(loggerConfig.Severity) {
	case "", "info":
		level.Set(slog.LevelInfo)
	case "err", "error":
		level.Set(slog.LevelError)
	case teleport.DebugLevel:
		level.Set(slog.LevelDebug)
	case "warn", "warning":
		level.Set(slog.LevelWarn)
	case "trace":
		level.Set(TraceLevel)
	default:
		return nil, nil, trace.BadParameter("unsupported logger severity: %q", loggerConfig.Severity)
	}

	configuredFields, err := ValidateFields(loggerConfig.ExtraFields)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var logger *slog.Logger
	switch strings.ToLower(loggerConfig.Format) {
	case "":
		fallthrough // not set. defaults to 'text'
	case "text":
		logger = slog.New(NewSlogTextHandler(w, SlogTextHandlerConfig{
			Level:            level,
			EnableColors:     loggerConfig.EnableColors,
			ConfiguredFields: configuredFields,
			Padding:          loggerConfig.Padding,
		}))
		slog.SetDefault(logger)
	case "json":
		logger = slog.New(NewSlogJSONHandler(w, SlogJSONHandlerConfig{
			Level:            level,
			ConfiguredFields: configuredFields,
		}))
		slog.SetDefault(logger)
	default:
		return nil, nil, trace.BadParameter("unsupported log output format : %q", loggerConfig.Format)
	}

	return logger, level, nil
}
