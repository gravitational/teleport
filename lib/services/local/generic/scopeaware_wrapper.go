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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// ScopedResourceMetadata is an RFD 153-style resource that additionally carries
// a scope. The scope may be empty on any individual resource, in which case the
// resource is unscoped (classic behavior).
type ScopedResourceMetadata interface {
	types.ResourceMetadata
	// GetScope returns the scope of the resource. An empty scope indicates the
	// resource is unscoped.
	GetScope() string
}

// NewScopeAwareServiceWrapper returns a new scope-aware generic service wrapper.
// It is the RFD 153 analog of ScopeAwareService: scoped resources are stored
// in a separate, scope-namespaced key range from unscoped resources, and the
// wrapper transparently routes reads and writes to the correct range.
func NewScopeAwareServiceWrapper[T ScopedResourceMetadata](cfg ScopeAwareServiceWrapperConfig[T]) (*ScopeAwareServiceWrapper[T], error) {
	serviceConfig := &ScopeAwareServiceConfig[scopedResourceMetadataAdapter[T]]{
		ScopedOnly:            cfg.ScopedOnly,
		Backend:               cfg.Backend,
		ResourceKind:          cfg.ResourceKind,
		PageLimit:             cfg.PageLimit,
		UnscopedBackendPrefix: cfg.UnscopedBackendPrefix,
		ScopedBackendPrefix:   cfg.ScopedBackendPrefix,
		MarshalFunc: func(w scopedResourceMetadataAdapter[T], opts ...services.MarshalOption) ([]byte, error) {
			return cfg.MarshalFunc(w.resource, opts...)
		},
		UnmarshalFunc: func(bytes []byte, opts ...services.MarshalOption) (scopedResourceMetadataAdapter[T], error) {
			r, err := cfg.UnmarshalFunc(bytes, opts...)
			return newScopedResourceMetadataAdapter(r), trace.Wrap(err)
		},
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
	}

	if cfg.ValidateFunc != nil {
		serviceConfig.ValidateFunc = func(w scopedResourceMetadataAdapter[T]) error {
			return cfg.ValidateFunc(w.resource)
		}
	}

	service, err := NewScopeAwareService(serviceConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ScopeAwareServiceWrapper[T]{service: service}, nil
}

// ScopeAwareServiceWrapperConfig holds configuration options for a
// ScopeAwareServiceWrapper. It mirrors ScopeAwareServiceConfig but operates on
// the bare RFD 153 resource type T rather than an adapter.
type ScopeAwareServiceWrapperConfig[T ScopedResourceMetadata] struct {
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
	// PageLimit is the maximum number of resources returned in a single page.
	PageLimit uint
	// MarshalFunc converts the resource to bytes for persistence.
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

// ScopeAwareServiceWrapper is an adapter for ScopeAwareService that makes it
// usable with RFD 153-style resources which implement ScopedResourceMetadata.
//
// As with ServiceWrapper, not all methods of the underlying service are
// exported; additional methods may be exported in the future as needed.
type ScopeAwareServiceWrapper[T ScopedResourceMetadata] struct {
	service *ScopeAwareService[scopedResourceMetadataAdapter[T]]
}

// CreateResource creates the given resource if it doesn't already exist. The
// resource is stored in the unscoped key range if its scope is empty, else in
// the scope-namespaced key range.
func (s *ScopeAwareServiceWrapper[T]) CreateResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.service.CreateResource(ctx, newScopedResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

// UpsertResource upserts the given resource into the key range determined by its scope.
func (s *ScopeAwareServiceWrapper[T]) UpsertResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.service.UpsertResource(ctx, newScopedResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

// ConditionalUpdateResource updates the given resource if the provided and
// existing revisions match, in the key range determined by its scope.
func (s *ScopeAwareServiceWrapper[T]) ConditionalUpdateResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.service.ConditionalUpdateResource(ctx, newScopedResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

// GetResource returns the resource for the given scope-qualified name. An empty
// scope reads from the unscoped key range.
func (s *ScopeAwareServiceWrapper[T]) GetResource(ctx context.Context, name scopes.QualifiedName) (T, error) {
	adapter, err := s.service.GetResource(ctx, name)
	return adapter.resource, trace.Wrap(err)
}

// DeleteResource deletes the resource for the given scope-qualified name. An
// empty scope deletes from the unscoped key range.
func (s *ScopeAwareServiceWrapper[T]) DeleteResource(ctx context.Context, name scopes.QualifiedName) error {
	return trace.Wrap(s.service.DeleteResource(ctx, name))
}

// DeleteAllResources deletes all scoped and unscoped resources.
func (s *ScopeAwareServiceWrapper[T]) DeleteAllResources(ctx context.Context) error {
	return trace.Wrap(s.service.DeleteAllResources(ctx))
}

// ListResources returns a page of resources over the unified scoped and
// unscoped collection. All unscoped resources are returned before scoped ones.
func (s *ScopeAwareServiceWrapper[T]) ListResources(ctx context.Context, pageSize int, nextToken string) ([]T, string, error) {
	return s.ListResourcesWithFilter(ctx, pageSize, nextToken, func(T) bool { return true })
}

// ListResourcesWithFilter returns a page of matching resources over the unified
// scoped and unscoped collection. All matching unscoped resources are returned
// before matching scoped ones.
func (s *ScopeAwareServiceWrapper[T]) ListResourcesWithFilter(ctx context.Context, pageSize int, nextToken string, matcher func(T) bool) ([]T, string, error) {
	adapters, next, err := s.service.ListResourcesWithFilter(ctx, pageSize, nextToken, func(w scopedResourceMetadataAdapter[T]) bool {
		return matcher(w.resource)
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	out := make([]T, 0, len(adapters))
	for _, adapter := range adapters {
		out = append(out, adapter.resource)
	}
	return out, next, nil
}

// Resources returns a stream of resources within the unified scope-aware range
// [startKey, endKey). Unscoped resources are ordered before scoped resources.
// The startKey and endKey values are resource cursors as defined by
// [scopes.MakeResourceCursor].
//
// This method may be used to implement RangeFoo.
func (s *ScopeAwareServiceWrapper[T]) Resources(ctx context.Context, startKey, endKey string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for adapter, err := range s.service.Resources(ctx, startKey, endKey) {
			if err != nil {
				var t T
				yield(t, err)
				return
			}
			if !yield(adapter.resource, nil) {
				return
			}
		}
	}
}

// MakeBackendItem returns a backend.Item for the given resource, keyed within
// the unscoped or scope-namespaced range according to the resource's scope.
func (s *ScopeAwareServiceWrapper[T]) MakeBackendItem(resource T) (backend.Item, error) {
	svc, err := s.service.WithScopePrefix(resource.GetScope())
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	return svc.MakeBackendItem(newScopedResourceMetadataAdapter(resource))
}

// BackendKey returns the backend.Key for the resource with the given
// scope-qualified name, within the unscoped or scope-namespaced range according
// to its scope.
func (s *ScopeAwareServiceWrapper[T]) BackendKey(name scopes.QualifiedName) (backend.Key, error) {
	svc, err := s.service.WithScopePrefix(name.Scope)
	if err != nil {
		return backend.Key{}, trace.Wrap(err)
	}
	return svc.resourceKey(name.Name), nil
}
