package loginrule

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate"
)

// parseEnv holds the "environment" including all identifiers which will be
// available to predicate expressions.
type parseEnv struct {
	// external holds the input traits which are referred to by the name
	// "external", in keeping with the syntax from role templates. For the
	// lowest priority login rule these will be the external traits coming from
	// the identity provider, for subsequent login rules this should be set to
	// the output traits of the previous login rule.
	external dict
}

func (p *parseEnv) getIdentifier(fields []string) (interface{}, error) {
	switch len(fields) {
	case 1:
		switch fields[0] {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "external":
			return p.external, nil
		default:
			return unknownIdentifier(fields[0]), nil
		}
	case 2:
		if fields[0] != "external" {
			return nil, trace.NotFound("identifier %q not found in env", fields[0])
		}
		return p.external[fields[1]], nil
	default:
		return nil, trace.BadParameter("error parsing %v: unsupported fields length: %d", fields, len(fields))
	}
}

// newParser returns a predicate.Parser set up with the given environment and
// the support functions and methods available to login rule predicate
// expression.
//
// TODO(nklaassen): implement remaining predicate helper functions https://github.com/gravitational/teleport/blob/master/rfd/0078-login-rules.md#predicate-helper-functions
func newParser(env *parseEnv) (predicate.Parser, error) {
	parser, err := predicate.NewParser(predicate.Def{
		Operators: predicate.Operators{
			AND: predicate.And,
			OR:  predicate.Or,
			NOT: predicate.Not,
		},
		GetIdentifier: env.getIdentifier,
		GetProperty:   predicate.GetStringMapValue,
		Functions: map[string]any{
			"set":                newSet,
			"dict":               newDict,
			"pair":               newPair,
			"union":              union,
			"ifelse":             ifelse,
			"strings.upper":      upper,
			"strings.lower":      lower,
			"strings.replaceall": replaceAll,
			"choose":             choose,
			"option":             newOption,
		},
		Methods: map[string]any{
			"add":        set.add,
			"contains":   set.contains,
			"put":        dict.put,
			"add_values": dict.addValues,
			"remove":     remover.remove,
		},
	})
	return parser, trace.Wrap(err)
}

type unknownIdentifier string

// remover is an interface used so that the parser can call the "remove" method
// on both set and dict.
type remover interface {
	remove(items ...any) (any, error)
}

type set map[string]struct{}

func newSet(values ...string) set {
	s := make(set, len(values))
	for _, value := range values {
		s[value] = struct{}{}
	}
	return s
}

// clone returns a copy of [s].
func (s set) clone() set {
	copy := make(set, len(s))
	for k := range s {
		copy[k] = struct{}{}
	}
	return copy
}

func (s set) items() []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out
}

func (s set) contains(value any) (bool, error) {
	str, ok := value.(string)
	if !ok {
		return false, trace.BadParameter("argument to set.contains must have type string, got %T", value)
	}
	_, ok = s[str]
	return ok, nil
}

// add returns a copy of the set with the given values added.
func (s set) add(values ...any) (set, error) {
	out := make(set)
	for value := range s {
		out[value] = struct{}{}
	}
	for _, value := range values {
		str, ok := value.(string)
		if !ok {
			return nil, trace.BadParameter("arguments to set.add must have type string, got %T", value)
		}
		out[str] = struct{}{}
	}
	return out, nil
}

// remove returns a copy of the set with values added.
func (s set) remove(values ...any) (any, error) {
	out := make(set, len(s))
	for value := range s {
		out[value] = struct{}{}
	}
	for _, value := range values {
		str, ok := value.(string)
		if !ok {
			return nil, trace.BadParameter("arguments to set.remove must have type string, got %T", value)
		}
		delete(out, str)
	}
	return out, nil
}

func union(sets ...any) (set, error) {
	result := make(set)
	for _, value := range sets {
		s, ok := value.(set)
		if !ok {
			return nil, trace.BadParameter("arguments to union must have type set, got %T", value)

		}
		for v := range s {
			result[v] = struct{}{}
		}
	}
	return result, nil
}

type dict map[string]set

// newDict returns a dict initialized with the key-value pairs as specified in
// [pairs].
func newDict(pairs ...any) (dict, error) {
	d := make(dict, len(pairs))
	for _, pairArg := range pairs {
		p, ok := pairArg.(pair)
		if !ok {
			return nil, trace.BadParameter("arguments to dict must have type pair, got %T", pairArg)
		}
		d[p.first] = p.second
	}
	return d, nil
}

