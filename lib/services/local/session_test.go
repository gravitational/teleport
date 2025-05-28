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

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestDeleteUserAppSessions(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	identity, err := NewTestIdentityService(backend)
	require.NoError(t, err)
	users := []string{"alice", "bob"}
	ctx := context.Background()

	// Create app sessions for different users.
	for _, user := range users {
		session, err := types.NewWebSession(uuid.New().String(), types.KindAppSession, types.WebSessionSpecV2{
			User:    user,
			Expires: clock.Now().Add(time.Hour),
		})
		require.NoError(t, err)

		err = identity.UpsertAppSession(ctx, session)
		require.NoError(t, err)
	}

	// Ensure the number of app sessions is correct.
	sessions, nextKey, err := identity.ListAppSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	require.Empty(t, nextKey)

	// Delete sessions of the first user.
	err = identity.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: users[0]})
	require.NoError(t, err)

	sessions, nextKey, err = identity.ListAppSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, users[1], sessions[0].GetUser())
	require.Empty(t, nextKey)

	// Delete sessions of the second user.
	err = identity.DeleteUserAppSessions(ctx, &proto.DeleteUserAppSessionsRequest{Username: users[1]})
	require.NoError(t, err)

	sessions, nextKey, err = identity.ListAppSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.Empty(t, nextKey)
}

func TestListAppSessions(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	identity, err := NewTestIdentityService(backend)
	require.NoError(t, err)

	users := []string{"alice", "bob"}
	ctx := context.Background()

	// the default page size is used if the pageSize
	// provide to ListAppSessions is 0 || > maxSessionPageSize
	const useDefaultPageSize = 0

	// Validate no sessions exist
	sessions, token, err := identity.ListAppSessions(ctx, useDefaultPageSize, "", "")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.Empty(t, token)

	// Create 3 pages worth of sessions. One full
	// page per user and one partial page with 5
	// sessions per user.
	for i := 0; i < maxSessionPageSize+5; i++ {
		for _, user := range users {
			session, err := types.NewWebSession(uuid.New().String(), types.KindAppSession, types.WebSessionSpecV2{
				User:    user,
				Expires: clock.Now().Add(time.Hour),
			})
			require.NoError(t, err)

			err = identity.UpsertAppSession(ctx, session)
			require.NoError(t, err)
		}
	}

	// Validate page size is truncated to maxSessionPageSize
	sessions, token, err = identity.ListAppSessions(ctx, maxSessionPageSize+maxSessionPageSize*2/3, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, maxSessionPageSize)
	require.NotEmpty(t, token)

	// reset token
	token = ""

	// Validate that sessions are retrieved for all users
	// with the default page size
	for {
		sessions, token, err = identity.ListAppSessions(ctx, useDefaultPageSize, token, "")
		require.NoError(t, err)
		if token == "" {
			require.Len(t, sessions, 10)
			break
		} else {
			require.Len(t, sessions, maxSessionPageSize)
		}
	}

	// reset token
	token = ""

	// Validate that sessions are retrieved per user with
	// a page size of 11
	for _, user := range users {
		for {
			sessions, token, err = identity.ListAppSessions(ctx, 11, token, user)
			require.NoError(t, err)

			for _, session := range sessions {
				require.Equal(t, user, session.GetUser())
			}

			if token == "" {
				require.Len(t, sessions, 7)
				break
			} else {
				require.Len(t, sessions, 11)
			}
		}
	}
}
