package loginrule

import (
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
		if fields[0] != "external" {
			return unknownIdentifier(fields[0]), nil
		}
		return p.external, nil
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
			"set":  newSet,
			"dict": newDict,
			"pair": newPair,
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

func union(sets ...set) set {
	result := make(set)
	for _, s := range sets {
		for v := range s {
			result[v] = struct{}{}
		}
	}
	return result
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
