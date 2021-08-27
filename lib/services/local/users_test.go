/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package local

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestRecoveryCodesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewIdentityService(backend)

	// Create a recovery codes resource.
	mockedCodes := []types.RecoveryCode{
		{HashedCode: []byte("code1")},
		{HashedCode: []byte("code2")},
		{HashedCode: []byte("code3")},
	}

	t.Run("upsert, get, delete recovery codes", func(t *testing.T) {
		t.Parallel()
		username := "someuser"

		rc1, err := types.NewRecoveryCodes(mockedCodes, backend.Clock().Now(), username)
		require.NoError(t, err)

		// Test creation of codes.
		err = service.UpsertRecoveryCodes(ctx, username, rc1)
		require.NoError(t, err)

		// Test fetching of codes.
		codes, err := service.GetRecoveryCodes(ctx, username)
		require.NoError(t, err)
		require.ElementsMatch(t, mockedCodes, codes.GetCodes())

		// Create new codes for same user.
		newMockedCodes := []types.RecoveryCode{
			{HashedCode: []byte("new-code1")},
			{HashedCode: []byte("new-code2")},
			{HashedCode: []byte("new-code3")},
		}
		rc2, err := types.NewRecoveryCodes(newMockedCodes, backend.Clock().Now(), username)
		require.NoError(t, err)

		// Test update of codes for same user.
		err = service.UpsertRecoveryCodes(ctx, username, rc2)
		require.NoError(t, err)

		// Test codes have been updated for same user.
		codes, err = service.GetRecoveryCodes(ctx, username)
		require.NoError(t, err)
		require.ElementsMatch(t, newMockedCodes, codes.GetCodes())
	})

	t.Run("deleting user deletes recovery codes", func(t *testing.T) {
		t.Parallel()
		username := "someuser2"

		// Create a user.
		userResource := &types.UserV2{}
		userResource.SetName(username)
		err = service.CreateUser(userResource)
		require.NoError(t, err)

		// Test codes exist for user.
		rc1, err := types.NewRecoveryCodes(mockedCodes, backend.Clock().Now(), username)
		require.NoError(t, err)
		err = service.UpsertRecoveryCodes(ctx, username, rc1)
		require.NoError(t, err)
		codes, err := service.GetRecoveryCodes(ctx, username)
		require.NoError(t, err)
		require.ElementsMatch(t, mockedCodes, codes.GetCodes())

		// Test deletion of recovery code along with user.
		err = service.DeleteUser(ctx, username)
		require.NoError(t, err)
		_, err = service.GetRecoveryCodes(ctx, username)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestRecoveryAttemptsCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backend, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewIdentityService(backend)

	// Predefine times for equality check.
	time1 := backend.Clock().Now()
	time2 := backend.Clock().Now().Add(2 * time.Minute)
	time3 := backend.Clock().Now().Add(4 * time.Minute)

	t.Run("create, get, and delete recovery attempts", func(t *testing.T) {
		t.Parallel()
		username := "someuser"

		// Test creation of recovery attempt.
		err = service.CreateUserRecoveryAttempt(ctx, username, &types.RecoveryAttempt{Time: time3, Expires: time3})
		require.NoError(t, err)
		err = service.CreateUserRecoveryAttempt(ctx, username, &types.RecoveryAttempt{Time: time1, Expires: time3})
		require.NoError(t, err)
		err = service.CreateUserRecoveryAttempt(ctx, username, &types.RecoveryAttempt{Time: time2, Expires: time3})
		require.NoError(t, err)

		// Test retrieving attempts sorted by oldest to latest.
		attempts, err := service.GetUserRecoveryAttempts(ctx, username)
		require.NoError(t, err)
		require.Len(t, attempts, 3)
		require.Equal(t, time1, attempts[0].Time)
		require.Equal(t, time2, attempts[1].Time)
		require.Equal(t, time3, attempts[2].Time)

		// Test delete all recovery attempts.
		err = service.DeleteUserRecoveryAttempts(ctx, username)
		require.NoError(t, err)
		attempts, err = service.GetUserRecoveryAttempts(ctx, username)
		require.NoError(t, err)
		require.Len(t, attempts, 0)
	})

	t.Run("deleting user deletes recovery attempts", func(t *testing.T) {
		t.Parallel()
		username := "someuser2"

		// Create a user, to test deletion of recovery attempts with user.
		userResource := &types.UserV2{}
		userResource.SetName(username)
		err = service.CreateUser(userResource)
		require.NoError(t, err)

		err = service.CreateUserRecoveryAttempt(ctx, username, &types.RecoveryAttempt{Time: time3, Expires: time3})
		require.NoError(t, err)
		attempts, err := service.GetUserRecoveryAttempts(ctx, username)
		require.NoError(t, err)
		require.Len(t, attempts, 1)

		err = service.DeleteUser(ctx, username)
		require.NoError(t, err)
		attempts, err = service.GetUserRecoveryAttempts(ctx, username)
		require.NoError(t, err)
		require.Len(t, attempts, 0)
	})
}
