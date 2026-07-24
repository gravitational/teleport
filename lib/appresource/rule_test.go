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
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRuleValidate(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		wantErr string
	}{
		{
			name:    "empty rule",
			rule:    Rule{},
			wantErr: "must set paths or unsafe_allow_all",
		},
		{
			name:    "present but empty paths",
			rule:    Rule{Paths: []string{}},
			wantErr: "must set paths or unsafe_allow_all",
		},
		{
			name: "paths alone",
			rule: Rule{Paths: []string{"/api/**"}},
		},
		{
			name: "unsafe_allow_all alone",
			rule: Rule{UnsafeAllowAll: true},
		},
		{
			name:    "unsafe_allow_all with paths",
			rule:    Rule{UnsafeAllowAll: true, Paths: []string{"/api/**"}},
			wantErr: "cannot be combined",
		},
		{
			name:    "unsafe_allow_all with methods",
			rule:    Rule{UnsafeAllowAll: true, Methods: []string{"GET"}},
			wantErr: "cannot be combined",
		},
		{
			name:    "unsafe_allow_all with where",
			rule:    Rule{UnsafeAllowAll: true, Where: "true"},
			wantErr: "cannot be combined",
		},
		{
			name:    "unsafe_allow_all with allow_encoded",
			rule:    Rule{UnsafeAllowAll: true, AllowEncoded: []string{"/"}},
			wantErr: "cannot be combined",
		},
		{
			name:    "unsafe_allow_all with allow_code",
			rule:    Rule{UnsafeAllowAll: true, AllowCode: "all"},
			wantErr: "cannot be combined",
		},
		{
			name:    "unsafe_allow_all with allow_reason",
			rule:    Rule{UnsafeAllowAll: true, AllowReason: "all"},
			wantErr: "cannot be combined",
		},
		{
			name:    "unsafe_allow_all with deny_code_hint",
			rule:    Rule{UnsafeAllowAll: true, DenyCodeHint: "no"},
			wantErr: "cannot be combined",
		},
		{
			name:    "unsafe_allow_all with deny_reason_hint",
			rule:    Rule{UnsafeAllowAll: true, DenyReasonHint: "no"},
			wantErr: "cannot be combined",
		},
		{
			name: "valid methods",
			rule: Rule{Paths: []string{"/api/**"}, Methods: []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE"}},
		},
		{
			name: "lowercase method",
			rule: Rule{Paths: []string{"/api/**"}, Methods: []string{"get"}},
		},
		{
			name:    "typoed method",
			rule:    Rule{Paths: []string{"/api/**"}, Methods: []string{"GTE"}},
			wantErr: "not a standard HTTP method",
		},
		{
			name:    "connect method",
			rule:    Rule{Paths: []string{"/api/**"}, Methods: []string{"CONNECT"}},
			wantErr: "not a standard HTTP method",
		},
		{
			name:    "empty method name",
			rule:    Rule{Paths: []string{"/api/**"}, Methods: []string{""}},
			wantErr: "not a standard HTTP method",
		},
		{
			name: "allow code and reason",
			rule: Rule{Paths: []string{"/api/**"}, AllowCode: "public_api", AllowReason: "Public API."},
		},
		{
			name:    "allow_reason without allow_code",
			rule:    Rule{Paths: []string{"/api/**"}, AllowReason: "Public API."},
			wantErr: "allow_reason set without allow_code",
		},
		{
			name:    "allow_code with illegal chars",
			rule:    Rule{Paths: []string{"/api/**"}, AllowCode: "Public-API"},
			wantErr: "invalid allow_code",
		},
		{
			name:    "allow_code with reserved prefix",
			rule:    Rule{Paths: []string{"/api/**"}, AllowCode: "teleport_mine"},
			wantErr: "invalid allow_code",
		},
		{
			name: "deny hint with where",
			rule: Rule{Paths: []string{"/api/**"}, Where: `contains(user.roles, "dev")`, DenyCodeHint: "needs_dev", DenyReasonHint: "Needs the dev role."},
		},
		{
			name:    "deny_reason_hint without deny_code_hint",
			rule:    Rule{Paths: []string{"/api/**"}, Where: "true", DenyReasonHint: "Needs the dev role."},
			wantErr: "deny_reason_hint set without deny_code_hint",
		},
		{
			name:    "deny_code_hint without where",
			rule:    Rule{Paths: []string{"/api/**"}, DenyCodeHint: "needs_dev"},
			wantErr: "deny_code_hint set without a where clause",
		},
		{
			name:    "deny_code_hint with reserved prefix",
			rule:    Rule{Paths: []string{"/api/**"}, Where: "true", DenyCodeHint: "teleport_no"},
			wantErr: "invalid deny_code_hint",
		},
		{
			name:    "where calls path.match",
			rule:    Rule{Paths: []string{"/api/**"}, Where: `path.match(literal("api"))`},
			wantErr: "where may not call path.match",
		},
		{
			name: "path.match inside a string literal",
			rule: Rule{Paths: []string{"/api/**"}, Where: `user.name == "path.match(x)"`},
		},
		{
			name: "match call on another receiver",
			rule: Rule{Paths: []string{"/api/**"}, Where: `foo.match("x")`},
		},
		{
			name:    "unparseable where",
			rule:    Rule{Paths: []string{"/api/**"}, Where: `user.name ==`},
			wantErr: "does not parse",
		},
		{
			name:    "unbalanced where fragment",
			rule:    Rule{Paths: []string{"/api/**"}, Where: `false) || (true`},
			wantErr: "does not parse",
		},
		{
			name: "allow_encoded slash",
			rule: Rule{Paths: []string{"/api/**"}, AllowEncoded: []string{"/"}},
		},
		{
			name:    "allow_encoded other char",
			rule:    Rule{Paths: []string{"/api/**"}, AllowEncoded: []string{"%"}},
			wantErr: "allow_encoded admits only",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestRuleFromYAML pins the YAML field names of the sugared authoring
// surface, written in the exact form an author would put under
// app_resources.
func TestRuleFromYAML(t *testing.T) {
	const doc = `
paths: ["/api/v4/projects/{project}/**"]
methods: [GET, HEAD]
where: contains(user.traits["allowed_projects"], vars.project)
allow_encoded: ["/"]
allow_code: repo_read
allow_reason: "Read access to the repository API"
deny_code_hint: project_not_allowed
deny_reason_hint: "Project is not in the caller's allowlist"
`
	var r Rule
	require.NoError(t, yaml.Unmarshal([]byte(doc), &r))
	want := Rule{
		Paths:          []string{"/api/v4/projects/{project}/**"},
		Methods:        []string{"GET", "HEAD"},
		Where:          `contains(user.traits["allowed_projects"], vars.project)`,
		AllowEncoded:   []string{"/"},
		AllowCode:      "repo_read",
		AllowReason:    "Read access to the repository API",
		DenyCodeHint:   "project_not_allowed",
		DenyReasonHint: "Project is not in the caller's allowlist",
	}
	require.Equal(t, want, r)
	require.NoError(t, r.validate())

	var unsafeRule Rule
	require.NoError(t, yaml.Unmarshal([]byte(`unsafe_allow_all: true`), &unsafeRule))
	require.Equal(t, Rule{UnsafeAllowAll: true}, unsafeRule)
	require.NoError(t, unsafeRule.validate())
}

// TestMethodClause pins the rendered membership test and its
// case-insensitive matching: the clause compiles in the predicate
// language and matches the request method with both sides folded to
// upper case.
func TestMethodClause(t *testing.T) {
	rule := Rule{Methods: []string{"GET", "head"}}
	clause := rule.methodClause()
	require.Equal(t, `contains(set("GET", "HEAD"), upper(request.method))`, clause)

	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"get", true},
		{"GeT", true},
		{"HEAD", true},
		{"head", true},
		{"POST", false},
		{"DELETE", false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got, _ := evaluate(t, clause, Request{Method: tt.method}, Identity{})
			require.Equal(t, tt.want, got)
		})
	}
}

