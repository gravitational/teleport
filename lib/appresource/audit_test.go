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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAllowCode pins that allow_code records its code and reason only when
// the wrapped expression is true, and is transparent to the boolean result.
func TestAllowCode(t *testing.T) {
	const expr = `allow_code("project_read", "User has access to project.", ` +
		`contains(user.traits["allowed_projects"], "acme") && request.method == "GET")`
	identity := Identity{Traits: map[string][]string{"allowed_projects": {"acme"}}}

	got, state := evaluate(t, expr, Request{Method: "GET"}, identity)
	require.True(t, got)
	require.Equal(t, "project_read", state.allowCode)
	require.Equal(t, "User has access to project.", state.allowReason)

	got, state = evaluate(t, expr, Request{Method: "POST"}, identity)
	require.False(t, got)
	require.Empty(t, state.allowCode)
	require.Empty(t, state.allowReason)
}

func TestAllowCodeBranchSpecific(t *testing.T) {
	const expr = `allow_code("read", "Read.", request.method == "GET") || ` +
		`allow_code("write", "Write.", request.method == "POST")`

	got, state := evaluate(t, expr, Request{Method: "GET"}, Identity{})
	require.True(t, got)
	require.Equal(t, "read", state.allowCode)

	got, state = evaluate(t, expr, Request{Method: "POST"}, Identity{})
	require.True(t, got)
	require.Equal(t, "write", state.allowCode)
}

func TestAllowCodeLastWins(t *testing.T) {
	const expr = `allow_code("first", "First.", true) && allow_code("second", "Second.", true)`

	got, state := evaluate(t, expr, Request{}, Identity{})
	require.True(t, got)
	require.Equal(t, "second", state.allowCode)
	require.Equal(t, "Second.", state.allowReason)
}

// TestAllowCodeOnDeny pins that allowCode stays set when an allow_code
// fired true but a later condition made the whole predicate false. The
// caller reads allowCode only on an allow.
func TestAllowCodeOnDeny(t *testing.T) {
	const expr = `allow_code("read", "Read.", true) && request.method == "POST"`

	got, state := evaluate(t, expr, Request{Method: "GET"}, Identity{})
	require.False(t, got)
	require.Equal(t, "read", state.allowCode)
}

// TestDenyHint pins that deny_hint records a hint exactly on the near-miss,
// when the conditions on its left matched but the wrapped condition failed.
func TestDenyHint(t *testing.T) {
	const expr = `request.method == "GET" && deny_hint("not_in_allowlist", ` +
		`"Project is not in your allowlist.", contains(user.traits["allowed_projects"], "acme"))`
	allowed := Identity{Traits: map[string][]string{"allowed_projects": {"acme"}}}

	// The wrapped condition holds: allow, no hint.
	got, state := evaluate(t, expr, Request{Method: "GET"}, allowed)
	require.True(t, got)
	require.Empty(t, state.denyHints)

	// Near miss: the method matches, the wrapped condition fails.
	got, state = evaluate(t, expr, Request{Method: "GET"}, Identity{})
	require.False(t, got)
	require.Equal(t, []Hint{{Code: "not_in_allowlist", Reason: "Project is not in your allowlist."}}, state.denyHints)

	// Method miss: the left of && fails first, so deny_hint never runs and
	// no hint fires.
	got, state = evaluate(t, expr, Request{Method: "POST"}, Identity{})
	require.False(t, got)
	require.Empty(t, state.denyHints)
}

// TestDenyHintOrder pins that several fired hints are recorded in evaluation
// order, and that a hint recorded on a branch that lost to a later match is
// still present.
func TestDenyHintOrder(t *testing.T) {
	const expr = `deny_hint("needs_dev", "You need the dev role.", contains(user.roles, "dev")) || ` +
		`deny_hint("needs_admin", "You need the admin role.", contains(user.roles, "admin"))`

	got, state := evaluate(t, expr, Request{}, Identity{})
	require.False(t, got)
	require.Equal(t, []Hint{
		{Code: "needs_dev", Reason: "You need the dev role."},
		{Code: "needs_admin", Reason: "You need the admin role."},
	}, state.denyHints)

	got, state = evaluate(t, expr, Request{}, Identity{Roles: []string{"admin"}})
	require.True(t, got)
	require.Equal(t, []Hint{{Code: "needs_dev", Reason: "You need the dev role."}}, state.denyHints)
}

// TestCodeValidationAtCompile pins that an illegal literal code in either
// wrapper fails at compile, not per request.
func TestCodeValidationAtCompile(t *testing.T) {
	valid := []string{
		`allow_code("a", "", true)`,
		`allow_code("` + strings.Repeat("a", 256) + `", "reason", true)`,
		`deny_hint("not_in_allowlist", "reason", true)`,
	}
	for _, expr := range valid {
		t.Run(expr, func(t *testing.T) {
			_, err := compilePredicate(expr)
			require.NoError(t, err)
		})
	}

	invalid := []struct {
		expr    string
		wantErr string
	}{
		{`allow_code("Project_Read", "reason", true)`, "must contain only"},
		{`allow_code(("Bad Code"), "reason", true)`, "must contain only"},
		{`deny_hint("Bad Code", "reason", true)`, "must contain only"},
		{`deny_hint("teleport_x", "reason", true)`, "reserved teleport_ prefix"},
		{`request.method == "GET" && deny_hint("Bad Code", "reason", true)`, "must contain only"},
	}
	for _, tt := range invalid {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := compilePredicate(tt.expr)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestCodeValidationAtEvaluation pins that a code built at evaluation, which
// the compile-time walk cannot see, is still rejected by the function.
func TestCodeValidationAtEvaluation(t *testing.T) {
	for _, expr := range []string{
		`allow_code(lower("BAD CODE"), "reason", true)`,
		`deny_hint(upper("bad_code"), "reason", true)`,
	} {
		t.Run(expr, func(t *testing.T) {
			pred, err := compilePredicate(expr)
			require.NoError(t, err)
			_, err = pred.Evaluate(newEnv(Request{}, Identity{}))
			require.ErrorContains(t, err, "must contain only")
		})
	}
}

func TestValidateAuditCode(t *testing.T) {
	valid := []string{"a", "0", "_", "project_read", "a1_b2", strings.Repeat("a", 256)}
	for _, code := range valid {
		require.NoError(t, validateAuditCode(code), code)
	}

	invalid := []string{"", "A", "Project_Read", "has space", "a-b", "teleport_", "teleport_x", "ümlaut", strings.Repeat("a", 257)}
	for _, code := range invalid {
		require.Error(t, validateAuditCode(code), code)
	}
}
