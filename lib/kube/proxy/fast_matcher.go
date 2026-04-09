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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// fastMatcher is a precompiled per-request RBAC matcher that reduces per-item matching overhead.
// Instead of calling matchKubernetesResource per item (which does cache lookups, rule iteration, per-field matching),
// the fast matcher resolves constant fields at compile time and only checks per-item fields during matching.
//
// Of the five rule fields:
//   - Kind: exact-match or wildcard only, constant per request, resolved at compile time.
//   - Verb: exact-match or wildcard only, constant per request, resolved at compile time.
//   - APIGroup: supports regex/glob, constant per request, resolved at compile time.
//   - Namespace: supports regex/glob, varies per item, checked per item.
//   - Name: supports regex/glob, varies per item, checked per item.
//
// The fast matcher handles the common "default" case in KubeResourceMatchesRegex.
// It cannot handle namespace special cases (when the requested kind is "namespaces").
type fastMatcher struct {
	allowRules []compiledMatchRule
	denyRules  []compiledMatchRule
}

// compiledMatchRule is a single RBAC rule with pre-compiled name and namespace matchers.
// Kind, verb, and apiGroup have already been resolved during rule filtering.
type compiledMatchRule struct {
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

// newFastMatcher filters rules by request parameters and compiles a fast matcher.
func newFastMatcher(mr metaResource, allowed, denied []types.KubernetesResource) (*fastMatcher, error) {
	// Local cache for this compilation pass.
	// Many rules share the same patterns, so this avoids compiling the same expression multiple times.
	cache := make(map[string]*regexp.Regexp)

	filteredAllowed, err := filterRules(mr, allowed, cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filteredDenied, err := filterRules(mr, denied, cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowRules, err := compileMatchRules(filteredAllowed, cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	denyRules, err := compileMatchRules(filteredDenied, cache)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &fastMatcher{
		allowRules: allowRules,
		denyRules:  denyRules,
	}, nil
}

// filterRules returns the subset of rules that match the given request parameters.
func filterRules(mr metaResource, rules []types.KubernetesResource, cache map[string]*regexp.Regexp) ([]types.KubernetesResource, error) {
	filtered := make([]types.KubernetesResource, 0, len(rules))
	for _, r := range rules {
		if !kindAllowed(r.Kind, mr.requestedResource.resourceKind) {
			continue
		}
		if !verbAllowed(r.Verbs, mr.verb) {
			continue
		}
		if !namespaceAllowed(r.Namespace, mr.requestedResource.namespace) {
			continue
		}
		match, err := apiGroupMatches(r.APIGroup, mr.requestedResource.apiGroup, cache)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !match {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered, nil
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
	// Empty rule namespace targets cluster-wide resources.
	// Keep it during pre-filtering; the compiled matcher only matches items with empty namespace.
	if ruleNamespace == "" {
		return true
	}
	// Pattern namespaces are kept since they need compilation.
	if isGlobOrRegexp(ruleNamespace) {
		return true
	}
	return ruleNamespace == requestedNamespace
}

func apiGroupMatches(ruleAPIGroup, requestedAPIGroup string, cache map[string]*regexp.Regexp) (bool, error) {
	if !isGlobOrRegexp(ruleAPIGroup) {
		return ruleAPIGroup == requestedAPIGroup, nil
	}
	if re, ok := cache[ruleAPIGroup]; ok {
		return re.MatchString(requestedAPIGroup), nil
	}
	re, err := utils.CompileExpression(ruleAPIGroup)
	if err != nil {
		return false, trace.Wrap(err)
	}
	cache[ruleAPIGroup] = re
	return re.MatchString(requestedAPIGroup), nil
}

func isGlobOrRegexp(expr string) bool {
	return strings.Contains(expr, "*") || utils.IsRegexp(expr)
}

func compileMatchRules(resources []types.KubernetesResource, cache map[string]*regexp.Regexp) ([]compiledMatchRule, error) {
	rules := make([]compiledMatchRule, 0, len(resources))
	for _, r := range resources {
		nameM, err := compileFieldMatcher(r.Name, cache)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		nsM, err := compileFieldMatcher(r.Namespace, cache)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		rules = append(rules, compiledMatchRule{
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

// match checks if a resource with the given name and namespace is allowed by the precompiled RBAC rules.
func (m *fastMatcher) match(name, namespace string) (bool, error) {
	for i := range m.denyRules {
		if m.denyRules[i].matches(name, namespace) {
			return false, nil
		}
	}
	for i := range m.allowRules {
		if m.allowRules[i].matches(name, namespace) {
			return true, nil
		}
	}
	return false, nil
}

// matches checks whether a single compiled rule matches the given fields.
// This mirrors the "default" case in KubeResourceMatchesRegex.
// Kind, verb, and apiGroup are already resolved during rule filtering.
func (r *compiledMatchRule) matches(name, namespace string) bool {
	if !r.name.match(name) {
		return false
	}
	if r.requiresNamespace && namespace == "" {
		return false
	}
	return r.namespace.match(namespace)
}
