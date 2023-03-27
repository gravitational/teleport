// Copyright 2022 Gravitational, Inc
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

package devicetrust

import (
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// FriendlyOSType returns a user-friendly OSType representation.
// Recommended for user-facing messages.
func FriendlyOSType(osType devicepb.OSType) string {
	switch osType {
	case devicepb.OSType_OS_TYPE_LINUX:
		return "Linux"
	case devicepb.OSType_OS_TYPE_MACOS:
		return "macOS"
	case devicepb.OSType_OS_TYPE_WINDOWS:
		return "Windows"
	default:
		return osType.String()
	}
}

// FriendlyOSType returns a user-friendly DeviceEnrollStatus representation.
// Recommended for user-facing messages.
func FriendlyDeviceEnrollStatus(enrollStatus devicepb.DeviceEnrollStatus) string {
	switch enrollStatus {
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED:
		return "enrolled"
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED:
		return "not enrolled"
	default:
		return enrollStatus.String()
	}
}

// ResourceOSTypeToString converts OSType to a string representation suitable for use in resource fields.
func ResourceOSTypeToString(osType devicepb.OSType) string {
	switch osType {
	case devicepb.OSType_OS_TYPE_LINUX:
		return "linux"
	case devicepb.OSType_OS_TYPE_MACOS:
		return "macos"
	case devicepb.OSType_OS_TYPE_WINDOWS:
		return "windows"
	default:
		return osType.String()
	}
}

// ResourceOSTypeFromString converts a string representation of OSType suitable for resource fields to OSType.
func ResourceOSTypeFromString(osType string) (devicepb.OSType, error) {
	switch osType {
	case "linux":
		return devicepb.OSType_OS_TYPE_LINUX, nil
	case "macos":
		return devicepb.OSType_OS_TYPE_MACOS, nil
	case "windows":
		return devicepb.OSType_OS_TYPE_WINDOWS, nil
	default:
		return devicepb.OSType_OS_TYPE_UNSPECIFIED, trace.BadParameter("unknown os type %q", osType)
	}
}

// ResourceEnrollStatusToString converts DeviceEnrollStatus to a string representation suitable for use in resource fields.
func ResourceEnrollStatusToString(enrollStatus devicepb.DeviceEnrollStatus) string {
	switch enrollStatus {
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED:
		return "enrolled"
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED:
		return "not_enrolled"
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED:
		return "unspecified"
	default:
		return enrollStatus.String()
	}
}

// ResourceEnrollStatusFromString converts a string representation of DeviceEnrollStatus suitable for resource fields to DeviceEnrollStatus.
func ResourceEnrollStatusFromString(enrollStatus string) (devicepb.DeviceEnrollStatus, error) {
	switch enrollStatus {
	case "enrolled":
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED, nil
	case "not_enrolled":
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED, nil
	case "unspecified":
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED, nil
	default:
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED, trace.BadParameter("unknown enroll status %q", enrollStatus)
	}
}
