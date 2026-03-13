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

package proxy

import (
	"regexp"
	"slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// fastResourceMatcher is a precompiled per-request RBAC matcher that reduces per-item matching overhead.
// Instead of calling matchKubernetesResource per item (which does cache lookups, rule iteration, per-field matching),
// the fast matcher pre-filters rules by kind and verb at compile time and pre-compiles all regex patterns.
// Per-item cost becomes direct regex matching against only
// the relevant rules.
//
// The fast matcher handles the common "default" case in KubeResourceMatchesRegex.
// It cannot handle namespace special cases (when the requested kind is "namespaces"),
// so tryCompileFastMatcher returns nil for those requests and the caller falls back to matchKubernetesResource.
type fastResourceMatcher struct {
	allowRules []compiledMatchRule
	denyRules  []compiledMatchRule
}

// compiledMatchRule is a single RBAC rule with pre-compiled regex patterns
// and pre-filtered kind/verb (only rules matching the request's kind and verb are included).
type compiledMatchRule struct {
	apiGroup  *regexp.Regexp
	name      *regexp.Regexp
	namespace *regexp.Regexp
	// requiresNamespace is true when the original rule had a non-empty, non-wildcard namespace pattern.
	// When true, resources with an empty namespace cannot match this rule.
	requiresNamespace bool
}

// maxFastMatcherRules is the maximum number of RBAC rules
// (allowed + denied combined, after kind/verb filtering) for which the fast matcher is used.
// Beyond this threshold we fall back to the cached matchKubernetesResource path.
// Benchmarks show the fast matcher is faster even at 4000 rules, so this is a
// conservative safety margin rather than a measured crossover point.
// It may be removed in a follow-up once we gain more production confidence.
var maxFastMatcherRules = 200

// tryCompileFastMatcher attempts to compile a fast matcher from the given RBAC rules.
// Returns nil (without error) if the fast matcher cannot handle the request,
// signaling the caller to fall back to matchKubernetesResource.
func tryCompileFastMatcher(kind, verb string, allowed, denied []types.KubernetesResource) (*fastResourceMatcher, error) {
	// The fast matcher cannot handle namespace special cases in KubeResourceMatchesRegex
	// (read-only namespace visibility, namespace kind matching with different target selection).
	if kind == "namespaces" {
		return nil, nil
	}

	// This is a quick check that eliminates irrelevant rules before compilation.
	allowed = filterRulesByKindVerb(kind, verb, allowed)
	denied = filterRulesByKindVerb(kind, verb, denied)

	// If too many rules survive kind/verb filtering, compilation cost exceeds the per-item savings.
	if len(allowed)+len(denied) > maxFastMatcherRules {
		return nil, nil
	}

	// Local cache for this compilation pass. Many rules share the same patterns (e.g., apiGroup="*", name="*"),
	// so this avoids compiling the same expression multiple times.
	compiled := make(map[string]*regexp.Regexp)

	allowRules, err := compileMatchRules(allowed, compiled)
	if err != nil {
		return nil, err
	}
	denyRules, err := compileMatchRules(denied, compiled)
	if err != nil {
		return nil, err
	}

	return &fastResourceMatcher{
		allowRules: allowRules,
		denyRules:  denyRules,
	}, nil
}

// filterRulesByKindVerb returns the subset of rules that match the given kind and verb.
func filterRulesByKindVerb(kind, verb string, rules []types.KubernetesResource) []types.KubernetesResource {
	return slices.DeleteFunc(slices.Clone(rules), func(r types.KubernetesResource) bool {
		return !kindAllowed(r.Kind, kind) || !verbAllowed(r.Verbs, verb)
	})
}

func kindAllowed(ruleKind, requestedKind string) bool {
	return ruleKind == types.Wildcard || ruleKind == requestedKind
}

func verbAllowed(allowedVerbs []string, verb string) bool {
	// This mirrors utils.isVerbAllowed: wildcard is only recognized as the first element.
	return len(allowedVerbs) != 0 &&
		(allowedVerbs[0] == types.Wildcard || slices.Contains(allowedVerbs, verb))
}

// compileMatchRules pre-compiles already-filtered RBAC rules. The cache map
// is shared across calls to deduplicate patterns that appear in both allow and
// deny rules (e.g., apiGroup="*").
func compileMatchRules(resources []types.KubernetesResource, cache map[string]*regexp.Regexp) ([]compiledMatchRule, error) {
	var rules []compiledMatchRule
	for _, r := range resources {
		apiGroupRe, err := compileCached(r.APIGroup, cache)
		if err != nil {
			return nil, err
		}
		nameRe, err := compileCached(r.Name, cache)
		if err != nil {
			return nil, err
		}
		nsRe, err := compileCached(r.Namespace, cache)
		if err != nil {
			return nil, err
		}

		rules = append(rules, compiledMatchRule{
			apiGroup:          apiGroupRe,
			name:              nameRe,
			namespace:         nsRe,
			requiresNamespace: r.Namespace != "" && r.Namespace != types.Wildcard,
		})
	}
	return rules, nil
}

// compileCached compiles a regex expression,
// caching the result in the given map to deduplicate patterns that appear in multiple rules (e.g., apiGroup="*").
func compileCached(expression string, cache map[string]*regexp.Regexp) (*regexp.Regexp, error) {
	if re, ok := cache[expression]; ok {
		return re, nil
	}
	re, err := utils.CompileExpression(expression)
	if err != nil {
		return nil, err
	}
	cache[expression] = re
	return re, nil
}

// match checks if a resource with the given name, namespace, and apiGroup is allowed by the precompiled RBAC rules.
func (m *fastResourceMatcher) match(name, namespace, apiGroup string) bool {
	for i := range m.denyRules {
		if m.denyRules[i].matches(name, namespace, apiGroup) {
			return false
		}
	}
	for i := range m.allowRules {
		if m.allowRules[i].matches(name, namespace, apiGroup) {
			return true
		}
	}
	return false
}

// matches checks whether a single compiled rule matches the given fields.
// This mirrors the "default" case in KubeResourceMatchesRegex.
func (r *compiledMatchRule) matches(name, namespace, apiGroup string) bool {
	if !r.apiGroup.MatchString(apiGroup) {
		return false
	}
	if !r.name.MatchString(name) {
		return false
	}
	// Mirror the check: if input namespace is empty but rule requires a
	// specific namespace, this rule cannot match.
	if namespace == "" && r.requiresNamespace {
		return false
	}
	return r.namespace.MatchString(namespace)
}
