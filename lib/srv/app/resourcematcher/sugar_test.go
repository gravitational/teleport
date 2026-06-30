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
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSugarEqualsConstructors pins that the sugared encoded and carve-out forms
// compile to the same tree their explicit constructors build, the same claim
// TestCompileEqualsConstructors makes for the base sugar. The encoded slash
// inside the braces must survive the brace-aware split as one segment.
func TestSugarEqualsConstructors(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    *Node
	}{
		{
			name:    "encoded capture binds a slashed segment",
			pattern: "/registry/{package:/}",
			want:    Literal("registry", mustNode(CaptureEncoded("package", []string{"/"}))),
		},
		{
			name:    "encoded capture then greedy",
			pattern: "/api/v4/projects/{project:/}/**",
			want:    Literal("api", Literal("v4", Literal("projects", mustNode(CaptureEncoded("project", []string{"/"}, Greedy()))))),
		},
		{
			name:    "anonymous encoded glob",
			pattern: "/registry/{:/}",
			want:    Literal("registry", mustNode(GlobEncoded([]string{"/"}))),
		},
		{
			name:    "carve-out excludes one segment and continues",
			pattern: "/files/!secret/**",
			want:    Literal("files", mustNode(GlobWithout([]string{"secret"}, Greedy()))),
		},
		{
			name:    "carve-out as the terminal segment",
			pattern: "/api/!admin",
			want:    Literal("api", mustNode(GlobWithout([]string{"admin"}))),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compile(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, got, "sugared string must equal the constructor tree")
		})
	}
}

// TestSugarErrors pins that the malformed encoded and carve-out forms are
// rejected at compile, so a typo fails closed rather than compiling a surprising
// matcher.
func TestSugarErrors(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"empty encoded set on a capture", "/registry/{package:}"},
		{"empty encoded set anonymous", "/registry/{:}"},
		{"unsupported encoded char", "/registry/{package:x}"},
		{"encoded capture name not an identifier", "/registry/{bad name:/}"},
		{"encoded capture name leading digit", "/registry/{1st:/}"},
		{"bare carve-out has no value", "/files/!/**"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(tt.pattern)
			require.Error(t, err)
		})
	}
}

// TestSugarEval pins the runtime semantics of the sugared forms end to end, so
// the brace-aware split and the new segment kinds decide a request the way the
// constructors do.
func TestSugarEval(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		want     bool
		wantVars map[string]string
	}{
		{
			name:     "encoded capture binds the decoded slashed id",
			pattern:  "/registry/{package:/}",
			path:     "/registry/@babel%2fcore",
			want:     true,
			wantVars: map[string]string{"package": "@babel/core"},
		},
		{
			name:     "encoded capture binds a plain id too",
			pattern:  "/registry/{package:/}",
			path:     "/registry/lodash",
			want:     true,
			wantVars: map[string]string{"package": "lodash"},
		},
		{
			name:    "anonymous encoded glob admits the slashed segment",
			pattern: "/registry/{:/}",
			path:    "/registry/@babel%2fcore",
			want:    true,
		},
		{
			name:    "carve-out denies the excluded segment",
			pattern: "/files/!secret/**",
			path:    "/files/secret/x",
			want:    false,
		},
		{
			name:    "carve-out admits any other segment",
			pattern: "/files/!secret/**",
			path:    "/files/report/x",
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := Compile(tt.pattern)
			require.NoError(t, err)
			tokens, err := Tokenize(tt.path)
			require.NoError(t, err)
			got, vars := Eval(tokens, node)
			require.Equal(t, tt.want, got)
			if tt.want && tt.wantVars != nil {
				require.Equal(t, tt.wantVars, vars)
			}
		})
	}
}

// TestSplitPattern pins that the brace-aware split keeps the encoded slash
// inside the braces as one segment while preserving the empty-segment semantics
// the base sugar relies on, so it is a drop-in for strings.Split on "/" outside
// braces.
func TestSplitPattern(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"files", []string{"files"}},
		{"files/", []string{"files", ""}},
		{"", []string{""}},
		{"/", []string{"", ""}},
		{"a/b/c", []string{"a", "b", "c"}},
		{"registry/{package:/}", []string{"registry", "{package:/}"}},
		{"api/v4/projects/{project:/}/x", []string{"api", "v4", "projects", "{project:/}", "x"}},
		{"registry/{:/}", []string{"registry", "{:/}"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			require.Equal(t, tt.want, splitPattern(tt.in))
		})
	}
}

