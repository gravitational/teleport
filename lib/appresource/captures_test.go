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

package appresource

import (
	"go/parser"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/set"
)

// TestCheckVars checks the capture names checkVars guarantees bound and the
// reads it rejects.
func TestCheckVars(t *testing.T) {
	tests := []struct {
		expr    string
		bound   []string
		want    []string
		wantErr string
	}{
		// A match guarantees its captures.
		{expr: `path.match(capture("p"))`, want: []string{"p"}},
		{expr: `path.match(literal("a", capture("p", capture("q"))))`, want: []string{"p", "q"}},
		// optional guarantees nothing below it.
		{expr: `path.match(literal("a", capture("p", optional(capture("q")))))`, want: []string{"p"}},
		// && accumulates, || intersects, ! and == guarantee nothing.
		{expr: `path.match(capture("p")) && path.match(glob(capture("q")))`, want: []string{"p", "q"}},
		{expr: `path.match(capture("p", capture("q"))) || path.match(capture("p"))`, want: []string{"p"}},
		{expr: `path.match(capture("p")) || path.match(slash())`},
		{expr: `!path.match(capture("p"))`},
		{expr: `path.match(capture("p")) == true`},
		// The audit wrappers are transparent.
		{expr: `allow_code("c", "r", path.match(capture("p")))`, want: []string{"p"}},
		// A read is valid after its bind, or with the name seeded in bound.
		{expr: `path.match(capture("p")) && vars.p == "x"`, want: []string{"p"}},
		{expr: `vars.p == "x"`, bound: []string{"p"}},
		// A read before, without, or on only one branch of its bind fails.
		{expr: `vars.p == "x"`, wantErr: "vars.p is read before"},
		{expr: `vars.p == "x" && path.match(capture("p"))`, wantErr: "vars.p is read before"},
		{expr: `(path.match(capture("p")) || path.match(slash())) && vars.p == "x"`, wantErr: "vars.p is read before"},
		{expr: `!path.match(capture("p")) && vars.p == "x"`, wantErr: "vars.p is read before"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			parsed, err := parser.ParseExpr(tt.expr)
			require.NoError(t, err)
			got, err := checkVars(parsed, set.New(tt.bound...))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, slices.Sorted(slices.Values(got.Elements())))
		})
	}
}
