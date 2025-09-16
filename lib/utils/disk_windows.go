//go:build windows
// +build windows

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
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

// PercentUsed is not supported on Windows.
func PercentUsed(path string) (float64, error) {
	return 0.0, trace.NotImplemented("disk usage not supported on Windows")
}

// FreeDiskWithReserve returns the available disk space (in bytes) on the disk at dir, minus `reservedFreeDisk`.
func FreeDiskWithReserve(dir string, reservedFreeDisk uint64) (uint64, error) {
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

// CanUserWriteTo is not supported on Windows.
func CanUserWriteTo(path string) (bool, error) {
	return false, trace.NotImplemented("path permission checking is not supported on Windows")
}
