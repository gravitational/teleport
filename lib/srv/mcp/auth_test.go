/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcp

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

func Test_rewriteAuthDetails(t *testing.T) {
	tests := []struct {
		name  string
		input *types.Rewrite
		want  rewriteAuthDetails
	}{
		{
			name:  "nil",
			input: nil,
			want:  rewriteAuthDetails{},
		},
		{
			name: "auth header",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer abcdef",
				}},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
			},
		},
		{
			name: "auth header with external traits",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer {{external.abcdef}}",
				}},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
			},
		},
		{
			name: "jwt",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "X-API-Key",
					Value: "{{internal.jwt}}",
				}},
			},
			want: rewriteAuthDetails{
				hasJWTTrait: true,
			},
		},
		{
			name: "id token",
			input: &types.Rewrite{
				Headers: []*types.Header{{
					Name:  "Authorization",
					Value: "Bearer {{internal.id_token}}",
				}},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
				hasIDTokenTrait:   true,
			},
		},
		{
			name: "multiple ",
			input: &types.Rewrite{
				Headers: []*types.Header{
					{
						Name:  "foo",
						Value: "bar",
					},
					{
						Name:  "Authorization",
						Value: "Bearer {{internal.id_token}}",
					},
					{
						Name:  "X-API-Key",
						Value: "{{internal.jwt}}",
					},
				},
			},
			want: rewriteAuthDetails{
				rewriteAuthHeader: true,
				hasIDTokenTrait:   true,
				hasJWTTrait:       true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := newRewriteAuthDetails(tt.input)
			require.Equal(t, tt.want, actual)
		})
	}
}

func Test_generateJWTAndTraits(t *testing.T) {
	app, err := types.NewAppV3(types.Metadata{
		Name: "test-http",
	}, types.AppSpecV3{
		URI: "https://example.com",
		Rewrite: &types.Rewrite{
			Headers: []*types.Header{{
				Name:  "Authorization",
				Value: "Bearer {{internal.id_token}}",
			}},
		},
	})
	require.NoError(t, err)

	testCtx := setupTestContext(t, withAdminRole(t), withApp(app))
	clock := clockwork.NewFakeClock()
	authClient := &mockAuthClient{}
	auth := sessionAuth{
		SessionCtx: testCtx.SessionCtx,
		authClient: authClient,
		clock:      clock,
	}

	jwt, traits, err := auth.generateJWTAndTraits(t.Context())
	require.NoError(t, err)
	require.Equal(t, "app-token-for-ai-by-jwt", jwt)
	require.NotEmpty(t, traits)
	require.Equal(t, []string{"app-token-for-ai-by-jwt"}, traits[constants.TraitJWT])
	require.Equal(t, []string{"app-token-for-ai-by-oidc_idp"}, traits[constants.TraitIDToken])

	// Two calls, one for JWT, and one for ID token.
	appTokenRequests := authClient.getAppTokenRequests()
	require.Len(t, appTokenRequests, 2)
	// Check token ttl.
	require.Equal(t, maxTokenDuration, appTokenRequests[0].Expires.Sub(clock.Now()))
	require.Equal(t, maxTokenDuration, appTokenRequests[1].Expires.Sub(clock.Now()))

	// Check token is cached.
	clock.Advance(time.Minute)
	_, _, err = auth.generateJWTAndTraits(t.Context())
	require.NoError(t, err)
	require.Len(t, authClient.getAppTokenRequests(), 2)

	// Refresh.
	clock.Advance(maxTokenDuration)
	_, _, err = auth.generateJWTAndTraits(t.Context())
	require.NoError(t, err)
	appTokenRequests = authClient.getAppTokenRequests()
	require.Len(t, appTokenRequests, 4)
	require.Equal(t, maxTokenDuration, appTokenRequests[2].Expires.Sub(clock.Now()))
	require.Equal(t, maxTokenDuration, appTokenRequests[3].Expires.Sub(clock.Now()))

	// Advance to right before identity expires so the token TTL is less than
	// maxTokenDuration.
	clock.Advance(testCtx.Identity.Expires.Sub(clock.Now()) - time.Minute)
	_, _, err = auth.generateJWTAndTraits(t.Context())
	require.NoError(t, err)
	appTokenRequests = authClient.getAppTokenRequests()
	require.Len(t, appTokenRequests, 6)
	require.Equal(t, testCtx.Identity.Expires, appTokenRequests[4].Expires)
	require.Equal(t, testCtx.Identity.Expires, appTokenRequests[5].Expires)
}
