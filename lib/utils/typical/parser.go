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

// Package typical (TYPed predICAte Library) is a library for building better predicate
// expression parsers faster. It is built on top of
// [predicate](github.com/gravitational/predicate).
//
// typical helps you (and forces you) to separate parse and evaluation into two
// distinct stages so that parsing can be cached and evaluation can be fast.
// Expressions can also be parsed to check their syntax without needing to
// evaluate them with some specific input.
//
// Functions can be defined by providing implementations accepting and returning
// values of ordinary types, the library handles the details that enable any
// function argument to be a subexpression that evaluates to the correct type.
//
// By default, typical provides strong type checking at parse time without any
// reflection during evaluation. If you define a function that evaluates to
// type any, you are opting in to evaluation-time type checking everywhere that
// function is called, but types will still be checked for all other parts of
// the expression. If you define a function that accepts an interface type it
// will be called via reflection during evaluation, but interface satisfaction
// is still checked at parse time.
package typical

import (
	"fmt"
	"reflect"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"
)

// Expression is a generic interface representing a parsed predicate expression
// which can be evaluated with environment type TEnv to produce a result of type
// TResult.
type Expression[TEnv, TResult any] interface {
	// Evaluate evaluates the already parsed predicate expression with the given
	// environment.
	Evaluate(environment TEnv) (TResult, error)
}

// ParserSpec defines a predicate language.
type ParserSpec[TEnv any] struct {
	// Variables define the literals and variables available to
	// expressions in the predicate language. It is a map from variable name to
	// definition.
	Variables map[string]Variable

	// Functions define the functions available to expressions in the
	// predicate language.
	Functions map[string]Function

	// Methods define the methods available to expressions in the
	// predicate language. Methods are just functions that take their receiver
	// as their first argument.
	Methods map[string]Function

	// GetUnknownIdentifier is used to retrieve any identifiers that cannot
	// be determined statically. If not defined, any unknown identifiers result
	// in a [UnknownIdentifierError].
	//
	// Useful in situations where a parser may allow specifying nested paths
	// to variables without requiring users to enumerate all paths in [ParserSpec.Variables].
	// Caution should be used when using this method as it shifts type safety
	// guarantees from parse time to evaluation time.
	// Typos in identifier names will also be caught at evaluation time instead of parse time.
	GetUnknownIdentifier func(env TEnv, fields []string) (any, error)
}

// Variable holds the definition of a literal or variable. It is expected to be
// either a literal static value, or the result of calling DynamicVariable or
// DynamicMap.
type Variable any

// Function holds the definition of a function. It is expected to be the result
// of calling one of (Unary|Binary|Ternary)(Variadic)?Function.
type Function interface {
	buildExpression(name string, args ...any) (any, error)
}

// Parser is a predicate expression parser configured to parse expressions of a
// specific expression language.
type Parser[TEnv, TResult any] struct {
	spec    ParserSpec[TEnv]
	pred    predicate.Parser
	options parserOptions
}

type parserOptions struct {
	invalidNamespaceHack bool
}

// ParserOption is an optional option for configuring a Parser.
type ParserOption func(*parserOptions)

// WithInvalidNamespaceHack is necessary because old parser versions
// accidentally allowed "<anything>.trait" as an alias for "external.trait". Some
// people wrote typos and now we have to maintain this.
// See https://github.com/gravitational/teleport/pull/21551
func WithInvalidNamespaceHack() ParserOption {
	return func(opts *parserOptions) {
		opts.invalidNamespaceHack = true
	}
}

// NewParser creates a predicate expression parser with the given specification.
func NewParser[TEnv, TResult any](spec ParserSpec[TEnv], opts ...ParserOption) (*Parser[TEnv, TResult], error) {
	var options parserOptions
	for _, opt := range opts {
		opt(&options)
	}

	p := &Parser[TEnv, TResult]{
		spec:    spec,
		options: options,
	}
	def := predicate.Def{
		GetIdentifier: p.getIdentifier,
		GetProperty:   p.getProperty,
		Functions:     make(map[string]any, len(spec.Functions)),
		Methods:       make(map[string]any, len(spec.Methods)),
		Operators: predicate.Operators{
			NOT: not[TEnv],
			AND: and[TEnv](),
			OR:  or[TEnv](),
			EQ:  eq[TEnv](),
			NEQ: neq[TEnv](),
		},
	}

	for name, f := range spec.Functions {
		name, f := name, f
		def.Functions[name] = func(args ...any) (any, error) {
			return f.buildExpression(name, args...)
		}
	}
	for name, f := range spec.Methods {
		name, f := name, f
		def.Methods[name] = func(args ...any) (any, error) {
			return f.buildExpression(name, args...)
		}
	}

	pred, err := predicate.NewParser(def)
	if err != nil {
		return nil, trace.Wrap(err, "creating predicate parser")
	}
	p.pred = pred
	return p, nil
}

// Parse parses the given expression string to produce an
// Expression[TEnv, TResult]. The returned Expression can be safely cached and
// called multiple times with different TEnv inputs.
func (p *Parser[TEnv, TResult]) Parse(expression string) (Expression[TEnv, TResult], error) {
	if expression == "" {
		return nil, trace.BadParameter("empty expression")
	}
	result, err := p.pred.Parse(expression)
	if err != nil {
		return nil, trace.Wrap(err, "parsing expression")
	}
	expr, err := coerce[TEnv, TResult](result)
	if err != nil {
		return nil, trace.Wrap(err, "expression evaluated to unexpected type")
	}
	return expr, nil
}

