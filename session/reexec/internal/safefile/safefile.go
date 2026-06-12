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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"
)

// OpenNoFollow opens a file for reading without following symlinks in
// any part of the path. If a symlink is encountered an error will be
// returned.
func OpenNoFollow(file string) (*os.File, error) {
	return OpenFileNoFollow(file, os.O_RDONLY, 0)
}

// OpenFileNoFollow opens a file without following symlinks in any part
// of the path. If a symlink is encountered an error will be returned.
func OpenFileNoFollow(file string, flags int, mode os.FileMode) (*os.File, error) {
	if !filepath.IsAbs(file) {
		return nil, trace.BadParameter("file path must be absolute")
	}
	dir, filename := filepath.Split(file)
	relDir, err := filepath.Rel(string(os.PathSeparator), dir)
	if err != nil {
		return nil, err
	}
	parent, err := os.OpenFile(string(os.PathSeparator), readOnlyPath, 0)
	if err != nil {
		return nil, err
	}
	// Open each directory one at a time to ensure no symlinks are followed.
	for relDir != "" {
		var part string
		part, relDir, _ = strings.Cut(relDir, string(os.PathSeparator))
		parent, err = openAtAndClose(parent, part, unix.O_DIRECTORY|readOnlyPath, 0)
		if err != nil {
			return nil, err
		}
	}
	// Set nonblock so we don't hang in case file is a pipe.
	f, err := openAtAndClose(parent, filename, flags|unix.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, trace.BadParameter("path does not point to a regular file")
	}
	return f, nil
}

// openAtAndClose opens a file under the parent directory without
// following symlinks, then closes the parent file. The parent file
// will be closed even if this function returns an error.
func openAtAndClose(parent *os.File, name string, flags int, mode os.FileMode) (*os.File, error) {
	defer parent.Close()
	syscallConn, err := parent.SyscallConn()
	if err != nil {
		return nil, err
	}
	var childFd int
	var openAtErr error
	ctrlErr := syscallConn.Control(func(fd uintptr) {
		for {
			childFd, openAtErr = unix.Openat(int(fd), name, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode))
			if !errors.Is(openAtErr, syscall.EINTR) {
				return
			}
		}
	})
	if ctrlErr != nil {
		return nil, ctrlErr
	} else if openAtErr != nil {
		return nil, openAtErr
	}
	return os.NewFile(uintptr(childFd), filepath.Join(parent.Name(), name)), nil
}
