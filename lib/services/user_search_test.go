/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type testUserSearchLister struct {
	users   []*types.UserV2
	err     error
	nextKey string
	calls   int
	request *userspb.ListUsersRequest
}

func (l *testUserSearchLister) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	l.calls++
	l.request = req
	if l.err != nil {
		return nil, l.err
	}

	return userspb.ListUsersResponse_builder{
		Users:         l.users,
		NextPageToken: l.nextKey,
	}.Build(), nil
}

func TestFindUsernamesBySearchKeywords(t *testing.T) {
	t.Parallel()

	const janeUsername = "123456"

	jane, err := types.NewUser(janeUsername)
	require.NoError(t, err)

	lister := &testUserSearchLister{
		users: []*types.UserV2{jane.(*types.UserV2)},
	}

	usernames, err := services.FindUsernamesBySearchKeywords(t.Context(), lister, []string{"Jane", "Garcia"})
	require.NoError(t, err)
	require.Equal(t, map[string]struct{}{janeUsername: {}}, usernames)

	// Verify that only one page of users was requested with the expected filter.
	require.Equal(t, 1, lister.calls)
	require.Equal(t, int32(apidefaults.DefaultChunkSize), lister.request.GetPageSize())
	require.Equal(t, []string{"Jane", "Garcia"}, lister.request.GetFilter().SearchKeywords)
	require.True(t, lister.request.GetFilter().SkipSystemUsers)

	_, err = services.FindUsernamesBySearchKeywords(t.Context(), &testUserSearchLister{err: errors.New("backend unavailable")}, []string{"Jane"})
	require.Error(t, err)
}

func TestFindUsernamesBySearchKeywordsSkipsBlankSearch(t *testing.T) {
	t.Parallel()

	lister := &testUserSearchLister{}
	usernames, err := services.FindUsernamesBySearchKeywords(t.Context(), lister, []string{"", "   "})
	require.NoError(t, err)
	require.Nil(t, usernames)
	require.Zero(t, lister.calls)
}
