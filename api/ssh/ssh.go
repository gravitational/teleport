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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
)

const (
	// VersionPrefix is the prefix for the SSH client version string used by Teleport SSH clients.
	VersionPrefix = "SSH-2.0-Teleport"

	// InBandMFAFeature is a flag included in the client version string to indicate support for in-band MFA (RFD 234).
	InBandMFAFeature = "mfav1"
)

// DefaultClientVersion returns the default SSH client identification string used by Teleport SSH clients.
const DefaultClientVersion = VersionPrefix + "_" + api.Version

// ClientVersionWithFeatures returns a client version string that includes the specified features. If no features are
// provided, it returns the default client version string.
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
// features. It returns the Teleport version and a slice of supported features. If no features are specified, the
// features slice will be nil. If the client version string does not have the expected Teleport prefix, it returns a
// NonTeleportSSHVersionError. If the client version string contains invalid characters, it returns a BadParameter
// error.
//
// It intentionally does not attempt to parse the Teleport version into a structured format, as the Teleport version
// string is primarily used for informational purposes and may include additional metadata in the future. It is also
// intentional that features may contain spaces.
//
//	Accepted formats are:
//	 - SSH-2.0-Teleport
//	 - SSH-2.0-Teleport <feature1,feature2,...>
//	 - SSH-2.0-Teleport_<teleport_version>
//	 - SSH-2.0-Teleport_<teleport_version> <feature1,feature2,...>
//	 - SSH-2.0-Teleport<teleport_version>
//	 - SSH-2.0-Teleport<teleport_version> <feature1,feature2,...>
func ParseClientVersion(clientVersion string) (string, []string, error) {
	// Validate that the client version string contains only allowed ASCII characters (32-126).
	for i := range clientVersion {
		b := clientVersion[i]

		if b < 32 || b > 126 {
			return "", nil, trace.BadParameter("SSH client version contain invalid characters %q", b)
		}
	}

	// Remove the version prefix and ensure it is actually a Teleport SSH client version string.
	rest, ok := strings.CutPrefix(clientVersion, VersionPrefix)
	if !ok {
		return "", nil, NonTeleportSSHVersionError{}
	}

	// No version or features provided after the prefix.
	if rest == "" {
		return "", nil, nil
	}

	// Separate the version part from the features part by the first space.
	versionPart, featuresPart, hasFeatures := strings.Cut(rest, " ")

	// Remove the leading underscore from the version part, if present.
	if versionPart != "" {
		versionPart = strings.TrimPrefix(versionPart, "_")
	}

	// If there are no features, return the version.
	if !hasFeatures {
		return versionPart, nil, nil
	}

	// Split the features part into individual features.
	features := strings.Split(featuresPart, ",")

	return versionPart, features, nil
}

// IsFeatureSupported checks if the given SSH client version string indicates support for the specified feature.
func IsFeatureSupported(clientVersion, feature string) (bool, error) {
	_, features, err := ParseClientVersion(clientVersion)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return slices.Contains(features, feature), nil
}
