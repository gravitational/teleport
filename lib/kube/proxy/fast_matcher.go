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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// fastResourceMatcher is a precompiled per-request RBAC matcher that reduces per-item matching overhead.
// Instead of calling matchKubernetesResource per item (which does cache lookups, rule iteration, per-field matching),
// the fast matcher pre-filters rules by kind, verb, and API group at compile time and pre-compiles all regex patterns.
// Per-item cost becomes direct regex matching against only the relevant rules.
//
// The fast matcher handles the common "default" case in KubeResourceMatchesRegex.
// It cannot handle namespace special cases (when the requested kind is "namespaces"),
// so tryCompileFastMatcher returns nil for those requests and the caller falls back to matchKubernetesResource.
type fastResourceMatcher struct {
	allowRules []compiledMatchRule
	denyRules  []compiledMatchRule
}

// compiledMatchRule is a single RBAC rule with pre-compiled matchers
// and pre-filtered kind/verb/apiGroup (only rules matching the request are included).
type compiledMatchRule struct {
	apiGroup  fieldMatcher
	name      fieldMatcher
	namespace fieldMatcher
	// requiresNamespace is true when the original rule had a non-empty, non-wildcard namespace pattern.
	// When true, resources with an empty namespace cannot match this rule.
	requiresNamespace bool
}

// fieldMatcher matches a string either by exact literal comparison or by compiled regex.
type fieldMatcher struct {
	literal string         // set for exact match
	re      *regexp.Regexp // set for pattern match
}

func (f fieldMatcher) match(s string) bool {
	if f.re == nil {
		return f.literal == s
	}
	return f.re.MatchString(s)
}

// maxFastMatcherRules is the maximum number of RBAC rules
// (allowed + denied combined, after kind/verb filtering) for which the fast matcher is used.
// Beyond this threshold we fall back to the cached matchKubernetesResource path.
// Benchmarks show the fast matcher is faster even at 4000 rules, so this is a
// conservative safety margin rather than a measured crossover point.
// It may be removed in a follow-up once we gain more production confidence.
const maxFastMatcherRules = 200

// tryCompileFastMatcher attempts to compile a fast matcher from the given RBAC rules.
// Returns nil (without error) if the fast matcher cannot handle the request,
// signaling the caller to fall back to matchKubernetesResource.
func tryCompileFastMatcher(mr metaResource, allowed, denied []types.KubernetesResource) (*fastResourceMatcher, error) {
	// The fast matcher cannot handle namespace special cases in KubeResourceMatchesRegex
	// (read-only namespace visibility, namespace kind matching with different target selection).
	if mr.requestedResource.resourceKind == "namespaces" {
		return nil, nil
	}

	// Pre-filter rules that cannot match this request.
	// Kind, verb, API group, and namespace (when targeting a specific namespace)
	// are uniform for all items in a list response, so rules that don't match can be dropped.
	allowed = filterRules(mr, allowed)
	denied = filterRules(mr, denied)

	// If too many rules survive kind/verb filtering, fall back to per-item matching.
	if len(allowed)+len(denied) > maxFastMatcherRules {
		return nil, nil
	}

	return compileFastMatcher(allowed, denied)
}

// compileFastMatcher compiles a fast matcher from pre-filtered RBAC rules.
func compileFastMatcher(allowed, denied []types.KubernetesResource) (*fastResourceMatcher, error) {
	// Local cache for this compilation pass. Many rules share the same patterns (e.g., apiGroup="*", name="*"),
	// so this avoids compiling the same expression multiple times.
	compiled := make(map[string]*regexp.Regexp)

	allowRules, err := compileMatchRules(allowed, compiled)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	denyRules, err := compileMatchRules(denied, compiled)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &fastResourceMatcher{
		allowRules: allowRules,
		denyRules:  denyRules,
	}, nil
}

