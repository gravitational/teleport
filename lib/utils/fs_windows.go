//go:build windows
// +build windows

package utils

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

// On Windows we use auxiliary .lock.tmp files to acquire locks, so we can still read/write target
// files themselves.
//
// .lock.tmp files are deliberately not cleaned up. Their presence doesn't matter to the actual
// locking. Repeatedly removing them on unlock when acquiring dozens of locks in a short timespan
// was causing flock.Flock.TryRLock to return either "access denied" or "The process cannot access
// the file because it is being used by another process".

import "strings"

const lockPostfix = ".lock.tmp"

func getPlatformLockFilePath(path string) string {
	// If target file is itself dedicated lockfile, we don't create another lockfile, since
	// we don't intend to read/write the target file itself.
	if strings.HasSuffix(path, ".lock") {
		return path
	}
	return path + lockPostfix
}
