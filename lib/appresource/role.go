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

// ValidateRole returns an error when role sets app_resources or
// app_resources_expressions that Teleport cannot evaluate, more than
// MaxRulesPerRole of them, or more than MaxPathSegmentsPerRole path
// segments. A role that sets neither field is valid, whatever its version.
func ValidateRole(role types.Role) error {
	allow := len(role.GetAppResources(types.Allow)) + len(role.GetAppResourcesExpressions(types.Allow))
	deny := len(role.GetAppResources(types.Deny)) + len(role.GetAppResourcesExpressions(types.Deny))
	if allow+deny == 0 {
		return nil
	}
	r, err := newRoleRules(role)
	if err != nil {
		return trace.Wrap(err)
	}
	if allow > MaxRulesPerRole {
		return trace.BadParameter("role %q holds %d app_resources and app_resources_expressions entries, over the limit of %d", role.GetName(), allow, MaxRulesPerRole)
	}
	_, err = compileRole(r)
	return trace.Wrap(err)
}

// CallerRoles are the roles of the caller of one app request.
type CallerRoles struct {
	// Held is every role the caller holds.
	Held []types.Role
	// Granting are the roles that grant the app.
	Granting []types.Role
}

// Partition is a role set split by what Teleport can evaluate.
type Partition struct {
	// IgnoredRoleNames are the names of the granting roles that predate v9,
	// sorted.
	IgnoredRoleNames []string
	// Errors are the errors for the roles Teleport cannot read or compile.
	// When a role sets rules under deny, they are the errors for those roles
	// only.
	Errors []RoleError

	// roleNames are the names of the compiled roles, or of every readable role
	// when overLimit is true, in the order they were given.
	roleNames []string
	// set is the compiled roles, in evaluation order.
	set roleSet
	// overLimit is true when the roles hold more rules or more path segments
	// together than one request may evaluate, so none of them compiled.
	overLimit bool
}

// Enforced returns true when a role of v9 or newer is in the partition, or a
// role sets rules under deny, so app_resources rules govern the request and
// the older roles are dropped.
func (p Partition) Enforced() bool {
	return len(p.roleNames) > 0 || len(p.Errors) > 0
}

// PartitionRoles splits roles.Granting into the three parts of [Partition],
// the roles this version of Teleport can read and compile, the names of the
// roles that predate v9, and an error per role it rejects.
//
// When a role in roles.Held or roles.Granting sets rules under deny, whatever
// its version, the partition holds only an error per such role, and the
// granting roles are not read or compiled.
func PartitionRoles(roles CallerRoles) Partition {
	if errs := denyRuleErrors(roles); len(errs) > 0 {
		return Partition{Errors: errs}
	}
	return partitionGranting(roles.Granting)
}

// denyRuleErrors returns an error per role in roles.Held or roles.Granting
// that sets rules under deny, each role once.
func denyRuleErrors(roles CallerRoles) []RoleError {
	var errs []RoleError
	rejected := map[string]struct{}{}
	for _, role := range slices.Concat(roles.Held, roles.Granting) {
		if _, ok := rejected[role.GetName()]; ok {
			continue
		}
		if err := checkDenyRules(role); err != nil {
			errs = append(errs, RoleError{Name: role.GetName(), Err: err})
			rejected[role.GetName()] = struct{}{}
		}
	}
	return errs
}

// partitionGranting splits the roles that grant the app into the roles
// Teleport can read and compile, the names of the pre-v9 roles, and an error
// per role it rejects.
func partitionGranting(granting []types.Role) Partition {
	var p Partition
	readable := make([]roleRules, 0, len(granting))
	for _, role := range granting {
		r, err := newRoleRules(role)
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
		for _, r := range readable {
			p.roleNames = append(p.roleNames, r.Name)
		}
		p.overLimit = true
		return p
	}
	compiled := make([]compiledRole, 0, len(readable))
	for _, r := range readable {
		c, err := compileRole(r)
		if err != nil {
			p.Errors = append(p.Errors, RoleError{Name: r.Name, Err: err})
			continue
		}
		p.roleNames = append(p.roleNames, r.Name)
		compiled = append(compiled, c)
	}
	p.set = newRoleSet(compiled)
	return p
}

