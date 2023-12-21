package typical

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/parse"
	"github.com/gravitational/teleport/lib/utils/typical"
)

type evaluationEnvVar map[string]typical.Variable

func defaultParserSpec[T any]() typical.ParserSpec {
	return typical.ParserSpec{
		Functions: map[string]typical.Function{
			"set": typical.UnaryVariadicFunction[T](
				func(args ...string) (Set, error) {
					return NewSet(args...), nil
				}),
			"dict": typical.UnaryVariadicFunction[T](
				func(pairs ...pair) (Dict, error) {
					return NewDict(pairs...)
				}),
			"pair": typical.BinaryFunction[T](
				func(a, b any) (pair, error) {
					return pair{a, b}, nil
				}),
			"union": typical.UnaryVariadicFunction[T](
				func(sets ...Set) (Set, error) {
					return union(sets...), nil
				}),
			"ifelse": typical.TernaryFunction[T](
				func(cond bool, a, b any) (any, error) {
					if cond {
						return a, nil
					}
					return b, nil
				}),
			"strings.upper": typical.UnaryFunction[T](
				func(input any) (any, error) {
					return StringTransform("strings.upper", input, strings.ToUpper)
				}),
			"strings.lower": typical.UnaryFunction[T](
				func(input any) (any, error) {
					return StringTransform("strings.lower", input, strings.ToLower)
				}),
			"strings.replaceall": typical.TernaryFunction[T](
				func(input any, match string, replacement string) (any, error) {
					f := func(s string) string {
						return strings.ReplaceAll(s, match, replacement)
					}
					return StringTransform("strings.replaceall", input, f)
				}),
			"choose": typical.UnaryVariadicFunction[T](
				func(opts ...option) (any, error) {
					return choose(opts...)
				}),
			"option": typical.BinaryFunction[T](
				func(cond bool, v any) (option, error) {
					return option{cond, v}, nil
				}),
			"email.local": typical.UnaryFunction[T](
				func(emails Set) (Set, error) {
					locals, err := parse.EmailLocal(emails.items())
					if err != nil {
						return nil, trace.Wrap(err)
					}
					return NewSet(locals...), nil
				}),
			"regexp.replace": typical.TernaryFunction[T](
				func(inputs Set, match string, replacement string) (Set, error) {
					replaced, err := parse.RegexpReplace(inputs.items(), match, replacement)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					return NewSet(replaced...), nil
				}),
			"strings.split": typical.BinaryFunction[T](
				func(inputs Set, sep string) (Set, error) {
					var outputs []string
					for input := range inputs {
						outputs = append(outputs, strings.Split(input, sep)...)
					}
					return NewSet(outputs...), nil
				}),
		},
		Methods: map[string]typical.Function{
			"add": typical.BinaryVariadicFunction[T](
				func(s Set, values ...string) (Set, error) {
					return s.add(values...), nil
				}),
			"contains": typical.BinaryFunction[T](
				func(s Set, str string) (bool, error) {
					return s.contains(str), nil
				}),
			"put": typical.TernaryFunction[T](
				func(d Dict, key string, value Set) (Dict, error) {
					return d.put(key, value), nil
				}),
			"add_values": typical.TernaryVariadicFunction[T](
				func(d Dict, key string, values ...string) (Dict, error) {
					return d.addValues(key, values...), nil
				}),
			"remove": typical.BinaryVariadicFunction[T](
				func(r remover, items ...string) (any, error) {
					return r.remove(items...), nil
				}),
		},
	}
}

// NewTypicalParser returns new typical parser using evaluation environment and default parser spec.
func NewTypicalParser[T any](vars evaluationEnvVar) (*typical.Parser[T, any], error) {
	defParserSpec := defaultParserSpec[T]()
	defParserSpec.Variables = vars
	parser, err := typical.NewParser[T, any](defParserSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return parser, nil
}

// traitsMapResultToSet returns Set of result string or set) and erros if the result
// cannot be evaluated to either Set or string.
func traitsMapResultToSet(result any, expr string) (Set, error) {
	switch v := result.(type) {
	case string:
		return NewSet(v), nil
	case Set:
		return v, nil
	default:
		return nil, trace.BadParameter("traits_map expression must evaluate to type string or set, the following expression evaluates to %T: %q", result, expr)
	}
}

// StringSliceFromDict returns string slice from a Dict.
func StringSliceFromDict(d Dict) []string {
	m := make([]string, 0, len(d))
	for _, s := range d {
		m = append(m, s.items()...)
	}
	return m
}

// StringSliceMapFromDict returns string slice map from a Dict.
func StringSliceMapFromDict(d Dict) map[string][]string {
	m := make(map[string][]string, len(d))
	for key, s := range d {
		m[key] = s.items()
	}
	return m
}

// DictFromStringSliceMap returns Dict from a string slices map type.
func DictFromStringSliceMap(m map[string][]string) Dict {
	d := make(Dict, len(m))
	for key, values := range m {
		d[key] = NewSet(values...)
	}
	return d
}

// DictFromStringSlice returns Dict from string slice.
func DictFromStringSlice(key string, s []string) Dict {
	d := make(Dict, len(s))
	d[key] = NewSet(s...)
	return d
}

// StringTransform transforms string formt.
func StringTransform(name string, input any, f func(string) string) (any, error) {
	switch typedInput := input.(type) {
	case string:
		return f(typedInput), nil
	case Set:
		return typedInput.transform(f), nil
	default:
		return nil, trace.BadParameter("failed to evaluate argument to %s: expected string or set, got value of type %T", name, input)
	}
}

// remover is an interface used so that the parser can call the "remove" method
// on both set and dict.
type remover interface {
	remove(items ...string) any
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
