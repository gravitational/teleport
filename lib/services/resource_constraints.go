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
	case *types.ResourceConstraints_Database:
		return buildDatabaseConstraintTransform(d)
	// TODO(kiosion): Future support for AWS Identity Center.
	// Need to decide on best way to handle; whether to continue using IdentityCenterAccountAssignments, or just Account, with PermissionSets carried in constraints.
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

	strs := getStrings()
	allowedSet := make(map[string]struct{}, len(strs))
	for _, str := range strs {
		allowedSet[str] = struct{}{}
	}

	return func(m RoleMatcher) RoleMatcher {
		principal := getPrincipal(m)
		if principal == "" {
			return m // non-principal-bearing matcher; no-op
		}
		return RoleMatcherFunc(func(role types.Role, cond types.RoleConditionType) (bool, error) {
			if _, ok := allowedSet[principal]; !ok {
				return false, nil
			}
			return m.Match(role, cond)
		})
	}
}

// buildDatabaseConstraintTransform builds a MatcherTransform for database
// constraints. Unlike single-dimension constraints (AWS ARNs, SSH logins),
// databases have multiple independent principal dimensions (users, names, roles).
// Each non-empty dimension is scoped independently: a databaseUserMatcher is
// checked against the users list, a DatabaseNameMatcher against the names list.
// If a dimension is empty in the constraint, matchers of that type pass through.
//
// Note: db_roles follow a different enforcement path than db_users/db_names.
// At connection time, db_users and db_names are checked via matchers passed to
// CheckAccess (and thus WithConstraints), but db_roles bypass CheckAccess
// entirely — they are enforced via CheckDatabaseRoles, which applies constraints
// through filterByConstrainedDatabaseRoles in access_checker.go.
func buildDatabaseConstraintTransform(d *types.ResourceConstraints_Database) MatcherTransform {
	if err := d.Validate(); err != nil {
		return func(m RoleMatcher) RoleMatcher {
			return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
				return false, trace.Wrap(err)
			})
		}
	}

	var allowedUsers map[string]struct{}
	if len(d.Database.Users) > 0 {
		allowedUsers = make(map[string]struct{}, len(d.Database.Users))
		for _, u := range d.Database.Users {
			allowedUsers[u] = struct{}{}
		}
	}

	var allowedNames map[string]struct{}
	if len(d.Database.Names) > 0 {
		allowedNames = make(map[string]struct{}, len(d.Database.Names))
		for _, n := range d.Database.Names {
			allowedNames[n] = struct{}{}
		}
	}

	return func(m RoleMatcher) RoleMatcher {
		switch lm := m.(type) {
		case *databaseUserMatcher:
			if allowedUsers == nil {
				return m
			}
			return RoleMatcherFunc(func(role types.Role, cond types.RoleConditionType) (bool, error) {
				if _, ok := allowedUsers[lm.user]; !ok {
					return false, nil
				}
				return m.Match(role, cond)
			})
		case *DatabaseNameMatcher:
			if allowedNames == nil {
				return m
			}
			return RoleMatcherFunc(func(role types.Role, cond types.RoleConditionType) (bool, error) {
				if _, ok := allowedNames[lm.Name]; !ok {
					return false, nil
				}
				return m.Match(role, cond)
			})
		default:
			return m
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
func MatcherFromConstraints(rc *types.ResourceConstraints) (RoleMatcher, error) {
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
	case *types.ResourceConstraints_Database:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		return matcherFromDatabaseConstraints(d.Database), nil
	default:
		return nil, trace.BadParameter("unsupported constraint details type %T", d)
	}
}

// matcherFromDatabaseConstraints builds a RoleMatcher that checks whether a
// role qualifies for a database with the given constraints. Each non-empty
// dimension (users, names, roles) produces an AnyOf matcher (the role must
// allow at least one of the specified values), and all dimensions are combined
// with AllOf (the role must satisfy every specified dimension).
//
// This is used during access request expansion/validation (MatcherFromConstraints),
// NOT during connection-time authorization. For db_roles specifically, this is
// the only matcher-based check — at connection time, db_roles are enforced
// separately via CheckDatabaseRoles/filterByConstrainedDatabaseRoles.
func matcherFromDatabaseConstraints(dbc *types.DatabaseResourceConstraints) RoleMatcher {
	var dimensionMatchers []RoleMatcher

	if len(dbc.Users) > 0 {
		userMatchers := make([]RoleMatcher, 0, len(dbc.Users))
		for _, user := range dbc.Users {
			userMatchers = append(userMatchers, &simpleDatabaseUserMatcher{user: user})
		}
		dimensionMatchers = append(dimensionMatchers, RoleMatchers(userMatchers).AnyOf())
	}

	if len(dbc.Names) > 0 {
		nameMatchers := make([]RoleMatcher, 0, len(dbc.Names))
		for _, name := range dbc.Names {
			nameMatchers = append(nameMatchers, &DatabaseNameMatcher{Name: name})
		}
		dimensionMatchers = append(dimensionMatchers, RoleMatchers(nameMatchers).AnyOf())
	}

	if len(dbc.Roles) > 0 {
		roleMatchers := make([]RoleMatcher, 0, len(dbc.Roles))
		for _, role := range dbc.Roles {
			roleMatchers = append(roleMatchers, &DatabaseRoleMatcher{Role: role})
		}
		dimensionMatchers = append(dimensionMatchers, RoleMatchers(roleMatchers).AnyOf())
	}

	if len(dimensionMatchers) == 0 {
		// No dimensions specified; should not happen if validation passes.
		return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
			return true, nil
		})
	}
	if len(dimensionMatchers) == 1 {
		return dimensionMatchers[0]
	}
	// All dimensions must be satisfied.
	all := RoleMatchers(dimensionMatchers)
	return RoleMatcherFunc(func(r types.Role, cond types.RoleConditionType) (bool, error) {
		return all.MatchAll(r, cond)
	})
}

// simpleDatabaseUserMatcher is a simplified version of databaseUserMatcher
// for use in constraint matching where we don't have the target Database
// object available. It checks the user against role.GetDatabaseUsers()
// without AWS IAM role ARN alternative name resolution.
type simpleDatabaseUserMatcher struct {
	user string
}

func (m *simpleDatabaseUserMatcher) Match(role types.Role, condition types.RoleConditionType) (bool, error) {
	match, _ := MatchDatabaseUser(role.GetDatabaseUsers(condition), m.user, true, false)
	return match, nil
}
