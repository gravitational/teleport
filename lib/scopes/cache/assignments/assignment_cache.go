/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package assignments

import (
	"context"
	"os"
	"strconv"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache"
)

const (
	defaultPageSize               = 256
	maxPageSize                   = 1024
	defaultMaxAssignmentTreeBytes = 2048
)

// AssignmentCacheConfig is the configuration for the assignment cache.
type AssignmentCacheConfig struct {
	// MaxAssignmentTreeBytes is the maximum encoded size for assignment trees, which make up
	// the bulk of the size of a scope pin when encoded on a certificate. See [pinning.PruneAssignmentTree]
	// for details on how pruning of oversized assignment trees works. Defaults to 2kb.
	MaxAssignmentTreeBytes int
}

// AssignmentCache is a cache for scoped role assignments.
type AssignmentCache struct {
	cache *cache.Cache[*scopedaccessv1.ScopedRoleAssignment, string]
	cfg   AssignmentCacheConfig
}

// NewAssignmentCache creates a new assignment cache instance with the given configuration.
func NewAssignmentCache(cfg AssignmentCacheConfig) *AssignmentCache {
	if cfg.MaxAssignmentTreeBytes == 0 {
		cfg.MaxAssignmentTreeBytes = defaultMaxAssignmentTreeBytes
	}

	if matb := os.Getenv("TELEPORT_UNSTABLE_MAX_ASSIGNMENT_TREE_BYTES"); matb != "" {
		parsed, err := strconv.Atoi(matb)
		if err == nil && parsed > 0 {
			cfg.MaxAssignmentTreeBytes = parsed
		}
	}

	return &AssignmentCache{
		cache: cache.Must(cache.Config[*scopedaccessv1.ScopedRoleAssignment, string]{
			Scope: func(assignment *scopedaccessv1.ScopedRoleAssignment) string {
				return assignment.GetScope()
			},
			Key: func(assignment *scopedaccessv1.ScopedRoleAssignment) string {
				return assignmentKey{
					name:    assignment.GetMetadata().GetName(),
					subKind: assignment.GetSubKind(),
				}.String()
			},
			Clone: proto.CloneOf[*scopedaccessv1.ScopedRoleAssignment],
		}),
		cfg: cfg,
	}
}

// GetScopedRoleAssignment gets an assignment by name.
func (c *AssignmentCache) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in get request")
	}

	assignment, ok := c.cache.Get(assignmentKey{
		name:    req.GetName(),
		subKind: req.GetSubKind(),
	}.String())
	if !ok {
		return nil, trace.NotFound("scoped role assignment %q not found", req.GetName())
	}

	return &scopedaccessv1.GetScopedRoleAssignmentResponse{
		Assignment: assignment,
	}, nil
}

// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
func (c *AssignmentCache) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	return c.ListScopedRoleAssignmentsWithFilter(ctx, req, func(*scopedaccessv1.ScopedRoleAssignment) bool { return true })
}

// ListScopedRoleAssignmentsWithFilter returns a paginated list of scoped role assignments filtered by the provided filter
// function. This method is used internally to implement access-controls on the ListScopedRoleAssignments grpc method.
func (c *AssignmentCache) ListScopedRoleAssignmentsWithFilter(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest, externalFilter func(*scopedaccessv1.ScopedRoleAssignment) bool) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	// validate the secondary assigned-scope filter. the primary scope filter is validated by the
	// Cache.ResourcesMatchingScopeFilter method invoked below.
	if err := scopes.ValidateFilter(req.GetAssignedScopeFilter()); err != nil {
		return nil, trace.Wrap(err)
	}

	filter := func(assignment *scopedaccessv1.ScopedRoleAssignment) bool {
		// primary scope filter matching is handled by the Cache.ResourcesMatchingScopeFilter method
		// invoked below. secondary filters (e.g. username) are applied here *after* the primary
		// has already been applied.
		if !scopedaccess.MatchSecondaryAssignmentFilters(req, assignment) {
			return false
		}

		// apply the external filter after the secondary filters as the externally provided filter
		// is often more expensive to evaluate.
		if !externalFilter(assignment) {
			return false
		}

		return true
	}

	cursor, err := cache.DecodeStringCursor(req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// the primary scope filter selects the cache traversal strategy.
	resources, err := c.cache.ResourcesMatchingScopeFilter(req.GetScopeFilter(), c.cache.WithFilter(filter), c.cache.WithCursor(cursor))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*scopedaccessv1.ScopedRoleAssignment
	var nextCursor cache.Cursor[string]
Outer:
	for scope := range resources {
		for assignment := range scope.Items() {
			if len(out) == pageSize {
				nextCursor = cache.Cursor[string]{
					Scope: scope.Scope(),
					Key: assignmentKey{
						name:    assignment.GetMetadata().GetName(),
						subKind: assignment.GetSubKind(),
					}.String(),
				}
				break Outer
			}
			out = append(out, assignment)
		}
	}

	var nextPageToken string
	if !nextCursor.IsZero() {
		nextPageToken, err = cache.EncodeStringCursor(nextCursor)
		if err != nil {
			return nil, trace.Errorf("failed to encode cursor %+v: %w (this is a bug)", nextCursor, err)
		}
	}

	return &scopedaccessv1.ListScopedRoleAssignmentsResponse{
		Assignments:   out,
		NextPageToken: nextPageToken,
	}, nil
}

// Put adds a new assignment to the cache. It will overwrite any existing assignment with the same name.
func (c *AssignmentCache) Put(assignment *scopedaccessv1.ScopedRoleAssignment) error {
	if err := scopedaccess.WeakValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	c.cache.Put(assignment)
	return nil
}

// Delete removes an assignment from the cache by name.
func (c *AssignmentCache) Delete(name, subKind string) {
	c.cache.Del(assignmentKey{
		name:    name,
		subKind: subKind,
	}.String())
}

// Len returns the total number of assignments in the cache.
func (c *AssignmentCache) Len() int {
	return c.cache.Len()
}

type assignmentKey struct {
	name    string
	subKind string
}

func (k assignmentKey) String() string {
	if k.subKind == "" {
		return k.name
	}
	return k.name + "/" + k.subKind
}
