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

package sftp

import (
	"context"
	"io"
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

func (l *localFS) Glob(ctx context.Context, pattern string) ([]string, error) {
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

func (l localFS) Open(ctx context.Context, path string) (fs.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &fileWrapper{file: f}, nil
}

func (l localFS) Create(ctx context.Context, path string, _ int64) (io.WriteCloser, error) {
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
		return trace.ConvertSystemError(err)
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

	return trace.ConvertSystemError(os.Chtimes(path, atime, mtime))
}
