/*
Copyright 2016 Gravitational, Inc.

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
	"sort"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// AccessService manages roles
type AccessService struct {
	backend.Backend
}

// NewAccessService returns new access service instance
func NewAccessService(backend backend.Backend) *AccessService {
	return &AccessService{Backend: backend}
}

// DeleteAllRoles deletes all roles
func (s *AccessService) DeleteAllRoles() error {
	return s.DeleteRange(context.TODO(), backend.Key(rolesPrefix), backend.RangeEnd(backend.Key(rolesPrefix)))
}

// GetRoles returns a list of roles registered with the local auth server
func (s *AccessService) GetRoles(ctx context.Context) ([]types.Role, error) {
	result, err := s.GetRange(ctx, backend.Key(rolesPrefix), backend.RangeEnd(backend.Key(rolesPrefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]types.Role, 0, len(result.Items))
	for _, item := range result.Items {
		role, err := services.UnmarshalRole(item.Value,
			services.WithResourceID(item.ID), services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, role)
	}
	sort.Sort(services.SortedRoles(out))
	return out, nil
}

// CreateRole creates a role on the backend.
func (s *AccessService) CreateRole(role types.Role) error {
	value, err := services.MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(rolesPrefix, role.GetName(), paramsPrefix),
		Value:   value,
		Expires: role.Expiry(),
	}

	_, err = s.Create(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertRole updates parameters about role
func (s *AccessService) UpsertRole(ctx context.Context, role types.Role) error {
	value, err := services.MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(rolesPrefix, role.GetName(), paramsPrefix),
		Value:   value,
		Expires: role.Expiry(),
		ID:      role.GetResourceID(),
	}

	_, err = s.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetRole returns a role by name
func (s *AccessService) GetRole(ctx context.Context, name string) (types.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing role name")
	}
	item, err := s.Get(ctx, backend.Key(rolesPrefix, name, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("role %v is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalRole(item.Value,
		services.WithResourceID(item.ID), services.WithExpires(item.Expires))
}

// DeleteRole deletes a role from the backend
func (s *AccessService) DeleteRole(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing role name")
	}
	err := s.Delete(ctx, backend.Key(rolesPrefix, name, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("role %q is not found", name)
		}
	}
	return trace.Wrap(err)
}

// GetLock gets a lock by name.
func (s *AccessService) GetLock(ctx context.Context, name string) (types.Lock, error) {
	if name == "" {
		return nil, trace.BadParameter("missing lock name")
	}
	item, err := s.Get(ctx, backend.Key(locksPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("lock %q is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalLock(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
}

// GetLocks gets all locks, matching at least one of the targets when specified.
func (s *AccessService) GetLocks(ctx context.Context, targets ...types.LockTarget) ([]types.Lock, error) {
	startKey := backend.Key(locksPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := []types.Lock{}
	for _, item := range result.Items {
		lock, err := services.UnmarshalLock(item.Value, services.WithResourceID(item.ID), services.WithExpires(item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// If no targets specified, return all of the found locks.
		if len(targets) == 0 {
			out = append(out, lock)
			continue
		}
		// Otherwise, use the targets as filters.
		for _, target := range targets {
			if target.Match(lock) {
				out = append(out, lock)
				break
			}
		}
	}
	return out, nil
}

// UpsertLock upserts a lock.
func (s *AccessService) UpsertLock(ctx context.Context, lock types.Lock) error {
	value, err := services.MarshalLock(lock)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(locksPrefix, lock.GetName()),
		Value:   value,
		Expires: lock.Expiry(),
		ID:      lock.GetResourceID(),
	}

	if _, err = s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteLock deletes a lock.
func (s *AccessService) DeleteLock(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing lock name")
	}
	err := s.Delete(ctx, backend.Key(locksPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("lock %q is not found", name)
		}
	}
	return trace.Wrap(err)
}

// DeleteLock deletes all locks.
func (s *AccessService) DeleteAllLocks(ctx context.Context) error {
	startKey := backend.Key(locksPrefix)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

const (
	rolesPrefix  = "roles"
	paramsPrefix = "params"
	locksPrefix  = "locks"
)
