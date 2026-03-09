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

	m, err := MatcherFromConstraints(rc)
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

	m, err := MatcherFromConstraints(rc)
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

func roleAllowingDatabaseUsersAndNames(users, names []string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.DatabaseLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.DatabaseUsers = append([]string{}, users...)
		rv.Spec.Allow.DatabaseNames = append([]string{}, names...)
	})
}

func roleAllowingDatabaseRoles(roles []string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.DatabaseLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.DatabaseRoles = append([]string{}, roles...)
		rv.Spec.Options.CreateDatabaseUserMode = types.CreateDatabaseUserMode_DB_USER_MODE_KEEP
	})
}

func roleAllowingDatabaseAll(users, names, roles []string) types.Role {
	return newRole(func(rv *types.RoleV6) {
		rv.Spec.Allow.DatabaseLabels = types.Labels{types.Wildcard: {types.Wildcard}}
		rv.Spec.Allow.Namespaces = []string{types.Wildcard}
		rv.Spec.Allow.DatabaseUsers = append([]string{}, users...)
		rv.Spec.Allow.DatabaseNames = append([]string{}, names...)
		rv.Spec.Allow.DatabaseRoles = append([]string{}, roles...)
		rv.Spec.Options.CreateDatabaseUserMode = types.CreateDatabaseUserMode_DB_USER_MODE_KEEP
	})
}

func TestWithConstraints_Database_ScopesUserMatcher(t *testing.T) {
	role := roleAllowingDatabaseUsersAndNames(
		[]string{"admin", "readonly"},
		[]string{"production", "staging"},
	)
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"readonly"},
			},
		},
	}
	guard := WithConstraints(rc)

	db, err := types.NewDatabaseV3(types.Metadata{Name: "test-db"}, types.DatabaseSpecV3{
		Protocol: "postgres", URI: "localhost:5432",
	})
	require.NoError(t, err)

	adminMatcher := NewDatabaseUserMatcher(db, "admin")
	roMatcher := NewDatabaseUserMatcher(db, "readonly")

	ok, err := guard(adminMatcher).Match(role, types.Allow)
	require.NoError(t, err)
	require.False(t, ok, "admin user should be denied by constraint scoping")

	ok, err = guard(roMatcher).Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "readonly user should be allowed by role and constraint")
}

func TestWithConstraints_Database_ScopesNameMatcher(t *testing.T) {
	role := roleAllowingDatabaseUsersAndNames(
		[]string{"admin"},
		[]string{"production", "staging"},
	)
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Names: []string{"staging"},
			},
		},
	}
	guard := WithConstraints(rc)

	prodMatcher := &DatabaseNameMatcher{Name: "production"}
	stagingMatcher := &DatabaseNameMatcher{Name: "staging"}

	ok, err := guard(prodMatcher).Match(role, types.Allow)
	require.NoError(t, err)
	require.False(t, ok, "production should be denied by constraint scoping")

	ok, err = guard(stagingMatcher).Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "staging should be allowed by role and constraint")
}

func TestWithConstraints_Database_NoOpForUnspecifiedDimension(t *testing.T) {
	role := roleAllowingDatabaseUsersAndNames(
		[]string{"admin"},
		[]string{"production", "staging"},
	)
	// Only constrain users, not names.
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"admin"},
			},
		},
	}
	guard := WithConstraints(rc)

	// Name matchers should pass through since names is not constrained.
	prodMatcher := &DatabaseNameMatcher{Name: "production"}
	ok, err := guard(prodMatcher).Match(role, types.Allow)
	require.NoError(t, err)
	require.True(t, ok, "name matcher should pass through when names not constrained")
}

func TestWithConstraints_Database_NoOpForNonPrincipalMatchers(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"admin"},
			},
		},
	}
	dummy := RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
		return true, nil
	})
	wrapped := WithConstraints(rc)(dummy)
	ok, err := wrapped.Match(roleAllowingDatabaseUsersAndNames([]string{"other"}, nil), types.Allow)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestWithConstraints_Database_ErrorCases(t *testing.T) {
	rcEmpty := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{},
		},
	}
	_, err := WithConstraints(rcEmpty)(&simpleDatabaseUserMatcher{user: "x"}).
		Match(roleAllowingDatabaseUsersAndNames([]string{"x"}, nil), types.Allow)
	require.ErrorIs(t, err, trace.BadParameter("database constraints require at least one of users, names, or roles"))
}

