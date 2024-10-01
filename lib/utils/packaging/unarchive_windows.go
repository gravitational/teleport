//go:build windows

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package packaging

import (
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

// Replace un-archives package from tools directory and replaces defined apps by symlinks.
func Replace(toolsDir string, archivePath string, hash string, apps []string) error {
	return replaceZip(toolsDir, archivePath, hash, apps)
}

// freeDiskWithReserve returns the available disk space.
func freeDiskWithReserve(dir string) (uint64, error) {
	var avail uint64
	err := windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(dir), &avail, nil, nil)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if reservedFreeDisk > avail {
		return 0, trace.Errorf("no free space left")
	}
	return avail - reservedFreeDisk, nil
}
