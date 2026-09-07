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

package appresource

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoleSetEvaluateOrder(t *testing.T) {
	roles := []Role{
		{Name: "b", Expressions: []string{`allow_code("from_b", "Allowed.", request.method == "GET")`}},
		{Name: "a", Expressions: []string{`allow_code("from_a", "Allowed.", request.method == "GET")`}},
	}
	set, err := CompileRoles(roles)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, set.roleNames())

	decision, err := set.Evaluate(Request{Method: "GET", Path: "/x"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, &AllowDetails{Role: "a", Code: "from_a", Reason: "Allowed."}, decision.Allow)
	require.Equal(t, []string{"a", "b"}, decision.Roles)
}

func TestRoleSetEvaluateVars(t *testing.T) {
	roles := []Role{{Name: "dev", Expressions: []string{`path.match(literal("api", capture("v", greedy()))) && vars.v == "one"`}}}
	set, err := CompileRoles(roles)
	require.NoError(t, err)

	decision, err := set.Evaluate(Request{Method: "GET", Path: "/api/one"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, map[string]string{"v": "one"}, decision.Allow.Vars)

	decision, err = set.Evaluate(Request{Method: "GET", Path: "/api/two"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.Nil(t, decision.Allow)
	require.Equal(t, DenyNotAllowed, decision.Deny.Kind)
	require.Empty(t, decision.Deny.Hints)
}

func TestRoleSetAggregatesHints(t *testing.T) {
	roles := []Role{
		{Name: "a", Expressions: []string{
			`deny_hint("needs_dev", "Dev role required.", contains(user.roles, "dev"))`,
			`deny_hint("needs_ops", "Ops role required.", contains(user.roles, "ops"))`,
		}},
		{Name: "b", Expressions: []string{`deny_hint("needs_admin", "Admin role required.", contains(user.roles, "admin"))`}},
	}
	set, err := CompileRoles(roles)
	require.NoError(t, err)

	decision, err := set.Evaluate(Request{Method: "GET", Path: "/x"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.Equal(t, DenyNotAllowed, decision.Deny.Kind)
	want := []Hint{
		{Code: "needs_dev", Reason: "Dev role required."},
		{Code: "needs_ops", Reason: "Ops role required."},
		{Code: "needs_admin", Reason: "Admin role required."},
	}
	require.Equal(t, want, decision.Deny.Hints)

	admin := Identity{Name: "root", Roles: []string{"admin"}}
	decision, err = set.Evaluate(Request{Method: "GET", Path: "/x"}, admin)
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Nil(t, decision.Deny)
}

func TestRoleSetAllowAll(t *testing.T) {
	roles := []Role{
		{Name: "a", Expressions: []string{`allow_code("from_a", "Allowed.", true)`}},
		{Name: "c", Resources: []Rule{{AllowAll: true}}},
		{Name: "b", Resources: []Rule{{AllowAll: true}}},
	}
	set, err := CompileRoles(roles)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "c", "a"}, set.roleNames())
	for _, request := range []Request{
		{Method: "GET", Path: "/x"},
		{Method: "BREW", Path: "/x"},
		{Method: "GET", Path: "/api/../secret"},
	} {
		decision, err := set.Evaluate(request, Identity{Name: "alice"})
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		require.Equal(t, &AllowDetails{Role: "b"}, decision.Allow)
		require.Equal(t, []string{"b", "c", "a"}, decision.Roles)
	}
}

func TestRoleSetNoRules(t *testing.T) {
	badPath := Request{Method: "GET", Path: "/api/../secret"}
	for _, roles := range [][]Role{nil, {{Name: "empty"}}} {
		set, err := CompileRoles(roles)
		require.NoError(t, err)
		decision, err := set.Evaluate(badPath, Identity{Name: "alice"})
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenyNotAllowed, decision.Deny.Kind)
		require.Equal(t, set.roleNames(), decision.Roles)
	}
}

func TestRoleSetInvalidRequest(t *testing.T) {
	badPath := Request{Method: "GET", Path: "/api/../secret"}
	badMethod := Request{Method: "BREW", Path: "/x"}

	// The rule would record a hint if it ran, so an empty hint list shows
	// the deny happened before any rule.
	roles := []Role{{Name: "expr", Expressions: []string{`deny_hint("ran", "Rule ran.", false)`}}}
	set, err := CompileRoles(roles)
	require.NoError(t, err)
	for _, request := range []Request{badPath, badMethod} {
		decision, err := set.Evaluate(request, Identity{Name: "alice"})
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenyInvalidRequest, decision.Deny.Kind)
		require.Empty(t, decision.Deny.Hints)
		require.Equal(t, []string{"expr"}, decision.Roles)
	}
}

func TestRoleSetRuleCap(t *testing.T) {
	const rulesPerRole = 64
	perRole := make([]string, rulesPerRole)
	for i := range perRole {
		perRole[i] = "false"
	}
	roleCount := maxRulesPerRequest / rulesPerRole
	roles := make([]Role, 0, roleCount+1)
	for i := range roleCount {
		roles = append(roles, Role{Name: "role-" + strconv.Itoa(i), Expressions: perRole})
	}
	set, err := CompileRoles(roles)
	require.NoError(t, err)
	decision, err := set.Evaluate(Request{Method: "GET", Path: "/x"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.Equal(t, DenyNotAllowed, decision.Deny.Kind)

	roles = append(roles, Role{Name: "one-more", Expressions: []string{"false"}})
	set, err = CompileRoles(roles)
	require.NoError(t, err)
	for _, request := range []Request{
		{Method: "GET", Path: "/x"},
		{Method: "BREW", Path: "/x"},
		{Method: "GET", Path: "/api/../secret"},
	} {
		decision, err = set.Evaluate(request, Identity{Name: "alice"})
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenyTooManyRules, decision.Deny.Kind)
		require.Len(t, decision.Roles, roleCount+1)
	}
}

func TestRoleSetHintCap(t *testing.T) {
	exprs := make([]string, maxHints+1)
	for i := range exprs {
		exprs[i] = fmt.Sprintf(`deny_hint("hint_%d", "No.", false)`, i)
	}
	set, err := CompileRoles([]Role{{Name: "hints", Expressions: exprs}})
	require.NoError(t, err)
	decision, err := set.Evaluate(Request{Method: "GET", Path: "/x"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.Len(t, decision.Deny.Hints, maxHints)
	require.Equal(t, "hint_0", decision.Deny.Hints[0].Code)
}

func TestCompileRolesValidatesRules(t *testing.T) {
	_, err := CompileRoles([]Role{{
		Name:      "dev",
		Resources: []Rule{{AllowAll: true, Methods: []string{"GET"}}},
	}})
	require.ErrorContains(t, err, `role "dev" app_resources 0`)

	long := `request.method == ` + strconv.Quote(strings.Repeat("x", maxExpressionBytes))
	_, err = CompileRoles([]Role{{Name: "dev", Expressions: []string{"true", long}}})
	require.ErrorContains(t, err, `role "dev" app_resources_expressions 1`)
	require.ErrorContains(t, err, "byte maximum")
}

func TestRoleSetEvaluatePathRule(t *testing.T) {
	roles := []Role{{Name: "dev", Resources: []Rule{{
		Paths:       []string{"/api/{version}/**"},
		Methods:     []string{"GET"},
		Where:       `vars.version == "v4"`,
		AllowCode:   "api_v4",
		AllowReason: "Allowed.",
	}}}}
	set, err := CompileRoles(roles)
	require.NoError(t, err)

	decision, err := set.Evaluate(Request{Method: "GET", Path: "/api/v4/health"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, &AllowDetails{Role: "dev", Vars: map[string]string{"version": "v4"}, Code: "api_v4", Reason: "Allowed."}, decision.Allow)

	decision, err = set.Evaluate(Request{Method: "POST", Path: "/api/v4/health"}, Identity{Name: "alice"})
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.Equal(t, DenyNotAllowed, decision.Deny.Kind)
}