// TestMethodClauseEmpty pins that an empty Methods list renders no
// clause, so it constrains nothing and any method is permitted.
func TestMethodClauseEmpty(t *testing.T) {
	require.Empty(t, Rule{}.methodClause())
}

// TestWhereByteCap pins the 1 KiB cap on a sugared where clause. A
// clause at the cap validates. One byte over is a load error, so an
// unbounded condition cannot ride into the evaluator or the audit log.
func TestWhereByteCap(t *testing.T) {
	atCap := `user.name == "x"` + strings.Repeat(" ", maxWhereBytes-len(`user.name == "x"`))
	require.Len(t, atCap, maxWhereBytes)
	require.NoError(t, Rule{Paths: []string{"/api/**"}, Where: atCap}.validate())

	err := Rule{Paths: []string{"/api/**"}, Where: atCap + " "}.validate()
	require.ErrorContains(t, err, "over the")
}

// TestReasonByteCap pins the 1 KiB cap on allow_reason and
// deny_reason_hint. A reason rides on every matching audit event, so an
// unbounded reason is rejected at load.
func TestReasonByteCap(t *testing.T) {
	atCap := strings.Repeat("x", maxReasonBytes)

	require.NoError(t, Rule{Paths: []string{"/api/**"}, AllowCode: "ok", AllowReason: atCap}.validate())
	err := Rule{Paths: []string{"/api/**"}, AllowCode: "ok", AllowReason: atCap + "x"}.validate()
	require.ErrorContains(t, err, "over the")

	require.NoError(t, Rule{Paths: []string{"/api/**"}, Where: "true", DenyCodeHint: "no", DenyReasonHint: atCap}.validate())
	err = Rule{Paths: []string{"/api/**"}, Where: "true", DenyCodeHint: "no", DenyReasonHint: atCap + "x"}.validate()
	require.ErrorContains(t, err, "over the")
}

