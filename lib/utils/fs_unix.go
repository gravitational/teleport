// +build !windows

/*
Copyright 2018 Gravitational, Inc.

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
	"os"
	"syscall"

	"github.com/gravitational/trace"
)

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
