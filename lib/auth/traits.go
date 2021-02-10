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

package auth

import (
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// TraitsToRoles maps the supplied traits to a list of teleport role names.
func TraitsToRoles(ms TraitMappingSet, traits map[string][]string) []string {
	var roles []string
	traitsToRoles(ms, traits, func(role string, expanded bool) {
		roles = append(roles, role)
	})
	return utils.Deduplicate(roles)
}

// TraitsToRoleMatchers maps the supplied traits to a list of role matchers. Prefer calling
// this function directly rather than calling TraitsToRoles and then building matchers from
// the resulting list since this function forces any roles which include substitutions to
// be literal matchers.
func TraitsToRoleMatchers(ms TraitMappingSet, traits map[string][]string) ([]parse.Matcher, error) {
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
func traitsToRoles(ms TraitMappingSet, traits map[string][]string, collect func(role string, expanded bool)) {
	for _, mapping := range ms {
		for traitName, traitValues := range traits {
			if traitName != mapping.Trait {
				continue
			}
		TraitLoop:
			for _, traitValue := range traitValues {
				for _, role := range mapping.Roles {
					outRole, err := utils.ReplaceRegexp(mapping.Value, role, traitValue)
					switch {
					case err != nil:
						if trace.IsNotFound(err) {
							log.WithError(err).Debugf("Failed to match expression %v, replace with: %v input: %v", mapping.Value, role, traitValue)
						}
						// this trait value clearly did not match, move on to another
						continue TraitLoop
					case outRole == "":
					case outRole != "":
						// skip empty replacement or empty role
						collect(outRole, outRole != role)
					}
				}
			}
		}
	}
}

// literalMatcher is used to "escape" values which are not allowed to
// take advantage of normal matcher syntax by limiting them to only
// literal matches.
type literalMatcher struct {
	value string
}

func (m literalMatcher) Match(in string) bool { return m.value == in }
