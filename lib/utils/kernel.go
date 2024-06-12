/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// KernelVersion parses /proc/sys/kernel/osrelease and returns the kernel
// version of the host. This only returns something meaningful on Linux.
func KernelVersion() (*semver.Version, error) {
	if runtime.GOOS != constants.LinuxOS {
		return nil, trace.BadParameter("requested kernel version on non-Linux host")
	}

	file, err := OpenFileNoUnsafeLinks("/proc/sys/kernel/osrelease")
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

// kernelVersionRegex extracts the first three digits of a version from
// a kernel version - this strips off any additional digits or additional
// information appended to the kernel version e.g:
// 5.15.68.1-microsoft-standard-WSL2 => 5.15.68
var kernelVersionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+`)

// kernelVersion reads in the kernel version from the reader and returns
// a *semver.Version.
func kernelVersion(reader io.Reader) (*semver.Version, error) {
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := kernelVersionRegex.FindString(string(buf))
	if s == "" {
		return nil, trace.BadParameter(
			"unable to extract kernel semver from string %q",
			string(buf),
		)
	}
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

	file, err := OpenFileNoUnsafeLinks(btfFile)
	if err == nil {
		file.Close()
		return nil
	}

	if os.IsNotExist(err) {
		return fmt.Errorf("%v was not found. Make sure the kernel was compiled with BTF support (CONFIG_DEBUG_INFO_BTF)", btfFile)
	}

	return fmt.Errorf("failed to open %v: %v", btfFile, err)
}
