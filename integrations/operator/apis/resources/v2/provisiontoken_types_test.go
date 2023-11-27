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

package v2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// This tests that `redirect_url` is consistently marshaled as a list of string
// This is not the case of wrappers.Strings which marshals as a string if it contains a single element
func TestTeleportOIDCConnectorSpec_MarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		spec         TeleportProvisionTokenSpec
		expectedJSON string
	}{
		{
			"Empty string",
			TeleportProvisionTokenSpec{SuggestedLabels: types.Labels{"foo": {""}}},
			`{"suggested_labels":{"foo":[""]},"roles":null,"join_method":""}`,
		},
		{
			"Single string",
			TeleportProvisionTokenSpec{SuggestedLabels: types.Labels{"foo": {"bar"}}},
			`{"suggested_labels":{"foo":["bar"]},"roles":null,"join_method":""}`,
		},
		{
			"Multiple strings",
			TeleportProvisionTokenSpec{SuggestedLabels: types.Labels{"foo": {"bar", "baz"}}},
			`{"suggested_labels":{"foo":["bar","baz"]},"roles":null,"join_method":""}`,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := json.Marshal(tc.spec)
			require.NoError(t, err)
			require.Equal(t, tc.expectedJSON, string(result))
		})
	}
}
