/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parse

import (
	"fmt"
	"net/mail"
	"reflect"
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// Expr is a node in the AST.
type Expr interface {
	// Kind indicates the expression kind.
	Kind() reflect.Kind
	// Evaluate evaluates the expression given the evaluation context.
	Evaluate(ctx EvaluateContext) (any, error)
}

// EvaluateContext is the evaluation context.
type EvaluateContext struct {
	// VarValue returns a list of values that a variable has.
	VarValue func(VarExpr) ([]string, error)
	// MatcherInput is the input to matchers.
	MatcherInput string
}

// StringLitExpr encodes a string literal expression.
type StringLitExpr struct {
	value string
}

// VarExpr encodes a variable expression with the form "namespace.name".
type VarExpr struct {
	namespace string
	name      string
}

// EmailLocalExpr encodes an expression with the form "email.local(expr)".
type EmailLocalExpr struct {
	email Expr
}

// RegexpReplaceExpr encodes an expression with the form "regexp.replace(expr, string, string)".
type RegexpReplaceExpr struct {
	source      Expr
	re          *regexp.Regexp
	replacement string
}

// RegexpMatchExpr encodes an expression with the form "regexp.match(string)".
type RegexpMatchExpr struct {
	re *regexp.Regexp
}

// RegexpNotMatchExpr encodes an expression with the form "regexp.not_match(string)".
type RegexpNotMatchExpr struct {
	re *regexp.Regexp
}

// String is the string representation of StringLitExpr.
func (e *StringLitExpr) String() string {
	return fmt.Sprintf("%q", e.value)
}

// String is the string representation of VarExpr.
func (e *VarExpr) String() string {
	return fmt.Sprintf("%s.%s", e.namespace, e.name)
}

// String is the string representation of EmailLocalExpr.
func (e *EmailLocalExpr) String() string {
	return fmt.Sprintf("%s.%s(%s)", EmailNamespace, EmailLocalFnName, e.email)
}

// String is the string representation of RegexpReplaceExpr.
func (e *RegexpReplaceExpr) String() string {
	return fmt.Sprintf("%s.%s(%s, %q, %q)", RegexpNamespace, RegexpReplaceFnName, e.source, e.re, e.replacement)
}

// String is the string representation of RegexpMatchExpr.
func (e *RegexpMatchExpr) String() string {
	return fmt.Sprintf("%s.%s(%q)", RegexpNamespace, RegexpMatchFnName, e.re.String())
}

// String is the string representation of RegexpNotMatchExpr.
func (e *RegexpNotMatchExpr) String() string {
	return fmt.Sprintf("%s.%s(%q)", RegexpNamespace, RegexpNotMatchFnName, e.re.String())
}

// Kind indicates the StringLitExpr kind.
func (e *StringLitExpr) Kind() reflect.Kind {
	return reflect.String
}

// Kind indicates the VarExpr kind.
func (e *VarExpr) Kind() reflect.Kind {
	return reflect.String
}

// Kind indicates the EmailLocalExpr kind.
func (e *EmailLocalExpr) Kind() reflect.Kind {
	return reflect.String
}

// Kind indicates the RegexpReplaceExpr kind.
func (e *RegexpReplaceExpr) Kind() reflect.Kind {
	return reflect.String
}

// Kind indicates the RegexpMatchExpr kind.
func (e *RegexpMatchExpr) Kind() reflect.Kind {
	return reflect.Bool
}

// Kind indicates the RegexpNotMatchExpr kind.
func (e *RegexpNotMatchExpr) Kind() reflect.Kind {
	return reflect.Bool
}

// Evaluate evaluates the StringLitExpr given the evaluation context.
func (e *StringLitExpr) Evaluate(ctx EvaluateContext) (any, error) {
	return []string{e.value}, nil
}

// Evaluate evaluates the VarExpr given the evaluation context.
func (e *VarExpr) Evaluate(ctx EvaluateContext) (any, error) {
	if e.namespace == LiteralNamespace {
		return []string{e.name}, nil
	}
	return ctx.VarValue(*e)
}

// Evaluate evaluates the EmailLocalExpr given the evaluation context.
func (e *EmailLocalExpr) Evaluate(ctx EvaluateContext) (any, error) {
	input, err := e.email.Evaluate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return stringListMap(input, func(email string) (string, error) {
		if email == "" {
			return "", trace.BadParameter(
				"found empty \"%s.%s\" argument",
				EmailNamespace,
				EmailLocalFnName,
			)
		}
		addr, err := mail.ParseAddress(email)
		if err != nil {
			return "", trace.BadParameter(
				"failed to parse \"%s.%s\" argument %q: %s",
				EmailNamespace,
				EmailLocalFnName,
				email,
				err,
			)
		}
		parts := strings.SplitN(addr.Address, "@", 2)
		if len(parts) != 2 {
			return "", trace.BadParameter(
				"could not find local part in \"%s.%s\" argument %q, %q",
				EmailNamespace,
				EmailLocalFnName,
				email,
				addr.Address,
			)
		}
		return parts[0], nil
	})
}

// Evaluate evaluates the RegexpReplaceExpr given the evaluation context.
func (e *RegexpReplaceExpr) Evaluate(ctx EvaluateContext) (any, error) {
	input, err := e.source.Evaluate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return stringListMap(input, func(in string) (string, error) {
		// filter out inputs which do not match the regexp at all
		if !e.re.MatchString(in) {
			return "", nil
		}
		return e.re.ReplaceAllString(in, e.replacement), nil
	})
}

// Evaluate evaluates the RegexpMatchExpr given the evaluation context.
func (e *RegexpMatchExpr) Evaluate(ctx EvaluateContext) (any, error) {
	return e.re.MatchString(ctx.MatcherInput), nil
}

// Evaluate evaluates the RegexpNotMatchExpr given the evaluation context.
func (e *RegexpNotMatchExpr) Evaluate(ctx EvaluateContext) (any, error) {
	return !e.re.MatchString(ctx.MatcherInput), nil
}

// stringListMap maps a list of strings.
func stringListMap(input any, f func(string) (string, error)) ([]string, error) {
	v, ok := input.([]string)
	if !ok {
		return nil, trace.BadParameter("expected []string, got %T", input)
	}

	out := make([]string, 0, len(v))
	for _, str := range v {
		v, err := f(str)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, v)
	}
	return out, nil
}

