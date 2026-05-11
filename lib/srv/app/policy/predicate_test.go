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

package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompilePredicate(t *testing.T) {
	tests := []struct {
		name string
		expr string
		env  Env
		want bool
		err  bool
	}{
		{
			name: "path equality",
			expr: `path.username == user.name`,
			env:  Env{UserName: "alice", Path: map[string]string{"username": "alice"}},
			want: true,
		},
		{
			name: "path inequality",
			expr: `path.username == user.name`,
			env:  Env{UserName: "alice", Path: map[string]string{"username": "bob"}},
			want: false,
		},
		{
			name: "contains role",
			expr: `contains(user.roles, "admin")`,
			env:  Env{UserRoles: []string{"admin", "user"}},
			want: true,
		},
		{
			name: "request method check",
			expr: `request.method == "GET"`,
			env:  Env{RequestMethod: "GET"},
			want: true,
		},
		{
			name: "compound expression",
			expr: `path.username == user.name && contains(user.roles, "engineer")`,
			env: Env{
				UserName:  "alice",
				UserRoles: []string{"engineer"},
				Path:      map[string]string{"username": "alice"},
			},
			want: true,
		},
		{
			name: "rejects unknown variable",
			expr: `path.username == user.naem`,
			err:  true,
		},
		{
			name: "regexp.match literal pattern",
			expr: `regexp.match("^GET$", request.method)`,
			env:  Env{RequestMethod: "GET"},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := CompilePredicate(tc.expr)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, err := p.Evaluate(tc.env)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRegexpMatch_RejectsRuntimePattern(t *testing.T) {
	_, err := CompilePredicate(`regexp.match(path.x, "anything")`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "string literal")
}

func TestRegexpMatch_RejectsInvalidPattern(t *testing.T) {
	_, err := CompilePredicate(`regexp.match("[", request.method)`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compiling pattern")
}

func TestValidateRegexpMatchCalls_NoPanic(t *testing.T) {
	cases := []string{
		`regexp.match(", user.name)`,
		`regexp.match(  , user.name)`,
		`regexp.match()`,
		`regexp.match("`,
	}
	for _, expr := range cases {
		t.Run(expr, func(t *testing.T) {
			_, err := CompilePredicate(expr)
			require.Error(t, err)
		})
	}
}

func TestRegexpMatch_LiteralWithCommasAndParens(t *testing.T) {
	for _, expr := range []string{
		`regexp.match("a,b", request.method)`,
		`regexp.match("a(b|c)d", request.method)`,
		`regexp.match("escaped \"quote\"", request.method)`,
	} {
		t.Run(expr, func(t *testing.T) {
			p, err := CompilePredicate(expr)
			require.NoError(t, err)
			_, err = p.Evaluate(Env{RequestMethod: "x"})
			require.NoError(t, err)
		})
	}
}
