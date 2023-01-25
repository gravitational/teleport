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
	"os"
	"time"

	"github.com/gravitational/trace"
)

// localFS provides API for accessing the files on
// the local file system
type localFS struct{}

func (l localFS) Type() string {
	return "local"
}

func (l localFS) Stat(ctx context.Context, path string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return fi, nil
}

func (l localFS) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// normally os.ReadDir would be used as it's potentially more efficient,
	// but because we want os.FileInfos of every file this is easier
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	fileInfos, err := f.Readdir(-1)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return fileInfos, nil
}

func (l localFS) Open(ctx context.Context, path string) (WriterToCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &WT{file: f}, nil
}

type WT struct {
	file *os.File
}

func (wt *WT) Read(p []byte) (n int, err error) {
	return wt.file.Read(p)
}

func (wt *WT) Close() error {
	return wt.file.Close()
}

func (wt *WT) WriteTo(w io.Writer) (n int64, err error) {
	return io.Copy(w, wt.file)
}

func (wt *WT) Stat() (os.FileInfo, error) {
	return wt.file.Stat()
}

func (l localFS) Create(ctx context.Context, path string, mode os.FileMode) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

func (l localFS) Mkdir(ctx context.Context, path string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := os.MkdirAll(path, mode)
	if err != nil && !os.IsExist(err) {
		return trace.ConvertSystemError(err)
	}

	return nil
}

func (l localFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return trace.Wrap(os.Chmod(path, mode))
}

func (l localFS) Chtimes(ctx context.Context, path string, atime, mtime time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return trace.ConvertSystemError(os.Chtimes(path, atime, mtime))
}
