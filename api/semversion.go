// Copyright 2025 Gravitational, Inc.
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

package api

import (
	"errors"
	"slices"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// SemVer returns the version of Teleport in use as a [semver.Version].
func SemVer() *semver.Version {
	return &semver.Version{
		Major:      VersionMajor,
		Minor:      VersionMinor,
		Patch:      VersionPatch,
		PreRelease: VersionPreRelease,
		Metadata:   VersionMetadata,
	}
}

// SSHVersionPrefix is the prefix for the SSH client version string used by Teleport SSH clients.
const SSHVersionPrefix = "SSH-2.0-Teleport_"

// SSHClientVersion returns the SSH client identification string used by Teleport SSH clients.
//
// Format: SSH-2.0-Teleport_<teleport_version> <features>
//
// Teleport version is used for telemetry and debugging purposes.
//
// Features are comma-separated flags indicated supported features of the client:
// - mfav1: Client supports in-band MFA (RFD 234).
func SSHClientVersion() string {
	return SSHVersionPrefix + Version + " " + "mfav1"
}

// ErrNonTeleportSSHVersion is returned by ParseSSHClientVersion when the provided SSH client version
// string does not have the expected Teleport prefix. The client is either not a Teleport client or is an older Teleport
// version that did not set a client version string.
var ErrNonTeleportSSHVersion = errors.New("SSH client version is not a Teleport version")

// ParseSSHClientVersion parses the given SSH client version string and extracts the Teleport version and supported
// features.
//
// It returns the parsed Teleport version as a [semver.Version], a slice of supported features, and an error if the
// version string is malformed.
func ParseSSHClientVersion(clientVersion string) (*semver.Version, []string, error) {
	rest, ok := strings.CutPrefix(clientVersion, SSHVersionPrefix)
	if !ok {
		return nil, nil, ErrNonTeleportSSHVersion
	}

	if rest == "" {
		return nil, nil, trace.BadParameter("invalid version %q: missing Teleport version", clientVersion)
	}

	if strings.HasPrefix(rest, " ") ||
		strings.HasSuffix(rest, " ") ||
		strings.Contains(rest, "  ") {
		return nil, nil, trace.BadParameter("invalid version %q: unexpected whitespace", clientVersion)
	}

	versionPart, featuresPart, hasFeatures := strings.Cut(rest, " ")
	if hasFeatures && strings.Contains(featuresPart, " ") {
		return nil, nil, trace.BadParameter(`invalid version %q: expected "<version>" or "<version> <feature[,feature...]>"`, clientVersion)
	}

	version, err := semver.NewVersion(versionPart)
	if err != nil {
		return nil, nil, trace.BadParameter("invalid version %q: invalid semantic version %q: %v", clientVersion, versionPart, err)
	}

	if !hasFeatures {
		return version, nil, nil
	}

	features := strings.Split(featuresPart, ",")
	if slices.Contains(features, "") {
		return nil, nil, trace.BadParameter("invalid version %q: empty feature name in %q", clientVersion, featuresPart)
	}

	return version, features, nil
}

// IsSSHFeatureSupported checks if the given SSH client version string indicates support for the specified feature. If
// the version string is malformed, an error is returned.
func IsSSHFeatureSupported(clientVersion, feature string) (bool, error) {
	_, features, err := ParseSSHClientVersion(clientVersion)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return slices.Contains(features, feature), nil
}
