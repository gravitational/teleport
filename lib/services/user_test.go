/*
Copyright 2015-2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/coreos/go-oidc/jose"
	"github.com/google/go-cmp/cmp"
	saml2 "github.com/russellhaering/gosaml2"
	samltypes "github.com/russellhaering/gosaml2/types"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestTraits(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		traitName string
	}{
		// Windows trait names are URLs.
		{
			traitName: "http://schemas.microsoft.com/ws/2008/06/identity/claims/windowsaccountname",
		},
		// Simple strings are the most common trait names.
		{
			traitName: "user-groups",
		},
	}

	for _, tt := range tests {
		user := &types.UserV2{
			Kind:    types.KindUser,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      "foo",
				Namespace: apidefaults.Namespace,
			},
			Spec: types.UserSpecV2{
				Traits: map[string][]string{
					tt.traitName: {"foo"},
				},
			},
		}

		data, err := json.Marshal(user)
		require.NoError(t, err)

		_, err = UnmarshalUser(data)
		require.NoError(t, err)
	}
}

func TestOIDCMapping(t *testing.T) {
	t.Parallel()

	type input struct {
		comment       string
		claims        jose.Claims
		expectedRoles []string
		warnings      []string
	}
	testCases := []struct {
		comment  string
		mappings []types.ClaimMapping
		inputs   []input
	}{
		{
			comment: "no mappings",
			inputs: []input{
				{
					claims:        jose.Claims{"a": "b"},
					expectedRoles: nil,
				},
			},
		},
		{
			comment: "simple mappings",
			mappings: []types.ClaimMapping{
				{Claim: "role", Value: "admin", Roles: []string{"admin", "bob"}},
				{Claim: "role", Value: "user", Roles: []string{"user"}},
			},
			inputs: []input{
				{
					comment:       "no match",
					claims:        jose.Claims{"a": "b"},
					expectedRoles: nil,
				},
				{
					comment:       "no value match",
					claims:        jose.Claims{"role": "b"},
					expectedRoles: nil,
					warnings: []string{
						`1 trait value(s) did not match expression "admin": ["b"]`,
						`1 trait value(s) did not match expression "user": ["b"]`,
					},
				},
				{
					comment:       "direct admin value match",
					claims:        jose.Claims{"role": "admin"},
					expectedRoles: []string{"admin", "bob"},
					warnings: []string{
						`1 trait value(s) did not match expression "user": ["admin"]`,
					},
				},
				{
					comment:       "direct user value match",
					claims:        jose.Claims{"role": "user"},
					expectedRoles: []string{"user"},
					warnings: []string{
						`1 trait value(s) did not match expression "admin": ["user"]`,
					},
				},
				{
					comment:       "direct user value match with array",
					claims:        jose.Claims{"role": []string{"user"}},
					expectedRoles: []string{"user"},
					warnings: []string{
						`1 trait value(s) did not match expression "admin": ["user"]`,
					},
				},
			},
		},
		{
			comment: "regexp mappings match",
			mappings: []types.ClaimMapping{
				{Claim: "role", Value: "^admin-(.*)$", Roles: []string{"role-$1", "bob"}},
			},
			inputs: []input{
				{
					comment:       "no match",
					claims:        jose.Claims{"a": "b"},
					expectedRoles: nil,
				},
				{
					comment:       "no match - subprefix",
					claims:        jose.Claims{"role": "adminz"},
					expectedRoles: nil,
					warnings: []string{
						`1 trait value(s) did not match expression "^admin-(.*)$": ["adminz"]`,
					},
				},
				{
					comment:       "value with capture match",
					claims:        jose.Claims{"role": "admin-hello"},
					expectedRoles: []string{"role-hello", "bob"},
				},
				{
					comment:       "multiple value with capture match, deduplication",
					claims:        jose.Claims{"role": []string{"admin-hello", "admin-ola"}},
					expectedRoles: []string{"role-hello", "bob", "role-ola"},
				},
				{
					comment:       "first matches, second does not",
					claims:        jose.Claims{"role": []string{"hello", "admin-ola"}},
					expectedRoles: []string{"role-ola", "bob"},
					warnings: []string{
						`1 trait value(s) did not match expression "^admin-(.*)$": ["hello"]`,
					},
				},
				{
					comment:       "no match with array in warning",
					claims:        jose.Claims{"role": []string{"foo", "bar", "baz"}},
					expectedRoles: nil,
					warnings: []string{
						`3 trait value(s) did not match expression "^admin-(.*)$": ["foo" "bar" "baz"]`,
					},
				},
				{
					comment:       "no match with big array trimmed in warning",
					claims:        jose.Claims{"role": []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9", "a10", "a11", "a12", "a13", "a14", "a15", "a16", "a17", "a18", "a19", "a20", "a21", "a22", "a23", "a24", "a25", "a26", "a27", "a28", "a29", "a30", "a31", "a32", "a33", "a34", "a35", "a36", "a37", "a38", "a39", "a40", "a41", "a42", "a43", "a44", "a45", "a46", "a47", "a48", "a49", "a50", "a51", "a52", "a53", "a54", "a55", "a56", "a57", "a58", "a59", "a60", "a61", "a62", "a63", "a64", "a65", "a66", "a67", "a68", "a69", "a70", "a71", "a72", "a73", "a74", "a75", "a76", "a77", "a78", "a79", "a80", "a81", "a82", "a83", "a84", "a85", "a86", "a87", "a88", "a89", "a90", "a91", "a92", "a93", "a94", "a95", "a96", "a97", "a98", "a99", "a100", "a101"}},
					expectedRoles: nil,
					warnings: []string{
						`101 trait value(s) did not match expression "^admin-(.*)$": ["a1" "a2" "a3" "a4" "a5" "a6" "a7" "a8" "a9" "a10" "a11" "a12" "a13" "a14" "a15" "a16" "a17" "a18" "a19" "a20" "a21" "a22" "a23" "a24" "a25" "a26" "a27" "a28" "a29" "a30" "a31" "a32" "a33" "a34" "a35" "a36" "a37" "a38" "a39" "a40" "a41" "a42" "a43" "a44" "a45" "a46" "a47" "a48" "a49" "a50" "a51" "a52" "a53" "a54" "a55" "a56" "a57" "a58" "a59" "a60" "a61" "a62" "a63" "a64" "a65" "a66" "a67" "a68" "a69" "a70" "a71" "a72" "a73" "a74" "a75" "a76" "a77" "a78" "a79" "a80" "a81" "a82" "a83" "a84" "a85" "a86" "a87" "a88" "a89" "a90" "a91" "a92" "a93" "a94" "a95" "a96" "a97" "a98" "a99" "a100"] (first 100 values)`,
					},
				},
			},
		},
		{
			comment: "empty expands are skipped",
			mappings: []types.ClaimMapping{
				{Claim: "role", Value: "^admin-(.*)$", Roles: []string{"$2", "bob"}},
			},
			inputs: []input{
				{
					comment:       "value with capture match",
					claims:        jose.Claims{"role": "admin-hello"},
					expectedRoles: []string{"bob"},
				},
			},
		},
		{
			comment: "glob wildcard match",
			mappings: []types.ClaimMapping{
				{Claim: "role", Value: "*", Roles: []string{"admin"}},
			},
			inputs: []input{
				{
					comment:       "empty value match",
					claims:        jose.Claims{"role": ""},
					expectedRoles: []string{"admin"},
				},
				{
					comment:       "any value match",
					claims:        jose.Claims{"role": "zz"},
					expectedRoles: []string{"admin"},
				},
			},
		},
		{
			comment: "invalid regexp",
			mappings: []types.ClaimMapping{
				{Claim: "role", Value: `^admin-(?!)$`, Roles: []string{"admin"}},
			},
			inputs: []input{
				{
					comment:       "invalid regexp",
					claims:        jose.Claims{},
					expectedRoles: nil,
					warnings: []string{
						`case-insensitive expression "^admin-(?!)$" is not a valid regexp`,
					},
				},
			},
		},
		{
			comment: "Whitespace/dashes",
			mappings: []types.ClaimMapping{
				{Claim: "groups", Value: "DemoCorp - Backend Engineers", Roles: []string{"backend"}},
				{Claim: "groups", Value: "DemoCorp - SRE Managers", Roles: []string{"approver"}},
				{Claim: "groups", Value: "DemoCorp - SRE", Roles: []string{"approver"}},
				{Claim: "groups", Value: "DemoCorp Infrastructure", Roles: []string{"approver", "backend"}},
			},
			inputs: []input{
				{
					comment: "Matches multiple groups",
					claims: jose.Claims{
						"groups": []string{"DemoCorp - Backend Engineers", "DemoCorp Infrastructure"},
					},
					expectedRoles: []string{"backend", "approver"},
					warnings: []string{
						`1 trait value(s) did not match expression "DemoCorp - Backend Engineers": ["DemoCorp Infrastructure"]`,
						`2 trait value(s) did not match expression "DemoCorp - SRE Managers": ["DemoCorp - Backend Engineers" "DemoCorp Infrastructure"]`,
						`2 trait value(s) did not match expression "DemoCorp - SRE": ["DemoCorp - Backend Engineers" "DemoCorp Infrastructure"]`,
						`1 trait value(s) did not match expression "DemoCorp Infrastructure": ["DemoCorp - Backend Engineers"]`,
					},
				},
				{
					comment: "Matches one group",
					claims: jose.Claims{
						"groups": []string{"DemoCorp - SRE"},
					},
					expectedRoles: []string{"approver"},
					warnings: []string{

						`1 trait value(s) did not match expression "DemoCorp - Backend Engineers": ["DemoCorp - SRE"]`,
						`1 trait value(s) did not match expression "DemoCorp - SRE Managers": ["DemoCorp - SRE"]`,
						`1 trait value(s) did not match expression "DemoCorp Infrastructure": ["DemoCorp - SRE"]`,
					},
				},
				{
					comment: "Matches one group with multiple roles",
					claims: jose.Claims{
						"groups": []string{"DemoCorp Infrastructure"},
					},
					expectedRoles: []string{"approver", "backend"},
					warnings: []string{
						`1 trait value(s) did not match expression "DemoCorp - Backend Engineers": ["DemoCorp Infrastructure"]`,
						`1 trait value(s) did not match expression "DemoCorp - SRE Managers": ["DemoCorp Infrastructure"]`,
						`1 trait value(s) did not match expression "DemoCorp - SRE": ["DemoCorp Infrastructure"]`,
					},
				},
				{
					comment: "No match only due to case-sensitivity",
					claims: jose.Claims{
						"groups": []string{"Democorp - SRE"},
					},
					expectedRoles: []string(nil),
					warnings: []string{
						`1 trait value(s) did not match expression "DemoCorp - Backend Engineers": ["Democorp - SRE"]`,
						`1 trait value(s) did not match expression "DemoCorp - SRE Managers": ["Democorp - SRE"]`,
						`trait "Democorp - SRE" matches value "DemoCorp - SRE" case-insensitively and would have yielded "approver" role`,
						`1 trait value(s) did not match expression "DemoCorp Infrastructure": ["Democorp - SRE"]`,
					},
				},
			},
		},
	}

	for i, testCase := range testCases {
		conn := types.OIDCConnectorV3{
			Spec: types.OIDCConnectorSpecV3{
				ClaimsToRoles: testCase.mappings,
			},
		}
		for _, input := range testCase.inputs {
			comment := fmt.Sprintf("OIDC Test case %v %q, input %q", i, testCase.comment, input.comment)
			_, outRoles := TraitsToRoles(conn.GetTraitMappings(), OIDCClaimsToTraits(input.claims))
			require.Empty(t, cmp.Diff(outRoles, input.expectedRoles), comment)
		}

		samlConn := types.SAMLConnectorV2{
			Spec: types.SAMLConnectorSpecV2{
				AttributesToRoles: claimMappingsToAttributeMappings(testCase.mappings),
			},
		}
		for _, input := range testCase.inputs {
			comment := fmt.Sprintf("SAML Test case %v %v, input %#v", i, testCase.comment, input)
			warnings, outRoles := TraitsToRoles(samlConn.GetTraitMappings(), SAMLAssertionsToTraits(claimsToAttributes(input.claims)))
			require.Empty(t, cmp.Diff(outRoles, input.expectedRoles), comment)
			require.Empty(t, cmp.Diff(warnings, input.warnings), comment)
		}
	}
}

// claimMappingsToAttributeMappings converts oidc claim mappings to
// attribute mappings, used in tests
func claimMappingsToAttributeMappings(in []types.ClaimMapping) []types.AttributeMapping {
	var out []types.AttributeMapping
	for _, m := range in {
		out = append(out, types.AttributeMapping{
			Name:  m.Claim,
			Value: m.Value,
			Roles: append([]string{}, m.Roles...),
		})
	}
	return out
}

// claimsToAttributes maps jose.Claims type to attributes for testing
func claimsToAttributes(claims jose.Claims) saml2.AssertionInfo {
	info := saml2.AssertionInfo{
		Values: make(map[string]samltypes.Attribute),
	}
	for claim, values := range claims {
		attr := samltypes.Attribute{
			Name: claim,
		}
		switch val := values.(type) {
		case string:
			attr.Values = []samltypes.AttributeValue{{Value: val}}
		case []string:
			for _, v := range val {
				attr.Values = append(attr.Values, samltypes.AttributeValue{Value: v})
			}
		default:
			panic(fmt.Sprintf("unsupported type %T", val))
		}
		info.Values[claim] = attr
	}
	return info
}
