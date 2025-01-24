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

package typical_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/predicate"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/typical"
)

func TestParser(t *testing.T) {
	t.Parallel()

	type env struct {
		labels map[string]string
		traits map[string][]string
	}

	parser, err := typical.NewParser[env, bool](typical.ParserSpec[env]{
		Variables: map[string]typical.Variable{
			"labels": typical.DynamicVariable(func(e env) (map[string]string, error) {
				return e.labels, nil
			}),
			"dynamic.labels": typical.DynamicMapFunction(func(e env, key string) (string, error) {
				return e.labels[key], nil
			}),
			"traits": typical.DynamicVariable(func(e env) (map[string][]string, error) {
				return e.traits, nil
			}),
			"true":       true,
			"false":      false,
			"namespaces": []string{"internal", "external"},
		},
		Functions: map[string]typical.Function{
			"not": typical.UnaryFunction[env](func(b bool) (bool, error) {
				return !b, nil
			}),
			"contains": typical.BinaryFunction[env](func(list []string, item string) (bool, error) {
				return slices.Contains(list, item), nil
			}),
			"ifelse": typical.TernaryFunction[env](func(cond bool, a any, b any) (any, error) {
				if cond {
					return a, nil
				}
				return b, nil
			}),
			"concat": typical.UnaryVariadicFunction[env](func(strs ...string) (string, error) {
				return strings.Join(strs, ""), nil
			}),
			"contains_all": typical.BinaryVariadicFunction[env](func(list []string, strs ...string) (bool, error) {
				for _, str := range strs {
					if !slices.Contains(list, str) {
						return false, nil
					}
				}
				return true, nil
			}),
			"error": typical.UnaryFunction[env](func(msg string) (any, error) {
				return nil, errors.New(msg)
			}),
			"head": typical.UnaryFunction[env](func(list []string) (string, error) {
				if len(list) == 0 {
					return "", trace.BadParameter("list has length 0")
				}
				return list[0], nil
			}),
			"labels_matching": typical.UnaryFunctionWithEnv(func(e env, keyExpr string) ([]string, error) {
				var matchingLabels []string
				for key, value := range e.labels {
					match, err := utils.MatchString(key, keyExpr)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					if match {
						matchingLabels = append(matchingLabels, value)
					}
				}
				return matchingLabels, nil
			}),
		},
		Methods: map[string]typical.Function{
			"add_trait_values": typical.TernaryVariadicFunction[env](func(m map[string][]string, key string, values ...string) (map[string][]string, error) {
				c := maps.Clone(m)
				c[key] = append(c[key], values...)
				return c, nil
			}),
		},
	})
	require.NoError(t, err)

	e := env{
		labels: map[string]string{
			"env":  "staging",
			"team": "dev",
		},
		traits: map[string][]string{
			"allow-env": {"dev", "staging"},
			"logins":    {"root", "ubuntu"},
		},
	}

	for _, tc := range []struct {
		desc                  string
		expr                  string
		expectParseError      []string
		expectEvaluationError []string
		expectMatch           bool
	}{
		{
			desc: "empty expression",
			expectParseError: []string{
				"empty expression",
			},
		},
		{
			desc: "unknown variable",
			expr: "nothing",
			expectParseError: []string{
				`unknown identifier: "nothing"`,
			},
		},
		{
			desc: "wrong result type",
			expr: `"not a bool"`,
			expectParseError: []string{
				"expression evaluated to unexpected type",
				"expected type bool, got value (not a bool) with type (string)",
			},
		},
		{
			desc:        "literal",
			expr:        "true",
			expectMatch: true,
		},
		{
			desc:        "unary function expression",
			expr:        "not(true)",
			expectMatch: false,
		},
		{
			desc: "unary function wrong type",
			expr: `not("true")`,
			expectParseError: []string{
				"parsing argument to (not)",
				"expected type bool, got value (true) with type (string)",
			},
		},
		{
			desc:        "negation of literal",
			expr:        "!true",
			expectMatch: false,
		},
		{
			desc:        "negation of expression",
			expr:        "!not(true)",
			expectMatch: true,
		},
		{
			desc: "negation of wrong type",
			expr: `!"test"`,
			expectParseError: []string{
				"parsing target of (!) operator",
				"expected type bool, got value (test) with type (string)",
			},
		},
		{
			desc:        "and literals",
			expr:        "true && false",
			expectMatch: false,
		},
		{
			desc:        "and expressions",
			expr:        "not(true) && not(false) && true",
			expectMatch: false,
		},
		{
			desc: "and with wrong type",
			expr: `true && "test"`,
			expectParseError: []string{
				"parsing rhs of (&&) operator",
				"expected type bool, got value (test) with type (string)",
			},
		},
		{
			desc:        "or literals",
			expr:        "true || false",
			expectMatch: true,
		},
		{
			desc:        "or expressions",
			expr:        "not(true) || not(false) || true",
			expectMatch: true,
		},
		{
			desc: "unary func no args",
			expr: "not()",
			expectParseError: []string{
				"function (not) accepts 1 argument, given 0",
			},
		},
		{
			desc:        "expression as argument",
			expr:        "not(not(true))",
			expectMatch: true,
		},
		{
			desc:        "literal string equality",
			expr:        `"test1" == "test2"`,
			expectMatch: false,
		},
		{
			desc:        "binary function with map lookups",
			expr:        `contains(traits["allow-env"], labels["env"])`,
			expectMatch: true,
		},
		{
			desc:        "dynamic map lookup",
			expr:        `contains(traits["allow-env"], dynamic.labels["env"])`,
			expectMatch: true,
		},
		{
			desc:        "map.key syntax",
			expr:        `contains(traits.logins, "root")`,
			expectMatch: true,
		},
		{
			desc: "key with wrong type",
			expr: `traits[false]`,
			expectParseError: []string{
				"parsing key of index expression",
				"expected type string",
			},
		},
		{
			desc: "indexing non-map",
			expr: `concat("a", "b")["key"]`,
			expectParseError: []string{
				"cannot take index of unexpected type",
				"expected type map",
				"got expression returning type (string)",
			},
		},
		{
			desc: "argument is expression returning wrong type",
			expr: `contains(traits, "root")`,
			expectParseError: []string{
				"parsing first argument to (contains)",
				"expected type []string, got expression returning type (map[string][]string)",
			},
		},
		{
			desc: "binary function with too many arguments",
			expr: `contains(traits["logins"], "root", "user")`,
			expectParseError: []string{
				"function (contains) accepts 2 arguments, given 3",
			},
		},
		{
			desc:        "string works as []string",
			expr:        `contains("test", "test")`,
			expectMatch: true,
		},
		{
			desc:        "string expression works as []string",
			expr:        `contains(concat("te", "st"), "test")`,
			expectMatch: true,
		},
		{
			desc:        "correct runtime type",
			expr:        `ifelse(true, true, "test") || true`,
			expectMatch: true,
		},
		{
			desc: "incorrect runtime type",
			expr: `ifelse(false, true, "test") || true`,
			expectEvaluationError: []string{
				"evaluating lhs of (||) operator",
				"expected type bool, got value (test) with type (string)",
			},
		},
		{
			desc:        "expression as interface argument",
			expr:        `ifelse(false, labels["env"], labels["team"]) == "dev"`,
			expectMatch: true,
		},
		{
			desc:        "unary variadic function",
			expr:        `concat("Hello", ", ", "World!") == "Hello, World!"`,
			expectMatch: true,
		},
		{
			desc: "unary variadic function wrong type",
			expr: `concat("Hello", ", ", "World!", false) == "Hello, World!"`,
			expectParseError: []string{
				"expected type string, got value (false) with type (bool)",
			},
		},
		{
			desc:        "binary variadic function",
			expr:        `contains_all(traits["logins"], "root", "ubuntu")`,
			expectMatch: true,
		},
		{
			desc: "ternary variadic method",
			expr: `contains_all(
				traits.add_trait_values("logins",
					"usera", "userb", "userc",
				)["logins"],
				"root",
				"userc",
				"userb",
				"usera",
			)`,
			expectMatch: true,
		},
		{
			desc:        "unary func with env",
			expr:        `contains_all(labels_matching("*"), "staging", "dev")`,
			expectMatch: true,
		},
		{
			desc: "unary func with env no arg",
			expr: `contains_all(labels_matching(), "staging", "dev")`,
			expectParseError: []string{
				"function (labels_matching) accepts 1 argument, given 0",
			},
		},
		{
			desc: "unary func with env wrong type",
			expr: `contains_all(labels_matching(traits["username"]), "staging", "dev")`,
			expectParseError: []string{
				"parsing argument to (labels_matching)",
				"expected type string, got expression returning type ([]string)",
			},
		},
		{
			desc: "unsupported function",
			expr: `compare(traits.logins, "user")`,
			expectParseError: []string{
				"unsupported function: compare",
			},
		},
		{
			desc: "unmatched parens",
			expr: "not(true,",
			expectParseError: []string{
				"expected ')', found 'EOF'",
			},
		},
		{
			desc: "error evaluating key",
			expr: `labels[error("haha")] == "test"`,
			expectEvaluationError: []string{
				"evaluating key of index expression",
				"haha",
			},
		},
		{
			desc: "error evaluating dynamic key",
			expr: `dynamic.labels[error("haha")] == "test"`,
			expectEvaluationError: []string{
				"evaluating key of index expression",
				"haha",
			},
		},
		{
			desc: "error evaluating argument",
			expr: `not(error("haha"))`,
			expectEvaluationError: []string{
				"evaluating argument to function (not)",
				"haha",
			},
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			expr, err := parser.Parse(tc.expr)
			for _, msg := range tc.expectParseError {
				require.ErrorContains(t, err, msg)
			}
			if len(tc.expectParseError) > 0 {
				return
			}
			require.NoError(t, err, trace.DebugReport(err))

			match, err := expr.Evaluate(e)
			for _, msg := range tc.expectEvaluationError {
				require.ErrorContains(t, err, msg)
			}
			if len(tc.expectEvaluationError) > 0 {
				return
			}
			require.NoError(t, err, trace.DebugReport(err))

			require.Equal(t, tc.expectMatch, match)
		})
	}
}

