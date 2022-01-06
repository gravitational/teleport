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

// Command print-api-module-version prints the version that should appear
// in Go import paths to stdout. The version will be empty for API major
// versions 0 or 1, and "/vX" for major versions greater than 1.
package main

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/teleport/api"
)

// printVersion writes the version that should appear in Go import
// paths to standard out
func printVersion(v string) {
	if ver := semver.New(v); ver.Major >= 2 {
		fmt.Printf("/v%d", ver.Major)
	}
}

func main() {
	sv := semver.New(api.Version)
	if sv.PreRelease != "" {
		return
	}
	if sv.Major >= 2 {
		fmt.Printf("/v%d", sv.Major)
	}
}
