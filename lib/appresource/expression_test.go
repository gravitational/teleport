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
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// evaluateResult compiles expr as an app_resources_expressions entry and
// evaluates it against request and identity.
func evaluateResult(t *testing.T, expr string, request Request, identity Identity) Result {
	t.Helper()
	expression, err := compileExpression(expr)
	require.NoError(t, err)
	result, err := evaluateExpression(expression, Env{Request: request, Identity: identity})
	require.NoError(t, err)
	return result
}

// TestAllowCode checks that allow_code records its code and reason only when
// the wrapped expression is true, and is transparent to the boolean result.
func TestAllowCode(t *testing.T) {
	const expr = `allow_code("project_read", "User has access to project.", ` +
		`contains(user.traits["allowed_projects"], "acme") && request.method == "GET")`
	identity := Identity{Traits: map[string][]string{"allowed_projects": {"acme"}}}

	result := evaluateResult(t, expr, Request{Method: "GET"}, identity)
	require.True(t, result.Value)
	require.Equal(t, "project_read", result.AuditRecord.AllowCode)
	require.Equal(t, "User has access to project.", result.AuditRecord.AllowReason)

	result = evaluateResult(t, expr, Request{Method: "POST"}, identity)
	require.False(t, result.Value)
	require.Empty(t, result.AuditRecord.AllowCode)
	require.Empty(t, result.AuditRecord.AllowReason)
}

// TestAllowCodeBranchSpecific checks that only the matching branch's
// allow_code is recorded. One allow_code per || branch is a reasonable use,
// giving each method its own audit code.
func TestAllowCodeBranchSpecific(t *testing.T) {
	const expr = `allow_code("read", "Read.", request.method == "GET") || ` +
		`allow_code("write", "Write.", request.method == "POST")`

	result := evaluateResult(t, expr, Request{Method: "GET"}, Identity{})
	require.True(t, result.Value)
	require.Equal(t, "read", result.AuditRecord.AllowCode)

	result = evaluateResult(t, expr, Request{Method: "POST"}, Identity{})
	require.True(t, result.Value)
	require.Equal(t, "write", result.AuditRecord.AllowCode)
}

func TestAllowCodeLastWins(t *testing.T) {
	const expr = `allow_code("first", "First.", true) && allow_code("second", "Second.", true)`

	result := evaluateResult(t, expr, Request{Method: "GET"}, Identity{})
	require.True(t, result.Value)
	require.Equal(t, "second", result.AuditRecord.AllowCode)
	require.Equal(t, "Second.", result.AuditRecord.AllowReason)
}

// TestAllowCodeOnDeny checks that a deny Result has no allow code.
func TestAllowCodeOnDeny(t *testing.T) {
	const expr = `allow_code("read", "Read.", true) && request.method == "POST"`

	result := evaluateResult(t, expr, Request{Method: "GET"}, Identity{})
	require.False(t, result.Value)
	require.Empty(t, result.AuditRecord.AllowCode)
	require.Empty(t, result.AuditRecord.AllowReason)
}

// TestDenyHint checks that a near-miss records the hint and a miss does
// not. See [Hint] for the terms.
func TestDenyHint(t *testing.T) {
	const expr = `request.method == "GET" && deny_hint("not_in_allowlist", ` +
		`"Project is not in your allowlist.", contains(user.traits["allowed_projects"], "acme"))`
	allowed := Identity{Traits: map[string][]string{"allowed_projects": {"acme"}}}

	// Match.
	result := evaluateResult(t, expr, Request{Method: "GET"}, allowed)
	require.True(t, result.Value)
	require.Empty(t, result.AuditRecord.DenyHints)

	// Near-miss, the hint is recorded.
	result = evaluateResult(t, expr, Request{Method: "GET"}, Identity{})
	require.False(t, result.Value)
	require.Equal(t, []Hint{{Code: "not_in_allowlist", Reason: "Project is not in your allowlist."}}, result.AuditRecord.DenyHints)

	// Miss, no hint is recorded.
	result = evaluateResult(t, expr, Request{Method: "POST"}, Identity{})
	require.False(t, result.Value)
	require.Empty(t, result.AuditRecord.DenyHints)
}

// TestDenyHintOrder checks that hints are recorded in evaluation order.
func TestDenyHintOrder(t *testing.T) {
	const expr = `deny_hint("needs_dev", "You need the dev role.", contains(user.roles, "dev")) || ` +
		`deny_hint("needs_admin", "You need the admin role.", contains(user.roles, "admin"))`

	result := evaluateResult(t, expr, Request{Method: "GET"}, Identity{})
	require.False(t, result.Value)
	require.Equal(t, []Hint{
		{Code: "needs_dev", Reason: "You need the dev role."},
		{Code: "needs_admin", Reason: "You need the admin role."},
	}, result.AuditRecord.DenyHints)

	result = evaluateResult(t, expr, Request{Method: "GET"}, Identity{Roles: []string{"admin"}})
	require.True(t, result.Value)
	require.Empty(t, result.AuditRecord.DenyHints)
}

