// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"log/slog"

	"github.com/gravitational/teleport/session/common/logutils/logtest"
)

//go:fix inline
func InitLogger(verbose func() bool) {
	logtest.InitLogger(verbose)
}

//go:fix inline
func With(args ...any) *slog.Logger {
	return logtest.With(args...)
}

//go:fix inline
func NewLogger() *slog.Logger {
	return logtest.NewLogger()
}
