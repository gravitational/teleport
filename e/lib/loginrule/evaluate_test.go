package loginrule

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	oss "github.com/gravitational/teleport/lib/loginrule"
)

func newLoginRuleWithTraitsMap(name string, priority int32, traitsMap map[string][]string) *loginrulepb.LoginRule {
	rule := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: name,
		},
		Version:   types.V1,
		Priority:  priority,
		TraitsMap: make(map[string]*wrappers.StringValues),
	}
	for key, values := range traitsMap {
		rule.TraitsMap[key] = &wrappers.StringValues{
			Values: values,
		}
	}
	return rule
}

func newLoginRuleWithTraitsExpression(name string, priority int32, expression string) *loginrulepb.LoginRule {
	return &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: name,
		},
		Version:          types.V1,
		Priority:         priority,
		TraitsExpression: expression,
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	baseInputTraits := map[string][]string{
		"groups":   []string{"devs", "security"},
		"username": []string{"alice"},
	}

	for _, tc := range []struct {
		desc           string
		rules          []*loginrulepb.LoginRule
		inputTraits    map[string][]string
		expectedTraits map[string][]string
		errorContains  []string
	}{
		{
			desc:           "no rules",
			inputTraits:    baseInputTraits,
			expectedTraits: baseInputTraits,
		},
		{
			desc: "simple traits map",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule0", 0, map[string][]string{
					"groups": []string{
						"external.groups",
						"admins",
					},
				}),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups": []string{"devs", "security", "admins"},
			},
		},
		{
			desc: "simple traits expression",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule0", 0, `dict(
					pair("groups", external.groups),
					pair("example", set("a", "b")),
				)`),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups":  []string{"devs", "security"},
				"example": []string{"a", "b"},
			},
		},
		{
			desc: "multiple rules",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule0", 0, map[string][]string{
					"groups": []string{
						"external.groups",
						"admins",
					},
				}),
				newLoginRuleWithTraitsExpression("rule1", 1, `dict(
					pair("groups", external.groups),
					pair("example", set("a", "b")),
				)`),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups":  []string{"devs", "security", "admins"},
				"example": []string{"a", "b"},
			},
		},
		{
			// Rules should be sorted by priority. The final rule to be
			// evaluated will set the output traits.
			desc: "priority sort",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"last_rule": []string{"rule0"},
				}),
				newLoginRuleWithTraitsMap("rule", 2, map[string][]string{
					"last_rule": []string{"rule2"},
				}),
				newLoginRuleWithTraitsMap("rule", 1, map[string][]string{
					"last_rule": []string{"rule1"},
				}),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"last_rule": []string{"rule2"},
			},
		},
		{
			// Rules with equal priority should be sorted by name. The final
			// rule to be evaluated will set the output traits.
			desc: "equal priority name sort",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule0", 0, map[string][]string{
					"last_rule": []string{"rule0"},
				}),
				newLoginRuleWithTraitsMap("rule2", 0, map[string][]string{
					"last_rule": []string{"rule2"},
				}),
				newLoginRuleWithTraitsMap("rule1", 0, map[string][]string{
					"last_rule": []string{"rule1"},
				}),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"last_rule": []string{"rule2"},
			},
		},
		{
			desc: "wrong map return type",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule0", 0, map[string][]string{
					"groups": []string{"external"},
				}),
			},
			errorContains: []string{
				"traits_map expression must evaluate to type string or set, the following expression evaluates to typical.Dict:",
			},
		},
		{
			desc: "wrong expression return type",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule0", 0, "external.groups"),
			},
			errorContains: []string{
				"traits_expression must evaluate to type dict, the following expression evaluates to typical.Set:",
			},
		},
		{
			desc: "ifelse",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"a": []string{
						`ifelse(true, "correct", "wrong")`,
						`ifelse(false, "wrong", "correct")`,
						`ifelse(ifelse(true, true, false), "correct", "wrong")`,
						`set(ifelse(true, "correct", "wrong"), "correct")`,
					},
					"groups": []string{
						`ifelse(true, external.groups, "wrong")`,
					},
				}),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"a":      []string{"correct"},
				"groups": baseInputTraits["groups"],
			},
		},
		{
			desc: "set methods",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"extragroups":            []string{`external.groups.add("extra", "surplus")`},
					"fewergroups":            []string{`external.groups.remove("security")`},
					"nogroups":               []string{`external.groups.remove("devs", "security").add("test").remove("test").remove("not-a-group")`},
					"groups-by-another-name": []string{`external.groups.remove("not-a-group")`},
					"logins": []string{
						// external.groups does not contain "admins", so we
						// expect to just get the username.
						`ifelse(external.groups.contains("admins"), external.username.add("root"), external.username)`,
						// external.groups does contain "security", so expect
						// the "security-team" login.
						`ifelse(external.groups.contains("security"), "security-team", set())`,
					},
				}),
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
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"groups": []string{`union(external.groups, set("test1", "test2"))`},
					"fruits": []string{`union(set("apple", "banana"), set("cherry"), set("dragonfruit", "eggplant"))`},
				}),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups": append([]string{"test1", "test2"}, baseInputTraits["groups"]...),
				"fruits": []string{"apple", "banana", "cherry", "dragonfruit", "eggplant"},
			},
		},
		{
			desc: "wrong set.add argument type",
			rules: []*loginrulepb.LoginRule{
				// Cannot add a set to a set - should use union.
				newLoginRuleWithTraitsExpression("rule", 0, `external.groups.add(external.username)`),
			},
			inputTraits: baseInputTraits,
			errorContains: []string{
				"parsing argument 2 to function (add)",
				"expected type string, got expression returning type (typical.Set)",
			},
		},
		{
			desc: "dict creation",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `dict(
					pair("x", external.a),
					pair("y", set("y")),
					pair("z", union(external.a, external.b)))`),
			},
			inputTraits: map[string][]string{
				"a": []string{"a"},
				"b": []string{"b"},
			},
			expectedTraits: map[string][]string{
				"x": []string{"a"},
				"y": []string{"y"},
				"z": []string{"a", "b"},
			},
		},
		{
			desc: "dict.add_values",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0,
					`external.add_values("a", "aa", "aaa").add_values("z", "z")`),
			},
			inputTraits: map[string][]string{
				"a": []string{"a"},
				"b": []string{"b"},
			},
			expectedTraits: map[string][]string{
				"a": []string{"a", "aa", "aaa"},
				"b": []string{"b"},
				"z": []string{"z"},
			},
		},
		{
			desc: "dict.put",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0,
					`external.put("a", set("aa", "aaa")).put("b", external.a).put("z", set("z"))`),
			},
			inputTraits: map[string][]string{
				"a": []string{"a"},
				"b": []string{"b"},
			},
			expectedTraits: map[string][]string{
				"a": []string{"aa", "aaa"},
				"b": []string{"a"},
				"z": []string{"z"},
			},
		},
		{
			desc: "dict.remove",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0,
					`external.remove("a", "b").remove("c").remove("z")`),
			},
			inputTraits: map[string][]string{
				"a": []string{"a"},
				"b": []string{"b"},
				"c": []string{"c"},
				"d": []string{"d"},
			},
			expectedTraits: map[string][]string{
				"d": []string{"d"},
			},
		},
		{
			desc: "string helpers",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"lower": []string{
						`strings.lower("APPLE")`,
						`strings.lower("BaNaNa")`,
						`strings.lower(set("cherry", "dragonFRUIT"))`,
						`strings.lower(external.username)`,
					},
					"upper": []string{
						`strings.upper("APPLE")`,
						`strings.upper("BaNaNa")`,
						`strings.upper(set("cherry", "dragonFRUIT"))`,
						`strings.upper(external.username)`,
					},
					"replaced": []string{
						`strings.replaceall("snake_case_example", "_", "-")`,
						`strings.replaceall(strings.replaceall("user@example.com", "@", "_"), ".", "-")`,
						`strings.replaceall(set("dev-team", "platform-team"), "-team", "")`,
					},
				}),
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
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"choose_first": []string{
						`choose(option(true, "first"), option(false, "second"))`,
					},
					"choose_second": []string{
						`choose(option(false, "first"), option(true, "second"))`,
					},
					"groups": []string{
						`choose(
							option(external.username.contains("alice"), set("devs", "security", "requester")),
							option(external.username.contains("bob"), set("security", "reviewer")),
							option(external.username.contains("charlie"), set("devs")),
							option(true, set()),
						)`,
					},
				}),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"choose_first":  []string{"first"},
				"choose_second": []string{"second"},
				"groups":        []string{"devs", "security", "requester"},
			},
		},
		{
			desc: "wrong choose argument type",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `choose(external.groups.contains("devs"), external)`),
			},
			errorContains: []string{
				"parsing argument 1 to function (choose)",
				"expected type typical.option, got expression returning type (bool)",
			},
		},
		{
			// Test that external traits dict can by indexed like
			// external["trait"] as well as external.trait (the latter syntax
			// does not support traits containing hyphens or some other special
			// characters).
			desc: "dict index",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"test":        {`external.test`},
					"with-hyphen": {`external["with-hyphen"]`},
				}),
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
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"groups": {
						`ifelse(external.groups.contains("security") || external.groups.contains("it"),
							external.groups.add("admins"),
							external.groups)`,
					},
				}),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups": {"devs", "security", "admins"},
			},
		},
		{
			desc: "traits_map quoted or unquoted strings",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"test": {`a`, `"b"`},
				}),
			},
			expectedTraits: map[string][]string{
				"test": {"a", "b"},
			},
		},
		{
			desc: "invalid function",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `replace(external, "groups", "roles")`),
			},
			errorContains: []string{
				"unsupported function: replace",
			},
		},
		{
			desc: "invalid method",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `external.replace("groups", "roles")`),
			},
			errorContains: []string{
				"unsupported function: external.replace",
			},
		},
		{
			desc: "invalid namespace",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `internal.groups`),
			},
			errorContains: []string{
				`unknown identifier: "internal.groups"`,
			},
		},
		{
			desc: "unclosed parens",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `external.put("logins", set("operator")`),
			},
			errorContains: []string{
				// The more specific error is "missing ',' before newline in
				// argument list" but that comes from vulcand/predicate and it's
				// not optimal so I don't want to assert it in a test.
				`error parsing expression`,
			},
		},
		{
			desc: "email.local",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `dict(pair("logins", email.local(external.emails)))`),
			},
			inputTraits: map[string][]string{
				"emails": {"Alice <alice@example.com>", "bob@example.com"},
			},
			expectedTraits: map[string][]string{
				"logins": {"alice", "bob"},
			},
		},
		{
			desc: "bad email",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `dict(pair("logins", email.local(external.emails)))`),
			},
			inputTraits: map[string][]string{
				"emails": {"charlie"},
			},
			errorContains: []string{
				`failed to parse "email.local" argument "charlie":`,
			},
		},
		{
			desc: "regexp.replace",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule", 0, `dict(
					pair("test1", regexp.replace(external.test1, "foo-(.*)", "$1")),
					pair("test2", regexp.replace(external.test2, "foo-(?P<suffix>.*)", "$suffix")),
					pair("test3", regexp.replace(external.test3, "foo-(.*)-(.*)", "$1.$2")),
				)`),
			},
			inputTraits: map[string][]string{
				"test1": {"foo-bar", "foo-baz"},
				"test2": {"foo-bar", "foo-baz"},
				"test3": {"foo-bar-baz", "not-matching"},
			},
			expectedTraits: map[string][]string{
				"test1": {"bar", "baz"},
				"test2": {"bar", "baz"},
				"test3": {"bar.baz"},
			},
		},
		{
			desc: "bad regexp",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
					"logins": {`regexp.replace(external.email, "(.*@example.com", "$1")`},
				}),
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
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsMap("rule", 0,
					map[string][]string{
						"logins": {`strings.split(external.commaLogins, ",")`},
						"localEmails": {
							`email.local(strings.split(external.oneSpaceEmails, " "))`,
							`email.local(strings.split(external.twoSpaceEmails, "  "))`,
							`email.local(strings.split(external.singleEmail, ","))`,
						},
					},
				),
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
	} {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := Evaluate(tc.rules, &oss.EvaluationInput{Traits: tc.inputTraits})
			if len(tc.errorContains) > 0 {
				for _, contains := range tc.errorContains {
					require.ErrorContains(t, err, contains, "error string does not contain expected snippet")
				}
				return
			}
			require.NoError(t, err, trace.DebugReport(err))

			var ruleNames []string
			for _, rule := range tc.rules {
				ruleNames = append(ruleNames, rule.Metadata.Name)
			}

			require.Empty(t, cmp.Diff(&oss.EvaluationOutput{
				Traits:       tc.expectedTraits,
				AppliedRules: ruleNames,
			}, result, cmpopts.SortSlices(func(a, b string) bool { return a < b })))
		})
	}
}