func (p *Parser[TEnv, TResult]) getIdentifier(selector []string) (any, error) {
	remaining := selector[:]
	var joined string
	for len(remaining) > 0 {
		if len(joined) > 0 {
			joined += "."
		}
		joined += remaining[0]
		remaining = remaining[1:]
		v, ok := p.spec.Variables[joined]
		if !ok {
			continue
		}
		if len(remaining) == 0 {
			// Exact match
			return v, nil
		}
		// Allow map lookups with map.key instead of map["key"]
		if len(remaining) == 1 {
			expr, err := p.getProperty(v, remaining[0])
			if err == nil {
				return expr, nil
			}
			// if err != nil just continue rather than returning a
			// potentially confusing index expression error when this wasn't
			// actually a map
		}
	}

	if p.options.invalidNamespaceHack && len(selector) < 3 {
		// With invalidNamespaceHack enabled, anything not matched above is an
		// alias for "external"
		external, ok := p.spec.Variables["external"]
		if !ok {
			return nil, trace.BadParameter(`invalidNamespaceHack enabled but "external" not present (this is a bug)`)
		}
		switch len(selector) {
		case 1:
			return external, nil
		case 2:
			expr, err := p.getProperty(external, selector[1])
			return expr, trace.Wrap(err)
		}
	}

	// Return a dynamic variable if and only if the parser was
	// constructed to opt in to the dangerous behavior.
	if p.spec.GetUnknownIdentifier != nil {
		return dynamicVariable[TEnv, any]{
			accessor: func(env TEnv) (any, error) {
				return p.spec.GetUnknownIdentifier(env, selector)
			},
		}, nil
	}

	return nil, UnknownIdentifierError(joined)
}

// UnknownIdentifierError is an error type that can be used to identify errors
// related to an unknown identifier in an expression.
type UnknownIdentifierError string

func (u UnknownIdentifierError) Error() string {
	return fmt.Sprintf("unknown identifier: %q", string(u))
}

func (u UnknownIdentifierError) Identifier() string {
	return string(u)
}

// getProperty is a helper for parsing map[key] expressions and returns either a
// propertyExpr or a dynamicMapExpr.
func (p *Parser[TEnv, TResult]) getProperty(mapVal, keyVal any) (any, error) {
	keyExpr, err := coerce[TEnv, string](keyVal)
	if err != nil {
		return nil, trace.Wrap(err, "parsing key of index expression")
	}

	if mapExpr, ok := mapVal.(Expression[TEnv, map[string]string]); ok {
		return propertyExpr[TEnv, string]{mapExpr, keyExpr}, nil
	}
	if mapExpr, ok := mapVal.(Expression[TEnv, map[string][]string]); ok {
		return propertyExpr[TEnv, []string]{mapExpr, keyExpr}, nil
	}
	if dynamicMap, ok := mapVal.(indexExpressionBuilder[TEnv]); ok {
		return dynamicMap.buildIndexExpression(keyExpr), nil
	}

	// Only allow falling back to an untyped expression if the parser was constructed
	// to allow unknown identifiers. This ensures compile time type safety for all
	// parsers that don't explicitly opt in to the more dangerous behavior required to
	// support dynamic fields.
	if p.spec.GetUnknownIdentifier != nil {
		if mapExpr, ok := mapVal.(Expression[TEnv, any]); ok {
			return untypedPropertyExpr[TEnv]{mapExpr, keyExpr}, nil
		}
	}

	return nil, trace.Wrap(unexpectedTypeError[map[string]string](mapVal), "cannot take index of unexpected type")
}

type propertyExpr[TEnv, TValues any] struct {
	mapExpr Expression[TEnv, map[string]TValues]
	keyExpr Expression[TEnv, string]
}

func (p propertyExpr[TEnv, TValues]) Evaluate(env TEnv) (TValues, error) {
	var nul TValues
	m, err := p.mapExpr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating base of index expression")
	}
	k, err := p.keyExpr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating key of index expression")
	}
	return m[k], nil
}

type untypedPropertyExpr[TEnv any] struct {
	mapExpr Expression[TEnv, any]
	keyExpr Expression[TEnv, string]
}

func (u untypedPropertyExpr[TEnv]) Evaluate(env TEnv) (any, error) {
	k, err := u.keyExpr.Evaluate(env)
	if err != nil {
		return nil, trace.Wrap(err, "evaluating key of index expression")
	}
	m, err := u.mapExpr.Evaluate(env)
	if err != nil {
		return nil, trace.Wrap(err, "evaluating base of index expression")
	}
	switch typedMap := m.(type) {
	case map[string]string:
		return typedMap[k], nil
	case map[string][]string:
		return typedMap[k], nil
	case map[string]any:
		return typedMap[k], nil
	default:
		return nil, trace.Wrap(unexpectedTypeError[map[string]any](u.mapExpr), "cannot take index of unexpected type")
	}
}

type indexExpressionBuilder[TEnv any] interface {
	buildIndexExpression(keyExpr Expression[TEnv, string]) any
}

// Getter is a generic interface for map-like values that allow you to Get a
// TValues by key
type Getter[TValues any] interface {
	Get(key string) (TValues, error)
}

