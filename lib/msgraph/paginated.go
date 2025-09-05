// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package msgraph

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

type iterateConfig struct {
	// filter is the $filter query param.
	// https://learn.microsoft.com/en-us/graph/filter-query-parameter?tabs=http
	filter string
	// top is the $top query param.
	// https://learn.microsoft.com/en-us/graph/query-parameters?tabs=http#top
	top int
	// selector is the $select query param.
	// https://learn.microsoft.com/en-us/graph/query-parameters?tabs=http#select
	selector string
	// header includes headers that are going to be set during iteration.
	header http.Header
	// count is the $count query param.
	// https://learn.microsoft.com/en-us/graph/query-parameters?tabs=http#count
	count bool
}

func (ic *iterateConfig) query() url.Values {
	q := make(url.Values)
	if ic.filter != "" {
		q.Set("$filter", ic.filter)
	}
	if ic.top > 0 {
		q.Set("$top", strconv.Itoa(ic.top))
	}
	if ic.selector != "" {
		q.Set("$select", ic.selector)
	}
	if ic.count {
		q.Set("$count", "true")
	}
	return q
}

func (c *Client) newIterateConfig() *iterateConfig {
	return &iterateConfig{
		top:    c.pageSize,
		header: make(http.Header),
	}
}

// IterateOpt is a function that can be passed to [Client] methods that iterate over API results.
type IterateOpt func(*iterateConfig)

// WithFilter sets the $filter query param.
// https://learn.microsoft.com/en-us/graph/filter-query-parameter?tabs=http
func WithFilter(filter string) IterateOpt {
	return func(ic *iterateConfig) {
		ic.filter = filter
	}
}

// WithTop sets the $top query param. It overrides the default page size set in [Config].
// https://learn.microsoft.com/en-us/graph/query-parameters?tabs=http#top
func WithTop(top int) IterateOpt {
	return func(ic *iterateConfig) {
		ic.top = top
	}
}

// WithSelect sets the $select query param.
// https://learn.microsoft.com/en-us/graph/query-parameters?tabs=http#select
func WithSelect(s string) IterateOpt {
	return func(ic *iterateConfig) {
		ic.selector = s
	}
}

// WithCount sets the $count query param.
// https://learn.microsoft.com/en-us/graph/query-parameters?tabs=http#count
func WithCount() IterateOpt {
	return func(ic *iterateConfig) {
		ic.count = true
	}
}

// WithHeader sets the value of a specific header.
func WithHeader(key, value string) IterateOpt {
	return func(ic *iterateConfig) {
		ic.header.Set(key, value)
	}
}

