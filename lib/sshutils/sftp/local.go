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

package sftp

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
)

// localFS provides API for accessing the files on
// the local file system
type localFS struct{}

func (l localFS) Type() string {
	return "local"
}

func (l localFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

func (l localFS) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (l localFS) ReadDir(path string) ([]os.FileInfo, error) {
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

func (l localFS) Open(path string) (File, error) {

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &fileWrapper{File: f}, nil
}

func (l localFS) Create(path string, _ int64) (File, error) {
	return l.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

func (l localFS) OpenFile(path string, flags int) (File, error) {
	return os.OpenFile(path, flags, defaults.FilePermissions)
}

func (l localFS) Mkdir(path string) error {
	err := os.MkdirAll(path, defaults.DirectoryPermissions)
	if err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

func (l localFS) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func (l localFS) Chtimes(path string, atime, mtime time.Time) error {
	return os.Chtimes(path, atime, mtime)
}

func (l localFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (l localFS) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

func (l localFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (l localFS) Link(oldname, newname string) error {
	return os.Link(oldname, newname)
}

func (l localFS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (l localFS) Remove(name string) error {
	return os.Remove(name)
}

func (l localFS) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (l localFS) Truncate(name string, size int64) error {
	return os.Truncate(name, size)
}

func (l localFS) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (l localFS) Getwd() (string, error) {
	return os.Getwd()
}

func (l localFS) RealPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}