type dynamicMap[TEnv, TValues any, TMap Getter[TValues]] struct {
	accessor func(TEnv) (TMap, error)
}

// DynamicMap returns a definition for a variable that can be indexed with
// map[key] or map.key syntax to get a TValues, or passed directly to another
// function as TMap. TMap must implement Getter[TValues]. Each time the
// variable is reference in an expression, [accessor] will be called to retrieve
// the Getter.
func DynamicMap[TEnv any, TValues any, TMap Getter[TValues]](accessor func(TEnv) (TMap, error)) Variable {
	return dynamicMap[TEnv, TValues, TMap]{accessor}
}

func (d dynamicMap[TEnv, TValues, TMap]) Evaluate(env TEnv) (TMap, error) {
	m, err := d.accessor(env)
	return m, trace.Wrap(err)
}

//nolint:unused // https://github.com/dominikh/go-tools/issues/1294
func (d dynamicMap[TEnv, TValues, TMap]) buildIndexExpression(keyExpr Expression[TEnv, string]) any {
	return dynamicVariable[TEnv, TValues]{func(env TEnv) (TValues, error) {
		var nul TValues
		key, err := keyExpr.Evaluate(env)
		if err != nil {
			return nul, trace.Wrap(err, "evaluating key of index expression")
		}
		m, err := d.accessor(env)
		if err != nil {
			return nul, trace.Wrap(err, "evaluating base of index expression")
		}
		v, err := m.Get(key)
		return v, trace.Wrap(err, "getting value from dynamic map")
	}}
}

type funcGetter[TValues any] func(key string) (TValues, error)

func (f funcGetter[TValues]) Get(key string) (TValues, error) {
	return f(key)
}

// DynamicMapFunction returns a definition for a variable that can be indexed
// with map[key] or map.key syntax to get a TValues. Each time the
// variable is indexed in an expression, [getFunc] will be called to retrieve
// the value.
func DynamicMapFunction[TEnv, TValues any](getFunc func(env TEnv, key string) (TValues, error)) Variable {
	return DynamicMap[TEnv, TValues](func(env TEnv) (funcGetter[TValues], error) {
		return funcGetter[TValues](func(key string) (TValues, error) {
			return getFunc(env, key)
		}), nil
	})
}

type dynamicVariable[TEnv, TVar any] struct {
	accessor func(TEnv) (TVar, error)
}

// DynamicVariable returns a normal variable definition. Whenever the variable is
// accessed during evaluation of an expression, accessor will be called to fetch
// the value of the variable from the current evaluation environment.
func DynamicVariable[TEnv, TVar any](accessor func(TEnv) (TVar, error)) Variable {
	return dynamicVariable[TEnv, TVar]{accessor}
}

func (d dynamicVariable[TEnv, TVar]) Evaluate(env TEnv) (TVar, error) {
	result, err := d.accessor(env)
	return result, trace.Wrap(err)
}

type unaryFunction[TEnv, TArg, TResult any] struct {
	impl func(TArg) (TResult, error)
}

// UnaryFunction returns a definition for a function that can be called with one
// argument. The argument may be a literal or a subexpression.
func UnaryFunction[TEnv, TArg, TResult any](impl func(TArg) (TResult, error)) Function {
	return unaryFunction[TEnv, TArg, TResult]{impl}
}

func (f unaryFunction[TEnv, TArg, TResult]) buildExpression(name string, args ...any) (any, error) {
	if len(args) != 1 {
		return nil, trace.BadParameter("function (%s) accepts 1 argument, given %d", name, len(args))
	}
	argExpr, err := coerce[TEnv, TArg](args[0])
	if err != nil {
		return nil, trace.Wrap(err, "parsing argument to (%s)", name)
	}
	return unaryFuncExpr[TEnv, TArg, TResult]{
		name:    name,
		impl:    f.impl,
		argExpr: argExpr,
	}, nil
}

type unaryFuncExpr[TEnv, TArg, TResult any] struct {
	name    string
	impl    func(TArg) (TResult, error)
	argExpr Expression[TEnv, TArg]
}

func (e unaryFuncExpr[TEnv, TArg, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	arg, err := e.argExpr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating argument to function (%s)", e.name)
	}
	res, err := e.impl(arg)
	return res, trace.Wrap(err, "evaluating function (%s)", e.name)
}

type unaryFunctionWithEnv[TEnv, TArg, TResult any] struct {
	impl func(TEnv, TArg) (TResult, error)
}

// UnaryFunctionWithEnv returns a definition for a function that can be called with one
// argument. The argument may be a literal or a subexpression. The [impl] will
// be called with the evaluation env as the first argument, followed by the
// actual argument passed in the expression.
func UnaryFunctionWithEnv[TEnv, TArg, TResult any](impl func(TEnv, TArg) (TResult, error)) Function {
	return unaryFunctionWithEnv[TEnv, TArg, TResult]{impl}
}

func (f unaryFunctionWithEnv[TEnv, TArg, TResult]) buildExpression(name string, args ...any) (any, error) {
	if len(args) != 1 {
		return nil, trace.BadParameter("function (%s) accepts 1 argument, given %d", name, len(args))
	}
	argExpr, err := coerce[TEnv, TArg](args[0])
	if err != nil {
		return nil, trace.Wrap(err, "parsing argument to (%s)", name)
	}
	return unaryFuncWithEnvExpr[TEnv, TArg, TResult]{
		name:    name,
		impl:    f.impl,
		argExpr: argExpr,
	}, nil
}

