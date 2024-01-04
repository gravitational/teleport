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

// Command version-check that outputs the version of kubectl used by the build.
// It doesn't live in the build.assets/tooling/ directory because it needs to
// be built with the go.mod file from the teleport root directory.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	// Import the kubectl module to get the version
	// This is a hack to get the version of kubectl
	// without having to parse the go.mod file.
	_ "k8s.io/kubectl"
)

func main() {
	version, ok := getKubectlModVersion()
	if !ok {
		fmt.Println("kubectl version not found")
		os.Exit(1)
	}
	fmt.Println(version)
}

// getKubectlModVersion returns the version of the kubectl module
// and replaces the v0 prefix with v1.
// This is a hack to get the version of kubectl
// without having to parse the go.mod file.
func getKubectlModVersion() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	for _, dep := range info.Deps {
		if dep.Path == "k8s.io/kubectl" {
			return strings.Replace(dep.Version, "v0.", "v1.", 1), true
		}
	}
	return "", false
}
