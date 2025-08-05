/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package expression

import (
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
	"github.com/gravitational/teleport/lib/utils/typical"
)

type evaluationEnvVar map[string]typical.Variable

// DefaultParserSpec is the default parser specification.
// Contains useful functions for manipulating and comparing strings.
func DefaultParserSpec[evaluationEnv any]() typical.ParserSpec[evaluationEnv] {
	return typical.ParserSpec[evaluationEnv]{
		Functions: map[string]typical.Function{
			"set": typical.UnaryVariadicFunction[evaluationEnv](
				func(args ...string) (Set, error) {
					return NewSet(args...), nil
				}),
			"dict": typical.UnaryVariadicFunction[evaluationEnv](
				func(pairs ...pair) (Dict, error) {
					return NewDict(pairs...)
				}),
			"pair": typical.BinaryFunction[evaluationEnv](
				func(a, b any) (pair, error) {
					return pair{a, b}, nil
				}),
			"union": typical.UnaryVariadicFunction[evaluationEnv](
				func(sets ...Set) (Set, error) {
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
					return StringTransform("strings.upper", input, strings.ToUpper)
				}),
			"strings.lower": typical.UnaryFunction[evaluationEnv](
				func(input any) (any, error) {
					return StringTransform("strings.lower", input, strings.ToLower)
				}),
			"strings.replaceall": typical.TernaryFunction[evaluationEnv](
				func(input any, match string, replacement string) (any, error) {
					f := func(s string) string {
						return strings.ReplaceAll(s, match, replacement)
					}
					return StringTransform("strings.replaceall", input, f)
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
				func(emails Set) (Set, error) {
					locals, err := parse.EmailLocal(emails.items())
					if err != nil {
						return Set{}, trace.Wrap(err)
					}
					return NewSet(locals...), nil
				}),
			"regexp.replace": typical.TernaryFunction[evaluationEnv](
				func(inputs Set, match string, replacement string) (Set, error) {
					replaced, err := parse.RegexpReplace(inputs.items(), match, replacement)
					if err != nil {
						return Set{}, trace.Wrap(err)
					}
					return NewSet(replaced...), nil
				}),
			"strings.split": typical.BinaryFunction[evaluationEnv](
				func(inputs Set, sep string) (Set, error) {
					var outputs []string
					for input := range inputs.s {
						outputs = append(outputs, strings.Split(input, sep)...)
					}
					return NewSet(outputs...), nil
				}),
			"time.RFC3339": typical.UnaryFunction[evaluationEnv](
				func(input string) (time.Time, error) {
					return time.Parse(time.RFC3339, input)
				}),
			"before": typical.BinaryFunction[evaluationEnv](
				func(t time.Time, other time.Time) (bool, error) {
					return t.Before(other), nil
				}),
			"after": typical.BinaryFunction[evaluationEnv](
				func(t time.Time, other time.Time) (bool, error) {
					return t.After(other), nil
				}),
			"between": typical.BinaryVariadicFunction[evaluationEnv](
				func(t time.Time, interval ...time.Time) (bool, error) {
					if len(interval) != 2 {
						return false, trace.BadParameter("between expected 2 parameters: got %v", len(interval))
					}
					first, second := interval[0], interval[1]
					if first.After(second) {
						first, second = second, first
					}
					return t.After(first) && t.Before(second), nil
				}),
			"contains_any": typical.BinaryFunction[evaluationEnv](
				func(s1, s2 Set) (bool, error) {
					for v := range s2.s {
						if s1.contains(v) {
							return true, nil
						}
					}
					return false, nil
				}),
			"contains_all": typical.BinaryFunction[evaluationEnv](
				func(s1, s2 Set) (bool, error) {
					for v := range s2.s {
						if !s1.contains(v) {
							return false, nil
						}
					}
					return len(s2.s) > 0, nil
				}),
			"is_empty": typical.UnaryFunction[evaluationEnv](
				func(s Set) (bool, error) {
					return len(s.s) == 0, nil
				}),
		},
		Methods: map[string]typical.Function{
			"add": typical.BinaryVariadicFunction[evaluationEnv](
				func(s Set, values ...string) (Set, error) {
					return s.add(values...), nil
				}),
			"contains": typical.BinaryFunction[evaluationEnv](
				func(s Set, str string) (bool, error) {
					return s.contains(str), nil
				}),
			"put": typical.TernaryFunction[evaluationEnv](
				func(d Dict, key string, value Set) (Dict, error) {
					return d.put(key, value), nil
				}),
			"add_values": typical.TernaryVariadicFunction[evaluationEnv](
				func(d Dict, key string, values ...string) (Dict, error) {
					return d.addValues(key, values...), nil
				}),
			"remove": typical.BinaryVariadicFunction[evaluationEnv](
				func(r remover, items ...string) (any, error) {
					return r.remove(items...), nil
				}),
			"before": typical.BinaryFunction[evaluationEnv](
				func(t time.Time, other time.Time) (bool, error) {
					return t.Before(other), nil
				}),
			"after": typical.BinaryFunction[evaluationEnv](
				func(t time.Time, other time.Time) (bool, error) {
					return t.After(other), nil
				}),
			"between": typical.BinaryVariadicFunction[evaluationEnv](
				func(t time.Time, interval ...time.Time) (bool, error) {
					if len(interval) != 2 {
						return false, trace.BadParameter("between expected 2 parameters: got %v", len(interval))
					}
					first, second := interval[0], interval[1]
					if first.After(second) {
						first, second = second, first
					}
					return t.After(first) && t.Before(second), nil
				}),
			"contains_any": typical.BinaryFunction[evaluationEnv](
				func(s1, s2 Set) (bool, error) {
					for v := range s2.s {
						if s1.contains(v) {
							return true, nil
						}
					}
					return false, nil
				}),
			"contains_all": typical.BinaryFunction[evaluationEnv](
				func(s1, s2 Set) (bool, error) {
					for v := range s2.s {
						if !s1.contains(v) {
							return false, nil
						}
					}
					return len(s2.s) > 0, nil
				}),
			"isempty": typical.UnaryFunction[evaluationEnv](
				func(s Set) (bool, error) {
					return len(s.s) == 0, nil
				}),
		},
	}
}

// NewTraitsExpressionParser returns new expression parser using evaluation environment and default parser spec.
func NewTraitsExpressionParser[TEnv any](vars evaluationEnvVar) (*typical.Parser[TEnv, any], error) {
	defParserSpec := DefaultParserSpec[TEnv]()
	defParserSpec.Variables = vars
	parser, err := typical.NewParser[TEnv, any](defParserSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return parser, nil
}

// traitsMapResultToSet returns Set for result type string or Set and errors if the result
// cannot be evaluated to either Set or string.
func traitsMapResultToSet(result any, expr string) (Set, error) {
	switch v := result.(type) {
	case string:
		return NewSet(v), nil
	case Set:
		return v, nil
	default:
		return Set{}, trace.BadParameter("traits_map expression must evaluate to type string or set, the following expression evaluates to %T: %q", result, expr)
	}
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

// StringTransform transforms string formt.
func StringTransform(name string, input any, f func(string) string) (any, error) {
	switch typedInput := input.(type) {
	case string:
		return f(typedInput), nil
	case Set:
		return Set{utils.SetTransform(typedInput.s, f)}, nil
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