type unaryFuncWithEnvExpr[TEnv, TArg, TResult any] struct {
	name    string
	impl    func(TEnv, TArg) (TResult, error)
	argExpr Expression[TEnv, TArg]
}

func (e unaryFuncWithEnvExpr[TEnv, TArg, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	arg, err := e.argExpr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating argument to function (%s)", e.name)
	}
	res, err := e.impl(env, arg)
	return res, trace.Wrap(err, "evaluating function (%s)", e.name)
}

type binaryFunction[TEnv, TArg1, TArg2, TResult any] struct {
	impl func(TArg1, TArg2) (TResult, error)
}

// BinaryFunction returns a definition for a function that can be called with
// two arguments. The arguments may be literals or subexpressions.
func BinaryFunction[TEnv, TArg1, TArg2, TResult any](impl func(TArg1, TArg2) (TResult, error)) Function {
	return binaryFunction[TEnv, TArg1, TArg2, TResult]{impl}
}

func (f binaryFunction[TEnv, TArg1, TArg2, TResult]) buildExpression(name string, args ...any) (any, error) {
	if len(args) != 2 {
		return nil, trace.BadParameter("function (%s) accepts 2 arguments, given %d", name, len(args))
	}
	arg1Expr, err := coerce[TEnv, TArg1](args[0])
	if err != nil {
		return nil, trace.Wrap(err, "parsing first argument to (%s)", name)
	}
	arg2Expr, err := coerce[TEnv, TArg2](args[1])
	if err != nil {
		return nil, trace.Wrap(err, "parsing second argument to (%s)", name)
	}
	return binaryFuncExpr[TEnv, TArg1, TArg2, TResult]{
		name:     name,
		impl:     f.impl,
		arg1Expr: arg1Expr,
		arg2Expr: arg2Expr,
	}, nil
}

type binaryFuncExpr[TEnv, TArg1, TArg2, TResult any] struct {
	name     string
	impl     func(TArg1, TArg2) (TResult, error)
	arg1Expr Expression[TEnv, TArg1]
	arg2Expr Expression[TEnv, TArg2]
}

func (e binaryFuncExpr[TEnv, TArg1, TArg2, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	arg1, err := e.arg1Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating first argument to function (%s)", e.name)
	}
	arg2, err := e.arg2Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating second argument to function (%s)", e.name)
	}
	res, err := e.impl(arg1, arg2)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating function (%s)", e.name)
	}
	return res, nil
}

type ternaryFunction[TEnv, TArg1, TArg2, TArg3, TResult any] struct {
	impl func(TArg1, TArg2, TArg3) (TResult, error)
}

// TernaryFunction returns a definition for a function that can be called with
// three arguments. The arguments may be literals or subexpressions.
func TernaryFunction[TEnv, TArg1, TArg2, TArg3, TResult any](impl func(TArg1, TArg2, TArg3) (TResult, error)) Function {
	return ternaryFunction[TEnv, TArg1, TArg2, TArg3, TResult]{impl}
}

func (f ternaryFunction[TEnv, TArg1, TArg2, TArg3, TResult]) buildExpression(name string, args ...any) (any, error) {
	if len(args) != 3 {
		return nil, trace.BadParameter("function (%s) accepts 3 arguments, given %d", name, len(args))
	}
	arg1Expr, err := coerce[TEnv, TArg1](args[0])
	if err != nil {
		return nil, trace.Wrap(err, "parsing first argument to (%s)", name)
	}
	arg2Expr, err := coerce[TEnv, TArg2](args[1])
	if err != nil {
		return nil, trace.Wrap(err, "parsing second argument to (%s)", name)
	}
	arg3Expr, err := coerce[TEnv, TArg3](args[2])
	if err != nil {
		return nil, trace.Wrap(err, "parsing third argument to (%s)", name)
	}
	return ternaryFuncExpr[TEnv, TArg1, TArg2, TArg3, TResult]{
		name:     name,
		impl:     f.impl,
		arg1Expr: arg1Expr,
		arg2Expr: arg2Expr,
		arg3Expr: arg3Expr,
	}, nil
}

type ternaryFuncExpr[TEnv, TArg1, TArg2, TArg3, TResult any] struct {
	name     string
	impl     func(TArg1, TArg2, TArg3) (TResult, error)
	arg1Expr Expression[TEnv, TArg1]
	arg2Expr Expression[TEnv, TArg2]
	arg3Expr Expression[TEnv, TArg3]
}

func (e ternaryFuncExpr[TEnv, TArg1, TArg2, TArg3, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	arg1, err := e.arg1Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating first argument to function (%s)", e.name)
	}
	arg2, err := e.arg2Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating second argument to function (%s)", e.name)
	}
	arg3, err := e.arg3Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating third argument to function (%s)", e.name)
	}
	res, err := e.impl(arg1, arg2, arg3)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating function (%s)", e.name)
	}
	return res, nil
}

type unaryVariadicFunction[TEnv, TVarArgs, TResult any] struct {
	impl func(...TVarArgs) (TResult, error)
}

