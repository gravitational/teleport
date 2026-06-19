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

// TestNegatedPathMatchFailsClosed pins the env-level tokenize guard. A negated
// path.match against a path the rule's decode cannot tokenize must fail closed,
// not open: the path is unreadable, so the rule does not match, even though the
// boolean would have been true. A readable non-matching path still negates
// normally, and a readable matching path is still excluded.
func TestNegatedPathMatchFailsClosed(t *testing.T) {
	rule := Rule{Pred: `!path.match(literal("admin", greedy()))`}
	compiled, err := rule.Compile()
	require.NoError(t, err)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			// Strict default decode cannot tokenize the percent byte, so the
			// path is unreadable and the rule fails closed rather than letting
			// the negation turn the failure into an allow.
			name: "encoded path the decode cannot read fails closed",
			path: "/adm%69n",
			want: false,
		},
		{
			// A readable path that does not match admin negates to allow.
			name: "readable non-matching path allows",
			path: "/files/report",
			want: true,
		},
		{
			// A readable path that matches admin is excluded by the negation.
			name: "readable matching path denies",
			path: "/admin/users",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compiled.Evaluate(Request{Method: "GET", Path: tt.path}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed)
		})
	}
}

// TestNoPathMatchSkipsTokenize pins the laziness. A rule with no path.match
// never tokenizes the path, so an otherwise unreadable path does not block an
// identity-only rule from matching.
func TestNoPathMatchSkipsTokenize(t *testing.T) {
	rule := Rule{Pred: `contains(user.roles, "admin")`}
	compiled, err := rule.Compile()
	require.NoError(t, err)

	// The path would fail to tokenize under the strict default decode, but the
	// rule never references it, so it is never tokenized and the rule matches on
	// identity alone.
	got, err := compiled.Evaluate(
		Request{Method: "GET", Path: "/adm%69n"},
		Identity{Roles: []string{"admin"}},
	)
	require.NoError(t, err)
	require.True(t, got.Allowed)
}
