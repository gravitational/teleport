// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"log/slog"

	"github.com/gravitational/teleport/lib/utils/log/packagelogger"
)

// NewPackageLogger creates a [slog.Logger] that defers setting
// any groups or attributes until the first use of the logger.
// This allows package global loggers to be created prior to
// any custom [slog.Handler] can be set via [slog.Default] AND
// still respect the formatting of the default handler set at
// runtime.
func NewPackageLogger(args ...any) *slog.Logger {
	return packagelogger.NewPackageLogger(args...)
}
