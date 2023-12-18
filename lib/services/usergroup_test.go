/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
