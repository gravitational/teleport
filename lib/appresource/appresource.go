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

// Package appresource implements fine-grained access to HTTP apps. It checks
// whether an HTTP app request is allowed by the app_resources and
// app_resources_expressions rules of a caller's roles.
//
// When a role of version v9 or newer grants an app, every request to the app
// is denied unless a rule of a role that grants the app allows it. The v8 and
// older roles that grant the app are then dropped. When only older roles grant
// the app, the rules do not apply. The rules apply to HTTP apps only.
// GovernsApp returns false for the app kinds they do not apply to.
//
// The rules are allow-only. A role that sets either field under deny denies
// every request.
//
// A rule is an app_resources entry, the sugared form, or an
// app_resources_expressions entry, the desugared form. The first rule that
// matches allows the request. A request no rule allows is denied as
// DenyNotAllowed, with the hints that rules recorded.
//
// The "Fine-Grained Access Controls" page of the Teleport docs describes the
// rule syntax, the evaluation order, and the limits.
package appresource

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// MaxRulesPerRole is the maximum number of rules in one role.
const MaxRulesPerRole = 64

// MaxPathSegmentsPerRole is the maximum number of path segments in one role.
// It bounds what one role costs to compile, on write and on every read.
const MaxPathSegmentsPerRole = MaxRulesPerRole * maxPaths * 4 // 16384

// maxRulesPerRequest is the maximum number of rules across a caller's roles.
const maxRulesPerRequest = 256

// MaxPathSegmentsPerRoleSet is the maximum number of path segments across a
// caller's roles. Nothing limits how many roles a caller can hold.
const MaxPathSegmentsPerRoleSet = maxRulesPerRequest * maxPaths * 4 // 65536

// expressionSegmentChars are the characters an app_resources_expressions
// entry counts as path segments. Each node a path.match tree builds comes from
// a call, a "/" in a string, an escape, or a string argument, so the count is
// at least the number of nodes.
const expressionSegmentChars = "(/\\\"`"

// roleRules is a role's name and its allow app_resources and
// app_resources_expressions entries.
type roleRules struct {
	// Name is the role name, returned in Decision.Roles.
	Name string
	// Resources are the role's app_resources entries.
	Resources []types.AppResource
	// Expressions are the role's app_resources_expressions entries.
	Expressions []string
}

// hasAllowAll returns true when the role's only rule is an allow_all rule.
func (r roleRules) hasAllowAll() bool {
	return len(r.Resources) == 1 && len(r.Expressions) == 0 && r.Resources[0].IsAllowAllOnly()
}

// roleSet is the compiled rules of a caller's roles, in evaluation order.
type roleSet []compiledRole

// compiledRole is one role's compiled rules. allowAll is set when any
// app_resources entry sets allow_all.
type compiledRole struct {
	name     string
	allowAll bool
	rules    []ruleEvaluator
}

// compileRole compiles one role's rules, app_resources entries first.
func compileRole(role roleRules) (compiledRole, error) {
	if err := checkPathSegments(role); err != nil {
		return compiledRole{}, trace.Wrap(err)
	}
	compiled := compiledRole{name: role.Name, allowAll: role.hasAllowAll()}
	for i, rule := range role.Resources {
		c, err := newCompiledRule(rule)
		if err != nil {
			return compiledRole{}, trace.Wrap(err, "role %q app_resources %d", role.Name, i)
		}
		compiled.rules = append(compiled.rules, c)
	}
	for i, expr := range role.Expressions {
		expression, err := compileExpression(expr)
		if err != nil {
			return compiledRole{}, trace.Wrap(err, "role %q app_resources_expressions %d", role.Name, i)
		}
		compiled.rules = append(compiled.rules, expression)
	}
	return compiled, nil
}

// newRoleSet returns roles in evaluation order, the roles with an allow_all
// rule first and each group sorted by name.
func newRoleSet(roles []compiledRole) roleSet {
	set := roleSet(slices.Clone(roles))
	slices.SortStableFunc(set, func(a, b compiledRole) int {
		if a.allowAll && !b.allowAll {
			return -1
		}
		if b.allowAll && !a.allowAll {
			return 1
		}
		return strings.Compare(a.name, b.name)
	})
	return set
}

// roleNames returns the role names in the set, in evaluation order.
func (s roleSet) roleNames() []string {
	names := make([]string, 0, len(s))
	for _, role := range s {
		names = append(names, role.name)
	}
	return names
}

// evaluate returns the first matching rule's decision, or a deny with the
// hints recorded across all roles. An allow_all rule allows every request,
// including one with an unsupported method or a malformed path. An error
// means a rule could not be evaluated, and the Decision is the zero value.
func (s roleSet) evaluate(request Request, identity Identity) (Decision, error) {
	roles := s.roleNames()
	if name := s.allowAllRole(); name != "" {
		return Decision{
			Allowed: true,
			Allow:   &AllowDetails{Role: name, AllowAll: true},
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
			for _, hint := range result.AuditRecord.DenyHints {
				if hint == unsafeEncodedHint && slices.Contains(hints, hint) {
					continue
				}
				hints = append(hints, hint)
			}
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
func (s roleSet) ruleCount() int {
	n := 0
	for _, role := range s {
		n += len(role.rules)
	}
	return n
}

// allowAllRole returns the first role's name when it has an allow_all rule,
// and "" otherwise. Roles with an allow_all rule sort first, so no other role
// can have one.
func (s roleSet) allowAllRole() string {
	if len(s) == 0 || !s[0].allowAll {
		return ""
	}
	return s[0].name
}

// exceedsRequestLimits returns true when roles hold more rules or more path
// segments together than one request may evaluate.
func exceedsRequestLimits(roles []roleRules) bool {
	rules, segments := 0, 0
	for _, role := range roles {
		rules += len(role.Resources) + len(role.Expressions)
		segments += pathSegments(role)
	}
	return rules > maxRulesPerRequest || segments > MaxPathSegmentsPerRoleSet
}

// pathSegments returns the number of path segments in the app_resources paths
// and the app_resources_expressions entries of role. An entry counts one
// segment for each character of expressionSegmentChars in its text.
func pathSegments(role roleRules) int {
	segments := 0
	for _, rule := range role.Resources {
		for _, path := range rule.Paths {
			segments += strings.Count(path, "/")
		}
	}
	for _, expr := range role.Expressions {
		for _, c := range expr {
			if strings.ContainsRune(expressionSegmentChars, c) {
				segments++
			}
		}
	}
	return segments
}

// checkPathSegments rejects a role whose rules hold more than
// MaxPathSegmentsPerRole path segments together.
func checkPathSegments(role roleRules) error {
	if segments := pathSegments(role); segments > MaxPathSegmentsPerRole {
		return trace.BadParameter("role %q holds %d path segments, over the limit of %d", role.Name, segments, MaxPathSegmentsPerRole)
	}
	return nil
}
