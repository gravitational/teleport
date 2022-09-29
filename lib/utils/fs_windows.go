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

import (
	"os"
)

// On Windows we use auxiliary .lock files to acquire locks, so we can still read/write target files
// themselves. On unlock we delete the .lock file.
const lockPostfix = ".lock"

func getPlatformLockFilePath(path string) string {
	return path + lockPostfix
}

func unlockWrapper(unlockFn func() error, path string) func() error {
	return func() error {
		if unlockFn == nil {
			return nil
		}
		err := unlockFn()

		// At this point file can be locked again, and we can get an error, so we do our best effort
		// to remove .lock file, but can't guarantee it. Last locker should be able to successfully clean it.
		_ = os.Remove(path)
		return err
	}
}
