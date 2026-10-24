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
	"iter"
	"os"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
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
	// mu is used to maintain consistency between the cache and the index.
	mu    sync.RWMutex
	cache *cache.Cache[*scopedaccessv1.ScopedRoleAssignment, string]
	index *assignmentIndex
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
		index: newAssignmentIndex(),
		cfg:   cfg,
	}
}

// GetScopedRoleAssignment gets an assignment by name.
func (c *AssignmentCache) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	if req.GetName() == "" {
		return nil, trace.BadParameter("missing scoped role assignment name in get request")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	assignment, ok := c.cache.Get(cache.ScopedKey[string]{
		Scope: req.GetScope(),
		Key: assignmentKey{
			name:    req.GetName(),
			subKind: req.GetSubKind(),
		}.String(),
	})
	if !ok {
		return nil, trace.NotFound("scoped role assignment %q not found in scope %q", req.GetName(), req.GetScope())
	}

	return scopedaccessv1.GetScopedRoleAssignmentResponse_builder{
		Assignment: assignment,
	}.Build(), nil
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

	c.mu.RLock()
	defer c.mu.RUnlock()

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

// ListSubAssignments returns a list of given user's assigned subassignments,
// sorted by the scope of effect. Only scopes that start from a certain prefix
// are returned. The `start` argument, if not empty, indicates that only scopes
// that are greater or equal to the start scope (in alphabetical order) are
// returned.
//
// This function is safe for concurrent use and a read lock is held on the
// cache for the entire time of iterating on the returned iterator.
func (c *AssignmentCache) ListSubAssignments(user, prefix, start string) iter.Seq[IndexedAssignment] {
	return func(yield func(IndexedAssignment) bool) {
		c.mu.RLock()
		defer c.mu.RUnlock()

		for assignment := range c.index.AssignmentsForUser(user, prefix, start) {
			if !yield(assignment) {
				return
			}
		}
	}
}

// Put adds a new assignment to the cache. It will overwrite any existing assignment with the same name.
func (c *AssignmentCache) Put(assignment *scopedaccessv1.ScopedRoleAssignment) error {
	if err := scopedaccess.WeakValidateAssignment(assignment); err != nil {
		return trace.Wrap(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If the entry has been edited, first remove it from the index prior to
	// putting the edited value to correctly account for the reference counter.
	key := cache.ScopedKey[string]{
		Scope: assignment.GetScope(),
		Key: assignmentKey{
			name:    assignment.GetMetadata().GetName(),
			subKind: assignment.GetSubKind(),
		}.String(),
	}
	existing, ok := c.cache.Get(key)
	if ok {
		c.index.Del(existing)
	}

	c.cache.Put(assignment)
	c.index.Put(assignment)
	return nil
}

// Delete removes an assignment from the cache by its scope-qualified name and sub-kind.
func (c *AssignmentCache) Delete(assignment apiscopes.QualifiedName, subKind string) {
	key := cache.ScopedKey[string]{
		Scope: assignment.Scope,
		Key: assignmentKey{
			name:    assignment.Name,
			subKind: subKind,
		}.String(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	a, ok := c.cache.Get(key)
	if ok {
		c.cache.Del(key)
		c.index.Del(a)
	}
}

// Len returns the total number of assignments in the cache.
func (c *AssignmentCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

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
