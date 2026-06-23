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

// TestOptionalEval pins the runtime semantics of optional(): the path may end
// at the node, or one of its children matches the remainder. optional(slash())
// recovers the old optional-slash behavior; an optional subtree makes a
// trailing tail optional from one tree with no duplicated prefix; several
// children are alternatives.
func TestOptionalEval(t *testing.T) {
	tests := []struct {
		name string
		root *Node
		path string
		want bool
	}{
		{"optional slash matches the bare path", Literal("files", mustNode(Optional(Slash()))), "/files", true},
		{"optional slash matches the trailing slash", Literal("files", mustNode(Optional(Slash()))), "/files/", true},
		{"optional slash rejects a deeper path", Literal("files", mustNode(Optional(Slash()))), "/files/x", false},

		{"optional subtree matches the bare prefix", Literal("files", mustNode(Optional(Literal("reports")))), "/files", true},
		{"optional subtree matches the full path", Literal("files", mustNode(Optional(Literal("reports")))), "/files/reports", true},
		{"optional subtree rejects a different tail", Literal("files", mustNode(Optional(Literal("reports")))), "/files/other", false},
		{"optional subtree rejects a deeper tail", Literal("files", mustNode(Optional(Literal("reports")))), "/files/reports/x", false},

		{"optional greedy subtree matches the bare prefix", Literal("files", mustNode(Optional(Literal("reports", Greedy())))), "/files", true},
		{"optional greedy subtree matches a deep tail", Literal("files", mustNode(Optional(Literal("reports", Greedy())))), "/files/reports/anything", true},
		{"optional greedy subtree rejects a different tail", Literal("files", mustNode(Optional(Literal("reports", Greedy())))), "/files/other", false},

		{"optional alternatives match the bare prefix", Literal("files", mustNode(Optional(Literal("a"), Literal("b")))), "/files", true},
		{"optional alternatives match the first", Literal("files", mustNode(Optional(Literal("a"), Literal("b")))), "/files/a", true},
		{"optional alternatives match the second", Literal("files", mustNode(Optional(Literal("a"), Literal("b")))), "/files/b", true},
		{"optional alternatives reject a third", Literal("files", mustNode(Optional(Literal("a"), Literal("b")))), "/files/c", false},
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

// TestOptionalRule pins optional() through the predicate surface, the slashed
// and unslashed request and the optional subtree both decided from one tree.
func TestOptionalRule(t *testing.T) {
	tests := []struct {
		name string
		pred string
		path string
		want bool
	}{
		{"optional slash matches the bare path", `path.match(literal("files", optional(slash())))`, "/files", true},
		{"optional slash matches the trailing slash", `path.match(literal("files", optional(slash())))`, "/files/", true},
		{"optional slash rejects a deeper path", `path.match(literal("files", optional(slash())))`, "/files/secret", false},
		{"optional subtree matches the bare prefix", `path.match(literal("files", optional(literal("reports"))))`, "/files", true},
		{"optional subtree matches the full path", `path.match(literal("files", optional(literal("reports"))))`, "/files/reports", true},
		{"optional subtree rejects a different tail", `path.match(literal("files", optional(literal("reports"))))`, "/files/other", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := Rule{Pred: tt.pred}.Compile()
			require.NoError(t, err)
			got, err := compiled.Evaluate(Request{Method: "GET", Path: tt.path}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed)
		})
	}
}

// TestOptionalNoChildren pins that an optional with no children is rejected at
// construction, since an empty optional has no subtree to make optional. Like
// the empty root, the matcher constructors run at evaluation rather than at
// compile, so the constructor is where the error surfaces.
func TestOptionalNoChildren(t *testing.T) {
	_, err := Optional()
	require.Error(t, err)
}

// TestOptionalCaptureNotGuaranteed pins that a capture inside an optional is
// never guaranteed, so a rule that reads it is a load error: the empty-match
// branch leaves the capture unbound on the path that ends early.
func TestOptionalCaptureNotGuaranteed(t *testing.T) {
	_, err := Rule{Pred: `path.match(literal("files", optional(capture("x")))) && vars.x == "y"`}.Compile()
	require.Error(t, err)
}

// TestOptionalInGreedyExceptRejected pins the load error for an optional inside
// a greedy_except exclusion. Its empty-match branch makes the exclusion match
// the zero-length tail, which refuses greedy's match-zero and silently forbids
// the bare prefix, so the rule fails to compile rather than wrongly denying.
func TestOptionalInGreedyExceptRejected(t *testing.T) {
	_, err := Rule{Pred: `path.match(literal("files", greedy_except(optional(literal("secret")))))`}.Compile()
	require.Error(t, err)

	// The constructor enforces the same rule as a backstop for a direct Go
	// caller building the tree without the string surface.
	_, err = GreedyExcept(mustNode(Optional(Literal("secret"))))
	require.Error(t, err)
}

// TestOptionalNodeToSource pins the round-trip rendering of optional().
func TestOptionalNodeToSource(t *testing.T) {
	require.Equal(t, `literal("files", optional(slash()))`,
		nodeToSource(Literal("files", mustNode(Optional(Slash())))))
	require.Equal(t, `literal("files", optional(literal("reports")))`,
		nodeToSource(Literal("files", mustNode(Optional(Literal("reports"))))))
}
