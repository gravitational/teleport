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

package organizations

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestHasActiveMatchers(t *testing.T) {
	for _, tt := range []struct {
		name     string
		filter   MatchingAccountsFilter
		expected bool
	}{
		{
			name:     "empty struct",
			filter:   MatchingAccountsFilter{},
			expected: false,
		},
		{
			name:     "include only",
			filter:   MatchingAccountsFilter{IncludeOUs: []string{"ou-a"}},
			expected: true,
		},
		{
			name:     "exclude only",
			filter:   MatchingAccountsFilter{ExcludeOUs: []string{"ou-b"}},
			expected: true,
		},
		{
			name:     "both",
			filter:   MatchingAccountsFilter{IncludeOUs: []string{"ou-a"}, ExcludeOUs: []string{"ou-b"}},
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, HasActiveMatchers(tt.filter))
		})
	}
}

func TestOrganizationalUnitsMatch(t *testing.T) {
	for _, tt := range []struct {
		name       string
		filter     MatchingAccountsFilter
		accountOUs []string
		expected   bool
	}{
		{
			name:       "no included OU: account does not match",
			filter:     MatchingAccountsFilter{ExcludeOUs: []string{"ou-b"}},
			accountOUs: []string{"r-root", "ou-a"},
			expected:   false,
		},
		{
			name:       "include everything: account matches",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{types.Wildcard}},
			accountOUs: []string{"r-root"},
			expected:   true,
		},
		{
			name:       "include everything with non-matching exclude: account matches",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{types.Wildcard}, ExcludeOUs: []string{"ou-b"}},
			accountOUs: []string{"r-root", "ou-a"},
			expected:   true,
		},
		{
			name:       "include everything with matching exclude: account does not match",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{types.Wildcard}, ExcludeOUs: []string{"ou-a"}},
			accountOUs: []string{"r-root", "ou-a"},
			expected:   false,
		},
		{
			name:       "include matches the account's parent",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{"ou-a"}},
			accountOUs: []string{"r-root", "ou-a"},
			expected:   true,
		},
		{
			name:       "include matches a 2nd level ancestor OU",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{"ou-a"}},
			accountOUs: []string{"r-root", "ou-a", "ou-child"},
			expected:   true,
		},
		{
			name:       "include does not match",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{"ou-b"}},
			accountOUs: []string{"r-root", "ou-a"},
			expected:   false,
		},
		{
			name:       "exclude has priority over include for parent OU",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{"ou-a"}, ExcludeOUs: []string{"ou-child"}},
			accountOUs: []string{"r-root", "ou-a", "ou-child"},
			expected:   false,
		},
		{
			name:       "exclude has priority over include for ancestor OU",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{"ou-child"}, ExcludeOUs: []string{"ou-a"}},
			accountOUs: []string{"r-root", "ou-a", "ou-child"},
			expected:   false,
		},
		{
			name:       "root OU is treated as an OU for inclusion",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{"r-root"}},
			accountOUs: []string{"r-root"},
			expected:   true,
		},
		{
			name:       "root OU can be excluded with wildcard include",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{types.Wildcard}, ExcludeOUs: []string{"r-root"}},
			accountOUs: []string{"r-root", "ou-a"},
			expected:   false,
		},
		{
			name:       "if exclude everything, then no account matches even if include everything",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{types.Wildcard}, ExcludeOUs: []string{types.Wildcard}},
			accountOUs: []string{"r-root", "ou-a"},
			expected:   false,
		},
		{
			name:       "when including everything, even if no chain is passed the account matches",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{types.Wildcard}},
			accountOUs: nil,
			expected:   true,
		},
		{
			name:       "when including a specific OU, when no OUs are passed, the account does not match",
			filter:     MatchingAccountsFilter{IncludeOUs: []string{"ou-a"}},
			accountOUs: nil,
			expected:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, OrganizationalUnitsMatch(tt.filter, tt.accountOUs))
		})
	}
}
