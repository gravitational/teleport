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

// TestDenyCodeNearMiss pins the collapsed deny code: the scalar deny_code and
// deny_reason fire on a deny exactly when the path and method matched but where
// failed, the near-miss territory.
func TestDenyCodeNearMiss(t *testing.T) {
	rule, err := ruleFromYAML(t, `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET]
where: contains(user.traits["allowed_projects"], vars.project)
allow_code: project_read
deny_code: not_in_allowlist
deny_reason: "Project is not in your allowlist."
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
deny_code: needs_admin
deny_reason: "You need the admin role."
`).Compile()
	require.Error(t, err)
}

// TestAppendDenyHintExpression pins that an expression rule can carry a deny
// hint through append_deny_hint, the primitive the sugared deny_code lowers to,
// so the expression and sugared surfaces share one deny mechanism. The call
// returns false, so it records the hint without ever turning the deny into an
// allow, and an illegal code is rejected at load.
func TestAppendDenyHintExpression(t *testing.T) {
	rule, err := compileExpression(
		`path.match(literal("api/v4/projects", capture("project", greedy()))) && ` +
			`(contains(user.traits["allowed_projects"], vars.project) || append_deny_hint("denied_project", "No access."))`)
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
	_, err = compileExpression(`path.match(greedy()) || append_deny_hint("Bad Code")`)
	require.Error(t, err)
}

// TestDenyReasonWithoutCodeRejected pins that a deny_reason with no deny_code is
// a load error: the reason has no code to ride on.
func TestDenyReasonWithoutCodeRejected(t *testing.T) {
	_, err := ruleFromYAML(t, `
paths: ["/api/health"]
deny_reason: "orphan reason"
`).Compile()
	require.Error(t, err)
}

// TestSetAllowCodeInExpression pins that an expression rule carries its allow
// code through a set_allow_code call, the primitive the sugared allow_code field
// lowers to, and that an expression rule has no deny mechanism.
func TestSetAllowCodeInExpression(t *testing.T) {
	rule, err := compileExpression(
		`path.match(literal("api/v4/user")) && set_allow_code("self_read", "Read your own user.")`)
	require.NoError(t, err)

	got, err := rule.Evaluate(Request{Method: "GET", Path: "/api/v4/user"}, Identity{})
	require.NoError(t, err)
	require.True(t, got.Allowed)
	require.Equal(t, "self_read", got.Allow.Code)
	require.Equal(t, "Read your own user.", got.Allow.Reason)

	// A deny from an expression rule carries no hint: the deny code is sugar-only.
	got, err = rule.Evaluate(Request{Method: "GET", Path: "/api/v4/groups"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Empty(t, got.Deny.Hints)
}

// TestSetAllowCodeMustBeTail pins the placement check. A set_allow_code that is
// not the last term of its && chain is rejected at load, because a code
// committed before a later term could leak into an allow that a different ||
// branch granted. The fix is to put set_allow_code at the tail.
func TestSetAllowCodeMustBeTail(t *testing.T) {
	// set_allow_code sits before a trailing && term, so it could run on a branch
	// that is not the one granting the allow. Rejected at load.
	_, err := compileExpression(
		`path.match(literal("api/v4/user")) && set_allow_code("self_read") && contains(user.roles, "admin")`)
	require.Error(t, err)

	// The leak the check prevents: set_allow_code on a false && branch that a
	// later || branch rescues. Rejected at load rather than leaking "leaked".
	_, err = compileExpression(
		`(set_allow_code("leaked") && path.match(literal("never"))) || path.match(literal("api/v4/user"))`)
	require.Error(t, err)

	// A negated set_allow_code is never tail-safe.
	_, err = compileExpression(`!set_allow_code("neg") || path.match(literal("api/v4/user"))`)
	require.Error(t, err)
}

// TestSetAllowCodeBranchSpecific pins the expressiveness a per-rule field
// cannot reach: the code depends on which alternative matched.
func TestSetAllowCodeBranchSpecific(t *testing.T) {
	rule, err := compileExpression(
		`(path.match(literal("api/v4/user")) && set_allow_code("user_read")) || ` +
			`(path.match(literal("api/v4/version")) && set_allow_code("version_read"))`)
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
