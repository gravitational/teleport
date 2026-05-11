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

package policy

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// Env holds the runtime bindings exposed to where: expressions.
type Env struct {
	UserName      string
	UserRoles     []string
	Path          map[string]string
	RequestMethod string
	RequestPath   string
}

// CompilePredicate parses a where: expression at policy-load time.
func CompilePredicate(expression string) (Predicate, error) {
	if err := validateRegexpMatchCalls(expression); err != nil {
		return nil, trace.Wrap(err)
	}
	p, err := getParser()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return p.Parse(expression)
}

// regexpMatchCall matches the start of a regexp.match invocation.
var regexpMatchCall = regexp.MustCompile(`regexp\.match\s*\(`)

// validateRegexpMatchCalls forces the first argument to regexp.match to
// be a string literal and pre-compiles the pattern so syntax errors
// surface at policy load instead of at first request. Without this an
// operator could accidentally hand attackers control of the compile via
// a path capture.
func validateRegexpMatchCalls(expression string) error {
	for _, idx := range regexpMatchCall.FindAllStringIndex(expression, -1) {
		rest := strings.TrimLeftFunc(expression[idx[1]:], unicode.IsSpace)
		if rest == "" {
			return trace.BadParameter("regexp.match: missing first argument")
		}
		quote := rune(rest[0])
		if quote != '"' && quote != '\'' && quote != '`' {
			return trace.BadParameter("regexp.match: first argument must be a string literal, got %q...", firstToken(rest))
		}
		pattern, err := parseGoStringLiteral(rest)
		if err != nil {
			return trace.Wrap(err, "regexp.match: parsing first argument")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return trace.Wrap(err, "regexp.match: compiling pattern %q", pattern)
		}
		regexCache.Store(pattern, re)
	}
	return nil
}

// parseGoStringLiteral parses a leading Go-style string literal at the
// start of s and returns its unquoted value. Supports the same three
// quoting forms strconv.Unquote accepts.
func parseGoStringLiteral(s string) (string, error) {
	quote := s[0]
	if quote == '`' {
		end := strings.IndexByte(s[1:], '`')
		if end < 0 {
			return "", trace.BadParameter("unterminated raw string literal")
		}
		return s[1 : 1+end], nil
	}
	// Walk the literal honoring escape sequences so embedded quotes do
	// not terminate it early.
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // skip the escaped byte
		case quote:
			return strconv.Unquote(s[:i+1])
		}
	}
	return "", trace.BadParameter("unterminated string literal")
}

func firstToken(s string) string {
	end := strings.IndexAny(s, " \t\n,)")
	if end < 0 {
		end = len(s)
	}
	if end > 16 {
		end = 16
	}
	return s[:end]
}

var (
	parserOnce sync.Once
	parser     *typical.Parser[Env, bool]
	parserErr  error
	regexCache sync.Map // map[string]*regexp.Regexp
)

func getParser() (*typical.Parser[Env, bool], error) {
	parserOnce.Do(func() {
		parser, parserErr = newParser()
	})
	return parser, parserErr
}

func newParser() (*typical.Parser[Env, bool], error) {
	spec := typical.ParserSpec[Env]{
		Variables: map[string]typical.Variable{
			"user.name": typical.DynamicVariable(func(e Env) (string, error) {
				return e.UserName, nil
			}),
			"user.roles": typical.DynamicVariable(func(e Env) ([]string, error) {
				return e.UserRoles, nil
			}),
			"path": typical.DynamicMapFunction(func(e Env, key string) (string, error) {
				return e.Path[key], nil
			}),
			"request.method": typical.DynamicVariable(func(e Env) (string, error) {
				return e.RequestMethod, nil
			}),
			"request.path": typical.DynamicVariable(func(e Env) (string, error) {
				return e.RequestPath, nil
			}),
		},
		Functions: map[string]typical.Function{
			"contains": typical.BinaryFunction[Env](func(list []string, item string) (bool, error) {
				return slices.Contains(list, item), nil
			}),
			"contains_any": typical.BinaryFunction[Env](func(list []string, items []string) (bool, error) {
				for _, it := range items {
					if slices.Contains(list, it) {
						return true, nil
					}
				}
				return false, nil
			}),
			"contains_all": typical.BinaryFunction[Env](func(list []string, items []string) (bool, error) {
				for _, it := range items {
					if !slices.Contains(list, it) {
						return false, nil
					}
				}
				return true, nil
			}),
			"regexp.match": typical.BinaryFunction[Env](regexpMatch),
			"strings.upper": typical.UnaryFunction[Env](func(s string) (string, error) {
				return strings.ToUpper(s), nil
			}),
			"strings.lower": typical.UnaryFunction[Env](func(s string) (string, error) {
				return strings.ToLower(s), nil
			}),
		},
	}
	return typical.NewParser[Env, bool](spec)
}

// regexpMatch compiles each unique pattern once via regexCache and
// reuses it for subsequent requests. validateRegexpMatchCalls has
// already rejected non-literal patterns at load time, so the cache key
// space is bounded by the policy library.
func regexpMatch(pattern, s string) (bool, error) {
	if cached, ok := regexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp).MatchString(s), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, trace.Wrap(err)
	}
	actual, _ := regexCache.LoadOrStore(pattern, re)
	return actual.(*regexp.Regexp).MatchString(s), nil
}
