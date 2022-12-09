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
		},
		Methods: map[string]any{
			"add":      set.add,
			"remove":   set.remove,
			"contains": set.contains,
		},
	})
	return parser, trace.Wrap(err)
}

type unknownIdentifier string

type set map[string]struct{}

func newSet(values ...string) set {
	s := make(map[string]struct{}, len(values))
	for _, value := range values {
		s[value] = struct{}{}
	}
	return s
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
func (s set) remove(values ...any) (set, error) {
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

func newDict(pairs ...pair) dict {
	d := make(dict, len(pairs))
	for _, p := range pairs {
		d[p.first] = p.second
	}
	return d
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

func ifelse(cond, valueIfTrue, valueIfFalse any) (any, error) {
	var b bool
	switch v := cond.(type) {
	case predicate.BoolPredicate:
		b = v()
	case bool:
		b = v
	default:
		return nil, trace.BadParameter("first argument to ifelse must be bool or predicate.BoolPredicate, got %T", v)
	}
	if b {
		return valueIfTrue, nil
	}
	return valueIfFalse, nil
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
