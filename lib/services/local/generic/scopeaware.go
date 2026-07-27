// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package generic

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
)

// ScopedResource is a resource type that has a scope. The scope may be empty
// on any individual resource of the type, that resource may be referred to as
// "unscoped" despite the type being scoped.
type ScopedResource interface {
	Resource
	// GetScope returns the scope of the resource.
	GetScope() string
}

// ScopeAwareService is a generic service for interacting with namespaced
// scoped resources in the backend. Scoped resources will be stored in a
// separate key range from resources with an empty scope, and namespaced by
// their scope. The ScopeAwareService transparently handles listing all scoped
// and unscoped resources, as well as creating and querying individual resources
// from the correct key range. A ScopeAwareService can also be configured to only
// consume scoped resources. This is useful for situations in which a resource
// has never had an unscoped variant.
type ScopeAwareService[T ScopedResource] struct {
	// scopedOnly indicates that the service will only operate on scoped resources.
	// The unscoped fallback path will be ignored in all cases.
	scopedOnly bool
	// UnscopedService is the underlying service for resources with an empty scope.
	// Resources will be keyed at <backend_prefix>/<name>. It will be nil if the
	// service was configured to be scoped only.
	UnscopedService *Service[T]
	// ScopedService is the underlying service for resources with a scope.
	// Resources will be keyed at <scoped_prefix>/<backend_prefix>/<encoded_scope>/<name>
	ScopedService *Service[T]

	backend                     backend.Backend
	runWhileLockedRetryInterval time.Duration
}

// ScopeAwareServiceConfig holds configuration options for ScopeAwareService.
type ScopeAwareServiceConfig[T ScopedResource] struct {
	// ScopedOnly indicates that the service will only operate on scoped resources.
	// The unscoped fallback path will be ignored in all cases.
	ScopedOnly bool
	// Backend used to persist the resource.
	Backend backend.Backend
	// ResourceKind is the friendly name of the resource.
	ResourceKind string
	// UnscopedBackendPrefix used when constructing the [backend.Item.Key] for unscoped resources.
	UnscopedBackendPrefix backend.Key
	// ScopedBackendPrefix used when constructing the [backend.Item.Key] for scoped resources.
	ScopedBackendPrefix backend.Key
	// PageLimit
	PageLimit uint
	// MarshlFunc converts the resource to bytes for persistence.
	MarshalFunc MarshalFunc[T]
	// UnmarshalFunc converts the bytes read from the backend to the resource.
	UnmarshalFunc UnmarshalFunc[T]
	// ValidateFunc optionally validates the resource prior to persisting it. Any errors
	// returned from the validation function will prevent writes to the backend.
	ValidateFunc func(T) error
	// RunWhileLockedRetryInterval is the interval to retry the RunWhileLocked function.
	// If set to 0, the default interval of 250ms will be used.
	// WARNING: If set to a negative value, the RunWhileLocked function will retry immediately.
	RunWhileLockedRetryInterval time.Duration
}