// TestAllowCodeFromLosingBranch documents the somewhat confusing behavior of
// allow_code in the expression syntax, when a winning branch does not set an
// allow_code but a previous losing one did. The losing branch's allow_code
// is reported. Unfortunately there is no clean solution to this other than
// only allowing allow_code as a top-level call, or intercepting typical's
// boolean operators, which it does not expose, so we will treat the
// expression below as nonsensical, just as we would `true && false`.
func TestAllowCodeFromLosingBranch(t *testing.T) {
	const expr = `
(allow_code("alice_read", "Alice read", user.name == "alice") && request.method == "GET") ||
request.method == "POST"`

	result := evaluateResult(t, expr, Request{Method: "POST"}, Identity{Name: "alice"})
	require.True(t, result.Value)
	require.Equal(t, "alice_read", result.AuditRecord.AllowCode)
	require.Equal(t, "Alice read", result.AuditRecord.AllowReason)
}

// TestEvaluationsDoNotShareRecord checks that each evaluation of one
// compiled expression gets a fresh record, so a Result has only its own
// hints.
func TestEvaluationsDoNotShareRecord(t *testing.T) {
	expression, err := compileExpression(`deny_hint("needs_dev", "You need the dev role.", contains(user.roles, "dev"))`)
	require.NoError(t, err)

	first, err := evaluateExpression(expression, Env{Request: Request{Method: "GET"}, Identity: Identity{Name: "alice"}})
	require.NoError(t, err)
	require.Len(t, first.AuditRecord.DenyHints, 1)

	second, err := evaluateExpression(expression, Env{Request: Request{Method: "GET"}, Identity: Identity{Name: "bob"}})
	require.NoError(t, err)
	require.Equal(t, []Hint{{Code: "needs_dev", Reason: "You need the dev role."}}, second.AuditRecord.DenyHints)
}

// TestConcurrentEvaluateExpression checks that concurrent evaluations of one
// compiled expression each get their own hints.
func TestConcurrentEvaluateExpression(t *testing.T) {
	expression, err := compileExpression(`deny_hint("needs_dev", "You need the dev role.", contains(user.roles, "dev"))`)
	require.NoError(t, err)
	wantHints := []Hint{{Code: "needs_dev", Reason: "You need the dev role."}}

	var wg sync.WaitGroup
	for i := range 10 {
		hasRole := i%2 == 0
		wg.Go(func() {
			env := Env{Request: Request{Method: "GET"}}
			if hasRole {
				env.Identity = Identity{Roles: []string{"dev"}}
			}
			result, err := evaluateExpression(expression, env)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, hasRole, result.Value)
			if hasRole {
				assert.Empty(t, result.AuditRecord.DenyHints)
				return
			}
			assert.Equal(t, wantHints, result.AuditRecord.DenyHints)
		})
	}
	wg.Wait()
}

// TestCompileWhereRejectsWrappers checks that the "where" spec omits the
// audit wrappers, so a sugared where clause calling one fails to parse.
func TestCompileWhereRejectsWrappers(t *testing.T) {
	for _, expr := range []string{
		`allow_code("ok", "reason", true)`,
		`deny_hint("no", "reason", true)`,
	} {
		t.Run(expr, func(t *testing.T) {
			_, err := CompileWhere(expr)
			require.ErrorContains(t, err, "unsupported function")
		})
	}
}

