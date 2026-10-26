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
	"fmt"
	"reflect"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ErrRolePredatesV9 is returned for a role of version v1 to v8.
var ErrRolePredatesV9 = errors.New("app_resources requires role version v9 or later")

// ErrRoleNotEvaluable is returned for a role with an unknown version or an
// unreadable rule.
var ErrRoleNotEvaluable = errors.New("cannot evaluate the role")

// ErrRoleHasDenyRules is returned for a role with rules under deny.
var ErrRoleHasDenyRules = fmt.Errorf("%w, deny overrides allow across the role set", ErrRoleNotEvaluable)

// RoleError is the error for one role, with the role's name.
type RoleError struct {
	// Name is the role name.
	Name string
	// Err is the error the role failed with.
	Err error
}

// Error returns the message of the wrapped error.
func (e RoleError) Error() string { return e.Err.Error() }

// Unwrap returns the wrapped error, so errors.Is matches the sentinels.
func (e RoleError) Unwrap() error { return e.Err }

// Partition is a role set split by what Teleport can evaluate.
type Partition struct {
	// Roles are the roles NewRole accepts, in the order they were given.
	Roles []Role
	// IgnoredRoleNames are the names of the roles that predate v9, sorted.
	IgnoredRoleNames []string
	// Errors are the errors for the roles Teleport cannot read or compile.
	Errors []RoleError

	// set is the compiled Roles, in evaluation order.
	set RoleSet
	// overLimit is true when the roles hold more rules or more path segments
	// together than one request may evaluate, so none of them compiled.
	overLimit bool
}

// Enforced returns true when a role of v9 or newer is in the partition, so
// app_resources rules govern the request and the older roles are dropped.
func (p Partition) Enforced() bool {
	return len(p.Roles) > 0 || len(p.Errors) > 0
}

// PartitionRoles splits roles into the three parts of [Partition], the roles
// this version of Teleport can read and compile, the names of the roles that
// predate v9, and an error per role it rejects.
//
// TODO(juliaogris): Consolidate compilation on PartitionRoles, so
// CompileRoles is not a second way to build a RoleSet. CompileRoles counts
// path segments only, so tctl allows a role set over the rule limit that
// holds an allow_all rule, and the agent denies it. Align tctl with the
// agent.
func PartitionRoles(roles []types.Role) Partition {
	var p Partition
	readable := make([]Role, 0, len(roles))
	for _, role := range roles {
		r, err := NewRole(role)
		switch {
		case errors.Is(err, ErrRolePredatesV9):
			p.IgnoredRoleNames = append(p.IgnoredRoleNames, role.GetName())
		case err != nil:
			p.Errors = append(p.Errors, RoleError{Name: role.GetName(), Err: err})
		default:
			readable = append(readable, r)
		}
	}
	slices.Sort(p.IgnoredRoleNames)
	// The request limits run before any role compiles, so an oversized set
	// costs nothing to reject.
	if exceedsRequestLimits(readable) {
		p.Roles = readable
		p.overLimit = true
		return p
	}
	p.Roles = make([]Role, 0, len(readable))
	compiled := make([]compiledRole, 0, len(readable))
	for _, r := range readable {
		c, err := compileRole(r)
		if err != nil {
			p.Errors = append(p.Errors, RoleError{Name: r.Name, Err: err})
			continue
		}
		p.Roles = append(p.Roles, r)
		compiled = append(compiled, c)
	}
	p.set = newRoleSet(compiled)
	return p
}

// RoleNames returns the name of every role in p that app_resources rules
// govern, the roles with an error included.
func (p Partition) RoleNames() []string {
	names := make([]string, 0, len(p.Roles)+len(p.Errors))
	for _, role := range p.Roles {
		names = append(names, role.Name)
	}
	for _, err := range p.Errors {
		names = append(names, err.Name)
	}
	return names
}

// Evaluate applies the roles in the partition to the given request and
// identity.
//
// Roles with more rules or more path segments together than one request may
// evaluate deny as DenyTooManyRules. A role this version of Teleport cannot
// read denies as DenyRoleVersionUnsupported, whatever any other role allows,
// because a role it cannot read may restrict the request in a way it cannot
// see.
//
// An error means a rule could not be evaluated, and the Decision is the zero
// value.
func (p Partition) Evaluate(request Request, identity Identity) (Decision, error) {
	if p.overLimit {
		return Decision{
			Deny:  &DenyDetails{Kind: DenyTooManyRules},
			Roles: p.RoleNames(),
		}, nil
	}
	if len(p.Errors) > 0 {
		return Decision{
			Deny:  &DenyDetails{Kind: DenyRoleVersionUnsupported},
			Roles: p.RoleNames(),
		}, nil
	}
	decision, err := p.set.Evaluate(request, identity)
	if err != nil {
		return Decision{}, trace.Wrap(err)
	}
	return decision, nil
}

// NewRole returns the allow app_resources and app_resources_expressions
// of role as a Role. Path patterns, methods, and where clauses are
// checked when the rule compiles.
func NewRole(role types.Role) (Role, error) {
	version := role.GetVersion()
	switch {
	case RoleVersionPredatesV9(version):
		return Role{}, trace.Wrap(ErrRolePredatesV9, "role %q has version %q", role.GetName(), version)
	case len(role.GetAppResources(types.Deny)) > 0:
		return Role{}, trace.Wrap(ErrRoleHasDenyRules, "role %q sets app_resources under deny", role.GetName())
	case len(role.GetAppResourcesExpressions(types.Deny)) > 0:
		return Role{}, trace.Wrap(ErrRoleHasDenyRules, "role %q sets app_resources_expressions under deny", role.GetName())
	// A role newer than v9 may set restrictions this package does not
	// recognize, so evaluating its known fields could widen access.
	case version != types.V9:
		return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q has unknown role version %q", role.GetName(), version)
	}
	allow := role.GetAppResources(types.Allow)
	expressions := role.GetAppResourcesExpressions(types.Allow)
	for i, r := range allow {
		switch {
		case len(r.XXX_unrecognized) > 0:
			return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d has an unrecognized field", role.GetName(), i)
		case r.AllowUnsafe != nil && len(r.AllowUnsafe.XXX_unrecognized) > 0:
			return Role{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d allow_unsafe has an unrecognized field", role.GetName(), i)
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

// HasDenyRules returns the name of the first role with app_resources or
// app_resources_expressions under deny, and true. It returns "" and false
// when no role has one.
func HasDenyRules(roles []types.Role) (string, bool) {
	for _, role := range roles {
		if len(role.GetAppResources(types.Deny)) > 0 || len(role.GetAppResourcesExpressions(types.Deny)) > 0 {
			return role.GetName(), true
		}
	}
	return "", false
}
