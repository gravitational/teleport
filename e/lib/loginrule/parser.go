package loginrule

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"
)

// evaluationEnv holds the "environment" including all identifiers which will be
// available to predicate expressions.
type evaluationEnv struct {
	// external holds the input traits which are referred to by the name
	// "external", in keeping with the syntax from role templates. For the
	// lowest priority login rule these will be the external traits coming from
	// the identity provider, for subsequent login rules this should be set to
	// the output traits of the previous login rule.
	external dict
}

type expr func(*evaluationEnv) (any, error)

// parseExpr takes a login rule expression as a string and returns an [expr]
// which can be evaluated with an [evaluationEnv] to get the final result.
func parseExpr(input string) (expr, error) {
	parser, err := predicate.NewParser(predicate.Def{
		GetIdentifier: getIdentifier,
		GetProperty:   getProperty,
		Operators: predicate.Operators{
			AND: buildAndExpr,
			OR:  buildOrExpr,
			NOT: buildNotExpr,
		},
		Functions: map[string]any{
			"set":                buildNewSetExpr,
			"dict":               buildNewDictExpr,
			"pair":               buildNewPairExpr,
			"union":              buildUnionExpr,
			"ifelse":             buildIfElseExpr,
			"strings.upper":      buildUpperExpr,
			"strings.lower":      buildLowerExpr,
			"strings.replaceall": buildReplaceAllExpr,
			"choose":             buildChooseExpr,
			"option":             buildOptionExpr,
		},
		Methods: map[string]any{
			"add":        buildSetAddExpr,
			"contains":   buildSetContainsExpr,
			"put":        buildDictPutExpr,
			"add_values": buildDictAddValuesExpr,
			"remove":     buildRemoveMethodExpr,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := parser.Parse(input)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var rootExpr expr
	switch e := result.(type) {
	case expr:
		rootExpr = e
	default:
		// It's possible that the entire expression evaluated to a string, which
		// is a valid expression within a traits_map.
		rootExpr = buildLiteralExpr(e)
	}

	return rootExpr, nil
}

type unknownIdentifier string

func getIdentifier(fields []string) (any, error) {
	switch len(fields) {
	case 1:
		switch fields[0] {
		case "true":
			return buildLiteralExpr(true), nil
		case "false":
			return buildLiteralExpr(false), nil
		case "external":
			return expr(func(env *evaluationEnv) (any, error) { return env.external, nil }), nil
		default:
			return expr(func(*evaluationEnv) (any, error) { return unknownIdentifier(fields[0]), nil }), nil
		}
	case 2:
		if fields[0] != "external" {
			return nil, trace.BadParameter("failed to parse %q, invalid namespace %q",
				strings.Join(fields, "."), fields[0])
		}
		return expr(func(env *evaluationEnv) (any, error) { return env.external[fields[1]], nil }), nil
	default:
		return nil, trace.BadParameter("failed to parse %q, found %d fields, max is 2",
			strings.Join(fields, "."), len(fields))
	}
}

func getProperty(base, key any) (any, error) {
	baseExpr, err := validateExpr[dict](base)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse base of index expression")
	}

	keyExpr, err := validateExpr[string](key)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse key of index expresion")
	}

	return expr(func(env *evaluationEnv) (any, error) {
		d, err := validateExprResult[dict](env, baseExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate base of index expression")
		}
		k, err := validateExprResult[string](env, keyExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate key of index expression")
		}
		return d[k], nil
	}), nil
}

func buildLiteralExpr(v any) expr {
	return func(*evaluationEnv) (any, error) {
		return v, nil
	}
}

func buildIfElseExpr(cond, ifTrue, ifFalse any) (expr, error) {
	condExpr, err := validateExpr[bool](cond)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse ifelse condition")
	}
	exprIfTrue, ok := ifTrue.(expr)
	if !ok {
		exprIfTrue = buildLiteralExpr(ifTrue)
	}
	exprIfFalse, ok := ifFalse.(expr)
	if !ok {
		exprIfFalse = buildLiteralExpr(ifFalse)
	}
	return func(env *evaluationEnv) (any, error) {
		cond, err := validateExprResult[bool](env, condExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate ifelse condition")
		}
		if cond {
			return exprIfTrue(env)
		}
		return exprIfFalse(env)
	}, nil
}

