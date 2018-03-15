/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/gravitational/trace"
)

// RemoveDirCloser removes directory and all it's contents
// when Close is called
type RemoveDirCloser struct {
	Path string
}

// Close removes directory and all it's contents
func (r *RemoveDirCloser) Close() error {
	return trace.ConvertSystemError(os.RemoveAll(r.Path))
}

// IsFile returns true if a given file path points to an existing file
func IsFile(fp string) bool {
	fi, err := os.Stat(fp)
	if err == nil {
		return !fi.IsDir()
	}
	return false
}

// IsDir is a helper function to quickly check if a given path is a valid directory
func IsDir(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err == nil {
		return fi.IsDir()
	}
	return false
}

// ReadAll is similarl to ioutil.ReadAll, except it doesn't use ever-increasing
// internal buffer, instead asking for the exact buffer size.
//
// This is useful when you want to limit the sze of Read/Writes (websockets)
func ReadAll(r io.Reader, bufsize int) (out []byte, err error) {
	buff := make([]byte, bufsize)
	n := 0
	for err == nil {
		n, err = r.Read(buff)
		if n > 0 {
			out = append(out, buff[:n]...)
		}
	}
	if err == io.EOF {
		err = nil
	}
	return out, err
}

// NormalizePath normalises path, evaluating symlinks and converting local
// paths to absolute
func NormalizePath(path string) (string, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return abs, nil
}

// OpenFile opens  file and returns file handle
func OpenFile(path string) (*os.File, error) {
	newPath, err := NormalizePath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fi, err := os.Stat(newPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if fi.IsDir() {
		return nil, trace.BadParameter("%v is not a file", path)
	}
	f, err := os.Open(newPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

// StatDir stats directory, returns error if file exists, but not a directory
func StatDir(path string) (os.FileInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if !fi.IsDir() {
		return nil, trace.BadParameter("%v is not a directory", path)
	}
	return fi, nil
}

// FSWriteLock grabs Flock-style filesystem lock on an open file
// in exclusive mode.
func FSWriteLock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// FSTryWriteLock tries to grab write lock, returns CompareFailed
// if lock is already grabbed
func FSTryWriteLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			return trace.CompareFailed("lock %v is acquired by another process", f.Name())
		}
		return trace.ConvertSystemError(err)
	}
	return nil
}

// FSReadLock grabs Flock-style filesystem lock on an open file
// in read (shared) mode
func FSReadLock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// FSUnlock unlcocks Flock-style filesystem lock
func FSUnlock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}
