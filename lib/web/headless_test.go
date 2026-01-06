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

package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authtest"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
)

func newTestHeadlessAuthn(t *testing.T, user string) *types.HeadlessAuthentication {
	t.Helper()

	sshKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubKey := ssh.MarshalAuthorizedKey(sshPub)

	tlsKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKey, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	headlessID := services.NewHeadlessAuthenticationID(sshPubKey)
	headlessAuthn := &types.HeadlessAuthentication{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: headlessID,
			},
		},
		User:            user,
		SshPublicKey:    sshPubKey,
		TlsPublicKey:    tlsPubKey,
		ClientIpAddress: "0.0.0.0",
		State:           types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
	}
	headlessAuthn.SetExpiry(time.Now().Add(time.Minute))
	require.NoError(t, headlessAuthn.CheckAndSetDefaults())

	return headlessAuthn
}

func TestPutHeadlessState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	user := "alice"
	pack := proxy.authPack(t, user, nil)

	headlessAuthn := newTestHeadlessAuthn(t, user)
	require.NoError(t, env.server.Auth().UpsertHeadlessAuthentication(ctx, headlessAuthn))

	makeWebauthnMFAResponse := func(t *testing.T) *client.MFAChallengeResponse {
		t.Helper()

		userClient, err := env.server.NewClient(authtest.TestUser(user))
		require.NoError(t, err)

		ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorWebauthn,
			Webauthn: &types.Webauthn{
				RPID: "localhost",
			},
		})
		require.NoError(t, err)
		_, err = env.server.Auth().UpsertAuthPreference(ctx, ap)
		require.NoError(t, err)

		webauthnDev, err := authtest.RegisterTestDevice(
			ctx,
			userClient,
			"webauthn",
			authproto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			nil,
			authtest.WithTestDeviceClock(env.clock),
		)
		require.NoError(t, err)

		chal, err := userClient.CreateAuthenticateChallenge(ctx, &authproto.CreateAuthenticateChallengeRequest{
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

	for _, tc := range []struct {
		name      string
		action    string
		setup     func(t *testing.T) *client.MFAChallengeResponse
		wantErr   func(error) bool
		wantState types.HeadlessAuthenticationState
	}{
		{
			name:      "denied",
			action:    "denied",
			wantErr:   func(err error) bool { return err == nil },
			wantState: types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED,
		},
		{
			name:      "accept with MFA succeeds",
			action:    "accept",
			setup:     makeWebauthnMFAResponse,
			wantErr:   func(err error) bool { return err == nil },
			wantState: types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED,
		},
		{
			name:   "accept without MFA is rejected",
			action: "accept",
			wantErr: func(err error) bool {
				return trace.IsBadParameter(err)
			},
			wantState: types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
		},
		{
			name:   "unknown action",
			action: "foo",
			wantErr: func(err error) bool {
				return trace.IsBadParameter(err)
			},
			wantState: types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// If MFA is required, set it up
			var mfaResp *client.MFAChallengeResponse
			if tc.setup != nil {
				mfaResp = tc.setup(t)
			}

			// Reset state of headless auth back to pending for this test
			reset := *headlessAuthn
			reset.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING
			require.NoError(t, env.server.Auth().UpsertHeadlessAuthentication(ctx, &reset))

			// Make the call to the put endpoint
			endpoint := pack.clt.Endpoint("webapi", "headless", headlessAuthn.GetName())
			payload := client.HeadlessRequest{Action: tc.action, MFAResponse: mfaResp}
			resp, err := pack.clt.PutJSON(ctx, endpoint, payload)
			require.True(t, tc.wantErr(err), "unexpected error: %v", err)
			if err == nil {
				require.Equal(t, http.StatusOK, resp.Code())
			}

			// Verify that headless auth has the expected state
			ha, err := env.server.Auth().GetHeadlessAuthentication(ctx, user, headlessAuthn.GetName())
			require.NoError(t, err)
			require.Equal(t, tc.wantState, ha.State)
		})
	}
}
