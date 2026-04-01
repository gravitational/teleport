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

package logutils

import (
	"context"
	"log/slog"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
)

const (
	TeleportComponentKey    = "trace.component"
	TeleportComponentFields = "trace.fields"
)

const (
	// TraceLevel is the logging level when set to Trace verbosity.
	TraceLevel = slog.LevelDebug - 1

	// TraceLevelText is the text representation of Trace verbosity.
	TraceLevelText = "TRACE"

	noColor = -1
	red     = 31
	yellow  = 33
	blue    = 36
	gray    = 37
	// LevelField is the log field that stores the verbosity.
	LevelField = "level"
	// ComponentField is the log field that stores the calling component.
	ComponentField = "component"
	// CallerField is the log field that stores the calling file and line number.
	CallerField = "caller"
	// TimestampField is the field that stores the timestamp the log was emitted.
	TimestampField = "timestamp"
	messageField   = "message"
	// defaultComponentPadding is a default padding for component field
	defaultComponentPadding = 11
	// defaultLevelPadding is a default padding for level field
	defaultLevelPadding = 4
)

// SupportedLevelsText lists the supported log levels in their text
// representation. All strings are in uppercase.
var SupportedLevelsText = []string{
	TraceLevelText,
	slog.LevelDebug.String(),
	slog.LevelInfo.String(),
	slog.LevelWarn.String(),
	slog.LevelError.String(),
}

func addTracingContextToRecord(ctx context.Context, r *slog.Record) {
	// there can't be a span from context from OTEL because we don't include
	// OTEL in this module
}

var defaultFormatFields = []string{LevelField, ComponentField, CallerField, TimestampField}

var knownFormatFields = map[string]struct{}{
	LevelField:     {},
	ComponentField: {},
	CallerField:    {},
	TimestampField: {},
}

// ValidateFields ensures the provided fields map to the allowed fields. An error
// is returned if any of the fields are invalid.
func ValidateFields(formatInput []string) (result []string, err error) {
	for _, component := range formatInput {
		component = strings.TrimSpace(component)
		if _, ok := knownFormatFields[component]; !ok {
			return nil, trace.BadParameter("invalid log format key: %q", component)
		}
		result = append(result, component)
	}
	return result, nil
}

// needsQuoting returns true if any non-printable characters are found.
func needsQuoting(text string) bool {
	for _, r := range text {
		if !unicode.IsPrint(r) {
			return true
		}
	}
	return false
}

func padMax(in string, chars int) string {
	switch {
	case len(in) < chars:
		return in + strings.Repeat(" ", chars-len(in))
	default:
		return in[:chars]
	}
}

// getCaller retrieves source information from the attribute
// and returns the file and line of the caller. The file is
// truncated from the absolute path to package/filename.
func getCaller(s *slog.Source) (file string, line int) {
	count := 0
	idx := strings.LastIndexFunc(s.File, func(r rune) bool {
		if r == '/' {
			count++
		}

		return count == 2
	})
	file = s.File[idx+1:]
	line = s.Line

	return file, line
}
