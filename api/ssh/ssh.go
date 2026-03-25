package ssh

import (
	"slices"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
)

// VersionPrefix is the prefix for the SSH client version string used by Teleport SSH clients.
const VersionPrefix = "SSH-2.0-Teleport_"

// ClientVersion returns the SSH client identification string used by Teleport SSH clients.
//
// Format: SSH-2.0-Teleport_<teleport_version> <features>
//
// Teleport version is used for telemetry and debugging purposes.
//
// Features are comma-separated flags indicated supported features of the client:
// - mfav1: Client supports in-band MFA (RFD 234).
func ClientVersion() string {
	return VersionPrefix + api.Version + " " + "mfav1"
}

// NonTeleportSSHVersionError is returned by ParseSSHClientVersion when the provided SSH client version
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
