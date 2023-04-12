package storage

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/mod/semver"
	"google.golang.org/protobuf/proto"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtent "github.com/gravitational/teleport/e/lib/devicetrust"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

const (
	deviceIDLength              = 36 // aka an UUID, anything else is unexpected.
	maxCredentialIDLength       = 40 // UUID is 36 chars.
	maxDeviceAssetTagLength     = 40 // macOS serial is 12 chars, UUID is 36 chars.
	maxDeviceSerialNumberLength = maxDeviceAssetTagLength

	maxDataModelIdentifierLength         = 40  // arbitrary, "large" number.
	maxDataOSVersionLength               = 40  // arbitrary, "large" number.
	maxDataOSBuildLength                 = 40  // arbitrary, "large" number.
	maxDataOSUsernameLength              = 40  // arbitrary, "large" number.
	maxDataJamfBinaryVersionLength       = 40  // arbitrary, "large" number.
	maxDataMacOSEnrollmentProfilesLength = 400 // arbitrary, "large" number.

	maxSourceNameLength = 40 // arbitrary, "large" number.
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
	}

	if dtent.MDMFeatureActive {
		if err := validateCollectedDataLike(cd.OsType, cd); err != nil {
			return trace.Wrap(err)
		}
		if len(cd.GetOsUsername()) > maxDataOSUsernameLength {
			return trace.BadParameter("device OS username exceeds %v characters", maxDeviceSerialNumberLength)
		}
	}

	// No further validation required for non-resources.
	if !createAsResource {
		return nil
	}

	// All fields must be set if writing collected data from a device resource.
	if !cd.RecordTime.IsValid() {
		return trace.BadParameter("record time missing or invalid")
	}

	return nil
}

// collectedDataLike represents the intersection of devicepb.CollectedData and
// devicepb.DeviceProfile.
type collectedDataLike interface {
	GetModelIdentifier() string
	GetOsVersion() string
	GetOsBuild() string
	GetJamfBinaryVersion() string
}

// validateCollectedDataLike validates the common fields between
// devicepb.CollectedData and devicepb.DeviceProfile.
func validateCollectedDataLike(osType devicepb.OSType, cd collectedDataLike) error {
	// Length checks for all variable-length fields.
	switch {
	case len(cd.GetModelIdentifier()) > maxDataModelIdentifierLength:
		return trace.BadParameter("model identifier exceeds %v characters", maxDataModelIdentifierLength)
	case len(cd.GetOsVersion()) > maxDataOSVersionLength:
		return trace.BadParameter("device OS version exceeds %v characters", maxDataOSVersionLength)
	case len(cd.GetOsBuild()) > maxDataOSBuildLength:
		return trace.BadParameter("device OS build exceeds %v characters", maxDataOSBuildLength)
	case len(cd.GetJamfBinaryVersion()) > maxDataJamfBinaryVersionLength:
		return trace.BadParameter("jamf binary version exceeds %v characters", maxDataJamfBinaryVersionLength)
	}

	// Parse OS version.
	if osType == devicepb.OSType_OS_TYPE_MACOS && cd.GetOsVersion() != "" {
		if err := validateSemver(cd.GetOsVersion()); err != nil {
			return trace.BadParameter("device OS version (macOS): %v", err)
		}
	}

	// TODO(codingllama): Further validate cd and cd-like fields?
	//  - OS version for other OSes
	//  - OS build
	//  - MacOS enrollment profile strings

	// Parse Jamf binary version.
	if cd.GetJamfBinaryVersion() != "" {
		if err := validateSemver(cd.GetJamfBinaryVersion()); err != nil {
			return trace.BadParameter("jamf binary version: %v", err)
		}
	}

	return nil
}

func validateSemver(v string) error {
	switch {
	case strings.HasPrefix(v, "v"):
		return trace.BadParameter("version number should not start with `v`")
	case !semver.IsValid(fmt.Sprintf("v%v", v)):
		return trace.BadParameter("not a valid semver: %q", v)
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
		return NewCollectedDataDriftError(
			fmt.Sprintf(
				"collected data OS type mismatch: %v vs %v",
				dtoss.FriendlyOSType(dev.OsType),
				dtoss.FriendlyOSType(cd.OsType)))
	case dev.AssetTag != cd.SerialNumber:
		return NewCollectedDataDriftError(
			fmt.Sprintf(
				"collected data serial number mismatch: %q vs %q",
				dev.AssetTag, cd.SerialNumber))
	}

	if !dtent.MDMFeatureActive || dev.Profile == nil {
		return nil
	}
	return trace.Wrap(validateDeviceProfileDrift(cd, dev.Profile))
}

