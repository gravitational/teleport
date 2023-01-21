/*
Copyright 2017-2020 Gravitational, Inc.

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

// TODO(awly): combine Expression and Matcher. It should be possible to write:
// `{{regexp.match(email.local(external.trait_name))}}`
package parse

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"
)

// Expression is a string expression template
// that can interpolate to some variables.
type Expression struct {
	// prefix is a prefix of the expression
	prefix string
	// suffix is a suffix of the expression
	suffix string
	// expr is the expression AST
	expr Expr
}

// MatchExpression is a match expression.
type MatchExpression struct {
	// prefix is a prefix of the expression
	prefix string
	// suffix is a suffix of the expression
	suffix string
	// matcher is the matcher in the expression
	matcher Expr
}

var reVariable = regexp.MustCompile(
	// prefix is anything that is not { or }
	`^(?P<prefix>[^}{]*)` +
		// variable is anything in brackets {{}} that is not { or }
		`{{(?P<expression>\s*[^}{]*\s*)}}` +
		// prefix is anything that is not { or }
		`(?P<suffix>[^}{]*)$`,
)

// NewExpression parses expressions like {{external.foo}} or {{internal.bar}},
// or a literal value like "prod". Call Interpolate on the returned Expression
// to get the final value based on traits or other dynamic values.
func NewExpression(value string) (*Expression, error) {
	match := reVariable.FindStringSubmatch(value)
	if len(match) == 0 {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return nil, trace.BadParameter(
				"%q is using template brackets '{{' or '}}', however expression does not parse, make sure the format is {{expression}}",
				value,
			)
		}
		expr := &VarExpr{namespace: LiteralNamespace, name: value}
		return &Expression{expr: expr}, nil
	}

	prefix, value, suffix := match[1], match[2], match[3]
	expr, err := parse(value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if expr.Kind() != reflect.String {
		return nil, trace.BadParameter("%q does not evaluate to a string", value)
	}

	return &Expression{
		prefix: strings.TrimLeftFunc(prefix, unicode.IsSpace),
		suffix: strings.TrimRightFunc(suffix, unicode.IsSpace),
		expr:   expr,
	}, nil
}

// Interpolate interpolates the variable adding prefix and suffix if present.
// The returned error is trace.NotFound in case the expression contains a variable
// and this variable is not found on any trait, nil in case of success,
// and BadParameter otherwise.
func (e *Expression) Interpolate(varValidation func(namespace, name string) error, traits map[string][]string) ([]string, error) {
	ctx := EvaluateContext{
		VarValue: func(v VarExpr) ([]string, error) {
			if err := varValidation(v.namespace, v.name); err != nil {
				return nil, trace.Wrap(err)
			}

			values, ok := traits[v.name]
			if !ok {
				return nil, trace.NotFound("variable not found: %s", v)
			}
			return values, nil
		},
	}

	result, err := e.expr.Evaluate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l, ok := result.([]string)
	if !ok {
		panic(fmt.Sprintf("unexpected string expression evaluation result type %T (this is a bug)", result))
	}

	var out []string
	for _, val := range l {
		if len(val) > 0 {
			out = append(out, e.prefix+val+e.suffix)
		}
	}
	return out, nil
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

		matcher, err := buildRegexpMatchExprFromLit(value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &MatchExpression{matcher: matcher}, nil
	}

	prefix, value, suffix := match[1], match[2], match[3]
	matcher, err := parse(value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if matcher.Kind() != reflect.Bool {
		return nil, trace.BadParameter("%q does not evaluate to a boolean", value)
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

	ctx := EvaluateContext{
		MatcherInput: in,
	}

	// Ignore err as there's no variable interpolation for now,
	// and thus `Evaluate` cannot error for matchers.
	result, _ := e.matcher.Evaluate(ctx)
	b, ok := result.(bool)
	if !ok {
		panic(fmt.Sprintf("unexpected match expression evaluation result type %T (this is a bug)", result))
	}
	return b
}

const (
	// LiteralNamespace is a namespace for Expressions that always return
	// static literal values.
	LiteralNamespace = "literal"
	// EmailLocalFnName is a name for email.local function
	EmailLocalFnName = "email.local"
	// RegexpMatchFnName is a name for regexp.match function.
	RegexpMatchFnName = "regexp.match"
	// RegexpNotMatchFnName is a name for regexp.not_match function.
	RegexpNotMatchFnName = "regexp.not_match"
	// RegexpReplaceFnName is a name for regexp.replace function.
	RegexpReplaceFnName = "regexp.replace"
)

// parse uses predicate in order to parse the expression.
func parse(exprStr string) (Expr, error) {
	parser, err := predicate.NewParser(predicate.Def{
		GetIdentifier: buildVarExpr,
		GetProperty:   buildVarExprFromProperty,
		Functions: map[string]interface{}{
			EmailLocalFnName:     buildEmailLocalExpr,
			RegexpReplaceFnName:  buildRegexpReplaceExpr,
			RegexpMatchFnName:    buildRegexpMatchExpr,
			RegexpNotMatchFnName: buildRegexpNotMatchExpr,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := parser.Parse(exprStr)
	if err != nil {
		return nil, trace.BadParameter("failed to parse: %q, error: %s", exprStr, err)
	}

	expr, ok := result.(Expr)
	if !ok {
		return nil, trace.BadParameter("failed to parse: %q, unexpected parser result type %T", exprStr, result)
	}

	if err := validateExpr(expr); err != nil {
		return nil, trace.Wrap(err)
	}

	return expr, nil
}
