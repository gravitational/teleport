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

	"github.com/gravitational/teleport/api/types"
)

// NewAccessRequestSearchMatcher returns a matcher that checks stored request fields and requester user-search matches.
func NewAccessRequestSearchMatcher(ctx context.Context, searchKeywords []string, users UserSearchLister) func(*types.AccessRequestV3) bool {
	searchKeywords = cleanSearchKeywords(searchKeywords)
	resolveToUsernames := NewSearchKeywordUsernameResolver(ctx, users)

	return func(accessRequest *types.AccessRequestV3) bool {
		return types.MatchSearch(accessRequest.SearchableFields(), searchKeywords, func(searchKeyword string) bool {
			_, ok := resolveToUsernames(searchKeyword)[accessRequest.GetUser()]
			return ok
		})
	}
}
