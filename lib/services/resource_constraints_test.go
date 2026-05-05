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

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func roleAllowingAWSARNs(arns ...string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.AppLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.AWSRoleARNs = append([]string{}, arns...)
	})
}

func TestWithConstraints_AWSConsole_ScopesLoginMatcher(t *testing.T) {
	const (
		adminARN    = "arn:aws:iam::123456789012:role/Admin"
		readOnlyARN = "arn:aws:iam::123456789012:role/ReadOnly"
	)

	role := roleAllowingAWSARNs(adminARN, readOnlyARN)
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AwsConsole{
			AwsConsole: &types.AWSConsoleResourceConstraints{
				RoleArns: []string{readOnlyARN},
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

func TestWithConstraints_NoOpForNonPrincipalMatchers(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AwsConsole{
			AwsConsole: &types.AWSConsoleResourceConstraints{RoleArns: []string{"x"}},
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
	// AWS console domain but missing list
	rcEmptyConsole := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AwsConsole{
			AwsConsole: &types.AWSConsoleResourceConstraints{RoleArns: nil},
		},
	}
	guard := WithConstraints(rcEmptyConsole)
	_, err := guard(NewAppAWSLoginMatcher("x")).Match(roleAllowingAWSARNs("x"), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("aws_console constraints require role_arns, none provided"))
}

func roleAllowingSSHLogins(logins ...string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.NodeLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.Logins = append([]string{}, logins...)
	})
}

func TestWithConstraints_SSH_ScopesLoginMatcher(t *testing.T) {
	const (
		rootLogin = "root"
		userLogin = "ubuntu"
	)

	role := roleAllowingSSHLogins(rootLogin, userLogin)
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Ssh{
			Ssh: &types.SSHResourceConstraints{
				Logins: []string{userLogin},
			},
		},
	}
	guard := WithConstraints(rc)

	rootMatcher := NewLoginMatcher(rootLogin)
	userMatcher := NewLoginMatcher(userLogin)
	rootMatcherScoped := guard(rootMatcher)
	userMatcherScoped := guard(userMatcher)

	ok, err := rootMatcherScoped.Match(role, types.Allow)
	require.NoError(t, err)
	require.False(t, ok, "root login should be denied by constraint scoping")

	ok, err = userMatcherScoped.Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "ubuntu login should be allowed by role and constraint")
}

func TestWithConstraints_SSH_NoOpForNonPrincipalMatchers(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Ssh{
			Ssh: &types.SSHResourceConstraints{Logins: []string{"root"}},
		},
	}
	dummy := RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
		return true, nil
	})
	wrapped := WithConstraints(rc)(dummy)
	ok, err := wrapped.Match(roleAllowingSSHLogins("other"), types.Allow)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestWithConstraints_SSH_ErrorCases(t *testing.T) {
	rcEmptySSH := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Ssh{
			Ssh: &types.SSHResourceConstraints{Logins: nil},
		},
	}
	_, err := WithConstraints(rcEmptySSH)(NewLoginMatcher("root")).Match(roleAllowingSSHLogins("root"), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("ssh constraints require logins, none provided"))
}