// buildStringTransformExprBuilder accepts a string transform function [f] and a
// function name used for error messages.
//
// It returns a function which can build expressions that transform their input
// (which may be a string or a set of strings) with [f].
//
// The return type of the expression will match the input type:
//   - if the input is a string, it returns a string
//   - if the input is a set, it returns a set
func buildStringTransformExprBuilder(f func(string) string, name string) func(input any) (expr, error) {
	return func(input any) (expr, error) {
		inputExpr, err := validateStringOrSetExpr(input)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse first argument (input) to %s", name)
		}
		return func(env *evaluationEnv) (any, error) {
			inputAny, err := inputExpr(env)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			switch input := inputAny.(type) {
			case string:
				return f(input), nil
			case set:
				return input.transform(f), nil
			default:
				return nil, trace.BadParameter("failed to evaluate argument to %s: expected string or set, got value of type %T", name, input)
			}
		}, nil
	}
}

var (
	buildUpperExpr = buildStringTransformExprBuilder(strings.ToUpper, "strings.upper")
	buildLowerExpr = buildStringTransformExprBuilder(strings.ToLower, "strings.lower")
)

func buildReplaceAllExpr(input, match, replacement any) (expr, error) {
	inputExpr, err := validateStringOrSetExpr(input)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse first argument (input) to strings.replaceall")
	}
	matchExpr, err := validateExpr[string](match)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse second argument (match) to strings.replaceall")
	}
	replacementExpr, err := validateExpr[string](replacement)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse third argument (replacement) to strings.replaceall")
	}
	return func(env *evaluationEnv) (any, error) {
		match, err := validateExprResult[string](env, matchExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate second argument (match) to strings.replaceall")
		}
		replacement, err := validateExprResult[string](env, replacementExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate third argument (replacement) to strings.replaceall")
		}

		inputAny, err := inputExpr(env)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		switch input := inputAny.(type) {
		case string:
			return strings.ReplaceAll(input, match, replacement), nil
		case set:
			return input.transform(func(s string) string {
				return strings.ReplaceAll(s, match, replacement)
			}), nil
		default:
			return nil, trace.BadParameter("failed to evaluate first argument (input) to strings.replaceall: expected string or set, got value of type %T", input)
		}
	}, nil
}

func buildChooseExpr(options ...any) (expr, error) {
	optionExprs, err := validateExprs[option](options...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse argument to choose")
	}
	return func(env *evaluationEnv) (any, error) {
		options, err := validateExprResults[option](env, optionExprs...)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate argument to choose")
		}
		return choose(options...)
	}, nil
}

func choose(options ...option) (any, error) {
	for _, opt := range options {
		if opt.condition {
			return opt.value, nil
		}
	}
	return nil, trace.BadParameter(`evaluating choose expression: no option could be selected, consider adding a default option by hardcoding the condition to "true"`)
}

type option struct {
	condition bool
	value     any
}

func buildOptionExpr(cond, value any) (expr, error) {
	condExpr, err := validateExpr[bool](cond)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse first argument (cond) to option constructor")
	}
	valueExpr, err := validateExpr[any](value)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse second argument (value) to option constructor")
	}
	return func(env *evaluationEnv) (any, error) {
		cond, err := validateExprResult[bool](env, condExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate first argument (cond) to option constructor")
		}
		value, err := valueExpr(env)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate second argument (value) to option constructor")
		}
		return option{cond, value}, nil
	}, nil
}

func buildBooleanExprBuilder(f func(bool, bool) bool, name string) func(lhs any, rhs any) (expr, error) {
	return func(lhs, rhs any) (expr, error) {
		aExpr, err := validateExpr[bool](lhs)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse left side of %s operator", name)
		}
		bExpr, err := validateExpr[bool](rhs)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse right side of %s operator", name)
		}
		return func(env *evaluationEnv) (any, error) {
			lhs, err := validateExprResult[bool](env, aExpr)
			if err != nil {
				return nil, trace.Wrap(err, "failed to evaluate left side of %s operator", name)
			}
			rhs, err := validateExprResult[bool](env, bExpr)
			if err != nil {
				return nil, trace.Wrap(err, "failed to evaluate right side of %s operator", name)
			}
			return f(lhs, rhs), nil
		}, nil
	}
}

