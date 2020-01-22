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
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
)

// KernelVersion parses /proc/sys/kernel/osrelease and returns the kernel
// version of the host. This only returns something meaningful on Linux.
func KernelVersion() (*semver.Version, error) {
	if runtime.GOOS != teleport.LinuxOS {
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
	buf, err := ioutil.ReadAll(reader)
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