func TestMatcherFromConstraints_Database_UsersOnly(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"admin", "readonly"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc)
	require.NoError(t, err)
	require.NotNil(t, m)

	role1 := roleAllowingDatabaseUsersAndNames([]string{"admin"}, nil)
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	role2 := roleAllowingDatabaseUsersAndNames([]string{"other"}, nil)
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcherFromConstraints_Database_NamesOnly(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Names: []string{"production"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc)
	require.NoError(t, err)
	require.NotNil(t, m)

	role1 := roleAllowingDatabaseUsersAndNames(nil, []string{"production"})
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	role2 := roleAllowingDatabaseUsersAndNames(nil, []string{"staging"})
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcherFromConstraints_Database_UsersAndNames(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"admin"},
				Names: []string{"production"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc)
	require.NoError(t, err)
	require.NotNil(t, m)

	// Role allows both dimensions.
	role1 := roleAllowingDatabaseUsersAndNames([]string{"admin"}, []string{"production"})
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	// Role allows user but not name.
	role2 := roleAllowingDatabaseUsersAndNames([]string{"admin"}, []string{"staging"})
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)

	// Role allows name but not user.
	role3 := roleAllowingDatabaseUsersAndNames([]string{"other"}, []string{"production"})
	ok, err = m.Match(role3, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcherFromConstraints_Database_RolesOnly(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Roles: []string{"reader"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc)
	require.NoError(t, err)
	require.NotNil(t, m)

	// Role that allows the constrained db role should match.
	role1 := roleAllowingDatabaseRoles([]string{"reader", "writer"})
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	// Role that does NOT allow the constrained db role should not match.
	role2 := roleAllowingDatabaseRoles([]string{"admin"})
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestMatcherFromConstraints_Database_UsersNamesAndRoles(t *testing.T) {
	rc := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"admin"},
				Names: []string{"production"},
				Roles: []string{"reader"},
			},
		},
	}

	m, err := MatcherFromConstraints(rc)
	require.NoError(t, err)
	require.NotNil(t, m)

	// Role allows all three dimensions.
	role1 := roleAllowingDatabaseAll([]string{"admin"}, []string{"production"}, []string{"reader"})
	ok, err := m.Match(role1, types.Allow)
	require.NoError(t, err)
	require.True(t, ok)

	// Role allows user and name but not role.
	role2 := roleAllowingDatabaseAll([]string{"admin"}, []string{"production"}, []string{"writer"})
	ok, err = m.Match(role2, types.Allow)
	require.NoError(t, err)
	require.False(t, ok)

	// Role allows user and role but not name.
	role3 := roleAllowingDatabaseAll([]string{"admin"}, []string{"staging"}, []string{"reader"})
	ok, err = m.Match(role3, types.Allow)
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
	dbConstraints := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"admin"},
				Names: []string{"production"},
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

	t.Run("KindDatabase with database constraints is accepted", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindDatabase, dbConstraints))
		require.NoError(t, err)
	})

	t.Run("KindDatabase with SSH constraints is rejected", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindDatabase, sshConstraints))
		require.Error(t, err)
		require.Contains(t, err.Error(), "ssh constraints are not valid for resource kind")
	})

	t.Run("KindNode with database constraints is rejected", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindNode, dbConstraints))
		require.Error(t, err)
		require.Contains(t, err.Error(), "database constraints are not valid for resource kind")
	})

	t.Run("nil constraints are accepted for any kind", func(t *testing.T) {
		err := ValidateAccessRequest(makeRequest(types.KindDatabase, nil))
		require.NoError(t, err)
	})
}

