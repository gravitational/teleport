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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

// TestUnmarshalDevice tests that devices can be successfully
// unmarshalled from YAML and JSON.
func TestUnmarshalDevice(t *testing.T) {
	for _, tc := range []struct {
		desc          string
		input         string
		errorContains string
		expected      *types.DeviceV1
	}{
		{
			desc: "success",
			input: `
{
  "kind": "device",
	"version": "v1",
	"metadata": {
		"name": "xaa"
	},
	"spec": {
		"asset_tag": "mymachine",
		"os_type": "macos",
		"enroll_status": "enrolled"
	}
}`,
			expected: &types.DeviceV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindDevice,
					Version: "v1",
					Metadata: types.Metadata{
						Namespace: defaults.Namespace,
						Name:      "xaa",
					},
				},
				Spec: &types.DeviceSpec{
					OsType:       "macos",
					AssetTag:     "mymachine",
					EnrollStatus: "enrolled",
				},
			},
		},
		{
			desc:          "fail string as num",
			errorContains: `ReadString: expects " or n, but found 4`,
			input: `
{
  "kind": "device",
	"version": "v1",
	"metadata": {
		"name": "xdd"
	},
	"spec": {
		"asset_tag": 4,
		"os_type": "macos",
		"enroll_status": "enrolled"
	}
}`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			out, err := UnmarshalDevice([]byte(tc.input))
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "error from UnmarshalDevice does not contain the expected string")
				return
			}
			require.NoError(t, err, "UnmarshalDevice returned unexpected error")
			require.Equal(t, tc.expected, out, "unmarshalled device does not match what was expected")
		})
	}
}
