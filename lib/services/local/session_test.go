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
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
	for range maxSessionPageSize + 5 {
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

func TestListExpiredAppSessions(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		backend, _ := memory.New(memory.Config{Context: ctx})
		identity, _ := NewTestIdentityService(backend)

		const totalExpired = 200
		const totalValid = 10

		// Create 210 sessions: 5 valid, 100 expired, 5 valid, 100 expired
		for i := range totalValid / 2 {
			sess := newTestAppSession(t, fmt.Sprintf("valid-%d", i))
			sess.SetExpiry(time.Now().Add(1 * time.Hour))
			err := identity.UpsertAppSession(ctx, sess)
			require.NoError(t, err)
		}

		for i := range totalExpired / 2 {
			sess := newTestAppSession(t, fmt.Sprintf("expired-%d", i))
			sess.SetExpiry(time.Now().Add(-30 * time.Minute))
			err := identity.UpsertAppSession(ctx, sess)
			require.NoError(t, err)
		}

		for i := range totalValid / 2 {
			sess := newTestAppSession(t, fmt.Sprintf("valid-%d", i+5))
			sess.SetExpiry(time.Now().Add(1 * time.Hour))
			err := identity.UpsertAppSession(ctx, sess)
			require.NoError(t, err)
		}

		for i := range totalExpired / 2 {
			sess := newTestAppSession(t, fmt.Sprintf("expired-%d", i+100))
			sess.SetExpiry(time.Now().Add(-30 * time.Minute))
			err := identity.UpsertAppSession(ctx, sess)
			require.NoError(t, err)
		}

		// Use arbitrary page size to force pagination logic to be exercised
		expired, err := stream.Collect(clientutils.ResourcesWithPageSize(
			ctx,
			identity.ListExpiredAppSessions,
			67,
		))
		require.NoError(t, err)
		assert.Len(t, expired, totalExpired)

		// List single page
		expired, nextToken, err := identity.ListExpiredAppSessions(ctx, 67, "")
		require.NoError(t, err)
		assert.Len(t, expired, 67)
		assert.NotEmpty(t, nextToken)

		time.Sleep(2 * time.Hour)

		// All 210 sessions should be expired
		allExpired, err := stream.Collect(clientutils.Resources(ctx, identity.ListExpiredAppSessions))
		require.NoError(t, err)
		assert.Len(t, allExpired, totalExpired+totalValid)
	})
}

func TestUpdateAppSession_UnsetBackendExpiry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mem, err := memory.New(memory.Config{Context: ctx})
	require.NoError(t, err)

	identity, err := NewTestIdentityService(mem)
	require.NoError(t, err)

	session := newTestAppSession(t, "updated-session")
	require.NoError(t, identity.UpsertAppSession(ctx, session))

	item, err := mem.Get(ctx, backend.NewKey(appsPrefix, sessionsPrefix, session.GetName()))
	require.NoError(t, err)
	require.True(t, item.Expires.IsZero(), "new app sessions should not have backend TTL")

	session, err = identity.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: session.GetName()})
	require.NoError(t, err)

	testDBSCPublicKey := []byte("test-dbsc-key")
	session.SetDBSCPublicKey(testDBSCPublicKey)
	require.NoError(t, identity.UpdateAppSession(ctx, session))

	item, err = mem.Get(ctx, backend.NewKey(appsPrefix, sessionsPrefix, session.GetName()))
	require.NoError(t, err)
	require.True(t, item.Expires.IsZero(), "updated app sessions should not regain backend TTL")
}

// Helper for quick session generation
func newTestAppSession(t *testing.T, name string) types.WebSession {
	t.Helper()
	s, err := types.NewWebSession(name, types.KindAppSession, types.WebSessionSpecV2{
		User:    "alice",
		Expires: time.Now().Add(12 * time.Hour),
	})
	require.NoError(t, err)
	return s
}

func TestListSnowflakeSessions(t *testing.T) {
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
	// provide to ListSnowflakeSessions is 0 || > maxSessionPageSize
	const useDefaultPageSize = 0

	// Validate no sessions exist
	sessions, next, err := identity.ListSnowflakeSessions(ctx, useDefaultPageSize, "")
	require.NoError(t, err)
	require.Empty(t, sessions)
	require.Empty(t, next)

	// Create 3 pages worth of sessions. One full
	// page per user and one partial page with 5
	// sessions per user.
	var expected []types.WebSession
	for range maxSessionPageSize + 5 {
		for _, user := range users {
			session, err := types.NewWebSession(uuid.New().String(), types.KindSnowflakeSession, types.WebSessionSpecV2{
				User:    user,
				Expires: clock.Now().Add(time.Hour),
			})
			require.NoError(t, err)

			err = identity.UpsertSnowflakeSession(ctx, session)
			require.NoError(t, err)
			expected = append(expected, session)
		}
	}

	// Validate page size is truncated to maxSessionPageSize
	sessions, next, err = identity.ListSnowflakeSessions(ctx, maxSessionPageSize+maxSessionPageSize*2/3, "")
	require.NoError(t, err)
	require.Len(t, sessions, maxSessionPageSize)
	require.NotEmpty(t, next)

	opts := []cmp.Option{
		cmpopts.SortSlices(func(a, b types.WebSession) bool {
			return a.GetName() < b.GetName()
		}),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	}

	sessions, err = stream.Collect(clientutils.Resources(ctx, identity.ListSnowflakeSessions))
	require.NoError(t, err)
	require.Len(t, sessions, len(expected))
	require.Empty(t, cmp.Diff(expected, sessions, opts...))

	// reset token
	next = ""

	// Validate that sessions are retrieved for all users
	// with the default page size
	for {
		sessions, next, err = identity.ListSnowflakeSessions(ctx, useDefaultPageSize, next)
		require.NoError(t, err)
		if next == "" {
			require.Len(t, sessions, 10)
			break
		} else {
			require.Len(t, sessions, maxSessionPageSize)
		}
	}
}

