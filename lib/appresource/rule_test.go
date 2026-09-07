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
	"go.yaml.in/yaml/v3"
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
			wantErr: "must set paths or allow_all",
		},
		{
			name:    "present but empty paths",
			rule:    Rule{Paths: []string{}},
			wantErr: "must set paths or allow_all",
		},
		{
			name: "paths alone",
			rule: Rule{Paths: []string{"/api/**"}},
		},
		{
			name: "allow_all alone",
			rule: Rule{AllowAll: true},
		},
		{
			name:    "allow_all with paths",
			rule:    Rule{AllowAll: true, Paths: []string{"/api/**"}},
			wantErr: "cannot be combined",
		},
		{
			name:    "allow_all with methods",
			rule:    Rule{AllowAll: true, Methods: []string{"GET"}},
			wantErr: "cannot be combined",
		},
		{
			name:    "allow_all with where",
			rule:    Rule{AllowAll: true, Where: "true"},
			wantErr: "cannot be combined",
		},
		{
			name:    "allow_all with allow_encoded",
			rule:    Rule{AllowAll: true, AllowEncoded: []string{"/"}},
			wantErr: "cannot be combined",
		},
		{
			name:    "allow_all with allow_code",
			rule:    Rule{AllowAll: true, AllowCode: "all"},
			wantErr: "cannot be combined",
		},
		{
			name:    "allow_all with allow_reason",
			rule:    Rule{AllowAll: true, AllowReason: "all"},
			wantErr: "cannot be combined",
		},
		{
			name:    "allow_all with deny_code_hint",
			rule:    Rule{AllowAll: true, DenyCodeHint: "no"},
			wantErr: "cannot be combined",
		},
		{
			name:    "allow_all with deny_reason_hint",
			rule:    Rule{AllowAll: true, DenyReasonHint: "no"},
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
			wantErr: "is not one of",
		},
		{
			name:    "connect method",
			rule:    Rule{Paths: []string{"/api/**"}, Methods: []string{"CONNECT"}},
			wantErr: "is not one of",
		},
		{
			name:    "empty method name",
			rule:    Rule{Paths: []string{"/api/**"}, Methods: []string{""}},
			wantErr: "is not one of",
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
			name: "whitespace-only where is unset",
			rule: Rule{Paths: []string{"/api/**"}, Where: "  \n"},
		},
		{
			name:    "deny_code_hint with whitespace-only where",
			rule:    Rule{Paths: []string{"/api/**"}, Where: "  \n", DenyCodeHint: "needs_dev"},
			wantErr: "deny_code_hint set without a where clause",
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
			name: "allow_encoded slash",
			rule: Rule{Paths: []string{"/api/**"}, AllowEncoded: []string{"/"}},
		},
		{
			name:    "allow_encoded other char",
			rule:    Rule{Paths: []string{"/api/**"}, AllowEncoded: []string{"%"}},
			wantErr: "allow_encoded allows only",
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

	var allowAllRule Rule
	require.NoError(t, yaml.Unmarshal([]byte(`allow_all: true`), &allowAllRule))
	require.Equal(t, Rule{AllowAll: true}, allowAllRule)
	require.NoError(t, allowAllRule.validate())
}

func TestWhereByteCap(t *testing.T) {
	atCap := `user.name == "x"` + strings.Repeat(" ", maxWhereBytes-len(`user.name == "x"`))
	require.Len(t, atCap, maxWhereBytes)
	require.NoError(t, Rule{Paths: []string{"/api/**"}, Where: atCap}.validate())

	err := Rule{Paths: []string{"/api/**"}, Where: atCap + " "}.validate()
	require.ErrorContains(t, err, "over the")

	_, err = CompileWhere(atCap)
	require.NoError(t, err)
	_, err = CompileWhere(atCap + " ")
	require.ErrorContains(t, err, "over the")
}

func TestReasonByteCap(t *testing.T) {
	atCap := strings.Repeat("x", maxReasonBytes)

	require.NoError(t, Rule{Paths: []string{"/api/**"}, AllowCode: "ok", AllowReason: atCap}.validate())
	err := Rule{Paths: []string{"/api/**"}, AllowCode: "ok", AllowReason: atCap + "x"}.validate()
	require.ErrorContains(t, err, "over the")

	require.NoError(t, Rule{Paths: []string{"/api/**"}, Where: "true", DenyCodeHint: "no", DenyReasonHint: atCap}.validate())
	err = Rule{Paths: []string{"/api/**"}, Where: "true", DenyCodeHint: "no", DenyReasonHint: atCap + "x"}.validate()
	require.ErrorContains(t, err, "over the")
}

func TestPathByteCap(t *testing.T) {
	atCap := "/" + strings.Repeat("a", maxPathBytes-1)
	require.Len(t, atCap, maxPathBytes)
	require.NoError(t, Rule{Paths: []string{atCap}}.validate())

	err := Rule{Paths: []string{atCap + "a"}}.validate()
	require.ErrorContains(t, err, "over the")
}

