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
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
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
			// Try to get the role name for the error, it allows admins to take action
			// against the "bad" role.
			h := &types.ResourceHeader{}
			_ = json.Unmarshal(item.Value, h)
			return nil, trace.WrapWithMessage(err, "role %q", h.GetName())
		}
		out = append(out, role)
	}
	sort.Sort(services.SortedRoles(out))
	return out, nil
}

// CreateRole creates a role on the backend.
func (s *AccessService) CreateRole(ctx context.Context, role types.Role) error {
	err := services.ValidateRoleName(role)
	if err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(rolesPrefix, role.GetName(), paramsPrefix),
		Value:   value,
		Expires: role.Expiry(),
	}

	_, err = s.Create(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertRole updates parameters about role
func (s *AccessService) UpsertRole(ctx context.Context, role types.Role) error {
	err := services.ValidateRoleName(role)
	if err != nil {
		return trace.Wrap(err)
	}

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

// GetLocks gets all/in-force locks that match at least one of the targets when specified.
func (s *AccessService) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
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
		if inForceOnly && !lock.IsInForce(s.Clock().Now()) {
			continue
		}
		// If no targets specified, return all of the found/in-force locks.
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

// DeleteLock deletes all/in-force locks.
func (s *AccessService) DeleteAllLocks(ctx context.Context) error {
	startKey := backend.Key(locksPrefix)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
func (s *AccessService) ReplaceRemoteLocks(ctx context.Context, clusterName string, newRemoteLocks []types.Lock) error {
	return backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:  s.Backend,
			LockName: "ReplaceRemoteLocks/" + clusterName,
			TTL:      time.Minute,
		},
	}, func(ctx context.Context) error {
		remoteLocksKey := backend.Key(locksPrefix, clusterName)
		origRemoteLocks, err := s.GetRange(ctx, remoteLocksKey, backend.RangeEnd(remoteLocksKey), backend.NoLimit)
		if err != nil {
			return trace.Wrap(err)
		}

		newRemoteLocksToStore := make(map[string]backend.Item, len(newRemoteLocks))
		for _, lock := range newRemoteLocks {
			if !strings.HasPrefix(lock.GetName(), clusterName) {
				lock.SetName(clusterName + "/" + lock.GetName())
			}
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
			newRemoteLocksToStore[string(item.Key)] = item
		}

		for _, origLockItem := range origRemoteLocks.Items {
			// If one of the new remote locks to store is already known,
			// perform a CompareAndSwap.
			key := string(origLockItem.Key)
			if newLockItem, ok := newRemoteLocksToStore[key]; ok {
				if _, err := s.CompareAndSwap(ctx, origLockItem, newLockItem); err != nil {
					return trace.Wrap(err)
				}
				delete(newRemoteLocksToStore, key)
				continue
			}

			// If an originally stored lock is not among the new locks,
			// delete it from the backend.
			if err := s.Delete(ctx, origLockItem.Key); err != nil {
				return trace.Wrap(err)
			}
		}

		// Store the remaining new locks.
		for _, newLockItem := range newRemoteLocksToStore {
			if _, err := s.Put(ctx, newLockItem); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
}

const (
	rolesPrefix  = "roles"
	paramsPrefix = "params"
	locksPrefix  = "locks"
)
