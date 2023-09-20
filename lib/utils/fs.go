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
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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
func NormalizePath(path string, evaluateSymlinks bool) (string, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	if evaluateSymlinks {
		s, err = filepath.EvalSymlinks(s)
		if err != nil {
			return "", trace.ConvertSystemError(err)
		}
	}
	return s, nil
}

// OpenFileAllowingUnsafeLinks opens a file, if the path includes a symlink, the returned os.File will be resolved to
// the actual file.  This will return an error if the file is not found or is a directory.
func OpenFileAllowingUnsafeLinks(path string) (*os.File, error) {
	return openFile(path, true /* allowSymlink */, true /* allowMultipleHardlinks */)
}

// OpenFileNoUnsafeLinks opens a file, ensuring it's an actual file and not a directory or symlink.  Depending on
// the os, it may also prevent hardlinks.  This is important because MacOS allows hardlinks without validating write
// permissions (similar to a symlink in that regard).
func OpenFileNoUnsafeLinks(path string) (*os.File, error) {
	return openFile(path, false /* allowSymlink */, runtime.GOOS != "darwin" /* allowMultipleHardlinks */)
}

func openFile(path string, allowSymlink, allowMultipleHardlinks bool) (*os.File, error) {
	newPath, err := NormalizePath(path, allowSymlink)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var fi os.FileInfo
	if allowSymlink {
		fi, err = os.Stat(newPath)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	} else {
		components := strings.Split(newPath, string(os.PathSeparator))
		var subPath string
		for _, p := range components {
			subPath = filepath.Join(subPath, p)
			if subPath == "" {
				subPath = string(os.PathSeparator)
			}

			fi, err = os.Lstat(subPath)
			if err != nil {
				return nil, trace.ConvertSystemError(err)
			} else if fi.Mode().Type()&os.ModeSymlink != 0 {
				return nil, trace.BadParameter("opening file %s, symlink not allowed in path: %s", path, subPath)
			}
		}
	}
	if !allowMultipleHardlinks {
		// hardlinks can only exist at the end file, not for directories within the path
		if linkCount, ok := getHardLinkCount(fi); ok && linkCount > 1 {
			return nil, trace.BadParameter("file has hardlink count greater than 1: %s", path)
		}
	}
	if fi.IsDir() {
		return nil, trace.BadParameter("%s is not a file", path)
	}
	f, err := os.Open(newPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

// StatFile stats path, returns error if it exists but a directory.
func StatFile(path string) (os.FileInfo, error) {
	newPath, err := NormalizePath(path, true)
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

// RemoveAllSecure is similar to `os.RemoveAll` however will delegate to the `RemoveSecure` implementation below
func RemoveAllSecure(path string) error {
	// Match os.RemoveAll protections in not permitting removal of "." directories
	if path == "." || (len(path) >= 2 && path[len(path)-1] == '.' && os.IsPathSeparator(path[len(path)-2])) {
		return &os.PathError{Op: "RemoveAllSecure", Path: path, Err: syscall.EINVAL} // error type matches os.RemoveAll
	}

	if info, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return trace.ConvertSystemError(err)
		}
	} else if info.IsDir() {
		var removeErrors []error
		if files, err := os.ReadDir(path); err != nil {
			// don't fail fast, allow removal at end to be attempted
			removeErrors = []error{err}
		} else {
			for _, f := range files {
				if err := RemoveAllSecure(filepath.Join(path, f.Name())); err != nil {
					removeErrors = append(removeErrors, err)
				}
			}
		}
		if err := os.Remove(path); err != nil {
			removeErrors = append(removeErrors, err)
		}
		if len(removeErrors) > 0 {
			if len(removeErrors) == 1 {
				return trace.ConvertSystemError(removeErrors[0])
			} else {
				return fmt.Errorf("multiple errors in directory removal: %v", removeErrors)
			}
		}
		return nil
	} else { // file or symlink
		return RemoveSecure(path)
	}
}

// RemoveSecure attempts to securely delete the file by first overwriting the file with random data three times
// followed by calling os.Remove(filePath).
func RemoveSecure(filePath string) error {
	var overwriteErr error
	for i := 0; i < 3; i++ {
		if err := overwriteFile(filePath); err != nil {
			if overwriteErr == nil {
				overwriteErr = err
			}
		}
	}
	// regardless of above errors we want to attempt a removal of the file
	removeErr := os.Remove(filePath)
	if overwriteErr == nil {
		return trace.ConvertSystemError(removeErr)
	} else if removeErr == nil {
		return trace.ConvertSystemError(overwriteErr)
	} else {
		return trace.Errorf("multiple errors on removal: %v - %v", overwriteErr, removeErr)
	}
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

func RecursiveChown(dir string, uid, gid int) error {
	if err := os.Chown(dir, uid, gid); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		err = os.Chown(path, uid, gid)
		if os.IsNotExist(err) { // empty symlinks cause an error here
			return nil
		}
		return trace.Wrap(err)
	}))
}

func CopyFile(src, dest string, perm os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	destFile, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	_, err = destFile.ReadFrom(srcFile)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// RecursivelyCopy will copy a directory from src to dest, if the
// directory exists, files will be overwritten. The skip paramater, if
// provided, will be passed the source and destination paths, and will
// skip files upon returning true
func RecursiveCopy(src, dest string, skip func(src, dest string) (bool, error)) error {
	return trace.Wrap(fs.WalkDir(os.DirFS(src), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}

		absSrcPath := filepath.Join(src, path)
		destPath := filepath.Join(dest, path)
		info, err := d.Info()
		if err != nil {
			return trace.Wrap(err)
		}
		originalPerm := info.Mode().Perm()

		if skip != nil {
			doSkip, err := skip(absSrcPath, destPath)
			if err != nil {
				return trace.Wrap(err)
			}
			if doSkip {
				return nil
			}
		}

		if d.IsDir() {
			err := os.Mkdir(destPath, originalPerm)
			if os.IsExist(err) {
				return nil
			}
			return trace.ConvertSystemError(err)
		}

		if d.Type().IsRegular() {
			if err := CopyFile(absSrcPath, destPath, originalPerm); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}

		if info.Mode().Type()&os.ModeSymlink != 0 {
			linkDest, err := os.Readlink(absSrcPath)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			if err := os.Symlink(linkDest, destPath); err != nil {
				return trace.ConvertSystemError(err)
			}
			return nil
		}

		return nil
	}))
}
