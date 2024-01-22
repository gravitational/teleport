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

package utils

import (
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/api/types"
)

// ContainsExpansion returns true if value contains
// expansion syntax, e.g. $1 or ${10}
func ContainsExpansion(val string) bool {
	return reExpansion.FindStringIndex(val) != nil
}

// GlobToRegexp replaces glob-style standalone wildcard values
// with real .* regexp-friendly values, does not modify regexp-compatible values,
// quotes non-wildcard values
func GlobToRegexp(in string) string {
	return replaceWildcard.ReplaceAllString(regexp.QuoteMeta(in), "(.*)")
}

// ReplaceRegexp replaces value in string, accepts regular expression and simplified
// wildcard syntax, it has several important differences with standard lib
// regexp replacer:
// * Wildcard globs '*' are treated as regular expression .* expression
// * Expression is treated as regular expression if it starts with ^ and ends with $
// * Full match is expected, partial replacements ignored
// * If there is no match, returns a NotFound error
func ReplaceRegexp(expression string, replaceWith string, input string) (string, error) {
	expr, err := RegexpWithConfig(expression, RegexpConfig{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return ReplaceRegexpWith(expr, replaceWith, input)
}

// RegexpWithConfig compiles a regular expression given some configuration.
// There are several important differences with standard lib (see ReplaceRegexp).
func RegexpWithConfig(expression string, config RegexpConfig) (*regexp.Regexp, error) {
	if !strings.HasPrefix(expression, "^") || !strings.HasSuffix(expression, "$") {
		// replace glob-style wildcards with regexp wildcards
		// for plain strings, and quote all characters that could
		// be interpreted in regular expression
		expression = "^" + GlobToRegexp(expression) + "$"
	}
	if config.IgnoreCase {
		expression = "(?i)" + expression
	}
	expr, err := regexp.Compile(expression)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return expr, nil
}

// ReplaceRegexp replaces string in a given regexp.
func ReplaceRegexpWith(expr *regexp.Regexp, replaceWith string, input string) (string, error) {
	// if there is no match, return NotFound error
	index := expr.FindStringIndex(input)
	if index == nil {
		return "", trace.NotFound("no match found")
	}
	return expr.ReplaceAllString(input, replaceWith), nil
}

// RegexpConfig defines the configuration of the regular expression matcher
type RegexpConfig struct {
	// IgnoreCase specifies whether matching is case-insensitive
	IgnoreCase bool
}

// KubeResourceMatchesRegex checks whether the input matches any of the given
// expressions.
// This function returns as soon as it finds the first match or when MatchString
// returns an error.
// This function supports regex expressions in the Name and Namespace fields,
// but not for the Kind field.
// The wildcard (*) expansion is also supported.
func KubeResourceMatchesRegexWithVerbsCollector(input types.KubernetesResource, resources []types.KubernetesResource) (bool, []string, error) {
	verbs := map[string]struct{}{}
	matchedAny := false
	for _, resource := range resources {
		if input.Kind != resource.Kind && resource.Kind != types.Wildcard {
			continue
		}
		switch ok, err := MatchString(input.Name, resource.Name); {
		case err != nil:
			return false, nil, trace.Wrap(err)
		case !ok:
			continue
		}

		if ok, err := MatchString(input.Namespace, resource.Namespace); err != nil {
			return false, nil, trace.Wrap(err)
		} else if !ok {
			continue
		}
		matchedAny = true
		if len(resource.Verbs) > 0 && resource.Verbs[0] == types.Wildcard {
			return true, []string{types.Wildcard}, nil
		}
		for _, verb := range resource.Verbs {
			verbs[verb] = struct{}{}
		}
	}

	return matchedAny, maps.Keys(verbs), nil
}

const (
	// KubeCustomResource is the type that represents a Kubernetes
	// CustomResource object. These objects are special in that they do not exist
	// in the user's resources list, but their access is determined by the
	// access level of their namespace resource.
	KubeCustomResource = "CustomResource"
)

// KubeResourceMatchesRegex checks whether the input matches any of the given
// expressions.
// This function returns as soon as it finds the first match or when matchString
// returns an error.
// This function supports regex expressions in the Name and Namespace fields,
// but not for the Kind field.
// The wildcard (*) expansion is also supported.
// input is the resource we are checking for access.
// resources is a list of resources that the user has access to - collected from
// their roles that match the Kubernetes cluster where the resource is defined.
func KubeResourceMatchesRegex(input types.KubernetesResource, resources []types.KubernetesResource) (bool, error) {
	if len(input.Verbs) != 1 {
		return false, trace.BadParameter("only one verb is supported, input: %v", input.Verbs)
	}
	// isClusterWideResource is true if the resource is cluster-wide, e.g. a
	// namespace resource or a clusterrole.
	isClusterWideResource := slices.Contains(types.KubernetesClusterWideResourceKinds, input.Kind)

	verb := input.Verbs[0]
	// If the user is list/read/watch a namespace, they should be able to see the
	// namespace they have resources defined for.
	// This is a special case because we don't want to require the user to have
	// access to the namespace resource itself.
	// This is only allowed for the list/read/watch verbs because we don't want
	// to allow the user to create/update/delete a namespace they don't have
	// permissions for.
	targetsReadOnlyNamespace := input.Kind == types.KindKubeNamespace &&
		slices.Contains([]string{types.KubeVerbGet, types.KubeVerbList, types.KubeVerbWatch}, verb)

	for _, resource := range resources {
		// If the resource has a wildcard verb, it matches all verbs.
		// Otherwise, the resource must have the verb we're looking for otherwise
		// it doesn't match.
		// When the resource has a wildcard verb, we only allow one verb in the
		// resource input.
		if !isVerbAllowed(resource.Verbs, verb) {
			continue
		}
		switch {
		// If the user has access to a specific namespace, they should be able to
		// access all resources in that namespace.
		case resource.Kind == types.KindKubeNamespace && input.Namespace != "":
			// Access to custom resources is determined by the access level of the
			// namespace resource where the custom resource is defined.
			// This is a special case because custom resources are not defined in the
			// user's resources list.
			// Access to namspaced resources is determined by the access level of the
			// namespace resource where the resource is defined or by the access level
			// of the resource if supported.
			if ok, err := MatchString(input.Namespace, resource.Name); err != nil || ok {
				return ok, trace.Wrap(err)
			}
		case targetsReadOnlyNamespace && resource.Kind != types.KindKubeNamespace && resource.Namespace != "":
			// If the user requests a read-only namespace get/list/watch, they should
			// be able to see the list of namespaces they have resources defined in.
			// This means that if the user has access to pods in the "foo" namespace,
			// they should be able to see the "foo" namespace in the list of namespaces
			// but only if the request is read-only.
			if ok, err := MatchString(input.Name, resource.Namespace); err != nil || ok {
				return ok, trace.Wrap(err)
			}
		default:
			if input.Kind != resource.Kind && resource.Kind != types.Wildcard {
				continue
			}
			switch ok, err := MatchString(input.Name, resource.Name); {
			case err != nil:
				return false, trace.Wrap(err)
			case !ok:
				continue
			case ok && input.Namespace == "" && isClusterWideResource:
				return true, nil
			}
			if ok, err := MatchString(input.Namespace, resource.Namespace); err != nil || ok {
				return ok, trace.Wrap(err)
			}
		}
	}

	return false, nil
}

// isVerbAllowed returns true if the verb is allowed in the resource.
// If the resource has a wildcard verb, it matches all verbs, otherwise
// the resource must have the verb we're looking for.
func isVerbAllowed(allowedVerbs []string, verb string) bool {
	return len(allowedVerbs) != 0 && (allowedVerbs[0] == types.Wildcard || slices.Contains(allowedVerbs, verb))
}

// SliceMatchesRegex checks if input matches any of the expressions. The
// match is always evaluated as a regex either an exact match or regexp.
func SliceMatchesRegex(input string, expressions []string) (bool, error) {
	for _, expression := range expressions {
		result, err := MatchString(input, expression)
		if err != nil || result {
			return result, trace.Wrap(err)
		}
	}

	return false, nil
}

// RegexMatchesAny returns true if [expression] matches any element of
// [inputs]. [expression] support globbing ("env-*") or normal regexp syntax if
// surrounded with ^$ ("^env-.*$").
func RegexMatchesAny(inputs []string, expression string) (bool, error) {
	expr, err := compileRegexCached(expression)
	if err != nil {
		return false, trace.Wrap(err)
	}
	for _, input := range inputs {
		// Since the expression is always surrounded by ^ and $ this is an exact
		// match for either a plain string (for example ^hello$) or for a regexp
		// (for example ^hel*o$).
		if expr.MatchString(input) {
			return true, nil
		}
	}
	return false, nil
}

// mustCache initializes a new [lru.Cache] with the provided size.
// A panic will be triggered if the creation of the cache fails.
func mustCache[K comparable, V any](size int) *lru.Cache[K, V] {
	cache, err := lru.New[K, V](size)
	if err != nil {
		panic(err)
	}

	return cache
}

// exprCache interns compiled regular expressions created in MatchString
// to improve performance.
var exprCache = mustCache[string, *regexp.Regexp](1000)

// MatchString will match an input against the given expression. The expression is cached for later use.
func MatchString(input, expression string) (bool, error) {
	expr, err := compileRegexCached(expression)
	if err != nil {
		return false, trace.BadParameter(err.Error())
	}

	// Since the expression is always surrounded by ^ and $ this is an exact
	// match for either a plain string (for example ^hello$) or for a regexp
	// (for example ^hel*o$).
	return expr.MatchString(input), nil
}

// CompileExpression compiles the given regex expression with Teleport's custom globbing
// and quoting logic.
func CompileExpression(expression string) (*regexp.Regexp, error) {
	if !strings.HasPrefix(expression, "^") || !strings.HasSuffix(expression, "$") {
		// replace glob-style wildcards with regexp wildcards
		// for plain strings, and quote all characters that could
		// be interpreted in regular expression
		expression = "^" + GlobToRegexp(expression) + "$"
	}

	expr, err := regexp.Compile(expression)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return expr, nil
}

func compileRegexCached(expression string) (*regexp.Regexp, error) {
	if expr, ok := exprCache.Get(expression); ok {
		return expr, nil
	}

	expr, err := CompileExpression(expression)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	exprCache.Add(expression, expr)
	return expr, nil
}

var (
	replaceWildcard = regexp.MustCompile(`(\\\*)`)
	reExpansion     = regexp.MustCompile(`\$[^\$]+`)
)
