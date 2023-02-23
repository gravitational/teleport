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

package inventory

import (
	"fmt"
	"regexp"
	"strings"
)

var matchOSVersion = regexp.MustCompile(`^[\w\s\.\/]+$`)
var matchGlibcVersion = regexp.MustCompile(`^\d+\.\d+$`)

// fetchOSVersion combines the content of '/etc/os-release' to be e.g.
// "Ubuntu 22.04".
func (c *fetchConfig) fetchOSVersion() string {
	filename := "/etc/os-release"
	out, err := c.read(filename)
	if err != nil {
		return ""
	}

	var name string
	var versionID string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			return invalid(filename, out)
		}

		switch parts[0] {
		case "NAME":
			name = strings.Trim(parts[1], `"`)
		case "VERSION_ID":
			versionID = strings.Trim(parts[1], `"`)
		}
	}

	if name == "" || versionID == "" {
		return invalid(filename, out)
	}

	osVersion := fmt.Sprintf("%s %s", name, versionID)
	if !matchOSVersion.MatchString(osVersion) {
		return invalid(filename, out)
	}

	return osVersion
}

// fetchGlibcVersion parses the output of 'ldd --version' and returns e.g.
// "2.31".
func (c *fetchConfig) fetchGlibcVersion() string {
	cmd := "ldd"
	out, err := c.exec(cmd, "--version")
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return invalid(cmd, out)
	}

	parts := strings.Fields(lines[0])
	glibcVersion := parts[len(parts)-1]
	if !matchGlibcVersion.MatchString(glibcVersion) {
		return invalid(cmd, out)
	}

	return glibcVersion
}