// buildStringLit builds a StringLitExpr.
func buildStringLit(value string) (Expr, error) {
	return &StringLitExpr{value: value}, nil
}

// buildVarExpr builds a VarExpr.
func buildVarExpr(fields []string) (Expr, error) {
	if len(fields) != 2 {
		return nil, trace.BadParameter(
			"found variable %q with %d fields, expected 2",
			strings.Join(fields, "."),
			len(fields),
		)
	}
	switch fields[0] {
	case LiteralNamespace, teleport.TraitInternalPrefix, teleport.TraitExternalPrefix:
		return &VarExpr{namespace: fields[0], name: fields[1]}, nil
	default:
		return nil, trace.BadParameter(
			"found variable %q with namespace %q, expected one of: %q, %q, %q",
			strings.Join(fields, "."),
			fields[0],
			LiteralNamespace,
			teleport.TraitInternalPrefix,
			teleport.TraitExternalPrefix,
		)
	}
}

// buildCallExpr builds one of the following:
// - EmailLocalExpr
// - RegexpReplaceExpr
// - RegexpMatchExpr
// - RegexpNotMatchExpr
func buildCallExpr(fields []string, args []Expr) (Expr, error) {
	if len(fields) != 2 {
		return nil, trace.BadParameter(
			"found function %q with %d fields, expected 2",
			strings.Join(fields, "."),
			len(fields),
		)
	}
	switch fields[0] {
	case EmailNamespace:
		return buildEmailExpr(fields[1], args)
	case RegexpNamespace:
		return buildRegexpExpr(fields[1], args)
	default:
		return nil, trace.BadParameter(
			"found function %q with namespace %q, expected one of: %q, %q",
			strings.Join(fields, "."),
			fields[0],
			EmailNamespace,
			RegexpNamespace,
		)
	}
}

// buildEmailExpr builds one of the following:
// - EmailLocalExpr
func buildEmailExpr(name string, args []Expr) (Expr, error) {
	switch name {
	case EmailLocalFnName:
		return buildEmailLocalExpr(args)
	default:
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with name %q, expected one of: %q",
			EmailNamespace,
			name,
			name,
			EmailLocalFnName,
		)
	}
}

