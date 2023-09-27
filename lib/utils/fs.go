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
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
)

// ErrUnsuccessfulLockTry designates an error when we temporarily couldn't acquire lock
// (most probably it was already locked by someone else), another try might succeed.
var ErrUnsuccessfulLockTry = errors.New("could not acquire lock on the file at this time")

const (
	// FSLockRetryDelay is a delay between attempts to acquire lock.
	FSLockRetryDelay = 10 * time.Millisecond
)

// OpenFileWithFlagsFunc defines a function used to open files providing options.
type OpenFileWithFlagsFunc func(name string, flag int, perm os.FileMode) (*os.File, error)

// EnsureLocalPath makes sure the path exists, or, if omitted results in the subpath in
// default gravity config directory, e.g.
//
// EnsureLocalPath("/custom/myconfig", ".gravity", "config") -> /custom/myconfig
// EnsureLocalPath("", ".gravity", "config") -> ${HOME}/.gravity/config
//
// It also makes sure that base dir exists
func EnsureLocalPath(customPath string, defaultLocalDir, defaultLocalPath string) (string, error) {
	if customPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil || homeDir == "" {
			return "", trace.BadParameter("could not get user home dir: %v", err)
		}
		customPath = filepath.Join(homeDir, defaultLocalDir, defaultLocalPath)
	}
	baseDir := filepath.Dir(customPath)
	_, err := StatDir(baseDir)
	if err != nil {
		if trace.IsNotFound(err) {
			if err := os.MkdirAll(baseDir, teleport.PrivateDirMode); err != nil {
				return "", trace.ConvertSystemError(err)
			}
		} else {
			return "", trace.Wrap(err)
		}
	}
	return customPath, nil
}

// IsDir is a helper function to quickly check if a given path is a valid directory
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	if err == nil {
		return fi.IsDir()
	}
	return false
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

// StatFile stats path, returns error if it exists but a directory.
func StatFile(path string) (os.FileInfo, error) {
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
	return fi, nil
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

// FSTryWriteLock tries to grab write lock, returns ErrUnsuccessfulLockTry
// if lock is already acquired by someone else
func FSTryWriteLock(filePath string) (unlock func() error, err error) {
	fileLock := flock.New(getPlatformLockFilePath(filePath))
	locked, err := fileLock.TryLock()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if !locked {
		return nil, trace.Retry(ErrUnsuccessfulLockTry, "")
	}

	return fileLock.Unlock, nil
}

// FSTryWriteLockTimeout tries to grab write lock, it's doing it until locks is acquired, or timeout is expired,
// or context is expired.
func FSTryWriteLockTimeout(ctx context.Context, filePath string, timeout time.Duration) (unlock func() error, err error) {
	fileLock := flock.New(getPlatformLockFilePath(filePath))
	timedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := fileLock.TryLockContext(timedCtx, FSLockRetryDelay); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return fileLock.Unlock, nil
}

// FSTryReadLock tries to grab write lock, returns ErrUnsuccessfulLockTry
// if lock is already acquired by someone else
func FSTryReadLock(filePath string) (unlock func() error, err error) {
	fileLock := flock.New(getPlatformLockFilePath(filePath))
	locked, err := fileLock.TryRLock()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if !locked {
		return nil, trace.Retry(ErrUnsuccessfulLockTry, "")
	}

	return fileLock.Unlock, nil
}

// FSTryReadLockTimeout tries to grab read lock, it's doing it until locks is acquired, or timeout is expired,
// or context is expired.
func FSTryReadLockTimeout(ctx context.Context, filePath string, timeout time.Duration) (unlock func() error, err error) {
	fileLock := flock.New(getPlatformLockFilePath(filePath))
	timedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := fileLock.TryRLockContext(timedCtx, FSLockRetryDelay); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return fileLock.Unlock, nil
}

// RemoveSecure attempts to securely delete the file by first overwriting the file with random data three times
// followed by calling os.Remove(filePath).
func RemoveSecure(filePath string) error {
	for i := 0; i < 3; i++ {
		if err := overwriteFile(filePath); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.ConvertSystemError(os.Remove(filePath))
}

func overwriteFile(filePath string) (err error) {
	f, err := os.OpenFile(filePath, os.O_WRONLY, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			if err == nil {
				err = trace.ConvertSystemError(closeErr)
			} else {
				log.WithError(closeErr).Warningf("Failed to close %v.", f.Name())
			}
		}
	}()

	fi, err := f.Stat()
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	// Rounding up to 4k to hide the original file size. 4k was chosen because it's a common block size.
	const block = 4096
	size := fi.Size() / block * block
	if fi.Size()%block != 0 {
		size += block
	}

	_, err = io.CopyN(f, rand.Reader, size)
	return trace.Wrap(err)
}

// RemoveFileIfExist removes file if exits.
func RemoveFileIfExist(filePath string) error {
	if !FileExists(filePath) {
		return nil
	}
	if err := os.Remove(filePath); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}
