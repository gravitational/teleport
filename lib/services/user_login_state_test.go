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

// TestGetUserOrLoginStateIgnoresBots ensures ULS is never returned for bots,
// only the actual user.
func TestGetUserOrLoginStateIgnoresBots(t *testing.T) {
	tests := []struct {
		name     string
		populate func(t *testing.T, m *mockGetter)
		username string

		assertError require.ErrorAssertionFunc
		assertState func(t *testing.T, state UserState)
	}{
		{
			name:     "standard user with state returns the login state",
			username: "example",
			populate: func(t *testing.T, m *mockGetter) {
				user, err := types.NewUser("example")
				require.NoError(t, err)
				user.AddRole("access")
				m.users["example"] = user

				uls, err := userloginstate.New(
					header.Metadata{
						Name:   "example",
						Labels: map[string]string{},
					},
					userloginstate.Spec{
						Roles: []string{"access"},
					},
				)
				require.NoError(t, err)
				m.userStates["example"] = uls
			},
			assertError: require.NoError,
			assertState: func(t *testing.T, state UserState) {
				require.IsType(t, &userloginstate.UserLoginState{}, state)
			},
		},
		{
			name:     "standard user without state returns the user",
			username: "example",
			populate: func(t *testing.T, m *mockGetter) {
				user, err := types.NewUser("example")
				require.NoError(t, err)
				user.AddRole("access")
				m.users["example"] = user
			},
			assertError: require.NoError,
			assertState: func(t *testing.T, state UserState) {
				require.IsType(t, &types.UserV2{}, state)
			},
		},
		{
			name:     "bot user without state returns the user",
			username: "bot-example",
			populate: func(t *testing.T, m *mockGetter) {
				user, err := types.NewUser("bot-example")
				require.NoError(t, err)
				meta := user.GetMetadata()
				meta.Labels = map[string]string{
					types.BotLabel: "example",
				}
				user.SetMetadata(meta)
				user.AddRole("access")
				m.users["bot-example"] = user
			},
			assertError: require.NoError,
			assertState: func(t *testing.T, state UserState) {
				require.IsType(t, &types.UserV2{}, state)
			},
		},
		{
			name:     "bot user with user login state still returns the user",
			username: "bot-example",
			populate: func(t *testing.T, m *mockGetter) {
				user, err := types.NewUser("bot-example")
				require.NoError(t, err)
				meta := user.GetMetadata()
				meta.Labels = map[string]string{
					types.BotLabel: "example",
				}
				user.SetMetadata(meta)
				user.AddRole("access")
				m.users["bot-example"] = user

				uls, err := userloginstate.New(
					header.Metadata{
						Name: "bot-example",
						Labels: map[string]string{
							types.BotLabel: "example",
						},
					},
					userloginstate.Spec{
						Roles: []string{"access"},
					},
				)
				require.NoError(t, err)
				m.userStates["bot-example"] = uls
			},
			assertError: require.NoError,
			assertState: func(t *testing.T, state UserState) {
				require.IsType(t, &types.UserV2{}, state)
			},
		},
		{
			name:     "bot user with corrupt user login state still returns the user",
			username: "bot-example",
			populate: func(t *testing.T, m *mockGetter) {
				user, err := types.NewUser("bot-example")
				require.NoError(t, err)
				meta := user.GetMetadata()
				meta.Labels = map[string]string{
					types.BotLabel: "example",
				}
				user.SetMetadata(meta)
				user.AddRole("access")
				m.users["bot-example"] = user

				// Note missing labels and roles.
				uls, err := userloginstate.New(
					header.Metadata{
						Name: "bot-example",
					},
					userloginstate.Spec{},
				)
				require.NoError(t, err)
				m.userStates["bot-example"] = uls
			},
			assertError: require.NoError,
			assertState: func(t *testing.T, state UserState) {
				require.IsType(t, &types.UserV2{}, state)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter := &mockGetter{
				userStates: map[string]*userloginstate.UserLoginState{},
				users:      map[string]types.User{},
			}
			tt.populate(t, getter)

			state, err := GetUserOrLoginState(t.Context(), getter, tt.username)
			tt.assertError(t, err)
			tt.assertState(t, state)
		})
	}
}
