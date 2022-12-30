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

package v3

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/wrappers"
)

// This tests that `redirect_url` is consistently marshaled as a list of string
// This is not the case of wrappers.Strings which marshals as a string if it contains a single element
func TestTeleportOIDCConnectorSpec_MarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		spec         TeleportOIDCConnectorSpec
		expectedJSON string
	}{
		{
			"Empty string",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{""}},
			`{"redirect_url":[""],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"Single string",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{"foo"}},
			`{"redirect_url":["foo"],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"Multiple strings",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{"foo", "bar"}},
			`{"redirect_url":["foo","bar"],"issuer_url":"","client_id":"","client_secret":""}`,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := json.Marshal(tc.spec)
			require.NoError(t, err)
			require.Equal(t, string(result), tc.expectedJSON)
		})
	}
}
