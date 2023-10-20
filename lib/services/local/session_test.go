// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	identity := NewIdentityService(backend)
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
	require.Len(t, sessions, 0)
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

	identity := NewIdentityService(backend)

	users := []string{"alice", "bob"}
	ctx := context.Background()

	// the default page size is used if the pageSize
	// provide to ListAppSessions is 0 || > maxPageSize
	const useDefaultPageSize = 0

	// Validate no sessions exist
	sessions, token, err := identity.ListAppSessions(ctx, useDefaultPageSize, "", "")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.Empty(t, token)

	// Create 3 pages worth of sessions. One full
	// page per user and one partial page with 5
	// sessions per user.
	for i := 0; i < maxPageSize+5; i++ {
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

	// Validate page size is truncated to maxPageSize
	sessions, token, err = identity.ListAppSessions(ctx, maxPageSize+maxPageSize*2/3, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, maxPageSize)
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
			require.Len(t, sessions, maxPageSize)
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

func TestDeleteUserSAMLIdPSessions(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	identity := NewIdentityService(backend)
	users := []string{"alice", "bob"}
	ctx := context.Background()

	// Create SAML IdP sessions for different users.
	for _, user := range users {
		session, err := types.NewWebSession(uuid.New().String(), types.KindSAMLIdPSession, types.WebSessionSpecV2{
			User:    user,
			Expires: clock.Now().Add(time.Hour),
		})
		require.NoError(t, err)

		err = identity.UpsertSAMLIdPSession(ctx, session)
		require.NoError(t, err)
	}

	// Ensure the number of SAML IdP sessions is correct.
	sessions, nextKey, err := identity.ListSAMLIdPSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	require.Empty(t, nextKey)

	// Delete sessions of the first user.
	err = identity.DeleteUserSAMLIdPSessions(ctx, users[0])
	require.NoError(t, err)

	sessions, nextKey, err = identity.ListSAMLIdPSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, users[1], sessions[0].GetUser())
	require.Empty(t, nextKey)

	// Delete sessions of the second user.
	err = identity.DeleteUserSAMLIdPSessions(ctx, users[1])
	require.NoError(t, err)

	sessions, nextKey, err = identity.ListSAMLIdPSessions(ctx, 10, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, 0)
	require.Empty(t, nextKey)
}

func TestListSAMLIdPSessions(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	identity := NewIdentityService(backend)

	users := []string{"alice", "bob"}
	ctx := context.Background()

	// the default page size is used if the pageSize
	// provide to ListSAMLIdPSessions is 0 || > maxPageSize
	const useDefaultPageSize = 0

	// Validate no sessions exist
	sessions, token, err := identity.ListSAMLIdPSessions(ctx, useDefaultPageSize, "", "")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.Empty(t, token)

	// Create 3 pages worth of sessions. One full
	// page per user and one partial page with 5
	// sessions per user.
	for i := 0; i < maxPageSize+5; i++ {
		for _, user := range users {
			session, err := types.NewWebSession(uuid.New().String(), types.KindSAMLIdPSession, types.WebSessionSpecV2{
				User:    user,
				Expires: clock.Now().Add(time.Hour),
			})
			require.NoError(t, err)

			err = identity.UpsertSAMLIdPSession(ctx, session)
			require.NoError(t, err)
		}
	}

	// Validate page size is truncated to maxPageSize
	sessions, token, err = identity.ListSAMLIdPSessions(ctx, maxPageSize+maxPageSize*2/3, "", "")
	require.NoError(t, err)
	require.Len(t, sessions, maxPageSize)
	require.NotEmpty(t, token)

	// reset token
	token = ""

	// Validate that sessions are retrieved for all users
	// with the default page size
	for {
		sessions, token, err = identity.ListSAMLIdPSessions(ctx, useDefaultPageSize, token, "")
		require.NoError(t, err)
		if token == "" {
			require.Len(t, sessions, 10)
			break
		} else {
			require.Len(t, sessions, maxPageSize)
		}
	}

	// reset token
	token = ""

	// Validate that sessions are retrieved per user with
	// a page size of 11
	for _, user := range users {
		for {
			sessions, token, err = identity.ListSAMLIdPSessions(ctx, 11, token, user)
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
