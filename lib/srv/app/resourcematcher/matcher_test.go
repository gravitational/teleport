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

// TestCompileEqualsConstructors pins the central design claim: the declarative
// string surface and the explicit constructor surface compile to one identical
// internal representation. The string is pure sugar over the constructor tree,
// so the two cannot diverge.
func TestCompileEqualsConstructors(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    *Node
	}{
		{
			name:    "literal chain",
			pattern: "/api/v4/projects",
			want:    Literal("api", Literal("v4", Literal("projects"))),
		},
		{
			name:    "capture and greedy",
			pattern: "/api/v4/projects/{project}/**",
			want:    Literal("api", Literal("v4", Literal("projects", Capture("project", Greedy())))),
		},
		{
			name:    "glob mid-path",
			pattern: "/api/*/repo",
			want:    Literal("api", Glob(Literal("repo"))),
		},
		{
			name:    "multi-segment literal helper equals single segments",
			pattern: "/a/b/c",
			want:    Literal("a/b/c"),
		},
		{
			name:    "sub-delims and pchar punctuation are legal literals",
			pattern: "/api/(group)/a:b@c",
			want:    Literal("api", Literal("(group)", Literal("a:b@c"))),
		},
		{
			name:    "trailing slash compiles to a slash node",
			pattern: "/api/v4/health/",
			want:    Literal("api", Literal("v4", Literal("health", Slash()))),
		},
		{
			name:    "bare root compiles to a slash node",
			pattern: "/",
			want:    Slash(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compile(tt.pattern)
			require.NoError(t, err)
			require.Equal(t, tt.want, got, "compiled string must equal the constructor tree")
		})
	}
}

// TestLiteralSplitEquivalence pins that the multi-segment Literal helper is
// exactly the nested single-segment form.
func TestLiteralSplitEquivalence(t *testing.T) {
	require.Equal(t,
		Literal("a", Literal("b", Literal("c", Greedy()))),
		Literal("a/b/c", Greedy()),
	)
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"no leading slash", "api/v4"},
		{"greedy not last", "/api/**/repo"},
		{"interior empty segment", "/api//v4"},
		{"leading empty segment", "//api"},
		{"bare double slash", "//"},
		{"empty capture name", "/api/{}"},
		{"triple star", "/api/v4/sd/{project}/***"},
		{"star suffix", "/api/v4/*x"},
		{"star prefix", "/api/v4/x*"},
		{"star inside segment", "/api/v4/a*b"},
		{"stray open brace", "/api/{project"},
		{"stray close brace", "/api/project}"},
		{"brace and text", "/api/a{project}"},
		{"nested braces", "/api/{{project}}"},
		{"illegal byte angle bracket", "/api/<"},
		{"illegal byte space", "/api/a b"},
		{"illegal byte pipe", "/api/a|b"},
		{"illegal byte backslash", "/api/a\\b"},
		{"illegal byte non-ascii", "/api/café"},
		{"capture name with dash", "/api/{a-b}"},
		{"capture name with space", "/api/{a b}"},
		{"capture name leading digit", "/api/{1st}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(tt.pattern)
			require.Error(t, err)
		})
	}
}

func TestEval(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		want     bool
		wantVars map[string]string
	}{
		{"exact match", "/foo", "/foo", true, map[string]string{}},
		{"trailing slash differs", "/foo", "/foo/", false, nil},
		{"trailing slash pattern matches trailing slash path", "/api/v4/health/", "/api/v4/health/", true, map[string]string{}},
		{"trailing slash pattern rejects bare path", "/api/v4/health/", "/api/v4/health", false, nil},
		{"bare root matches root path", "/", "/", true, map[string]string{}},
		{"bare root rejects non-root path", "/", "/foo", false, nil},
		{"glob one segment", "/foo/*", "/foo/bar", true, map[string]string{}},
		{"glob rejects empty", "/foo/*", "/foo/", false, nil},
		{"glob no extra segment", "/foo/*", "/foo/bar/baz", false, nil},
		{"greedy matches zero", "/foo/**", "/foo", true, map[string]string{}},
		{"greedy matches many", "/foo/**", "/foo/bar/baz", true, map[string]string{}},
		{"capture binds", "/foo/{x}", "/foo/bar", true, map[string]string{"x": "bar"}},
		{
			name:     "capture then greedy",
			pattern:  "/api/v4/projects/{project}/**",
			path:     "/api/v4/projects/gitlab-org/repository/tree",
			want:     true,
			wantVars: map[string]string{"project": "gitlab-org"},
		},
		{
			name:    "literal mismatch",
			pattern: "/api/v4/projects/{project}/**",
			path:    "/api/v4/groups/gitlab-org",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := Compile(tt.pattern)
			require.NoError(t, err)
			tokens, err := Tokenize(tt.path)
			require.NoError(t, err)
			ok, vars := Eval(tokens, root)
			require.Equal(t, tt.want, ok)
			require.Equal(t, tt.wantVars, vars)
		})
	}
}

// TestAlternatives pins that several children at one node branch into
// OR-ed alternatives, so two patterns sharing a prefix combine into one tree.
func TestAlternatives(t *testing.T) {
	root := Literal("api", Literal("v4",
		Literal("projects", Greedy()),
		Literal("groups", Greedy()),
	))
	for _, path := range []string{"/api/v4/projects/x", "/api/v4/groups/y"} {
		tokens, err := Tokenize(path)
		require.NoError(t, err)
		ok, _ := Eval(tokens, root)
		require.True(t, ok, path)
	}
	tokens, err := Tokenize("/api/v4/users/z")
	require.NoError(t, err)
	ok, _ := Eval(tokens, root)
	require.False(t, ok)
}
