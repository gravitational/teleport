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
		}, types.UserGroupSpecV1{})
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
		}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	data, err := MarshalUserGroup(expected)
	require.NoError(t, err)
	actual, err := UnmarshalUserGroup(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var userGroupYAML = `---
kind: user_group
version: v1
metadata:
  name: test-group
`
