/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package appresource

import (
	"errors"
	"reflect"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ErrRolePredatesV9 is returned for a role of version v1 to v8.
var ErrRolePredatesV9 = errors.New("app_resources requires role version v9 or later")

// ErrRoleNotEvaluable is returned for a role with an unknown version or an
// unreadable rule.
var ErrRoleNotEvaluable = errors.New("cannot evaluate the role")

// NewRole returns the allow app_resources and app_resources_expressions
// of role as a Role. Path patterns, methods, and where clauses are
// checked when the rule compiles.
func NewRole(role types.Role) (Role, error) {
	version := role.GetVersion()
	switch {
	case RoleVersionPredatesV9(version):
		return Role{}, trace.Wrap(ErrRolePredatesV9, "role %q has version %q", role.GetName(), version)
	// A role newer than v9 may set restrictions this package does not
	// recognize, so evaluating its known fields could widen access.
	case version != types.V9:
		return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q has unknown role version %q", role.GetName(), version)
	case len(role.GetAppResources(types.Deny)) > 0:
		return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q sets app_resources under deny", role.GetName())
	case len(role.GetAppResourcesExpressions(types.Deny)) > 0:
		return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q sets app_resources_expressions under deny", role.GetName())
	}
	allow := role.GetAppResources(types.Allow)
	expressions := role.GetAppResourcesExpressions(types.Allow)
	for i, r := range allow {
		switch {
		case len(r.XXX_unrecognized) > 0:
			return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d has an unrecognized field", role.GetName(), i)
		case r.AllowAll && !r.IsAllowAllOnly():
			return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d combines allow_all with another field", role.GetName(), i)
		case r.AllowAll && len(allow) > 1:
			return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d sets allow_all with another rule", role.GetName(), i)
		case r.AllowAll && len(expressions) > 0:
			return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d sets allow_all with an app_resources_expressions entry", role.GetName(), i)
		case reflect.ValueOf(r).IsZero():
			return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d is blank", role.GetName(), i)
		}
	}
	return Role{
		Name:        role.GetName(),
		Resources:   allow,
		Expressions: expressions,
	}, nil
}

// RoleVersionPredatesV9 returns true when version is one of the role
// versions v1 to v8.
func RoleVersionPredatesV9(version string) bool {
	switch version {
	case types.V1, types.V2, types.V3, types.V4, types.V5, types.V6, types.V7, types.V8:
		return true
	}
	return false
}
