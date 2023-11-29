//go:build unix && !darwin

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

package utils

import (
	"syscall"

	"github.com/gravitational/trace"
)

// cloexecSocketpair returns a unix/local stream socketpair whose file
// descriptors are flagged close-on-exec. This implementation creates the
// socketpair directly in close-on-exec mode.
func cloexecSocketpair() (uintptr, uintptr, error) {
	// SOCK_CLOEXEC on socketpair is supported since Linux 2.6.27 and go's
	// minimum requirement is 2.6.32 (FreeBSD supports it since FreeBSD 10 and
	// go 1.20+ requires FreeBSD 12)
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}

	return uintptr(fds[0]), uintptr(fds[1]), nil
}
