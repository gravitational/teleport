//go:build windows
// +build windows

package utils

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

import (
	"os"
	"strings"
)

const lockPostfix = ".lock.tmp"

// On Windows we use auxiliary .lock.tmp files to acquire locks, so we can still read/write target
// files themselves.
//
// .lock.tmp files are deliberately not cleaned up. Their presence doesn't matter to the actual
// locking. Repeatedly removing them on unlock when acquiring dozens of locks in a short timespan
// was causing flock.Flock.TryRLock to return either "access denied" or "The process cannot access
// the file because it is being used by another process".
func getPlatformLockFilePath(path string) string {
	// If target file is itself dedicated lockfile, we don't create another lockfile, since
	// we don't intend to read/write the target file itself.
	if strings.HasSuffix(path, ".lock") {
		return path
	}
	return path + lockPostfix
}

func getHardLinkCount(fi os.FileInfo) (uint64, bool) {
	// Although hardlinks on Windows are possible, Go does not currently expose the hardlinks associated to a file on windows
	return 0, false
}
