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

package logger

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// These values are meant to be kept in sync with teleport/lib/config.
// (We avoid importing that package here because integrations must not require CGo)
const (
	// logFileDefaultMode is the preferred permissions mode for log file.
	logFileDefaultMode fs.FileMode = 0o644
	// logFileDefaultFlag is the preferred flags set to log file.
	logFileDefaultFlag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
)

type Config struct {
	Output   string `toml:"output"`
	Severity string `toml:"severity"`
	// Format is only used by plugins using slog
	Format string `toml:"format"`
}

type Fields = log.Fields

type contextKey struct{}

var extraFields = []string{logutils.LevelField, logutils.ComponentField, logutils.CallerField}

// Init sets up logger for a typical daemon scenario until configuration
// file is parsed
func Init() {
	formatter := &logutils.TextFormatter{
		EnableColors:     utils.IsTerminal(os.Stderr),
		ComponentPadding: 1, // We don't use components so strip the padding
		ExtraFields:      extraFields,
	}

	log.SetOutput(os.Stderr)
	if err := formatter.CheckAndSetDefaults(); err != nil {
		log.WithError(err).Error("unable to create text log formatter")
		return
	}

	log.SetFormatter(formatter)
}

func Setup(conf Config) error {
	switch conf.Output {
	case "stderr", "error", "2":
		log.SetOutput(os.Stderr)
	case "", "stdout", "out", "1":
		log.SetOutput(os.Stdout)
	default:
		// assume it's a file path:
		logFile, err := os.Create(conf.Output)
		if err != nil {
			return trace.Wrap(err, "failed to create the log file")
		}
		log.SetOutput(logFile)
	}

	switch strings.ToLower(conf.Severity) {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "err", "error":
		log.SetLevel(log.ErrorLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "trace":
		log.SetLevel(log.TraceLevel)
	default:
		return trace.BadParameter("unsupported logger severity: '%v'", conf.Severity)
	}

	return nil
}

// NewSLogLogger builds a slog.Logger from the logger.Config.
// TODO: this code is adapted from `config.applyLogConfig`, we'll want to deduplicate the logic next time we refactor the logging setup
func (conf Config) NewSLogLogger() (*slog.Logger, error) {
	var w io.Writer
	switch conf.Output {
	case "":
		w = logutils.NewSharedWriter(os.Stderr)
	case "stderr", "error", "2":
		w = logutils.NewSharedWriter(os.Stderr)
	case "stdout", "out", "1":
		w = logutils.NewSharedWriter(os.Stdout)
	case teleport.Syslog:
		w = os.Stderr
		sw, err := utils.NewSyslogWriter()
		if err != nil {
			slog.Default().ErrorContext(context.Background(), "Failed to switch logging to syslog", "error", err)
			break
		}

		// If syslog output has been configured and is supported by the operating system,
		// then the shared writer is not needed because the syslog writer is already
		// protected with a mutex.
		w = sw
	default:
		// Assume this is a file path.
		sharedWriter, err := logutils.NewFileSharedWriter(conf.Output, logFileDefaultFlag, logFileDefaultMode)
		if err != nil {
			return nil, trace.Wrap(err, "failed to init the log file shared writer")
		}
		w = logutils.NewWriterFinalizer[*logutils.FileSharedWriter](sharedWriter)
		if err := sharedWriter.RunWatcherReopen(context.Background()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	level := new(slog.LevelVar)
	switch strings.ToLower(conf.Severity) {
	case "", "info":
		level.Set(slog.LevelInfo)
	case "err", "error":
		level.Set(slog.LevelError)
	case teleport.DebugLevel:
		level.Set(slog.LevelDebug)
	case "warn", "warning":
		level.Set(slog.LevelWarn)
	case "trace":
		level.Set(logutils.TraceLevel)
	default:
		return nil, trace.BadParameter("unsupported logger severity: %q", conf.Severity)
	}

	configuredFields, err := logutils.ValidateFields(extraFields)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var slogLogger *slog.Logger
	switch strings.ToLower(conf.Format) {
	case "":
		fallthrough // not set. defaults to 'text'
	case "text":
		enableColors := utils.IsTerminal(os.Stderr)
		slogLogger = slog.New(logutils.NewSlogTextHandler(w, logutils.SlogTextHandlerConfig{
			Level:            level,
			EnableColors:     enableColors,
			ConfiguredFields: configuredFields,
		}))
		slog.SetDefault(slogLogger)
	case "json":
		slogLogger = slog.New(logutils.NewSlogJSONHandler(w, logutils.SlogJSONHandlerConfig{
			Level:            level,
			ConfiguredFields: configuredFields,
		}))
		slog.SetDefault(slogLogger)
	default:
		return nil, trace.BadParameter("unsupported log output format : %q", conf.Format)
	}

	return slogLogger, nil
}

func WithLogger(ctx context.Context, logger log.FieldLogger) context.Context {
	return withLogger(ctx, logger)
}

func withLogger(ctx context.Context, logger log.FieldLogger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

func WithField(ctx context.Context, key string, value interface{}) (context.Context, log.FieldLogger) {
	logger := Get(ctx).WithField(key, value)
	return withLogger(ctx, logger), logger
}

func WithFields(ctx context.Context, logFields Fields) (context.Context, log.FieldLogger) {
	logger := Get(ctx).WithFields(logFields)
	return withLogger(ctx, logger), logger
}

func SetField(ctx context.Context, key string, value interface{}) context.Context {
	ctx, _ = WithField(ctx, key, value)
	return ctx
}

func SetFields(ctx context.Context, logFields Fields) context.Context {
	ctx, _ = WithFields(ctx, logFields)
	return ctx
}

func Get(ctx context.Context) log.FieldLogger {
	if logger, ok := ctx.Value(contextKey{}).(log.FieldLogger); ok && logger != nil {
		return logger
	}

	return Standard()
}

func Standard() log.FieldLogger {
	return log.StandardLogger()
}
