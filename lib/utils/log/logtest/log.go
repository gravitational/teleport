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

package logtest

import (
	"flag"
	"log/slog"
	"sync"

	"github.com/gravitational/teleport/lib/utils/log"
)

var initLoggerOnce = sync.Once{}

// InitLogger initializes the standard logger.
func InitLogger(verbose func() bool) {
	initLoggerOnce.Do(func() {
		if !flag.Parsed() {
			// Parse flags to check testing.Verbose().
			flag.Parse()
		}

		if !verbose() {
			slog.SetDefault(slog.New(slog.DiscardHandler))
			return
		}

		log.Initialize(log.Config{
			Severity: slog.LevelDebug.String(),
			Format:   "json",
		})
	})
}

// With creates a new [slog.Logger] with the provided attributes for test environments.
func With(args ...any) *slog.Logger {
	InitLogger(func() bool { return false })
	return slog.With(args...)
}

// NewLogger creates a new [slog.Logger] for test environments.
func NewLogger() *slog.Logger {
	InitLogger(func() bool { return false })
	return slog.Default()
}
