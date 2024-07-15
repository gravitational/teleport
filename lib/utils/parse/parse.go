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

// TODO(nklaassen): evaluate the risks and utility of allowing traits to be used
// as regular expressions. The only thing blocking this today is that all trait
// values are lists and the regex must be a single value. It could be possible
// to write:
// `{{regexp.match(email.local(head(external.trait_name)))}}`
package parse

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/typical"
)

const (
	// EmailLocalFnName is a name for email.local function
	EmailLocalFnName = "email.local"
	// RegexpMatchFnName is a name for regexp.match function.
	RegexpMatchFnName = "regexp.match"
	// RegexpNotMatchFnName is a name for regexp.not_match function.
	RegexpNotMatchFnName = "regexp.not_match"
	// RegexpReplaceFnName is a name for regexp.replace function.
	RegexpReplaceFnName = "regexp.replace"
)

var (
	traitsTemplateParser = mustNewTraitsTemplateParser()

	matcherParser = mustNewMatcherParser()

	reVariable = regexp.MustCompile(
		// prefix is anything that is not { or }
		`^(?P<prefix>[^}{]*)` +
			// variable is anything in brackets {{}} that is not { or }
			`{{(?P<expression>\s*[^}{]*\s*)}}` +
			// suffix is anything that is not { or }
			`(?P<suffix>[^}{]*)$`,
	)
)

// TraitsTemplateExpression can interpolate user trait values into a string
// template to produce some values.
type TraitsTemplateExpression struct {
	// prefix is a prefix of the expression
	prefix string
	// suffix is a suffix of the expression
	suffix string
	// expr is the expression AST
	expr traitsTemplateExpression
}

// NewTraitsTemplateExpression parses expressions like {{external.foo}} or {{internal.bar}},
// or a literal value like "prod". Call Interpolate on the returned Expression
// to get the final value based on user traits.
func NewTraitsTemplateExpression(value string) (*TraitsTemplateExpression, error) {
	match := reVariable.FindStringSubmatch(value)
	if len(match) == 0 {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return nil, trace.BadParameter(
				"%q is using template brackets '{{' or '}}', however expression does not parse, make sure the format is {{expression}}",
				value,
			)
		}
		expr := typical.LiteralExpr[traitsTemplateEnv, []string]{
			Value: []string{value},
		}
		return &TraitsTemplateExpression{expr: expr}, nil
	}

	prefix, value, suffix := match[1], match[2], match[3]

	expr, err := parseTraitsTemplateExpression(value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &TraitsTemplateExpression{
		prefix: strings.TrimLeftFunc(prefix, unicode.IsSpace),
		suffix: strings.TrimRightFunc(suffix, unicode.IsSpace),
		expr:   expr,
	}, nil
}