func TestMatcherFromConstraints_SSH_BuildsAnyOf(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Ssh{
			Ssh: &types.SSHResourceConstraints{
				Logins: []string{"root", "ubuntu"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc, nil)
	require.NoError(t, err)
	require.NotNil(t, m)

	role1 := roleAllowingSSHLogins("root")
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	role2 := roleAllowingSSHLogins("other")
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcherFromConstraints_AWSConsole_BuildsAnyOf(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AwsConsole{
			AwsConsole: &types.AWSConsoleResourceConstraints{
				RoleArns: []string{
					"arn:aws:iam::123456789012:role/ReadOnly",
					"arn:aws:iam::123456789012:role/Admin",
				},
			},
		},
	}

	m, err := MatcherFromConstraints(rc, nil)
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

func TestValidateAccessRequest_ConstraintKinds(t *testing.T) {
	makeRequest := func(kind string, constraints *types.ResourceConstraints) types.AccessRequest {
		req, err := types.NewAccessRequest(uuid.New().String(), "user", "role")
		require.NoError(t, err)
		req.SetRequestedResourceAccessIDs([]types.ResourceAccessID{
			{
				Id:          types.ResourceID{ClusterName: "cluster", Kind: kind, Name: "res"},
				Constraints: constraints,
			},
		})
		return req
	}

	sshConstraints := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Ssh{
			Ssh: &types.SSHResourceConstraints{
				Logins: []string{"root"},
			},
		},
	}
	awsConstraints := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AwsConsole{
			AwsConsole: &types.AWSConsoleResourceConstraints{
				RoleArns: []string{"arn:aws:iam::123456789012:role/Admin"},
			},
		},
	}

	t.Run("KindNode with SSH constraints is accepted", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindNode, sshConstraints))
		require.NoError(t, err)
	})

	t.Run("KindApp with AWS constraints is accepted", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindApp, awsConstraints))
		require.NoError(t, err)
	})

	t.Run("KindNode with AWS constraints is rejected", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindNode, awsConstraints))
		require.Error(t, err)
		require.Contains(t, err.Error(), "aws_console constraints are not valid for resource kind")
	})

	t.Run("KindApp with SSH constraints is rejected", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindApp, sshConstraints))
		require.Error(t, err)
		require.Contains(t, err.Error(), "ssh constraints are not valid for resource kind")
	})

	t.Run("unsupported kind with constraints is rejected", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindDatabase, sshConstraints))
		require.Error(t, err)
		require.Contains(t, err.Error(), "ssh constraints are not valid for resource kind")
	})
	t.Run("nil constraints are accepted for any kind", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindDatabase, nil))
		require.NoError(t, err)
	})

	azureConstraints := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AzureApp{
			AzureApp: &types.AzureAppResourceConstraints{
				AzureIdentities: []string{"identity1"},
			},
		},
	}
	gcpConstraints := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_GcpApp{
			GcpApp: &types.GCPAppResourceConstraints{
				GcpServiceAccounts: []string{"sa@project.iam.gserviceaccount.com"},
			},
		},
	}

	t.Run("KindApp with Azure constraints is accepted", func(t *testing.T) {
		require.NoError(t, ValidateAccessRequest(makeRequest(types.KindApp, azureConstraints)))
	})
	t.Run("KindApp with GCP constraints is accepted", func(t *testing.T) {
		require.NoError(t, ValidateAccessRequest(makeRequest(types.KindApp, gcpConstraints)))
	})

	t.Run("KindNode with Azure constraints is rejected", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindNode, azureConstraints))
		require.Error(t, err)
		require.Contains(t, err.Error(), "azure_app constraints are not valid for resource kind")
	})
	t.Run("KindNode with GCP constraints is rejected", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindNode, gcpConstraints))
		require.Error(t, err)
		require.Contains(t, err.Error(), "gcp_app constraints are not valid for resource kind")
	})
}

func roleAllowingAzureIdentities(identities ...string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.AppLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.AzureIdentities = append([]string{}, identities...)
	})
}

func TestWithConstraints_AzureApp_ScopesIdentityMatcher(t *testing.T) {
	const (
		identity1 = "/subscriptions/sub1/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1"
		identity2 = "/subscriptions/sub1/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id2"
	)

	role := roleAllowingAzureIdentities(identity1, identity2)
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AzureApp{
			AzureApp: &types.AzureAppResourceConstraints{
				AzureIdentities: []string{identity2},
			},
		},
	}
	guard := WithConstraints(rc)

	m1 := &AzureIdentityMatcher{Identity: identity1}
	m2 := &AzureIdentityMatcher{Identity: identity2}

	ok, err := guard(m1).Match(role, types.Allow)
	require.NoError(t, err)
	require.False(t, ok, "identity1 should be denied by constraint scoping")

	ok, err = guard(m2).Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "identity2 should be allowed by role and constraint")
}

