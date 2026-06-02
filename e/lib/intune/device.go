package intune

import (
	"strings"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/msgraph"
)

// managedDeviceToDevice converts an Intune device to a Teleport device.
//
// If the Intune device has an unrecognized OS, managedDeviceToDevice returns a nil error and an
// empty Device struct rather than an error. This lets the service avoid emitting a warning in such
// a scenario, as customers might have hundreds of devices with OSes not supported by Device Trust.
func managedDeviceToDevice(md *msgraph.ManagedDevice) (*devicepb.Device, error) {
	if md == nil {
		// This is rather unexpected, but let's guard against it anyway.
		return nil, trace.BadParameter("managed device is nil")
	}

	if md.SerialNumber == "" {
		return nil, trace.BadParameter("device has no serial number")
	}

	osType := operatingSystemToOSType(md.OperatingSystem, md.Model)
	if osType == devicepb.OSType_OS_TYPE_UNSPECIFIED {
		return &devicepb.Device{}, nil
	}

	osVersion, osBuild, err := osVersionToVersionAndBuild(osType, md.OSVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.Device{
		OsType:   osType,
		AssetTag: md.SerialNumber,
		Profile: &devicepb.DeviceProfile{
			ExternalId: md.ID,
			OsVersion:  osVersion,
			OsBuild:    osBuild,
			// ModelIdentifier is not synced because of a mismatch between Intune's "model" field and the
			// model identifier reported by tsh. https://github.com/gravitational/teleport.e/pull/7581
		},
	}, nil
}

func operatingSystemToOSType(os string, model string) devicepb.OSType {
	os = strings.ToLower(os)

	switch os {
	case "macos":
		return devicepb.OSType_OS_TYPE_MACOS
	case "windows":
		return devicepb.OSType_OS_TYPE_WINDOWS
	case "ios":
		model = strings.ToLower(model)
		// Intune reports a single "iOS" operatingSystem for both iPhones and iPads. The only way to
		// distinguish between them is to look at the model field which has values like "iPad (10th
		// generation)", "iPhone 14", "iPad mini (A17 Pro)", "iPad Air (5th generation)".
		if strings.HasPrefix(model, "iphone") {
			return devicepb.OSType_OS_TYPE_IOS
		}
		if strings.HasPrefix(model, "ipad") {
			return devicepb.OSType_OS_TYPE_IPADOS
		}
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	}
	// Intune reports Linux machines as e.g. "Linux (ubuntu)".
	// TODO(ravicious): Use the part in parentheses as OsId in [devicepb.DeviceProfile]. If OsId is
	// present in a device profile, it's used to detect data drift between what's synced from an MDM
	// and what's reported by tsh (see lib/devicetrust/storage.validateDataLikeDrift). However, at the
	// moment the Jamf integration doesn't populate this field too. Before populating it, we'd need to
	// make sure that both Jamf and Intune report the same value as tsh on different Linux OSes.
	if strings.HasPrefix(os, "linux") {
		return devicepb.OSType_OS_TYPE_LINUX
	}
	return devicepb.OSType_OS_TYPE_UNSPECIFIED
}

// osVersionToVersionAndBuild tries to parse osVersion from Intune to a format aligned with what tsh
// reports.
func osVersionToVersionAndBuild(osType devicepb.OSType, input string) (string, string, error) {
	// If there's no reported OS version, just use empty strings. The Jamf integration uses a similar
	// approach. If a field in the device profile is empty, it will simply be skipped when verifying
	// collected data against the device profile during device auth and auto-enrollment.
	//
	// When the OS version is not empty, it must be successfully converted to a known format.
	// Otherwise we'd risk that data in the device profile from Intune never matches versions reported
	// by tsh, thus preventing auto-enrollment.
	if input == "" {
		return "", "", nil
	}

	switch osType {
	case devicepb.OSType_OS_TYPE_MACOS:
		// For example "15.5 (24F74)". Jamf and tsh are able to get these two values separately, Intune
		// returns it in a single string.
		version, rawBuild, found := strings.Cut(input, " ")
		if !found {
			return "", "", trace.BadParameter("unexpected macOS osVersion=%q", input)
		}
		build := strings.Trim(rawBuild, "()")
		return version, build, nil
	case devicepb.OSType_OS_TYPE_WINDOWS:
		// For example "10.0.26100.4351". Jamf and tsh report the first three parts as the version and
		// the third part as the OS build, effectively ignoring the fourth part. Intune has four parts,
		// so we have to drop the last one.
		// We also accept a scenario where Intune would return just three parts, just in case.
		subs := strings.SplitN(input, ".", 4)
		if len(subs) < 3 {
			return "", "", trace.BadParameter("unexpected Windows osVersion=%q", input)
		}
		version := strings.Join(subs[:3], ".")
		build := subs[2]
		return version, build, nil
	default:
		return input, "", nil
	}
}