// UnaryVariadicFunction returns a definition for a function that can be called
// with any number of arguments of a single type. The arguments may be literals
// or subexpressions.
func UnaryVariadicFunction[TEnv, TVarArgs, TResult any](impl func(...TVarArgs) (TResult, error)) Function {
	return unaryVariadicFunction[TEnv, TVarArgs, TResult]{impl}
}

func (f unaryVariadicFunction[TEnv, TVarArgs, TResult]) buildExpression(name string, args ...any) (any, error) {
	varArgExprs := make([]Expression[TEnv, TVarArgs], len(args))
	for i, arg := range args {
		argExpr, err := coerce[TEnv, TVarArgs](arg)
		if err != nil {
			return nil, trace.Wrap(err, "parsing argument %d to function (%s)", i+1, name)
		}
		varArgExprs[i] = argExpr
	}
	return unaryVariadicFuncExpr[TEnv, TVarArgs, TResult]{
		name:        name,
		impl:        f.impl,
		varArgExprs: varArgExprs,
	}, nil
}

type unaryVariadicFuncExpr[TEnv, TVarArgs, TResult any] struct {
	name        string
	impl        func(...TVarArgs) (TResult, error)
	varArgExprs []Expression[TEnv, TVarArgs]
}

func (e unaryVariadicFuncExpr[TEnv, TVarArgs, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	varArgs := make([]TVarArgs, len(e.varArgExprs))
	for i, argExpr := range e.varArgExprs {
		arg, err := argExpr.Evaluate(env)
		if err != nil {
			return nul, trace.Wrap(err, "evaluating argument %d to function (%s)", i+1, e.name)
		}
		varArgs[i] = arg
	}
	res, err := e.impl(varArgs...)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating function (%s)", e.name)
	}
	return res, nil
}

type unaryVariadicFunctionWithEnv[TEnv, TVarArgs, TResult any] struct {
	impl func(TEnv, ...TVarArgs) (TResult, error)
}

// UnaryVariadicFunctionWithEnv returns a definition for a function that can be called
// with any number of arguments with a single type. The [impl] will
// be called with the evaluation env as the first argument, followed by the
// actual arguments passed in the expression.
func UnaryVariadicFunctionWithEnv[TEnv, TVarArgs, TResult any](impl func(TEnv, ...TVarArgs) (TResult, error)) Function {
	return unaryVariadicFunctionWithEnv[TEnv, TVarArgs, TResult]{impl}
}

func (f unaryVariadicFunctionWithEnv[TEnv, TVarArgs, TResult]) buildExpression(name string, args ...any) (any, error) {
	varArgExprs := make([]Expression[TEnv, TVarArgs], len(args))
	for i, arg := range args {
		argExpr, err := coerce[TEnv, TVarArgs](arg)
		if err != nil {
			return nil, trace.Wrap(err, "parsing argument %d to function (%s)", i+1, name)
		}
		varArgExprs[i] = argExpr
	}
	return unaryVariadicFuncWithEnvExpr[TEnv, TVarArgs, TResult]{
		name:        name,
		impl:        f.impl,
		varArgExprs: varArgExprs,
	}, nil
}

type unaryVariadicFuncWithEnvExpr[TEnv, TVarArgs, TResult any] struct {
	name        string
	impl        func(TEnv, ...TVarArgs) (TResult, error)
	varArgExprs []Expression[TEnv, TVarArgs]
}

func (e unaryVariadicFuncWithEnvExpr[TEnv, TVarArgs, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	varArgs := make([]TVarArgs, len(e.varArgExprs))
	for i, argExpr := range e.varArgExprs {
		arg, err := argExpr.Evaluate(env)
		if err != nil {
			return nul, trace.Wrap(err, "evaluating argument %d to function (%s)", i+1, e.name)
		}
		varArgs[i] = arg
	}
	res, err := e.impl(env, varArgs...)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating function (%s)", e.name)
	}
	return res, nil
}

type binaryVariadicFunction[TEnv, TArg1, TVarArgs, TResult any] struct {
	impl func(TArg1, ...TVarArgs) (TResult, error)
}

// BinaryVariadicFunction returns a definition for a function that can be called
// with one or more arguments. The arguments may be literals or subexpressions.
func BinaryVariadicFunction[TEnv, TArg1, TVarArgs, TResult any](impl func(TArg1, ...TVarArgs) (TResult, error)) Function {
	return binaryVariadicFunction[TEnv, TArg1, TVarArgs, TResult]{impl}
}

func (f binaryVariadicFunction[TEnv, TArg1, TVarArgs, TResult]) buildExpression(name string, args ...any) (any, error) {
	if len(args) == 0 {
		return nil, trace.BadParameter("function (%s) accepts 1 or more argument, given %d", name, len(args))
	}
	arg1Expr, err := coerce[TEnv, TArg1](args[0])
	if err != nil {
		return nil, trace.Wrap(err, "parsing first argument to function (%s)", name)
	}
	args = args[1:]
	varArgExprs := make([]Expression[TEnv, TVarArgs], len(args))
	for i, arg := range args {
		argExpr, err := coerce[TEnv, TVarArgs](arg)
		if err != nil {
			return nil, trace.Wrap(err, "parsing argument %d to function (%s)", i+2, name)
		}
		varArgExprs[i] = argExpr
	}
	return binaryVariadicFuncExpr[TEnv, TArg1, TVarArgs, TResult]{
		name:        name,
		impl:        f.impl,
		arg1Expr:    arg1Expr,
		varArgExprs: varArgExprs,
	}, nil
}

