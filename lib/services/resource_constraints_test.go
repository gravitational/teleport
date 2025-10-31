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
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func roleAllowingAWSARNs(arns ...string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.AppLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.AWSRoleARNs = append([]string{}, arns...)
	})
}

func roleAllowingIC(assignments ...types.IdentityCenterAccountAssignment) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.AppLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.SetIdentityCenterAccountAssignments(types.Allow, assignments)
	})
}

func TestWithConstraints_AWSConsole_ScopesLoginMatcher(t *testing.T) {
	const (
		adminARN    = "arn:aws:iam::123456789012:role/Admin"
		readOnlyARN = "arn:aws:iam::123456789012:role/ReadOnly"
	)

	role := roleAllowingAWSARNs(adminARN, readOnlyARN)
	rc := &types.ResourceConstraints{
		Domain: types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_CONSOLE,
		Details: &types.ResourceConstraints_AWSConsole{
			AWSConsole: &types.AWSConsoleResourceConstraints{
				RoleARNs: []string{readOnlyARN},
			},
		},
	}
	guard := WithConstraints(rc)

	adminMatcher := NewAppAWSLoginMatcher(adminARN)
	roMatcher := NewAppAWSLoginMatcher(readOnlyARN)
	adminMatcherScoped := guard(adminMatcher)
	roMatcherScoped := guard(roMatcher)

	ok, err := adminMatcherScoped.Match(role, types.Allow)
	require.NoError(t, err)
	require.False(t, ok, "admin arn should be denied by constraint scoping")

	ok, err = roMatcherScoped.Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "read-only arn should be allowed by role and constraint")
}

func TestWithConstraints_AWSIC_ScopesAssignmentMatcher(t *testing.T) {
	icReadOnlyAssignment := types.IdentityCenterAccountAssignment{
		Account:       "123456789012",
		PermissionSet: "arn:aws:iam::aws:policy/ReadOnlyAccess",
	}
	icAdminAssignment := types.IdentityCenterAccountAssignment{
		Account:       "123456789012",
		PermissionSet: "arn:aws:iam::aws:policy/Admin",
	}

	role := roleAllowingIC(icReadOnlyAssignment, icAdminAssignment)
	rc := &types.ResourceConstraints{
		Domain: types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_IDENTITY_CENTER,
		Details: &types.ResourceConstraints_AWSIC{
			AWSIC: &types.AWSIdentityCenterResourceConstraints{
				AccountAssignments: []types.IdentityCenterAccountAssignment{icReadOnlyAssignment},
			},
		},
	}

	guard := WithConstraints(rc)

	roMatcher := &IdentityCenterAccountAssignmentMatcher{
		accountID:        icReadOnlyAssignment.Account,
		permissionSetARN: icReadOnlyAssignment.PermissionSet,
	}
	adminMatcher := &IdentityCenterAccountAssignmentMatcher{
		accountID:        icAdminAssignment.Account,
		permissionSetARN: icAdminAssignment.PermissionSet,
	}
	roMatcherScoped := guard(roMatcher)
	adminMatcherScoped := guard(adminMatcher)

	ok, err := adminMatcher.Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "admin assignment should be allowed by role")

	ok, err = adminMatcherScoped.Match(role, types.Allow)
	require.NoError(t, err)
	require.False(t, ok, "admin assignment should be denied by constraint scoping")

	ok, err = roMatcherScoped.Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "read-only assignment should be allowed by role and constraint")
}

