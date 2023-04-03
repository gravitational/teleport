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

var MinIPPropagationVersion = semver.New(VersionBeforeAlpha("12.2.0")).String()

// CheckVersion compares a version with a minimum version supported.
func CheckVersion(currentVersion, minVersion string) error {
	currentSemver, minSemver, err := versionStringToSemver(currentVersion, minVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	if currentSemver.LessThan(*minSemver) {
		return trace.BadParameter("incompatible versions: %v < %v", currentVersion, minVersion)
	}

	return nil
}

// VersionBeforeAlpha appends "-aa" to the version so that it comes before <version>-alpha.
// This ban be used to make version checks work during development.
func VersionBeforeAlpha(version string) string {
	return version + "-aa"
}

// MinVerWithoutPreRelease compares semver strings, but skips prerelease. This allows to compare
// two versions and ignore dev,alpha,beta, etc. strings.
func MinVerWithoutPreRelease(currentVersion, minVersion string) (bool, error) {
	currentSemver, minSemver, err := versionStringToSemver(currentVersion, minVersion)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Erase pre-release string, so only version is compared.
	currentSemver.PreRelease = ""
	minSemver.PreRelease = ""

	return !currentSemver.LessThan(*minSemver), nil
}

func versionStringToSemver(ver1, ver2 string) (*semver.Version, *semver.Version, error) {
	v1Semver, err := semver.NewVersion(ver1)
	if err != nil {
		return nil, nil, trace.Wrap(err, "unsupported version format, need semver format: %q, e.g 1.0.0", v1Semver)
	}

	v2Semver, err := semver.NewVersion(ver2)
	if err != nil {
		return nil, nil, trace.Wrap(err, "unsupported version format, need semver format: %q, e.g 1.0.0", v2Semver)
	}

	return v1Semver, v2Semver, nil
}
