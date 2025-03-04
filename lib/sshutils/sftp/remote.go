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
	"os"
	portablepath "path"
	"time"

	"github.com/pkg/sftp"
)

// remoteFS provides API for accessing the files on
// the local file system
type remoteFS struct {
	c *sftp.Client
}

func (r *remoteFS) Type() string {
	return "remote"
}

func (r *remoteFS) Glob(ctx context.Context, pattern string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	matches, err := r.c.Glob(pattern)
	if err != nil {
		return nil, err
	}

	return matches, nil
}

func (r *remoteFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fi, err := r.c.Stat(path)
	if err != nil {
		return nil, err
	}

	return fi, nil
}

func (r *remoteFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fileInfos, err := r.c.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for i := range fileInfos {
		// if the file is a symlink, return the info of the linked file
		if fileInfos[i].Mode().Type()&os.ModeSymlink != 0 {
			fileInfos[i], err = r.c.Stat(portablepath.Join(path, fileInfos[i].Name()))
			if err != nil {
				return nil, err
			}
		}
	}

	return fileInfos, nil
}

func (r *remoteFS) Open(ctx context.Context, path string) (File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := r.c.Open(path)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (r *remoteFS) Create(ctx context.Context, path string, _ int64) (File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := r.c.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (r *remoteFS) Mkdir(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.c.MkdirAll(path)
	if err != nil {
		return err
	}

	return nil
}

func (r *remoteFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return r.c.Chmod(path, mode)
}

func (r *remoteFS) Chtimes(ctx context.Context, path string, atime, mtime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return r.c.Chtimes(path, atime, mtime)
}

func (r *remoteFS) Rename(ctx context.Context, oldpath, newpath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.c.Rename(oldpath, newpath)
}

func (r *remoteFS) Lstat(ctx context.Context, name string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fi, err := r.c.Lstat(name)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

func (r *remoteFS) RemoveAll(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.c.RemoveAll(path)
}

func (r *remoteFS) Link(ctx context.Context, oldname, newname string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.c.Link(oldname, newname)
}

func (r *remoteFS) Symlink(ctx context.Context, oldname, newname string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.c.Symlink(oldname, newname)
}

func (r *remoteFS) Remove(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.c.Remove(name)
}

func (r *remoteFS) Chown(ctx context.Context, name string, uid, gid int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.c.Chown(name, uid, gid)
}

func (r *remoteFS) Truncate(ctx context.Context, name string, size int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.c.Truncate(name, size)
}

func (r *remoteFS) Readlink(ctx context.Context, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	dest, err := r.c.ReadLink(name)
	if err != nil {
		return "", err
	}
	return dest, nil
}
