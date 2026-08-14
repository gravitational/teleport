package service_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/jamf"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
)

func TestMobileToOSType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		deviceType      string
		modelIdentifier string
		want            devicepb.OSType
	}{
		{
			name:            "iPad",
			deviceType:      "iOS",
			modelIdentifier: "iPad15,7",
			want:            devicepb.OSType_OS_TYPE_IPADOS,
		},
		{
			name:            "iPhone",
			deviceType:      "iOS",
			modelIdentifier: "iPhone15,2",
			want:            devicepb.OSType_OS_TYPE_IOS,
		},
		{
			name:            "unknown device type",
			deviceType:      "watchOS",
			modelIdentifier: "Watch6,1",
			want:            devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
		{
			name:            "iOS with unknown model",
			deviceType:      "iOS",
			modelIdentifier: "AppleTV11,1",
			want:            devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
		{
			// This is important because Jamf returns "iOS" when querying
			// the endpoint that returns a paginated list and "ios" when getting
			// details of a specific mobile device.
			name:            "case insensitive device type",
			deviceType:      "ios",
			modelIdentifier: "iPhone15,2",
			want:            devicepb.OSType_OS_TYPE_IOS,
		},
		{
			name:            "case insensitive model identifier (iPhone)",
			deviceType:      "ios",
			modelIdentifier: "iphone15,2",
			want:            devicepb.OSType_OS_TYPE_IOS,
		},
		{
			name:            "case insensitive model identifier (iPad)",
			deviceType:      "ios",
			modelIdentifier: "ipad15,7",
			want:            devicepb.OSType_OS_TYPE_IPADOS,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := jamfservice.MobileToOSType(test.deviceType, test.modelIdentifier)
			require.Equal(t, test.want, got)
		})
	}
}

func TestMobileToDevice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		device  *jamf.MobileDevice
		want    *devicepb.Device
		wantErr string
	}{
		{
			name: "iPad with all sections",
			device: &jamf.MobileDevice{
				MobileDeviceID: "1",
				DeviceType:     "iOS",
				General: &jamf.MobileDeviceGeneralSection{
					OSVersion:                  "26.3.1",
					OSBuild:                    "23D8133",
					OSSupplementalBuildVersion: "23D771330a",
				},
				Hardware: &jamf.MobileDeviceHardwareSection{
					SerialNumber:    "CXXXXXXXXXX1",
					ModelIdentifier: "iPad15,7",
				},
			},
			want: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_IPADOS,
				AssetTag: "CXXXXXXXXXX1",
				Profile: devicepb.DeviceProfile_builder{
					ModelIdentifier:     "iPad15,7",
					ExternalId:          "1",
					OsVersion:           "26.3.1",
					OsBuild:             "23D8133",
					OsBuildSupplemental: "23D771330a",
				}.Build(),
			}.Build(),
		},
		{
			name: "iPhone without general section",
			device: &jamf.MobileDevice{
				MobileDeviceID: "2",
				DeviceType:     "iOS",
				Hardware: &jamf.MobileDeviceHardwareSection{
					SerialNumber:    "CXXXXXXXXXX2",
					ModelIdentifier: "iPhone15,2",
				},
			},
			want: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_IOS,
				AssetTag: "CXXXXXXXXXX2",
				Profile: devicepb.DeviceProfile_builder{
					ModelIdentifier: "iPhone15,2",
					ExternalId:      "2",
				}.Build(),
			}.Build(),
		},
		{
			name:    "nil device",
			device:  nil,
			wantErr: "mobile device is nil",
		},
		{
			name: "no hardware section",
			device: &jamf.MobileDevice{
				MobileDeviceID: "3",
				DeviceType:     "iOS",
			},
			wantErr: "no hardware section",
		},
		{
			name: "unknown device type",
			device: &jamf.MobileDevice{
				MobileDeviceID: "4",
				DeviceType:     "tvOS",
				Hardware: &jamf.MobileDeviceHardwareSection{
					SerialNumber:    "CXXXXXXXXXX3",
					ModelIdentifier: "AppleTV11,1",
				},
			},
			wantErr: "unexpected deviceType",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := jamfservice.MobileToDevice(test.device)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("MobileDeviceToDevice mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
