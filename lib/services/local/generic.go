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

package local

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// marshalFunc is a type signature for a marshaling function.
type marshalFunc[T types.Resource] func(T, ...services.MarshalOption) ([]byte, error)

// UnmarshalFunc is a type signature for an unmarshaling function.
type unmarshalFunc[T types.Resource] func([]byte, ...services.MarshalOption) (T, error)

// genericResourceService is a generic service for interacting with resources in the backend.
type genericResourceService[T types.Resource] struct {
	backend       backend.Backend
	resourceKind  string
	limit         int
	backendPrefix string
	marshalFunc   marshalFunc[T]
	unmarshalFunc unmarshalFunc[T]

	// modificationPostCheckValidator is an additional check that runs after running CheckAndSetDefaults on an object.
	modificationPostCheckValidator func(resource T, name string) error
	// modificationLockName is the name of a backend lock to be used during modification. If this is empty, no lock will be used.
	modificationLockName string
	// modifacationLockTTL is the TTL of the backend lock acquired during modification. Will not be used if no lock name is set.
	modificationLockTTL time.Duration
	// preModifyValidator is a function that will run prior to actually modifying the object. If a modification lock is to be used,
	// this will run inside the lock.
	preModifyValidator func(ctx context.Context, resource T, name string) error
}

// ListResources returns a paginated list of resources.
func (s *genericResourceService[T]) listResources(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
	rangeStart := backend.Key(s.backendPrefix, pageToken)
	rangeEnd := backend.RangeEnd(rangeStart)

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > s.limit {
		pageSize = s.limit
	}

	// If backend.NoLimit is requested and the service permits it, set the limit to NoLimit.
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

// getResource returns the specified resource.
func (s *genericResourceService[T]) getResource(ctx context.Context, name string) (resource T, err error) {
	item, err := s.backend.Get(ctx, backend.Key(s.backendPrefix, name))
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

// createResource creates a new resource.
func (s *genericResourceService[T]) createResource(ctx context.Context, resource T, name string) error {
	return s.modifyResource(ctx, resource, name, func(ctx context.Context, item backend.Item) error {
		_, err := s.backend.Create(ctx, item)
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("%s %q already exists", s.resourceKind, name)
		}
		return trace.Wrap(err)
	})
}

// updateResource updates an existing resource.
func (s *genericResourceService[T]) updateResource(ctx context.Context, resource T, name string) error {
	return s.modifyResource(ctx, resource, name, func(ctx context.Context, item backend.Item) error {
		_, err := s.backend.Update(ctx, item)
		if trace.IsNotFound(err) {
			return trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return trace.Wrap(err)
	})
}

func (s *genericResourceService[T]) modifyResource(ctx context.Context, resource T, name string, backendModificationFunc func(context.Context, backend.Item) error) error {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.modificationPostCheckValidator != nil {
		if err := s.modificationPostCheckValidator(resource, name); err != nil {
			return trace.Wrap(err)
		}
	}
	value, err := s.marshalFunc(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(s.backendPrefix, name),
		Value:   value,
		Expires: resource.Expiry(),
		ID:      resource.GetResourceID(),
	}

	modification := func(ctx context.Context) error {
		if s.preModifyValidator != nil {
			if err := s.preModifyValidator(ctx, resource, name); err != nil {
				return trace.Wrap(err)
			}
		}

		return trace.Wrap(backendModificationFunc(ctx, item))
	}

	if s.modificationLockName != "" {
		return trace.Wrap(backend.RunWhileLocked(ctx, s.backend, s.modificationLockName, s.modificationLockTTL, modification))
	}

	return trace.Wrap(modification(ctx))
}

// deleteResource removes the specified resource.
func (s *genericResourceService[T]) deleteResource(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, backend.Key(s.backendPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllResources removes all resources.
func (s *genericResourceService[T]) deleteAllResources(ctx context.Context) error {
	startKey := backend.Key(s.backendPrefix)
	return trace.Wrap(s.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}
