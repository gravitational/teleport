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

// fetchOSVersionInfo returns the content of '/etc/os-release'.
func (c *fetchConfig) fetchOSVersionInfo() string {
	out, err := c.read("/etc/os-release")
	if err != nil {
		return ""
	}

	return out
}

// fetchGlibcVersionInfo returns the output of 'ldd --version'.
func (c *fetchConfig) fetchGlibcVersionInfo() string {
	out, err := c.exec("ldd", "--version")
	if err != nil {
		return ""
	}

	return out
}
