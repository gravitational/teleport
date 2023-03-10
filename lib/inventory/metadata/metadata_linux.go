//go:build linux
// +build linux

/*
Copyright 2023 Gravitational, Inc.

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

package metadata

// #include <gnu/libc-version.h>
import "C"

import (
	"fmt"
	"strings"
)

// fetchOSVersion combines the content of '/etc/os-release' to be e.g.
// "ubuntu 22.04".
func (c *fetchConfig) fetchOSVersion() string {
	filename := "/etc/os-release"
	out, err := c.read(filename)
	if err != nil {
		return ""
	}

	id := "linux"
	versionID := "(unknown)"
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		switch key {
		case "ID":
			id = strings.Trim(value, `"`)
		case "VERSION_ID":
			versionID = strings.Trim(value, `"`)
		}
	}

	return fmt.Sprintf("%s %s", id, versionID)
}

// fetchGlibcVersion returns the glibc version string as returned by
// gnu_get_libc_version.
func (c *fetchConfig) fetchGlibcVersion() string {
	return C.GoString(C.gnu_get_libc_version())
}
