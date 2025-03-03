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
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// localFS provides API for accessing the files on
// the local file system
type localFS struct{}

func (l localFS) Type() string {
	return "local"
}

func (l localFS) Glob(ctx context.Context, pattern string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return matches, nil
}

func (l localFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return fi, nil
}

func (l localFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fileInfos := make([]fs.FileInfo, len(entries))
	for i, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// if the file is a symlink, return the info of the linked file
		if info.Mode().Type()&os.ModeSymlink != 0 {
			info, err = os.Stat(filepath.Join(path, info.Name()))
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

		fileInfos[i] = info
	}

	return fileInfos, nil
}

func (l localFS) Open(ctx context.Context, path string) (File, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &fileWrapper{File: f}, nil
}

func (l localFS) Create(ctx context.Context, path string, _ int64) (File, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.FilePermissions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

func (l localFS) Mkdir(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}

	err := os.MkdirAll(path, defaults.DirectoryPermissions)
	if err != nil && !os.IsExist(err) {
		return trace.Wrap(err)
	}

	return nil
}

func (l localFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.Chmod(path, mode))
}

func (l localFS) Chtimes(ctx context.Context, path string, atime, mtime time.Time) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.Chtimes(path, atime, mtime))
}

func (l localFS) Rename(ctx context.Context, oldpath, newpath string) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.Rename(oldpath, newpath))
}

func (l localFS) Lstat(ctx context.Context, name string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	fi, err := os.Lstat(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fi, nil
}

func (l localFS) RemoveAll(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.RemoveAll(path))
}

func (l localFS) Link(ctx context.Context, oldname, newname string) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.Link(oldname, newname))
}

func (l localFS) Symlink(ctx context.Context, oldname, newname string) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.Symlink(oldname, newname))
}

func (l localFS) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.Remove(name))
}

func (l localFS) Chown(ctx context.Context, name string, uid, gid int) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.Chown(name, uid, gid))
}

func (l localFS) Truncate(ctx context.Context, name string, size int64) error {
	if err := ctx.Err(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.Truncate(name, size))
}

func (l localFS) Readlink(ctx context.Context, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", trace.Wrap(err)
	}
	dest, err := os.Readlink(name)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return dest, nil
}
