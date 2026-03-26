// Copyright 2026 Gravitational, Inc
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

package ssh

import (
	"slices"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
)

const (
	// VersionPrefix is the prefix for the SSH client version string used by Teleport SSH clients.
	VersionPrefix = "SSH-2.0-Teleport_"

	// InBandMFAFeature is a flag included in the client version string to indicate support for in-band MFA (RFD 234).
	InBandMFAFeature = "mfav1"
)

// DefaultClientVersion returns the default SSH client identification string used by Teleport SSH clients.
const DefaultClientVersion = VersionPrefix + api.Version

// ClientVersionWithFeatures returns a client version string that includes the specified features. If no features are
// provided, it returns the default client version string.
//
// It returns a string in the format: SSH-2.0-Teleport_<teleport_version> <features>
//
// Examples:
//   - SSH-2.0-Teleport_19.0.0
//   - SSH-2.0-Teleport_19.0.0 mfav1
//   - SSH-2.0-Teleport_19.0.0 mfav1,foov1,barv1
func ClientVersionWithFeatures(features ...string) string {
	if len(features) == 0 {
		return DefaultClientVersion
	}

	return DefaultClientVersion + " " + strings.Join(features, ",")
}

// NonTeleportSSHVersionError is returned by ParseClientVersion when the provided SSH client version
// string does not have the expected Teleport prefix. The client is either not a Teleport client or is an older Teleport
// version that did not set a client version string.
type NonTeleportSSHVersionError struct{}

// Error returns the error message for NonTeleportSSHVersionError.
func (NonTeleportSSHVersionError) Error() string {
	return "SSH client version is not a Teleport version"
}

// ParseClientVersion parses the given SSH client version string and extracts the Teleport version and supported
// features.
//
// It returns the parsed Teleport version as a [semver.Version], a slice of supported features, and an error if the
// version string is malformed.
func ParseClientVersion(clientVersion string) (*semver.Version, []string, error) {
	rest, ok := strings.CutPrefix(clientVersion, VersionPrefix)
	if !ok {
		return nil, nil, NonTeleportSSHVersionError{}
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

// IsFeatureSupported checks if the given SSH client version string indicates support for the specified feature. If
// the version string is malformed, an error is returned.
func IsFeatureSupported(clientVersion, feature string) (bool, error) {
	_, features, err := ParseClientVersion(clientVersion)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return slices.Contains(features, feature), nil
}
