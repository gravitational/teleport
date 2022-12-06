package loginrule

import (
	"testing"

	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
)

func newLoginRule(name string, priority int32, traitsMap map[string][]string, expression string) *loginrulepb.LoginRule {
	rule := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name: name,
		},
		Priority:         priority,
		TraitsExpression: expression,
		TraitsMap:        make(map[string]*wrappers.StringValues),
	}
	for key, values := range traitsMap {
		rule.TraitsMap[key] = &wrappers.StringValues{
			Values: values,
		}
	}
	return rule
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	baseInputTraits := map[string][]string{
		"groups": []string{"devs", "security"},
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
				newLoginRule("rule0", 0, map[string][]string{
					"groups": []string{
						"external.groups",
						"admins",
					},
				}, ""),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"groups": []string{"devs", "security", "admins"},
			},
		},
		{
			desc: "simple traits expression",
			rules: []*loginrulepb.LoginRule{
				newLoginRule("rule0", 0, nil,
					`dict(
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
				newLoginRule("rule0", 0, map[string][]string{
					"groups": []string{
						"external.groups",
						"admins",
					},
				}, ""),
				newLoginRule("rule1", 1, nil,
					`dict(
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
				newLoginRule("rule", 0, map[string][]string{
					"last_rule": []string{"rule0"},
				}, ""),
				newLoginRule("rule", 2, map[string][]string{
					"last_rule": []string{"rule2"},
				}, ""),
				newLoginRule("rule", 1, map[string][]string{
					"last_rule": []string{"rule1"},
				}, ""),
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
				newLoginRule("rule0", 0, map[string][]string{
					"last_rule": []string{"rule0"},
				}, ""),
				newLoginRule("rule2", 0, map[string][]string{
					"last_rule": []string{"rule2"},
				}, ""),
				newLoginRule("rule1", 0, map[string][]string{
					"last_rule": []string{"rule1"},
				}, ""),
			},
			inputTraits: baseInputTraits,
			expectedTraits: map[string][]string{
				"last_rule": []string{"rule2"},
			},
		},
		{
			desc: "wrong map return type",
			rules: []*loginrulepb.LoginRule{
				newLoginRule("rule0", 0, map[string][]string{
					"groups": []string{"external"},
				}, ""),
			},
			errorContains: "traits_map expression must evaluate to type string or set, the following expression evaluates to loginrule.dict:",
		},
		{
			desc: "wrong expression return type",
			rules: []*loginrulepb.LoginRule{
				newLoginRule("rule0", 0, nil, "external.groups"),
			},
			errorContains: "traits_expression must evaluate to type dict, the following expression evaluates to loginrule.set:",
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
