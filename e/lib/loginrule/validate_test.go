package loginrule

import (
	"testing"

	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc          string
		rule          *loginrulepb.LoginRule
		errorContains string
	}{
		{
			desc: "valid",
			rule: loginrulepb.LoginRule_builder{
				Metadata: &types.Metadata{
					Name: "expressionless_rule",
				},
				Version:          "v1",
				TraitsExpression: `external`,
			}.Build(),
		},
		{
			desc: "no metadata",
			rule: loginrulepb.LoginRule_builder{
				Version:          "v1",
				TraitsExpression: `external`,
			}.Build(),
			errorContains: "must contain metadata",
		},
		{
			desc: "no name",
			rule: loginrulepb.LoginRule_builder{
				Metadata: &types.Metadata{
					Name: "",
				},
				Version:          "v1",
				TraitsExpression: `external`,
			}.Build(),
			errorContains: "must have non-empty metadata.name",
		},
		{
			desc: "no expressions",
			rule: loginrulepb.LoginRule_builder{
				Metadata: &types.Metadata{
					Name: "expressionless_rule",
				},
				Version: "v1",
			}.Build(),
			errorContains: "both traits_map and traits_expression are empty",
		},
		{
			desc: "too many expressions",
			rule: loginrulepb.LoginRule_builder{
				Metadata: &types.Metadata{
					Name: "rule",
				},
				Version:          "v1",
				TraitsExpression: "external",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			}.Build(),
			errorContains: "both traits_map and traits_expression are non-empty",
		},
		{
			desc:          "unparseable traits_expression",
			rule:          newLoginRuleWithTraitsExpression("rule", 0, `)`),
			errorContains: `failed to parse expression ")"`,
		},
		{
			desc: "unparseable traits_map expression",
			rule: newLoginRuleWithTraitsMap("rule", 0, map[string][]string{
				"groups": []string{")"},
			}),
			errorContains: `failed to parse expression ")" for trait "groups"`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			err := Validate(tc.rule)
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "error does not contain expected message")
				return
			}
			require.NoError(t, err, "unexpected error")
		})
	}
}
