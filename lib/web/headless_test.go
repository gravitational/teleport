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

package web

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
)

func TestPutHeadlessState(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Create a user with appropriate roles
	username := "headless-test-user"
	role := services.NewPresetEditorRole()
	pack := proxy.authPack(t, username, []types.Role{role})

	// Set up WebAuthn as the second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorWebauthn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = env.server.Auth().UpsertAuthPreference(t.Context(), ap)
	require.NoError(t, err)

	// Register a WebAuthn device for the user.
	userClient, err := env.server.NewClient(authtest.TestUser(username))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, userClient.Close())
	})
	webauthnDev, err := authtest.RegisterTestDevice(
		t.Context(),
		userClient,
		"webauthn",
		authproto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		nil, /* authenticator */
	)
	require.NoError(t, err)

	getMFAResponse := func() *client.MFAChallengeResponse {
		t.Helper()
		// Create an authentication challenge and solve it with the WebAuthn device.
		chal, err := userClient.CreateAuthenticateChallenge(t.Context(), &authproto.CreateAuthenticateChallengeRequest{
			Request: &authproto.CreateAuthenticateChallengeRequest_ContextUser{},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_HEADLESS_LOGIN,
			},
		})
		require.NoError(t, err)
		mfaResp, err := webauthnDev.SolveAuthn(chal)
		require.NoError(t, err)

		return &client.MFAChallengeResponse{
			WebauthnResponse: wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()),
		}
	}

	tests := []struct {
		name string
		// setupHeadless creates new headless auth and returns its ID.
		setupHeadless  func(context.Context) string
		request        client.HeadlessRequest
		useMFA         bool
		expectedStatus int
		expectedErrMsg string
	}{
		{
			name: "invalid headless authentication ID",
			setupHeadless: func(ctx context.Context) string {
				return "non-existent-id"
			},
			request: client.HeadlessRequest{
				Action: "denied",
			},
			expectedStatus: 404,
			expectedErrMsg: "not found",
		},
		{
			name: "invalid action",
			setupHeadless: func(ctx context.Context) string {
				sshPubKey := []byte("fake-ssh-public-key-invalid-action")
				headlessID := services.NewHeadlessAuthenticationID(sshPubKey)

				ha, err := types.NewHeadlessAuthentication(username, headlessID, env.clock.Now().Add(5*time.Minute))
				require.NoError(t, err)
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
				ha.SshPublicKey = sshPubKey

				err = env.server.Auth().UpsertHeadlessAuthentication(ctx, ha)
				require.NoError(t, err)
				return ha.GetName()
			},
			request: client.HeadlessRequest{
				Action: "invalid-action",
			},
			expectedStatus: 400,
			expectedErrMsg: "unknown action invalid-action",
		},
		{
			name: "accept without MFA response",
			setupHeadless: func(ctx context.Context) string {
				sshPubKey := []byte("fake-ssh-public-key-accept-no-mfa")
				headlessID := services.NewHeadlessAuthenticationID(sshPubKey)

				ha, err := types.NewHeadlessAuthentication(username, headlessID, env.clock.Now().Add(5*time.Minute))
				require.NoError(t, err)
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
				ha.SshPublicKey = sshPubKey

				err = env.server.Auth().UpsertHeadlessAuthentication(ctx, ha)
				require.NoError(t, err)
				return ha.GetName()
			},
			request: client.HeadlessRequest{
				Action: "accept",
			},
			expectedStatus: 400,
			expectedErrMsg: "expected MFA auth challenge response",
		},
		{
			name: "accept with MFA response",
			setupHeadless: func(ctx context.Context) string {
				// Create the headless authentication request.
				sshPubKey := []byte("fake-ssh-public-key-accept-with-mfa")
				headlessID := services.NewHeadlessAuthenticationID(sshPubKey)

				ha, err := types.NewHeadlessAuthentication(username, headlessID, env.clock.Now().Add(5*time.Minute))
				require.NoError(t, err)
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
				ha.SshPublicKey = sshPubKey

				err = env.server.Auth().UpsertHeadlessAuthentication(ctx, ha)
				require.NoError(t, err)

				return ha.GetName()
			},
			useMFA: true,
			request: client.HeadlessRequest{
				Action: "accept",
			},
			expectedStatus: 200,
		},
		{
			name: "denied without MFA response",
			setupHeadless: func(ctx context.Context) string {
				sshPubKey := []byte("fake-ssh-public-key")
				headlessID := services.NewHeadlessAuthenticationID(sshPubKey)

				ha, err := types.NewHeadlessAuthentication(username, headlessID, env.clock.Now().Add(5*time.Minute))
				require.NoError(t, err)
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
				ha.SshPublicKey = sshPubKey

				err = env.server.Auth().UpsertHeadlessAuthentication(ctx, ha)
				require.NoError(t, err)
				return ha.GetName()
			},
			request: client.HeadlessRequest{
				Action: "denied",
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headlessID := tt.setupHeadless(t.Context())

			request := tt.request
			if tt.useMFA {
				request.MFAResponse = getMFAResponse()
			}

			endpoint := pack.clt.Endpoint("webapi", "headless", headlessID)
			resp, err := pack.clt.PutJSON(t.Context(), endpoint, request)
			require.Equal(t, tt.expectedStatus, resp.Code(), "unexpected status code")
			if tt.expectedStatus != 200 {
				require.ErrorContains(t, err, tt.expectedErrMsg, "unexpected error message")
				return
			}
			require.NoError(t, err)

			// Verify the state was updated correctly.
			ha, err := env.server.Auth().GetHeadlessAuthentication(t.Context(), username, headlessID)
			require.NoError(t, err)

			var expectedState types.HeadlessAuthenticationState
			switch tt.request.Action {
			case "accept":
				expectedState = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
			case "denied":
				expectedState = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED
			}
			require.Equal(t, expectedState, ha.State)
		})
	}
}

