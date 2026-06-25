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

// TestLowerUpper pins the lower and upper string helpers, used to compare
// case-insensitively in a where clause. lower folds a value to lower case and
// upper to upper case, so a membership test matches regardless of the request's
// case.
func TestLowerUpper(t *testing.T) {
	lowered, err := compileExpression(
		`path.match(greedy()) && contains(set("get", "head"), lower(request.method))`)
	require.NoError(t, err)

	uppered, err := compileExpression(
		`path.match(greedy()) && contains(set("GET", "HEAD"), upper(request.method))`)
	require.NoError(t, err)

	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"get", true},
		{"GeT", true},
		{"DELETE", false},
		{"delete", false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got, err := lowered.Evaluate(Request{Method: tt.method, Path: "/anything"}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed)

			got, err = uppered.Evaluate(Request{Method: tt.method, Path: "/anything"}, Identity{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Allowed)
		})
	}
}
