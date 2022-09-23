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
type localFS struct {
	ctx context.Context
}

func (l *localFS) SetContext(ctx context.Context) {
	l.ctx = ctx
}

func (l *localFS) Type() string {
	return "local"
}

func (l *localFS) Stat(path string) (os.FileInfo, error) {
	if err := l.ctx.Err(); err != nil {
		return nil, err
	}

	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return fi, nil
}

func (l *localFS) ReadDir(path string) ([]os.FileInfo, error) {
	if err := l.ctx.Err(); err != nil {
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

func (l *localFS) Open(path string) (io.ReadCloser, error) {
	if err := l.ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

func (l *localFS) Create(path string, length uint64) (io.WriteCloser, error) {
	if err := l.ctx.Err(); err != nil {
		return nil, err
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return f, nil
}

func (l *localFS) Mkdir(path string) error {
	if err := l.ctx.Err(); err != nil {
		return err
	}

	// the permissions used here are somewhat arbitrary, they should
	// get modified after this is called
	err := os.MkdirAll(path, 0750)
	if err != nil && !os.IsExist(err) {
		return trace.ConvertSystemError(err)
	}

	return nil
}

func (l *localFS) Chmod(path string, mode os.FileMode) error {
	if err := l.ctx.Err(); err != nil {
		return err
	}

	return trace.Wrap(os.Chmod(path, mode))
}

func (l *localFS) Chtimes(path string, atime, mtime time.Time) error {
	return trace.ConvertSystemError(os.Chtimes(path, atime, mtime))
}
