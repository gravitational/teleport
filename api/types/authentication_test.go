/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMarshalUnmarshalRequireMFAType tests encoding/decoding of the RequireMFAType.
func TestEncodeDecodeRequireMFAType(t *testing.T) {
	for _, tt := range []struct {
		requireMFAType RequireMFAType
		encoded        any
	}{
		{
			requireMFAType: RequireMFAType_OFF,
			encoded:        false,
		}, {
			requireMFAType: RequireMFAType_SESSION,
			encoded:        true,
		}, {
			requireMFAType: RequireMFAType_SESSION_AND_HARDWARE_KEY,
			encoded:        RequireMFATypeHardwareKeyString,
		}, {
			requireMFAType: RequireMFAType_HARDWARE_KEY_TOUCH,
			encoded:        RequireMFATypeHardwareKeyTouchString,
		}, {
			requireMFAType: RequireMFAType_HARDWARE_KEY_PIN,
			encoded:        RequireMFATypeHardwareKeyPINString,
		}, {
			requireMFAType: RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN,
			encoded:        RequireMFATypeHardwareKeyTouchAndPINString,
		},
	} {
		t.Run(tt.requireMFAType.String(), func(t *testing.T) {
			t.Run("encode", func(t *testing.T) {
				encoded, err := tt.requireMFAType.encode()
				require.NoError(t, err)
				require.Equal(t, tt.encoded, encoded)
			})

			t.Run("decode", func(t *testing.T) {
				var decoded RequireMFAType
				err := decoded.decode(tt.encoded)
				require.NoError(t, err)
				require.Equal(t, tt.requireMFAType, decoded)
			})
		})
	}
}
