/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generic

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// MarshalFunc is a type signature for a marshaling function.
type MarshalFunc[T types.Resource] func(T, ...services.MarshalOption) ([]byte, error)

// UnmarshalFunc is a type signature for an unmarshaling function.
type UnmarshalFunc[T types.Resource] func([]byte, ...services.MarshalOption) (T, error)

// ServiceConfig is the configuration for the service configuration.
type ServiceConfig[T types.Resource] struct {
	Backend       backend.Backend
	ResourceKind  string
	PageLimit     uint
	BackendPrefix string
	MarshalFunc   MarshalFunc[T]
	UnmarshalFunc UnmarshalFunc[T]
}

func (c *ServiceConfig[T]) CheckAndSetDefaults() error {
	if c.Backend == nil {
		return trace.BadParameter("backend is missing")
	}
	if c.ResourceKind == "" {
		return trace.BadParameter("resource kind is missing")
	}
	// We should allow page limit to be 0 for services that don't use pagination. Some services are
	// intended to be internally facing only, and those services may not need to set this limit.
	if c.PageLimit == 0 {
		c.PageLimit = defaults.DefaultChunkSize
	}
	if c.BackendPrefix == "" {
		return trace.BadParameter("backend prefix is missing")
	}
	if c.MarshalFunc == nil {
		return trace.BadParameter("marshal func is missing")
	}
	if c.UnmarshalFunc == nil {
		return trace.BadParameter("unmarshal func is missing")
	}

	return nil
}

// Service is a generic service for interacting with resources in the backend.
type Service[T types.Resource] struct {
	backend       backend.Backend
	resourceKind  string
	pageLimit     uint
	backendPrefix string
	marshalFunc   MarshalFunc[T]
	unmarshalFunc UnmarshalFunc[T]
}

// NewService will return a new generic service with the given config. This will
// panic if the configuration is invalid.
func NewService[T types.Resource](cfg *ServiceConfig[T]) (*Service[T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service[T]{
		backend:       cfg.Backend,
		resourceKind:  cfg.ResourceKind,
		pageLimit:     cfg.PageLimit,
		backendPrefix: cfg.BackendPrefix,
		marshalFunc:   cfg.MarshalFunc,
		unmarshalFunc: cfg.UnmarshalFunc,
	}, nil
}

// GetResources returns a list of all resources.
func (s *Service[T]) GetResources(ctx context.Context) ([]T, error) {
	rangeStart := backend.Key(s.backendPrefix, "")
	rangeEnd := backend.RangeEnd(rangeStart)

	// no filter provided get the range directly
	result, err := s.backend.GetRange(ctx, rangeStart, rangeEnd, backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		resource, err := s.unmarshalFunc(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, resource)
	}

	return out, nil
}

// ListResources returns a paginated list of resources.
func (s *Service[T]) ListResources(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	rangeStart := backend.Key(s.backendPrefix, pageToken)
	rangeEnd := backend.RangeEnd(backend.Key(s.backendPrefix, ""))

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > int(s.pageLimit) {
		pageSize = int(s.pageLimit)
	}

	limit := pageSize + 1

	// no filter provided get the range directly
	result, err := s.backend.GetRange(ctx, rangeStart, rangeEnd, limit)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	out := make([]T, 0, len(result.Items))
	for _, item := range result.Items {
		resource, err := s.unmarshalFunc(item.Value)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		out = append(out, resource)
	}

	var nextKey string
	if len(out) > pageSize {
		nextKey = backend.GetPaginationKey(out[len(out)-1])
		// Truncate the last item that was used to determine next row existence.
		out = out[:pageSize]
	}

	return out, nextKey, nil
}

// GetResource returns the specified resource.
func (s *Service[T]) GetResource(ctx context.Context, name string) (resource T, err error) {
	item, err := s.backend.Get(ctx, s.MakeKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return resource, trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return resource, trace.Wrap(err)
	}
	resource, err = s.unmarshalFunc(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	return resource, trace.Wrap(err)
}

// CreateResource creates a new resource.
func (s *Service[T]) CreateResource(ctx context.Context, resource T) error {
	item, err := s.MakeBackendItem(resource, resource.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.Create(ctx, item)
	if trace.IsAlreadyExists(err) {
		return trace.AlreadyExists("%s %q already exists", s.resourceKind, resource.GetName())
	}

	return trace.Wrap(err)
}

// UpdateResource updates an existing resource.
func (s *Service[T]) UpdateResource(ctx context.Context, resource T) error {
	item, err := s.MakeBackendItem(resource, resource.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.Update(ctx, item)
	if trace.IsNotFound(err) {
		return trace.NotFound("%s %q doesn't exist", s.resourceKind, resource.GetName())
	}

	return trace.Wrap(err)
}

// Upsert upserts a resource.
func (s *Service[T]) UpsertResource(ctx context.Context, resource T) error {
	item, err := s.MakeBackendItem(resource, resource.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.Put(ctx, item)
	return trace.Wrap(err)
}

// DeleteResource removes the specified resource.
func (s *Service[T]) DeleteResource(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, s.MakeKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllResources removes all resources.
func (s *Service[T]) DeleteAllResources(ctx context.Context) error {
	startKey := backend.Key(s.backendPrefix, "")
	return trace.Wrap(s.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// UpdateAndSwapResource will get the resource from the backend, modify it, and swap the new value into the backend.
func (s *Service[T]) UpdateAndSwapResource(ctx context.Context, name string, modify func(T) error) error {
	existingItem, err := s.backend.Get(ctx, s.MakeKey(name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return trace.Wrap(err)
	}

	resource, err := s.unmarshalFunc(existingItem.Value,
		services.WithResourceID(existingItem.ID), services.WithExpires(existingItem.Expires))
	if err != nil {
		return trace.Wrap(err)
	}

	err = modify(resource)
	if err != nil {
		return trace.Wrap(err)
	}

	replacementItem, err := s.MakeBackendItem(resource, name)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.backend.CompareAndSwap(ctx, *existingItem, replacementItem)

	return trace.Wrap(err)
}

// MakeBackendItem will check and make the backend item.
func (s *Service[T]) MakeBackendItem(resource T, name string) (backend.Item, error) {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	value, err := s.marshalFunc(resource)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}
	item := backend.Item{
		Key:     s.MakeKey(name),
		Value:   value,
		Expires: resource.Expiry(),
		ID:      resource.GetResourceID(),
	}

	return item, nil
}

// MakeKey will make a key for the service given a name.
func (s *Service[T]) MakeKey(name string) []byte {
	return backend.Key(s.backendPrefix, name)
}

// RunWhileLocked will run the given function in a backend lock. This is a wrapper around the backend.RunWhileLocked function.
func (s *Service[T]) RunWhileLocked(ctx context.Context, lockName string, ttl time.Duration, fn func(context.Context, backend.Backend) error) error {
	return trace.Wrap(backend.RunWhileLocked(ctx,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:  s.backend,
				LockName: lockName,
				TTL:      ttl,
			},
		}, func(ctx context.Context) error {
			return fn(ctx, s.backend)
		}))
}