func TestCheckDatabaseRoles_WithConstraints(t *testing.T) {
	// Role that allows auto-user provisioning with several database roles.
	role := &types.RoleV6{
		Metadata: types.Metadata{Name: "db-role", Namespace: "default"},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseRoles:  []string{"reader", "writer", "admin"},
			},
		},
	}

	database, err := types.NewDatabaseV3(types.Metadata{
		Name:   "prod-postgres",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	t.Run("no constraints returns all allowed roles", func(t *testing.T) {
		accessInfo := &AccessInfo{
			Username: "alice",
			Roles:    []string{"db-role"},
		}
		checker := NewAccessCheckerWithRoleSet(accessInfo, "cluster", RoleSet{role})

		roles, err := checker.CheckDatabaseRoles(database, nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader", "writer", "admin"}, roles)
	})

	t.Run("constraints filter returned roles", func(t *testing.T) {
		accessInfo := &AccessInfo{
			Username: "alice",
			Roles:    []string{"db-role"},
			AllowedResourceAccessIDs: []types.ResourceAccessID{
				{
					Id: types.ResourceID{
						ClusterName: "cluster",
						Kind:        types.KindDatabase,
						Name:        "prod-postgres",
					},
					Constraints: &types.ResourceConstraints{
						Details: &types.ResourceConstraints_Database{
							Database: &types.DatabaseResourceConstraints{
								Roles: []string{"reader"},
							},
						},
					},
				},
			},
		}
		checker := NewAccessCheckerWithRoleSet(accessInfo, "cluster", RoleSet{role})

		roles, err := checker.CheckDatabaseRoles(database, nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader"}, roles)
	})

	t.Run("constraints filter user-requested roles", func(t *testing.T) {
		accessInfo := &AccessInfo{
			Username: "alice",
			Roles:    []string{"db-role"},
			AllowedResourceAccessIDs: []types.ResourceAccessID{
				{
					Id: types.ResourceID{
						ClusterName: "cluster",
						Kind:        types.KindDatabase,
						Name:        "prod-postgres",
					},
					Constraints: &types.ResourceConstraints{
						Details: &types.ResourceConstraints_Database{
							Database: &types.DatabaseResourceConstraints{
								Roles: []string{"reader", "writer"},
							},
						},
					},
				},
			},
		}
		checker := NewAccessCheckerWithRoleSet(accessInfo, "cluster", RoleSet{role})

		// Request reader and writer - both allowed by constraint.
		roles, err := checker.CheckDatabaseRoles(database, []string{"reader", "writer"})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader", "writer"}, roles)
	})

	t.Run("user-requested role not in constraints is denied by role check first", func(t *testing.T) {
		accessInfo := &AccessInfo{
			Username: "alice",
			Roles:    []string{"db-role"},
			AllowedResourceAccessIDs: []types.ResourceAccessID{
				{
					Id: types.ResourceID{
						ClusterName: "cluster",
						Kind:        types.KindDatabase,
						Name:        "prod-postgres",
					},
					Constraints: &types.ResourceConstraints{
						Details: &types.ResourceConstraints_Database{
							Database: &types.DatabaseResourceConstraints{
								Roles: []string{"reader"},
							},
						},
					},
				},
			},
		}
		checker := NewAccessCheckerWithRoleSet(accessInfo, "cluster", RoleSet{role})

		// Request "superadmin" which isn't in the role's allowed roles at all.
		_, err := checker.CheckDatabaseRoles(database, []string{"superadmin"})
		require.Error(t, err)
	})

	t.Run("constraints with no roles do not filter", func(t *testing.T) {
		accessInfo := &AccessInfo{
			Username: "alice",
			Roles:    []string{"db-role"},
			AllowedResourceAccessIDs: []types.ResourceAccessID{
				{
					Id: types.ResourceID{
						ClusterName: "cluster",
						Kind:        types.KindDatabase,
						Name:        "prod-postgres",
					},
					Constraints: &types.ResourceConstraints{
						Details: &types.ResourceConstraints_Database{
							Database: &types.DatabaseResourceConstraints{
								Users: []string{"admin"},
							},
						},
					},
				},
			},
		}
		checker := NewAccessCheckerWithRoleSet(accessInfo, "cluster", RoleSet{role})

		// No role constraints, so all roles should be returned.
		roles, err := checker.CheckDatabaseRoles(database, nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader", "writer", "admin"}, roles)
	})

	t.Run("constraints for different database do not affect", func(t *testing.T) {
		accessInfo := &AccessInfo{
			Username: "alice",
			Roles:    []string{"db-role"},
			AllowedResourceAccessIDs: []types.ResourceAccessID{
				{
					Id: types.ResourceID{
						ClusterName: "cluster",
						Kind:        types.KindDatabase,
						Name:        "other-database",
					},
					Constraints: &types.ResourceConstraints{
						Details: &types.ResourceConstraints_Database{
							Database: &types.DatabaseResourceConstraints{
								Roles: []string{"reader"},
							},
						},
					},
				},
			},
		}
		checker := NewAccessCheckerWithRoleSet(accessInfo, "cluster", RoleSet{role})

		// Constraints target "other-database", not "prod-postgres", so all roles returned.
		roles, err := checker.CheckDatabaseRoles(database, nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader", "writer", "admin"}, roles)
	})
}

// TestDatabaseConstraints_EndToEnd simulates a user who has assumed an access
// request with database constraints, then tests all three enforcement paths:
//   - db_users: enforced via databaseUserMatcher through CheckAccess → WithConstraints
//   - db_names: enforced via DatabaseNameMatcher through CheckAccess → WithConstraints
//   - db_roles: enforced via CheckDatabaseRoles → filterByConstrainedDatabaseRoles
func TestDatabaseConstraints_EndToEnd(t *testing.T) {
	// Role allows broad access: multiple users, names, and roles.
	role := &types.RoleV6{
		Metadata: types.Metadata{Name: "db-all", Namespace: "default"},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				Namespaces:     []string{"default"},
				DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				DatabaseUsers:  []string{"admin", "readonly", "analyst"},
				DatabaseNames:  []string{"production", "staging", "analytics"},
				DatabaseRoles:  []string{"reader", "writer", "admin"},
			},
		},
	}

	database, err := types.NewDatabaseV3(types.Metadata{
		Name:   "prod-postgres",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	constraints := &types.ResourceConstraints{
		Details: &types.ResourceConstraints_Database{
			Database: &types.DatabaseResourceConstraints{
				Users: []string{"readonly"},
				Names: []string{"analytics"},
				Roles: []string{"reader"},
			},
		},
	}

	accessInfo := &AccessInfo{
		Username: "alice",
		Roles:    []string{"db-all"},
		AllowedResourceAccessIDs: []types.ResourceAccessID{
			{
				Id: types.ResourceID{
					ClusterName: "cluster",
					Kind:        types.KindDatabase,
					Name:        "prod-postgres",
				},
				Constraints: constraints,
			},
		},
	}
	checker := NewAccessCheckerWithRoleSet(accessInfo, "cluster", RoleSet{role})

	state := AccessState{MFAVerified: true}

	// -- db_users enforcement via CheckAccess + WithConstraints --

	t.Run("db_user in constraints is allowed", func(t *testing.T) {
		err := checker.CheckAccess(database, state, NewDatabaseUserMatcher(database, "readonly"))
		require.NoError(t, err)
	})

	t.Run("db_user NOT in constraints is denied", func(t *testing.T) {
		err := checker.CheckAccess(database, state, NewDatabaseUserMatcher(database, "admin"))
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
	})

	// -- db_names enforcement via CheckAccess + WithConstraints --

	t.Run("db_name in constraints is allowed", func(t *testing.T) {
		err := checker.CheckAccess(database, state, &DatabaseNameMatcher{Name: "analytics"})
		require.NoError(t, err)
	})

	t.Run("db_name NOT in constraints is denied", func(t *testing.T) {
		err := checker.CheckAccess(database, state, &DatabaseNameMatcher{Name: "production"})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
	})

	// -- db_roles enforcement via CheckDatabaseRoles --

	t.Run("db_roles filtered by constraints", func(t *testing.T) {
		roles, err := checker.CheckDatabaseRoles(database, nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader"}, roles,
			"only 'reader' should be returned; 'writer' and 'admin' should be filtered by constraint")
	})

	t.Run("user-requested db_role in constraints is allowed", func(t *testing.T) {
		roles, err := checker.CheckDatabaseRoles(database, []string{"reader"})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader"}, roles)
	})

	t.Run("user-requested db_role allowed by role but NOT in constraints is filtered", func(t *testing.T) {
		roles, err := checker.CheckDatabaseRoles(database, []string{"reader", "writer"})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader"}, roles,
			"'writer' is allowed by the role but not in constraints, should be filtered out")
	})

	// -- Without constraints (no AllowedResourceAccessIDs), everything is allowed --

	unconstrained := NewAccessCheckerWithRoleSet(&AccessInfo{
		Username: "alice",
		Roles:    []string{"db-all"},
	}, "cluster", RoleSet{role})

	t.Run("unconstrained db_user is allowed", func(t *testing.T) {
		err := unconstrained.CheckAccess(database, state, NewDatabaseUserMatcher(database, "admin"))
		require.NoError(t, err)
	})

	t.Run("unconstrained db_name is allowed", func(t *testing.T) {
		err := unconstrained.CheckAccess(database, state, &DatabaseNameMatcher{Name: "production"})
		require.NoError(t, err)
	})

	t.Run("unconstrained db_roles returns all", func(t *testing.T) {
		roles, err := unconstrained.CheckDatabaseRoles(database, nil)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"reader", "writer", "admin"}, roles)
	})
}
