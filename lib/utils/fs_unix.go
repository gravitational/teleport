//go:build !windows
// +build !windows

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

package utils

import (
	"os"
	"syscall"
)

// On non-windows we just lock the target file itself.
func getPlatformLockFilePath(path string) string {
	return path
}

func getHardLinkCount(fi os.FileInfo) (uint64, bool) {
	if statT, ok := fi.Sys().(*syscall.Stat_t); ok {
		// we must do a cast here because this will be uint16 on OSX
		//nolint:unconvert // the cast is only necessary for macOS
		return uint64(statT.Nlink), true
	} else {
		return 0, false
	}
}
