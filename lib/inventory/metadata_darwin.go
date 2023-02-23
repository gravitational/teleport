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
)

var matchOSVersion = regexp.MustCompile(`^macOS \d+\.\d+\.\d+$`)

// fetchOSVersion combines the output of 'sw_vers' to be e.g. "macOS 13.2.1".
func (c *fetchConfig) fetchOSVersion() string {
	command := "sw_vers"
	productName, err := c.exec(command, "-productName")
	if err != nil {
		return ""
	}

	productVersion, err := c.exec(command, "-productVersion")
	if err != nil {
		return ""
	}

	osVersion := fmt.Sprintf("%s %s", productName, productVersion)
	if !matchOSVersion.MatchString(osVersion) {
		return invalid(command, osVersion)
	}

	return osVersion
}

// fetchGlibcVersion returns "" on darwin.
func (c *fetchConfig) fetchGlibcVersion() string {
	return ""
}
