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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// TestTraitsToRoleMatchers verifies expected matching behavior
// of the TraitMappingSet.TraitsToRoleMatchers method.
func TestTraitsToRoleMatchers(t *testing.T) {
	traits := map[string][]string{
		"groups": {
			"admins",
			"devs",
			"populists",
		},
		"sketchy": {
			"*",
			"^(.*)$",
			"*-prod",
		},
		"sketchy-pfx": {
			"pfx-dev",
			"pfx-*",
		},
		"requirements": {
			"staging-only",
			"democracy",
		},
	}

	roles := []string{
		"admin-prod",
		"admin-staging",
		"dev-prod",
		"dev-staging",
		"dictator",
	}

	tts := []struct {
		desc    string
		tm      types.TraitMapping
		matches []string
	}{
		{
			desc: "literal matching",
			tm: types.TraitMapping{
				Trait: "groups",
				Value: "populists",
				Roles: []string{"dictator"},
			},
			matches: []string{"dictator"},
		},
		{
			desc: "basic prefix-based matching",
			tm: types.TraitMapping{
				Trait: "groups",
				Value: "devs",
				Roles: []string{"dev-*"},
			},
			matches: []string{"dev-prod", "dev-staging"},
		},
		{
			desc: "basic suffix-based matching",
			tm: types.TraitMapping{
				Trait: "requirements",
				Value: "staging-only",
				Roles: []string{"*-staging"},
			},
			matches: []string{"admin-staging", "dev-staging"},
		},
		{
			desc: "negative matching",
			tm: types.TraitMapping{
				Trait: "requirements",
				Value: "democracy",
				Roles: []string{"{{regexp.not_match(\"dictator\")}}"},
			},
			matches: []string{"admin-prod", "admin-staging", "dev-prod", "dev-staging"},
		},
		{
			desc: "pattern-like trait submatch substitution cannot result in non-literal matchers",
			tm: types.TraitMapping{
				Trait: "sketchy-pfx",
				Value: "pfx-*",
				Roles: []string{"${1}-prod", "${1}-staging", "${1}"},
			},
			matches: []string{"dev-prod", "dev-staging"},
		},
		{
			desc: "pattern-like trait value substitution cannot result in non-literal matchers",
			tm: types.TraitMapping{
				Trait: "sketchy",
				Value: "*",
				Roles: []string{"dev-${1}", "admin-${1}", "${1}"},
			},
			matches: []string{},
		},
	}

	for _, tt := range tts {
		matchers, err := TraitsToRoleMatchers([]types.TraitMapping{tt.tm}, traits)
		require.NoError(t, err, tt.desc)

		// collect all roles which match at least on of the
		// constructed matchers.
		var matches []string
	Outer:
		for _, role := range roles {
			for _, matcher := range matchers {
				if matcher.Match(role) {
					matches = append(matches, role)
					continue Outer
				}
			}
		}

		// verify that the resulting matches, once deduplicated, are equivalent
		// to the expected matches.
		require.ElementsMatch(t, apiutils.Deduplicate(matches), tt.matches, tt.desc)
	}
}
