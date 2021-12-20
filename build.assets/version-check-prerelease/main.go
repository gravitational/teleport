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
// Command version-check-prerelease exits non-zero when given a
// git tag that is a prerelease. This allows us to avoid publishing
// releases for internal builds.
package main

import (
	"flag"
	"log"
	"strings"

	"github.com/gravitational/trace"
)

func main() {
	tag, err := parseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags; %v.", err)
	}

	if err := check(tag); err != nil {
		log.Fatalf("Check failed: %v.", err)
	}
}

func parseFlags() (string, error) {
	tag := flag.String("tag", "", "tag to validate")
	flag.Parse()

	if *tag == "" {
		return "", trace.BadParameter("tag missing")
	}
	return *tag, nil
}

func check(tag string) error {
	if strings.Contains(tag, "-") { // https://semver.org/#spec-item-9
		return trace.BadParameter("version is pre-release: %v", tag)
	}
	if strings.Contains(tag, "+") { // https://semver.org/#spec-item-10
		return trace.BadParameter("version contains build metadata: %v", tag)
	}
	return nil
}
