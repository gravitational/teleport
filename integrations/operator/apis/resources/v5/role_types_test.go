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

package v5

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

// This tests that `redirect_url` is consistently marshaled as a list of string
// This is not the case of wrappers.Strings which marshals as a string if it contains a single element
func TestTeleportOIDCConnectorSpec_MarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		spec         TeleportRoleSpec
		expectedJSON string
	}{
		{
			"Empty string",
			TeleportRoleSpec{
				Allow: types.RoleConditions{
					NodeLabels: map[string]utils.Strings{"foo": {""}}}},
			`{"allow":{"node_labels":{"foo":[""]}},"deny":{},"options":{"forward_agent":false,"cert_format":"","record_session":null,"desktop_clipboard":null,"desktop_directory_sharing":null,"pin_source_ip":false,"ssh_file_copy":null,"create_desktop_user":null,"create_db_user":null}}`,
		},
		{
			"Single string",
			TeleportRoleSpec{
				Allow: types.RoleConditions{
					NodeLabels: map[string]utils.Strings{"foo": {"bar"}}}},
			`{"allow":{"node_labels":{"foo":["bar"]}},"deny":{},"options":{"forward_agent":false,"cert_format":"","record_session":null,"desktop_clipboard":null,"desktop_directory_sharing":null,"pin_source_ip":false,"ssh_file_copy":null,"create_desktop_user":null,"create_db_user":null}}`,
		},
		{
			"Multiple strings",
			TeleportRoleSpec{
				Allow: types.RoleConditions{
					NodeLabels: map[string]utils.Strings{"foo": {"bar", "baz"}}}},
			`{"allow":{"node_labels":{"foo":["bar","baz"]}},"deny":{},"options":{"forward_agent":false,"cert_format":"","record_session":null,"desktop_clipboard":null,"desktop_directory_sharing":null,"pin_source_ip":false,"ssh_file_copy":null,"create_desktop_user":null,"create_db_user":null}}`,
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
