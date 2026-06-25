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
	"log/slog"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
)

const logKeySearchKeywords = "search_keywords"

var userSearchLogger = slog.With(teleport.ComponentKey, "user.search")

// UserSearchLister lists users for display-value search.
type UserSearchLister interface {
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
}

// FindUsernamesBySearchKeywords returns usernames whose searchable user fields match the keywords.
func FindUsernamesBySearchKeywords(ctx context.Context, users UserSearchLister, searchKeywords []string) (map[string]struct{}, error) {
	searchKeywords = cleanSearchKeywords(searchKeywords)
	if len(searchKeywords) == 0 {
		return nil, nil
	}

	rsp, err := users.ListUsers(ctx, userspb.ListUsersRequest_builder{
		PageSize: apidefaults.DefaultChunkSize,
		Filter: &types.UserFilter{
			SearchKeywords:  searchKeywords,
			SkipSystemUsers: true,
		},
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	usernames := make(map[string]struct{}, len(rsp.GetUsers()))
	for _, user := range rsp.GetUsers() {
		usernames[user.GetName()] = struct{}{}
	}

	if rsp.GetNextPageToken() != "" {
		userSearchLogger.WarnContext(ctx, "User search result truncated while resolving search keywords",
			logKeySearchKeywords, searchKeywords,
			"page_size", apidefaults.DefaultChunkSize,
		)
	}

	return usernames, nil
}

type searchKeywordUsernameResolver struct {
	ctx   context.Context
	users UserSearchLister
	// usernamesBySearchKeyword caches the resolved usernames for each keyword.
	usernamesBySearchKeyword map[string]map[string]struct{}
}

func newSearchKeywordUsernameResolver(ctx context.Context, users UserSearchLister) func(string) map[string]struct{} {
	resolver := &searchKeywordUsernameResolver{
		ctx:                      ctx,
		users:                    users,
		usernamesBySearchKeyword: make(map[string]map[string]struct{}),
	}
	return resolver.resolveUsernames
}

func (r *searchKeywordUsernameResolver) resolveUsernames(searchKeyword string) map[string]struct{} {
	searchKeyword = strings.TrimSpace(searchKeyword)
	if searchKeyword == "" {
		return nil
	}

	if usernames, ok := r.usernamesBySearchKeyword[searchKeyword]; ok {
		return usernames
	}

	usernames, err := FindUsernamesBySearchKeywords(r.ctx, r.users, []string{searchKeyword})
	if err != nil {
		userSearchLogger.WarnContext(r.ctx, "Failed to resolve search keyword to users",
			logKeySearchKeywords, []string{searchKeyword},
			"error", err,
		)
		r.usernamesBySearchKeyword[searchKeyword] = nil
		return nil
	}

	r.usernamesBySearchKeyword[searchKeyword] = usernames
	return usernames
}

func cleanSearchKeywords(searchKeywords []string) []string {
	if len(searchKeywords) == 0 {
		return nil
	}

	cleaned := make([]string, 0, len(searchKeywords))
	for _, keyword := range searchKeywords {
		keyword = strings.TrimSpace(keyword)
		if keyword != "" {
			cleaned = append(cleaned, keyword)
		}
	}
	return cleaned
}
