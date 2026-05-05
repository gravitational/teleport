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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/set"
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

	switch d := rc.Details.(type) {
	case *types.ResourceConstraints_AwsConsole:
		return buildStringConstraintTransform(
			d.Validate,
			func() []string { return d.AwsConsole.RoleArns },
			func(m RoleMatcher) string {
				principal := ""
				switch lm := m.(type) {
				case *awsAppLoginMatcher:
					principal = lm.awsRole
				case *AWSRoleARNMatcher:
					principal = lm.RoleARN
				}
				return principal
			},
		)
	case *types.ResourceConstraints_Ssh:
		return buildStringConstraintTransform(
			d.Validate,
			func() []string { return d.Ssh.Logins },
			func(m RoleMatcher) string {
				lm, ok := m.(*loginMatcher)
				if !ok {
					return ""
				}
				return lm.login
			},
		)
	case *types.ResourceConstraints_AzureApp:
		return buildStringConstraintTransform(
			d.Validate,
			func() []string { return d.AzureApp.AzureIdentities },
			func(m RoleMatcher) string {
				lm, ok := m.(*AzureIdentityMatcher)
				if !ok {
					return ""
				}
				return lm.Identity
			},
		)
	case *types.ResourceConstraints_GcpApp:
		return buildStringConstraintTransform(
			d.Validate,
			func() []string { return d.GcpApp.GcpServiceAccounts },
			func(m RoleMatcher) string {
				lm, ok := m.(*GCPServiceAccountMatcher)
				if !ok {
					return ""
				}
				return lm.ServiceAccount
			},
		)
	default:
		return func(m RoleMatcher) RoleMatcher {
			return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
				return false, trace.BadParameter("unsupported constraint details type %T", d)
			})
		}
	}
}

// buildStringConstraintTransform factors out shared logic for string-list-based
// ResourceConstraints (e.g., AWS role ARNs, SSH logins). It handles validation,
// then builds the principal-gated RoleMatcher transform.
func buildStringConstraintTransform(
	validate func() error,
	getStrings func() []string,
	getPrincipal func(RoleMatcher) string,
) MatcherTransform {
	if err := validate(); err != nil {
		return func(m RoleMatcher) RoleMatcher {
			return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
				return false, trace.Wrap(err)
			})
		}
	}

	allowedSet := set.New(getStrings()...)

	return func(m RoleMatcher) RoleMatcher {
		principal := getPrincipal(m)
		if principal == "" {
			return m // non-principal-bearing matcher; no-op
		}
		return RoleMatcherFunc(func(role types.Role, cond types.RoleConditionType) (bool, error) {
			if !allowedSet.Contains(principal) {
				return false, nil
			}
			return m.Match(role, cond)
		})
	}
}

// MatcherFromConstraints constructs a RoleMatcher encoding the requested
// ResourceConstraints for role resolution/validation time.
//
// This matcher is intended for use in request expansion, to decide whether a
// role qualifies for a resource where ResourceConstraints are specified.
//
// The resource parameter is used for resource-specific matching (e.g.,
// extracting an IC account ID to build assignment matchers). It may be nil
// when the resource is not available.
//
// For enforcement of ResourceConstraints at authorization time, use
// WithConstraints to decorate principal-bearing matchers instead.
func MatcherFromConstraints(rc *types.ResourceConstraints, resource types.ResourceWithLabels) (RoleMatcher, error) {
	if rc == nil {
		return nil, nil
	}

	switch d := rc.Details.(type) {
	case *types.ResourceConstraints_AwsConsole:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		matchers := make([]RoleMatcher, 0, len(d.AwsConsole.RoleArns))
		for _, arn := range d.AwsConsole.RoleArns {
			matchers = append(matchers, &AWSRoleARNMatcher{RoleARN: arn})
		}
		return RoleMatchers(matchers).AnyOf(), nil
	case *types.ResourceConstraints_Ssh:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		matchers := make([]RoleMatcher, 0, len(d.Ssh.Logins))
		for _, login := range d.Ssh.Logins {
			matchers = append(matchers, NewLoginMatcher(login))
		}
		return RoleMatchers(matchers).AnyOf(), nil
	case *types.ResourceConstraints_AzureApp:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		matchers := make([]RoleMatcher, 0, len(d.AzureApp.AzureIdentities))
		for _, identity := range d.AzureApp.AzureIdentities {
			matchers = append(matchers, &AzureIdentityMatcher{Identity: identity})
		}
		return RoleMatchers(matchers).AnyOf(), nil
	case *types.ResourceConstraints_GcpApp:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		matchers := make([]RoleMatcher, 0, len(d.GcpApp.GcpServiceAccounts))
		for _, account := range d.GcpApp.GcpServiceAccounts {
			matchers = append(matchers, &GCPServiceAccountMatcher{ServiceAccount: account})
		}
		return RoleMatchers(matchers).AnyOf(), nil
	default:
		return nil, trace.BadParameter("unsupported constraint details type %T", d)
	}
}
