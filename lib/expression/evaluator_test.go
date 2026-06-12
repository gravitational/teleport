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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/typical"
)

var (
	baseInputTraits = map[string][]string{
		"groups":   []string{"devs", "security"},
		"username": []string{"alice"},
	}

	testCases = []struct {
		desc           string
		expressions    map[string][]string
		inputTraits    map[string][]string
		expectedTraits map[string][]string
		errorContains  []string
	}{
		{
			desc:           "no rules",
			inputTraits:    baseInputTraits,
			expectedTraits: map[string][]string{},
		},
		{
			desc: "simple traits map",
			expressions: map[string][]string{"groups": []string{
				"user.spec.traits.groups",
			}},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups": []string{"devs", "security"},
			},
		},
		{
			desc:        "wrong map return type",
			expressions: map[string][]string{"groups": []string{"user.spec.traits"}},
			errorContains: []string{
				"traits_map expression must evaluate to type string or set, the following expression evaluates to expression.Dict:",
			},
		},
		{
			desc: "ifelse",
			expressions: map[string][]string{
				"a": []string{
					`ifelse(true, "correct", "wrong")`,
					`ifelse(false, "wrong", "correct")`,
					`ifelse(ifelse(true, true, false), "correct", "wrong")`,
					`set(ifelse(true, "correct", "wrong"), "correct")`,
				},
				"groups": []string{
					`ifelse(true, user.spec.traits.groups, "wrong")`,
				},
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"a":      []string{"correct"},
				"groups": baseInputTraits["groups"],
			},
		},
		{
			desc: "set methods",
			expressions: map[string][]string{
				"extragroups":            []string{`user.spec.traits.groups.add("extra", "surplus")`},
				"fewergroups":            []string{`user.spec.traits.groups.remove("security")`},
				"nogroups":               []string{`user.spec.traits.groups.remove("devs", "security").add("test").remove("test").remove("not-a-group")`},
				"groups-by-another-name": []string{`user.spec.traits.groups.remove("not-a-group")`},
				"logins": []string{
					// user.spec.traits.groups does not contain "admins", so we
					// expect to just get the username.
					`ifelse(user.spec.traits.groups.contains("admins"), user.spec.traits.username.add("root"), user.spec.traits.username)`,
					// user.spec.traits.groups does contain "security", so expect
					// the "security-team" login.
					`ifelse(user.spec.traits.groups.contains("security"), "security-team", set())`,
				},
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"extragroups":            append([]string{"extra", "surplus"}, baseInputTraits["groups"]...),
				"fewergroups":            []string{"devs"},
				"nogroups":               []string{},
				"groups-by-another-name": baseInputTraits["groups"],
				"logins":                 []string{"alice", "security-team"},
			},
		},
		{
			desc: "set union",
			expressions: map[string][]string{
				"groups": []string{`union(user.spec.traits.groups, set("test1", "test2"))`},
				"fruits": []string{`union(set("apple", "banana"), set("cherry"), set("dragonfruit", "eggplant"))`},
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups": append([]string{"test1", "test2"}, baseInputTraits["groups"]...),
				"fruits": []string{"apple", "banana", "cherry", "dragonfruit", "eggplant"},
			},
		},
		{
			desc: "string helpers",
			expressions: map[string][]string{
				"lower": []string{
					`strings.lower("APPLE")`,
					`strings.lower("BaNaNa")`,
					`strings.lower(set("cherry", "dragonFRUIT"))`,
					`strings.lower(user.spec.traits.username)`,
				},
				"upper": []string{
					`strings.upper("APPLE")`,
					`strings.upper("BaNaNa")`,
					`strings.upper(set("cherry", "dragonFRUIT"))`,
					`strings.upper(user.spec.traits.username)`,
				},
				"replaced": []string{
					`strings.replaceall("snake_case_example", "_", "-")`,
					`strings.replaceall(strings.replaceall("user@example.com", "@", "_"), ".", "-")`,
					`strings.replaceall(set("dev-team", "platform-team"), "-team", "")`,
				},
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"lower":    []string{"apple", "banana", "cherry", "dragonfruit", "alice"},
				"upper":    []string{"APPLE", "BANANA", "CHERRY", "DRAGONFRUIT", "ALICE"},
				"replaced": []string{"snake-case-example", "user_example-com", "dev", "platform"},
			},
		},
		{
			desc: "choose",
			expressions: map[string][]string{
				"choose_first": []string{
					`choose(option(true, "first"), option(false, "second"))`,
				},
				"choose_second": []string{
					`choose(option(false, "first"), option(true, "second"))`,
				},
				"groups": []string{
					`choose(
							option(user.spec.traits.username.contains("alice"), set("devs", "security", "requester")),
							option(user.spec.traits.username.contains("bob"), set("security", "reviewer")),
							option(user.spec.traits.username.contains("charlie"), set("devs")),
							option(true, set()),
						)`,
				},
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"choose_first":  []string{"first"},
				"choose_second": []string{"second"},
				"groups":        []string{"devs", "security", "requester"},
			},
		},
		{
			// Test that user.spec.traits traits dict can by indexed like
			// user.spec.traits["trait"] as well as user.spec.traits.trait (the latter syntax
			// does not support traits containing hyphens or some other special
			// characters).
			desc: "dict index",
			expressions: map[string][]string{
				"test":        {`user.spec.traits.test`},
				"with-hyphen": {`user.spec.traits["with-hyphen"]`},
			},
			inputTraits: map[string][]string{
				"test":        {"test"},
				"with-hyphen": {"-"},
			},
			expectedTraits: map[string][]string{
				"test":        {"test"},
				"with-hyphen": {"-"},
			},
		},
		{
			// Test that return value of helper (contains) can be handled by `||`,
			// and return value of `||` can be handled by helper (ifelse).
			desc: "boolean expressions",
			expressions: map[string][]string{
				"groups": {
					`ifelse(user.spec.traits.groups.contains("security") || user.spec.traits.groups.contains("it"),
							user.spec.traits.groups.add("admins"),
							user.spec.traits.groups)`,
				},
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups": {"devs", "security", "admins"},
			},
		},
		{
			desc: "traits_map quoted or unquoted strings",
			expressions: map[string][]string{
				"test": {`a`, `"b"`},
			},
			expectedTraits: map[string][]string{
				"test": {"a", "b"},
			},
		},
		{
			desc: "bad regexp",
			expressions: map[string][]string{
				"logins": {`regexp.replace(user.spec.traits.email, "(.*@example.com", "$1")`},
			},
			inputTraits: map[string][]string{
				"email": {"alice@example.com"},
			},
			errorContains: []string{
				"evaluating function (regexp.replace)",
				`invalid regexp "(.*@example.com"`,
			},
		},
		{
			desc: "strings.split",
			expressions: map[string][]string{
				"logins": {`strings.split(user.spec.traits.commaLogins, ",")`},
				"localEmails": {
					`email.local(strings.split(user.spec.traits.oneSpaceEmails, " "))`,
					`email.local(strings.split(user.spec.traits.twoSpaceEmails, "  "))`,
					`email.local(strings.split(user.spec.traits.singleEmail, ","))`,
				},
			},
			inputTraits: map[string][]string{
				"commaLogins":    {"alice,bob,charlie"},
				"oneSpaceEmails": {"alice@example.com bob@example.com charlie@example.com"},
				"twoSpaceEmails": {"darrell@example.com  esther@example.com"},
				"singleEmail":    {"frank@example.com"},
			},
			expectedTraits: map[string][]string{
				"logins":      {"alice", "bob", "charlie"},
				"localEmails": {"alice", "bob", "charlie", "darrell", "esther", "frank"},
			},
		},
		{
			desc: "methods on nil set from nonexistent map key",
			expressions: map[string][]string{
				"a": {`user.spec.traits["a"].add("a")`},
				"b": {`ifelse(user.spec.traits["b"].contains("b"), set("z"), set("b"))`},
				"c": {`ifelse(user.spec.traits["c"].contains_any(set("c")), set("z"), set("c"))`},
				"d": {`ifelse(user.spec.traits["d"].isempty(), set("d"), set("z"))`},
				"e": {`user.spec.traits["e"].remove("e")`},
				"f": {`user.spec.traits["f"].remove("f").add("f")`},
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"a": {"a"},
				"b": {"b"},
				"c": {"c"},
				"d": {"d"},
				"e": {},
				"f": {"f"},
			},
		},
	}
)

