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

// mustNode panics if a matcher constructor returns an error, so a table can
// build trees inline with the f(g()) call form. A panic fails the test.
func mustNode(n *Node, err error) *Node {
	if err != nil {
		panic(err)
	}
	return n
}

// TestCarveOutEval pins the runtime semantics of the three carve-out
// constructors. Each folds a deny into one matcher tree, so a path that the
// exclusion covers no longer matches even though the surrounding glob or greedy
// would otherwise admit it.
func TestCarveOutEval(t *testing.T) {
	tests := []struct {
		name string
		root *Node
		path string
		want bool
	}{
		// greedy_without excludes the <value>/** subtrees by first segment.
		{
			name: "greedy_without admits an unrelated tail",
			root: Literal("files", mustNode(GreedyWithout("secret", "private"))),
			path: "/files/docs/report",
			want: true,
		},
		{
			name: "greedy_without denies the excluded subtree",
			root: Literal("files", mustNode(GreedyWithout("secret", "private"))),
			path: "/files/secret/report",
			want: false,
		},
		{
			name: "greedy_without denies the excluded segment itself",
			root: Literal("files", mustNode(GreedyWithout("secret"))),
			path: "/files/secret",
			want: false,
		},
		{
			name: "greedy_without excludes only the first tail segment",
			root: Literal("files", mustNode(GreedyWithout("secret"))),
			path: "/files/docs/secret",
			want: true,
		},
		{
			name: "greedy_without admits the bare prefix with no tail",
			root: Literal("files", mustNode(GreedyWithout("secret"))),
			path: "/files",
			want: true,
		},
		// greedy_except scopes the exclusion by the matcher's terminal-ness.
		{
			name: "greedy_except exact denies only the exact segment",
			root: Literal("files", mustNode(GreedyExcept(Literal("secret")))),
			path: "/files/secret",
			want: false,
		},
		{
			name: "greedy_except exact admits a deeper path under the segment",
			root: Literal("files", mustNode(GreedyExcept(Literal("secret")))),
			path: "/files/secret/public",
			want: true,
		},
		{
			name: "greedy_except subtree denies the whole subtree",
			root: Literal("files", mustNode(GreedyExcept(Literal("secret", Greedy())))),
			path: "/files/secret/public",
			want: false,
		},
		// glob_without excludes single segment values and continues.
		{
			name: "glob_without admits an unexcluded segment",
			root: Literal("files", mustNode(GlobWithout([]string{"secret", "private"}))),
			path: "/files/docs",
			want: true,
		},
		{
			name: "glob_without denies an excluded segment",
			root: Literal("files", mustNode(GlobWithout([]string{"secret", "private"}))),
			path: "/files/private",
			want: false,
		},
		{
			name: "glob_without matches one segment only",
			root: Literal("files", mustNode(GlobWithout([]string{"secret"}))),
			path: "/files/docs/report",
			want: false,
		},
		{
			name: "glob_without continues to children",
			root: Literal("files", mustNode(GlobWithout([]string{"secret"}, Literal("readme")))),
			path: "/files/docs/readme",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Tokenize(tt.path)
			require.NoError(t, err)
			ok, _ := Eval(tokens, tt.root)
			require.Equal(t, tt.want, ok)
		})
	}
}

// TestGreedyWithoutEqualsGreedyExcept pins that the string sugar collapses to
// the matcher form: greedy_without(s) is exactly greedy_except(literal(s,
// greedy())), so the two surfaces cannot diverge.
func TestGreedyWithoutEqualsGreedyExcept(t *testing.T) {
	without := mustNode(GreedyWithout("secret"))
	except := mustNode(GreedyExcept(Literal("secret", Greedy())))
	require.Equal(t, except, without)
}

// TestCarveOutConstructorErrors pins the load-time guards: an empty excluded
// value is dead under the strict URL rules, and a capture inside an exclusion
// can never be read, so both are rejected at construction.
func TestCarveOutConstructorErrors(t *testing.T) {
	_, err := GreedyWithout("")
	require.Error(t, err)

	_, err = GlobWithout([]string{""})
	require.Error(t, err)

	_, err = GreedyExcept(Capture("x"))
	require.Error(t, err)

	_, err = GreedyExcept(Literal("dir", Capture("x")))
	require.Error(t, err)
}

// TestCarveOutCapturesDoNotLeak pins that a capture bound inside an exclusion
// never reaches the result map. The exclusion is a negative test, so even when
// it inspects a segment it must not bind it. (The constructor rejects this at
// load; the tree is built by hand here to exercise the evaluator directly.)
func TestCarveOutCapturesDoNotLeak(t *testing.T) {
	root := Literal("files", &Node{
		kind:         kindGreedy,
		greedyExcept: []*Node{Capture("leaked", Greedy())},
	})
	tokens, err := Tokenize("/files/secret")
	require.NoError(t, err)
	ok, vars := Eval(tokens, root)
	// The exclusion matches "secret", so the path is denied, and no capture
	// leaks out of the failed negative branch.
	require.False(t, ok)
	require.Nil(t, vars)
}

// TestCarveOutSingleMatchRule pins the design goal: a carve-out needs only one
// path.match, with no negated second match. The rule allows /files/** and
// denies the /files/secret/** subtree, all from one positive call, so the rule
// stays on one decode policy and dodges the fail-open inversion a negated match
// carries.
func TestCarveOutSingleMatchRule(t *testing.T) {
	rule := Rule{
		Pred: `path.match(literal("files", greedy_without("secret")))`,
	}
	compiled, err := rule.Compile()
	require.NoError(t, err)

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/files/docs/report", true},
		{"/files", true},
		{"/files/secret", false},
		{"/files/secret/report", false},
	} {
		got, err := compiled.Evaluate(Request{Method: "GET", Path: tc.path}, Identity{})
		require.NoError(t, err)
		require.Equal(t, tc.want, got.Allowed, tc.path)
	}
}

// TestCarveOutGlobWithoutRule pins glob_without through the predicate surface,
// excluding single segment values via set().
func TestCarveOutGlobWithoutRule(t *testing.T) {
	rule := Rule{
		Pred: `path.match(literal("files", glob_without(set("secret", "private"))))`,
	}
	compiled, err := rule.Compile()
	require.NoError(t, err)

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/files/docs", true},
		{"/files/secret", false},
		{"/files/private", false},
		{"/files/docs/extra", false},
	} {
		got, err := compiled.Evaluate(Request{Method: "GET", Path: tc.path}, Identity{})
		require.NoError(t, err)
		require.Equal(t, tc.want, got.Allowed, tc.path)
	}
}

// TestCarveOutRejectsCaptureInException pins the load-time rejection: a capture
// inside a greedy_except is a deny test that binds nothing, so a rule that
// writes one is rejected when the rule is compiled, not silently at evaluation.
func TestCarveOutRejectsCaptureInException(t *testing.T) {
	_, err := Rule{
		Pred: `path.match(literal("files", greedy_except(capture("x", greedy()))))`,
	}.Compile()
	require.Error(t, err)
}
