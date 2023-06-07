// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"

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
func KubeResourceMatchesRegex(input types.KubernetesResource, resources []types.KubernetesResource) (bool, error) {
	for _, resource := range resources {
		// TODO(tigrato): evaluate if we should support wildcards as well
		// for future compatibility.
		if input.Kind != resource.Kind {
			continue
		}
		switch ok, err := MatchString(input.Name, resource.Name); {
		case err != nil:
			return false, trace.Wrap(err)
		case !ok:
			continue
		}
		if ok, err := MatchString(input.Namespace, resource.Namespace); err != nil || ok {
			return ok, trace.Wrap(err)
		}
	}

	return false, nil
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
