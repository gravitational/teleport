// +build !windows

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