func TestWebTokenCRUD(t *testing.T) {
	ctx := t.Context()
	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	identity, err := NewTestIdentityService(backend)
	require.NoError(t, err)

	newToken := func(name, user string) types.WebToken {

		// types.NewWebToken
		expires := clock.Now().Add(time.Hour)
		token, err := types.NewWebToken(expires, types.WebTokenSpecV3{
			Token: name,
			User:  user,
		})

		require.NoError(t, err)
		return token
	}

	// Initially we expect no tokens.
	out, err := identity.GetWebTokens(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	out, next, err := identity.ListWebTokens(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, next)

	out, err = stream.Collect(identity.RangeWebTokens(ctx, "", ""))
	require.NoError(t, err)
	require.Empty(t, out)

	// Create some tokens.
	var expected []types.WebToken
	for i := range 5 {
		tk := newToken(fmt.Sprintf("resource-%d", i), "bob")
		err := identity.UpsertWebToken(ctx, tk)
		require.NoError(t, err)
		expected = append(expected, tk)
	}

	// Fetch all tokens.
	out, err = identity.GetWebTokens(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, next, err = identity.ListWebTokens(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
	require.Empty(t, next)

	out, err = stream.Collect(identity.RangeWebTokens(ctx, "", ""))
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Fetch a specific token.
	token, err := identity.GetWebToken(ctx, types.GetWebTokenRequest{
		Token: expected[1].GetName(),
		User:  expected[1].GetUser(),
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected[1], token,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to fetch a token that doesn't exist.
	_, err = identity.GetWebToken(ctx, types.GetWebTokenRequest{
		Token: "doesnotexist",
		User:  "alice",
	})
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	// Upsert.
	expected[1].SetUser("alice")
	err = identity.UpsertWebToken(ctx, expected[1])
	require.NoError(t, err)
	token, err = identity.GetWebToken(ctx, types.GetWebTokenRequest{
		Token: expected[1].GetName(),
		User:  expected[1].GetUser(),
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected[1], token,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	page1, page2Start, err := identity.ListWebTokens(ctx, 2, "")
	require.NoError(t, err)
	assert.Len(t, page1, 2)
	assert.NotEmpty(t, page2Start)

	page2, next, err := identity.ListWebTokens(ctx, 1000, page2Start)
	require.NoError(t, err)
	assert.Len(t, page2, len(expected)-2)
	assert.Empty(t, next)

	listed := append(page1, page2...)

	assert.Empty(t, cmp.Diff(expected, listed,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, err = stream.Collect(identity.RangeWebTokens(ctx, "", page2Start))
	require.NoError(t, err)
	assert.Len(t, out, len(page1))
	assert.Empty(t, cmp.Diff(page1, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, err = stream.Collect(identity.RangeWebTokens(ctx, "", ""))
	require.NoError(t, err)
	assert.Len(t, out, len(expected))
	assert.Empty(t, cmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, err = stream.Collect(identity.RangeWebTokens(ctx, page2Start, ""))
	require.NoError(t, err)
	assert.Len(t, out, len(expected)-2)
	assert.Empty(t, cmp.Diff(expected, append(page1, out...),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Try to delete a token that doesn't exist.
	err = identity.DeleteWebToken(ctx, types.DeleteWebTokenRequest{
		Token: "doesnotexist",
		User:  "doesnotexist",
	})
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	err = identity.DeleteWebToken(ctx, types.DeleteWebTokenRequest{
		Token: expected[0].GetToken(),
		User:  expected[0].GetUser(),
	})
	require.NoError(t, err)

	// Verify deleted
	out, err = stream.Collect(identity.RangeWebTokens(ctx, "", ""))
	require.NoError(t, err)
	assert.Len(t, out, len(expected)-1)
	assert.Empty(t, cmp.Diff(expected[1:], out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// Delete all tokens.
	err = identity.DeleteAllWebTokens(ctx)
	require.NoError(t, err)
	out, err = identity.GetWebTokens(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	out, next, err = identity.ListWebTokens(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, next)

	out, err = stream.Collect(identity.RangeWebTokens(ctx, "", ""))
	require.NoError(t, err)
	require.Empty(t, out)

}
