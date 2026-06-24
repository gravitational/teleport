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
			name:     "encoded capture binds the raw slashed id",
			pattern:  "/registry/{package:/}",
			path:     "/registry/@babel%2fcore",
			want:     true,
			wantVars: map[string]string{"package": "@babel%2fcore"},
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

// TestAllowEncodedClause pins how the allow_encoded field lowers: a rule with an
// encoded node and allow_encoded emits the option on path.match, and the field
// with no paths to gate is a load error.
func TestAllowEncodedClause(t *testing.T) {
	r := Rule{Paths: []string{"/registry/{package:/}"}, AllowEncoded: []string{"/"}}
	got, err := r.pathClause()
	require.NoError(t, err)
	require.Equal(t,
		`path.match(literal("registry", capture_encoded("package", set("/"))), allow_encoded(set("/")))`,
		got)

	_, err = Rule{AllowEncoded: []string{"/"}}.pathClause()
	require.Error(t, err)

	_, err = Rule{Paths: []string{"/x"}, AllowEncoded: []string{"x"}}.pathClause()
	require.Error(t, err)
}
