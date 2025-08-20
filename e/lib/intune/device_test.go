package intune

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/intune/api"
)

func TestOperatingSystemToOSType(t *testing.T) {
	tests := []struct {
		in  string
		out devicepb.OSType
	}{
		{
			in:  "macOS",
			out: devicepb.OSType_OS_TYPE_MACOS,
		},
		{
			in:  "macos",
			out: devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
		{
			in:  "Windows",
			out: devicepb.OSType_OS_TYPE_WINDOWS,
		},
		{
			in:  "Linux (ubuntu)",
			out: devicepb.OSType_OS_TYPE_LINUX,
		},
		{
			in:  "iOS",
			out: devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
		{
			in:  "Android",
			out: devicepb.OSType_OS_TYPE_UNSPECIFIED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			require.Equal(t, tt.out.String(), operatingSystemToOSType(tt.in).String())
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
	tests := []struct {
		name     string
		md       *api.ManagedDevice
		check    func(*testing.T, *api.ManagedDevice, *devicepb.Device)
		errCheck func(*testing.T, error)
	}{
		{
			name: "valid device",
			md: &api.ManagedDevice{
				ID:                      "id-1234",
				LastSyncDateTime:        time.Unix(1685468902, 0), // 2023-05-30T17:48:02+00:00
				DeviceRegistrationState: "registered",
				SerialNumber:            "sn-1234",
				Model:                   "MacBookPro9,2",
				OperatingSystem:         "macOS",
				OSVersion:               "15.5 (24F74)",
			},
			check: func(t *testing.T, md *api.ManagedDevice, d *devicepb.Device) {
				assert.Equal(t, md.ID, d.Profile.ExternalId)
				assert.Equal(t, devicepb.OSType_OS_TYPE_MACOS, d.OsType)
				assert.Equal(t, md.SerialNumber, d.AssetTag)
				assert.Equal(t, md.Model, d.Profile.ModelIdentifier)
				assert.Equal(t, "15.5", d.Profile.OsVersion)
				assert.Equal(t, "24F74", d.Profile.OsBuild)
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
			md: &api.ManagedDevice{
				OperatingSystem: "foobar",
				SerialNumber:    "1234",
			},
			check: func(t *testing.T, _ *api.ManagedDevice, d *devicepb.Device) {
				require.Equal(t, &devicepb.Device{}, d)
			},
		},
		{
			name: "no serial number",
			md:   &api.ManagedDevice{},
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
