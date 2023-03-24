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

package device

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// TestUnmarshalDevice tests that devices can be successfully
// unmarshalled from YAML and JSON.
func TestUnmarshalDevice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc          string
		input         string
		errorContains string
		expected      *devicepb.Device
	}{
		{
			desc: "success",
			input: `---
kind: device
version: v1
metadata:
  name: xaa
spec:
  asset_tag: mymachine
  os_type: macos
  enroll_status: enrolled
`,
			expected: &devicepb.Device{
				Id:            "xaa",
				AssetTag:      "mymachine",
				OsType:        devicepb.OSType_OS_TYPE_MACOS,
				ApiVersion:    "v1",
				EnrollStatus:  devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
				CollectedData: []*devicepb.DeviceCollectedData{},
			},
		},
		{
			desc:          "fail string as num",
			errorContains: `ReadString: expects " or n`,
			input: `---
kind: device
version: v1
metadata:
  name: secretid
spec:
  asset_tag: 4
  os_type: macos
  enroll_status: enrolled
`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// Mimic tctl resource command by using the same decoder and
			// initially unmarshalling into services.UnknownResource
			reader := strings.NewReader(tc.input)
			decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
			var raw services.UnknownResource
			err := decoder.Decode(&raw)
			require.NoError(t, err)
			require.Equal(t, "device", raw.Kind)

			out, err := UnmarshalDevice(raw.Raw)
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "error from UnmarshalDevice does not contain the expected string")
				return
			}
			require.NoError(t, err, "UnmarshalDevice returned unexpected error")

			require.Equal(t, tc.expected, out, "unmarshalled device  does not match what was expected")
		})
	}
}
