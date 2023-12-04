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
	"os"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Output   string `toml:"output"`
	Severity string `toml:"severity"`
}

type Fields = log.Fields

type contextKey struct{}

// InitLogger sets up logger for a typical daemon scenario until configuration
// file is parsed
func Init() {
	log.SetFormatter(&trace.TextFormatter{
		DisableTimestamp: true,
		EnableColors:     trace.IsTerminal(os.Stderr),
		ComponentPadding: 1, // We don't use components so strip the padding
	})
	log.SetOutput(os.Stderr)
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