func TestUnknownIdentifier(t *testing.T) {
	t.Parallel()

	type metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	}

	type spec struct {
		Hostname string `json:"hostname"`
	}

	type resource struct {
		Metadata metadata `json:"metadata"`
		Spec     spec     `json:"spec"`
	}

	cases := []struct {
		name               string
		expression         string
		knownVariablesOnly bool
		parseAssertion     require.ErrorAssertionFunc
		evalAssertion      func(t *testing.T, ok bool, err error)
	}{
		{
			name:           "dynamic variable indexed and passed to function",
			expression:     `hasPrefix(resource.metadata.labels["foo"], "b")`,
			parseAssertion: require.NoError,
			evalAssertion: func(t *testing.T, ok bool, err error) {
				assert.True(t, ok)
				require.NoError(t, err)
			},
		},
		{
			name:           "dynamic variable found via path not found",
			expression:     `hasPrefix(resource.metadata.labels.foo, "b")`,
			parseAssertion: require.NoError,
			evalAssertion: func(t *testing.T, ok bool, err error) {
				assert.False(t, ok)
				require.Error(t, err)
			},
		},
		{
			name:           "incorrect type from dynamic variable passed to function",
			expression:     `hasPrefix(resource.metadata.labels, "b")`,
			parseAssertion: require.NoError,
			evalAssertion: func(t *testing.T, ok bool, err error) {
				assert.False(t, ok)
				require.Error(t, err)
			},
		},
		{
			name:           "dynamic map",
			expression:     `exists(resource.metadata.labels, "foo")`,
			parseAssertion: require.NoError,
			evalAssertion: func(t *testing.T, ok bool, err error) {
				assert.True(t, ok)
				require.NoError(t, err)
			},
		},
		{
			name:           "unknown dynamic variable",
			expression:     "unknown",
			parseAssertion: require.NoError,
			evalAssertion: func(t *testing.T, ok bool, err error) {
				assert.False(t, ok)
				require.ErrorContains(t, err, "missing field names")
			},
		},
		{
			name:               "unknown variable",
			expression:         "unknown",
			knownVariablesOnly: true,
			parseAssertion: func(t require.TestingT, err error, i ...interface{}) {
				var u typical.UnknownIdentifierError
				require.ErrorAs(t, err, &u, i...)
				require.ErrorAs(t, trace.Wrap(err), &u, i...)
				require.Equal(t, "unknown", u.Identifier(), i...)
			},
		},
	}

	srv := resource{
		Metadata: metadata{
			Name:   "test",
			Labels: map[string]string{"foo": "bar", "animal": "llama"},
		},
		Spec: spec{Hostname: "server01"},
	}

	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			spec := typical.ParserSpec[resource]{
				Functions: map[string]typical.Function{
					"hasPrefix": typical.BinaryFunction[resource, string, string, bool](func(s, suffix string) (bool, error) {
						return strings.HasPrefix(s, suffix), nil
					}),
					"exists": typical.BinaryFunction[resource, map[string]string, string, bool](func(m map[string]string, key string) (bool, error) {
						_, ok := m[key]
						return ok, nil
					}),
				},
				GetUnknownIdentifier: func(env resource, fields []string) (any, error) {
					f, err := predicate.GetFieldByTag(env, teleport.JSON, fields[1:])
					return f, trace.Wrap(err)
				},
			}

			if test.knownVariablesOnly {
				spec.GetUnknownIdentifier = nil
			}

			parser, err := typical.NewParser[resource, bool](spec)
			require.NoError(t, err, "creating parser")

			expression, err := parser.Parse(test.expression)
			test.parseAssertion(t, err)
			if err != nil {
				return
			}

			ok, err := expression.Evaluate(srv)
			test.evalAssertion(t, ok, err)
		})
	}
}
