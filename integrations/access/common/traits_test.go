/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func TestGetUserTraits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	tests := []struct {
		description string
		setupMock   func(*mockUserClient)
		username    string
		expected    trait.Traits
	}{
		{
			description: "get user login state",
			setupMock: func(m *mockUserClient) {
				m.On("GetUserLoginState", mock.Anything, mock.Anything).
					Return(&userloginstate.UserLoginState{
						Spec: userloginstate.Spec{
							Traits: trait.Traits{
								"foo": {"bar"},
							},
						},
					}, nil)
			},
			expected: trait.Traits{
				"foo": {"bar"},
			},
		},
		{
			description: "get user",
			setupMock: func(m *mockUserClient) {
				m.On("GetUserLoginState", mock.Anything, mock.Anything).
					Return(nil, trace.AccessDenied("test error"))
				m.On("GetUser", mock.Anything, mock.Anything, mock.Anything).
					Return(&types.UserV2{
						Spec: types.UserSpecV2{
							Traits: wrappers.Traits{
								"foo": {"bar"},
							},
						},
					}, nil)
			},
			expected: trait.Traits{
				"foo": {"bar"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			client := &mockUserClient{}
			tt.setupMock(client)

			traits, err := GetUserTraits(ctx, client, "username")
			require.NoError(t, err)
			require.Len(t, traits, len(tt.expected))

			for key, val := range traits {
				require.ElementsMatch(t, tt.expected[key], val)
			}
		})
	}
}

type mockUserClient struct {
	mock.Mock
	services.UserOrLoginStateGetter
}

func (m *mockUserClient) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	args := m.Called(ctx, name, withSecrets)
	return args.Get(0).(types.User), args.Error(1)
}

func (m *mockUserClient) GetUserLoginState(ctx context.Context, name string) (*userloginstate.UserLoginState, error) {
	args := m.Called(ctx, name)
	userLoginState, ok := args.Get(0).(*userloginstate.UserLoginState)
	if ok {
		return userLoginState, args.Error(1)
	}
	return nil, args.Error(1)
}