func TestWithConstraints_NoOpForNonPrincipalMatchers(t *testing.T) {
	rc := &types.ResourceConstraints{
		Domain: types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_CONSOLE,
		Details: &types.ResourceConstraints_AWSConsole{
			AWSConsole: &types.AWSConsoleResourceConstraints{RoleARNs: []string{"x"}},
		},
	}
	guard := WithConstraints(rc)

	dummy := RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
		return true, nil
	})

	wrapped := guard(dummy)
	ok, err := wrapped.Match(roleAllowingAWSARNs("y"), types.Allow)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestWithConstraints_ErrorCases(t *testing.T) {
	// Unsupported domain
	rcBad := &types.ResourceConstraints{
		Domain: 9999,
	}
	guard := WithConstraints(rcBad)
	_, err := guard(NewAppAWSLoginMatcher("x")).Match(roleAllowingAWSARNs("x"), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("unsupported constraint domain %q", rcBad.Domain))

	// AWS console domain but missing list
	rcEmptyConsole := &types.ResourceConstraints{
		Domain: types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_CONSOLE,
		Details: &types.ResourceConstraints_AWSConsole{
			AWSConsole: &types.AWSConsoleResourceConstraints{RoleARNs: nil},
		},
	}
	guard = WithConstraints(rcEmptyConsole)
	_, err = guard(NewAppAWSLoginMatcher("x")).Match(roleAllowingAWSARNs("x"), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("aws_console constraints require role_arns, none provided"))

	// AWS IC domain but missing list
	rcEmptyIC := &types.ResourceConstraints{
		Domain: types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_IDENTITY_CENTER,
		Details: &types.ResourceConstraints_AWSIC{
			AWSIC: &types.AWSIdentityCenterResourceConstraints{AccountAssignments: nil},
		},
	}
	guard = WithConstraints(rcEmptyIC)
	_, err = guard(&IdentityCenterAccountAssignmentMatcher{
		accountID:        "a",
		permissionSetARN: "p",
	}).Match(roleAllowingIC(), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("aws_ic constraints require account_assignments, none provided"))
}

func TestMatcherFromConstraints_AWSConsole_BuildsAnyOf(t *testing.T) {
	rid := types.ResourceID{
		ClusterName: "c",
		Kind:        types.KindApp,
		Name:        "aws-console",
		Constraints: &types.ResourceConstraints{
			Domain: types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_CONSOLE,
			Details: &types.ResourceConstraints_AWSConsole{
				AWSConsole: &types.AWSConsoleResourceConstraints{
					RoleARNs: []string{
						"arn:aws:iam::123456789012:role/ReadOnly",
						"arn:aws:iam::123456789012:role/Admin",
					},
				},
			},
		},
	}

	m, err := MatcherFromConstraints(rid)
	require.NoError(t, err)
	require.NotNil(t, m)

	role1 := roleAllowingAWSARNs("arn:aws:iam::123456789012:role/ReadOnly")
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	role2 := roleAllowingAWSARNs("arn:aws:iam::123456789012:role/Other")
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcherFromConstraints_AWSIC_BuildsAnyOf(t *testing.T) {
	rid := types.ResourceID{
		ClusterName: "c",
		Kind:        types.KindIdentityCenterAccount,
		Name:        "aws-dev",
		Constraints: &types.ResourceConstraints{
			Domain: types.ResourceConstraintDomain_CONSTRAINT_DOMAIN_AWS_IDENTITY_CENTER,
			Details: &types.ResourceConstraints_AWSIC{
				AWSIC: &types.AWSIdentityCenterResourceConstraints{
					AccountAssignments: []types.IdentityCenterAccountAssignment{
						{Account: "111111111111", PermissionSet: "arn:aws:iam::aws:policy/ReadOnlyAccess"},
						{Account: "222222222222", PermissionSet: "arn:aws:iam::aws:policy/Billing"},
					},
				},
			},
		},
	}

	m, err := MatcherFromConstraints(rid)
	require.NoError(t, err)
	require.NotNil(t, m)

	role1 := roleAllowingIC(
		types.IdentityCenterAccountAssignment{Account: "111111111111", PermissionSet: "arn:aws:iam::aws:policy/ReadOnlyAccess"},
	)
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	role2 := roleAllowingIC(
		types.IdentityCenterAccountAssignment{Account: "111111111111", PermissionSet: "arn:aws:iam::aws:policy/Other"},
	)
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}