// clone returns a deep copy of [d].
func (d dict) clone() dict {
	copy := make(dict, len(d))
	for key, set := range d {
		copy[key] = set.clone()
	}
	return copy
}

// addValues returns a copy of [d] with [values] added at [key].
func (d dict) addValues(key any, values ...any) (dict, error) {
	keyStr, ok := key.(string)
	if !ok {
		return nil, trace.BadParameter("first argument (key) to dict.add_values must have type string, got %T", key)
	}

	copy := d.clone()
	for _, value := range values {
		valueStr, ok := value.(string)
		if !ok {
			return nil, trace.BadParameter("variadic arguments (values) to dict.add_values must have type string, got %T", value)
		}
		s := copy[keyStr]
		if s == nil {
			copy[keyStr] = map[string]struct{}{
				valueStr: struct{}{},
			}
		} else {
			copy[keyStr][valueStr] = struct{}{}
		}
	}
	return copy, nil
}

// remove returns a copy of [d] with [keys] removed.
func (d dict) remove(keys ...any) (any, error) {
	copy := d.clone()
	for _, key := range keys {
		keyStr, ok := key.(string)
		if !ok {
			return nil, trace.BadParameter("arguments (keys) to dict.remove must have type string, got %T", key)
		}
		delete(copy, keyStr)
	}
	return copy, nil
}

// put returns a copy of [d] with [key] set to [value].
func (d dict) put(key, value any) (dict, error) {
	keyStr, ok := key.(string)
	if !ok {
		return nil, trace.BadParameter("first argument (key) to dict.put must have type string, got %T", key)
	}
	valueSet, ok := value.(set)
	if !ok {
		return nil, trace.BadParameter("second argument (value) to dict.put must have type set, got %t", value)
	}
	copy := d.clone()
	copy[keyStr] = valueSet
	return copy, nil
}

type pair struct {
	first  string
	second set
}

func newPair(first string, second set) pair {
	return pair{
		first:  first,
		second: second,
	}
}

func boolValue(expr any) (bool, error) {
	switch v := expr.(type) {
	case predicate.BoolPredicate:
		return v(), nil
	case bool:
		return v, nil
	default:
		return false, trace.BadParameter("expected bool or predicate.BoolPredicate, got %T", v)
	}
}

func ifelse(cond, valueIfTrue, valueIfFalse any) (any, error) {
	b, err := boolValue(cond)
	if err != nil {
		return nil, trace.Wrap(err, "parsing first argument to ifelse")
	}
	if b {
		return valueIfTrue, nil
	}
	return valueIfFalse, nil
}

func choose(options ...any) (any, error) {
	for _, optionAny := range options {
		opt, ok := optionAny.(*option)
		if !ok {
			return nil, trace.BadParameter("arguments to choose must have type option, got %T", optionAny)
		}
		if opt.condition {
			return opt.value, nil
		}
	}
	return nil, trace.BadParameter(`parsing choose expression: no option could be selected, consider adding a default option by hardcoding the condition to "true"`)
}

type option struct {
	condition bool
	value     any
}

func newOption(cond, value any) (*option, error) {
	b, err := boolValue(cond)
	if err != nil {
		return nil, trace.Wrap(err, "parsing first argument to option")
	}
	return &option{
		condition: b,
		value:     value,
	}, nil
}

// stringTransform transforms [input], using [f].
// It returns either a `string` or a `set`, depending on [input].
func stringTransform(input any, f func(string) string) (any, error) {
	switch v := input.(type) {
	case string:
		return f(v), nil
	case set:
		out := make(set, len(v))
		for str := range v {
			out[f(str)] = struct{}{}
		}
		return out, nil
	}
	return nil, trace.BadParameter("expected string or set, got %T", input)
}

func upper(input any) (any, error) {
	out, err := stringTransform(input, strings.ToUpper)
	return out, trace.Wrap(err, "parsing upper")
}

func lower(input any) (any, error) {
	out, err := stringTransform(input, strings.ToLower)
	return out, trace.Wrap(err, "parsing upper")
}

func replaceAll(input, match, replacement any) (any, error) {
	matchStr, ok := match.(string)
	if !ok {
		return nil, trace.BadParameter("second argument (match) to strings.replaceall must have type string, got %T", match)
	}
	replacementStr, ok := replacement.(string)
	if !ok {
		return nil, trace.BadParameter("third argument (replacement) to strings.replaceall must have type string, got %T", replacement)
	}
	out, err := stringTransform(input, func(inputStr string) string {
		return strings.ReplaceAll(inputStr, matchStr, replacementStr)
	})
	return out, trace.Wrap(err, "parsing strings.replaceall")
}
