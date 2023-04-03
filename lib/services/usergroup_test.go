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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestUserGroupUnmarshal verifies a group resource can be unmarshaled.
func TestUserGroupUnmarshal(t *testing.T) {
	expected, err := types.NewUserGroup(
		types.Metadata{
			Name: "test-group",
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(userGroupYAML))
	require.NoError(t, err)
	actual, err := UnmarshalUserGroup(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestUserGroupMarshal verifies a marshaled group resource can be unmarshaled back.
func TestUserGroupMarshal(t *testing.T) {
	expected, err := types.NewUserGroup(
		types.Metadata{
			Name: "test-group",
		},
	)
	require.NoError(t, err)
	data, err := MarshalUserGroup(expected)
	require.NoError(t, err)
	actual, err := UnmarshalUserGroup(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestCompareUserGroup(t *testing.T) {
	tests := []struct {
		name      string
		userGroup types.UserGroup
		want      bool
	}{
		{
			name:      "equal",
			userGroup: userGroupWithModification(nil),
			want:      true,
		},
		{
			name:      "kind",
			userGroup: userGroupWithModification(func(ug *types.UserGroupV1) { ug.Kind = "diff" }),
		},
		{
			name:      "subkind",
			userGroup: userGroupWithModification(func(ug *types.UserGroupV1) { ug.SetSubKind("diff") }),
		},
		{
			name:      "version",
			userGroup: userGroupWithModification(func(ug *types.UserGroupV1) { ug.Version = "diff" }),
		},
		{
			name:      "metadata",
			userGroup: userGroupWithModification(func(ug *types.UserGroupV1) { ug.Metadata = types.Metadata{Name: "diff"} }),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userGroup := userGroupWithModification(nil)
			require.Equal(t, test.want, CompareUserGroups(userGroup, test.userGroup))
		})
	}
}

// userGroupWithModification returns a userGroup with modifications performed by the modFn function.
func userGroupWithModification(modFn func(*types.UserGroupV1)) types.UserGroup {
	userGroup := &types.UserGroupV1{
		ResourceHeader: types.ResourceHeader{
			Kind:     "kind",
			SubKind:  "subkind",
			Version:  "version",
			Metadata: metadataWithModification(nil),
		},
	}

	if modFn != nil {
		modFn(userGroup)
	}

	return userGroup
}

var userGroupYAML = `---
kind: user_group
version: v1
metadata:
  name: test-group
`
