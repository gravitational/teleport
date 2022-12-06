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

package services

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
)

// maxMismatchedTraitValuesLogged indicates the maximum number of trait values (that do not match a
// certain expression) to be shown in a warning
const maxMismatchedTraitValuesWarned = 100

// TraitsToRoles maps the supplied traits to a list of teleport role names.
// Returns the list of roles mapped from traits.
// `warnings` optionally contains the list of warnings potentially interesting to the user.
func TraitsToRoles(ms types.TraitMappingSet, traits map[string][]string) (warnings []string, roles []string) {
	warnings = traitsToRoles(ms, traits, func(role string, expanded bool) {
		roles = append(roles, role)
	})
	return warnings, apiutils.Deduplicate(roles)
}

// TraitsToRoleMatchers maps the supplied traits to a list of role matchers. Prefer calling
// this function directly rather than calling TraitsToRoles and then building matchers from
// the resulting list since this function forces any roles which include substitutions to
// be literal matchers.
func TraitsToRoleMatchers(ms types.TraitMappingSet, traits map[string][]string) ([]parse.Matcher, error) {
	var matchers []parse.Matcher
	var firstErr error
	traitsToRoles(ms, traits, func(role string, expanded bool) {
		if expanded || utils.ContainsExpansion(role) {
			// mapping process included variable expansion; we therefore
			// "escape" normal matcher syntax and look only for exact matches.
			// (note: this isn't about combatting maliciously constructed traits,
			// traits are from trusted identity sources, this is just
			// about avoiding unnecessary footguns).
			matchers = append(matchers, literalMatcher{
				value: role,
			})
			return
		}
		m, err := parse.NewMatcher(role)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		matchers = append(matchers, m)
	})
	if firstErr != nil {
		return nil, trace.Wrap(firstErr)
	}
	return matchers, nil
}

// traitsToRoles maps the supplied traits to teleport role names and passes them to a collector.
func traitsToRoles(ms types.TraitMappingSet, traits map[string][]string, collect func(role string, expanded bool)) (warnings []string) {
	// if no traits, avoid compiling of trait mapping values as regular expressions
	if len(traits) == 0 {
		return
	}

	for _, mapping := range ms {
		// compile each trait mapping value exactly twice
		regexpIgnoreCase, err := utils.RegexpWithConfig(mapping.Value, utils.RegexpConfig{IgnoreCase: true})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("case-insensitive expression %q is not a valid regexp", mapping.Value))
			continue
		}
		regexp, err := utils.RegexpWithConfig(mapping.Value, utils.RegexpConfig{})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("case-sensitive expression %q is not a valid regexp", mapping.Value))
			continue
		}

		for traitName, traitValues := range traits {
			if traitName != mapping.Trait {
				continue
			}
			var mismatched []string
		TraitLoop:
			for _, traitValue := range traitValues {
				for _, role := range mapping.Roles {
					// Run the initial replacement case-insensitively. Doing so will filter out all literal non-matches
					// but will match on case discrepancies. We do another case-sensitive match below to see if the
					// case is different
					outRole, err := utils.ReplaceRegexpWith(regexpIgnoreCase, role, traitValue)
					switch {
					case err != nil:
						// this trait value clearly did not match, move on to another
						mismatched = append(mismatched, fmt.Sprintf("%q", traitValue))
						continue TraitLoop
					case outRole == "":
					case outRole != "":
						// Run the replacement case-sensitively to see if it matches.
						// If there's no match, the trait specifies a mapping which is case-sensitive;
						// we should log a warning but return an error.
						// See https://github.com/gravitational/teleport/issues/6016 for details.
						if _, err := utils.ReplaceRegexpWith(regexp, role, traitValue); err != nil {
							warnings = append(warnings, fmt.Sprintf("trait %q matches value %q case-insensitively and would have yielded %q role", traitValue, mapping.Value, outRole))
							continue
						}
						// skip empty replacement or empty role
						collect(outRole, outRole != role)
					}
				}
			}

			// warn at most maxMismatchedTraitValuesWarned trait values to prevent huge warning lines
			if len(mismatched) > maxMismatchedTraitValuesWarned {
				warnings = append(warnings, fmt.Sprintf(
					"%d trait value(s) did not match expression %q: %s (first %d values)",
					len(mismatched),
					mapping.Value,
					mismatched[0:maxMismatchedTraitValuesWarned],
					maxMismatchedTraitValuesWarned,
				))
			} else if len(mismatched) > 0 {
				warnings = append(warnings, fmt.Sprintf(
					"%d trait value(s) did not match expression %q: %s",
					len(mismatched),
					mapping.Value,
					mismatched,
				))
			}
		}
	}
	return
}

// literalMatcher is used to "escape" values which are not allowed to
// take advantage of normal matcher syntax by limiting them to only
// literal matches.
type literalMatcher struct {
	value string
}

func (m literalMatcher) Match(in string) bool { return m.value == in }
