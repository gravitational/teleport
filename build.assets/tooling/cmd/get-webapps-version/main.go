// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command get-webapps-version determines the appropriate version
// of webapps to check out for a given version of teleport.
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println(webappsVersion(
		os.Getenv("DRONE_TAG"),
		os.Getenv("DRONE_TARGET_BRANCH"),
	))
}

func webappsVersion(tag, targetBranch string) string {
	// if this build was triggered from a tag on the
	// gravitational/teleport repo, assume that same
	// tag exists on gravitational/webapps
	if tag != "" {
		return tag
	}

	// if this build is on one of the teleport release branches,
	// map to the equivalent release branch in webapps
	if strings.HasPrefix(targetBranch, "branch/") {
		return "teleport-" + strings.TrimPrefix(targetBranch, "branch/")
	}

	// otherwise, this is a build on master, so just use master on webapps
	return "master"
}
