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

	"github.com/stretchr/testify/require"

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
