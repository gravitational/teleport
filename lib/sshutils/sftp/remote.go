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
	"cmp"
	"context"
	"os"
	portablepath "path"
	"time"

	"github.com/gravitational/trace"
	"github.com/pkg/sftp"
)

func runWithContext(ctx context.Context, f func()) error {
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// remoteFS provides API for accessing the files on
// the local file system
type remoteFS struct {
	c *sftp.Client
}

// NewRemoteFilesystem creates a new FileSystem over SFTP.
func NewRemoteFilesystem(c *sftp.Client) FileSystem {
	return &remoteFS{c: c}
}

func (r *remoteFS) Type() string {
	return "remote"
}

func (r *remoteFS) Glob(ctx context.Context, pattern string) (matches []string, err error) {
	ctxErr := runWithContext(ctx, func() {
		matches, err = r.c.Glob(pattern)
	})
	return matches, cmp.Or(ctxErr, err)
}

func (r *remoteFS) Stat(ctx context.Context, path string) (fi os.FileInfo, err error) {
	ctxErr := runWithContext(ctx, func() {
		fi, err = r.c.Stat(path)
	})
	return fi, cmp.Or(ctxErr, err)
}

func (r *remoteFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
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

func (r *remoteFS) Open(ctx context.Context, path string) (File, error) {
	return r.OpenFile(ctx, path, os.O_RDONLY)
}

func (r *remoteFS) Create(ctx context.Context, path string, _ int64) (File, error) {
	return r.OpenFile(ctx, path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
}

func (r *remoteFS) OpenFile(ctx context.Context, path string, flags int) (f File, err error) {
	ctxErr := runWithContext(ctx, func() {
		f, err = r.c.OpenFile(path, flags)
	})
	return f, cmp.Or(ctxErr, err)
}

func (r *remoteFS) Mkdir(ctx context.Context, path string) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.MkdirAll(path)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Chmod(ctx context.Context, path string, mode os.FileMode) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Chmod(path, mode)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Chtimes(ctx context.Context, path string, atime, mtime time.Time) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Chtimes(path, atime, mtime)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Rename(ctx context.Context, oldpath, newpath string) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Rename(oldpath, newpath)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Lstat(ctx context.Context, name string) (fi os.FileInfo, err error) {
	ctxErr := runWithContext(ctx, func() {
		fi, err = r.c.Lstat(name)
	})
	return fi, cmp.Or(ctxErr, err)
}

func (r *remoteFS) RemoveAll(ctx context.Context, path string) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.RemoveAll(path)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Link(ctx context.Context, oldname, newname string) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Link(oldname, newname)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Symlink(ctx context.Context, oldname, newname string) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Symlink(oldname, newname)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Remove(ctx context.Context, name string) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Remove(name)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Chown(ctx context.Context, name string, uid, gid int) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Chown(name, uid, gid)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Truncate(ctx context.Context, name string, size int64) (err error) {
	ctxErr := runWithContext(ctx, func() {
		err = r.c.Truncate(name, size)
	})
	return cmp.Or(ctxErr, err)
}

func (r *remoteFS) Readlink(ctx context.Context, name string) (dest string, err error) {
	ctxErr := runWithContext(ctx, func() {
		dest, err = r.c.ReadLink(name)
	})
	return dest, cmp.Or(ctxErr, err)
}

func (r *remoteFS) Getwd(ctx context.Context) (wd string, err error) {
	ctxErr := runWithContext(ctx, func() {
		wd, err = r.c.Getwd()
	})
	return wd, cmp.Or(ctxErr, err)
}
