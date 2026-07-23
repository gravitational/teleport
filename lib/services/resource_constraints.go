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
	"strings"

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

// BuildResourceConstraintMatchers returns RoleMatchers derived from any
// ResourceConstraints requested for the given resource, correlating the
// resource against resourceAccessIDs by kind and name. Entries without
// constraints contribute no matchers, so resource kinds that cannot carry
// constraints are unaffected.
//
// Correlating by kind and name mirrors how requested resources are looked up
// from their IDs (see [accessrequest.GetResourcesByResourceIDs]); callers are
// expected to pass resources and resourceAccessIDs scoped to the same cluster.
//
// TODO(kiosion): When constraints extend for Kubernetes support, kube sub-resource
// IDs need name-only correlation against the kube_cluster resource, like
// getKubeResourcesFromResourceIDs
func BuildResourceConstraintMatchers(resourceAccessIDs []types.ResourceAccessID, resource types.ResourceWithLabels) ([]RoleMatcher, error) {
	var matchers []RoleMatcher
	for _, raid := range resourceAccessIDs {
		rid := raid.GetResourceID()
		if rid.Name != resource.GetName() || rid.Kind != resource.GetKind() {
			continue
		}
		rm, err := MatcherFromConstraints(raid.GetConstraints(), resource)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if rm != nil {
			matchers = append(matchers, rm)
		}
	}
	return matchers, nil
}

