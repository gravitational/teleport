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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
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

func TestTraits(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		traitName string
	}{
		// Windows trait names are URLs.
		{
			traitName: "http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname",
		},
		// Simple strings are the most common trait names.
		{
			traitName: "user-groups",
		},
	}

	for _, tt := range tests {
		user := &types.UserV2{
			Kind:    types.KindUser,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      "foo",
				Namespace: apidefaults.Namespace,
			},
			Spec: types.UserSpecV2{
				Traits: map[string][]string{
					tt.traitName: {"foo"},
				},
			},
		}

		data, err := json.Marshal(user)
		require.NoError(t, err)

		_, err = UnmarshalUser(data)
		require.NoError(t, err)
	}
}

type traitsToRolesInput struct {
	name          string
	traits        map[string][]string
	expectedRoles []string
	warnings      []string
}

var traitsToRolesTestCases = []struct {
	name     string
	mappings []types.TraitMapping
	inputs   []traitsToRolesInput
}{
	{
		name: "no mappings",
		inputs: []traitsToRolesInput{
			{
				name:          "no match",
				traits:        map[string][]string{"a": {"b"}},
				expectedRoles: nil,
			},
		},
	},
	{
		name: "simple mappings",
		mappings: []types.TraitMapping{
			{Trait: "role", Value: "admin", Roles: []string{"admin", "bob"}},
			{Trait: "role", Value: "user", Roles: []string{"user"}},
		},
		inputs: []traitsToRolesInput{
			{
				name:          "no match",
				traits:        map[string][]string{"a": {"b"}},
				expectedRoles: nil,
			},
			{
				name:          "no value match",
				traits:        map[string][]string{"role": {"b"}},
				expectedRoles: nil,
			},
			{
				name:          "direct admin value match",
				traits:        map[string][]string{"role": {"admin"}},
				expectedRoles: []string{"admin", "bob"},
			},
			{
				name:          "direct user value match",
				traits:        map[string][]string{"role": {"user"}},
				expectedRoles: []string{"user"},
			},
			{
				name:          "direct user value match with array",
				traits:        map[string][]string{"role": {"user"}},
				expectedRoles: []string{"user"},
			},
		},
	},
	{
		name: "regexp mappings match",
		mappings: []types.TraitMapping{
			{Trait: "role", Value: "^admin-(.*)$", Roles: []string{"role-$1", "bob"}},
		},
		inputs: []traitsToRolesInput{
			{
				name:          "no match",
				traits:        map[string][]string{"a": {"b"}},
				expectedRoles: nil,
			},
			{
				name:          "no match - subprefix",
				traits:        map[string][]string{"role": {"adminz"}},
				expectedRoles: nil,
			},
			{
				name:          "value with capture match",
				traits:        map[string][]string{"role": {"admin-hello"}},
				expectedRoles: []string{"role-hello", "bob"},
			},
			{
				name:          "multiple value with capture match, deduplication",
				traits:        map[string][]string{"role": {"admin-hello", "admin-ola"}},
				expectedRoles: []string{"role-hello", "bob", "role-ola"},
			},
			{
				name:          "first matches, second does not",
				traits:        map[string][]string{"role": {"hello", "admin-ola"}},
				expectedRoles: []string{"role-ola", "bob"},
			},
		},
	},
	{
		name: "regexp compilation",
		mappings: []types.TraitMapping{
			{Trait: "role", Value: `^admin-(?!)$`, Roles: []string{"admin"}}, // "?!" is invalid.
			{Trait: "role", Value: "^admin-(.*)$", Roles: []string{"role-$1", "bob"}},
			{Trait: "role", Value: `^admin2-(?!)$`, Roles: []string{"admin2"}}, // "?!" is invalid.
		},
		inputs: []traitsToRolesInput{
			{
				name:          "invalid regexp",
				traits:        map[string][]string{"role": {"admin-hello", "dev"}},
				expectedRoles: []string{"role-hello", "bob"},
				warnings: []string{
					`case-insensitive expression "^admin-(?!)$" is not a valid regexp`,
					`case-insensitive expression "^admin2-(?!)$" is not a valid regexp`,
				},
			},
			{
				name:          "regexp are not compiled if not needed",
				traits:        map[string][]string{},
				expectedRoles: nil,
				// if the regexp were compiled, we would have the same warnings as above
				warnings: nil,
			},
		},
	},
	{
		name: "empty expands are skipped",
		mappings: []types.TraitMapping{
			{Trait: "role", Value: "^admin-(.*)$", Roles: []string{"$2", "bob"}},
		},
		inputs: []traitsToRolesInput{
			{
				name:          "value with capture match",
				traits:        map[string][]string{"role": {"admin-hello"}},
				expectedRoles: []string{"bob"},
			},
		},
	},
	{
		name: "glob wildcard match",
		mappings: []types.TraitMapping{
			{Trait: "role", Value: "*", Roles: []string{"admin"}},
		},
		inputs: []traitsToRolesInput{
			{
				name:          "empty value match",
				traits:        map[string][]string{"role": {""}},
				expectedRoles: []string{"admin"},
			},
			{
				name:          "any value match",
				traits:        map[string][]string{"role": {"zz"}},
				expectedRoles: []string{"admin"},
			},
		},
	},
	{
		name: "Whitespace/dashes",
		mappings: []types.TraitMapping{
			{Trait: "groups", Value: "DemoCorp - Backend Engineers", Roles: []string{"backend"}},
			{Trait: "groups", Value: "DemoCorp - SRE Managers", Roles: []string{"approver"}},
			{Trait: "groups", Value: "DemoCorp - SRE", Roles: []string{"approver"}},
			{Trait: "groups", Value: "DemoCorp Infrastructure", Roles: []string{"approver", "backend"}},
		},
		inputs: []traitsToRolesInput{
			{
				name: "Matches multiple groups",
				traits: map[string][]string{
					"groups": {"DemoCorp - Backend Engineers", "DemoCorp Infrastructure"},
				},
				expectedRoles: []string{"backend", "approver"},
			},
			{
				name: "Matches one group",
				traits: map[string][]string{
					"groups": {"DemoCorp - SRE"},
				},
				expectedRoles: []string{"approver"},
			},
			{
				name: "Matches one group with multiple roles",
				traits: map[string][]string{
					"groups": {"DemoCorp Infrastructure"},
				},
				expectedRoles: []string{"approver", "backend"},
			},
			{
				name: "No match only due to case-sensitivity",
				traits: map[string][]string{
					"groups": {"Democorp - SRE"},
				},
				expectedRoles: nil,
				warnings: []string{
					`trait "Democorp - SRE" matches value "DemoCorp - SRE" case-insensitively and would have yielded "approver" role`,
				},
			},
		},
	},
}

func TestTraitsToRoles(t *testing.T) {
	t.Parallel()
	for _, testCase := range traitsToRolesTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			for _, input := range testCase.inputs {
				t.Run(input.name, func(t *testing.T) {
					t.Parallel()
					warnings, outRoles := TraitsToRoles(testCase.mappings, input.traits)
					require.Equal(t, input.expectedRoles, outRoles)
					require.Equal(t, input.warnings, warnings)
				})
			}
		})
	}
}

func BenchmarkTraitToRoles(b *testing.B) {
	for _, testCase := range traitsToRolesTestCases {
		b.Run(testCase.name, func(b *testing.B) {
			for _, input := range testCase.inputs {
				b.Run(input.name, func(b *testing.B) {
					for b.Loop() {
						TraitsToRoles(testCase.mappings, input.traits)
					}
				})
			}
		})
	}
}
