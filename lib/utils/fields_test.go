/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFields(t *testing.T) {
	t.Parallel()

	now := time.Now().Round(time.Minute)

	sliceString := []string{"test", "string", "slice"}
	sliceInterface := []any{"test", "string", "slice"}
	f := Fields{
		"one":      1,
		"name":     "vincent",
		"time":     now,
		"strings":  sliceString,
		"strings2": sliceInterface,
	}

	require.Equal(t, 1, f.GetInt("one"))
	require.Equal(t, 0, f.GetInt("two"))
	require.Equal(t, "vincent", f.GetString("name"))
	require.Empty(t, f.GetString("city"))
	require.Equal(t, now, f.GetTime("time"))
	require.Equal(t, sliceString, f.GetStrings("strings"))
	require.Equal(t, sliceString, f.GetStrings("strings2"))
	require.Nil(t, f.GetStrings("strings3"))
}

func TestToFieldsCondition(t *testing.T) {
	t.Parallel()

	// !equals(login, "root") && contains(participants, "test-user")
	expr := &types.WhereExpr{And: types.WhereExpr2{
		L: &types.WhereExpr{Not: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}}},
		R: &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: "test-user"}}},
	}}

	cond, err := ToFieldsCondition(expr)
	require.NoError(t, err)

	require.False(t, cond(Fields{}))
	require.False(t, cond(Fields{"login": "root", "participants": []string{"test-user", "observer"}}))
	require.False(t, cond(Fields{"login": "guest", "participants": []string{"another-user"}}))
	require.True(t, cond(Fields{"login": "guest", "participants": []string{"test-user", "observer"}}))
	require.True(t, cond(Fields{"participants": []string{"test-user"}}))
}
