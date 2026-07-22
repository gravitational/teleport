// Copyright 2025 Gravitational, Inc.
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

// Package hardwarekey defines types and interfaces for hardware private keys.

package hardwarekey_test

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

func TestXxx(t *testing.T) {
	for _, tt := range []struct {
		slotString       hardwarekey.PIVSlotKeyString
		expectPIVSlotKey hardwarekey.PIVSlotKey
		assertError      require.ErrorAssertionFunc
	}{
		{
			slotString:       "9a",
			expectPIVSlotKey: 0x9a,
			assertError:      require.NoError,
		}, {
			slotString:       "9c",
			expectPIVSlotKey: 0x9c,
			assertError:      require.NoError,
		}, {
			slotString:       "9d",
			expectPIVSlotKey: 0x9d,
			assertError:      require.NoError,
		}, {
			slotString:       "9e",
			expectPIVSlotKey: 0x9e,
			assertError:      require.NoError,
		}, {
			slotString:       "invalid_uint",
			expectPIVSlotKey: 0,
			assertError:      require.Error,
		}, {
			slotString:       "9b", // unsupported slot key
			expectPIVSlotKey: 0,
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
			},
		},
	} {
		t.Run(string(tt.slotString), func(t *testing.T) {
			pivSlotKey, err := tt.slotString.Parse()
			tt.assertError(t, err)
			require.Equal(t, tt.expectPIVSlotKey, pivSlotKey)
		})
	}
}
