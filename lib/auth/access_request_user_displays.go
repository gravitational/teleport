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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func (a *ServerWithRoles) addAccessRequestUserDisplays(ctx context.Context, req *proto.ListAccessRequestsRequest, rsp *proto.ListAccessRequestsResponse, err error) (*proto.ListAccessRequestsResponse, error) {
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !req.GetIncludeUserDisplays() || len(rsp.AccessRequests) == 0 {
		return rsp, nil
	}

	displays, err := resolveAccessRequestUserDisplays(ctx, a.authServer, rsp.AccessRequests)
	if err != nil {
		a.authServer.logger.WarnContext(ctx, "Failed to resolve user displays for access request page", "error", err)
		return rsp, nil
	}
	rsp.UserDisplays = displays
	return rsp, nil
}

func resolveAccessRequestUserDisplays(ctx context.Context, getter services.UserGetter, requests []*types.AccessRequestV3) (map[string]*proto.UserDisplay, error) {
	displays, err := services.ResolveUserDisplays(ctx, getter, accessRequestUsernames(requests))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return toProtoUserDisplays(displays), nil
}

func accessRequestUsernames(requests []*types.AccessRequestV3) []string {
	var usernames []string
	for _, req := range requests {
		usernames = append(usernames, req.GetUser())
		for _, review := range req.GetReviews() {
			usernames = append(usernames, review.Author)
		}
		usernames = append(usernames, req.GetSuggestedReviewers()...)
	}
	return usernames
}

func toProtoUserDisplays(displays map[string]types.UserDisplay) map[string]*proto.UserDisplay {
	if displays == nil {
		return nil
	}

	out := make(map[string]*proto.UserDisplay, len(displays))
	for username, display := range displays {
		out[username] = &proto.UserDisplay{
			Primary:   display.Primary,
			Secondary: display.Secondary,
		}
	}
	return out
}