// TestExpressionReasonByteCap pins that a constant reason in an
// expression entry's allow_code or deny_hint call is held to the same
// 1 KiB cap as the sugared reason fields, so the two surfaces bound a
// reason identically.
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

// TestExpressionByteCap pins the 4 KiB cap on one
// app_resources_expressions entry. An entry at the cap compiles. One
// byte over is a load error.
func TestExpressionByteCap(t *testing.T) {
	atCap := `contains(user.roles, "dev")`
	atCap += strings.Repeat(" ", maxExpressionBytes-len(atCap))
	require.Len(t, atCap, maxExpressionBytes)
	_, err := compileExpression(atCap)
	require.NoError(t, err, "an expression at the cap compiles")

	_, err = compileExpression(atCap + " ")
	require.ErrorContains(t, err, "over the")
}

// TestCompileExpression pins the load path of one
// app_resources_expressions entry: an empty or blank entry is a load
// error, and a valid entry compiles to a predicate that evaluates.
func TestCompileExpression(t *testing.T) {
	_, err := compileExpression("")
	require.ErrorContains(t, err, "cannot be empty")
	_, err = compileExpression("   ")
	require.ErrorContains(t, err, "cannot be empty")

	pred, err := compileExpression(`contains(user.roles, "dev") && request.method == "GET"`)
	require.NoError(t, err)

	got, err := pred.Evaluate(newEnv(Request{Method: "GET"}, Identity{Roles: []string{"dev"}}))
	require.NoError(t, err)
	require.True(t, got)

	got, err = pred.Evaluate(newEnv(Request{Method: "POST"}, Identity{Roles: []string{"dev"}}))
	require.NoError(t, err)
	require.False(t, got)
}

// TestCompileExpressionRejectsBadCode pins that an illegal constant
// audit code in an expression entry fails at compile, through the same
// AST walk a sugared rule's codes go through.
func TestCompileExpressionRejectsBadCode(t *testing.T) {
	_, err := compileExpression(`allow_code("teleport_mine", "Reserved.", true)`)
	require.ErrorContains(t, err, "teleport_")
}
