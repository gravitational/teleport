/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package services_test

import (
	"testing"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// TestMarshalingProvisionTokenKube tests validation cases specific to kubernetes provision tokens.
func TestMarshalingProvisionTokenKube(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		token            types.ProvisionToken
		wantMarshalErr   bool
		wantUnmarshalErr bool
	}{
		{
			name: "valid kube token",
			token: &types.ProvisionTokenV2{
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "kube-token",
				},
				Spec: types.ProvisionTokenSpecV2{
					Roles:      []types.SystemRole{types.RoleNode},
					JoinMethod: types.JoinMethodKubernetes,
					Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
						Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
							{
								ServiceAccount: "namespace:service_account",
							},
						},
					},
				},
			},
			wantMarshalErr:   false,
			wantUnmarshalErr: false,
		},
		{
			name: "too many parts in service account name",
			token: &types.ProvisionTokenV2{
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "kube-token",
				},
				Spec: types.ProvisionTokenSpecV2{
					Roles:      []types.SystemRole{types.RoleNode},
					JoinMethod: types.JoinMethodKubernetes,
					Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
						Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
							{
								ServiceAccount: "too:many:parts",
							},
						},
					},
				},
			},
			wantMarshalErr:   true,
			wantUnmarshalErr: true,
		},
		{
			name: "missing account name",
			token: &types.ProvisionTokenV2{
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "kube-token",
				},
				Spec: types.ProvisionTokenSpecV2{
					Roles:      []types.SystemRole{types.RoleNode},
					JoinMethod: types.JoinMethodKubernetes,
					Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
						Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
							{
								ServiceAccount: "namespace:",
							},
						},
					},
				},
			},
			// this is a newer validation, so we should expect unmarshaling to succeed in order
			// to prevent existing tokens that are now malformed from causing issues
			wantUnmarshalErr: false,
			wantMarshalErr:   true,
		},
		{
			name: "missing namespace",
			token: &types.ProvisionTokenV2{
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "kube-token",
				},
				Spec: types.ProvisionTokenSpecV2{
					Roles:      []types.SystemRole{types.RoleNode},
					JoinMethod: types.JoinMethodKubernetes,
					Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
						Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
							{
								ServiceAccount: ":service_account",
							},
						},
					},
				},
			},
			// this is a newer validation, so we should expect unmarshaling to succeed in order
			// to prevent existing tokens that are now malformed from causing issues
			wantUnmarshalErr: false,
			wantMarshalErr:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// since there are test cases where MarshalProvisionToken would fail but UnmarshalProvisionToken
			// would not, we use utils.FastMarshal directly to ensure we can always properly assert for both
			// paths
			data, err := utils.FastMarshal(c.token)
			require.NoError(t, err)

			_, err = services.UnmarshalProvisionToken(data)
			if c.wantUnmarshalErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			_, err = services.MarshalProvisionToken(c.token)
			if c.wantMarshalErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultsAppliedDuringMarshal(t *testing.T) {
	t.Parallel()

	token := &types.ProvisionTokenV2{
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "kube-token",
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleNode},
			JoinMethod: types.JoinMethodKubernetes,
			Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
				// a default value of "in_cluster" should be applied during
				// marshaling when type is left unspecified
				Type: types.KubernetesJoinTypeUnspecified,
				Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
					{
						ServiceAccount: "namespace:service_account",
					},
				},
			},
		},
	}

	marshaled, err := services.MarshalProvisionToken(token)
	require.NoError(t, err)

	unmarshaled, err := services.UnmarshalProvisionToken(marshaled)
	require.NoError(t, err)

	require.Equal(t, types.KubernetesJoinTypeInCluster, unmarshaled.GetKubernetes().Type)
}

