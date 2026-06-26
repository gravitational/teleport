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

package resourcematcher

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAllowCode pins that a matching rule carries its allow_code and
// allow_reason on the decision, and that neither appears on a deny.
func TestAllowCode(t *testing.T) {
	rule, err := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/repository/**"]
methods: [GET]
allow_code: allowed_project
allow_reason: "User has access to project"
`).Compile()
	require.NoError(t, err)

	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/x/repository/tree"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "allowed_project", got.Allow.Code)
	require.Equal(t, "User has access to project", got.Allow.Reason)

	denied, err := rule.Evaluate(Request{Method: "POST", Path: "/api/v4/projects/x/repository/tree"}, Identity{})
	require.NoError(t, err)
	require.False(t, denied.Allowed)
	require.Nil(t, denied.Allow)
}

func TestAllowCodeValidation(t *testing.T) {
	for _, code := range []string{"Project_Read", "teleport_internal", "has space", ""} {
		// The empty string means "no code", which is allowed; the others are
		// invalid charset or reserved prefix.
		_, err := ruleFromYAML(t, `
paths: ["/api/health"]
allow_code: `+strconv.Quote(code)+`
`).Compile()
		if code == "" {
			require.NoError(t, err, code)
			continue
		}
		require.Error(t, err, code)
	}
}

// TestDenyCodeNearMiss pins the collapsed deny hint: the scalar deny_code_hint
// and deny_reason_hint fire on a deny exactly when the path and method matched
// but where failed, the near-miss territory.
func TestDenyCodeNearMiss(t *testing.T) {
	rule, err := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET]
where: contains(user.traits["allowed_projects"], vars.project)
allow_code: project_read
deny_code_hint: not_in_allowlist
deny_reason_hint: "Project is not in your allowlist."
`).Compile()
	require.NoError(t, err)

	allowed := Identity{Traits: map[string][]string{"allowed_projects": {"ok"}}}

	// Allow: path, method, and where all hold. The deny code does not fire.
	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/ok/issues"}, allowed)
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "project_read", got.Allow.Code)
	require.Nil(t, got.Deny)

	// Near miss: path and method match, where fails. The deny code fires.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/secret/issues"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, []Hint{{Code: "not_in_allowlist", Reason: "Project is not in your allowlist."}}, got.Deny.Hints)

	// Method miss: the path matches but the method does not, so the near-miss
	// territory, which requires both, does not fire.
	got, err = rule.Evaluate(Request{Method: "POST", Path: "/api/v4/projects/secret/issues"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Empty(t, got.Deny.Hints)

	// Path miss: outside the rule's territory entirely. No deny code.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/groups/secret"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Empty(t, got.Deny.Hints)
}

// TestWhereOnlyRejected pins that a where-only rule is a load error in v9: a
// rule must scope its access through paths or opt into everything through
// unsafe_allow_all, so a rule with only a where clause, even one carrying a deny
// code, no longer compiles.
func TestWhereOnlyRejected(t *testing.T) {
	_, err := ruleFromYAML(t, `
where: contains(user.roles, "admin")
deny_code_hint: needs_admin
deny_reason_hint: "You need the admin role."
`).Compile()
	require.Error(t, err)
}

// TestDenyHintExpression pins that an expression rule can carry a deny hint
// through deny_hint, the primitive the sugared deny_code_hint lowers to, so
// the expression and sugared surfaces share one deny mechanism. The call
// returns the value of the inner expression it wraps and records the hint only
// when that value is false, so a misplaced call can never turn a deny into an
// allow, and an illegal code is rejected at load.
func TestDenyHintExpression(t *testing.T) {
	rule, err := compileExpression(
		`path.match(literal("api/v4/projects", capture("project", greedy()))) && ` +
			`deny_hint("denied_project", "No access.", contains(user.traits["allowed_projects"], vars.project))`)
	require.NoError(t, err)

	allowed := Identity{Traits: map[string][]string{"allowed_projects": {"ok"}}}

	// Path matches, where holds: allow, no hint.
	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/ok/issues"}, allowed)
	require.NoError(t, err)
	require.True(t, got.Allowed)

	// Path matches, where fails: the hint fires on the near-miss.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/projects/secret/issues"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, []Hint{{Code: "denied_project", Reason: "No access."}}, got.Deny.Hints)

	// Path does not match: no near-miss, no hint.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/other"}, allowed)
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Empty(t, got.Deny.Hints)

	// An illegal code is rejected at load.
	_, err = compileExpression(`deny_hint("Bad Code", "reason", path.match(greedy()))`)
	require.Error(t, err)
}

// TestDenyReasonWithoutCodeRejected pins that a deny_reason_hint with no
// deny_code_hint is a load error: the reason has no code to ride on.
func TestDenyReasonWithoutCodeRejected(t *testing.T) {
	_, err := ruleFromYAML(t, `
paths: ["/api/health"]
deny_reason_hint: "orphan reason"
`).Compile()
	require.Error(t, err)
}

// TestAllowCodeInExpression pins that an expression rule carries its allow
// code through an allow_code wrapper, the primitive the sugared allow_code
// field lowers to, and that an expression rule with no deny_hint wrapper
// carries no deny hint.
func TestAllowCodeInExpression(t *testing.T) {
	rule, err := compileExpression(
		`allow_code("self_read", "Read your own user.", path.match(literal("api/v4/user")))`)
	require.NoError(t, err)

	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/user"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "self_read", got.Allow.Code)
	require.Equal(t, "Read your own user.", got.Allow.Reason)

	// A deny from this expression carries no hint: it calls no deny_hint.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/groups"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Empty(t, got.Deny.Hints)
}

// TestAllowCodeOnlyRecordsOnMatch pins that allow_code records its code only
// when the wrapped expression evaluates to true. A non-matching rule that
// wraps a false expression records nothing, so a later matching rule sees the
// empty allow code and carries its own.
func TestAllowCodeOnlyRecordsOnMatch(t *testing.T) {
	// A rule whose wrapped expression is false: the code must not leak into
	// the evaluation state, because the rule denies.
	rule, err := compileExpression(
		`allow_code("never", "should not be recorded", path.match(literal("api/v4/never")))`)
	require.NoError(t, err)

	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/user"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Nil(t, got.Allow)
}

// TestAllowCodeBranchSpecific pins the expressiveness a per-rule field cannot
// reach: the code depends on which alternative matched.
func TestAllowCodeBranchSpecific(t *testing.T) {
	rule, err := compileExpression(
		`allow_code("user_read", "Read user", path.match(literal("api/v4/user"))) || ` +
			`allow_code("version_read", "Read version", path.match(literal("api/v4/version")))`)
	require.NoError(t, err)

	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/user"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "user_read", got.Allow.Code)

	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/version"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "version_read", got.Allow.Code)
}
