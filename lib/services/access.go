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

package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// LockGetter is a service that gets locks.
type LockGetter interface {
	// GetLock gets a lock by name.
	GetLock(ctx context.Context, name string) (types.Lock, error)
	// GetLocks gets all/in-force locks that match at least one of the targets when specified.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)
}

// Access service manages roles and permissions.
type Access interface {
	// GetRoles returns a list of roles.
	GetRoles(ctx context.Context) ([]types.Role, error)
	// ListRoles is a paginated role getter.
	ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
	// CreateRole creates a role.
	CreateRole(ctx context.Context, role types.Role) (types.Role, error)
	// UpdateRole updates an existing role.
	UpdateRole(ctx context.Context, role types.Role) (types.Role, error)
	// UpsertRole creates or updates role.
	UpsertRole(ctx context.Context, role types.Role) (types.Role, error)
	// DeleteAllRoles deletes all roles.
	DeleteAllRoles(ctx context.Context) error
	// GetRole returns role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)
	// DeleteRole deletes role by name.
	DeleteRole(ctx context.Context, name string) error

	LockGetter
	// UpsertLock upserts a lock.
	UpsertLock(context.Context, types.Lock) error
	// DeleteLock deletes a lock.
	DeleteLock(context.Context, string) error
	// DeleteAllLocks deletes all/in-force locks.
	DeleteAllLocks(context.Context) error
	// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
	ReplaceRemoteLocks(ctx context.Context, clusterName string, locks []types.Lock) error
}

var dynamicLabelsErrorMessage = fmt.Sprintf("labels with %q prefix are not allowed in deny rules", types.TeleportDynamicLabelPrefix)

// CheckDynamicLabelsInDenyRules checks if any deny rules in the given role use
// labels prefixed with "dynamic/".
func CheckDynamicLabelsInDenyRules(r types.Role) error {
	for _, kind := range types.LabelMatcherKinds {
		labelMatchers, err := r.GetLabelMatchers(types.Deny, kind)
		if err != nil {
			return trace.Wrap(err)
		}
		for label := range labelMatchers.Labels {
			if strings.HasPrefix(label, types.TeleportDynamicLabelPrefix) {
				return trace.BadParameter("%s", dynamicLabelsErrorMessage)
			}
		}
		const expressionMatch = `"` + types.TeleportDynamicLabelPrefix
		if strings.Contains(labelMatchers.Expression, expressionMatch) {
			return trace.BadParameter("%s", dynamicLabelsErrorMessage)
		}
	}

	for _, where := range []string{
		r.GetAccessReviewConditions(types.Deny).Where,
		r.GetImpersonateConditions(types.Deny).Where,
	} {
		if strings.Contains(where, types.TeleportDynamicLabelPrefix) {
			return trace.BadParameter("%s", dynamicLabelsErrorMessage)
		}
	}

	return nil
}