type binaryVariadicFuncExpr[TEnv, TArg1, TVarArgs, TResult any] struct {
	name        string
	impl        func(TArg1, ...TVarArgs) (TResult, error)
	arg1Expr    Expression[TEnv, TArg1]
	varArgExprs []Expression[TEnv, TVarArgs]
}

func (e binaryVariadicFuncExpr[TEnv, TArg1, TVarArgs, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	arg1, err := e.arg1Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating first argument to function (%s)", e.name)
	}
	varArgs := make([]TVarArgs, len(e.varArgExprs))
	for i, argExpr := range e.varArgExprs {
		arg, err := argExpr.Evaluate(env)
		if err != nil {
			return nul, trace.Wrap(err, "evaluating argument %d to function (%s)", i+2, e.name)
		}
		varArgs[i] = arg
	}
	res, err := e.impl(arg1, varArgs...)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating function (%s)", e.name)
	}
	return res, nil
}

type ternaryVariadicFunction[TEnv, TArg1, TArg2, TVarArgs, TResult any] struct {
	impl func(TArg1, TArg2, ...TVarArgs) (TResult, error)
}

// TernaryVariadicFunction returns a definition for a function that can be called
// with one or more arguments. The arguments may be literals or subexpressions.
func TernaryVariadicFunction[TEnv, TArg1, Targ2, TVarArgs, TResult any](impl func(TArg1, Targ2, ...TVarArgs) (TResult, error)) Function {
	return ternaryVariadicFunction[TEnv, TArg1, Targ2, TVarArgs, TResult]{impl}
}

func (f ternaryVariadicFunction[TEnv, TArg1, TArg2, TVarArgs, TResult]) buildExpression(name string, args ...any) (any, error) {
	if len(args) < 2 {
		return nil, trace.BadParameter("function (%s) accepts 2 or more arguments, given %d", name, len(args))
	}
	arg1Expr, err := coerce[TEnv, TArg1](args[0])
	if err != nil {
		return nil, trace.Wrap(err, "parsing first argument to function (%s)", name)
	}
	arg2Expr, err := coerce[TEnv, TArg2](args[1])
	if err != nil {
		return nil, trace.Wrap(err, "parsing second argument to function (%s)", name)
	}
	args = args[2:]
	varArgExprs := make([]Expression[TEnv, TVarArgs], len(args))
	for i, arg := range args {
		argExpr, err := coerce[TEnv, TVarArgs](arg)
		if err != nil {
			return nil, trace.Wrap(err, "parsing argument %d to function (%s)", i+3, name)
		}
		varArgExprs[i] = argExpr
	}
	return ternaryVariadicFuncExpr[TEnv, TArg1, TArg2, TVarArgs, TResult]{
		name:        name,
		impl:        f.impl,
		arg1Expr:    arg1Expr,
		arg2Expr:    arg2Expr,
		varArgExprs: varArgExprs,
	}, nil
}

type ternaryVariadicFuncExpr[TEnv, TArg1, TArg2, TVarArgs, TResult any] struct {
	name        string
	impl        func(TArg1, TArg2, ...TVarArgs) (TResult, error)
	arg1Expr    Expression[TEnv, TArg1]
	arg2Expr    Expression[TEnv, TArg2]
	varArgExprs []Expression[TEnv, TVarArgs]
}

func (e ternaryVariadicFuncExpr[TEnv, TArg1, TArg2, TVarArgs, TResult]) Evaluate(env TEnv) (TResult, error) {
	var nul TResult
	arg1, err := e.arg1Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating first argument to function (%s)", e.name)
	}
	arg2, err := e.arg2Expr.Evaluate(env)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating second argument to function (%s)", e.name)
	}
	varArgs := make([]TVarArgs, len(e.varArgExprs))
	for i, argExpr := range e.varArgExprs {
		arg, err := argExpr.Evaluate(env)
		if err != nil {
			return nul, trace.Wrap(err, "evaluating argument %d to function (%s)", i+3, e.name)
		}
		varArgs[i] = arg
	}
	res, err := e.impl(arg1, arg2, varArgs...)
	if err != nil {
		return nul, trace.Wrap(err, "evaluating function (%s)", e.name)
	}
	return res, nil
}

type booleanOperator[TEnv, TArgs any] struct {
	name string
	f    func(env TEnv, a, b Expression[TEnv, TArgs]) (bool, error)
}

func (b booleanOperator[TEnv, TArgs]) buildExpression(lhs, rhs any) (Expression[TEnv, bool], error) {
	lhsExpr, err := coerce[TEnv, TArgs](lhs)
	if err != nil {
		return nil, trace.Wrap(err, "parsing lhs of (%s) operator", b.name)
	}
	rhsExpr, err := coerce[TEnv, TArgs](rhs)
	if err != nil {
		return nil, trace.Wrap(err, "parsing rhs of (%s) operator", b.name)
	}
	return booleanOperatorExpr[TEnv, TArgs]{b.name, b.f, lhsExpr, rhsExpr}, nil
}

type booleanOperatorExpr[TEnv, TArgs any] struct {
	name             string
	f                func(env TEnv, a, b Expression[TEnv, TArgs]) (bool, error)
	lhsExpr, rhsExpr Expression[TEnv, TArgs]
}

