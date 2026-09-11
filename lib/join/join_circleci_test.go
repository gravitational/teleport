/*
 * Teleport
 * Copyright (C) 2023-2025  Gravitational, Inc.
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

package join_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/join/circleci"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/utils/slices"
)

func TestJoinCircleCI(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	var (
		validIDToken  = "valid-token"
		validOrg      = "valid-org"
		validProject  = "valid-project"
		validContextA = "valid-context-a"
		validContextB = "valid-context-b"
	)
	// stand up auth server with mocked CircleCI token validator
	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:            t.TempDir(),
			ScopesFeatures: scopes.Features{Enabled: true},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, authServer.Shutdown(t.Context())) })
	auth := authServer.Auth()

	authServer.Auth().SetCircleCITokenValidator(func(
		ctx context.Context, issuerURL, organizationID, token string,
	) (*circleci.IDTokenClaims, error) {
		if organizationID == validOrg && token == validIDToken {
			return &circleci.IDTokenClaims{
				Sub:        "org/valid-org/project/valid-project/user/USER_ID",
				ProjectID:  validProject,
				ContextIDs: []string{validContextA, validContextB},
			}, nil
		}
		return nil, errMockInvalidToken
	})

	// helper for creating RegisterUsingTokenRequest
	sshPrivateKey, sshPublicKey, err := testauthority.GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := authtest.PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	scopedBot := CreateScopedBot(t, authServer.Auth(), "circle")

	newRequest := func(idToken string) *types.RegisterUsingTokenRequest {
		return &types.RegisterUsingTokenRequest{
			HostID:       "host-id",
			Role:         types.RoleNode,
			IDToken:      idToken,
			PublicTLSKey: tlsPublicKey,
			PublicSSHKey: sshPublicKey,
		}
	}
	provisionTokenSpec := func(spec *types.ProvisionTokenSpecV2CircleCI) types.ProvisionTokenSpecV2 {
		return types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodCircleCI,
			Roles:      types.SystemRoles{types.RoleNode},
			CircleCI:   spec,
		}
	}

	scopedSpec := func(spec types.ProvisionTokenSpecV2) *joiningv1.CircleCI {
		return joiningv1.CircleCI_builder{
			OrganizationId: spec.CircleCI.OrganizationID,
			Allow: slices.Map(spec.CircleCI.Allow, func(in *types.ProvisionTokenSpecV2CircleCI_Rule) *joiningv1.CircleCI_Rule {
				return joiningv1.CircleCI_Rule_builder{
					ProjectId: in.ProjectID,
					ContextId: in.ContextID,
				}.Build()
			}),
		}.Build()
	}

	// helpers for error assertions
	allowRulesNotMatched := require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...any) {
		messageMatch := assert.ErrorContains(t, err, "id token claims did not match any allow rules")
		typeMatch := assert.True(t, trace.IsAccessDenied(err))
		require.True(t, messageMatch && typeMatch)
	})
	tokenNotMatched := func(t require.TestingT, err error, i ...any) {
		require.ErrorContains(t, err, "invalid token")
	}
	tests := []struct {
		name        string
		request     *types.RegisterUsingTokenRequest
		tokenSpec   types.ProvisionTokenSpecV2
		assertError require.ErrorAssertionFunc
	}{
		{
			name:    "matching-all",
			request: newRequest(validIDToken),
			tokenSpec: provisionTokenSpec(&types.ProvisionTokenSpecV2CircleCI{
				OrganizationID: validOrg,
				Allow: []*types.ProvisionTokenSpecV2CircleCI_Rule{
					{
						ProjectID: validProject,
						ContextID: validContextA,
					},
				},
			}),
			assertError: require.NoError,
		},
		{
			name:    "matching-second-context",
			request: newRequest(validIDToken),
			tokenSpec: provisionTokenSpec(&types.ProvisionTokenSpecV2CircleCI{
				OrganizationID: validOrg,
				Allow: []*types.ProvisionTokenSpecV2CircleCI_Rule{
					{
						ContextID: validContextB,
					},
				},
			}),
			assertError: require.NoError,
		},
		{
			name:    "invalid-org",
			request: newRequest(validIDToken),
			tokenSpec: provisionTokenSpec(&types.ProvisionTokenSpecV2CircleCI{
				OrganizationID: "not-this-org",
				Allow: []*types.ProvisionTokenSpecV2CircleCI_Rule{
					{
						ContextID: validContextB,
					},
				},
			}),
			assertError: tokenNotMatched,
		},
		{
			name:    "invalid-IDToken",
			request: newRequest("not-this-token"),
			tokenSpec: provisionTokenSpec(&types.ProvisionTokenSpecV2CircleCI{
				OrganizationID: validOrg,
				Allow: []*types.ProvisionTokenSpecV2CircleCI_Rule{
					{
						ContextID: validContextB,
					},
				},
			}),
			assertError: tokenNotMatched,
		},
		{
			name:    "missing-IDToken-in-request",
			request: newRequest(""),
			tokenSpec: provisionTokenSpec(&types.ProvisionTokenSpecV2CircleCI{
				OrganizationID: validOrg,
				Allow: []*types.ProvisionTokenSpecV2CircleCI_Rule{
					{
						ContextID: validContextB,
					},
				},
			}),
			assertError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)

				msg := err.Error()
				hasBackendError := strings.Contains(msg, "IDToken is required")
				hasClientError := strings.Contains(msg, "'CIRCLE_OIDC_TOKEN' must be present")

				require.True(t, hasBackendError || hasClientError, "matches at least one known error when IDToken is unset")
			},
		},
		{
			name:    "invalid-context",
			request: newRequest(validIDToken),
			tokenSpec: provisionTokenSpec(&types.ProvisionTokenSpecV2CircleCI{
				OrganizationID: validOrg,
				Allow: []*types.ProvisionTokenSpecV2CircleCI_Rule{
					{
						ProjectID: validProject,
						ContextID: "not-this-context",
					},
				},
			}),
			assertError: allowRulesNotMatched,
		},
		{
			name:    "invalid-project",
			request: newRequest(validIDToken),
			tokenSpec: provisionTokenSpec(&types.ProvisionTokenSpecV2CircleCI{
				OrganizationID: validOrg,
				Allow: []*types.ProvisionTokenSpecV2CircleCI_Rule{
					{
						ProjectID: "invalid-project",
						ContextID: validContextA,
					},
				},
			}),
			assertError: allowRulesNotMatched,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create Token resource for use in test
			token, err := types.NewProvisionTokenFromSpec(
				tt.name, time.Now().Add(time.Minute), tt.tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, auth.CreateToken(ctx, token))
			tt.request.Token = token.GetName()

			nopClient, err := authServer.NewClient(authtest.TestNop())
			require.NoError(t, err)

			_, err = authServer.Auth().CreateScopedToken(t.Context(), joiningv1.CreateScopedTokenRequest_builder{
				Token: joiningv1.ScopedToken_builder{
					Kind:    types.KindScopedToken,
					Version: types.V1,
					Metadata: headerv1.Metadata_builder{
						Name: strings.ToLower(tt.name),
					}.Build(),
					Scope: "/test",
					Spec: joiningv1.ScopedTokenSpec_builder{
						UsageMode:  joining.TokenUsageModeBot,
						Bot:        scopedBot,
						JoinMethod: string(types.JoinMethodCircleCI),
						Roles:      []string{types.RoleBot.String()},
						Circleci:   scopedSpec(tt.tokenSpec),
					}.Build(),
				}.Build(),
			}.Build())
			require.NoError(t, err)

			t.Run("legacy", func(t *testing.T) {
				_, err = auth.RegisterUsingToken(ctx, tt.request)
				tt.assertError(t, err)
			})

			t.Run("legacy joinclient", func(t *testing.T) {
				_, err := joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
					Token:      tt.request.Token,
					JoinMethod: types.JoinMethodCircleCI,
					ID: state.IdentityID{
						Role:     tt.request.Role,
						NodeName: "testnode",
						HostUUID: tt.request.HostID,
					},
					IDToken:    tt.request.IDToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
				if err != nil {
					return
				}
			})

			t.Run("new joinclient", func(t *testing.T) {
				_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
					Token:      tt.request.Token,
					JoinMethod: types.JoinMethodCircleCI,
					ID: state.IdentityID{
						Role:     types.RoleInstance, // RoleNode is not allowed
						NodeName: "testnode",
					},
					IDToken:    tt.request.IDToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
				if err != nil {
					return
				}
			})

			t.Run("scoped", func(t *testing.T) {
				_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
					Token: apiscopes.QualifiedName{
						Scope: testTokenScope,
						Name:  strings.ToLower(tt.name),
					}.String(),
					JoinMethod: types.JoinMethodCircleCI,
					ID: state.IdentityID{
						Role: types.RoleBot,
					},
					IDToken:    tt.request.IDToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
				if err != nil {
					return
				}
			})
		})
	}

}
