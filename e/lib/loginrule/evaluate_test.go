package loginrule

import (
	"testing"

	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
)

func newLoginRuleWithTraitsMap(name string, priority int32, traitsMap map[string][]string) *loginrulepb.LoginRule {
	rule := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: name,
		},
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
		errorContains  string
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
			errorContains: "traits_map expression must evaluate to type string or set, the following expression evaluates to loginrule.dict:",
		},
		{
			desc: "wrong expression return type",
			rules: []*loginrulepb.LoginRule{
				newLoginRuleWithTraitsExpression("rule0", 0, "external.groups"),
			},
			errorContains: "traits_expression must evaluate to type dict, the following expression evaluates to loginrule.set:",
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
			inputTraits:   baseInputTraits,
			errorContains: "arguments to set.add must have type string, got loginrule.set",
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
			errorContains: "arguments to choose must have type option, got bool",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := Evaluate(tc.rules, &EvaluationInput{Traits: tc.inputTraits})
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains)
				return
			}
			require.NoError(t, err)
			require.Len(t, result.Traits, len(tc.expectedTraits), "length of output traits does not match length of expected traits")
			for key, values := range tc.expectedTraits {
				require.Contains(t, result.Traits, key, "output traits does not contain expected key")
				require.ElementsMatch(t, values, result.Traits[key], "values for output traits at key %s do not match the expected", key)
			}
		})
	}
}
