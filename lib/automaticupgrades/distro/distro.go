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

package distro

import (
	"os"
)

const (
	// DebianDistro specifies a Debian distribution of Linux
	DebianDistro = "debian"
)

// Distribution returns the current Linux distro
func Distribution() string {
	switch {
	case exists("/etc/debian_version"):
		return DebianDistro
	default:
		return ""
	}
}

// exists returns true if the specified file exists
func exists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}
