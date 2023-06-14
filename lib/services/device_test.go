/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