// NewScopeAwareService returns a new scope-aware service.
func NewScopeAwareService[T ScopedResource](cfg *ScopeAwareServiceConfig[T]) (*ScopeAwareService[T], error) {
	if !cfg.ScopedOnly && cfg.ScopedBackendPrefix.Compare(cfg.UnscopedBackendPrefix) == 0 {
		return nil, trace.BadParameter("scoped and unscoped backend services cannot have the same prefix")
	}

	scopedService, err := NewService(&ServiceConfig[T]{
		Backend:                     cfg.Backend,
		ResourceKind:                cfg.ResourceKind,
		PageLimit:                   cfg.PageLimit,
		BackendPrefix:               cfg.ScopedBackendPrefix,
		MarshalFunc:                 cfg.MarshalFunc,
		UnmarshalFunc:               cfg.UnmarshalFunc,
		ValidateFunc:                cfg.ValidateFunc,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ScopedOnly {
		return &ScopeAwareService[T]{
			scopedOnly:    true,
			ScopedService: scopedService,
		}, nil
	}

	unscopedService, err := NewService(&ServiceConfig[T]{
		Backend:                     cfg.Backend,
		ResourceKind:                cfg.ResourceKind,
		PageLimit:                   cfg.PageLimit,
		BackendPrefix:               cfg.UnscopedBackendPrefix,
		MarshalFunc:                 cfg.MarshalFunc,
		UnmarshalFunc:               cfg.UnmarshalFunc,
		ValidateFunc:                cfg.ValidateFunc,
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ScopeAwareService[T]{
		UnscopedService:             unscopedService,
		ScopedService:               scopedService,
		backend:                     cfg.Backend,
		runWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	}, nil
}

// Resources returns a stream of resources within the unified scope-aware range
// [startKey, endKey). Unscoped resources are ordered before scoped resources.
//
// The startKey and endKey values are resource cursors as defined by
// [scopes.MakeResourceCursor]. An unscoped cursor is the resource name. A scoped
// cursor uses [scopes.ResourceCursorPrefix] followed by the scoped backend
// service's relative key. [scopes.ResourceCursorScopedStart] is the boundary
// between unscoped and scoped resources.
func (s *ScopeAwareService[T]) Resources(ctx context.Context, startKey, endKey string) iter.Seq2[T, error] {
	if s.scopedOnly {
		if endKey != "" && !scopes.IsScopedResourceCursor(endKey) {
			return stream.Empty[T]()
		}
		if endKey == scopes.ResourceCursorScopedStart() {
			return stream.Empty[T]()
		}

		scopedStartKey := ""
		if scopes.IsScopedResourceCursor(startKey) {
			scopedStartKey = strings.TrimPrefix(startKey, scopes.ResourceCursorPrefix)
		}
		scopedEndKey := strings.TrimPrefix(endKey, scopes.ResourceCursorPrefix)
		return s.ScopedService.Resources(ctx, scopedStartKey, scopedEndKey)
	}

	var streams []stream.Stream[T]

	if !scopes.IsScopedResourceCursor(startKey) {
		unscopedEndKey := endKey
		if scopes.IsScopedResourceCursor(endKey) {
			unscopedEndKey = ""
		}
		streams = append(streams, s.UnscopedService.Resources(ctx, startKey, unscopedEndKey))
	}

	if endKey != "" && !scopes.IsScopedResourceCursor(endKey) {
		return stream.Chain(streams...)
	}
	if endKey == scopes.ResourceCursorScopedStart() {
		return stream.Chain(streams...)
	}

	scopedStartKey := ""
	if scopes.IsScopedResourceCursor(startKey) {
		scopedStartKey = strings.TrimPrefix(startKey, scopes.ResourceCursorPrefix)
	}
	scopedEndKey := strings.TrimPrefix(endKey, scopes.ResourceCursorPrefix)
	streams = append(streams, s.ScopedService.Resources(ctx, scopedStartKey, scopedEndKey))

	return stream.Chain(streams...)
}

// GetResources returns all unscoped and scoped resources.
func (s *ScopeAwareService[T]) GetResources(ctx context.Context) ([]T, error) {
	return stream.Collect(s.Resources(ctx, "", ""))
}

// ListResources returns a page of resources over the unified scoped
// and unscoped collection. It always returns all unscoped resources before
// matching scoped resources.
func (s *ScopeAwareService[T]) ListResources(ctx context.Context, pageSize int, nextToken string) ([]T, string, error) {
	return s.ListResourcesWithFilter(ctx, pageSize, nextToken, func(T) bool { return true })
}

// ListResourcesWithFilter returns a page of matching resources over the
// unified scoped and unscoped collection. It always returns all matching
// unscoped resources before matching scoped resources.
func (s *ScopeAwareService[T]) ListResourcesWithFilter(ctx context.Context, pageSize int, nextToken string, matcher func(T) bool) ([]T, string, error) {
	if pageSize <= 0 || pageSize > int(s.ScopedService.pageLimit) {
		pageSize = int(s.ScopedService.pageLimit)
	}

	if s.scopedOnly && nextToken != "" && !scopes.IsScopedResourceCursor(nextToken) {
		return nil, "", trace.BadParameter("scoped-only storage service received invalid pagination token")
	}

	// If in scoped only mode we don't care about unscoped resources. If the
	// token was scoped the caller has already paged over all unscoped
	// resources and we should return the next page of scoped resources.
	if s.scopedOnly || scopes.IsScopedResourceCursor(nextToken) {
		nextToken = strings.TrimPrefix(nextToken, scopes.ResourceCursorPrefix)
		resources, nextToken, err := s.ScopedService.ListResourcesWithFilter(ctx, pageSize, nextToken, matcher)
		if nextToken != "" {
			nextToken = scopes.ResourceCursorPrefix + nextToken
		}
		return resources, nextToken, trace.Wrap(err)
	}

	// Fetch the next page of matching unscoped resources.
	resources, nextToken, err := s.UnscopedService.ListResourcesWithFilter(ctx, pageSize, nextToken, matcher)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if nextToken != "" {
		// There are remaining unscoped resources, return this page.
		return resources, nextToken, nil
	}
	if len(resources) >= pageSize {
		// The page is full but nextToken is empty indicating there are no more
		// unscoped resources. Return with scopedPageTokenPrefix so the next
		// page begins with scoped resources.
		return resources, scopes.ResourceCursorScopedStart(), nil
	}

	// Reached the end of unscoped resources within pageSize, try to fill in
	// the page with scoped resources.
	remainingPageSize := pageSize - len(resources)
	scopedResources, nextToken, err := s.ScopedService.ListResourcesWithFilter(ctx, remainingPageSize, "", matcher)
	if nextToken != "" {
		nextToken = scopes.ResourceCursorPrefix + nextToken
	}
	return append(resources, scopedResources...), nextToken, trace.Wrap(err)
}

// GetResource returns a resource, if it exists, for the given scope-qualified name.
// If the scope is empty, it returns an unscoped resource from the unscoped key range.
// If the scope is non-empty, it returns a scoped resource from the scoped key range.
func (s *ScopeAwareService[T]) GetResource(ctx context.Context, scopedName scopes.QualifiedName) (T, error) {
	svc, err := s.WithScopePrefix(scopedName.Scope)
	if err != nil {
		var nul T
		return nul, trace.Wrap(err)
	}
	return svc.GetResource(ctx, scopedName.Name)
}

// DeleteResource deletes a resource for the given scope-qualified name.
// If the scope is empty, it deletes an unscoped resource from the unscoped key range.
// If the scope is non-empty, it deletes a scoped resource from the scoped key range.
func (s *ScopeAwareService[T]) DeleteResource(ctx context.Context, scopedName scopes.QualifiedName) error {
	svc, err := s.WithScopePrefix(scopedName.Scope)
	if err != nil {
		return trace.Wrap(err)
	}
	return svc.DeleteResource(ctx, scopedName.Name)
}

// DeleteAllResources deletes all scoped and unscoped resources (if not in scoped only mode).
func (s *ScopeAwareService[T]) DeleteAllResources(ctx context.Context) error {
	if s.scopedOnly {
		return trace.Wrap(s.ScopedService.DeleteAllResources(ctx))
	}

	return trace.NewAggregate(
		s.UnscopedService.DeleteAllResources(ctx),
		s.ScopedService.DeleteAllResources(ctx),
	)
}

// CreateResource creates the given scoped resource if it doesn't already
// exist. If the scope is empty, it will be inserted in the unscoped key range,
// else it will be inserted in the scoped key range.
func (s *ScopeAwareService[T]) CreateResource(ctx context.Context, resource T) (T, error) {
	svc, err := s.WithScopePrefix(resource.GetScope())
	if err != nil {
		var nul T
		return nul, trace.Wrap(err)
	}
	return svc.CreateResource(ctx, resource)
}

// UpsertResource upserts the given scoped resource. If the scope is empty, it
// will be inserted in the unscoped key range, else it will be inserted in the
// scoped key range.
func (s *ScopeAwareService[T]) UpsertResource(ctx context.Context, resource T) (T, error) {
	svc, err := s.WithScopePrefix(resource.GetScope())
	if err != nil {
		var nul T
		return nul, trace.Wrap(err)
	}
	return svc.UpsertResource(ctx, resource)
}

// UpdateResource updates the given scoped resource. If the scope is empty, it
// will be updated in the unscoped key range, else it will be updated in the
// scoped key range.
func (s *ScopeAwareService[T]) UpdateResource(ctx context.Context, resource T) (T, error) {
	svc, err := s.WithScopePrefix(resource.GetScope())
	if err != nil {
		var nul T
		return nul, trace.Wrap(err)
	}
	return svc.UpdateResource(ctx, resource)
}

// ConditionalUpdateResource updates the given scoped resource if the revision
// matches. If the scope is empty, it will be updated in the unscoped key
// range, else it will be updated in the scoped key range.
func (s *ScopeAwareService[T]) ConditionalUpdateResource(ctx context.Context, resource T) (T, error) {
	svc, err := s.WithScopePrefix(resource.GetScope())
	if err != nil {
		var nul T
		return nul, trace.Wrap(err)
	}
	return svc.ConditionalUpdateResource(ctx, resource)
}

// MakeBackendItem will check and make the backend item.
func (s *ScopeAwareService[T]) MakeBackendItem(resource T) (backend.Item, error) {
	svc, err := s.WithScopePrefix(resource.GetScope())
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	return svc.MakeBackendItem(resource)
}

// WithScopePrefix returns the unscoped service when scope is empty, otherwise
// returns the scoped service with the encoded scope appended to its backend prefix.
func (s *ScopeAwareService[T]) WithScopePrefix(scope string) (*Service[T], error) {
	if scope == "" {
		if s.scopedOnly {
			return nil, trace.BadParameter("scoped-only storage service received an empty scope")
		}
		return s.UnscopedService, nil
	}
	encodedScope, err := scopes.EncodeForKey(scope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.ScopedService.WithPrefix(encodedScope), nil
}

// WithScopedResourcePrefix returns a [*Service] with a prefix for the given
// scope-qualified name appended to the backend prefix.
//
// If the given scope is empty, it will return the UnscopedService with the
// given name as an added prefix.
//
// If the given scope is non-empty, it will return the ScopedService with the
// encoded scope and the name as an added prefix.
//
// This may be appropriate for dependent resources keyed by a unique scoped
// resource, i.e. members of a scoped access list.
func (s *ScopeAwareService[T]) WithScopedResourcePrefix(scopedName scopes.QualifiedName) (*Service[T], error) {
	if scopedName.Scope == "" {
		if s.scopedOnly {
			return nil, trace.BadParameter("scoped-only storage service received an empty scope")
		}
		return s.UnscopedService.WithPrefix(scopedName.Name), nil
	}
	encodedScope, err := scopes.EncodeForKey(scopedName.Scope)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s.ScopedService.WithPrefix(encodedScope, scopedName.Name), nil
}

// RunWhileLocked will run the given function in a backend lock. This is a wrapper around the backend.RunWhileLocked function.
func (s *ScopeAwareService[T]) RunWhileLocked(ctx context.Context, lockNameComponents []string, ttl time.Duration, fn func(context.Context, backend.Backend) error) error {
	return trace.Wrap(backend.RunWhileLocked(ctx,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            s.backend,
				LockNameComponents: lockNameComponents,
				TTL:                ttl,
				RetryInterval:      s.runWhileLockedRetryInterval,
			},
		}, func(ctx context.Context) error {
			return fn(ctx, s.backend)
		}))
}
