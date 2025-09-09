// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package filter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestMatches(t *testing.T) {
	type TestItem struct {
		ID   string
		Name string
	}

	items := []TestItem{
		{ID: "1", Name: "apple"},
		{ID: "2", Name: "banana"},
		{ID: "3", Name: "cherry"},
		{ID: "4", Name: "avocado"},
	}

	paramFunc := func() MatchParam[TestItem] {
		return MatchParam[TestItem]{
			GetName: func(item TestItem) string {
				return item.Name
			},
			GetID: func(item TestItem) string {
				return item.ID
			},
		}
	}

	tests := []struct {
		name     string
		items    []TestItem
		filters  Filters
		param    MatchParam[TestItem]
		expected []TestItem
	}{
		{
			name:     "Filter by ID",
			items:    items,
			filters:  Filters{&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "2"}}},
			expected: []TestItem{{ID: "2", Name: "banana"}},
		},
		{
			name:     "Filter by Name",
			items:    items,
			filters:  Filters{&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}}},
			expected: []TestItem{{ID: "1", Name: "apple"}, {ID: "4", Name: "avocado"}},
		},
		{
			name:     "Exclude All",
			items:    items,
			filters:  Filters{&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "teleport.internal/exclude_all"}}},
			expected: nil,
		},
		{
			name:     "No Filters",
			items:    items,
			filters:  Filters{},
			expected: nil,
		},
		{
			name:  "Multiple Filters",
			items: items,
			filters: Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "2"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "4"}},
			},
			expected: []TestItem{
				{ID: "1", Name: "apple"},
				{ID: "2", Name: "banana"},
				{ID: "4", Name: "avocado"},
			},
		},
		{
			name:  "Exclude by ID",
			items: items,
			filters: Filters{
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "2"}},
			},
			expected: []TestItem{
				{ID: "1", Name: "apple"},
				{ID: "3", Name: "cherry"},
				{ID: "4", Name: "avocado"},
			},
		},
		{
			name:  "Exclude by NameRegex",
			items: items,
			filters: Filters{
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "a*"}},
			},
			expected: []TestItem{
				{ID: "2", Name: "banana"},
				{ID: "3", Name: "cherry"},
			},
		},
		{
			name:  "Include and Exclude - exclude wins",
			items: items,
			filters: Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "*"}},
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "1"}},
			},
			expected: []TestItem{
				{ID: "2", Name: "banana"},
				{ID: "3", Name: "cherry"},
				{ID: "4", Name: "avocado"},
			},
		},
		{
			name:  "Include and Exclude - exclude wins case 2",
			items: items,
			filters: Filters{
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "a*"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "b*"}},
			},
			expected: []TestItem{
				{ID: "2", Name: "banana"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filtered []TestItem
			for _, i := range items {
				if Matches(i, tt.filters, paramFunc()) {
					filtered = append(filtered, i)
				}
			}
			require.ElementsMatch(t, tt.expected, filtered)
		})
	}
}

func TestPoorlyFormedFiltersAreAnError(t *testing.T) {
	testCases := []struct {
		name           string
		filters        Filters
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name: "Bad regex",
			filters: Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "^[)$"}},
			},
			errorAssertion: require.Error,
		},

		{
			name: "Bad exclude regex",
			filters: Filters{
				&types.PluginSyncFilter{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "^[)$"}},
			},
			errorAssertion: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			test.errorAssertion(t, test.filters.validate())
		})
	}
}
