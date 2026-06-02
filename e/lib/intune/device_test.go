package intune

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/msgraph"
)

func TestOperatingSystemToOSType(t *testing.T) {
	tests := []struct {
		os    string
		model string
		want  devicepb.OSType
	}{
		{
			os:   "macOS",
			want: devicepb.OSType_OS_TYPE_MACOS,
		},
		{
			os:   "Windows",
			want: devicepb.OSType_OS_TYPE_WINDOWS,
		},
		{
			os:   "Linux (ubuntu)",
			want: devicepb.OSType_OS_TYPE_LINUX,
		},
		{
			os:    "iOS",
			model: "unknown model",
			want:  devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
		{
			os:    "iOS",
			model: "iPhone 14",
			want:  devicepb.OSType_OS_TYPE_IOS,
		},
		{
			// Verify case-insensitive compare.
			os:    "ios",
			model: "iphone 14",
			want:  devicepb.OSType_OS_TYPE_IOS,
		},
		{
			os:    "iOS",
			model: "iPad (A16)",
			want:  devicepb.OSType_OS_TYPE_IPADOS,
		},
		{
			os:   "Android",
			want: devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(strings.TrimSpace(fmt.Sprintf("%s %s", tt.os, tt.model)), func(t *testing.T) {
			require.Equal(t, tt.want.String(), operatingSystemToOSType(tt.os, tt.model).String())
		})
	}
}

func TestOSVersionToVersionAndBuild(t *testing.T) {
	tests := []struct {
		os          devicepb.OSType
		in          string
		wantVersion string
		wantBuild   string
		errCheck    require.ErrorAssertionFunc
	}{
		{
			os:          devicepb.OSType_OS_TYPE_WINDOWS,
			in:          "10.0.26100.4351",
			wantVersion: "10.0.26100",
			wantBuild:   "26100",
		},
		{
			os:          devicepb.OSType_OS_TYPE_MACOS,
			in:          "15.5 (24F74)",
			wantVersion: "15.5",
			wantBuild:   "24F74",
		},
		{
			os:          devicepb.OSType_OS_TYPE_LINUX,
			in:          "24.04",
			wantVersion: "24.04",
			wantBuild:   "",
		},
		{
			os:          devicepb.OSType_OS_TYPE_WINDOWS,
			in:          "10.0.26100",
			wantVersion: "10.0.26100",
			wantBuild:   "26100",
		},
		{
			os:       devicepb.OSType_OS_TYPE_WINDOWS,
			in:       "10.0",
			errCheck: require.Error,
		},
		{
			os:       devicepb.OSType_OS_TYPE_MACOS,
			in:       "15.5",
			errCheck: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.os, tt.in), func(t *testing.T) {
			version, build, err := osVersionToVersionAndBuild(tt.os, tt.in)

			if tt.errCheck != nil {
				tt.errCheck(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantVersion, version, "unexpected version")
			assert.Equal(t, tt.wantBuild, build, "unexpected build")
		})
	}
}

func TestManagedDeviceToDevice(t *testing.T) {
	genericLastSyncDateTime := time.Unix(1685468902, 0) // 2023-05-30T17:48:02+00:00

	tests := []struct {
		name     string
		md       *msgraph.ManagedDevice
		check    func(*testing.T, *msgraph.ManagedDevice, *devicepb.Device)
		errCheck func(*testing.T, error)
	}{
		{
			name: "valid device",
			md: &msgraph.ManagedDevice{
				ID:                      "id-1234",
				LastSyncDateTime:        genericLastSyncDateTime,
				DeviceRegistrationState: "registered",
				SerialNumber:            "sn-1234",
				OperatingSystem:         "macOS",
				Model:                   "MacBookPro9,2",
				OSVersion:               "15.5 (24F74)",
			},
			check: func(t *testing.T, md *msgraph.ManagedDevice, d *devicepb.Device) {
				want := &devicepb.Device{
					AssetTag: md.SerialNumber,
					OsType:   devicepb.OSType_OS_TYPE_MACOS,
					Profile: &devicepb.DeviceProfile{
						ExternalId:      md.ID,
						OsVersion:       "15.5",
						OsBuild:         "24F74",
						ModelIdentifier: "",
					},
				}
				require.Equal(t, want, d)
			},
		},
		{
			name: "iPhone",
			md: &msgraph.ManagedDevice{
				ID:                      "id-1234",
				LastSyncDateTime:        genericLastSyncDateTime,
				DeviceRegistrationState: "registered",
				SerialNumber:            "sn-1234",
				OperatingSystem:         "iOS",
				Model:                   "iPhone 16e",
				OSVersion:               "26.3.1",
			},
			check: func(t *testing.T, md *msgraph.ManagedDevice, d *devicepb.Device) {
				want := &devicepb.Device{
					AssetTag: md.SerialNumber,
					OsType:   devicepb.OSType_OS_TYPE_IOS,
					Profile: &devicepb.DeviceProfile{
						ExternalId:      md.ID,
						OsVersion:       "26.3.1",
						OsBuild:         "",
						ModelIdentifier: "",
					},
				}
				require.Equal(t, want, d)
			},
		},
		{
			name: "iPad",
			md: &msgraph.ManagedDevice{
				ID:                      "id-1234",
				LastSyncDateTime:        genericLastSyncDateTime,
				DeviceRegistrationState: "registered",
				SerialNumber:            "sn-1234",
				OperatingSystem:         "iOS",
				Model:                   "iPad (10th generation)",
				OSVersion:               "18.6",
			},
			check: func(t *testing.T, md *msgraph.ManagedDevice, d *devicepb.Device) {
				want := &devicepb.Device{
					AssetTag: md.SerialNumber,
					OsType:   devicepb.OSType_OS_TYPE_IPADOS,
					Profile: &devicepb.DeviceProfile{
						ExternalId:      md.ID,
						OsVersion:       "18.6",
						OsBuild:         "",
						ModelIdentifier: "",
					},
				}
				require.Equal(t, want, d)
			},
		},
		{
			name: "iOS-like with unknown model",
			md: &msgraph.ManagedDevice{
				ID:                      "id-1234",
				LastSyncDateTime:        genericLastSyncDateTime,
				DeviceRegistrationState: "registered",
				SerialNumber:            "sn-1234",
				OperatingSystem:         "iOS",
				Model:                   "Apple Phone 2",
				OSVersion:               "18.6",
			},
			check: func(t *testing.T, md *msgraph.ManagedDevice, d *devicepb.Device) {
				require.Equal(t, &devicepb.Device{}, d)
			},
		},
		{
			name: "nil managed device",
			md:   nil,
			errCheck: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "is nil")
			},
		},
		{
			name: "unspecified OS returns empty device",
			md: &msgraph.ManagedDevice{
				OperatingSystem: "foobar",
				SerialNumber:    "1234",
			},
			check: func(t *testing.T, _ *msgraph.ManagedDevice, d *devicepb.Device) {
				require.Equal(t, &devicepb.Device{}, d)
			},
		},
		{
			name: "no serial number",
			md:   &msgraph.ManagedDevice{},
			errCheck: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "no serial number")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := managedDeviceToDevice(tt.md)
			if tt.errCheck != nil {
				tt.errCheck(t, err)
				return
			}

			require.NoError(t, err)
			tt.check(t, tt.md, d)
		})
	}
}
