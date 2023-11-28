/*
Copyright 2022 Gravitational, Inc.

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
