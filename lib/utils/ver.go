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

package utils

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// MeetsMinVersion returns true if gotVer is empty or at least minVer.
func MeetsMinVersion(gotVer, minVer string) bool {
	if gotVer == "" {
		return true // Ignore empty versions.
	}

	err := CheckMinVersion(gotVer, minVer)

	// Non BadParameter errors are semver parsing errors.
	return !trace.IsBadParameter(err)
}

// MeetsMaxVersion returns true if gotVer is empty or at most maxVer.
func MeetsMaxVersion(gotVer, maxVer string) bool {
	if gotVer == "" {
		return true // Ignore empty versions.
	}

	err := CheckMaxVersion(gotVer, maxVer)

	// Non BadParameter errors are semver parsing errors.
	return !trace.IsBadParameter(err)
}

// CheckMinVersion compares a version with a minimum version supported.
func CheckMinVersion(currentVersion, minVersion string) error {
	currentSemver, minSemver, err := versionStringToSemver(currentVersion, minVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	if currentSemver.LessThan(*minSemver) {
		return trace.BadParameter("incompatible versions: %v < %v", currentVersion, minVersion)
	}

	return nil
}

// CheckMaxVersion compares a version with a maximum version supported.
func CheckMaxVersion(currentVersion, maxVersion string) error {
	currentSemver, maxSemver, err := versionStringToSemver(currentVersion, maxVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	if maxSemver.LessThan(*currentSemver) {
		return trace.BadParameter("incompatible versions: %v > %v", currentVersion, maxVersion)
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

// MajorSemver returns the major version as a semver string.
// Ex: 13.4.3 -> 13.0.0
func MajorSemver(version string) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return fmt.Sprintf("%d.0.0", ver.Major), nil
}

// MajorSemverWithWildcards returns the major version as a semver string.
// Ex: 13.4.3 -> 13.x.x
func MajorSemverWithWildcards(version string) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return fmt.Sprintf("%d.x.x", ver.Major), nil
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
