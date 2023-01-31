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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// genericResourceService is a generic service for interacting with resources in the backend.
type genericResourceService[T types.Resource] struct {
	backend.Backend

	resourceHumanReadableName string
	limit                     int
	backendPrefix             string
	marshalFunc               services.MarshalFunc[T]
	unmarshalFunc             services.UnmarshalFunc[T]
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
	var limit int
	if s.limit == backend.NoLimit && pageSize == backend.NoLimit {
		limit = backend.NoLimit
	} else {
		// Increment pageSize to allow for the extra item represented by nextKey.
		// We skip this item in the results below.
		limit = pageSize + 1
	}

	// no filter provided get the range directly
	result, err := s.GetRange(ctx, rangeStart, rangeEnd, limit)
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

	// Skip the pagination logic if backend.NoLimit has been requested.
	var nextKey string
	if limit != backend.NoLimit && len(out) > pageSize {
		nextKey = backend.GetPaginationKey(out[len(out)-1])
		// Truncate the last item that was used to determine next row existence.
		out = out[:pageSize]
	}

	return out, nextKey, nil
}

// getResource returns the specified resource.
func (s *genericResourceService[T]) getResource(ctx context.Context, name string, extraKeyParts ...string) (T, error) {
	key := s.fullKey(name, extraKeyParts...)
	item, err := s.Get(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return *new(T), trace.NotFound("%s %q doesn't exist", s.resourceHumanReadableName, string(key))
		}
		return *new(T), trace.Wrap(err)
	}
	resource, err := s.unmarshalFunc(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
	return resource, trace.Wrap(err)
}

// createResource creates a new resource.
func (s *genericResourceService[T]) createResource(ctx context.Context, resource T, name string, extraKeyParts ...string) error {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := s.marshalFunc(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     s.fullKey(name, extraKeyParts...),
		Value:   value,
		Expires: resource.Expiry(),
		ID:      resource.GetResourceID(),
	}
	_, err = s.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// updateResource updates an existing resource.
func (s *genericResourceService[T]) updateResource(ctx context.Context, resource T, name string, extraKeyParts ...string) error {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	value, err := s.marshalFunc(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     s.fullKey(name, extraKeyParts...),
		Value:   value,
		Expires: resource.Expiry(),
		ID:      resource.GetResourceID(),
	}
	_, err = s.Update(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// deleteResource removes the specified resource.
func (s *genericResourceService[T]) deleteResource(ctx context.Context, name string, extraKeyParts ...string) error {
	key := s.fullKey(name, extraKeyParts...)
	err := s.Delete(ctx, key)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("%s %q doesn't exist", s.resourceHumanReadableName, string(key))
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllResources removes all resources.
func (s *genericResourceService[T]) deleteAllResources(ctx context.Context) error {
	startKey := backend.Key(s.backendPrefix)
	err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// fullKey calculates a key from a name and extra key parts.
func (s *genericResourceService[T]) fullKey(name string, extraKeyParts ...string) []byte {
	return backend.Key(append([]string{s.backendPrefix, name}, extraKeyParts...)...)
}