// buildEmailLocalExpr builds a EmailLocalExpr.
func buildEmailLocalExpr(args []Expr) (Expr, error) {
	if len(args) != 1 {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with %d arguments, expected 1",
			EmailNamespace,
			EmailLocalFnName,
			len(args),
		)
	}

	// Validate first argument.
	if args[0].Kind() != reflect.String {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with argument that does not evaluate to a string",
			EmailNamespace,
			EmailLocalFnName,
		)
	}

	return &EmailLocalExpr{email: args[0]}, nil
}

// buildRegexpExpr builds one of the following:
// - RegexpReplaceExpr
// - RegexpMatchExpr
// - RegexpNotMatchExpr
func buildRegexpExpr(name string, args []Expr) (Expr, error) {
	switch name {
	case RegexpReplaceFnName:
		return buildRegexpReplaceExpr(args)
	case RegexpMatchFnName:
		return buildRegexpMatchExpr(args)
	case RegexpNotMatchFnName:
		return buildRegexpNotMatchExpr(args)
	default:
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with name %q, expected one of: %q, %q, %q",
			RegexpNamespace,
			name,
			name,
			RegexpReplaceFnName,
			RegexpMatchFnName,
			RegexpNotMatchFnName,
		)
	}
}

// buildRegexpReplaceExpr builds a RegexpReplaceExpr.
func buildRegexpReplaceExpr(args []Expr) (Expr, error) {
	if len(args) != 3 {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with %d arguments, expected 3",
			RegexpNamespace,
			RegexpReplaceFnName,
			len(args),
		)
	}

	// Validate first argument.
	if args[0].Kind() != reflect.String {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with 1st argument that does not evaluate to a string",
			RegexpNamespace,
			RegexpReplaceFnName,
		)
	}

	// Validate second argument.
	match, ok := args[1].(*StringLitExpr)
	if !ok {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with 2nd argument that is not a string",
			RegexpNamespace,
			RegexpReplaceFnName,
		)
	}
	re, err := regexp.Compile(match.value)
	if err != nil {
		return nil, trace.BadParameter(
			"failed to parse \"%s.%s\" 2nd argument regexp %q: %v",
			RegexpNamespace,
			RegexpReplaceFnName,
			match,
			err,
		)
	}

	// Validate third argument.
	replacement, ok := args[2].(*StringLitExpr)
	if !ok {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with 3rd argument that is not a string",
			RegexpNamespace,
			RegexpReplaceFnName,
		)
	}

	return &RegexpReplaceExpr{source: args[0], re: re, replacement: replacement.value}, nil
}

// buildRegexpMatchExprFromLit builds a RegexpMatchExpr from a string literal.
func buildRegexpMatchExprFromLit(raw string) (Expr, error) {
	match, err := newRegexp(raw, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RegexpMatchExpr{re: match}, nil
}

// buildRegexpMatchExpr builds a RegexpMatchExpr.
func buildRegexpMatchExpr(args []Expr) (Expr, error) {
	match, err := buildRegexpMatchFnExpr(RegexpMatchFnName, args)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RegexpMatchExpr{re: match}, nil
}

// buildRegexpNotMatchExpr builds a RegexpNotMatchExpr.
func buildRegexpNotMatchExpr(args []Expr) (Expr, error) {
	notMatch, err := buildRegexpMatchFnExpr(RegexpNotMatchFnName, args)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RegexpNotMatchExpr{re: notMatch}, nil
}

func buildRegexpMatchFnExpr(functionName string, args []Expr) (*regexp.Regexp, error) {
	if len(args) != 1 {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with %d arguments, expected 1",
			RegexpNamespace,
			functionName,
			len(args),
		)
	}

	// Validate first argument.
	// For now, only support a single match expression. In the future, we could
	// consider handling variables and transforms by propagating user traits to
	// the matching logic. For example
	// `{{regexp.match(external.allowed_env_trait)}}`.
	arg, ok := args[0].(*StringLitExpr)
	if !ok {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with 1st argument that is not a string, no variables and transformations are allowed",
			RegexpNamespace,
			functionName,
		)
	}

	re, err := newRegexp(arg.value, false)
	if err != nil {
		return nil, trace.BadParameter(
			"found function \"%s.%s\" with 1st argument that is not a valid regexp: %s",
			RegexpNamespace,
			functionName,
			err,
		)
	}
	return re, nil
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
