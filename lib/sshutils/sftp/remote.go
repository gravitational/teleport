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

func runWithContext(ctx context.Context, f func() error) error {
	res := make(chan error)
	go func() {
		res <- f()
	}()
	select {
	case err := <-res:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runWithContext2[T any](ctx context.Context, f func() (T, error)) (T, error) {
	type result struct {
		t   T
		err error
	}
	res := make(chan result)
	go func() {
		t, err := f()
		res <- result{t: t, err: err}
	}()
	select {
	case r := <-res:
		return r.t, r.err
	case <-ctx.Done():
		var t T // get zero value
		return t, ctx.Err()
	}
}

// RemoteFS provides API for accessing the files on
// the local file system
type RemoteFS struct {
	c *sftp.Client
}

// NewRemoteFilesystem creates a new FileSystem over SFTP.
func NewRemoteFilesystem(c *sftp.Client) *RemoteFS {
	return &RemoteFS{c: c}
}

func (r *RemoteFS) Type() string {
	return "remote"
}

func (r *RemoteFS) Glob(ctx context.Context, pattern string) ([]string, error) {
	return runWithContext2(ctx, func() ([]string, error) {
		return r.c.Glob(pattern)
	})
}

func (r *RemoteFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	return runWithContext2(ctx, func() (os.FileInfo, error) {
		return r.c.Stat(path)
	})
}

func (r *RemoteFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	fileInfos, err := r.c.ReadDirContext(ctx, path)
	if err != nil {
		return nil, err
	}
	for i := range fileInfos {
		// if the file is a symlink, return the info of the linked file
		if fileInfos[i].Mode().Type()&os.ModeSymlink != 0 {
			fileInfos[i], err = r.Stat(ctx, portablepath.Join(path, fileInfos[i].Name()))
			if err != nil {
				return nil, err
			}
		}
	}

	return fileInfos, nil
}

func (r *RemoteFS) Open(ctx context.Context, path string) (File, error) {
	return r.OpenFile(ctx, path, os.O_RDONLY)
}

func (r *RemoteFS) Create(ctx context.Context, path string, _ int64) (File, error) {
	return r.OpenFile(ctx, path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

func (r *RemoteFS) OpenFile(ctx context.Context, path string, flags int) (File, error) {
	return runWithContext2(ctx, func() (File, error) {
		return r.c.OpenFile(path, flags)
	})
}

func (r *RemoteFS) Mkdir(ctx context.Context, path string) error {
	return runWithContext(ctx, func() error {
		return r.c.MkdirAll(path)
	})
}

func (r *RemoteFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	return runWithContext(ctx, func() error {
		return r.c.Chmod(path, mode)
	})
}

func (r *RemoteFS) Chtimes(ctx context.Context, path string, atime, mtime time.Time) error {
	return runWithContext(ctx, func() error {
		return r.c.Chtimes(path, atime, mtime)
	})
}

func (r *RemoteFS) Rename(ctx context.Context, oldpath, newpath string) error {
	return runWithContext(ctx, func() error {
		return r.c.Rename(oldpath, newpath)
	})
}

func (r *RemoteFS) Lstat(ctx context.Context, name string) (os.FileInfo, error) {
	return runWithContext2(ctx, func() (os.FileInfo, error) {
		return r.c.Lstat(name)
	})
}

func (r *RemoteFS) RemoveAll(ctx context.Context, path string) error {
	return runWithContext(ctx, func() error {
		return r.c.RemoveAll(path)
	})
}

func (r *RemoteFS) Link(ctx context.Context, oldname, newname string) error {
	return runWithContext(ctx, func() error {
		return r.c.Link(oldname, newname)
	})
}

func (r *RemoteFS) Symlink(ctx context.Context, oldname, newname string) error {
	return runWithContext(ctx, func() error {
		return r.c.Symlink(oldname, newname)
	})
}

func (r *RemoteFS) Remove(ctx context.Context, name string) error {
	return runWithContext(ctx, func() error {
		return r.c.Remove(name)
	})
}

func (r *RemoteFS) Chown(ctx context.Context, name string, uid, gid int) error {
	return runWithContext(ctx, func() error {
		return r.c.Chown(name, uid, gid)
	})
}

func (r *RemoteFS) Truncate(ctx context.Context, name string, size int64) error {
	return runWithContext(ctx, func() error {
		return r.c.Truncate(name, size)
	})
}

func (r *RemoteFS) Readlink(ctx context.Context, name string) (string, error) {
	return runWithContext2(ctx, func() (string, error) {
		return r.c.ReadLink(name)
	})
}

func (r *RemoteFS) Getwd(ctx context.Context) (string, error) {
	return runWithContext2(ctx, func() (string, error) {
		return r.c.Getwd()
	})
}