// filterRules returns the subset of rules that match the given request parameters.
func filterRules(mr metaResource, rules []types.KubernetesResource) []types.KubernetesResource {
	return slices.DeleteFunc(slices.Clone(rules), func(r types.KubernetesResource) bool {
		return !kindAllowed(r.Kind, mr.requestedResource.resourceKind) ||
			!verbAllowed(r.Verbs, mr.verb) ||
			!apiGroupAllowed(r.APIGroup, mr.requestedResource.apiGroup) ||
			!namespaceAllowed(r.Namespace, mr.requestedResource.namespace)
	})
}

func kindAllowed(ruleKind, requestedKind string) bool {
	return ruleKind == types.Wildcard || ruleKind == requestedKind
}

func verbAllowed(allowedVerbs []string, verb string) bool {
	return utils.IsVerbAllowed(allowedVerbs, verb)
}

func namespaceAllowed(ruleNamespace, requestedNamespace string) bool {
	if requestedNamespace == "" {
		// Cluster-wide request: all rules pass since items may come from any namespace.
		return true
	}
	// Empty rule namespace matches any namespace.
	if ruleNamespace == "" {
		return true
	}
	return patternCanMatch(ruleNamespace, requestedNamespace)
}

func apiGroupAllowed(ruleAPIGroup, requestedAPIGroup string) bool {
	return patternCanMatch(ruleAPIGroup, requestedAPIGroup)
}

func patternCanMatch(pattern, value string) bool {
	if isGlobOrRegexp(pattern) {
		return true
	}
	return pattern == value
}

func isGlobOrRegexp(expr string) bool {
	return strings.Contains(expr, "*") || utils.IsRegexp(expr)
}

// compileMatchRules pre-compiles already-filtered RBAC rules. The cache map
// is shared across calls to deduplicate patterns that appear in both allow and
// deny rules (e.g., apiGroup="*").
func compileMatchRules(resources []types.KubernetesResource, cache map[string]*regexp.Regexp) ([]compiledMatchRule, error) {
	var rules []compiledMatchRule
	for _, r := range resources {
		apiGroupM, err := compileFieldMatcher(r.APIGroup, cache)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		nameM, err := compileFieldMatcher(r.Name, cache)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		nsM, err := compileFieldMatcher(r.Namespace, cache)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rules = append(rules, compiledMatchRule{
			apiGroup:          apiGroupM,
			name:              nameM,
			namespace:         nsM,
			requiresNamespace: r.Namespace != "" && r.Namespace != types.Wildcard,
		})
	}
	return rules, nil
}

// compileFieldMatcher returns a fieldMatcher for the given expression.
// Literal expressions (no wildcards or regex) use direct string comparison.
// Patterns are compiled to regex, with results cached across rules.
func compileFieldMatcher(expression string, cache map[string]*regexp.Regexp) (fieldMatcher, error) {
	if !isGlobOrRegexp(expression) {
		return fieldMatcher{literal: expression}, nil
	}
	if re, ok := cache[expression]; ok {
		return fieldMatcher{re: re}, nil
	}
	re, err := utils.CompileExpression(expression)
	if err != nil {
		return fieldMatcher{}, trace.Wrap(err)
	}
	cache[expression] = re
	return fieldMatcher{re: re}, nil
}

// match checks if a resource with the given name, namespace, and apiGroup is allowed by the precompiled RBAC rules.
func (m *fastResourceMatcher) match(name, namespace, apiGroup string) (bool, error) {
	for i := range m.denyRules {
		if m.denyRules[i].matches(name, namespace, apiGroup) {
			return false, nil
		}
	}
	for i := range m.allowRules {
		if m.allowRules[i].matches(name, namespace, apiGroup) {
			return true, nil
		}
	}
	return false, nil
}

// matches checks whether a single compiled rule matches the given fields.
// This mirrors the "default" case in KubeResourceMatchesRegex.
func (r *compiledMatchRule) matches(name, namespace, apiGroup string) bool {
	if !r.apiGroup.match(apiGroup) {
		return false
	}
	if !r.name.match(name) {
		return false
	}
	if r.requiresNamespace && namespace == "" {
		return false
	}
	return r.namespace.match(namespace)
}
