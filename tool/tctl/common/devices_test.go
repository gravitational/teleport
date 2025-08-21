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
