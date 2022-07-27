/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	sliceInterface := []interface{}{"test", "string", "slice"}
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
	require.Equal(t, "", f.GetString("city"))
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