// TestNodeToSourceRoundTripExclusions pins that nodeToSource renders a carve-out
// node back to a constructor that re-parses to the same node, the round-trip the
// sugar relies on. A glob_without renders as glob_without, not a plain glob, and
// a greedy carrying exclusions renders as greedy_except, so the exclusion is
// never silently dropped.
func TestNodeToSourceRoundTripExclusions(t *testing.T) {
	tests := []struct {
		name string
		node *Node
		want string
	}{
		{
			name: "glob_without with a child",
			node: mustNode(GlobWithout([]string{"secret"}, Greedy())),
			want: `glob_without(set("secret"), greedy())`,
		},
		{
			name: "glob_without multiple excludes terminal",
			node: mustNode(GlobWithout([]string{"private", "secret"})),
			want: `glob_without(set("private", "secret"))`,
		},
		{
			name: "greedy_without renders as greedy_except",
			node: mustNode(GreedyWithout("private", "secret")),
			want: `greedy_except(literal("private", greedy()), literal("secret", greedy()))`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeToSource(tt.node)
			require.Equal(t, tt.want, got)
			// The rendered source must re-parse cleanly inside a path.match, so the
			// exclusion survives the desugar round-trip rather than tripping the
			// validators. The parser yields a bool, not the node, so equality is
			// pinned by the rendered string above and by the golden behavioral
			// round-trip; this only proves the source is well-formed.
			_, err := compilePredicate("path.match(" + got + ")")
			require.NoError(t, err)
		})
	}
}

// TestEncodedNodeClause pins how an encoded-node pattern lowers: the {name:/}
// sugar becomes a capture_encoded node, the sole per-segment opt-in, with no
// rule-wide encoding option on path.match.
func TestEncodedNodeClause(t *testing.T) {
	r := Rule{Paths: []string{"/registry/{package:/}"}}
	got, err := r.pathClause()
	require.NoError(t, err)
	require.Equal(t,
		`path.match(literal("registry", capture_encoded("package", set("/"))))`,
		got)
}

// TestUnsafeAllowAll pins the escape hatch: it desugars to the constant true,
// allows every request, and bypasses even the tokenizer floor, so a path that a
// normal rule set would reject as malformed is still allowed. This last part is
// a property of the rule set, not the lowered predicate, so it cannot be a
// golden case (the desugared true would be denied by the floor); it is pinned
// here instead.
func TestUnsafeAllowAll(t *testing.T) {
	expr, err := Rule{UnsafeAllowAll: true}.desugar()
	require.NoError(t, err)
	require.Equal(t, "true", expr)

	set, err := CompileRoles([]Role{{Name: "r", Resources: []Rule{{UnsafeAllowAll: true}}}})
	require.NoError(t, err)

	// Every well-formed request is allowed, regardless of method or path.
	for _, path := range []string{"/anything", "/a/b/c", "/api/v4/projects/g%2Fp/issues"} {
		got, err := set.Evaluate(Request{Method: "DELETE", Path: path}, Identity{})
		require.NoError(t, err)
		require.True(t, got.Allowed, "path %q", path)
	}

	// A path the tokenizer rejects as malformed (a "." segment, consecutive
	// slashes, or a non-slash percent-escape) is still allowed: unsafe_allow_all
	// turns the floor off for the whole set.
	for _, path := range []string{"/a/../b", "/a//b", "/a/%20/b"} {
		got, err := set.Evaluate(Request{Method: "GET", Path: path}, Identity{})
		require.NoError(t, err)
		require.True(t, got.Allowed, "malformed path %q should still be allowed", path)
	}

	// Without unsafe_allow_all, the same malformed path is an invalid request.
	safe, err := CompileRoles([]Role{{Name: "r", Resources: []Rule{{Paths: []string{"/**"}}}}})
	require.NoError(t, err)
	got, err := safe.Evaluate(Request{Method: "GET", Path: "/a/../b"}, Identity{})
	require.NoError(t, err)
	require.False(t, got.Allowed)
	require.Equal(t, DenyInvalidRequest, got.Deny.Kind)
}

// TestUnsafeAllowAllStandsAlone pins that unsafe_allow_all is all-or-nothing: it
// cannot be combined with any other field, and a rule that sets neither paths
// nor unsafe_allow_all is rejected.
func TestUnsafeAllowAllStandsAlone(t *testing.T) {
	combos := []Rule{
		{UnsafeAllowAll: true, Paths: []string{"/api"}},
		{UnsafeAllowAll: true, Methods: []string{"GET"}},
		{UnsafeAllowAll: true, Where: `contains(user.roles, "admin")`},
		{UnsafeAllowAll: true, AllowCode: "x"},
		{UnsafeAllowAll: true, DenyCodeHint: "x"},
	}
	for i, r := range combos {
		_, err := r.Compile()
		require.Error(t, err, "combo %d should be rejected", i)
	}

	// A rule that scopes nothing and does not opt into everything is rejected.
	_, err := Rule{Methods: []string{"GET"}}.Compile()
	require.Error(t, err)
	_, err = Rule{Where: `contains(user.roles, "admin")`}.Compile()
	require.Error(t, err)
}

// TestValidateMethods pins that a rule's methods are checked against the
// standard HTTP methods, case-insensitively, so a typo is a load error while a
// lower-cased standard method is accepted.
func TestValidateMethods(t *testing.T) {
	_, err := Rule{Paths: []string{"/api"}, Methods: []string{"GET", "post", "PATCH"}}.Compile()
	require.NoError(t, err)

	for _, m := range []string{"GTE", "FETCH", "get ", ""} {
		_, err := Rule{Paths: []string{"/api"}, Methods: []string{m}}.Compile()
		require.Error(t, err, "method %q should be rejected", m)
	}
}