// TestCodeValidationAtCompile checks that an illegal literal code in either
// wrapper fails at compile, not per request.
func TestCodeValidationAtCompile(t *testing.T) {
	valid := []string{
		`allow_code("a", "", true)`,
		`allow_code("` + strings.Repeat("a", 256) + `", "reason", true)`,
		`deny_hint("not_in_allowlist", "reason", true)`,
	}
	for _, expr := range valid {
		t.Run(expr, func(t *testing.T) {
			_, err := compileExpression(expr)
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
		{`allow_code("teleport_mine", "Reserved.", true)`, "reserved teleport_ prefix"},
		{`request.method == "GET" && deny_hint("Bad Code", "reason", true)`, "must contain only"},
	}
	for _, tt := range invalid {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := compileExpression(tt.expr)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestNonLiteralAuditCodeRejected checks that a code built at evaluation
// fails at compile, so a wrapper cannot error on a request and deny it.
func TestNonLiteralAuditCodeRejected(t *testing.T) {
	tests := []struct {
		expr    string
		wantErr string
	}{
		{`allow_code(lower("BAD CODE"), "reason", true)`, "code argument of allow_code"},
		{`deny_hint(upper("bad_code"), "reason", true)`, "code argument of deny_hint"},
		{`allow_code(user.name, "reason", true)`, "code argument of allow_code"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := compileExpression(tt.expr)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, "must be a string literal")
		})
	}
}

// TestDynamicReasonClamped checks that a reason built at evaluation is
// allowed and truncated to maxReasonBytes on a rune boundary. The "x"
// prefix puts the cap mid-rune, so the clamp must step back one byte.
func TestDynamicReasonClamped(t *testing.T) {
	identity := Identity{Name: "x" + strings.Repeat("é", maxReasonBytes)}
	result := evaluateResult(t, `deny_hint("no", user.name, false)`, Request{Method: "GET"}, identity)
	require.Len(t, result.AuditRecord.DenyHints, 1)
	reason := result.AuditRecord.DenyHints[0].Reason
	require.Len(t, reason, maxReasonBytes-1)
	require.True(t, utf8.ValidString(reason), "clamp cut inside a rune")

	identity = Identity{Name: strings.Repeat("x", maxReasonBytes+1)}
	result = evaluateResult(t, `allow_code("ok", user.name, true)`, Request{Method: "GET"}, identity)
	require.True(t, result.Value)
	require.Len(t, result.AuditRecord.AllowReason, maxReasonBytes)
}

// TestClamp checks the clamp edges: at-cap unchanged, one-over cut,
// mid-rune step-back, and invalid UTF-8 walking the cut to zero.
func TestClamp(t *testing.T) {
	atCap := strings.Repeat("x", maxReasonBytes)
	require.Equal(t, atCap, clamp(atCap, maxReasonBytes))
	require.Equal(t, atCap, clamp(atCap+"x", maxReasonBytes))

	midRune := "x" + strings.Repeat("é", maxReasonBytes)
	require.Equal(t, midRune[:maxReasonBytes-1], clamp(midRune, maxReasonBytes))

	require.Empty(t, clamp(strings.Repeat("\x80", maxReasonBytes+1), maxReasonBytes))
}

// TestDenyHintCap checks that one evaluation records at most maxHints hints.
func TestDenyHintCap(t *testing.T) {
	terms := make([]string, maxHints+1)
	for i := range terms {
		terms[i] = `deny_hint("no", "Reason.", false)`
	}
	result := evaluateResult(t, strings.Join(terms, " || "), Request{Method: "GET"}, Identity{})
	require.False(t, result.Value)
	require.Len(t, result.AuditRecord.DenyHints, maxHints)
}

// TestExpressionReasonByteCap checks that a literal reason in an expression
// entry's allow_code or deny_hint call is rejected at the same 1 KiB cap as
// the sugared reason fields. A dynamic reason is clamped at evaluation
// instead.
func TestExpressionReasonByteCap(t *testing.T) {
	atCap := strings.Repeat("x", maxReasonBytes)

	_, err := compileExpression(fmt.Sprintf("deny_hint(%q, %q, true)", "no", atCap))
	require.NoError(t, err)
	_, err = compileExpression(fmt.Sprintf("deny_hint(%q, %q, true)", "no", atCap+"x"))
	require.ErrorContains(t, err, "over the")

	_, err = compileExpression(fmt.Sprintf("allow_code(%q, %q, true)", "ok", atCap))
	require.NoError(t, err)
	_, err = compileExpression(fmt.Sprintf("allow_code(%q, %q, true)", "ok", atCap+"x"))
	require.ErrorContains(t, err, "over the")
}

// TestExpressionByteCap checks the 4 KiB cap on one
// app_resources_expressions entry. An entry at the cap compiles. One byte
// over is a load error.
func TestExpressionByteCap(t *testing.T) {
	atCap := `contains(user.roles, "dev")`
	atCap += strings.Repeat(" ", maxExpressionBytes-len(atCap))
	require.Len(t, atCap, maxExpressionBytes)
	_, err := compileExpression(atCap)
	require.NoError(t, err, "an expression at the cap compiles")

	_, err = compileExpression(atCap + " ")
	require.ErrorContains(t, err, "over the")
}

// TestCompileExpression checks the load path of one
// app_resources_expressions entry. An empty or blank entry is a load error,
// and a valid entry compiles to an expression that evaluates.
func TestCompileExpression(t *testing.T) {
	_, err := compileExpression("")
	require.ErrorContains(t, err, "cannot be empty")
	_, err = compileExpression("   ")
	require.ErrorContains(t, err, "cannot be empty")

	expression, err := compileExpression(`contains(user.roles, "dev") && request.method == "GET"`)
	require.NoError(t, err)

	result, err := evaluateExpression(expression, Env{Request: Request{Method: "GET"}, Identity: Identity{Roles: []string{"dev"}}})
	require.NoError(t, err)
	require.True(t, result.Value)

	result, err = evaluateExpression(expression, Env{Request: Request{Method: "POST"}, Identity: Identity{Roles: []string{"dev"}}})
	require.NoError(t, err)
	require.False(t, result.Value)
}
