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
	// TODO(kiosion): Future support for AWS Identity Center.
	// Need to decide on best way to handle; whether to continue using IdentityCenterAccountAssignments, or just Account, with PermissionSets carried in constraints.
	default:
		return func(m RoleMatcher) RoleMatcher {
			return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
				return false, trace.BadParameter("unrecognized constraint type %T. Request the resource without constraints", d)
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

// BuildResourceConstraintMatchers returns constraint matchers for the resource's kind and name.
// If localCluster is nonempty, only IDs with that cluster name match.
// An empty localCluster retains all matches to avoid dropping constraints.
//
// TODO(kiosion): When constraints extend for Kubernetes support, kube sub-resource
// IDs need name-only correlation against the kube_cluster resource, like
// GetKubeResourcesFromResourceIDs
func BuildResourceConstraintMatchers(resourceAccessIDs []types.ResourceAccessID, resource types.Resource, localCluster string) ([]RoleMatcher, error) {
	var matchers []RoleMatcher
	for _, raid := range resourceAccessIDs {
		rid := raid.GetResourceID()
		if rid.Name != resource.GetName() || rid.Kind != resource.GetKind() {
			continue
		}
		if localCluster != "" && rid.ClusterName != localCluster {
			continue
		}
		rm, err := MatcherFromConstraints(raid.GetConstraints())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if rm != nil {
			matchers = append(matchers, rm)
		}
	}
	return matchers, nil
}

type constraintDimension struct {
	key     string
	values  []string
	matcher func(value string) RoleMatcher
}

// constraintDimensions returns constraint values and matchers grouped by principal type.
func constraintDimensions(rc *types.ResourceConstraints) ([]constraintDimension, error) {
	switch d := rc.Details.(type) {
	case *types.ResourceConstraints_AwsConsole:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		return []constraintDimension{{
			key:     types.PrincipalTypeRoleARNs,
			values:  d.AwsConsole.RoleArns,
			matcher: func(value string) RoleMatcher { return &AWSRoleARNMatcher{RoleARN: value} },
		}}, nil
	case *types.ResourceConstraints_Ssh:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		return []constraintDimension{{
			key:     types.PrincipalTypeLogins,
			values:  d.Ssh.Logins,
			matcher: NewLoginMatcher,
		}}, nil
	default:
		return nil, trace.BadParameter("unrecognized constraint type %T. Request the resource without constraints", d)
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
func MatcherFromConstraints(rc *types.ResourceConstraints) (RoleMatcher, error) {
	if rc == nil {
		return nil, nil
	}

	dimensions, err := constraintDimensions(rc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var matchers []RoleMatcher
	for _, dimension := range dimensions {
		for _, value := range dimension.values {
			matchers = append(matchers, dimension.matcher(value))
		}
	}
	return RoleMatchers(matchers).AnyOf(), nil
}

// CheckResourceAccessWithConstraints checks access to the resource and each principal requested for it.
func CheckResourceAccessWithConstraints(checker AccessChecker, resource types.ResourceWithLabels, state AccessState, resourceAccessIDs []types.ResourceAccessID, localCluster string) error {
	if err := checker.CheckAccess(resource, state); err != nil {
		return trace.Wrap(err)
	}
	for _, raid := range resourceAccessIDs {
		rid := raid.GetResourceID()
		if rid.Name != resource.GetName() || rid.Kind != resource.GetKind() {
			continue
		}
		if localCluster != "" && rid.ClusterName != localCluster {
			continue
		}
		if raid.GetConstraints() == nil {
			continue
		}
		dimensions, err := constraintDimensions(raid.GetConstraints())
		if err != nil {
			return trace.Wrap(err)
		}
		// Separate checks allow different roles to grant different principals.
		for _, dimension := range dimensions {
			for _, value := range dimension.values {
				if err := checker.CheckAccess(resource, state, dimension.matcher(value)); err != nil {
					return trace.Wrap(err)
				}
			}
		}
	}
	return nil
}