func TestGetHeadless(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	username := "headless-get-user"
	role := services.NewPresetEditorRole()
	pack := proxy.authPack(t, username, []types.Role{role})

	tests := []struct {
		name string
		// setupHeadless creates new headless auth and returns its ID.
		setupHeadless  func(context.Context) string
		expectedStatus int
		expectedErrMsg string
	}{
		{
			name: "get existing headless authentication",
			setupHeadless: func(ctx context.Context) string {
				sshPubKey := []byte("fake-ssh-public-key-get")
				headlessID := services.NewHeadlessAuthenticationID(sshPubKey)

				ha, err := types.NewHeadlessAuthentication(username, headlessID, env.clock.Now().Add(5*time.Minute))
				require.NoError(t, err)
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
				ha.SshPublicKey = sshPubKey

				err = env.server.Auth().UpsertHeadlessAuthentication(ctx, ha)
				require.NoError(t, err)
				return ha.GetName()
			},
			expectedStatus: 200,
		},
		{
			name: "non-existent headless authentication",
			setupHeadless: func(ctx context.Context) string {
				return "non-existent-id"
			},
			expectedStatus: 400,
			expectedErrMsg: "requested invalid headless session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headlessID := tt.setupHeadless(t.Context())

			endpoint := pack.clt.Endpoint("webapi", "headless", headlessID)
			resp, err := pack.clt.Get(t.Context(), endpoint, nil)
			require.Equal(t, tt.expectedStatus, resp.Code(), "unexpected status code")
			if tt.expectedStatus != 200 {
				require.ErrorContains(t, err, tt.expectedErrMsg, "unexpected error message")
				return
			}
			require.NoError(t, err)

			var ha types.HeadlessAuthentication
			err = json.Unmarshal(resp.Bytes(), &ha)
			require.NoError(t, err)
			require.Equal(t, headlessID, ha.Metadata.Name)
		})
	}
}