// buildDatabaseConstraintTransform builds a MatcherTransform for database
// constraints. Each non-empty dimension is scoped independently: a
// databaseUserMatcher is checked against the users list, a DatabaseNameMatcher
// against the names list. If a dimension is empty, matchers of that type pass
// through. db_roles are not handled here — they bypass CheckAccess and are
// enforced via filterByConstrainedDatabaseRoles in access_checker.go.
func buildDatabaseConstraintTransform(d *types.ResourceConstraints_Database) MatcherTransform {
	if err := d.Validate(); err != nil {
		return func(m RoleMatcher) RoleMatcher {
			return RoleMatcherFunc(func(_ types.Role, _ types.RoleConditionType) (bool, error) {
				return false, trace.Wrap(err)
			})
		}
	}

	var allowedUsers map[string]struct{}
	if len(d.Database.Users) > 0 && !slices.Contains(d.Database.Users, types.Wildcard) {
		allowedUsers = set.New(d.Database.Users...)
	}

	var allowedNames map[string]struct{}
	if len(d.Database.Names) > 0 && !slices.Contains(d.Database.Names, types.Wildcard) {
		allowedNames = set.New(d.Database.Names...)
	}

	return func(m RoleMatcher) RoleMatcher {
		switch lm := m.(type) {
		case *databaseUserMatcher:
			if allowedUsers == nil {
				return m
			}
			return RoleMatcherFunc(func(role types.Role, cond types.RoleConditionType) (bool, error) {
				if !matchesAllowedUsers(allowedUsers, lm.user, lm.alternativeNames, lm.caseInsensitive) {
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

// matchesAllowedUsers checks whether the primary user or any of its alternative
// names appear in the allowed set, optionally case-insensitive.
func matchesAllowedUsers(allowed map[string]struct{}, user string, alternativeNames []string, caseInsensitive bool) bool {
	if containsUser(allowed, user, caseInsensitive) {
		return true
	}
	for _, name := range alternativeNames {
		if containsUser(allowed, name, caseInsensitive) {
			return true
		}
	}
	return false
}

func containsUser(allowed map[string]struct{}, name string, caseInsensitive bool) bool {
	if !caseInsensitive {
		_, ok := allowed[name]
		return ok
	}
	for k := range allowed {
		if strings.EqualFold(k, name) {
			return true
		}
	}
	return false
}

// MatcherFromConstraints constructs a RoleMatcher encoding the requested
// ResourceConstraints for role resolution/validation time.
//
// This matcher is intended for use in request expansion, to decide whether a
// role qualifies for a resource where ResourceConstraints are specified.
//
// For enforcement of ResourceConstraints at authorization time, use
// WithConstraints to decorate principal-bearing matchers instead.
//
// The resource parameter is used for resource-specific matching (e.g., AWS IAM
// role ARN alternative name resolution for database user constraints). It may
// be nil when the resource is not available.
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
	case *types.ResourceConstraints_Database:
		if err := d.Validate(); err != nil {
			return nil, trace.Wrap(err)
		}
		var db types.Database
		if resource != nil {
			db, _ = resource.(types.Database)
		}
		return matcherFromDatabaseConstraints(d.Database, db), nil
	default:
		return nil, trace.BadParameter("unsupported constraint details type %T", d)
	}
}

// matcherFromDatabaseConstraints builds a RoleMatcher for request
// expansion/validation. Each non-empty dimension (users, names, roles)
// produces an AnyOf matcher, and all dimensions are combined with AllOf.
// When db is non-nil, full AWS IAM role ARN alternative name resolution
// is used for database user matching. A wildcard constraint value flows
// through like any other: it is matched only by a role granting the
// wildcard for that dimension, so a wildcard request requires such a role
// to qualify and retains it through resolution.
func matcherFromDatabaseConstraints(dbc *types.DatabaseResourceConstraints, db types.Database) RoleMatcher {
	var dimensionMatchers []RoleMatcher

	if len(dbc.Users) > 0 {
		userMatchers := make([]RoleMatcher, 0, len(dbc.Users))
		for _, user := range dbc.Users {
			if db != nil {
				userMatchers = append(userMatchers, NewDatabaseUserMatcher(db, user))
			} else {
				userMatchers = append(userMatchers, &simpleDatabaseUserMatcher{user: user})
			}
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
		return nil
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

// DatabasePrincipalSets computes a database's principal dimensions (db_users,
// db_names, db_roles), each split into granted and requestable values and
// attributed to the roles granting them, one entry per role. Attribution is
// per role because a database (user, name) pair is only usable when a single
// role grants both; consumers join dimensions on the role name to compose
// valid combinations.
//
// base is the caller's held role set. extended, when non-nil, additionally
// includes search_as_roles; values granted only by roles outside the held set
// are requestable. Roles that grant no principals for this database are
// omitted.
func DatabasePrincipalSets(base, extended AccessChecker, database types.Database, localCluster string) []types.ResourcePrincipalSet {
	checker := extended
	if checker == nil {
		checker = base
	}

	held := make(map[string]struct{})
	for _, name := range base.RoleNames() {
		held[name] = struct{}{}
	}

	type dimension struct {
		kind        string
		granted     map[string]struct{}
		requestable map[string]struct{}
		byRole      []types.RolePrincipalValues
	}
	newDimension := func(kind string) *dimension {
		return &dimension{
			kind:        kind,
			granted:     make(map[string]struct{}),
			requestable: make(map[string]struct{}),
		}
	}
	dims := []*dimension{
		newDimension(types.PrincipalKindDBUsers),
		newDimension(types.PrincipalKindDBNames),
		newDimension(types.PrincipalKindDBRoles),
	}

	info := &AccessInfo{
		Username: checker.AccessInfo().Username,
		Traits:   checker.Traits(),
	}
	for _, role := range checker.Roles() {
		singleChecker := NewAccessCheckerWithRoleSet(info, localCluster, RoleSet{role})

		var users, names, dbRoles []string
		if res, err := singleChecker.EnumerateDatabaseUsers(database); err == nil {
			users, _ = res.ToEntities()
		}
		namesResult := singleChecker.EnumerateDatabaseNames(database)
		names, _ = namesResult.ToEntities()
		if r, err := singleChecker.CheckDatabaseRoles(database, nil); err == nil {
			// db_roles have no wildcard matching: a role field carrying a
			// literal "*" keeps its meaning for unconstrained access only
			// and stays unreachable through constraints, so it is not
			// enumerated as a selectable principal.
			dbRoles = slices.DeleteFunc(r, func(v string) bool { return v == types.Wildcard })
		}

		_, isHeld := held[role.GetName()]
		for i, values := range [][]string{users, names, dbRoles} {
			if len(values) == 0 {
				continue
			}
			values = slices.Clone(values)
			slices.Sort(values)
			dims[i].byRole = append(dims[i].byRole, types.RolePrincipalValues{
				Role:            role.GetName(),
				RequiresRequest: !isHeld,
				Values:          values,
			})
			target := dims[i].requestable
			if isHeld {
				target = dims[i].granted
			}
			for _, v := range values {
				target[v] = struct{}{}
			}
		}
	}

	var out []types.ResourcePrincipalSet
	for _, d := range dims {
		if len(d.byRole) == 0 {
			continue
		}
		slices.SortFunc(d.byRole, func(a, b types.RolePrincipalValues) int {
			return strings.Compare(a.Role, b.Role)
		})
		var granted, requestable []string
		for v := range d.granted {
			granted = append(granted, v)
		}
		for v := range d.requestable {
			if _, ok := d.granted[v]; !ok {
				requestable = append(requestable, v)
			}
		}
		slices.Sort(granted)
		slices.Sort(requestable)
		out = append(out, types.ResourcePrincipalSet{
			Kind:        d.kind,
			Granted:     granted,
			Requestable: requestable,
			ByRole:      d.byRole,
		})
	}
	return out
}

// ValidateDatabaseConstraintCoverage verifies that the requested database
// constraints are satisfiable by the applicable roles. Every requested value
// must be granted by at least one applicable role, and a wildcard value only
// by a role granting the wildcard for that dimension. When both users and
// names are requested, every value must additionally participate in at least
// one requested (user, name) combination that a single applicable role grants
// in full: session-time role matchers are evaluated per role, and
// per-dimension coverage alone would admit requests that pass approval and
// fail at connect.
func ValidateDatabaseConstraintCoverage(
	constraints *types.DatabaseResourceConstraints,
	applicableRoles []types.Role,
	database types.Database,
	username string,
	traits map[string][]string,
	localCluster string,
) error {
	if constraints == nil {
		return nil
	}

	// Grants are kept per role rather than unioned: the combination check
	// below needs to know which single role co-grants a (user, name) pair.
	type roleGrants struct {
		users, usersDenied, names, namesDenied, dbRoles set.Set[string]
		userWildcard, nameWildcard                      bool
	}
	grants := make([]roleGrants, 0, len(applicableRoles))
	for _, role := range applicableRoles {
		singleChecker := NewAccessCheckerWithRoleSet(&AccessInfo{
			Username: username,
			Traits:   traits,
		}, localCluster, RoleSet{role})

		g := roleGrants{}
		if res, err := singleChecker.EnumerateDatabaseUsers(database); err == nil {
			g.userWildcard = res.WildcardAllowed()
			allowed, denied := res.ToEntities()
			g.users = set.New(allowed...)
			g.usersDenied = set.New(denied...)
		}
		namesResult := singleChecker.EnumerateDatabaseNames(database)
		g.nameWildcard = namesResult.WildcardAllowed()
		allowed, denied := namesResult.ToEntities()
		g.names = set.New(allowed...)
		g.namesDenied = set.New(denied...)
		if r, err := singleChecker.CheckDatabaseRoles(database, nil); err == nil {
			g.dbRoles = set.New(r...)
		}
		grants = append(grants, g)
	}

	grantsUser := func(g roleGrants, u string) bool {
		if u == types.Wildcard {
			return g.userWildcard
		}
		if g.usersDenied.Contains(u) {
			return false
		}
		return g.userWildcard || g.users.Contains(u)
	}
	grantsName := func(g roleGrants, n string) bool {
		if n == types.Wildcard {
			return g.nameWildcard
		}
		if g.namesDenied.Contains(n) {
			return false
		}
		return g.nameWildcard || g.names.Contains(n)
	}

	// Per-dimension coverage: every requested value is granted by some
	// applicable role. A wildcard is covered only by a wildcard grant, never
	// by static grants.
	for _, u := range constraints.Users {
		if !slices.ContainsFunc(grants, func(g roleGrants) bool { return grantsUser(g, u) }) {
			return trace.BadParameter("requested database user %q is not granted by any applicable role", u)
		}
	}
	for _, n := range constraints.Names {
		if !slices.ContainsFunc(grants, func(g roleGrants) bool { return grantsName(g, n) }) {
			return trace.BadParameter("requested database name %q is not granted by any applicable role", n)
		}
	}
	for _, r := range constraints.Roles {
		if !slices.ContainsFunc(grants, func(g roleGrants) bool { return g.dbRoles.Contains(r) }) {
			return trace.BadParameter("requested database role %q is not granted by any applicable role", r)
		}
	}

	// Combination coverage: database access requires a single role to grant
	// the (user, name) pair together, so every requested value must form a
	// pair with some value from the other dimension under one role. An
	// omitted dimension applies no narrowing and is exempt.
	if len(constraints.Users) == 0 || len(constraints.Names) == 0 {
		return nil
	}
	pairGranted := func(u, n string) bool {
		return slices.ContainsFunc(grants, func(g roleGrants) bool {
			return grantsUser(g, u) && grantsName(g, n)
		})
	}
	for _, u := range constraints.Users {
		if !slices.ContainsFunc(constraints.Names, func(n string) bool { return pairGranted(u, n) }) {
			return trace.BadParameter("requested database user %q is not granted together with any requested database name by a single role", u)
		}
	}
	for _, n := range constraints.Names {
		if !slices.ContainsFunc(constraints.Users, func(u string) bool { return pairGranted(u, n) }) {
			return trace.BadParameter("requested database name %q is not granted together with any requested database user by a single role", n)
		}
	}

	return nil
}
