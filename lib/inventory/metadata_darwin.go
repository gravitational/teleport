//go:build darwin
// +build darwin

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

var matchProductVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// fetchOSVersion combines the output of 'sw_vers' to be e.g. "macOS 13.2.1".
func (c *fetchConfig) fetchOSVersion() string {
	return c.cmd("sw_vers", func(out string) (string, bool) {
		var productName string
		var productVersion string

		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				return "", false
			}

			switch parts[0] {
			case "ProductName":
				productName = strings.TrimSpace(parts[1])
			case "ProductVersion":
				productVersion = strings.TrimSpace(parts[1])
			}
		}

		if productName != "macOS" || !matchProductVersion.MatchString(productVersion) {
			return "", false
		}

		return fmt.Sprintf("%s %s", productName, productVersion), true
	})
}

// fetchGlibcVersion returns "" on darwin.
func (c *fetchConfig) fetchGlibcVersion() string {
	return ""
}
