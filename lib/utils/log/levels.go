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
	"log/slog"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// SupportedLevelsText list of the supported log levels in their text
// representation. All strings are in uppercase.
var SupportedLevelsText = []string{
	TraceLevelText,
	slog.LevelDebug.String(),
	slog.LevelInfo.String(),
	slog.LevelWarn.String(),
	slog.LevelError.String(),
}

// SlogLevelToLogrusLevel converts a [slog.Level] to its equivalent
// [logrus.Level].
func SlogLevelToLogrusLevel(level slog.Level) logrus.Level {
	switch level {
	case TraceLevel:
		return logrus.TraceLevel
	case slog.LevelDebug:
		return logrus.DebugLevel
	case slog.LevelInfo:
		return logrus.InfoLevel
	case slog.LevelWarn:
		return logrus.WarnLevel
	case slog.LevelError:
		return logrus.ErrorLevel
	default:
		return logrus.FatalLevel
	}
}

// UnmarshalText unmarshals log level text representation to slog.Level.
func UnmarshalText(data []byte) (slog.Level, error) {
	if strings.EqualFold(string(data), TraceLevelText) {
		return TraceLevel, nil
	}

	var level slog.Level
	if err := level.UnmarshalText(data); err != nil {
		return level, trace.Wrap(err)
	}

	return level, nil
}

// MarshalText marshals log level to its text representation.
func MarshalText(level slog.Level) string {
	if level == TraceLevel {
		return TraceLevelText
	}

	return level.String()
}