// iterateSimple implements pagination for "simple" object lists, where additional logic isn't needed
func iterateSimple[T any](c *Client, ctx context.Context, endpoint string, f func(*T) bool, iterateOpts ...IterateOpt) error {
	var err error
	itErr := c.iterate(ctx, endpoint, func(msg json.RawMessage) bool {
		var page []T
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		for _, item := range page {
			if !f(&item) {
				return false
			}
		}
		return true
	}, iterateOpts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}

func iteratePage[T any](c *Client, ctx context.Context, endpoint string, f func([]T) bool, iterateOpts ...IterateOpt) error {
	var err error
	itErr := c.iterate(ctx, endpoint, func(msg json.RawMessage) bool {
		var page []T
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		if !f(page) {
			return false
		}
		return true
	}, iterateOpts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}

// iterate implements pagination for "list" endpoints.
func (c *Client) iterate(ctx context.Context, endpoint string, f func(json.RawMessage) bool, iterateOpts ...IterateOpt) error {
	ic := c.newIterateConfig()
	for _, opt := range iterateOpts {
		opt(ic)
	}

	uri := *c.baseURL
	uri.Path = path.Join(uri.Path, endpoint)

	uri.RawQuery = ic.query().Encode()

	uriString := uri.String()
	for uriString != "" {
		resp, err := c.request(ctx, http.MethodGet, uriString, ic.header, nil /* payload */)
		if err != nil {
			return trace.Wrap(err)
		}

		var page oDataPage
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return trace.Wrap(err)
		}
		resp.Body.Close()
		uriString = page.NextLink
		if !f(page.Value) {
			break
		}
	}

	return nil
}

// IterateApplications lists all applications in the Entra ID directory using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/application-list].
func (c *Client) IterateApplications(ctx context.Context, f func(*Application) bool) error {
	return iterateSimple(c, ctx, "applications", f)
}

// IterateGroups lists all groups in the Entra ID directory using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/group-list].
func (c *Client) IterateGroups(ctx context.Context, f func(*Group) bool) error {
	return iterateSimple(c, ctx, "groups", f)
}

// IterateUsers lists all users in the Entra ID directory using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/user-list].
func (c *Client) IterateUsers(ctx context.Context, f func(*User) bool) error {
	return iterateSimple(c, ctx, "users", f)
}

// IterateServicePrincipals lists all service principals in the Entra ID directory using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list].
func (c *Client) IterateServicePrincipals(ctx context.Context, f func(principal *ServicePrincipal) bool) error {
	return iterateSimple(c, ctx, "servicePrincipals", f)
}

// IterateGroupMembers lists all members for the given Entra ID group using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/group-list-members].
func (c *Client) IterateGroupMembers(ctx context.Context, groupID string, f func(GroupMember) bool) error {
	var err error
	itErr := c.iterate(ctx, path.Join("groups", groupID, "members"), func(msg json.RawMessage) bool {
		var page []json.RawMessage
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		for _, entry := range page {
			var member GroupMember
			member, err = decodeGroupMember(entry)
			if err != nil {
				var gmErr *unsupportedGroupMember
				if errors.As(err, &gmErr) {
					slog.DebugContext(ctx, "unsupported group member", "type", gmErr.Type)
					err = nil // Reset so that we do not return the error up if this is the last entry
					continue
				} else {
					return false
				}
			}
			if !f(member) {
				return false
			}
		}
		return true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}

const (
	graphNamespaceGroups         = "microsoft.graph.group"
	graphNamespaceDirectoryRoles = "microsoft.graph.directoryRole"
)

const (
	securityGroupsFilter = `mailEnabled eq false and securityEnabled eq true`
)

// IterateUsersTransitiveMemberOf lists groups that the user is a member of
// through a direct or nested group membership.
// This method calls user's transitiveMemberOf endpoint https://learn.microsoft.com/en-us/graph/api/user-list-transitivememberof?view=graph-rest-1.0&tabs=http.
// Supported endpoints:
// - All groups and directory roles: /v1.0/users/<user-id>/transitiveMemberOf
// - Security groups: /v1.0/users/<user-id>/transitiveMemberOf/microsoft.graph.group?$filter=mailEnabled eq false and securityEnabled eq true
// - Directory roles: /v1.0/users/<user-id>/transitiveMemberOf/microsoft.graph.directoryRole
// Only group ID is extracted from the response, so the DirectoryObject struct is sufficient
// to parse groups as well ass directory roles response.
func (c *Client) IterateUsersTransitiveMemberOf(ctx context.Context, userID, groupType string, f func(*Group) bool) error {
	// MS Graph expects $count query parameter and
	// "ConsistencyLevel: eventual" header set when using
	// advanced query parameter such as $filter.
	// https://learn.microsoft.com/en-us/graph/aad-advanced-queries?tabs=http#legend
	iterateOpts := []IterateOpt{
		WithSelect("id"),
		WithCount(),
		WithHeader("ConsistencyLevel", "eventual"),
	}

	endpoint := path.Join("users", userID, "transitiveMemberOf")
	switch groupType {
	case types.EntraIDAllGroups:
		// default endpoint suffices.
	case types.EntraIDDirectoryRoles:
		endpoint = path.Join(endpoint, graphNamespaceDirectoryRoles)
	case types.EntraIDSecurityGroups:
		endpoint = path.Join(endpoint, graphNamespaceGroups)
		iterateOpts = append(iterateOpts, WithFilter(securityGroupsFilter))
	default:
		return trace.BadParameter("unexpected group type %q received, expected types are %q", groupType, types.EntraIDGroupsTypes)
	}

	var err error
	itErr := c.iterate(ctx, endpoint, func(msg json.RawMessage) bool {
		var page []Group
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		for _, item := range page {
			if !f(&item) {
				return false
			}
		}
		return true
	}, iterateOpts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}
