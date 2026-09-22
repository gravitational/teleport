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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func setAccessRequestUserInfo(ctx context.Context, getter services.UserGetter, req types.AccessRequest, serverSuggestedReviewers []string, logger *slog.Logger) {
	displays, err := services.ResolveUserDisplays(ctx, getter, append([]string{req.GetUser()}, serverSuggestedReviewers...))
	if err != nil {
		logger.WarnContext(ctx, "Failed to resolve access request user displays", "request_id", req.GetName(), "error", err)
		// No display resolved from server, drop whatever displays the client might have sent.
		req.SetUserInfo(nil)
		req.SetSuggestedReviewersInfo(nil)
		return
	}
	req.SetUserInfo(types.NewAccessRequestUserInfo(displays[req.GetUser()]))

	var reviewerInfo map[string]types.AccessRequestUserInfo
	for _, username := range serverSuggestedReviewers {
		if info := types.NewAccessRequestUserInfo(displays[username]); info != nil {
			if reviewerInfo == nil {
				reviewerInfo = make(map[string]types.AccessRequestUserInfo)
			}
			reviewerInfo[username] = *info
		}
	}
	req.SetSuggestedReviewersInfo(reviewerInfo)
}

func setAccessReviewAuthorInfo(ctx context.Context, getter services.UserGetter, rev *types.AccessReview, logger *slog.Logger) {
	displays, err := services.ResolveUserDisplays(ctx, getter, []string{rev.Author})
	if err != nil {
		logger.WarnContext(ctx, "Failed to resolve access review author display", "author", rev.Author, "error", err)
		// No display resolved from server, drop whatever displays the client might have sent.
		rev.AuthorInfo = nil
		return
	}
	rev.AuthorInfo = types.NewAccessRequestUserInfo(displays[rev.Author])
}
