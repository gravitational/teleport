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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
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

// TestOSTypeFlagValues covers the --os values offered by `tctl devices add`.
// They come from devicepb.OSType, so an OSType that ResourceOSTypeToString
// doesn't know about would reach the CLI as its bare proto name.
func TestOSTypeFlagValues(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"linux", "macos", "windows"},
		osTypeFlagValues(), "osTypeFlagValues mismatch")
}

// TestDeviceAddOSFlag covers the --os values `tctl devices add` accepts.
// The flag is the only thing keeping UNSPECIFIED out, as
// types.ResourceOSTypeFromString resolves "unspecified" without an error.
func TestDeviceAddOSFlag(t *testing.T) {
	t.Parallel()

	parse := func(t *testing.T, os string) error {
		t.Helper()
		c := DevicesCommand{}
		app := utils.InitCLIParser("tctl", GlobalHelpString)
		c.Initialize(app, &tctlcfg.GlobalCLIFlags{}, servicecfg.MakeDefaultConfig())
		_, err := app.Parse([]string{"devices", "add", "--os", os, "--asset-tag", "C00AA0AAAA0A"})
		return err
	}

	for _, os := range osTypeFlagValues() {
		t.Run(os, func(t *testing.T) {
			assert.NoError(t, parse(t, os), "--os %v rejected", os)
		})
	}

	for _, os := range []string{"unspecified", "OS_TYPE_MACOS", "bogus"} {
		t.Run("rejects "+os, func(t *testing.T) {
			assert.ErrorContains(t, parse(t, os), "enum value must be one of linux,", "--os %v accepted", os)
		})
	}
}
