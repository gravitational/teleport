/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
