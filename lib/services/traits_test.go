/*
Copyright 2020 Gravitational, Inc.

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

package services

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
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
		require.ElementsMatch(t, utils.Deduplicate(matches), tt.matches, tt.desc)
	}
}
