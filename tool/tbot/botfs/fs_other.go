//go:build !linux
// +build !linux

/*
Copyright 2022 Gravitational, Inc.

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

package botfs

import "github.com/gravitational/trace"

// Create attempts to create the given file or directory without
// evaluating symlinks. This is only supported on recent Linux kernel versions
// (5.6+). The resulting file permissions are unspecified; Chmod should be
// called afterward.
func Create(path string, isDir bool, symlinksMode SymlinksMode) error {
	if symlinksMode == SymlinksSecure {
		return trace.BadParameter("cannot write with `symlinks: secure` on unsupported platform")
	}

	return trace.Wrap(createStandard(path, isDir))
}

// Write stores the given data to the file at the given path.
func Write(path string, data []byte, symlinksMode SymlinksMode) error {
	return nil
}

// HasACLSupport determines if this binary / system supports ACLs. This
// catch-all implementation just returns false.
func HasACLSupport() (bool, error) {
	return false, nil
}

// HasSecureWriteSupport determines if `CreateSecure()` should be supported
// on this OS / kernel version. This is only supported on Linux, so this
// catch-all implementation just returns fales.
func HasSecureWriteSupport() (bool, error) {
	return false, nil
}
