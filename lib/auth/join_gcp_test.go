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

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/gcp"
)

type mockGCPTokenValidator struct {
	tokens map[string]gcp.IDTokenClaims
}

func (m *mockGCPTokenValidator) Validate(_ context.Context, token string) (*gcp.IDTokenClaims, error) {
	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}
	return &claims, nil
}

func TestAuth_RegisterUsingToken_GCP(t *testing.T) {
	t.Parallel()

	validIDToken := "test.fake.jwt"
	idTokenValidator := &mockGCPTokenValidator{
		tokens: map[string]gcp.IDTokenClaims{
			validIDToken: {
				Email: "service-account@example.com",
				Google: gcp.Google{
					ComputeEngine: gcp.ComputeEngine{
						ProjectID:    "project1",
						Zone:         "us-west1-b",
						InstanceID:   "1234",
						InstanceName: "test-instance",
					},
				},
			},
		},
	}
	var withTokenValidator ServerOption = func(server *Server) error {
		server.gcpIDTokenValidator = idTokenValidator
		return nil
	}
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir(), withTokenValidator)
	require.NoError(t, err)
	auth := p.a

	// helper for creating RegisterUsingTokenRequest
	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)
	newRequest := func(idToken string) *types.RegisterUsingTokenRequest {
		return &types.RegisterUsingTokenRequest{
			HostID:       "host-id",
			Role:         types.RoleNode,
			IDToken:      idToken,
			PublicTLSKey: tlsPublicKey,
			PublicSSHKey: sshPublicKey,
		}
	}

	allowRule := func(modifier func(*types.ProvisionTokenSpecV2GCP_Rule)) *types.ProvisionTokenSpecV2GCP_Rule {
		rule := &types.ProvisionTokenSpecV2GCP_Rule{
			ProjectIDs:      []string{"project1"},
			Locations:       []string{"us-west1-b"},
			ServiceAccounts: []string{"service-account@example.com"},
		}
		if modifier != nil {
			modifier(rule)
		}
		return rule
	}

	allowRulesNotMatched := require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...any) {
		require.ErrorContains(t, err, "id token claims did not match any allow rules")
		require.True(t, trace.IsAccessDenied(err))
	})
	tests := []struct {
		name        string
		request     *types.RegisterUsingTokenRequest
		tokenSpec   types.ProvisionTokenSpecV2
		assertError require.ErrorAssertionFunc
	}{
		{
			name: "success",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGCP,
				Roles:      []types.SystemRole{types.RoleNode},
				GCP: &types.ProvisionTokenSpecV2GCP{
					Allow: []*types.ProvisionTokenSpecV2GCP_Rule{
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "multiple allow rules",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGCP,
				Roles:      []types.SystemRole{types.RoleNode},
				GCP: &types.ProvisionTokenSpecV2GCP{
					Allow: []*types.ProvisionTokenSpecV2GCP_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GCP_Rule) {
							rule.ProjectIDs = []string{"not-matching"}
						}),
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "match region to zone",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGCP,
				Roles:      []types.SystemRole{types.RoleNode},
				GCP: &types.ProvisionTokenSpecV2GCP{
					Allow: []*types.ProvisionTokenSpecV2GCP_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GCP_Rule) {
							rule.Locations = []string{"us-west1"}
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "incorrect project id",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGCP,
				Roles:      []types.SystemRole{types.RoleNode},
				GCP: &types.ProvisionTokenSpecV2GCP{
					Allow: []*types.ProvisionTokenSpecV2GCP_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GCP_Rule) {
							rule.ProjectIDs = []string{"not matching"}
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect location",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGCP,
				Roles:      []types.SystemRole{types.RoleNode},
				GCP: &types.ProvisionTokenSpecV2GCP{
					Allow: []*types.ProvisionTokenSpecV2GCP_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GCP_Rule) {
							rule.Locations = []string{"somewhere else"}
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect service account",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGCP,
				Roles:      []types.SystemRole{types.RoleNode},
				GCP: &types.ProvisionTokenSpecV2GCP{
					Allow: []*types.ProvisionTokenSpecV2GCP_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GCP_Rule) {
							rule.ServiceAccounts = []string{"something-else@example.com"}
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := types.NewProvisionTokenFromSpec(
				tc.name, time.Now().Add(time.Minute), tc.tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, auth.CreateToken(ctx, token))
			tc.request.Token = tc.name

			_, err = auth.RegisterUsingToken(ctx, tc.request)
			tc.assertError(t, err)
		})
	}
}

func TestIsGCPZoneInLocation(t *testing.T) {
	t.Parallel()
	passingCases := []struct {
		name     string
		location string
		zone     string
	}{
		{
			name:     "matching zone",
			location: "us-west1-b",
			zone:     "us-west1-b",
		},
		{
			name:     "matching region",
			location: "us-west1",
			zone:     "us-west1-b",
		},
	}
	for _, tc := range passingCases {
		t.Run("accept "+tc.name, func(t *testing.T) {
			require.True(t, isGCPZoneInLocation(tc.location, tc.zone))
		})
	}

	failingCases := []struct {
		name     string
		location string
		zone     string
	}{
		{
			name:     "non-matching zone",
			location: "europe-southwest1-b",
			zone:     "us-west1-b",
		},
		{
			name:     "non-matching region",
			location: "europe-southwest1",
			zone:     "us-west1-b",
		},
		{
			name:     "malformed location",
			location: "us",
			zone:     "us-west1-b",
		},
		{
			name:     "similar but non-matching region",
			location: "europe-west1",
			zone:     "europe-west12-a",
		},
		{
			name:     "empty zone",
			location: "us-west1",
			zone:     "",
		},
		{
			name:     "empty location",
			location: "",
			zone:     "us-west1-b",
		},
		{
			name:     "invalid zone",
			location: "us-west1",
			zone:     "us-west1",
		},
	}
	for _, tc := range failingCases {
		t.Run("reject "+tc.name, func(t *testing.T) {
			require.False(t, isGCPZoneInLocation(tc.location, tc.zone))
		})
	}
}
