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
	"log/slog"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type Config struct {
	Output   string `toml:"output"`
	Severity string `toml:"severity"`
	// Format is only used by plugins using slog
	Format string `toml:"format"`
}

type contextKey struct{}

var extraFields = []string{logutils.LevelField, logutils.ComponentField, logutils.CallerField}

// Init sets up logger for a typical daemon scenario until configuration
// file is parsed
func Init() {
	enableColors := utils.IsTerminal(os.Stderr)
	logutils.Initialize(logutils.Config{
		Severity:     slog.LevelInfo.String(),
		Format:       "text",
		ExtraFields:  extraFields,
		EnableColors: enableColors,
		Padding:      1,
	})
}

func Setup(conf Config) error {
	var enableColors bool
	switch conf.Output {
	case "stderr", "error", "2":
		enableColors = utils.IsTerminal(os.Stderr)
	case "", "stdout", "out", "1":
		enableColors = utils.IsTerminal(os.Stdout)
	default:
	}

	_, _, err := logutils.Initialize(logutils.Config{
		Output:       conf.Output,
		Severity:     conf.Severity,
		Format:       conf.Format,
		ExtraFields:  extraFields,
		EnableColors: enableColors,
		Padding:      1,
	})
	return trace.Wrap(err)
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

func With(ctx context.Context, args ...any) (context.Context, *slog.Logger) {
	logger := Get(ctx).With(args...)
	return WithLogger(ctx, logger), logger
}

func Get(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}

	return slog.Default()
}
