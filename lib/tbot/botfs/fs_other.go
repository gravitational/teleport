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

import (
	"io"
	"os/user"
	"sync"

	"github.com/gravitational/trace"
)

// unsupportedPlatformWarning is used to reduce log spam when on an unsupported
// platform.
var unsupportedPlatformWarning sync.Once

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

// Read reads the contents of the given file into memory.
func Read(path string, symlinksMode SymlinksMode) ([]byte, error) {
	switch symlinksMode {
	case SymlinksSecure:
		return nil, trace.BadParameter("cannot read with `symlinks: secure` on unsupported platform")
	case SymlinksTrySecure:
		unsupportedPlatformWarning.Do(func() {
			log.Warn("Secure symlinks not supported on this platform, set `symlinks: insecure` to disable this message", path)
		})
	}

	file, err := openStandard(path, ReadMode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

// Write stores the given data to the file at the given path.
func Write(path string, data []byte, symlinksMode SymlinksMode) error {
	switch symlinksMode {
	case SymlinksSecure:
		return trace.BadParameter("cannot write with `symlinks: secure` on unsupported platform")
	case SymlinksTrySecure:
		unsupportedPlatformWarning.Do(func() {
			log.Warn("Secure symlinks not supported on this platform, set `symlinks: insecure` to disable this message", path)
		})
	}

	file, err := openStandard(path, WriteMode)
	if err != nil {
		return trace.Wrap(err)
	}

	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// VerifyACL verifies whether the ACL of the given file allows writes from the
// bot user.
//
//nolint:staticcheck // staticcheck does not like our nop implementations.
func VerifyACL(path string, opts *ACLOptions) error {
	return trace.NotImplemented("ACLs not supported on this platform")
}

// ConfigureACL configures ACLs of the given file to allow writes from the bot
// user.
//
//nolint:staticcheck // staticcheck does not like our nop implementations.
func ConfigureACL(path string, owner *user.User, opts *ACLOptions) error {
	return trace.NotImplemented("ACLs not supported on this platform")
}

// HasACLSupport determines if this binary / system supports ACLs. This
// catch-all implementation just returns false.
func HasACLSupport() (bool, error) {
	return false, nil
}

// HasSecureWriteSupport determines if `CreateSecure()` should be supported
// on this OS / kernel version. This is only supported on Linux, so this
// catch-all implementation just returns false.
func HasSecureWriteSupport() (bool, error) {
	return false, nil
}