func (b booleanOperatorExpr[TEnv, TArgs]) Evaluate(env TEnv) (bool, error) {
	return b.f(env, b.lhsExpr, b.rhsExpr)
}

func and[TEnv any]() func(lhs, rhs any) (Expression[TEnv, bool], error) {
	return booleanOperator[TEnv, bool]{
		name: "&&",
		f: func(env TEnv, lhsExpr, rhsExpr Expression[TEnv, bool]) (bool, error) {
			lhs, err := lhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating lhs of (&&) operator")
			}
			// Short-circuit if possible.
			if !lhs {
				return false, nil
			}
			rhs, err := rhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating rhs of (&&) operator")
			}
			return rhs, nil
		},
	}.buildExpression
}

func or[TEnv any]() func(lhs, rhs any) (Expression[TEnv, bool], error) {
	return booleanOperator[TEnv, bool]{
		name: "||",
		f: func(env TEnv, lhsExpr, rhsExpr Expression[TEnv, bool]) (bool, error) {
			lhs, err := lhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating lhs of (||) operator")
			}
			// Short-circuit if possible.
			if lhs {
				return true, nil
			}
			rhs, err := rhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating rhs of (||) operator")
			}
			return rhs, nil
		},
	}.buildExpression
}

func eq[TEnv any]() func(lhs, rhs any) (Expression[TEnv, bool], error) {
	return func(lhs any, rhs any) (Expression[TEnv, bool], error) {
		// If the LHS operand type isn't known at compile time (e.g. an `ifelse`
		// function call) try the RHS instead.
		operand := lhs
		if _, isAny := operand.(Expression[TEnv, any]); isAny {
			operand = rhs
		}
		switch operand.(type) {
		case string, Expression[TEnv, string]:
			return eqExpression[TEnv, string](lhs, rhs)
		case int, Expression[TEnv, int]:
			return eqExpression[TEnv, int](lhs, rhs)
		case Expression[TEnv, any]:
			return nil, trace.Errorf("operator (==) can only be used when at least one operand type is known at compile-time")
		default:
			return nil, trace.Errorf("operator (==) not supported for type: %s", typeName(operand))
		}
	}
}

func eqExpression[TEnv any, TResult comparable](lhs any, rhs any) (Expression[TEnv, bool], error) {
	return booleanOperator[TEnv, TResult]{
		name: "==",
		f: func(env TEnv, lhsExpr, rhsExpr Expression[TEnv, TResult]) (bool, error) {
			lhs, err := lhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating lhs of (==) operator")
			}
			rhs, err := rhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating rhs of (==) operator")
			}
			return lhs == rhs, nil
		},
	}.buildExpression(lhs, rhs)
}

func neq[TEnv any]() func(lhs, rhs any) (Expression[TEnv, bool], error) {
	return func(lhs any, rhs any) (Expression[TEnv, bool], error) {
		// If the LHS operand type isn't known at compile time (e.g. an `ifelse`
		// function call) try the RHS instead.
		operand := lhs
		if _, isAny := operand.(Expression[TEnv, any]); isAny {
			operand = rhs
		}
		switch operand.(type) {
		case string, Expression[TEnv, string]:
			return neqExpression[TEnv, string](lhs, rhs)
		case int, Expression[TEnv, int]:
			return neqExpression[TEnv, int](lhs, rhs)
		case Expression[TEnv, any]:
			return nil, trace.Errorf("operator (!=) can only be used when at least one operand type is known at compile-time")
		default:
			return nil, trace.Errorf("operator (!=) not supported for type: %s", typeName(operand))
		}
	}
}

func neqExpression[TEnv any, TResult comparable](lhs any, rhs any) (Expression[TEnv, bool], error) {
	return booleanOperator[TEnv, TResult]{
		name: "!=",
		f: func(env TEnv, lhsExpr, rhsExpr Expression[TEnv, TResult]) (bool, error) {
			lhs, err := lhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating lhs of (!=) operator")
			}
			rhs, err := rhsExpr.Evaluate(env)
			if err != nil {
				return false, trace.Wrap(err, "evaluating rhs of (!=) operator")
			}
			return lhs != rhs, nil
		},
	}.buildExpression(lhs, rhs)
}

type notExpr[TEnv any] struct {
	tExpr Expression[TEnv, bool]
}

func (e notExpr[TEnv]) Evaluate(env TEnv) (bool, error) {
	t, err := e.tExpr.Evaluate(env)
	if err != nil {
		return false, trace.Wrap(err, "evaluating target of (!) operator")
	}
	return !t, nil
}

func not[TEnv any](a any) (Expression[TEnv, bool], error) {
	tExpr, err := coerce[TEnv, bool](a)
	if err != nil {
		return nil, trace.Wrap(err, "parsing target of (!) operator")
	}
	return notExpr[TEnv]{tExpr}, nil
}

type LiteralExpr[TEnv, T any] struct {
	Value T
}

func (l LiteralExpr[TEnv, T]) Evaluate(TEnv) (T, error) {
	return l.Value, nil
}

