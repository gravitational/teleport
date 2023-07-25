package loginrule

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/parse"
	"github.com/gravitational/teleport/lib/utils/typical"
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

type loginRuleExpr typical.Expression[evaluationEnv, any]

func parseExpr(input string) (loginRuleExpr, error) {
	expr, err := loginRuleParser.Parse(input)
	return expr, trace.Wrap(err)
}

var loginRuleParser = mustLoginRuleParser()

func mustLoginRuleParser() *typical.Parser[evaluationEnv, any] {
	parser, err := typical.NewParser[evaluationEnv, any](typical.ParserSpec{
		Variables: map[string]typical.Variable{
			"true":  true,
			"false": false,
			"external": typical.DynamicMap[evaluationEnv, set](func(env evaluationEnv) (dict, error) {
				return env.external, nil
			}),
		},
		Functions: map[string]typical.Function{
			"set": typical.UnaryVariadicFunction[evaluationEnv](
				func(args ...string) (set, error) {
					return newSet(args...), nil
				}),
			"dict": typical.UnaryVariadicFunction[evaluationEnv](
				func(pairs ...pair) (dict, error) {
					return newDict(pairs...)
				}),
			"pair": typical.BinaryFunction[evaluationEnv](
				func(a, b any) (pair, error) {
					return pair{a, b}, nil
				}),
			"union": typical.UnaryVariadicFunction[evaluationEnv](
				func(sets ...set) (set, error) {
					return union(sets...), nil
				}),
			"ifelse": typical.TernaryFunction[evaluationEnv](
				func(cond bool, a, b any) (any, error) {
					if cond {
						return a, nil
					}
					return b, nil
				}),
			"strings.upper": typical.UnaryFunction[evaluationEnv](
				func(input any) (any, error) {
					return stringTransform("strings.upper", input, strings.ToUpper)
				}),
			"strings.lower": typical.UnaryFunction[evaluationEnv](
				func(input any) (any, error) {
					return stringTransform("strings.lower", input, strings.ToLower)
				}),
			"strings.replaceall": typical.TernaryFunction[evaluationEnv](
				func(input any, match string, replacement string) (any, error) {
					f := func(s string) string {
						return strings.ReplaceAll(s, match, replacement)
					}
					return stringTransform("strings.replaceall", input, f)
				}),
			"choose": typical.UnaryVariadicFunction[evaluationEnv](
				func(opts ...option) (any, error) {
					return choose(opts...)
				}),
			"option": typical.BinaryFunction[evaluationEnv](
				func(cond bool, v any) (option, error) {
					return option{cond, v}, nil
				}),
			"email.local": typical.UnaryFunction[evaluationEnv](
				func(emails set) (set, error) {
					locals, err := parse.EmailLocal(emails.items())
					if err != nil {
						return nil, trace.Wrap(err)
					}
					return newSet(locals...), nil
				}),
			"regexp.replace": typical.TernaryFunction[evaluationEnv](
				func(inputs set, match string, replacement string) (set, error) {
					replaced, err := parse.RegexpReplace(inputs.items(), match, replacement)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					return newSet(replaced...), nil
				}),
		},
		Methods: map[string]typical.Function{
			"add": typical.BinaryVariadicFunction[evaluationEnv](
				func(s set, values ...string) (set, error) {
					return s.add(values...), nil
				}),
			"contains": typical.BinaryFunction[evaluationEnv](
				func(s set, str string) (bool, error) {
					return s.contains(str), nil
				}),
			"put": typical.TernaryFunction[evaluationEnv](
				func(d dict, key string, value set) (dict, error) {
					return d.put(key, value), nil
				}),
			"add_values": typical.TernaryVariadicFunction[evaluationEnv](
				func(d dict, key string, values ...string) (dict, error) {
					return d.addValues(key, values...), nil
				}),
			"remove": typical.BinaryVariadicFunction[evaluationEnv](
				func(r remover, items ...string) (any, error) {
					return r.remove(items...), nil
				}),
		},
	})
	if err != nil {
		panic(trace.Wrap(err, "creating login rule parser (this is a bug)"))
	}
	return parser
}

func stringTransform(name string, input any, f func(string) string) (any, error) {
	switch typedInput := input.(type) {
	case string:
		return f(typedInput), nil
	case set:
		return typedInput.transform(f), nil
	default:
		return nil, trace.BadParameter("failed to evaluate argument to %s: expected string or set, got value of type %T", name, input)
	}
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

// remover is an interface used so that the parser can call the "remove" method
// on both set and dict.
type remover interface {
	remove(items ...string) any
}
