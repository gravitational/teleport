/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package generic

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// NewService153 will return a new generic service for a given RFD 153-style resource.
func NewService153[T types.ResourceMetadata](
	backend backend.Backend,
	resourceKind string,
	backendPrefix string,
	marshalFunc MarshalFunc[T],
	unmarshalFunc UnmarshalFunc[T]) (ServiceCommon[T], error) {

	cfg := &ServiceConfig[resourceMetadataAdapter[T]]{
		Backend:       backend,
		ResourceKind:  resourceKind,
		PageLimit:     0, // use default page limit
		BackendPrefix: backendPrefix,
		MarshalFunc: func(w resourceMetadataAdapter[T], option ...services.MarshalOption) ([]byte, error) {
			return marshalFunc(w.resource)
		},
		UnmarshalFunc: func(bytes []byte, option ...services.MarshalOption) (resourceMetadataAdapter[T], error) {
			r, err := unmarshalFunc(bytes, option...)
			return newResourceMetadataAdapter(r), trace.Wrap(err)
		},
	}
	svc, err := NewService[resourceMetadataAdapter[T]](cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &service153[T]{svc: svc}, nil
}

var _ ServiceCommon[*machineidv1.Bot] = (*service153[*machineidv1.Bot])(nil)

// service153 is a service adapter for RFD 153-style resources.
type service153[T types.ResourceMetadata] struct {
	svc *Service[resourceMetadataAdapter[T]]
}

func (s service153[T]) WithPrefix(parts ...string) ServiceCommon[T] {
	svcNew := s.svc.withPrefix(parts...)
	return &service153[T]{svc: svcNew}
}

func (s service153[T]) GetResources(ctx context.Context) ([]T, error) {
	adapters, err := s.svc.GetResources(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []T
	for _, adapter := range adapters {
		out = append(out, adapter.resource)
	}
	return out, nil
}

func (s service153[T]) ListResources(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	adapters, nextToken, err := s.svc.ListResources(ctx, pageSize, pageToken)
	var out []T
	for _, adapter := range adapters {
		out = append(out, adapter.resource)
	}
	return out, nextToken, trace.Wrap(err)
}

func (s service153[T]) GetResource(ctx context.Context, name string) (resource T, err error) {
	adapter, err := s.svc.GetResource(ctx, name)
	return adapter.resource, trace.Wrap(err)
}

func (s service153[T]) CreateResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.svc.CreateResource(ctx, newResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

func (s service153[T]) UpdateResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.svc.UpdateResource(ctx, newResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

func (s service153[T]) UpsertResource(ctx context.Context, resource T) (T, error) {
	adapter, err := s.svc.UpsertResource(ctx, newResourceMetadataAdapter(resource))
	return adapter.resource, trace.Wrap(err)
}

func (s service153[T]) DeleteResource(ctx context.Context, name string) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, name))
}

func (s service153[T]) DeleteAllResources(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteAllResources(ctx))
}

func (s service153[T]) UpdateAndSwapResource(ctx context.Context, name string, modify func(T) error) (T, error) {
	adapter, err := s.svc.UpdateAndSwapResource(ctx, name, func(r resourceMetadataAdapter[T]) error {
		return trace.Wrap(modify(r.resource))
	})
	return adapter.resource, trace.Wrap(err)
}

func (s service153[T]) MakeBackendItem(resource T, name string) (backend.Item, error) {
	return s.svc.MakeBackendItem(newResourceMetadataAdapter(resource), name)
}

func (s service153[T]) RunWhileLocked(ctx context.Context, lockName string, ttl time.Duration, fn func(context.Context, backend.Backend) error) error {
	return trace.Wrap(s.svc.RunWhileLocked(ctx, lockName, ttl, fn))
}
