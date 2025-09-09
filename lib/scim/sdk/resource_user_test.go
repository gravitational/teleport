// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package scimsdk

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestUserMarshalUnmarshal(t *testing.T) {
	want := User{
		ID:         "userID",
		ExternalID: "userExternalID",
		Meta: &Metadata{
			ResourceType: "User",
		},
		Schemas:  []string{"userSchema"},
		UserName: "userName",
		Name: &Name{
			FamilyName: "familyName",
			GivenName:  "givenName",
		},
		DisplayName: "displayName",
		Active:      true,
		Attributes:  AttributeSet{"attr1": "value1"},
	}

	buff, err := json.Marshal(&want)
	require.NoError(t, err)

	var attrGot = map[string]any{}
	err = json.Unmarshal(buff, &attrGot)
	require.NoError(t, err)

	attrWant := map[string]any{
		"id":         "userID",
		"externalId": "userExternalID",
		"meta": map[string]any{
			"resourceType": "User",
		},
		"schemas":  []any{"userSchema"},
		"userName": "userName",
		"name": map[string]any{
			"familyName": "familyName",
			"givenName":  "givenName",
		},
		"displayName": "displayName",
		"active":      true,
		"attr1":       "value1",
	}
	require.Empty(t, cmp.Diff(attrWant, attrGot))

	var got User
	require.NoError(t, json.Unmarshal(buff, &got))

	require.Empty(t, cmp.Diff(want, got))
}
