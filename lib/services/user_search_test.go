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

package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
)

type testUserSearchLister struct {
	users   []*types.UserV2
	err     error
	calls   int
	ctx     context.Context
	request *userspb.ListUsersRequest
}

func (l *testUserSearchLister) ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	l.calls++
	l.ctx = ctx
	l.request = req
	if l.err != nil {
		return nil, l.err
	}

	return userspb.ListUsersResponse_builder{
		Users: l.users,
	}.Build(), nil
}

func TestFindUsernamesBySearchKeywords(t *testing.T) {
	t.Parallel()

	const janeUsername = "123456"

	jane, err := types.NewUser(janeUsername)
	require.NoError(t, err)
	jane.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
	})

	lister := &testUserSearchLister{
		users: []*types.UserV2{jane.(*types.UserV2)},
	}

	usernames, err := findUsernamesBySearchKeywords(t.Context(), lister, []string{"Jane", "Garcia"})
	require.NoError(t, err)
	require.Equal(t, usernameSet{janeUsername: {}}, usernames)

	// Verify that only one page of users was requested with the expected filter.
	require.Equal(t, 1, lister.calls)
	require.Equal(t, int32(apidefaults.DefaultChunkSize), lister.request.GetPageSize())
	require.Equal(t, []string{"Jane", "Garcia"}, lister.request.GetFilter().SearchKeywords)
	require.True(t, lister.request.GetFilter().SkipSystemUsers)

	_, err = findUsernamesBySearchKeywords(t.Context(), &testUserSearchLister{err: errors.New("backend unavailable")}, []string{"Jane"})
	require.Error(t, err)
}

func TestFindUsernamesBySearchKeywordsMatchesOnlyDisplayValues(t *testing.T) {
	t.Parallel()

	const janeUsername = "123456"

	jane, err := types.NewUser(janeUsername)
	require.NoError(t, err)
	jane.SetRoles([]string{"finance-role"})
	jane.SetStaticLabels(map[string]string{
		"cost-center": "secret-cost-center",
	})
	jane.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
		"okta/email":       {"jane.garcia@example.com"},
		"department":       {"classified-department"},
	})
	janeV2 := jane.(*types.UserV2)

	tests := []struct {
		name     string
		keyword  string
		expected usernameSet
	}{
		{
			name:     "primary display",
			keyword:  "Jane",
			expected: usernameSet{janeUsername: {}},
		},
		{
			name:     "secondary display",
			keyword:  "jane.garcia@example.com",
			expected: usernameSet{janeUsername: {}},
		},
		{name: "role", keyword: "finance-role"},
		{name: "label key", keyword: "cost-center"},
		{name: "label value", keyword: "secret-cost-center"},
		{name: "trait key", keyword: "department"},
		{name: "trait value", keyword: "classified-department"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.True(t, janeV2.MatchSearch([]string{test.keyword}))

			usernames, err := findUsernamesBySearchKeywords(t.Context(), &testUserSearchLister{
				users: []*types.UserV2{janeV2},
			}, []string{test.keyword})
			require.NoError(t, err)
			if test.expected == nil {
				require.Empty(t, usernames)
				return
			}
			require.Equal(t, test.expected, usernames)
		})
	}
}

func TestSearchKeywordUsernameResolver(t *testing.T) {
	t.Parallel()

	const janeUsername = "123456"

	jane, err := types.NewUser(janeUsername)
	require.NoError(t, err)
	jane.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
	})

	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "test-value")
	lister := &testUserSearchLister{users: []*types.UserV2{jane.(*types.UserV2)}}
	resolveUsernames := NewSearchKeywordUsernameResolver(lister)

	require.Equal(t, map[string]struct{}{janeUsername: {}}, resolveUsernames(ctx, " Jane "))
	require.Equal(t, map[string]struct{}{janeUsername: {}}, resolveUsernames(ctx, "Jane"))
	require.Equal(t, 1, lister.calls)
	require.Equal(t, ctx, lister.ctx)
	require.Nil(t, resolveUsernames(ctx, "   "))
	require.Equal(t, 1, lister.calls)
}

func TestNewAccessRequestSearchMatcherResolvesLazilyAndMemoizes(t *testing.T) {
	t.Parallel()

	const janeUsername = "123456"

	jane, err := types.NewUser(janeUsername)
	require.NoError(t, err)
	jane.SetTraits(map[string][]string{
		"okta/displayName": {"Jane Garcia"},
	})

	accessRequest, err := types.NewAccessRequest("request", janeUsername, "db-access")
	require.NoError(t, err)
	accessRequestV3 := accessRequest.(*types.AccessRequestV3)
	lister := &testUserSearchLister{users: []*types.UserV2{jane.(*types.UserV2)}}

	storedFieldMatcher := NewAccessRequestSearchMatcher([]string{"db-access"}, lister)
	require.True(t, storedFieldMatcher(t.Context(), accessRequestV3))
	require.Zero(t, lister.calls)

	displayMatcher := NewAccessRequestSearchMatcher([]string{"Jane"}, lister)
	require.True(t, displayMatcher(t.Context(), accessRequestV3))
	require.True(t, displayMatcher(t.Context(), accessRequestV3))
	require.Equal(t, 1, lister.calls)
}

func TestNewAccessRequestSearchMatcherDegradesGracefullyOnUserLookupFailure(t *testing.T) {
	t.Parallel()

	const displayOnlyUsername = "123456"

	displayOnly, err := types.NewAccessRequest("display-only", displayOnlyUsername, "role")
	require.NoError(t, err)

	resolvedUser, err := types.NewUser(displayOnlyUsername)
	require.NoError(t, err)
	resolvedUser.SetTraits(map[string][]string{
		"okta/displayName": {"Jane"},
	})
	resolvedUsers := []*types.UserV2{resolvedUser.(*types.UserV2)}

	successLister := &testUserSearchLister{users: resolvedUsers}
	successMatcher := NewAccessRequestSearchMatcher([]string{"Jane"}, successLister)
	require.True(t, successMatcher(t.Context(), displayOnly.(*types.AccessRequestV3)), "display-only match should pass when user lookup resolves the requester")

	failedLister := &testUserSearchLister{
		users: resolvedUsers,
		err:   errors.New("backend unavailable"),
	}
	failedMatcher := NewAccessRequestSearchMatcher([]string{"Jane"}, failedLister)
	// Gracefully degrade to return false when user lookup fails.
	require.False(t, failedMatcher(t.Context(), displayOnly.(*types.AccessRequestV3)), "same display-only match should be skipped when user lookup fails")
}
