/*
Copyright 2018 Gravitational, Inc.

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

package events

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
)

func TestEventFields(t *testing.T) {
	t.Parallel()
	now := time.Now().Round(time.Minute)

	slice := []string{"test", "string", "slice"}
	slice2 := []interface{}{"test", "string", "slice"}
	f := EventFields{
		"one":      1,
		"name":     "vincent",
		"time":     now,
		"strings":  slice,
		"strings2": slice2,
	}

	require.Equal(t, 1, f.GetInt("one"))
	require.Equal(t, 0, f.GetInt("two"))
	require.Equal(t, "vincent", f.GetString("name"))
	require.Equal(t, "", f.GetString("city"))
	require.Equal(t, now, f.GetTime("time"))
	require.Equal(t, slice, f.GetStrings("strings"))
	require.Equal(t, slice, f.GetStrings("strings2"))
	require.Nil(t, f.GetStrings("strings3"))
}

func TestUpdateEventFields(t *testing.T) {
	t.Parallel()

	event := Event{
		Name: "test.event",
		Code: "TEST0001I",
	}
	fields := EventFields{
		EventUser:   "test@example.com",
		LoginMethod: LoginMethodOIDC,
	}
	require.NoError(t, UpdateEventFields(event, fields, clockwork.NewFakeClock(), utils.NewFakeUID()))

	// Check the fields have been updated appropriately.
	require.Equal(t, EventFields{
		EventType:   event.Name,
		EventID:     fixtures.UUID,
		EventCode:   event.Code,
		EventTime:   time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC),
		EventUser:   "test@example.com",
		LoginMethod: LoginMethodOIDC,
	}, fields)
}

func TestToEventFieldsCondition(t *testing.T) {
	t.Parallel()

	// !equals(login, "root") && contains(participants, "test-user")
	expr := &types.WhereExpr{And: types.WhereExpr2{
		L: &types.WhereExpr{Not: &types.WhereExpr{Equals: types.WhereExpr2{L: &types.WhereExpr{Field: "login"}, R: &types.WhereExpr{Literal: "root"}}}},
		R: &types.WhereExpr{Contains: types.WhereExpr2{L: &types.WhereExpr{Field: "participants"}, R: &types.WhereExpr{Literal: "test-user"}}},
	}}

	cond, err := ToEventFieldsCondition(expr)
	require.NoError(t, err)

	require.False(t, cond(EventFields{}))
	require.False(t, cond(EventFields{"login": "root", "participants": []string{"test-user", "observer"}}))
	require.False(t, cond(EventFields{"login": "guest", "participants": []string{"another-user"}}))
	require.True(t, cond(EventFields{"login": "guest", "participants": []string{"test-user", "observer"}}))
	require.True(t, cond(EventFields{"participants": []string{"test-user"}}))
}