var (
	buildAndExpr = buildBooleanExprBuilder(func(lhs, rhs bool) bool { return lhs && rhs }, "&&")
	buildOrExpr  = buildBooleanExprBuilder(func(lhs, rhs bool) bool { return lhs || rhs }, "||")
)

func buildNotExpr(arg any) (expr, error) {
	argExpr, err := validateExpr[bool](arg)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse operand to ! (NOT) operator")
	}
	return func(env *evaluationEnv) (any, error) {
		arg, err := validateExprResult[bool](env, argExpr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate operand to ! (NOT) operator")
		}
		return !arg, nil
	}, nil
}

// remover is an interface used so that the parser can call the "remove" method
// on both set and dict.
type remover interface {
	remove(items ...string) any
}

func buildRemoveMethodExpr(recv expr, args ...any) (expr, error) {
	argExprs, err := validateExprs[string](args...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse arguments to remove method")
	}
	return func(env *evaluationEnv) (any, error) {
		r, err := validateExprResult[remover](env, recv)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate receiver for remove method")
		}
		args, err := validateExprResults[string](env, argExprs...)
		if err != nil {
			return nil, trace.Wrap(err, "failed to evaluate argument to remove method")
		}
		return r.remove(args...), nil
	}, nil
}

// validateStringOrSetExpr is meant to coerce an expression argument which must be
// either a string or set into a subexpression rather than a string literal.
func validateStringOrSetExpr(arg any) (expr, error) {
	switch e := arg.(type) {
	case expr:
		// Can't validate expression result type at parse time, this must be
		// checked during evaluation.
		return e, nil
	case string:
		// The predicate parser may return a string literal rather than an expr.
		// Convert it to an expr to avoid special cases during evaluation.
		// Set literals are impossible, they must be returned from an expr.
		return buildLiteralExpr(arg), nil
	default:
		return nil, trace.BadParameter("expected expression or value of type string, got %T", arg)
	}
}

// validateExpr is a generic function meant to coerce expression arguments into
// subexpressions instead of literal types which may be supplied when the
// predicate parser encounters a string or int literal. The generic type T is
// used to validate that any literals must have the expected type, and generates
// a more specific error message when an unexpected type is encountered.
func validateExpr[T any](arg any) (expr, error) {
	switch e := arg.(type) {
	case expr:
		// Can't validate expression result type at parse time, this must be
		// checked during evaluation (by validateExprResult).
		return e, nil
	case T:
		// The predicate parser may return literal types like string or int
		// rather than an expr. If it is the expected type, convert it to an
		// expr to avoid special cases during evaluation.
		return buildLiteralExpr(arg), nil
	default:
		// Catch cases where we expect a specific type (or any expression), but
		// instead we got a literal of the wrong type.
		return nil, trace.BadParameter("expected expression or value of type %T, got %T", *new(T), arg)
	}
}

// validateExprs calls validateExpr for each argument and returns the results in
// a slice, or a single error for the first argument which failed.
func validateExprs[T any](args ...any) ([]expr, error) {
	exprs := make([]expr, len(args))
	for i, arg := range args {
		e, err := validateExpr[T](arg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		exprs[i] = e
	}
	return exprs, nil
}

// validateExprResult is a generic function that evaluates a given expression
// and then attempts to coerce the result to type T. It returns a non-nil error
// if the evaluation fails or the result has the wrong type.
func validateExprResult[T any](env *evaluationEnv, e expr) (T, error) {
	var result T
	resultAny, err := e(env)
	if err != nil {
		return result, trace.Wrap(err)
	}
	result, ok := resultAny.(T)
	if !ok {
		return result, trace.BadParameter("expected value of type %T, got %T", *new(T), resultAny)
	}
	return result, nil
}

// validateExprResults calls validateExpr for each argument expr and returns the
// results in a slice, or a single error for the first expr which failed.
func validateExprResults[T any](env *evaluationEnv, exprs ...expr) ([]T, error) {
	results := make([]T, len(exprs))
	for i, e := range exprs {
		result, err := validateExprResult[T](env, e)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results[i] = result
	}
	return results, nil
}
