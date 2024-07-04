//go:build !linux
// +build !linux

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

package botfs

import (
	"context"
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
			log.WarnContext(
				context.TODO(),
				"Secure symlinks not supported on this platform, set `symlinks: insecure` to disable this message",
				"path", path,
			)
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
			log.WarnContext(
				context.TODO(),
				"Secure symlinks not supported on this platform, set `symlinks: insecure` to disable this message",
				"path", path,
			)
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
func HasACLSupport() bool {
	return false
}

// HasSecureWriteSupport determines if `CreateSecure()` should be supported
// on this OS / kernel version. This is only supported on Linux, so this
// catch-all implementation just returns false.
func HasSecureWriteSupport() bool {
	return false
}