// Interpolate interpolates the variable adding prefix and suffix if present.
// The returned error is trace.NotFound in case the expression contains a variable
// and this variable is not found on any trait, nil in case of success,
// and BadParameter otherwise.
func (e *TraitsTemplateExpression) Interpolate(varValidation func(namespace, name string) error, traits map[string][]string) ([]string, error) {
	result, err := e.expr.Evaluate(traitsTemplateEnv{
		traits:         traits,
		traitValidator: varValidation,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []string
	for _, val := range result {
		// Filter out values that mapped to the empty string.
		if len(val) > 0 {
			out = append(out, e.prefix+val+e.suffix)
		}
	}
	return out, nil
}

type traitsTemplateEnv struct {
	traits         map[string][]string
	traitValidator func(namespace, name string) error
}

type traitsTemplateExpression typical.Expression[traitsTemplateEnv, []string]

func parseTraitsTemplateExpression(exprString string) (traitsTemplateExpression, error) {
	expr, err := traitsTemplateParser.Parse(exprString)
	return expr, trace.Wrap(err)
}

func mustNewTraitsTemplateParser() *typical.CachedParser[traitsTemplateEnv, []string] {
	parser, err := newTraitsTemplateParser()
	if err != nil {
		panic(trace.Wrap(err, "failed to create template parser (this is a bug)"))
	}
	return parser
}

func newTraitsTemplateParser() (*typical.CachedParser[traitsTemplateEnv, []string], error) {
	traitsVariable := func(name string) typical.Variable {
		return typical.DynamicMapFunction(func(e traitsTemplateEnv, key string) ([]string, error) {
			if e.traitValidator != nil {
				if err := e.traitValidator(name, key); err != nil {
					return nil, trace.Wrap(err)
				}
			}
			values, ok := e.traits[key]
			if !ok {
				return nil, trace.NotFound("trait not found: %s.%s", name, key)
			}
			return values, nil
		})
	}

	parser, err := typical.NewCachedParser[traitsTemplateEnv, []string](typical.ParserSpec[traitsTemplateEnv]{
		Variables: map[string]typical.Variable{
			"external": traitsVariable("external"),
			"internal": traitsVariable("internal"),
		},
		Functions: map[string]typical.Function{
			EmailLocalFnName:    typical.UnaryFunction[traitsTemplateEnv](EmailLocal),
			RegexpReplaceFnName: typical.TernaryFunction[traitsTemplateEnv](RegexpReplace),
		},
	}, typical.WithInvalidNamespaceHack())
	return parser, trace.Wrap(err)
}

// EmailLocal returns a new list which is a result of getting the local part of
// each email from the input list.
func EmailLocal(inputs []string) ([]string, error) {
	return stringListMap(inputs, func(email string) (string, error) {
		if email == "" {
			return "", trace.BadParameter(
				"found empty %q argument",
				EmailLocalFnName,
			)
		}
		addr, err := mail.ParseAddress(email)
		if err != nil {
			return "", trace.BadParameter(
				"failed to parse %q argument %q: %s",
				EmailLocalFnName,
				email,
				err,
			)
		}
		parts := strings.SplitN(addr.Address, "@", 2)
		if len(parts) != 2 {
			return "", trace.BadParameter(
				"could not find local part in %q argument %q, %q",
				EmailLocalFnName,
				email,
				addr.Address,
			)
		}
		return parts[0], nil
	})
}

// RegexpReplace returns a new list which is the result of replacing each instance
// of [match] with [replacement] for each item in the input list.
func RegexpReplace(inputs []string, match string, replacement string) ([]string, error) {
	re, err := newRegexp(match, false)
	if err != nil {
		return nil, trace.Wrap(err, "invalid regexp %q", match)
	}
	return stringListMap(inputs, func(in string) (string, error) {
		// Filter out inputs which do not match the regexp at all.
		if !re.MatchString(in) {
			return "", nil
		}
		return re.ReplaceAllString(in, replacement), nil
	})
}

// MatchExpression is a match expression.
type MatchExpression struct {
	// prefix is a prefix of the expression
	prefix string
	// suffix is a suffix of the expression
	suffix string
	// matcher is the matcher in the expression
	matcher Matcher
}

// Matcher matches strings against some internal criteria (e.g. a regexp)
type Matcher interface {
	Match(in string) bool
}

// MatcherFn converts function to a matcher interface
type MatcherFn func(in string) bool

// Match matches string against a regexp
func (fn MatcherFn) Match(in string) bool {
	return fn(in)
}

// NewAnyMatcher returns a matcher function based
// on incoming values
func NewAnyMatcher(in []string) (Matcher, error) {
	matchers := make([]Matcher, len(in))
	for i, v := range in {
		m, err := NewMatcher(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		matchers[i] = m
	}
	return MatcherFn(func(in string) bool {
		for _, m := range matchers {
			if m.Match(in) {
				return true
			}
		}
		return false
	}), nil
}

// NewMatcher parses a matcher expression. Currently supported expressions:
// - string literal: `foo`
// - wildcard expression: `*` or `foo*bar`
// - regexp expression: `^foo$`
// - regexp function calls:
//   - positive match: `{{regexp.match("foo.*")}}`
//   - negative match: `{{regexp.not_match("foo.*")}}`
//
// These expressions do not support variable interpolation (e.g.
// `{{internal.logins}}`), like Expression does.
func NewMatcher(value string) (*MatchExpression, error) {
	match := reVariable.FindStringSubmatch(value)
	if len(match) == 0 {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return nil, trace.BadParameter(
				"%q is using template brackets '{{' or '}}', however expression does not parse, make sure the format is {{expression}}",
				value,
			)
		}

		re, err := newRegexp(value, true)
		if err != nil {
			return nil, trace.Wrap(err, "parsing match expression")
		}
		return &MatchExpression{
			matcher: matcher{re},
		}, nil
	}

	prefix, value, suffix := match[1], match[2], match[3]
	matcher, err := parseMatcherExpression(value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &MatchExpression{
		prefix:  prefix,
		suffix:  suffix,
		matcher: matcher,
	}, nil
}

func (e *MatchExpression) Match(in string) bool {
	if !strings.HasPrefix(in, e.prefix) || !strings.HasSuffix(in, e.suffix) {
		return false
	}
	in = strings.TrimPrefix(in, e.prefix)
	in = strings.TrimSuffix(in, e.suffix)

	return e.matcher.Match(in)
}

// match expressions currently have no environment (you can't access any traits
// or other variables).
type matcherEnv struct{}

func parseMatcherExpression(raw string) (Matcher, error) {
	matchExpr, err := matcherParser.Parse(raw)
	if err != nil {
		return nil, trace.Wrap(err, "parsing match expression")
	}
	matcher, err := matchExpr.Evaluate(matcherEnv{})
	return matcher, trace.Wrap(err, "evaluating match expression")
}

func mustNewMatcherParser() *typical.CachedParser[matcherEnv, Matcher] {
	parser, err := newMatcherParser()
	if err != nil {
		panic(trace.Wrap(err, "failed to create match parser (this is a bug)"))
	}
	return parser
}

func newMatcherParser() (*typical.CachedParser[matcherEnv, Matcher], error) {
	parser, err := typical.NewCachedParser[matcherEnv, Matcher](typical.ParserSpec[matcherEnv]{
		Functions: map[string]typical.Function{
			RegexpMatchFnName:    typical.UnaryFunction[matcherEnv](regexpMatch),
			RegexpNotMatchFnName: typical.UnaryFunction[matcherEnv](regexpNotMatch),
		},
	})
	return parser, trace.Wrap(err)
}

func regexpMatch(match string) (Matcher, error) {
	re, err := newRegexp(match, false)
	if err != nil {
		return nil, trace.Wrap(err, "parsing argument to regexp.match")
	}
	return matcher{re}, nil
}

func regexpNotMatch(match string) (Matcher, error) {
	re, err := newRegexp(match, false)
	if err != nil {
		return nil, trace.Wrap(err, "parsing argument to regexp.not_match")
	}
	return notMatcher{re}, nil
}

type matcher struct {
	re *regexp.Regexp
}

func (m matcher) Match(in string) bool {
	return m.re.MatchString(in)
}

type notMatcher struct {
	re *regexp.Regexp
}

func (n notMatcher) Match(in string) bool {
	return !n.re.MatchString(in)
}

func stringListMap(inputs []string, f func(string) (string, error)) ([]string, error) {
	out := make([]string, 0, len(inputs))
	for _, input := range inputs {
		mapped, err := f(input)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Filter out values that mapped to the empty string.
		if len(mapped) == 0 {
			continue
		}
		out = append(out, mapped)
	}
	return out, nil
}

func newRegexp(raw string, escape bool) (*regexp.Regexp, error) {
	if escape {
		if !strings.HasPrefix(raw, "^") || !strings.HasSuffix(raw, "$") {
			// replace glob-style wildcards with regexp wildcards
			// for plain strings, and quote all characters that could
			// be interpreted in regular expression
			raw = "^" + utils.GlobToRegexp(raw) + "$"
		}
	}

	re, err := regexp.Compile(raw)
	if err != nil {
		return nil, trace.BadParameter(
			"failed to parse regexp %q: %v",
			raw,
			err,
		)
	}
	return re, nil
}