// TestGenericOIDCStructSurvivesRoundTrip ensures `MustMatchFields` (a Struct
// type) roundtrips correctly, i.e. it's custom marshal/unmarshal is executed
// since the standard jsoniter implementation can't handle struct fields.
func TestGenericOIDCStructSurvivesRoundTrip(t *testing.T) {
	t.Parallel()

	token := &types.ProvisionTokenV2{
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "generic-oidc-token",
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleNode},
			JoinMethod: types.JoinMethodGenericOIDC,
			GenericOIDC: &types.ProvisionTokenSpecV2GenericOIDC{
				Issuer:   "https://example.com",
				Audience: "example.teleport.sh/generic-oidc-token",
				MustMatchFields: types.NewStructFromGogoValues(map[string]*gogotypes.Value{
					"example": {Kind: &gogotypes.Value_StringValue{
						StringValue: "foo",
					}},
					"nested": {
						Kind: &gogotypes.Value_StructValue{
							StructValue: &gogotypes.Struct{Fields: map[string]*gogotypes.Value{
								"number": {Kind: &gogotypes.Value_NumberValue{
									NumberValue: 123.456,
								}},
								"string": {Kind: &gogotypes.Value_StringValue{
									StringValue: "string value",
								}},
								"bool": {Kind: &gogotypes.Value_BoolValue{
									BoolValue: true,
								}},
							}},
						},
					},
				}),
				AllowAny: []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Expression: "claims.foo == \"bar\"",
					},
					{
						Conditions: []*types.ProvisionTokenSpecV2GenericOIDC_Condition{
							{
								Attribute: "foo",
								Eq: &types.ProvisionTokenSpecV2GenericOIDC_ConditionEq{
									Value: "bar",
								},
							},
						},
					},
				},
			},
		},
	}

	marshaled, err := services.MarshalProvisionToken(token)
	require.NoError(t, err)

	unmarshaled, err := services.UnmarshalProvisionToken(marshaled)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(token, unmarshaled, protocmp.Transform()), "generic_oidc structs must roundtrip correctly")

	token = &types.ProvisionTokenV2{
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "generic-oidc-token",
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleNode},
			JoinMethod: types.JoinMethodGenericOIDC,
			GenericOIDC: &types.ProvisionTokenSpecV2GenericOIDC{
				Issuer:          "https://example.com",
				Audience:        "example.teleport.sh/generic-oidc-token",
				MustMatchFields: nil,
				AllowAny: []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Expression: "claims.foo == \"bar\"",
					},
					{
						Conditions: []*types.ProvisionTokenSpecV2GenericOIDC_Condition{
							{
								Attribute: "foo",
								Eq: &types.ProvisionTokenSpecV2GenericOIDC_ConditionEq{
									Value: "bar",
								},
							},
						},
					},
				},
			},
		},
	}

	marshaled, err = services.MarshalProvisionToken(token)
	require.NoError(t, err)

	unmarshaled, err = services.UnmarshalProvisionToken(marshaled)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(token, unmarshaled, protocmp.Transform()), "nil MustMatchFields must roundtrip correctly")
}

// TestGenericOIDCStructSurvivesRoundTrip_FastMarshal ensures struct
// roundtripping works explicitly through FastMarshal.
func TestGenericOIDCStructSurvivesRoundTrip_FastMarshal(t *testing.T) {
	t.Parallel()

	want := &types.ProvisionTokenSpecV2GenericOIDC{
		Issuer:   "https://example.com",
		Audience: "example",
		MustMatchFields: types.NewStructFromGogoValues(map[string]*gogotypes.Value{
			"sub":    {Kind: &gogotypes.Value_StringValue{StringValue: "repo:foo/bar"}},
			"count":  {Kind: &gogotypes.Value_NumberValue{NumberValue: 3}},
			"active": {Kind: &gogotypes.Value_BoolValue{BoolValue: true}},
		}),
	}

	// Explicitly route through FastMarshal to ensure it calls our overridden
	// marshal impl, otherwise the test could plausibly be testing jsonpb
	// and the actual impl could fail.
	data, err := utils.FastMarshal(want)
	require.NoError(t, err)

	var got types.ProvisionTokenSpecV2GenericOIDC
	require.NoError(t, utils.FastUnmarshal(data, &got))

	require.Empty(t, cmp.Diff(want, &got, protocmp.Transform()), "generic_oidc must_match_fields must survive round-trip")
}

func TestGenericOIDCUnmarshalDocumentedFieldNames(t *testing.T) {
	raw := []byte(`{
		"kind": "token",
		"version": "v2",
		"metadata": {"name":"go"},
		"spec":{
			"roles": ["Node"],
			"join_method": "generic_oidc",
			"generic_oidc": {
				"issuer": "https://example.com",
				"audience": "aud",
				"allow_any":[{"expression":"x == 1"}],
				"must_match_fields": {"sub":"repo:foo/bar"}
			}
		}
	}`)

	tok, err := services.UnmarshalProvisionToken(raw)
	require.NoError(t, err) // fails today: CheckAndSetDefaults rejects empty config
	spec, err := tok.GetGenericOIDC()
	require.NoError(t, err)
	require.Equal(t, "https://example.com", spec.Issuer)
	require.Len(t, spec.AllowAny, 1)
	require.Equal(t, "repo:foo/bar", spec.MustMatchFields.GetFields()["sub"].GetStringValue())
}
