/*
Copyright 2021 Gravitational, Inc.

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
	CreateRole(ctx context.Context, role types.Role) error
	// UpsertRole creates or updates role.
	UpsertRole(ctx context.Context, role types.Role) error
	// DeleteAllRoles deletes all roles.
	DeleteAllRoles() error
	// GetRole returns role by name.
	GetRole(ctx context.Context, name string) (types.Role, error)
	// DeleteRole deletes role by name.
	DeleteRole(ctx context.Context, name string) error

	LockGetter
	// UpsertLock upserts a lock.
	UpsertLock(context.Context, types.Lock) error
	// DeleteLock deletes a lock.
	DeleteLock(context.Context, string) error
	// DeleteLock deletes all/in-force locks.
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
				return trace.BadParameter(dynamicLabelsErrorMessage)
			}
		}
		const expressionMatch = `"` + types.TeleportDynamicLabelPrefix
		if strings.Contains(labelMatchers.Expression, expressionMatch) {
			return trace.BadParameter(dynamicLabelsErrorMessage)
		}
	}

	for _, where := range []string{
		r.GetAccessReviewConditions(types.Deny).Where,
		r.GetImpersonateConditions(types.Deny).Where,
	} {
		if strings.Contains(where, types.TeleportDynamicLabelPrefix) {
			return trace.BadParameter(dynamicLabelsErrorMessage)
		}
	}

	return nil
}
