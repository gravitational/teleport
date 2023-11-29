//go:build darwin

// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package socketpair

import (
	"syscall"

	"github.com/gravitational/trace"
)

// cloexecSocketpair returns a unix/local stream socketpair whose file
// descriptors are flagged close-on-exec. This implementation acquires
// [syscall.ForkLock] as it creates the socketpair and sets the two file
// descriptors close-on-exec.
func cloexecSocketpair() (uintptr, uintptr, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}

	syscall.CloseOnExec(fds[0])
	syscall.CloseOnExec(fds[1])

	return uintptr(fds[0]), uintptr(fds[1]), nil
}
