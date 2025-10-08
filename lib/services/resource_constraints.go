/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
	"slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// MatcherTransform defines a func wrapping a RoleMatcher to modify or extend its behavior.
type MatcherTransform func(RoleMatcher) RoleMatcher

// WithConstraints returns a MatcherTransform that scopes principal-bearing
// RoleMatchers to any provided ResourceConstraints.
//
// For matchers that encode a specific principal (e.g., AWS Role ARN, IC assignment,
// SSH login), the returned transform first checks that principal against the provided
// ResourceConstraints; if it's not present, the transformed matcher fails fast. If it is
// present, the original matcher's logic is applied.
//
// For non-principal-bearing matchers, the transform is a no-op.
//
// This enforces that even if a role would otherwise match a principal on a
// resource, the principal must also be allowed by the resource's Constraints.
func WithConstraints(rc *types.ResourceConstraints) MatcherTransform {
	if rc == nil {
		return func(m RoleMatcher) RoleMatcher { return m }
	}

	switch rc.Domain {
	case types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_CONSOLE:
		ac := rc.GetAWSConsole()
		if ac == nil || len(ac.RoleARNs) == 0 {
			return func(m RoleMatcher) RoleMatcher {
				return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
					// TODO(kiosion): Do we want to error here, or just log / return the original matcher? Should missing constraints, with a domain, be considered a failure?
					return false, trace.BadParameter("aws_console constraints require role_arns, none provided")
				})
			}
		}
		return func(m RoleMatcher) RoleMatcher {
			lm, ok := m.(*awsAppLoginMatcher)
			if !ok {
				return m
			}
			return RoleMatcherFunc(func(role types.Role, cond types.RoleConditionType) (bool, error) {
				if !slices.Contains(ac.RoleARNs, lm.awsRole) {
					return false, nil
				}
				return m.Match(role, cond)
			})
		}
	case types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_IDENTITY_CENTER:
		ic := rc.GetAWSIC()
		if ic == nil || len(ic.AccountAssignments) == 0 {
			return func(m RoleMatcher) RoleMatcher {
				return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
					return false, trace.BadParameter("aws_ic constraints require account_assignments, none provided")
				})
			}
		}
		return func(m RoleMatcher) RoleMatcher {
			am, ok := m.(*IdentityCenterAccountAssignmentMatcher)
			if !ok {
				return m
			}
			return RoleMatcherFunc(func(role types.Role, cond types.RoleConditionType) (bool, error) {
				if !slices.ContainsFunc(ic.AccountAssignments, func(a types.IdentityCenterAccountAssignment) bool {
					return a.Account == am.accountID && a.PermissionSet == am.permissionSetARN
				}) {
					return false, nil
				}
				return m.Match(role, cond)
			})
		}
	default:
		return func(m RoleMatcher) RoleMatcher {
			return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
				return false, trace.BadParameter("unsupported constraint domain %q", rc.Domain)
			})
		}
	}
}

// MatcherFromConstraints constructs a RoleMatcher encoding the requested
// ResourceConstraints for role resolution/validation time.
//
// This matcher is intended for use in request expansion, to decide whether a
// role qualifies for a resource where ResourceConstraints are specified.
//
// For enforcement of ResourceConstraints at authorization time, use
// WithConstraints to decorate principal-bearing matchers instead.
func MatcherFromConstraints(rid types.ResourceID) (RoleMatcher, error) {
	if rid.Constraints == nil {
		return nil, nil
	}

	rc := rid.Constraints
	switch rc.Domain {
	case types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_CONSOLE:
		ac := rc.GetAWSConsole()
		if ac == nil || len(ac.RoleARNs) == 0 {
			return nil, trace.BadParameter("aws_console constraints require role_arns, none provided")
		}
		matchers := make([]RoleMatcher, 0, len(ac.RoleARNs))
		for _, arn := range ac.RoleARNs {
			matchers = append(matchers, &AWSRoleARNMatcher{RoleARN: arn})
		}
		return RoleMatchers(matchers).AnyOf(), nil
	case types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_IDENTITY_CENTER:
		ic := rc.GetAWSIC()
		if ic == nil || len(ic.AccountAssignments) == 0 {
			return nil, trace.BadParameter("aws_ic constraints require account_assignments, none provided")
		}
		// Match any of the listed permission set names against the role's IC assignments.
		psMatchers := make([]RoleMatcher, 0, len(ic.AccountAssignments))
		for _, ac := range ic.AccountAssignments {
			psMatchers = append(psMatchers, &IdentityCenterAccountAssignmentMatcher{accountID: ac.Account, permissionSetARN: ac.PermissionSet})
		}
		return RoleMatchers(psMatchers).AnyOf(), nil
	default:
		return nil, trace.BadParameter("unsupported constraint domain %v", rid.Constraints.Domain)
	}
}
