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

package metadata

import (
	"fmt"
)

// fetchOSVersion returns something equivalent to the output of
// '$(sw_vers -productName) $(sw_vers -productVersion)'.
func (c *fetchConfig) fetchOSVersion() string {
	productName, err := c.exec("sw_vers", "-productName")
	if err != nil {
		return ""
	}

	productVersion, err := c.exec("sw_vers", "-productVersion")
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s %s", productName, productVersion)
}

// fetchGlibcVersion returns "" on darwin.
func (c *fetchConfig) fetchGlibcVersion() string {
	return ""
}
