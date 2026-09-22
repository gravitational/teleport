/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package safefile provides safe file operations that avoid following symlinks.
package safefile

import (
	"os"
)

// OpenFileNoFollow opens a file without following symlinks in any part
// of the path. If a symlink is encountered an error will be returned.
func OpenFileNoFollow(file string, flags int, mode os.FileMode) (*os.File, error) {
	return openFileNoFollow(file, flags, mode)
}
