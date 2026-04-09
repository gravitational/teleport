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

package sftputils

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// LocalFS provides API for accessing the files on
// the local file system
type LocalFS struct{}

func (l LocalFS) Type() string {
	return "local"
}

func (l LocalFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

func (l LocalFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (l LocalFS) ReadDir(path string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	fileInfos := make([]fs.FileInfo, len(entries))
	for i, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		// If the file is a valid symlink, return the info of the linked file.
		if info.Mode()&os.ModeSymlink != 0 {
			resolvedInfo, err := os.Stat(filepath.Join(path, info.Name()))
			if err == nil {
				info = resolvedInfo
			}
		}

		fileInfos[i] = info
	}

	return fileInfos, nil
}

func (l LocalFS) Open(path string) (File, error) {

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &fileWrapper{File: f}, nil
}

func (l LocalFS) Create(path string, _ int64) (File, error) {
	return l.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

func (l LocalFS) OpenFile(path string, flags int) (File, error) {
	return os.OpenFile(path, flags, 0o644)
}

func (l LocalFS) Mkdir(path string) error {
	err := os.MkdirAll(path, 0o755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

func (l LocalFS) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func (l LocalFS) Chtimes(path string, atime, mtime time.Time) error {
	return os.Chtimes(path, atime, mtime)
}

func (l LocalFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (l LocalFS) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

func (l LocalFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (l LocalFS) Link(oldname, newname string) error {
	return os.Link(oldname, newname)
}

func (l LocalFS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (l LocalFS) Remove(name string) error {
	return os.Remove(name)
}

func (l LocalFS) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (l LocalFS) Truncate(name string, size int64) error {
	return os.Truncate(name, size)
}

func (l LocalFS) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (l LocalFS) Getwd() (string, error) {
	return os.Getwd()
}

func (l LocalFS) RealPath(path string) (string, error) {
	return Realpath(path)
}

func (l LocalFS) Close() error {
	return nil
}

// RealPath canonicalizes a path name, including resolving ".." and
// following symlinks.
func Realpath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}
