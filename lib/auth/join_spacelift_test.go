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
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/spacelift"
)

type mockSpaceliftTokenValidator struct {
	tokens map[string]spacelift.IDTokenClaims
}

func (m *mockSpaceliftTokenValidator) Validate(
	_ context.Context, hostname string, token string,
) (*spacelift.IDTokenClaims, error) {
	if hostname != "example.app.spacelift.io" {
		return nil, fmt.Errorf("bad hostname: %s", hostname)
	}

	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &claims, nil
}

func TestAuth_RegisterUsingToken_Spacelift(t *testing.T) {
	validIDToken := "test.fake.jwt"
	idTokenValidator := &mockSpaceliftTokenValidator{
		tokens: map[string]spacelift.IDTokenClaims{
			validIDToken: {
				Sub:        "space:root:stack:machineid-spacelift-test:run_type:TASK:scope:write",
				SpaceID:    "root",
				CallerType: "stack",
				CallerID:   "machineid-spacelift-test",
				RunType:    "TASK",
				RunID:      "01HEQ9W9CJ5GWD35SKJ46X789V",
				Scope:      "write",
			},
		},
	}
	var withTokenValidator ServerOption = func(server *Server) error {
		server.spaceliftIDTokenValidator = idTokenValidator
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

	allowRule := func(modifier func(*types.ProvisionTokenSpecV2Spacelift_Rule)) *types.ProvisionTokenSpecV2Spacelift_Rule {
		rule := &types.ProvisionTokenSpecV2Spacelift_Rule{
			SpaceID:    "root",
			CallerID:   "machineid-spacelift-test",
			CallerType: "stack",
			Scope:      "write",
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
		name          string
		setEnterprise bool
		request       *types.RegisterUsingTokenRequest
		tokenSpec     types.ProvisionTokenSpecV2
		assertError   require.ErrorAssertionFunc
	}{
		{
			name:          "success",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name:          "missing enterprise",
			setEnterprise: false,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(nil),
					},
				},
			},
			request: newRequest(validIDToken),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "requires Teleport Enterprise")
			},
		},
		{
			name:          "multiple allow rules",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Spacelift_Rule) {
							rule.SpaceID = "bar"
						}),
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name:          "incorrect space_id",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Spacelift_Rule) {
							rule.SpaceID = "bar"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect caller_id",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Spacelift_Rule) {
							rule.CallerID = "bar"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect caller_type",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Spacelift_Rule) {
							rule.CallerType = "bar"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect scope",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Spacelift_Rule) {
							rule.Scope = "bar"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "invalid token",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodSpacelift,
				Roles:      []types.SystemRole{types.RoleNode},
				Spacelift: &types.ProvisionTokenSpecV2Spacelift{
					Hostname: "example.app.spacelift.io",
					Allow: []*types.ProvisionTokenSpecV2Spacelift_Rule{
						allowRule(nil),
					},
				},
			},
			request: newRequest("some other token"),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, errMockInvalidToken)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnterprise {
				modules.SetTestModules(
					t,
					&modules.TestModules{TestBuildType: modules.BuildEnterprise},
				)
			}

			token, err := types.NewProvisionTokenFromSpec(
				tt.name, time.Now().Add(time.Minute), tt.tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, auth.CreateToken(ctx, token))
			tt.request.Token = tt.name

			_, err = auth.RegisterUsingToken(ctx, tt.request)
			tt.assertError(t, err)
		})
	}
}
