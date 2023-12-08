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

	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/utils"
)

// TestUserLoginStateUnmarshal verifies a user login state resource can be unmarshaled.
func TestUserLoginStateUnmarshal(t *testing.T) {
	expected, err := userloginstate.New(
		header.Metadata{
			Name: "test-user",
		},
		userloginstate.Spec{
			Roles: []string{
				"role1",
				"role2",
				"role3",
			},
			Traits: map[string][]string{
				"trait1": {"value1", "value2"},
				"trait2": {"value3", "value4"},
			},
		},
	)
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(`---
kind: user_login_state
version: v1
metadata:
  name: test-user
spec:
  roles:
  - role1
  - role2
  - role3
  traits:
    trait1:
    - value1
    - value2
    trait2:
    - value3
    - value4
`))
	require.NoError(t, err)
	actual, err := UnmarshalUserLoginState(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestUserLoginStateMarshal verifies a marshaled user login state resource can be unmarshaled back.
func TestUserLoginStateMarshal(t *testing.T) {
	expected, err := userloginstate.New(
		header.Metadata{
			Name: "test-user",
		},
		userloginstate.Spec{
			Roles: []string{
				"role1",
				"role2",
				"role3",
			},
			Traits: map[string][]string{
				"trait1": {"value1", "value2"},
				"trait2": {"value3", "value4"},
			},
		},
	)
	require.NoError(t, err)
	data, err := MarshalUserLoginState(expected)
	require.NoError(t, err)
	actual, err := UnmarshalUserLoginState(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
