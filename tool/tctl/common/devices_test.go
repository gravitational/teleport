// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func TestDeviceSourceToString(t *testing.T) {
	tests := []struct {
		name   string
		source *devicepb.DeviceSource
		want   string
	}{
		{
			name:   "default name for origin",
			source: &devicepb.DeviceSource{Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE, Name: "intune"},
			want:   "Intune",
		},
		{
			name:   "custom name",
			source: &devicepb.DeviceSource{Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF, Name: "cool jamf"},
			want:   "cool jamf",
		},
		{
			name:   "no source",
			source: nil,
			want:   "",
		},
		{
			name:   "unsupported origin",
			source: &devicepb.DeviceSource{Origin: 1337, Name: "even cooler jamf"},
			// Show the name instead of something like "unknown" as name is required and likely more
			// informative than displaying "unknown".
			want: "even cooler jamf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, deviceSourceToString(tt.source))
		})
	}
}
