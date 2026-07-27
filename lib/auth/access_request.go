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
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// fetchAccessRequests lists access requests, filtered by match if non-nil, and attaches users display values.
func (a *Server) fetchAccessRequests(ctx context.Context, req *proto.ListAccessRequestsRequest, match func(*types.AccessRequestV3) bool) (*proto.ListAccessRequestsResponse, error) {
	var rsp *proto.ListAccessRequestsResponse
	var err error
	if match == nil {
		rsp, err = a.ListAccessRequests(ctx, req)
	} else {
		rsp, err = a.ListMatchingAccessRequests(ctx, req, match)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addAccessRequestUserDisplays(ctx, rsp, a, a.logger)
	return rsp, nil
}

// addAccessRequestUserDisplays is best-effort: on resolution failure the page
// is left without display values.
func addAccessRequestUserDisplays(ctx context.Context, rsp *proto.ListAccessRequestsResponse, getter services.UserGetter, logger *slog.Logger) {
	if len(rsp.AccessRequests) == 0 {
		return
	}

	displays, err := resolveAccessRequestUserDisplays(ctx, getter, rsp.AccessRequests)
	if err != nil {
		logger.WarnContext(ctx, "Failed to resolve user displays for access request page", "error", err)
		return
	}
	rsp.UserDisplays = displays
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
		usernames = append(usernames, req.GetReferencedUsers()...)
	}
	return usernames
}

// addAccessRequestDryRunUserDisplays attaches display values for the supplied
// usernames to the request's dry-run enrichment. Best-effort: on resolution
// failure the enrichment is left without display values.
func addAccessRequestDryRunUserDisplays(ctx context.Context, req types.AccessRequest, usernames []string, getter services.UserGetter, logger *slog.Logger) {
	displays, err := services.ResolveUserDisplays(ctx, getter, usernames)
	if err != nil {
		logger.WarnContext(ctx, "Failed to resolve user displays for dry-run access request", "error", err)
		return
	}

	enrichment := req.GetDryRunEnrichment()
	enrichment.UserDisplays = toAccessRequestUserDisplays(displays)
	req.SetDryRunEnrichment(enrichment)
}

func toAccessRequestUserDisplays(displays map[string]types.UserDisplay) map[string]types.AccessRequestUserDisplay {
	if len(displays) == 0 {
		return nil
	}

	out := make(map[string]types.AccessRequestUserDisplay, len(displays))
	for username, display := range displays {
		out[username] = types.AccessRequestUserDisplay{
			Primary:   display.Primary,
			Secondary: display.Secondary,
		}
	}
	return out
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