// RoleNames returns the name of every role in p that app_resources rules
// govern, the roles with an error included.
func (p Partition) RoleNames() []string {
	names := make([]string, 0, len(p.roleNames)+len(p.Errors))
	names = append(names, p.roleNames...)
	for _, err := range p.Errors {
		names = append(names, err.Name)
	}
	return names
}

// Evaluate applies the roles in the partition to the given request and
// identity.
//
// A role that sets rules under deny denies as DenyRoleVersionUnsupported.
// Otherwise, roles with more rules or more path segments together than one
// request may evaluate deny as DenyTooManyRules. A role this version of
// Teleport cannot read denies as DenyRoleVersionUnsupported, whatever any
// other role allows, because a role it cannot read may restrict the request
// in a way it cannot see.
//
// An error means a rule could not be evaluated, and the Decision denies as
// DenyRoleVersionUnsupported.
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
	decision, err := p.set.evaluate(request, identity)
	if err != nil {
		return Decision{
			Deny:  &DenyDetails{Kind: DenyRoleVersionUnsupported},
			Roles: p.RoleNames(),
		}, trace.Wrap(err)
	}
	return decision, nil
}

// newRoleRules returns the allow app_resources and app_resources_expressions
// of role as a roleRules. Path patterns, methods, and where clauses are
// checked when the rule compiles.
func newRoleRules(role types.Role) (roleRules, error) {
	version := role.GetVersion()
	if RoleVersionPredatesV9(version) {
		return roleRules{}, trace.Wrap(ErrRolePredatesV9, "role %q has version %q", role.GetName(), version)
	}
	if err := checkDenyRules(role); err != nil {
		return roleRules{}, trace.Wrap(err)
	}
	// A role newer than v9 may set restrictions this package does not
	// recognize, so evaluating its known fields could widen access.
	if version != types.V9 {
		return roleRules{}, trace.Wrap(ErrRoleNotEvaluable, "role %q has unknown role version %q", role.GetName(), version)
	}
	allow := role.GetAppResources(types.Allow)
	expressions := role.GetAppResourcesExpressions(types.Allow)
	for i, r := range allow {
		switch {
		case len(r.XXX_unrecognized) > 0:
			return roleRules{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d has an unrecognized field", role.GetName(), i)
		case r.AllowUnsafe != nil && len(r.AllowUnsafe.XXX_unrecognized) > 0:
			return roleRules{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d allow_unsafe has an unrecognized field", role.GetName(), i)
		case r.AllowAll && !r.IsAllowAllOnly():
			return roleRules{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d combines allow_all with another field", role.GetName(), i)
		case r.AllowAll && len(allow) > 1:
			return roleRules{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d sets allow_all with another rule", role.GetName(), i)
		case r.AllowAll && len(expressions) > 0:
			return roleRules{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d sets allow_all with an app_resources_expressions entry", role.GetName(), i)
		case reflect.ValueOf(r).IsZero():
			return roleRules{}, trace.Wrap(ErrRoleNotEvaluable, "role %q app_resources %d is blank", role.GetName(), i)
		}
	}
	return roleRules{
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

// checkDenyRules returns an error wrapping ErrRoleHasDenyRules when role sets
// app_resources or app_resources_expressions under deny, and nil otherwise.
func checkDenyRules(role types.Role) error {
	switch {
	case len(role.GetAppResources(types.Deny)) > 0:
		return trace.Wrap(ErrRoleHasDenyRules, "role %q sets app_resources under deny", role.GetName())
	case len(role.GetAppResourcesExpressions(types.Deny)) > 0:
		return trace.Wrap(ErrRoleHasDenyRules, "role %q sets app_resources_expressions under deny", role.GetName())
	}
	return nil
}
