// Copyright 2023 Gravitational, Inc
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

package types

import (
	"testing"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/stretchr/testify/require"
)

// TestUnmarshalDevice tests that devices can be successfully
// unmarshalled from YAML and JSON.
func TestUnmarshalDevice(t *testing.T) {
	for _, tc := range []struct {
		desc          string
		input         string
		errorContains string
		expected      *DeviceV1
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
			expected: &DeviceV1{
				ResourceHeader: ResourceHeader{
					Kind:    KindDevice,
					Version: "v1",
					Metadata: Metadata{
						Namespace: defaults.Namespace,
						Name:      "xaa",
					},
				},
				Spec: &DeviceSpec{
					OsType:       "macos",
					AssetTag:     "mymachine",
					EnrollStatus: "enrolled",
				},
			},
		},
		{
			desc:          "fail string as num",
			errorContains: `cannot unmarshal number`,
			input: `
{
  "kind": "device",
	"version": "v1",
	"metadata": {
		"name": "secretid"
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
			require.Equal(t, tc.expected, out, "unmarshalled device  does not match what was expected")
		})
	}
}
