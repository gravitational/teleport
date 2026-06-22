/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package queries

import (
	"strings"
	"testing"
	"unicode"

	"pgregory.net/rapid"

	"github.com/stretchr/testify/require"
)

func TestProperty_ListRunningLinuxVMsQuery_Filters(t *testing.T) {
	charIsAllowed := func(char rune) bool {
		return unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '.' || char == '_' || char == '(' || char == ')' || char == '/'
	}

	rapid.Check(t, func(t *rapid.T) {
		resourceGroupFilter := rapid.String().Draw(t, "resourceGroupFilter")
		locationsFilter := rapid.SliceOf(rapid.String()).Draw(t, "locationsFilter")

		listVMsQuery, err := NewListRunningLinuxVMsQuery(ListRunningLinuxVMsQueryFilters{
			ResourceGroupFilter: resourceGroupFilter,
			LocationsFilter:     locationsFilter,
		})
		if err != nil {
			return
		}

		// Resulting Query must not have special characters that could break the KQL query in the specified filters.
		kqlQuery, err := listVMsQuery.Query()
		require.NoError(t, err)

		// Collect every term within ' and ".
		// Unmatched quotes are errors.
		var quotedTerms []string
		for {
			if len(kqlQuery) == 0 {
				break
			}

			index := strings.IndexAny(kqlQuery, "'\"")
			if index == -1 {
				break
			}
			quoteChar := kqlQuery[index]
			kqlQuery = kqlQuery[index+1:]

			endIndex := strings.Index(kqlQuery, string(quoteChar))
			if endIndex == -1 {
				t.Fatalf("unmatched quote %q in query: %s", quoteChar, kqlQuery)
			}
			quotedTerms = append(quotedTerms, kqlQuery[:endIndex])
			kqlQuery = kqlQuery[endIndex+1:]
		}

		if resourceGroupFilter != "" {
			require.Contains(t, quotedTerms, resourceGroupFilter)
		}
		for _, locationsFilter := range locationsFilter {
			require.Contains(t, quotedTerms, locationsFilter)
		}
		for _, term := range quotedTerms {
			for _, char := range term {
				if !charIsAllowed(char) {
					t.Fatalf("filter values must not contain invalid chars, got %q in term %q", char, term)
				}
			}
		}
	})
}