func validateDeviceProfileDrift(cd *devicepb.DeviceCollectedData, profile *devicepb.DeviceProfile) error {
	// OS username.
	if len(profile.OsUsernames) > 0 {
		found := false
		for _, username := range profile.OsUsernames {
			if username == cd.OsUsername {
				found = true
				break
			}
		}
		if !found {
			return NewCollectedDataDriftError("device OS username not present in profile")
		}
	}

	// Data-like fields.
	return trace.Wrap(validateDataLikeDrift(cd, profile))
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
	}

	if dtent.MDMFeatureActive {
		if d.Source != nil {
			if err := validateDeviceSource(d.Source); err != nil {
				return trace.Wrap(err)
			}
		}
		if d.Profile != nil {
			if err := validateDeviceProfile(d.OsType, d.Profile, createAsResource); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// No further validation required for non-resources.
	if !createAsResource {
		return nil
	}

	// Validate "simple" readonly fields.
	switch {
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

func validateDeviceSource(source *devicepb.DeviceSource) error {
	switch {
	case source.Name == "":
		return trace.BadParameter("device source name required")
	case len(source.Name) > maxSourceNameLength:
		return trace.BadParameter("device source name exceeds %v characters", maxSourceNameLength)
	case source.Origin == devicepb.DeviceOrigin_DEVICE_ORIGIN_UNSPECIFIED:
		return trace.BadParameter("unknown or invalid device source origin")
	default:
		return nil
	}
}

func validateDeviceProfile(osType devicepb.OSType, profile *devicepb.DeviceProfile, createAsResource bool) error {
	if err := validateCollectedDataLike(osType, profile); err != nil {
		return trace.Wrap(err)
	}

	// Usernames.
	for i, username := range profile.OsUsernames {
		switch {
		case username == "":
			return trace.BadParameter("device profile username[%v]: username cannot be empty", i)
		case len(username) > maxDataOSUsernameLength:
			return trace.BadParameter("device profile username[%v]: username exceeds %v characters", i, maxDataOSUsernameLength)
		}
	}

	// No further validation required for non-resources.
	if !createAsResource {
		return nil
	}

	if profile.UpdateTime != nil && !profile.UpdateTime.IsValid() {
		return trace.BadParameter("invalid device profile update time")
	}
	return nil
}

func validateDeviceForUpdate(updated, stored *devicepb.Device) error {
	switch {
	case updated.ApiVersion != currentAPIVersion:
		return trace.BadParameter("unsupported api_version: %q", updated.ApiVersion)
	case updated.Id != stored.Id:
		return trace.BadParameter("id is readonly and cannot be updated")
	case updated.OsType != stored.OsType:
		return trace.BadParameter("os_type is readonly and cannot be updated")
	case updated.AssetTag != stored.AssetTag:
		return trace.BadParameter("asset_tag is readonly and cannot be updated")
	case !proto.Equal(updated.CreateTime, stored.CreateTime):
		return trace.BadParameter("create_time is readonly and cannot be updated")
	case !proto.Equal(updated.UpdateTime, stored.UpdateTime):
		// UpdateTime is changed as part of the update, but we make an attempt to
		// flag changes here first.
		return trace.BadParameter("update_time is readonly and cannot be updated")
	case !proto.Equal(updated.Credential, stored.Credential):
		return trace.BadParameter("credential is readonly and cannot be updated")
	}

	// EnrollStatus can only transition to NOT_ENROLLED.
	const notEnrolled = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED
	if updated.EnrollStatus != stored.EnrollStatus && updated.EnrollStatus != notEnrolled {
		return trace.BadParameter(
			"enroll_status can only be manually transitioned to %q",
			dtoss.FriendlyDeviceEnrollStatus(notEnrolled))
	}

	if dtent.MDMFeatureActive {
		// Source is mutable.
		if updated.Source != nil {
			if err := validateDeviceSource(updated.Source); err != nil {
				return trace.Wrap(err)
			}
		}
		// Profile is mutable.
		if updated.Profile != nil {
			if err := validateDeviceProfile(updated.OsType, updated.Profile, false /* createAsResource */); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

func validateCollectedDataDrift(target, source *devicepb.DeviceCollectedData) error {
	switch {
	case target == nil || source == nil:
		return trace.BadParameter("target and source required")
	case target.OsType != source.OsType:
		return NewCollectedDataDriftError("os_type drift detected")
	case target.SerialNumber != source.SerialNumber:
		return NewCollectedDataDriftError("serial number drift detected")
	case source.OsUsername != "" && source.OsUsername != target.OsUsername:
		return NewCollectedDataDriftError("device OS username drift detected")
	}
	if err := validateDataLikeDrift(target, source); err != nil {
		return trace.Wrap(err)
	}

	// TODO(codingllama): Detect drift on macos enrollment profiles.

	return nil
}

func validateDataLikeDrift(target, source collectedDataLike) error {
	switch {
	case source.GetModelIdentifier() != "" && target.GetModelIdentifier() != source.GetModelIdentifier():
		return NewCollectedDataDriftError("device model drift detected")
	case isBackwardsVersionDrift(source.GetOsVersion(), target.GetOsVersion()):
		return NewCollectedDataDriftError("device OS version drift detected")
	default:
		return nil
	}

	// TODO(codingllama): Verify Jamf binary drift?
	//  Backwards drift seems unlikely but could happen.
	//  Going from present to missing is also an interesting signal, but could be
	//  a legitimate change.
}

func isBackwardsVersionDrift(v1, v2 string) bool {
	if v1 == "" {
		return false
	}
	// v2 is required if v1 is known.
	if v2 == "" {
		return true
	}

	v1Prefixed := fmt.Sprintf("v%v", v1)
	v2Prefixed := fmt.Sprintf("v%v", v2)
	if !semver.IsValid(v1Prefixed) {
		return false // can't detect drift
	}

	return semver.Compare(v1Prefixed, v2Prefixed) > 0
}
