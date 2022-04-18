/*
Copyright 2019 Gravitational, Inc.

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
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
)

// KernelVersion parses /proc/sys/kernel/osrelease and returns the kernel
// version of the host. This only returns something meaningful on Linux.
func KernelVersion() (*semver.Version, error) {
	if runtime.GOOS != constants.LinuxOS {
		return nil, trace.BadParameter("requested kernel version on non-Linux host")
	}

	file, err := os.Open("/proc/sys/kernel/osrelease")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()

	ver, err := kernelVersion(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ver, nil
}

// kernelVersion reads in the kernel version from the reader and returns
// a *semver.Version.
func kernelVersion(reader io.Reader) (*semver.Version, error) {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Only keep the major, minor, and patch, throw away everything after "-".
	parts := bytes.Split(buf, []byte("-"))
	s := strings.TrimSpace(string(parts[0]))

	ver, err := semver.NewVersion(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ver, nil
}

const btfFile = "/sys/kernel/btf/vmlinux"

// HasBTF checks that the kernel has been compiled with BTF support and
// that the type information can be opened. Returns nil if BTF is there
// and accessible, otherwise an error describing the problem.
func HasBTF() error {
	if runtime.GOOS != constants.LinuxOS {
		return trace.BadParameter("requested kernel version on non-Linux host")
	}

	file, err := os.Open(btfFile)
	if err == nil {
		file.Close()
		return nil
	}

	if os.IsNotExist(err) {
		return fmt.Errorf("%v was not found. Make sure the kernel was compiled with BTF support (CONFIG_DEBUG_INFO_BTF)", btfFile)
	}

	return fmt.Errorf("failed to open %v: %v", btfFile, err)
}
