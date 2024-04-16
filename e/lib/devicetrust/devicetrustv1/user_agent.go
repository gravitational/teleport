package devicetrustv1

import (
	"context"
	"log/slog"
	"strings"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// loggedGetOSFromUserAgent debug-logs the result of [getOSFromUserAgent] before
// returning it.
func loggedGetOSFromUserAgent(logger *slog.Logger, ua string) devicepb.OSType {
	os := getOSFromUserAgent(ua)
	logger.DebugContext(context.Background(),
		"Mapped user agent to OS",
		"os", os,
		"user_agent", ua,
	)
	return os
}

// getOSFromUserAgent returns the [devicepb.OSType] inferred from the user agent
// string.
//
// This function makes no attempt to parse the user agent, instead it assumes a
// valid string and looks for certain keywords within it.
//
// Returns [devicepb.OSType_OS_TYPE_UNSPECIFIED] for both unmapped systems and
// unknown strings.
func getOSFromUserAgent(ua string) devicepb.OSType {
	// Partially based on
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Browser_detection_using_the_user_agent#os.

	// Be case-insensitive.
	ua = strings.ToLower(ua)

	switch {
	case strings.Contains(ua, "mobile"), strings.Contains(ua, "android"):
		// Ignore mobile user agents.
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	case strings.Contains(ua, "xbox"):
		// Looks remarkably like Windows otherwise.
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	case strings.Contains(ua, "mac os x"):
		return devicepb.OSType_OS_TYPE_MACOS
	case strings.Contains(ua, "windows"):
		return devicepb.OSType_OS_TYPE_WINDOWS
	case strings.Contains(ua, "linux"):
		return devicepb.OSType_OS_TYPE_LINUX
	}

	return devicepb.OSType_OS_TYPE_UNSPECIFIED
}
