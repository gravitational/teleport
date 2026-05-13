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
	"iter"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/gravitational/trace"
	jsoniter "github.com/json-iterator/go"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/utils"
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

// newIterateConfigDelta creates a new iterateConfig.
// It does not set up $top query as newIterateConfig does because
// some delta endpoints like user and groups does not support it.
// Clients can explicitly pass WithTop() to include it.
func (c *Client) newIterateConfigDelta() *iterateConfig {
	return &iterateConfig{
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
		if err = utils.FastUnmarshal(msg, &page); err != nil {
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
		if err = utils.FastUnmarshal(msg, &page); err != nil {
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
		if err := jsoniter.ConfigFastest.NewDecoder(resp.Body).Decode(&page); err != nil {
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
func (c *Client) IterateApplications(ctx context.Context, f func(*models.Application) bool, opts ...IterateOpt) error {
	return iterateSimple(c, ctx, "applications", f, opts...)
}

// IterateGroups lists all groups in the Entra ID directory using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/group-list].
func (c *Client) IterateGroups(ctx context.Context, f func(*models.Group) bool, opts ...IterateOpt) error {
	return iterateSimple(c, ctx, "groups", f, opts...)
}

// IterateUsers lists all users in the Entra ID directory using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/user-list].
func (c *Client) IterateUsers(ctx context.Context, f func(*models.User) bool, opts ...IterateOpt) error {
	return iterateSimple(c, ctx, "users", f, opts...)
}

// iterateDelta implements pagination for Graph delta API endpoints.
// It expects a valid delta link for the [endpoint] available in the [ds].
func (c *Client) iterateDelta(ctx context.Context, endpoint string, ds DeltaStore) iter.Seq2[json.RawMessage, error] {
	if ds == nil {
		return func(yield func(json.RawMessage, error) bool) {
			yield(nil, trace.BadParameter("missing delta store"))
		}
	}
	deltaURI := ds.Get(endpoint)
	if deltaURI == "" {
		return func(yield func(json.RawMessage, error) bool) {
			yield(nil, trace.Wrap(ErrMissingDeltaLink))
		}
	}

	// Below, the delta link host is checked against the baseURL host
	// which has already gone through validation when constructing the
	// graph client. This isn't strictly necessary because as per the delta
	// API docs, the client must save the whole delta link and use it as it
	// is in the next delta request.
	// https://learn.microsoft.com/en-us/graph/delta-query-overview#state-tokens
	// https://learn.microsoft.com/en-us/graph/api/group-delta?view=graph-rest-1.0&tabs=http
	if err := validateDeltaLink(c.baseURL, deltaURI); err != nil {
		return func(yield func(json.RawMessage, error) bool) {
			yield(nil, trace.Wrap(err))
		}
	}

	// For the first request, uriString will be the same as deltaURI.
	// If response is paginated, uriString will be assigned
	// with a new NextLink.
	uriString := deltaURI
	// No extra headers expected for delta query.
	header := make(http.Header)

	return func(yield func(json.RawMessage, error) bool) {
		var deltaLink string

		for uriString != "" {
			resp, err := c.request(ctx, http.MethodGet, uriString, header, nil /* payload */)
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}

			var page models.ODataPage
			if err := jsoniter.ConfigFastest.NewDecoder(resp.Body).Decode(&page); err != nil {
				resp.Body.Close()
				yield(nil, trace.Wrap(err))
				return
			}

			resp.Body.Close()
			uriString = page.NextLink

			if page.DeltaLink != "" {
				deltaLink = page.DeltaLink
			}

			if !yield(page.Value, nil) {
				return
			}
		}

		if deltaLink != "" {
			ds.Set(endpoint, deltaLink)
		}
	}
}

// IterateUserDeltas iterates over users delta response.
// A delta token for the user endpont
// must be set up before calling this method.
func (c *Client) IterateUserDeltas(
	ctx context.Context,
	endpoint string,
	ds DeltaStore,
) iter.Seq2[*models.ListUsersDeltaResponse, error] {
	return func(yield func(*models.ListUsersDeltaResponse, error) bool) {
		for msg, iterErr := range c.iterateDelta(ctx, endpoint, ds) {
			if iterErr != nil {
				yield(nil, trace.Wrap(iterErr))
				return
			}
			var page []*models.ListUsersDeltaResponse
			if err := utils.FastUnmarshal(msg, &page); err != nil {
				yield(nil, trace.Wrap(err))
				return
			}
			for _, item := range page {
				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

// IterateGroupDeltas iterates over groups delta response.
// A delta token for the group endpont
// must be set up before calling this method.
func (c *Client) IterateGroupDeltas(
	ctx context.Context,
	endpoint string,
	ds DeltaStore,
) iter.Seq2[*models.ListGroupsDeltaResponse, error] {
	return func(yield func(*models.ListGroupsDeltaResponse, error) bool) {
		for msg, iterErr := range c.iterateDelta(ctx, endpoint, ds) {
			if iterErr != nil {
				yield(nil, trace.Wrap(iterErr))
				return
			}
			var page []*models.ListGroupsDeltaResponse
			if err := utils.FastUnmarshal(msg, &page); err != nil {
				yield(nil, trace.Wrap(err))
				return
			}
			for _, item := range page {
				item.Owners = filterUnsupportedGroupOwners(item.Owners)
				item.Members = filterUnsupportedGroupMembers(item.Members)
				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

func filterUnsupportedGroupOwners(in []models.OwnersDelta) []models.OwnersDelta {
	if in == nil {
		return nil
	}
	out := make([]models.OwnersDelta, 0, len(in))
	for _, owner := range in {
		if owner.User == nil {
			continue
		}
		switch owner.Type {
		case models.ODataUser:
			out = append(out, models.OwnersDelta{
				User: &models.User{
					DirectoryObject: models.DirectoryObject{
						ID:          owner.ID,
						DisplayName: owner.DisplayName,
					},
				},
				Type:    owner.Type,
				Removed: owner.Removed,
			})
		default:
			// owners such as #microsoft.graph.servicePrincipal are discarded.
		}
	}
	return out
}

func filterUnsupportedGroupMembers(in []models.MembersDelta) []models.MembersDelta {
	if in == nil {
		return nil
	}
	out := make([]models.MembersDelta, 0, len(in))
	for _, member := range in {
		if member.DirectoryObject == nil {
			continue
		}
		switch member.Type {
		case models.ODataUser, models.ODataGroup:
			out = append(out, models.MembersDelta{
				DirectoryObject: &models.DirectoryObject{
					ID:          member.ID,
					DisplayName: member.DisplayName,
				},
				Type:    member.Type,
				Removed: member.Removed,
			})
		default:
			// members such as #microsoft.graph.device are discarded.
		}
	}
	return out
}

// SetupLatestDelta configures latest delta token for the given endpoint.
// Should always be called before iterating over user and group delta API.
func (c *Client) SetupLatestDelta(ctx context.Context, endpoint string, ds DeltaStore, opts ...IterateOpt) (err error) {
	if ds == nil {
		return trace.BadParameter("missing delta store")
	}

	// Wipe out existing cache but preserve older link on error.
	oldLink := ds.Get(endpoint)
	defer func() {
		if err != nil && oldLink != "" {
			ds.Set(endpoint, oldLink)
		}
	}()
	ds.Clear(endpoint)

	// Configure URL. At minimum, this needs $deltatoken=latest
	// and $select query passed by the caller.
	ic := c.newIterateConfigDelta()
	for _, opt := range opts {
		opt(ic)
	}
	q := ic.query()
	q.Set("$deltatoken", "latest")
	uri := *c.baseURL
	uri.Path = path.Join(uri.Path, endpoint)
	uri.RawQuery = q.Encode()
	uriString := uri.String()

	var resp *http.Response
	resp, err = c.request(ctx, http.MethodGet, uriString, ic.header, nil /* payload */)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	var page models.ODataPage
	if err = jsoniter.ConfigFastest.NewDecoder(resp.Body).Decode(&page); err != nil {
		return trace.Wrap(err)
	}
	if page.DeltaLink == "" {
		return trace.Errorf("missing delta link in latest delta query response")
	}

	ds.Set(endpoint, page.DeltaLink)

	return nil
}

// IterateServicePrincipals lists all service principals in the Entra ID directory using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/serviceprincipal-list].
func (c *Client) IterateServicePrincipals(ctx context.Context, f func(principal *models.ServicePrincipal) bool, opts ...IterateOpt) error {
	return iterateSimple(c, ctx, "servicePrincipals", f, opts...)
}

// IterateGroupMembers lists all members for the given Entra ID group using pagination.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/group-list-members].
func (c *Client) IterateGroupMembers(ctx context.Context, groupID string, f func(models.GroupMember) bool, opts ...IterateOpt) error {
	var err error
	itErr := c.iterate(ctx, path.Join("groups", groupID, "members"), func(msg json.RawMessage) bool {
		var page []json.RawMessage
		if err = utils.FastUnmarshal(msg, &page); err != nil {
			return false
		}
		for _, entry := range page {
			var member models.GroupMember
			member, err = models.DecodeGroupMember(entry)
			if err != nil {
				var gmErr *models.UnsupportedGroupMember
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
	}, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}

// IterateGroupOwners lists Microsoft Entra ID group owners.
// Group owners are of User object type and can be either User
// or Service Principals. Teleport only supports User as group owners.
// `f` will be called for each object in the result set.
// if `f` returns `false`, the iteration is stopped (equivalent to `break` in a normal loop).
// Ref: [https://learn.microsoft.com/en-us/graph/api/group-list-owners?view=graph-rest-1.0].
func (c *Client) IterateGroupOwners(ctx context.Context, groupID string, f func(*models.User) bool, opts ...IterateOpt) error {
	// Group owners of user type is requested by
	// using "microsoft.graph.user" OData cast.
	return iterateSimple(c, ctx, path.Join("groups", groupID, "owners", "microsoft.graph.user"), f, opts...)
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
func (c *Client) IterateUsersTransitiveMemberOf(ctx context.Context, userID, groupType string, f func(*models.Group) bool) error {
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
		var page []models.Group
		if err = utils.FastUnmarshal(msg, &page); err != nil {
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

// validateDeltaLink checks host of the baseURL and deltaLink matches.
func validateDeltaLink(baseURL *url.URL, deltaLink string) error {
	deltaURL, err := url.Parse(deltaLink)
	if err != nil {
		return trace.BadParameter("invalid delta link URL %s", deltaLink)
	}
	if deltaURL.Scheme != "https" {
		return trace.BadParameter("delta link must be of HTTPs scheme, received %q", deltaURL.Scheme)
	}
	if baseURL.Host != deltaURL.Host {
		return trace.BadParameter("base URL and delta link URL host mismatch, base=%q delta=%q", baseURL.Host, deltaURL.Host)
	}
	return nil
}
