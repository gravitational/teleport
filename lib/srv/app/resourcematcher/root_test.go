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

// TestRootEval pins that a root node OR-s several first segments by matching
// each child against the same segment, consuming no token of its own.
func TestRootEval(t *testing.T) {
	root := mustNode(Root(
		Literal("api", Greedy()),
		Literal("admin", Greedy()),
		Literal("health"),
	))
	tests := []struct {
		path string
		want bool
	}{
		{"/api/v4/projects", true},
		{"/admin/users", true},
		{"/health", true},
		{"/health/live", false},
		{"/other/thing", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			tokens, err := Tokenize(tt.path)
			require.NoError(t, err)
			ok, _ := Eval(tokens, root)
			require.Equal(t, tt.want, ok)
		})
	}
}

// TestRootCaptures pins that a capture under one root branch binds when that
// branch matches.
func TestRootCaptures(t *testing.T) {
	root := mustNode(Root(
		Literal("projects", Capture("project", Greedy())),
		Literal("groups", Capture("group", Greedy())),
	))
	tokens, err := Tokenize("/projects/gitlab-org/tree")
	require.NoError(t, err)
	ok, vars := Eval(tokens, root)
	require.True(t, ok)
	require.Equal(t, "gitlab-org", vars["project"])
}

// TestRootConstructorEmpty pins that an empty root matches nothing and is
// rejected at construction.
func TestRootConstructorEmpty(t *testing.T) {
	_, err := Root()
	require.Error(t, err)
}

// TestRootSingleMatchRule pins the design goal: several first segments fold into
// one path.match through root(), instead of an || of separate matches.
func TestRootSingleMatchRule(t *testing.T) {
	compiled, err := compileExpression(`path.match(root(literal("api", greedy()), literal("admin", greedy())))`)
	require.NoError(t, err)

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/api/v4/projects", true},
		{"/admin/users", true},
		{"/other/thing", false},
	} {
		got, err := compiled.Evaluate(Request{Method: "GET", Path: tc.path}, Identity{})
		require.NoError(t, err)
		require.Equal(t, tc.want, got.Allowed, tc.path)
	}
}

// TestRootNestedIsNoopGrouping pins that root() carries no positional rule:
// a nested or doubled root() compiles and behaves the same as its children
// would in the parent's position. root() consumes no token and OR-s its
// children, so wrapping a child in another root() is a no-op grouping.
func TestRootNestedIsNoopGrouping(t *testing.T) {
	tests := []struct {
		name string
		pred string
		path string
		want bool
	}{
		{
			name: "root under a literal acts as alternation of continuations",
			pred: `path.match(literal("files", root(literal("a", greedy()), literal("b", greedy()))))`,
			path: "/files/a/x",
			want: true,
		},
		{
			name: "root under a literal misses when no alternative matches",
			pred: `path.match(literal("files", root(literal("a", greedy()), literal("b", greedy()))))`,
			path: "/files/c/x",
			want: false,
		},
		{
			name: "root under a root acts as a single root",
			pred: `path.match(root(root(literal("a", greedy())), literal("b", greedy())))`,
			path: "/a/x",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := compileExpression(tt.pred)
			require.NoError(t, err)
			got, err := compiled.Evaluate(Request{Method: "GET", Path: tt.path}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed)
		})
	}
}

// TestRootAtTopCompiles pins that a well-placed root, as the matcher argument of
// path.match, compiles.
func TestRootAtTopCompiles(t *testing.T) {
	_, err := compileExpression(`path.match(root(literal("api", greedy()), literal("admin", greedy())))`)
	require.NoError(t, err)
}

// TestRootNodeToSource pins the round-trip rendering of a root node.
func TestRootNodeToSource(t *testing.T) {
	node := mustNode(Root(Literal("api", Greedy()), Literal("admin", Greedy())))
	require.Equal(t, `root(literal("api", greedy()), literal("admin", greedy()))`, nodeToSource(node))
}