func TestWithConstraints_AzureApp_NoOpForNonPrincipalMatchers(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AzureApp{
			AzureApp: &types.AzureAppResourceConstraints{AzureIdentities: []string{"x"}},
		},
	}
	dummy := RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
		return true, nil
	})
	ok, err := WithConstraints(rc)(dummy).Match(roleAllowingAzureIdentities("y"), types.Allow)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestWithConstraints_AzureApp_ErrorCases(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AzureApp{
			AzureApp: &types.AzureAppResourceConstraints{AzureIdentities: nil},
		},
	}
	_, err := WithConstraints(rc)(&AzureIdentityMatcher{Identity: "x"}).Match(roleAllowingAzureIdentities("x"), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("azure_app constraints require azure_identities, none provided"))
}

func TestMatcherFromConstraints_AzureApp_BuildsAnyOf(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_AzureApp{
			AzureApp: &types.AzureAppResourceConstraints{
				AzureIdentities: []string{"id1", "id2"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc, nil)
	require.NoError(t, err)
	require.NotNil(t, m)

	ok, err := m.Match(roleAllowingAzureIdentities("id1"), types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = m.Match(roleAllowingAzureIdentities("other"), types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}

func roleAllowingGCPServiceAccounts(accounts ...string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.AppLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.GCPServiceAccounts = append([]string{}, accounts...)
	})
}

func TestWithConstraints_GCPApp_ScopesAccountMatcher(t *testing.T) {
	const (
		account1 = "sa1@project.iam.gserviceaccount.com"
		account2 = "sa2@project.iam.gserviceaccount.com"
	)

	role := roleAllowingGCPServiceAccounts(account1, account2)
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_GcpApp{
			GcpApp: &types.GCPAppResourceConstraints{
				GcpServiceAccounts: []string{account2},
			},
		},
	}
	guard := WithConstraints(rc)

	m1 := &GCPServiceAccountMatcher{ServiceAccount: account1}
	m2 := &GCPServiceAccountMatcher{ServiceAccount: account2}

	ok, err := guard(m1).Match(role, types.Allow)
	require.NoError(t, err)
	require.False(t, ok, "account1 should be denied by constraint scoping")

	ok, err = guard(m2).Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "account2 should be allowed by role and constraint")
}

func TestWithConstraints_GCPApp_NoOpForNonPrincipalMatchers(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_GcpApp{
			GcpApp: &types.GCPAppResourceConstraints{GcpServiceAccounts: []string{"x"}},
		},
	}
	dummy := RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
		return true, nil
	})
	ok, err := WithConstraints(rc)(dummy).Match(roleAllowingGCPServiceAccounts("y"), types.Allow)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestWithConstraints_GCPApp_ErrorCases(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_GcpApp{
			GcpApp: &types.GCPAppResourceConstraints{GcpServiceAccounts: nil},
		},
	}
	_, err := WithConstraints(rc)(&GCPServiceAccountMatcher{ServiceAccount: "x"}).Match(roleAllowingGCPServiceAccounts("x"), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("gcp_app constraints require gcp_service_accounts, none provided"))
}

func TestMatcherFromConstraints_GCPApp_BuildsAnyOf(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_GcpApp{
			GcpApp: &types.GCPAppResourceConstraints{
				GcpServiceAccounts: []string{"sa1@project.iam.gserviceaccount.com", "sa2@project.iam.gserviceaccount.com"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc, nil)
	require.NoError(t, err)
	require.NotNil(t, m)

	ok, err := m.Match(roleAllowingGCPServiceAccounts("sa1@project.iam.gserviceaccount.com"), types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = m.Match(roleAllowingGCPServiceAccounts("other@project.iam.gserviceaccount.com"), types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}