func TestEvaluateTraitsMap(t *testing.T) {
	t.Parallel()

	type evaluationEnv struct {
		Traits Dict
	}

	typicalEnvVar := map[string]typical.Variable{
		"true":  true,
		"false": false,
		"user.spec.traits": typical.DynamicMap[evaluationEnv, Set](func(env evaluationEnv) (Dict, error) {
			return env.Traits, nil
		}),
	}

	attributeParser, err := NewTraitsExpressionParser[evaluationEnv](typicalEnvVar)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := EvaluateTraitsMap[evaluationEnv](
				evaluationEnv{
					Traits: DictFromStringSliceMap(tc.inputTraits),
				},
				tc.expressions,
				func(input string) (typical.Expression[evaluationEnv, any], error) {
					expr, err := attributeParser.Parse(input)
					return expr, trace.Wrap(err)
				})
			if len(tc.errorContains) > 0 {
				for _, contains := range tc.errorContains {
					require.ErrorContains(t, err, contains, "error string does not contain expected snippet")
				}
				return
			}
			require.NoError(t, err, trace.DebugReport(err))
			require.Empty(t, cmp.Diff(tc.expectedTraits, StringSliceMapFromDict(result), cmpopts.SortSlices(func(a, b string) bool { return a < b })))
		})
	}
}

func FuzzTraitsExpressionParser(f *testing.F) {
	type evaluationEnv struct {
		Traits Dict
	}
	parser, err := NewTraitsExpressionParser[evaluationEnv](map[string]typical.Variable{
		"true":  true,
		"false": false,
		"user.spec.traits": typical.DynamicMap[evaluationEnv, Set](func(env evaluationEnv) (Dict, error) {
			return env.Traits, nil
		}),
	})
	require.NoError(f, err)
	for _, tc := range testCases {
		for _, expressions := range tc.expressions {
			for _, expression := range expressions {
				f.Add(expression)
			}
		}
	}
	f.Fuzz(func(t *testing.T, expression string) {
		expr, err := parser.Parse(expression)
		if err != nil {
			// Many/most fuzzed expressions won't parse, as long as we didn't
			// panic that's okay.
			return
		}
		// If the expression parsed, try to evaluate it, errors are okay just
		// make sure we don't panic.
		_, _ = expr.Evaluate(evaluationEnv{DictFromStringSliceMap(baseInputTraits)})
	})
}
