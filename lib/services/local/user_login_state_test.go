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
		cmpopts.IgnoreFields(header.Metadata{}, "ID"),
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
