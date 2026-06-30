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

// TestMatch pins the friendly Match wrapper: it tokenizes the path and walks
// the tree, returning the match verdict and any captures in one struct. It is
// the standalone parallel to the path.match predicate, so an encoded char is
// admitted only by an encoded node, a plain node fails closed on it, and a path
// the tokenizer rejects does not match.
func TestMatch(t *testing.T) {
	tests := []struct {
		name     string
		root     *Node
		path     string
		want     bool
		captures map[string]string
	}{
		{
			name: "literal with a greedy tail, no pre-tokenizing",
			root: Literal("ab/c", Greedy()),
			path: "/ab/c/",
			want: true,
		},
		{
			name: "encoded slash admitted by the encoded node",
			root: Literal("p", mustNode(GlobEncoded([]string{"/"}))),
			path: "/p/a%2Fb",
			want: true,
		},
		{
			name: "encoded slash fails closed against a plain glob",
			root: Literal("p", Glob()),
			path: "/p/a%2Fb",
			want: false,
		},
		{
			name:     "captures are returned",
			root:     Literal("user", Capture("name")),
			path:     "/user/bob",
			want:     true,
			captures: map[string]string{"name": "bob"},
		},
		{
			name: "consecutive slashes are a tokenize error",
			root: Literal("a", Literal("b")),
			path: "/a//b",
			want: false,
		},
		{
			name: "a non-separator percent-escape is a tokenize error",
			root: Literal("a"),
			path: "/a%2Eb",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.root, tt.path)
			require.Equal(t, tt.want, got.Matched)
			if tt.want {
				require.Equal(t, tt.captures, nilIfEmpty(got.Captures))
			} else {
				require.Nil(t, got.Captures)
			}
		})
	}
}

// nilIfEmpty maps an empty capture map to nil, so a match that bound nothing
// compares equal to a test case that sets no captures.
func nilIfEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}
