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
	"slices"

	"github.com/gravitational/teleport/api/types"
)

// HasActiveMatchers reports whether matcher specifies any filtering.
func HasActiveMatchers(filter MatchingAccountsFilter) bool {
	return len(filter.IncludeOUs) > 0 || len(filter.ExcludeOUs) > 0
}

// OrganizationalUnitsMatch returns whether the Organizational Unit chain of an account should be included according to the filter.
func OrganizationalUnitsMatch(matcher MatchingAccountsFilter, accountOrganizationalUnits []string) bool {
	if len(matcher.IncludeOUs) == 0 {
		return false
	}
	if slices.Contains(matcher.ExcludeOUs, types.Wildcard) {
		return false
	}
	for _, ou := range accountOrganizationalUnits {
		if slices.Contains(matcher.ExcludeOUs, ou) {
			return false
		}
	}
	if slices.Contains(matcher.IncludeOUs, types.Wildcard) {
		return true
	}
	for _, ou := range accountOrganizationalUnits {
		if slices.Contains(matcher.IncludeOUs, ou) {
			return true
		}
	}
	return false
}
