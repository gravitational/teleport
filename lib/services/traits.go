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
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
)

// maxMismatchedTraitValuesLogged indicates the maximum number of trait values (that do not match a
// certain expression) to be shown in the log
const maxMismatchedTraitValuesLogged = 100

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
TraitMappingLoop:
	for _, mapping := range ms {
		var regexpIgnoreCase *regexp.Regexp
		var regexp *regexp.Regexp

		for traitName, traitValues := range traits {
			if traitName != mapping.Trait {
				continue
			}
			var mismatched []string
		TraitLoop:
			for _, traitValue := range traitValues {
				for _, role := range mapping.Roles {
					// this ensures that the case-insensitive regexp is compiled at most once, and only if strictly needed;
					// after this if, regexpIgnoreCase must be non-nil
					if regexpIgnoreCase == nil {
						var err error
						regexpIgnoreCase, err = utils.RegexpWithConfig(mapping.Value, utils.RegexpConfig{IgnoreCase: true})
						if err != nil {
							warnings = append(warnings, fmt.Sprintf("case-insensitive expression %q is not a valid regexp", mapping.Value))
							continue TraitMappingLoop
						}
					}

					// Run the initial replacement case-insensitively. Doing so will filter out all literal non-matches
					// but will match on case discrepancies. We do another case-sensitive match below to see if the
					// case is different
					outRole, err := utils.ReplaceRegexpWith(regexpIgnoreCase, role, traitValue)
					switch {
					case err != nil:
						// this trait value clearly did not match, move on to another
						mismatched = append(mismatched, traitValue)
						continue TraitLoop
					case outRole == "":
					case outRole != "":
						// this ensures that the case-sensitive regexp is compiled at most once, and only if strictly needed;
						// after this if, regexp must be non-nil
						if regexp == nil {
							var err error
							regexp, err = utils.RegexpWithConfig(mapping.Value, utils.RegexpConfig{})
							if err != nil {
								warnings = append(warnings, fmt.Sprintf("case-sensitive expression %q is not a valid regexp", mapping.Value))
								continue TraitMappingLoop
							}
						}

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

			// show at most maxMismatchedTraitValuesLogged trait values to prevent huge log lines
			switch l := len(mismatched); {
			case l > maxMismatchedTraitValuesLogged:
				slog.
					DebugContext(context.Background(), "trait value(s) did not match (showing first %d values)",
						"mismatch_count", len(mismatched),
						"max_mismatch_logged", maxMismatchedTraitValuesLogged,
						"expression", mapping.Value,
						"values", mismatched[0:maxMismatchedTraitValuesLogged],
					)
			case l > 0:
				slog.DebugContext(context.Background(), "trait value(s) did not match",
					"mismatch_count", len(mismatched),
					"expression", mapping.Value,
					"values", mismatched,
				)
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

func literalMatchers(literals []string) []parse.Matcher {
	matchers := make([]parse.Matcher, 0, len(literals))
	for _, literal := range literals {
		matchers = append(matchers, literalMatcher{literal})
	}
	return matchers
}