func TestPathCountCap(t *testing.T) {
	atCap := make([]string, maxPaths)
	for i := range atCap {
		atCap[i] = fmt.Sprintf("/api/%d/**", i)
	}
	require.NoError(t, Rule{Paths: atCap}.validate())

	err := Rule{Paths: append(atCap, "/api/over/**")}.validate()
	require.ErrorContains(t, err, "over the cap")
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

func TestPathTree(t *testing.T) {
	tests := []struct {
		pattern string
		want    Node
	}{
		{"/health", Literal("health")},
		{"/files/", Literal("files", Slash())},
		{"/api/*", Literal("api", Glob())},
		{"/api/**", Literal("api", Greedy())},
		{"/repo/{id}/log", Literal("repo", Capture("id", Literal("log")))},
		{"/files/!secret/**", Literal("files", GlobWithout([]string{"secret"}, Greedy()))},
		{"/", Slash()},
		{"/content/café.md", Literal("content", Literal("café.md"))},
		{"/a!b", Literal("a!b")},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			got, err := pathTree(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPathTreeErrors(t *testing.T) {
	tests := []struct {
		pattern string
		wantErr string
	}{
		{"health", "must start with /"},
		{"", "must start with /"},
		{"/api//v1", "empty segment"},
		{"//", "empty segment"},
		{"/api/**/x", "** must be the last segment"},
		{"/a*", "is not a literal"},
		{"/***", "is not a literal"},
		{"/!*", "is not a literal"},
		{"/!a*b", "is not a literal"},
		{"/!{b}", "is not a literal"},
		{"/{project:/}", "is not a literal"},
		{"/{bad name}", "must be a letter or underscore"},
		{"/{}", "must be a letter or underscore"},
		{"/!", "cannot be empty"},
		{"/!a%20b", "contains %"},
		{"/a%20b", "contains %"},
		{"/..", `"." or ".."`},
		{"/{v}/{v}", "already bound on this branch"},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			_, err := pathTree(tt.pattern)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, "path pattern")
		})
	}
}

func TestRuleCompileErrors(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		wantErr string
	}{
		{
			name:    "invalid rule",
			rule:    Rule{},
			wantErr: "must set paths or allow_all",
		},
		{
			name:    "where calls path.match",
			rule:    Rule{Paths: []string{"/api/**"}, Where: `path.match(literal("x"))`},
			wantErr: "unsupported function",
		},
		{
			name:    "where calls allow_code",
			rule:    Rule{Paths: []string{"/api/**"}, Where: `allow_code("c", "r", true)`},
			wantErr: "unsupported function",
		},
		{
			name:    "where does not parse",
			rule:    Rule{Paths: []string{"/api/**"}, Where: `user.name ==`},
			wantErr: "compiling where clause",
		},
		{
			name:    "capture missing from the only path",
			rule:    Rule{Paths: []string{"/health"}, Where: `vars.version == "v4"`},
			wantErr: `where reads vars.version, but path "/health" has no {version} capture`,
		},
		{
			name:    "capture bound by another path",
			rule:    Rule{Paths: []string{"/api/{version}/**", "/health"}, Where: `vars.version == "v4"`},
			wantErr: `where reads vars.version, but path "/health" has no {version} capture`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newCompiledRule(tt.rule)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRuleEvaluate(t *testing.T) {
	project := Rule{
		Paths:          []string{"/api/v4/projects/{project}/**"},
		Methods:        []string{"GET", "HEAD"},
		Where:          `contains(user.traits["projects"], vars.project)`,
		AllowCode:      "project_read",
		AllowReason:    "Read access.",
		DenyCodeHint:   "not_in_projects",
		DenyReasonHint: "Project not allowed.",
	}
	files := Rule{Paths: []string{"/files/!secret/**", "/files/"}}
	member := Identity{Traits: map[string][]string{"projects": {"42"}}}
	tests := []struct {
		name     string
		rule     Rule
		method   string
		path     string
		identity Identity
		want     Result
	}{
		{
			name:   "allow_all",
			rule:   Rule{AllowAll: true},
			method: "DELETE",
			path:   "/anything",
			want:   Result{Value: true},
		},
		{
			name:   "!secret matches public",
			rule:   files,
			method: "GET",
			path:   "/files/public/report.pdf",
			want:   Result{Value: true, vars: map[string]string{}},
		},
		{
			name:   "!secret rejects secret",
			rule:   files,
			method: "GET",
			path:   "/files/secret/report.pdf",
			want:   Result{},
		},
		{
			name:   "trailing slash alternative",
			rule:   files,
			method: "GET",
			path:   "/files/",
			want:   Result{Value: true, vars: map[string]string{}},
		},
		{
			name:     "allowed with capture and code",
			rule:     project,
			method:   "GET",
			path:     "/api/v4/projects/42/issues",
			identity: member,
			want: Result{
				Value:       true,
				AuditRecord: AuditRecord{AllowCode: "project_read", AllowReason: "Read access."},
				vars:        map[string]string{"project": "42"},
			},
		},
		{
			name:     "method fails before where",
			rule:     project,
			method:   "POST",
			path:     "/api/v4/projects/42/issues",
			identity: member,
			want:     Result{},
		},
		{
			name:     "path fails before where",
			rule:     project,
			method:   "GET",
			path:     "/health",
			identity: member,
			want:     Result{},
		},
		{
			name:     "where fails with hint",
			rule:     project,
			method:   "GET",
			path:     "/api/v4/projects/43/issues",
			identity: member,
			want: Result{
				AuditRecord: AuditRecord{DenyHints: []Hint{{Code: "not_in_projects", Reason: "Project not allowed."}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := newCompiledRule(tt.rule)
			require.NoError(t, err)
			env, err := NewEnv(Request{Method: tt.method, Path: tt.path}, tt.identity)
			require.NoError(t, err)
			got, err := evaluateExpression(compiled, env)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
