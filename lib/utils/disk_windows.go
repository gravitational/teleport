//go:build windows
// +build windows

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
