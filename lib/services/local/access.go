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

package local

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// AccessService manages roles
type AccessService struct {
	backend.Backend
	logger *slog.Logger
}

// NewAccessService returns new access service instance
func NewAccessService(backend backend.Backend) *AccessService {
	return &AccessService{
		Backend: backend,
		logger:  slog.With(teleport.ComponentKey, "AccessService"),
	}
}

// DeleteAllRoles deletes all roles
func (s *AccessService) DeleteAllRoles(ctx context.Context) error {
	startKey := backend.NewKey(rolesPrefix)
	endKey := backend.RangeEnd(startKey)
	return s.DeleteRange(ctx, startKey, endKey)
}

// GetRoles returns a list of roles registered with the local auth server
func (s *AccessService) GetRoles(ctx context.Context) ([]types.Role, error) {
	var maxIterations = 100_000
	var roles []types.Role
	var req proto.ListRolesRequest
	var iterations int
	for {
		iterations++
		if iterations > maxIterations {
			return nil, trace.Errorf("too many internal get role page iterations (%d), this is a bug", iterations)
		}
		rsp, err := s.ListRoles(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range rsp.Roles {
			roles = append(roles, r)
		}
		req.StartKey = rsp.NextKey
		if req.StartKey == "" {
			break
		}
	}

	return roles, nil
}

// ListRoles is a paginated role getter.
func (s *AccessService) ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error) {
	const maxPageSize = 16_000

	limit := int(req.Limit)

	if limit == 0 {
		// it can take a lot of effort to parse roles and until a page is done
		// parsing, it will be held in memory - so keep this reasonably small
		limit = 100
	}

	if limit > maxPageSize {
		return nil, trace.BadParameter("page size of %d is too large", limit)
	}

	startKey := backend.ExactKey(rolesPrefix)
	if req.StartKey != "" {
		startKey = backend.NewKey(rolesPrefix, req.StartKey, paramsPrefix)
	}

	endKey := backend.RangeEnd(backend.ExactKey(rolesPrefix))

	var roles []*types.RoleV6
	if err := backend.IterateRange(ctx, s.Backend, startKey, endKey, limit+1, func(items []backend.Item) (stop bool, err error) {
		for _, item := range items {
			if len(roles) > limit {
				return true, nil
			}

			if !item.Key.HasSuffix(backend.NewKey(paramsPrefix)) {
				// Item represents a different resource type in the
				// same namespace.
				continue
			}

			role, err := services.UnmarshalRoleV6(
				item.Value,
				services.WithExpires(item.Expires),
				services.WithRevision(item.Revision),
			)
			if err != nil {
				s.logger.WarnContext(ctx, "Failed to unmarshal role",
					"key", item.Key,
					"error", err,
				)
				continue
			}

			// if a filter was provided, skip roles that fail to match.
			if req.Filter != nil && !req.Filter.Match(role) {
				continue
			}

			roles = append(roles, role)
		}

		return len(roles) > limit, nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	var nextKey string
	if len(roles) > limit {
		nextKey = roles[limit].GetName()
		roles = roles[:limit]
	}

	return &proto.ListRolesResponse{
		Roles:   roles,
		NextKey: nextKey,
	}, nil
}

// CreateRole creates a new role.
func (s *AccessService) CreateRole(ctx context.Context, role types.Role) (types.Role, error) {
	err := services.ValidateRoleName(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rev := role.GetRevision()
	value, err := services.MarshalRole(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(rolesPrefix, role.GetName(), paramsPrefix),
		Value:    value,
		Expires:  role.Expiry(),
		Revision: rev,
	}

	lease, err := s.Create(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role.SetRevision(lease.Revision)
	return role, nil
}

// UpdateRole updates an existing role.
func (s *AccessService) UpdateRole(ctx context.Context, role types.Role) (types.Role, error) {
	err := services.ValidateRoleName(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rev := role.GetRevision()
	value, err := services.MarshalRole(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(rolesPrefix, role.GetName(), paramsPrefix),
		Value:    value,
		Expires:  role.Expiry(),
		Revision: rev,
	}

	lease, err := s.ConditionalUpdate(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role.SetRevision(lease.Revision)
	return role, nil
}

// UpsertRole creates or overwrites an existing role.
func (s *AccessService) UpsertRole(ctx context.Context, role types.Role) (types.Role, error) {
	err := services.ValidateRoleName(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rev := role.GetRevision()
	value, err := services.MarshalRole(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := backend.Item{
		Key:      backend.NewKey(rolesPrefix, role.GetName(), paramsPrefix),
		Value:    value,
		Expires:  role.Expiry(),
		Revision: rev,
	}

	lease, err := s.Put(ctx, item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role.SetRevision(lease.Revision)
	return role, nil
}

// GetRole returns a role by name
func (s *AccessService) GetRole(ctx context.Context, name string) (types.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing role name")
	}
	item, err := s.Get(ctx, backend.NewKey(rolesPrefix, name, paramsPrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			// This error message format should be kept in sync with web/packages/teleport/src/services/api/api.isRoleNotFoundError
			return nil, trace.NotFound("role %v is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalRole(item.Value,
		services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// DeleteRole deletes a role from the backend
func (s *AccessService) DeleteRole(ctx context.Context, name string) error {
	if name == "" {
		return trace.BadParameter("missing role name")
	}
	err := s.Delete(ctx, backend.NewKey(rolesPrefix, name, paramsPrefix))
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
	item, err := s.Get(ctx, backend.NewKey(locksPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("lock %q is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalLock(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
}

// GetLocks gets all/in-force locks that match at least one of the targets when specified.
func (s *AccessService) GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error) {
	startKey := backend.ExactKey(locksPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := []types.Lock{}
	for _, item := range result.Items {
		lock, err := services.UnmarshalLock(item.Value, services.WithExpires(item.Expires), services.WithRevision(item.Revision))
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
	rev := lock.GetRevision()
	value, err := services.MarshalLock(lock)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(locksPrefix, lock.GetName()),
		Value:    value,
		Expires:  lock.Expiry(),
		Revision: rev,
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
	err := s.Delete(ctx, backend.NewKey(locksPrefix, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("lock %q is not found", name)
		}
	}
	return trace.Wrap(err)
}

// DeleteLock deletes all/in-force locks.
func (s *AccessService) DeleteAllLocks(ctx context.Context) error {
	startKey := backend.ExactKey(locksPrefix)
	return s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey))
}

// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
func (s *AccessService) ReplaceRemoteLocks(ctx context.Context, clusterName string, newRemoteLocks []types.Lock) error {
	return backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            s.Backend,
			LockNameComponents: []string{"ReplaceRemoteLocks", clusterName},
			TTL:                time.Minute,
		},
	}, func(ctx context.Context) error {
		remoteLocksKey := backend.ExactKey(locksPrefix, clusterName)
		origRemoteLocks, err := s.GetRange(ctx, remoteLocksKey, backend.RangeEnd(remoteLocksKey), backend.NoLimit)
		if err != nil {
			return trace.Wrap(err)
		}

		newRemoteLocksToStore := make(map[string]backend.Item, len(newRemoteLocks))
		for _, lock := range newRemoteLocks {
			key := backend.NewKey(locksPrefix)
			if !strings.HasPrefix(lock.GetName(), clusterName) {
				key = key.AppendKey(backend.NewKey(clusterName, lock.GetName()))
				lock.SetName(clusterName + "/" + lock.GetName())
			} else {
				key = key.AppendKey(backend.NewKey(lock.GetName()))
			}
			rev := lock.GetRevision()
			value, err := services.MarshalLock(lock)
			if err != nil {
				return trace.Wrap(err)
			}
			item := backend.Item{
				Key:      key,
				Value:    value,
				Expires:  lock.Expiry(),
				Revision: rev,
			}
			newRemoteLocksToStore[item.Key.String()] = item
		}

		for _, origLockItem := range origRemoteLocks.Items {
			// If one of the new remote locks to store is already known,
			// perform a CompareAndSwap.
			key := origLockItem.Key.String()
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
