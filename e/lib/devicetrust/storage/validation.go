package storage

import (
	"crypto"
	"crypto/x509"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

const (
	deviceIDLength              = 36 // aka an UUID, anything else is unexpected.
	maxCredentialIDLength       = 40 // UUID is 36 chars.
	maxDeviceAssetTagLength     = 40 // macOS serial is 12 chars, UUID is 36 chars.
	maxDeviceSerialNumberLength = maxDeviceAssetTagLength
)

// ValidateDeviceCredential validates a devicepb.DeviceCredential instance.
// Returns the parsed public key.
// The storage package is ultimately responsible for making sure data written to
// storage is valid, but this function is exposed to allow for early failures in
// multi-step ceremonies, such as device enrollment.
func ValidateDeviceCredential(cred *devicepb.DeviceCredential) (crypto.PublicKey, error) {
	switch {
	case cred == nil:
		return nil, trace.BadParameter("device credential required")
	case cred.Id == "":
		return nil, trace.BadParameter("credential ID required")
	case len(cred.Id) > maxCredentialIDLength:
		return nil, trace.BadParameter("credential ID exceeds %v characters", maxCredentialIDLength)
	case len(cred.PublicKeyDer) == 0:
		return nil, trace.BadParameter("credential public key required")
	}
	pubKey, err := x509.ParsePKIXPublicKey(cred.PublicKeyDer)
	if err != nil {
		return nil, trace.BadParameter("invalid credential public key DER")
	}
	return pubKey, nil
}

// ValidateCollectedData validates a devicepb.DeviceCollectedData instance,
// making sure all required fields are set and present fields are valid.
// The storage package is ultimately responsible for making sure data written to
// storage is valid, but this function is exposed to allow for early failures in
// multi-step ceremonies, such as device enrollment and authentication.
func ValidateCollectedData(cd *devicepb.DeviceCollectedData) error {
	return validateCollectedData(cd, false /* createAsResource */)
}

func validateCollectedData(cd *devicepb.DeviceCollectedData, createAsResource bool) error {
	switch {
	case cd == nil:
		return trace.BadParameter("device collected data required")
	case !cd.CollectTime.IsValid():
		return trace.BadParameter("device collect time missing or invalid")
	case cd.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return trace.BadParameter("device data OS type required")
	case cd.SerialNumber == "":
		return trace.BadParameter("device serial number required")
	case len(cd.SerialNumber) > maxDeviceSerialNumberLength:
		return trace.BadParameter("device serial number exceeds %v characters", maxDeviceSerialNumberLength)

	// No further validation required for non-resources.
	case !createAsResource:
		return nil

	// All fields must be set if writing collected data from a device resource.
	case !cd.RecordTime.IsValid():
		return trace.BadParameter("record time missing or invalid")

	default:
		return nil
	}
}

// ValidateCollectedDataAgainstDevice validates collected data against an
// existing device, ensure both match.
// The storage package is ultimately responsible for making sure data written to
// storage is valid, but this function is exposed to allow for early failures in
// multi-step ceremonies, such as device enrollment and authentication.
func ValidateCollectedDataAgainstDevice(cd *devicepb.DeviceCollectedData, dev *devicepb.Device) error {
	switch {
	case dev.OsType != cd.OsType:
		return trace.BadParameter(
			"collected data OS type mismatch: %v vs %v",
			dtoss.FriendlyOSType(dev.OsType),
			dtoss.FriendlyOSType(cd.OsType))
	case dev.AssetTag != cd.SerialNumber:
		return trace.BadParameter(
			"collected data serial number mismatch: %q vs %q",
			dev.AssetTag, cd.SerialNumber)
	}
	return nil
}

func validateDeviceForCreate(d *devicepb.Device, createAsResource bool) error {
	switch {
	case d == nil:
		return trace.BadParameter("device required")
	case d.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return trace.BadParameter("unknown or invalid os_type")
	case d.AssetTag == "":
		return trace.BadParameter("asset_tag required")
	case len(d.AssetTag) > maxDeviceAssetTagLength:
		return trace.BadParameter("asset_tag exceeds %v characters", maxDeviceAssetTagLength)

	// No further validation required for non-resources.
	case !createAsResource:
		return nil

	case d.ApiVersion != "" && d.ApiVersion != currentAPIVersion: // Only v1 supported.
		return trace.BadParameter("invalid or unsupported api_version: %v", d.ApiVersion)
	case d.CreateTime != nil && d.UpdateTime == nil,
		d.CreateTime == nil && d.UpdateTime != nil:
		return trace.BadParameter("either both or none of create_time and update_time must be set")
	case d.CreateTime != nil && !d.CreateTime.IsValid():
		return trace.BadParameter("invalid create_time")
	case d.UpdateTime != nil && !d.UpdateTime.IsValid():
		return trace.BadParameter("invalid update_time")
	case d.CreateTime != nil && d.UpdateTime != nil && d.CreateTime.AsTime().After(d.UpdateTime.AsTime()):
		return trace.BadParameter("create_time cannot be more recent than update_time")
	}

	// ID.
	if d.Id != "" {
		if _, err := uuid.Parse(d.Id); err != nil {
			return trace.BadParameter("device ID is not an UUID")
		}
	}

	// DeviceCredential.
	if d.Credential != nil {
		if _, err := ValidateDeviceCredential(d.Credential); err != nil {
			return trace.Wrap(err)
		}
	}

	// CollectedData.
	for i, cd := range d.CollectedData {
		if err := validateCollectedData(cd, true /* createAsResource */); err != nil {
			return trace.Wrap(err, "device collected_data[%v]", i)
		}
		if err := ValidateCollectedDataAgainstDevice(cd, d); err != nil {
			return trace.Wrap(err, "device collected_data[%v]", i)
		}
	}

	return nil
}
