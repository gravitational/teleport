package utils

import "os"

// For the moment, these functions are no-ops that should be replaced with a
// native implementation in the future.
func FSWriteLock(f *os.File) error {
  return nil
}

func FSTryWriteLock(f *os.File) error {
	return nil
}

func FSReadLock(f *os.File) error {
	return nil
}

func FSUnlock(f *os.File) error {
	return nil
}
