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

import devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"

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
