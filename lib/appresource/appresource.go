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

// Package appresource checks whether an HTTP app request is allowed
// by a role. Roles carry allow-only rules. A rule can match on
// request path, HTTP method, and a where predicate over the user
// identity. A rule sets either paths, with the other fields
// optional, or allow_all, which stands alone.
//
// Example role fragment:
//
//	allow:
//	  app_resources:
//	    - paths:
//	        - /api/v4/user/{username}
//	      where: user.name == vars.username
package appresource

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// MaxRulesPerRole is the maximum number of rules and expressions in one role.
const MaxRulesPerRole = 64

// MaxPathSegmentsPerRole is the maximum number of path segments in one role.
// It bounds what one role costs to compile, on write and on every read.
const MaxPathSegmentsPerRole = MaxRulesPerRole * maxPaths * 4 // 16384

// maxRulesPerRequest is the maximum number of rules across a caller's roles.
const maxRulesPerRequest = 256

// MaxPathSegmentsPerRoleSet is the maximum number of path segments across a
// caller's roles. Nothing limits how many roles a caller can hold.
const MaxPathSegmentsPerRoleSet = maxRulesPerRequest * maxPaths * 4 // 65536

// Role is a role's name and its allow app_resources and
// app_resources_expressions entries.
type Role struct {
	// Name is the role name, returned in Decision.Roles.
	Name string
	// Resources are the role's app_resources entries.
	Resources []types.AppResource
	// Expressions are the role's app_resources_expressions entries.
	Expressions []string
}

// hasAllowAll returns true when any app_resources entry sets allow_all.
func (r Role) hasAllowAll() bool {
	return slices.ContainsFunc(r.Resources, func(rule types.AppResource) bool { return rule.AllowAll })
}

// RoleSet is the compiled rules of a caller's roles, in evaluation order.
type RoleSet []compiledRole

// compiledRole is one role's compiled rules. allowAll is set when any
// app_resources entry sets allow_all.
type compiledRole struct {
	name     string
	allowAll bool
	rules    []ruleEvaluator
}

// CompileRoles compiles the roles a caller holds into a RoleSet. Roles with
// an allow_all rule come first, each group sorted by name, and within a role
// app_resources entries come before app_resources_expressions entries. It
// returns an error for an invalid rule.
func CompileRoles(roles []Role) (RoleSet, error) {
	sorted := slices.Clone(roles)
	slices.SortStableFunc(sorted, func(a, b Role) int {
		if a.hasAllowAll() && !b.hasAllowAll() {
			return -1
		}
		if b.hasAllowAll() && !a.hasAllowAll() {
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	if err := checkRoleSet(sorted); err != nil {
		return nil, trace.Wrap(err)
	}
	set := make(RoleSet, 0, len(sorted))
	for _, role := range sorted {
		if err := checkPathSegments(role); err != nil {
			return nil, trace.Wrap(err)
		}
		compiled := compiledRole{name: role.Name, allowAll: role.hasAllowAll()}
		for i, rule := range role.Resources {
			c, err := newCompiledRule(rule)
			if err != nil {
				return nil, trace.Wrap(err, "role %q app_resources %d", role.Name, i)
			}
			compiled.rules = append(compiled.rules, c)
		}
		for i, expr := range role.Expressions {
			expression, err := compileExpression(expr)
			if err != nil {
				return nil, trace.Wrap(err, "role %q app_resources_expressions %d", role.Name, i)
			}
			compiled.rules = append(compiled.rules, expression)
		}
		set = append(set, compiled)
	}
	return set, nil
}

// roleNames returns the role names in the set, in evaluation order.
func (s RoleSet) roleNames() []string {
	names := make([]string, 0, len(s))
	for _, role := range s {
		names = append(names, role.name)
	}
	return names
}

// Evaluate returns the first matching rule's decision, or a deny with the
// hints recorded across all roles. An allow_all rule allows every request,
// including one with an unsupported method or a malformed path. An error
// means a rule could not be evaluated, and the Decision is the zero value.
func (s RoleSet) Evaluate(request Request, identity Identity) (Decision, error) {
	roles := s.roleNames()
	if name := s.allowAllRole(); name != "" {
		return Decision{
			Allowed: true,
			Allow:   &AllowDetails{Role: name},
			Roles:   roles,
		}, nil
	}
	n := s.ruleCount()
	if n > maxRulesPerRequest {
		return Decision{
			Deny:  &DenyDetails{Kind: DenyTooManyRules},
			Roles: roles,
		}, nil
	}
	if n == 0 {
		return Decision{
			Deny:  &DenyDetails{Kind: DenyNotAllowed},
			Roles: roles,
		}, nil
	}
	env, err := NewEnv(request, identity)
	if err != nil {
		return Decision{
			Deny:  &DenyDetails{Kind: DenyInvalidRequest},
			Roles: roles,
		}, nil
	}

	var hints []Hint
	for _, role := range s {
		for _, rule := range role.rules {
			result, err := evaluateExpression(rule, env)
			if err != nil {
				return Decision{}, trace.Wrap(err)
			}
			if result.Value {
				record := result.AuditRecord
				return Decision{
					Allowed: true,
					Allow:   &AllowDetails{Role: role.name, Vars: result.vars, Code: record.AllowCode, Reason: record.AllowReason},
					Roles:   roles,
				}, nil
			}
			hints = append(hints, result.AuditRecord.DenyHints...)
			if len(hints) > maxHints {
				hints = hints[:maxHints]
			}
		}
	}
	return Decision{
		Deny:  &DenyDetails{Kind: DenyNotAllowed, Hints: hints},
		Roles: roles,
	}, nil
}

// ruleCount returns the number of rules across the set.
func (s RoleSet) ruleCount() int {
	n := 0
	for _, role := range s {
		n += len(role.rules)
	}
	return n
}

// allowAllRole returns the first role's name when it has an allow_all rule,
// and "" otherwise. Roles with an allow_all rule sort first, so no other role
// can have one.
func (s RoleSet) allowAllRole() string {
	if len(s) == 0 || !s[0].allowAll {
		return ""
	}
	return s[0].name
}

// checkRoleSet rejects a set of roles holding more than
// MaxPathSegmentsPerRoleSet path segments together. It runs before any role
// compiles, so an oversized set costs nothing to reject.
func checkRoleSet(roles []Role) error {
	segments := 0
	for _, role := range roles {
		segments += pathSegments(role)
	}
	if segments > MaxPathSegmentsPerRoleSet {
		return trace.BadParameter("the roles hold %d path segments, over the cap of %d", segments, MaxPathSegmentsPerRoleSet)
	}
	return nil
}

// pathSegments returns the number of path segments across every path pattern
// of role.
func pathSegments(role Role) int {
	segments := 0
	for _, rule := range role.Resources {
		for _, path := range rule.Paths {
			segments += strings.Count(path, "/")
		}
	}
	return segments
}

// checkPathSegments rejects a role whose path patterns hold more than
// MaxPathSegmentsPerRole segments together.
func checkPathSegments(role Role) error {
	if segments := pathSegments(role); segments > MaxPathSegmentsPerRole {
		return trace.BadParameter("role %q holds %d path segments, over the cap of %d", role.Name, segments, MaxPathSegmentsPerRole)
	}
	return nil
}
