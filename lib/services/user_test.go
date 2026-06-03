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

package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
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

type oidcInput struct {
	comment       string
	claims        map[string]any
	expectedRoles []string
	warnings      []string
}

var oidcTestCases = []struct {
	comment  string
	mappings []types.ClaimMapping
	inputs   []oidcInput
}{
	{
		comment: "no mappings",
		inputs: []oidcInput{
			{
				comment:       "no match",
				claims:        map[string]any{"a": "b"},
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
		inputs: []oidcInput{
			{
				comment:       "no match",
				claims:        map[string]any{"a": "b"},
				expectedRoles: nil,
			},
			{
				comment:       "no value match",
				claims:        map[string]any{"role": "b"},
				expectedRoles: nil,
			},
			{
				comment:       "direct admin value match",
				claims:        map[string]any{"role": "admin"},
				expectedRoles: []string{"admin", "bob"},
			},
			{
				comment:       "direct user value match",
				claims:        map[string]any{"role": "user"},
				expectedRoles: []string{"user"},
			},
			{
				comment:       "direct user value match with array",
				claims:        map[string]any{"role": []string{"user"}},
				expectedRoles: []string{"user"},
			},
		},
	},
	{
		comment: "regexp mappings match",
		mappings: []types.ClaimMapping{
			{Claim: "role", Value: "^admin-(.*)$", Roles: []string{"role-$1", "bob"}},
		},
		inputs: []oidcInput{
			{
				comment:       "no match",
				claims:        map[string]any{"a": "b"},
				expectedRoles: nil,
			},
			{
				comment:       "no match - subprefix",
				claims:        map[string]any{"role": "adminz"},
				expectedRoles: nil,
			},
			{
				comment:       "value with capture match",
				claims:        map[string]any{"role": "admin-hello"},
				expectedRoles: []string{"role-hello", "bob"},
			},
			{
				comment:       "multiple value with capture match, deduplication",
				claims:        map[string]any{"role": []string{"admin-hello", "admin-ola"}},
				expectedRoles: []string{"role-hello", "bob", "role-ola"},
			},
			{
				comment:       "first matches, second does not",
				claims:        map[string]any{"role": []string{"hello", "admin-ola"}},
				expectedRoles: []string{"role-ola", "bob"},
			},
		},
	},
	{
		comment: "regexp compilation",
		mappings: []types.ClaimMapping{
			{Claim: "role", Value: `^admin-(?!)$`, Roles: []string{"admin"}}, // "?!" is invalid.
			{Claim: "role", Value: "^admin-(.*)$", Roles: []string{"role-$1", "bob"}},
			{Claim: "role", Value: `^admin2-(?!)$`, Roles: []string{"admin2"}}, // "?!" is invalid.
		},
		inputs: []oidcInput{
			{
				comment:       "invalid regexp",
				claims:        map[string]any{"role": []string{"admin-hello", "dev"}},
				expectedRoles: []string{"role-hello", "bob"},
				warnings: []string{
					`case-insensitive expression "^admin-(?!)$" is not a valid regexp`,
					`case-insensitive expression "^admin2-(?!)$" is not a valid regexp`,
				},
			},
			{
				comment:       "regexp are not compiled if not needed",
				claims:        map[string]any{},
				expectedRoles: nil,
				// if the regexp were compiled, we would have the same warnings as above
				warnings: nil,
			},
		},
	},
	{
		comment: "empty expands are skipped",
		mappings: []types.ClaimMapping{
			{Claim: "role", Value: "^admin-(.*)$", Roles: []string{"$2", "bob"}},
		},
		inputs: []oidcInput{
			{
				comment:       "value with capture match",
				claims:        map[string]any{"role": "admin-hello"},
				expectedRoles: []string{"bob"},
			},
		},
	},
	{
		comment: "glob wildcard match",
		mappings: []types.ClaimMapping{
			{Claim: "role", Value: "*", Roles: []string{"admin"}},
		},
		inputs: []oidcInput{
			{
				comment:       "empty value match",
				claims:        map[string]any{"role": ""},
				expectedRoles: []string{"admin"},
			},
			{
				comment:       "any value match",
				claims:        map[string]any{"role": "zz"},
				expectedRoles: []string{"admin"},
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
		inputs: []oidcInput{
			{
				comment: "Matches multiple groups",
				claims: map[string]any{
					"groups": []string{"DemoCorp - Backend Engineers", "DemoCorp Infrastructure"},
				},
				expectedRoles: []string{"backend", "approver"},
			},
			{
				comment: "Matches one group",
				claims: map[string]any{
					"groups": []string{"DemoCorp - SRE"},
				},
				expectedRoles: []string{"approver"},
			},
			{
				comment: "Matches one group with multiple roles",
				claims: map[string]any{
					"groups": []string{"DemoCorp Infrastructure"},
				},
				expectedRoles: []string{"approver", "backend"},
			},
			{
				comment: "No match only due to case-sensitivity",
				claims: map[string]any{
					"groups": []string{"Democorp - SRE"},
				},
				expectedRoles: []string(nil),
				warnings: []string{
					`trait "Democorp - SRE" matches value "DemoCorp - SRE" case-insensitively and would have yielded "approver" role`,
				},
			},
		},
	},
}

func TestOIDCMapping(t *testing.T) {
	t.Parallel()

	for i, testCase := range oidcTestCases {
		conn := types.OIDCConnectorV3{
			Spec: types.OIDCConnectorSpecV3{
				ClaimsToRoles: testCase.mappings,
			},
		}
		for _, input := range testCase.inputs {
			comment := fmt.Sprintf("OIDC Test case %v %q, input %q", i, testCase.comment, input.comment)
			_, outRoles := TraitsToRoles(conn.GetTraitMappings(), oidcClaimsToTraits(input.claims))
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
func BenchmarkTraitToRoles(b *testing.B) {
	for _, testCase := range oidcTestCases {
		samlConn := types.SAMLConnectorV2{
			Spec: types.SAMLConnectorSpecV2{
				AttributesToRoles: claimMappingsToAttributeMappings(testCase.mappings),
			},
		}
		mappings := samlConn.GetTraitMappings()
		for _, input := range testCase.inputs {
			testCaseInputName := fmt.Sprintf("%s %s", testCase.comment, input.comment)
			traits := SAMLAssertionsToTraits(claimsToAttributes(input.claims))

			b.Run(testCaseInputName, func(b *testing.B) {
				for b.Loop() {
					TraitsToRoles(mappings, traits)
				}
			})
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
			Roles: slices.Clone(m.Roles),
		})
	}
	return out
}

// oidcClaimsToTraits converts OIDC-style claims into teleport-specific trait format
func oidcClaimsToTraits(claims map[string]any) map[string][]string {
	traits := make(map[string][]string)

	for claimName, v := range claims {

		switch claimValue := v.(type) {
		case string:
			traits[claimName] = []string{claimValue}
		case []string:
			traits[claimName] = claimValue
		case []any:
			for _, vv := range claimValue {
				traits[claimName] = append(traits[claimName], vv.(string))
			}
		}
	}

	return traits
}

// claimsToAttributes maps map[string]any type to attributes for testing
func claimsToAttributes(claims map[string]any) saml2.AssertionInfo {
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

func TestUsernameForCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		username         string
		originCluster    string
		localClusterName string
		expected         string
	}{

		{
			username:         "alice",
			originCluster:    "leaf",
			localClusterName: "root",
			expected:         "remote-alice-leaf",
		},
		{
			username:         "bob",
			originCluster:    "",
			localClusterName: "root",
			expected:         "bob",
		},
		{
			username:         "carol",
			originCluster:    "leaf.cluster",
			localClusterName: "root.cluster",
			expected:         "remote-carol-leaf.cluster",
		},
		{
			username:         "dave",
			originCluster:    "leaf-cluster",
			localClusterName: "leaf-cluster",
			expected:         "dave",
		},
	}

	for _, test := range tests {
		t.Run(test.username, func(t *testing.T) {
			result := UsernameForCluster(
				UsernameForClusterConfig{
					User:              test.username,
					OriginClusterName: test.originCluster,
					LocalClusterName:  test.localClusterName,
				},
			)
			require.Equal(t, test.expected, result)
		})
	}
}

// fakeUserGetter is a UserGetter backed by an in-memory map. It records lookup
// counts per username so tests can assert the dedupe contract.
type fakeUserGetter struct {
	users   map[string]types.User
	failFor map[string]error // username -> error returned instead of a lookup

	calls map[string]int // username -> GetUser call count
}

func (f *fakeUserGetter) GetUser(_ context.Context, name string, _ bool) (types.User, error) {
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[name]++
	if err, ok := f.failFor[name]; ok {
		return nil, err
	}
	user, ok := f.users[name]
	if !ok {
		return nil, trace.NotFound("user %q does not exist", name)
	}
	return user, nil
}

func newUserWithTraits(t *testing.T, name string, traits map[string][]string) types.User {
	t.Helper()
	user, err := types.NewUser(name)
	require.NoError(t, err)
	if traits != nil {
		user.SetTraits(traits)
	}
	return user
}

func TestResolveUserDisplays(t *testing.T) {
	t.Parallel()

	t.Run("dedupes and issues one lookup per unique username", func(t *testing.T) {
		t.Parallel()
		getter := &fakeUserGetter{users: map[string]types.User{
			"alice": newUserWithTraits(t, "alice", nil),
			"bob":   newUserWithTraits(t, "bob", nil),
		}}

		out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice", "alice", "bob", "alice"})
		require.NoError(t, err)
		require.Len(t, out, 2)
		require.Contains(t, out, "alice")
		require.Contains(t, out, "bob")
		require.Equal(t, 1, getter.calls["alice"])
		require.Equal(t, 1, getter.calls["bob"])
	})

	t.Run("returns the display value for a found user", func(t *testing.T) {
		t.Parallel()
		alice := newUserWithTraits(t, "alice", map[string][]string{
			"displayName": {"Alice Liddell"},
			"email":       {"alice@example.com"},
		})

		want := alice.GetDisplay()
		// Sanity-check that the chosen traits actually produce a display, so the
		// assertion below is meaningful.
		require.NotEqual(t, types.UserDisplay{}, want)

		getter := &fakeUserGetter{users: map[string]types.User{"alice": alice}}
		out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice"})
		require.NoError(t, err)
		require.Equal(t, want, out["alice"])
	})

	t.Run("found user with no display is present with a zero value", func(t *testing.T) {
		t.Parallel()
		getter := &fakeUserGetter{users: map[string]types.User{
			"plain": newUserWithTraits(t, "plain", nil),
		}}

		out, err := ResolveUserDisplays(context.Background(), getter, []string{"plain"})
		require.NoError(t, err)

		// present with the zero value, not missing from the map
		require.Contains(t, out, "plain")
		require.Equal(t, types.UserDisplay{}, out["plain"])
	})

	t.Run("missing users are absent and do not fail resolution", func(t *testing.T) {
		t.Parallel()
		getter := &fakeUserGetter{users: map[string]types.User{
			"alice": newUserWithTraits(t, "alice", nil),
			"bob":   newUserWithTraits(t, "bob", nil),
		}}

		out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice", "ghost", "bob"})
		require.NoError(t, err)
		require.Len(t, out, 2)
		require.Contains(t, out, "alice")
		require.Contains(t, out, "bob")
		require.NotContains(t, out, "ghost")
	})

	t.Run("aborts on non-NotFound errors without a partial map", func(t *testing.T) {
		t.Parallel()
		for _, errorCase := range []struct {
			name string
			err  error
		}{
			{"transient backend error", errors.New("backend timeout")},
			// A canceled/expired context surfaces through the getter as a
			// non-NotFound error and must abort like any other.
			{"context cancellation", context.Canceled},
		} {
			t.Run(errorCase.name, func(t *testing.T) {
				t.Parallel()
				getter := &fakeUserGetter{
					users:   map[string]types.User{"alice": newUserWithTraits(t, "alice", nil)},
					failFor: map[string]error{"bob": errorCase.err},
				}

				out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice", "bob", "carol"})
				require.Error(t, err)
				require.ErrorIs(t, err, errorCase.err)  // original error preserved
				require.Contains(t, err.Error(), "bob") // names the error user
				require.Nil(t, out)                     // no partial map handed back
			})
		}
	})
}