// coerce is called at parse time to attempt to convert arg to an
// Expression[TEnv, TArg]. In most cases (for valid expressions) we can convert
// all arguments to known types at parse time so that reflection is not
// necessary at evaluation time.
func coerce[TEnv, TArg any](arg any) (Expression[TEnv, TArg], error) {
	if typedArgExpr, ok := arg.(Expression[TEnv, TArg]); ok {
		// This is already an expression returning exactly the expected type, it
		// can be returned unmodified.
		return typedArgExpr, nil
	}

	// any(*new(TArg)) is a trick to create an interface wrapping an instance of
	// the generic type TArg so that we can do a type assertion on TArg.
	if _, ok := any(*new(TArg)).([]string); ok {
		// If we are expecting a []string and given a string, wrap it in a slice.
		// This happens at parse time without reflection during evaluation.
		// It's probably possible to do this for any slice type with heavy use
		// of reflect, but for now []string is sufficient.
		//
		// This enables functions like strings.upper(str) to accept lists or
		// single strings, which is common in existing predicate expressions.
		var sliceExpr Expression[TEnv, []string]
		switch typedArg := arg.(type) {
		case string:
			sliceExpr = LiteralExpr[TEnv, []string]{[]string{typedArg}}
		case []string:
			// This case will be necessary if we caught a slice literal.
			sliceExpr = LiteralExpr[TEnv, []string]{typedArg}
		case Expression[TEnv, string]:
			sliceExpr = dynamicVariable[TEnv, []string]{
				func(env TEnv) ([]string, error) {
					str, err := typedArg.Evaluate(env)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					return []string{str}, nil
				},
			}
		default:
			return nil, unexpectedTypeError[TArg](arg)
		}
		// We know TArg is []string so this assertion is safe.
		return sliceExpr.(Expression[TEnv, TArg]), nil
	}

	if anyExpr, ok := arg.(Expression[TEnv, any]); ok {
		// This is an expression evaluating to (any). We cannot check
		// the type until evaluation time. The programmer must opt into this
		// behavior by configuring a function that returns (any).
		//
		// This enables the result of functions like
		// "ifelse(condition, optionA, optionB)" (which can return any type) to
		// be used as arguments to other functions expecting specific types.
		return dynamicVariable[TEnv, TArg]{
			func(env TEnv) (TArg, error) {
				var nul TArg
				a, err := anyExpr.Evaluate(env)
				if err != nil {
					return nul, trace.Wrap(err)
				}
				if typedArg, ok := a.(TArg); ok {
					return typedArg, nil
				}
				return nul, unexpectedTypeError[TArg](a)
			},
		}, nil
	}

	if evaluateMethod, ok := reflect.TypeOf(arg).MethodByName("Evaluate"); ok {
		// Sanity check the Evaluate method actually returns two results,
		// the second of which must be an error
		if evaluateMethod.Type.NumOut() != 2 ||
			reflect.PointerTo(evaluateMethod.Type.Out(1)) != reflect.TypeOf((*error)(nil)) {
			return nil, unexpectedTypeError[TArg](arg)
		}

		// This argument is an Expr with a result type other than TArg or any.
		// If TArg is an interface and the result type implements TArg, we can
		// make it work. TArg is likely to be (any).
		//
		// This enables functions like "ifelse(condition, optionA, optionB)" to
		// accept any type as an argument, and methods like
		// 'traits["groups"].remove("admins")' to work on any receiver
		// implementing a "remover" interface.
		//
		// Since the result of arg has an unknown type, we can not assert it to
		// a specific Expr[TEnv, T], so we must check the interface
		// implementation and call the Evaluate method via reflection.

		// Make sure that the result type implements TArg at parse time, and
		// return a parse error if not.
		expectedType := reflect.TypeOf(new(TArg)).Elem()
		resultType := evaluateMethod.Type.Out(0)
		if expectedType.Kind() != reflect.Interface || !resultType.Implements(expectedType) {
			// The result type does not implement TArg
			return nil, unexpectedTypeError[TArg](arg)
		}

		return dynamicVariable[TEnv, TArg]{
			func(env TEnv) (TArg, error) {
				// The first argument to the method is the receiver.
				resultValues := evaluateMethod.Func.Call([]reflect.Value{
					reflect.ValueOf(arg), reflect.ValueOf(env),
				})
				// We can safely assert the results to TArg and error below
				// because the method return types were checked above.
				err := resultValues[1].Interface()
				if err != nil {
					var nul TArg
					return nul, trace.Wrap(err.(error))
				}
				result := resultValues[0].Interface().(TArg)
				return result, nil
			},
		}, nil
	}

	if typedArg, ok := arg.(TArg); ok {
		// This argument is a literal matching TArg. This must be checked last
		// in case TArg is (any).
		return LiteralExpr[TEnv, TArg]{typedArg}, nil
	}

	return nil, unexpectedTypeError[TArg](arg)
}

func unexpectedTypeError[TExpected any](v any) error {
	prefix := fmt.Sprintf("expected type %s, ", reflect.TypeOf((*TExpected)(nil)).Elem())
	evaluateMethod, ok := reflect.TypeOf(v).MethodByName("Evaluate")
	if !ok {
		// This isn't an expr
		return trace.BadParameter(prefix+"got value (%+v) with type (%T)", v, v)
	}
	resultType := evaluateMethod.Type.Out(0)
	return trace.BadParameter(prefix+"got expression returning type (%s)", resultType)
}

func typeName(v any) string {
	evaluateMethod, ok := reflect.TypeOf(v).MethodByName("Evaluate")
	if !ok {
		// This isn't an expr
		return fmt.Sprintf("%T", v)
	}
	return evaluateMethod.Type.Out(0).String()
}
