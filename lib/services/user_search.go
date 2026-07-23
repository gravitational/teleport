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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var userSearchLogger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.Component("user", "search"))

// UserSearchLister lists users for display-value search.
type UserSearchLister interface {
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
}

// usernameSet is a set of usernames.
type usernameSet map[string]struct{}

// findUsernamesBySearchKeywords returns usernames whose resolved display values match the keywords.
func findUsernamesBySearchKeywords(ctx context.Context, users UserSearchLister, searchKeywords []string) (usernameSet, error) {
	if len(searchKeywords) == 0 {
		return nil, nil
	}

	usernames := make(usernameSet, apidefaults.DefaultChunkSize)
	var pageToken string
	for {
		rsp, err := users.ListUsers(ctx, userspb.ListUsersRequest_builder{
			PageSize:  apidefaults.DefaultChunkSize,
			PageToken: pageToken,
			Filter: &types.UserFilter{
				SearchKeywords:  searchKeywords,
				SkipSystemUsers: true,
			},
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, user := range rsp.GetUsers() {
			display := user.GetDisplay()
			// Exclude non-display trait matches.
			if !types.MatchSearch([]string{display.Primary, display.Secondary}, searchKeywords, nil) {
				continue
			}
			usernames[user.GetName()] = struct{}{}
			if len(usernames) == apidefaults.DefaultChunkSize {
				return usernames, nil
			}
		}

		pageToken = rsp.GetNextPageToken()
		if pageToken == "" {
			return usernames, nil
		}
	}
}

type searchKeywordUsernameResolver struct {
	users UserSearchLister
	// usernamesBySearchKeyword caches the resolved usernames for each keyword.
	usernamesBySearchKeyword map[string]usernameSet
}

// NewSearchKeywordUsernameResolver returns a memoizing resolver for search-keyword username matches.
func NewSearchKeywordUsernameResolver(users UserSearchLister) func(context.Context, string) map[string]struct{} {
	resolver := &searchKeywordUsernameResolver{
		users:                    users,
		usernamesBySearchKeyword: make(map[string]usernameSet),
	}
	return resolver.resolveUsernames
}

func (r *searchKeywordUsernameResolver) resolveUsernames(ctx context.Context, searchKeyword string) map[string]struct{} {
	searchKeyword = strings.TrimSpace(searchKeyword)
	if searchKeyword == "" {
		return nil
	}

	if usernames, ok := r.usernamesBySearchKeyword[searchKeyword]; ok {
		return usernames
	}

	usernames, err := findUsernamesBySearchKeywords(ctx, r.users, []string{searchKeyword})
	if err != nil {
		userSearchLogger.WarnContext(ctx, "Failed to resolve search keyword to users",
			"search_keywords", []string{searchKeyword},
			"error", err,
		)
		r.usernamesBySearchKeyword[searchKeyword] = nil
		return nil
	}

	r.usernamesBySearchKeyword[searchKeyword] = usernames
	return usernames
}

// NewAccessRequestSearchMatcher returns a matcher that checks stored request fields and requester user-search matches.
func NewAccessRequestSearchMatcher(searchKeywords []string, users UserSearchLister) func(context.Context, *types.AccessRequestV3) bool {
	resolveToUsernames := NewSearchKeywordUsernameResolver(users)

	return func(ctx context.Context, accessRequest *types.AccessRequestV3) bool {
		return types.MatchSearch(accessRequest.SearchableFields(), searchKeywords, func(searchKeyword string) bool {
			_, ok := resolveToUsernames(ctx, searchKeyword)[accessRequest.GetUser()]
			return ok
		})
	}
}
