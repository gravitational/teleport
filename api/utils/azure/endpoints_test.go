// Copyright 2022 Gravitational, Inc
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

package azure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsMSSQLServerEndpoint(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc     string
		endpoint string
		result   bool
	}{
		// valid
		{"only suffix", ".database.windows.net", true},
		{"suffix with port", ".database.windows.net:1604", true},
		{"full name", "random.database.windows.net:1604", true},
		// invalid
		{"empty", "", false},
		{"without suffix", "hello:1604", false},
		{"wrong suffix", "hello.database.azure.com:1604", false},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.result, IsMSSQLServerEndpoint(tc.endpoint))
		})
	}
}

func TestParseMSSQLEndpoint(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc     string
		endpoint string
		valid    bool
		name     string
	}{
		// valid
		{"valid", "random.database.windows.net:1604", true, "random"},
		// invalid
		{"empty", "", false, ""},
		{"malformed adddress", "abc", false, ""},
		{"only suffix", ".database.windows.net:1604", false, ""},
		{"without suffix", "example.com:1604", false, ""},
		{"without port", "random.database.windows.net", false, ""},
		{"wrong suffix", "random.database.azure.com:1604", false, ""},
		{"more segments than supported", "hello.random.database.windows.net:1604", false, ""},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			name, err := ParseMSSQLEndpoint(tc.endpoint)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, tc.name, name)
		})
	}
}
