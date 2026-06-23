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

// TestSlashEval pins the trailing-slash terminal. slash() requires the trailing
// empty segment a final "/" produces and admits no further segment.
func TestSlashEval(t *testing.T) {
	tests := []struct {
		name string
		root *Node
		path string
		want bool
	}{
		{"slash requires the trailing slash", Literal("files", Slash()), "/files/", true},
		{"slash rejects the bare path", Literal("files", Slash()), "/files", false},
		{"slash rejects a deeper path", Literal("files", Slash()), "/files/x", false},
		{"slash alone matches the bare root", Slash(), "/", true},
		{"slash alone rejects a non-root", Slash(), "/files", false},
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

// TestEmptyLiteralRejected pins that an empty literal segment is rejected on
// both authoring surfaces, since slash() now owns the trailing slash. An
// illegal byte is likewise rejected through the predicate literal(), closing
// the gap where only the string surface validated bytes.
func TestEmptyLiteralRejected(t *testing.T) {
	predTests := []struct {
		name string
		pred string
	}{
		{"empty literal", `path.match(literal(""))`},
		{"trailing slash inside a literal", `path.match(literal("files/"))`},
		{"interior empty segment", `path.match(literal("a//b"))`},
		{"illegal byte", `path.match(literal("a<b"))`},
	}
	for _, tt := range predTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Rule{Pred: tt.pred}.Compile()
			require.Error(t, err)
		})
	}

	// The string surface rejects the same empty interior segment.
	_, err := Compile("/a//b")
	require.Error(t, err)
}

// TestLiteralPanicsOnEmpty pins the constructor backstop: a direct Go caller
// that builds an empty literal segment has a bug, so Literal panics rather than
// build a node that can never match.
func TestLiteralPanicsOnEmpty(t *testing.T) {
	require.Panics(t, func() { Literal("") })
	require.Panics(t, func() { Literal("files/") })
}

// TestSlashNodeToSource pins the round-trip rendering of the trailing-slash
// terminal.
func TestSlashNodeToSource(t *testing.T) {
	require.Equal(t, `literal("files", slash())`, nodeToSource(Literal("files", Slash())))
}
