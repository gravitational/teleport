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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestUserLoginStateCRUD tests backend operations with user login state resources.
func TestUserLoginStateCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	service, err := NewUserLoginStateService(backend)
	require.NoError(t, err)

	// Create a couple user login states.
	state1 := newUserLoginState(t, "state1")
	state2 := newUserLoginState(t, "state2")

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	}

	// Neither state should exist.
	_, err = service.GetUserLoginState(ctx, state1.GetName())
	require.True(t, trace.IsNotFound(err))
	_, err = service.GetUserLoginState(ctx, state2.GetName())
	require.True(t, trace.IsNotFound(err))

	// Create both states.
	state, err := service.UpsertUserLoginState(ctx, state1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(state1, state, cmpOpts...))
	state, err = service.UpsertUserLoginState(ctx, state2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(state2, state, cmpOpts...))

	// Fetch a specific state.
	state, err = service.GetUserLoginState(ctx, state2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(state2, state, cmpOpts...))

	// Update a state.
	state1.SetExpiry(clock.Now().Add(30 * time.Minute))
	state, err = service.UpsertUserLoginState(ctx, state1)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(state1, state, cmpOpts...))
	state, err = service.GetUserLoginState(ctx, state1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(state1, state, cmpOpts...))

	// Delete a state.
	err = service.DeleteUserLoginState(ctx, state1.GetName())
	require.NoError(t, err)
	_, err = service.GetUserLoginState(ctx, state1.GetName())
	require.True(t, trace.IsNotFound(err))

	// Try to delete a state that doesn't exist.
	err = service.DeleteUserLoginState(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)
}

func newUserLoginState(t *testing.T, name string) *userloginstate.UserLoginState {
	t.Helper()

	userLoginState, err := userloginstate.New(
		header.Metadata{
			Name: name,
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

	return userLoginState
}
