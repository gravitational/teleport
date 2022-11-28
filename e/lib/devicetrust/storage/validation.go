package storage

import (
	"crypto"
	"crypto/x509"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
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
	switch {
	case cd == nil:
		return trace.BadParameter("device collected data required")
	case !cd.CollectTime.IsValid():
		return trace.BadParameter("device collect time missing or invalid")
	case cd.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return trace.BadParameter("device data OS type required")
	case cd.SerialNumber == "":
		return trace.BadParameter("device serial number required")
	}
	return nil
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
