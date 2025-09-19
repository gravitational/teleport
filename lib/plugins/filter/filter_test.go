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
	items := []MatchParam{
		{ID: "1", Name: "apple"},
		{ID: "2", Name: "banana"},
		{ID: "3", Name: "cherry"},
		{ID: "4", Name: "avocado"},
	}

	tests := []struct {
		name     string
		items    []MatchParam
		filters  Filters
		expected []MatchParam
	}{
		{
			name:     "Filter by ID",
			items:    items,
			filters:  Filters{&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "2"}}},
			expected: []MatchParam{{ID: "2", Name: "banana"}},
		},
		{
			name:     "Filter by Name",
			items:    items,
			filters:  Filters{&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}}},
			expected: []MatchParam{{ID: "1", Name: "apple"}, {ID: "4", Name: "avocado"}},
		},
		{
			name:     "Exclude All",
			items:    items,
			filters:  Filters{&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "teleport.internal/exclude_all"}}},
			expected: nil,
		},
		{
			name:     "No Filters (matches all)",
			items:    items,
			filters:  Filters{},
			expected: items,
		},
		{
			name:  "Multiple Filters",
			items: items,
			filters: Filters{
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "2"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
				&types.PluginSyncFilter{Include: &types.PluginSyncFilter_Id{Id: "4"}},
			},
			expected: []MatchParam{
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
			expected: []MatchParam{
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
			expected: []MatchParam{
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
			expected: []MatchParam{
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
			expected: []MatchParam{
				{ID: "2", Name: "banana"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filtered []MatchParam
			for _, i := range items {
				if Matches(tt.filters, MatchParam{
					ID:   i.ID,
					Name: i.Name,
				}) {
					filtered = append(filtered, i)
				}
			}
			require.ElementsMatch(t, tt.expected, filtered)
		})
	}
}

func TestNew(t *testing.T) {
	type unsupportedFilterType struct {
		types.PluginSyncFilter_Id
	}
	testCases := []struct {
		name           string
		filters        []*types.PluginSyncFilter
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name: "bad regex",
			filters: []*types.PluginSyncFilter{
				{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "^[)$"}},
			},
			errorAssertion: require.Error,
		},
		{
			name: "bad exclude regex",
			filters: []*types.PluginSyncFilter{
				{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "^[)$"}},
			},
			errorAssertion: require.Error,
		},
		{
			name:           "empty filter",
			filters:        nil,
			errorAssertion: require.NoError,
		},
		{
			name: "empty include id",
			filters: []*types.PluginSyncFilter{
				{Include: &types.PluginSyncFilter_Id{}},
			},
			errorAssertion: require.Error,
		},
		{
			name: "empty exclude id",
			filters: []*types.PluginSyncFilter{
				{Exclude: &types.PluginSyncFilter_ExcludeId{}},
			},
			errorAssertion: require.Error,
		},
		{
			name: "unknown filter type",
			filters: []*types.PluginSyncFilter{
				{Include: &unsupportedFilterType{}},
			},
			errorAssertion: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.filters)
			test.errorAssertion(t, err)
		})
	}
}
