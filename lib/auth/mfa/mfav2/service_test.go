// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mfav1_test

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/types"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authtest"
	mfav2impl "github.com/gravitational/teleport/lib/auth/mfa/mfav2"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

const (
	sourceCluster = "test-cluster"
)

func TestCompleteBrowserMFAChallenge_Success(t *testing.T) {
	t.Parallel()

	authServer, service, _, user := setupAuthServer(t, nil)

	requestID := "test-request-id"
	authServer.requestIDs.Store(requestID, struct{}{})

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	resp, err := service.CompleteBrowserMFAChallenge(
		ctx,
		&mfav2.CompleteBrowserMFAChallengeRequest{
			BrowserMfaResponse: &mfav1.BrowserMFAResponse{
				RequestId: requestID,
				WebauthnResponse: &webauthnpb.CredentialAssertionResponse{
					Type: "public-key",
				},
			},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.TshRedirectUrl)
	require.Contains(t, resp.TshRedirectUrl, "127.0.0.1")
}

func TestCompleteBrowserMFAChallenge_NonUserDenied(t *testing.T) {
	t.Parallel()

	_, service, _, _ := setupAuthServer(t, nil)

	// Use a context with a non-user role (proxy).
	ctx := authz.ContextWithUser(t.Context(), authtest.TestBuiltin(types.RoleProxy).I)

	resp, err := service.CompleteBrowserMFAChallenge(
		ctx,
		&mfav2.CompleteBrowserMFAChallengeRequest{
			BrowserMfaResponse: &mfav1.BrowserMFAResponse{
				RequestId: "test-request-id",
				WebauthnResponse: &webauthnpb.CredentialAssertionResponse{
					Type: "public-key",
				},
			},
		},
	)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
	require.ErrorContains(t, err, "only local or remote users can complete a browser MFA challenge")
	require.Nil(t, resp)
}

func TestCompleteBrowserMFAChallenge_InvalidRequest(t *testing.T) {
	t.Parallel()

	authServer, service, _, user := setupAuthServer(t, nil)

	ctx := authz.ContextWithUser(t.Context(), authtest.TestUserWithRoles(user.GetName(), user.GetRoles()).I)

	for _, testCase := range []struct {
		name          string
		req           *mfav2.CompleteBrowserMFAChallengeRequest
		expectedError string
	}{
		{
			name: "missing BrowserMfaResponse",
			req: &mfav2.CompleteBrowserMFAChallengeRequest{
				BrowserMfaResponse: nil,
			},
			expectedError: "missing browser_mfa_response in request",
		},
		{
			name: "missing RequestId",
			req: &mfav2.CompleteBrowserMFAChallengeRequest{
				BrowserMfaResponse: &mfav1.BrowserMFAResponse{
					RequestId: "",
					WebauthnResponse: &webauthnpb.CredentialAssertionResponse{
						Type: "public-key",
					},
				},
			},
			expectedError: "missing request_id in browser_mfa_response",
		},
		{
			name: "missing WebauthnResponse",
			req: &mfav2.CompleteBrowserMFAChallengeRequest{
				BrowserMfaResponse: &mfav1.BrowserMFAResponse{
					RequestId:        "test-request-id",
					WebauthnResponse: nil,
				},
			},
			expectedError: "missing webauthn_response in browser_mfa_response",
		},
		{
			name: "non-existent RequestId",
			req: func() *mfav2.CompleteBrowserMFAChallengeRequest {
				return &mfav2.CompleteBrowserMFAChallengeRequest{
					BrowserMfaResponse: &mfav1.BrowserMFAResponse{
						RequestId: "non-existent-request-id",
						WebauthnResponse: &webauthnpb.CredentialAssertionResponse{
							Type: "public-key",
						},
					},
				}
			}(),
			expectedError: "invalid browser MFA challenge request ID",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// For the "invalid RequestId" test, ensure we have at least one valid ID stored.
			if testCase.name == "invalid RequestId" {
				authServer.requestIDs.Store("valid-id", struct{}{})
			}

			resp, err := service.CompleteBrowserMFAChallenge(ctx, testCase.req)
			require.Error(t, err)
			require.ErrorContains(t, err, testCase.expectedError)
			require.Nil(t, resp)
		})
	}
}

func setupAuthServer(t *testing.T, devices []*types.MFADevice) (*mockAuthServer, *mfav2impl.Service, *eventstest.MockRecorderEmitter, types.User) {
	t.Helper()

	emitter := &eventstest.MockRecorderEmitter{}

	authServer, err := NewMockAuthServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			AuditLog:    &eventstest.MockAuditLog{Emitter: emitter},
			ClusterName: sourceCluster,
			Dir:         t.TempDir(),
			AuthPreferenceSpec: &types.AuthPreferenceSpecV2{
				SecondFactors: []types.SecondFactorType{
					types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
					types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
		},
	},
		devices,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := authServer.Close(); err != nil {
			t.Logf("Failed to close auth server: %v", err)
		}
	})

	role, err := authtest.CreateRole(t.Context(), authServer.Auth(), "test-role", types.RoleSpecV6{})
	require.NoError(t, err)

	user, err := authtest.CreateUser(t.Context(), authServer.Auth(), "test-user", role)
	require.NoError(t, err)

	service, err := mfav2impl.NewService(mfav2impl.ServiceConfig{
		Authorizer: authServer.AuthServer.Authorizer,
		AuthServer: authServer,
	})
	require.NoError(t, err)

	return authServer, service, emitter, user
}
