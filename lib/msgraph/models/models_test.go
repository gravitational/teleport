// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package models

import (
	"encoding/json"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
)

func TestDecodeGroupMember(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		membersPayload json.RawMessage
		expectedMember GroupMember
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name: "supported user type",
			membersPayload: json.RawMessage(`
				{
				"@odata.type": "#microsoft.graph.user",
				"id": "9f615773-8219-4a5e-9eb1-8e701324c683",
				"mail": "alice@example.com"
				}
			`),
			expectedMember: &User{
				DirectoryObject: DirectoryObject{
					ID: to.Ptr("9f615773-8219-4a5e-9eb1-8e701324c683"),
				},
				Mail: to.Ptr("alice@example.com"),
			},
			errorAssertion: require.NoError,
		},
		{
			name: "supported group type",
			membersPayload: json.RawMessage(`
				{
				"@odata.type": "#microsoft.graph.group",
				"id": "7db727c5-924a-4f6d-b1f0-d44e6cafa87c",
				"displayName": "Test Group 1"
				}
			`),
			expectedMember: &Group{
				DirectoryObject: DirectoryObject{
					ID:          to.Ptr("7db727c5-924a-4f6d-b1f0-d44e6cafa87c"),
					DisplayName: to.Ptr("Test Group 1"),
				},
			},
			errorAssertion: require.NoError,
		},
		{
			name: "unsupported device type",
			membersPayload: json.RawMessage(`
				{
				"@odata.type": "#microsoft.graph.device",
				"id": "1566d9a7-c652-44e7-a75e-665b77431435",
				"mail": "device@example.com"
				}
				`),
			expectedMember: nil,
			errorAssertion: func(t require.TestingT, err error, i ...any) {
				var gmErr *UnsupportedGroupMember
				require.ErrorAs(t, err, &gmErr)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			member, err := DecodeGroupMember(tt.membersPayload)
			tt.errorAssertion(t, err)

			require.Equal(t, tt.expectedMember, member, "expected decoded group member to match")
		})
	}
}
