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

package generic

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// NewServiceWrapper will return a new generic service wrapper. It is compatible with resources aligned with RFD 153.
func NewServiceWrapper[T types.ResourceMetadata](cfg ServiceConfig[T]) (*ServiceWrapper[T], error) {
	serviceConfig := &ServiceConfig[resourceMetadataAdapter[T]]{
		Backend:       cfg.Backend,
		ResourceKind:  cfg.ResourceKind,
		PageLimit:     cfg.PageLimit,
		BackendPrefix: cfg.BackendPrefix,
		MarshalFunc: func(w resourceMetadataAdapter[T], option ...services.MarshalOption) ([]byte, error) {
			return cfg.MarshalFunc(w.resource, option...)
		},
		UnmarshalFunc: func(bytes []byte, option ...services.MarshalOption) (resourceMetadataAdapter[T], error) {
			r, err := cfg.UnmarshalFunc(bytes, option...)
			return newResourceMetadataAdapter(r), trace.Wrap(err)
		},
		RunWhileLockedRetryInterval: cfg.RunWhileLockedRetryInterval,
		NameKeyFunc:                 cfg.NameKeyFunc,
	}

	if cfg.ValidateFunc != nil {
		serviceConfig.ValidateFunc = func(rma resourceMetadataAdapter[T]) error {
			return cfg.ValidateFunc(rma.resource)
		}
	}

	if cfg.ResourceKeyFunc != nil {
		serviceConfig.ResourceKeyFunc = func(rma resourceMetadataAdapter[T]) string {
			return cfg.ResourceKeyFunc(rma.resource)
		}
	}

	service, err := NewService(serviceConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServiceWrapper[T]{service: service}, nil
}

// ServiceWrapper is an adapter for Service that makes it usable with RFD 153-style resources,
// which implement types.ResourceMetadata.
//
// Not all methods from Service are exported, in the effort to reduce the API complexity
// as well as adhere to the guidance from RFD 153, but additional methods may be exported in the future as needed.
type ServiceWrapper[T types.ResourceMetadata] struct {
	service *Service[resourceMetadataAdapter[T]]
}

// WithPrefix will return a service wrapper with the given parts appended to the backend prefix.
func (s *ServiceWrapper[T]) WithPrefix(parts ...string) *ServiceWrapper[T] {
	if len(parts) == 0 {
		return s
	}

	return &ServiceWrapper[T]{service: s.service.WithPrefix(parts...)}
}

// UpsertResource upserts a resource.
func (s ServiceWrapper[T]) UpsertResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.service.UpsertResource(ctx, newResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

// UnconditionalUpdateResource updates an existing resource without checking the provided resource revision.
// Because UnconditionalUpdateResource can blindly overwrite an existing item, ConditionalUpdateResource should
// be preferred.
// See https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md#update-1 for more details
// about the Update operation.
func (s ServiceWrapper[T]) UnconditionalUpdateResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.service.UpdateResource(ctx, newResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

// ConditionalUpdateResource updates an existing resource if the provided
// resource and the existing resource have matching revisions.
// See https://github.com/gravitational/teleport/blob/master/rfd/0126-backend-migrations.md#optimistic-locking for more
// details about the conditional update.
func (s ServiceWrapper[T]) ConditionalUpdateResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.service.ConditionalUpdateResource(ctx, newResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

// CreateResource creates a new resource.
func (s ServiceWrapper[T]) CreateResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.service.CreateResource(ctx, newResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

// GetResource returns the specified resource.
func (s ServiceWrapper[T]) GetResource(ctx context.Context, name string) (resource T, err error) {
	adapter, err := s.service.GetResource(ctx, name)
	return adapter.resource, trace.Wrap(err)
}

// DeleteResource removes the specified resource.
func (s ServiceWrapper[T]) DeleteResource(ctx context.Context, name string) error {
	return trace.Wrap(s.service.DeleteResource(ctx, name))
}

// DeleteAllResources removes all resources.
func (s ServiceWrapper[T]) DeleteAllResources(ctx context.Context) error {
	startKey := s.service.backendPrefix.ExactKey()
	return trace.Wrap(s.service.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// ListResources returns a paginated list of resources.
func (s ServiceWrapper[T]) ListResources(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	adapters, nextToken, err := s.service.ListResources(ctx, pageSize, pageToken)
	out := make([]T, 0, len(adapters))
	for _, adapter := range adapters {
		out = append(out, adapter.resource)
	}
	return out, nextToken, trace.Wrap(err)
}

// ListResourcesWithFilter returns a paginated list of resources that match the provided filter.
func (s ServiceWrapper[T]) ListResourcesWithFilter(ctx context.Context, pageSize int, pageToken string, matcher func(T) bool) ([]T, string, error) {
	adapters, nextToken, err := s.service.ListResourcesWithFilter(
		ctx,
		pageSize,
		pageToken,
		func(rma resourceMetadataAdapter[T]) bool {
			return matcher(rma.resource)
		})

	out := make([]T, 0, len(adapters))
	for _, adapter := range adapters {
		out = append(out, adapter.resource)
	}
	return out, nextToken, trace.Wrap(err)
}
