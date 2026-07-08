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

package auth

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const (
	displayNameTrait = "displayName"
	emailTrait       = "email"
)

func TestResolveAccessRequestUserDisplays(t *testing.T) {
	t.Parallel()

	const (
		requester         = "display-requester"
		reviewer          = "display-reviewer"
		suggestedReviewer = "display-suggested-reviewer"
		plainUser         = "display-plain-user"
		missingUser       = "display-missing-user"
	)

	req := mustAccessRequestV3(t, requester)
	req.SetReviews([]types.AccessReview{
		{Author: reviewer},
		{Author: requester},
	})
	req.SetSuggestedReviewers([]string{suggestedReviewer, plainUser, missingUser})

	getter := &userDisplayGetter{
		users: map[string]types.User{
			requester:         newDisplayUser(t, requester, "Request User", "requester@example.com"),
			reviewer:          newDisplayUser(t, reviewer, "Review User", "reviewer@example.com"),
			suggestedReviewer: newDisplayUser(t, suggestedReviewer, "Suggested User", "suggested@example.com"),
			plainUser:         newDisplayUser(t, plainUser, "", ""),
		},
	}

	got, err := resolveAccessRequestUserDisplays(context.Background(), getter, []*types.AccessRequestV3{req})
	require.NoError(t, err)

	require.Equal(t, map[string]*proto.UserDisplay{
		requester: {
			Primary:   "Request User",
			Secondary: "requester@example.com",
		},
		reviewer: {
			Primary:   "Review User",
			Secondary: "reviewer@example.com",
		},
		suggestedReviewer: {
			Primary:   "Suggested User",
			Secondary: "suggested@example.com",
		},
		plainUser: {},
	}, got)
	require.NotContains(t, got, missingUser)
}

func TestAddAccessRequestUserDisplaysKeepsResponseOnResolveError(t *testing.T) {
	t.Parallel()

	const requester = "display-requester"

	req := mustAccessRequestV3(t, requester)
	listRsp := &proto.ListAccessRequestsResponse{
		AccessRequests: []*types.AccessRequestV3{req},
	}

	addAccessRequestUserDisplays(
		context.Background(),
		listRsp,
		&userDisplayGetter{
			failFor: map[string]error{
				requester: errors.New("backend unavailable"),
			},
		},
		slog.New(slog.DiscardHandler),
	)
	require.Equal(t, []*types.AccessRequestV3{req}, listRsp.AccessRequests)
	require.Empty(t, listRsp.UserDisplays)
}

type userDisplayGetter struct {
	users   map[string]types.User
	failFor map[string]error
}

func (g *userDisplayGetter) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	if err, ok := g.failFor[name]; ok {
		return nil, err
	}
	if user, ok := g.users[name]; ok {
		return user, nil
	}
	return nil, trace.NotFound("user %q not found", name)
}

func mustAccessRequestV3(t *testing.T, user string) *types.AccessRequestV3 {
	t.Helper()

	req, err := services.NewAccessRequest(user, "display-role")
	require.NoError(t, err)

	reqV3, ok := req.(*types.AccessRequestV3)
	require.True(t, ok)
	return reqV3
}

func newDisplayUser(t *testing.T, name, primary, secondary string) types.User {
	t.Helper()

	user, err := types.NewUser(name)
	require.NoError(t, err)
	traits := make(map[string][]string)
	if primary != "" {
		traits[displayNameTrait] = []string{primary}
	}
	if secondary != "" {
		traits[emailTrait] = []string{secondary}
	}
	user.SetTraits(traits)
	return user
}
