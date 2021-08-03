/*
Copyright 2018 Gravitational, Inc.

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

package utils

import (
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// CheckVersion compares a version with a minimum version supported.
// This is used for comparing server-client compatibility in both direction.
func CheckVersion(currentVersion, minVersion string) error {
	currentSemver, err := semver.NewVersion(currentVersion)
	if err != nil {
		return trace.Wrap(err, "unsupported version format, need semver format: %q, e.g 1.0.0", currentVersion)
	}

	minSemver, err := semver.NewVersion(minVersion)
	if err != nil {
		return trace.Wrap(err, "unsupported version format, need semver format: %q, e.g 1.0.0", minVersion)
	}

	if currentSemver.LessThan(*minSemver) {
		return trace.BadParameter("incompatible versions: %v < %v", currentVersion, minVersion)
	}

	return nil
}

// SetVersionBeforeAlpha appends "-aa" to the version so that it comes before <version>-alpha.
// This ban be used to make version checks work during development.
func SetVersionBeforeAlpha(version string) string {
	return version + "-aa"
}
