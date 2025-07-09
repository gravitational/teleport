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
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

func TestServer_CreateAuthenticateChallenge_authPreference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	reqUserPassword := func(user, pass string) *proto.CreateAuthenticateChallengeRequest {
		return &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{
				UserCredentials: &proto.UserCredentials{
					Username: user,
					Password: []byte(pass),
				},
			},
		}
	}

	reqPasswordless := func(_, _ string) *proto.CreateAuthenticateChallengeRequest {
		return &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_Passwordless{
				Passwordless: &proto.Passwordless{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN,
			},
		}
	}

	makeWebauthnSpec := func() *types.AuthPreferenceSpecV2 {
		return &types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorWebauthn,
			Webauthn: &types.Webauthn{
				RPID: "localhost",
			},
		}
	}

	tests := []struct {
		name            string
		spec            *types.AuthPreferenceSpecV2
		createReq       func(user, pass string) *proto.CreateAuthenticateChallengeRequest
		wantErr         error
		assertChallenge func(*proto.MFAAuthenticateChallenge)
	}{
		{
			name: "OK second_factor:off",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOff,
			},
			createReq: reqUserPassword,
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.Empty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:otp",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOTP,
			},
			createReq: reqUserPassword,
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, challenge.GetTOTP())
				require.Empty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:webauthn (derived from U2F)",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				U2F: &types.U2F{
					AppID: "https://localhost",
				},
			},
			createReq: reqUserPassword,
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name:      "OK second_factor:webauthn (standalone)",
			spec:      makeWebauthnSpec(),
			createReq: reqUserPassword,
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:webauthn uses explicit RPID",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				U2F: &types.U2F{
					AppID: "https://myoldappid.com",
				},
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
			createReq: reqUserPassword,
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
				require.Equal(t, "localhost", challenge.GetWebauthnChallenge().GetPublicKey().GetRpId())
			},
		},
		{
			name: "OK second_factor:optional",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOptional,
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
			createReq: reqUserPassword,
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:on",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOn,
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
			createReq: reqUserPassword,
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "allow_passwordless=false and passwordless challenge",
			spec: func() *types.AuthPreferenceSpecV2 {
				spec := makeWebauthnSpec()
				spec.AllowPasswordless = &types.BoolOption{Value: false}
				return spec
			}(),
			createReq: reqPasswordless,
			wantErr:   types.ErrPasswordlessDisabledBySettings,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			svr := newTestTLSServer(t)
			authServer := svr.Auth()
			mfa := configureForMFA(t, svr)
			user := mfa.User
			pass := mfa.Password

			authPreference, err := types.NewAuthPreference(*test.spec)
			require.NoError(t, err)
			_, err = authServer.UpsertAuthPreference(ctx, authPreference)
			require.NoError(t, err)

			challenge, err := authServer.CreateAuthenticateChallenge(ctx, test.createReq(user, pass))
			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr, "CreateAuthenticateChallenge error mismatch")
				return
			}
			require.NoError(t, err, "CreateAuthenticateChallenge errored unexpectedly")
			test.assertChallenge(challenge)
		})
	}
}

func TestCreateAuthenticateChallenge_WithAuth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	clt, err := srv.NewClient(TestUser(u.username))
	require.NoError(t, err)

	res, err := clt.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
		},
	})
	require.NoError(t, err)

	// MFA authentication works.
	// TODO(codingllama): Use a public endpoint to verify?
	mfaResp, err := u.webDev.SolveAuthn(res)
	require.NoError(t, err)
	requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN}
	_, err = srv.Auth().ValidateMFAAuthResponse(ctx, mfaResp, u.username, requiredExt)
	require.NoError(t, err)
}

func TestCreateAuthenticateChallenge_WithUserCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	tests := []struct {
		name     string
		wantErr  bool
		userCred *proto.UserCredentials
	}{
		{
			name:    "invalid password",
			wantErr: true,
			userCred: &proto.UserCredentials{
				Username: u.username,
				Password: []byte("invalid-password"),
			},
		},
		{
			name:    "invalid username",
			wantErr: true,
			userCred: &proto.UserCredentials{
				Username: "invalid-username",
				Password: u.password,
			},
		},
		{
			name: "valid credentials",
			userCred: &proto.UserCredentials{
				Username: u.username,
				Password: u.password,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := srv.Auth().CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: tc.userCred},
			})

			switch {
			case tc.wantErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
				require.NotNil(t, res.GetTOTP())
				require.NotEmpty(t, res.GetWebauthnChallenge())
			}
		})
	}
}

func TestCreateAuthenticateChallenge_WithUserCredentials_WithLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	for i := 1; i <= defaults.MaxLoginAttempts; i++ {
		_, err = srv.Auth().CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
				Username: u.username,
				Password: []byte("invalid-password"),
			}},
		})
		require.Error(t, err)

		// Test last attempt returns locked error.
		if i == defaults.MaxLoginAttempts {
			require.Equal(t, MaxFailedAttemptsErrMsg, err.Error())
		} else {
			require.NotEqual(t, MaxFailedAttemptsErrMsg, err.Error())
		}
	}
}

func TestCreateAuthenticateChallenge_WithRecoveryStartToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	tests := []struct {
		name       string
		wantErr    bool
		getRequest func() *proto.CreateAuthenticateChallengeRequest
	}{
		{
			name:    "invalid token type",
			wantErr: true,
			getRequest: func() *proto.CreateAuthenticateChallengeRequest {
				wrongToken, err := srv.Auth().createRecoveryToken(ctx, u.username, authclient.UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
				require.NoError(t, err)

				return &proto.CreateAuthenticateChallengeRequest{
					Request: &proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID{RecoveryStartTokenID: wrongToken.GetName()},
				}
			},
		},
		{
			name:    "token not found",
			wantErr: true,
			getRequest: func() *proto.CreateAuthenticateChallengeRequest {
				return &proto.CreateAuthenticateChallengeRequest{
					Request: &proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID{RecoveryStartTokenID: "token-not-found"},
				}
			},
		},
		{
			name: "valid token",
			getRequest: func() *proto.CreateAuthenticateChallengeRequest {
				startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, authclient.UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
				require.NoError(t, err)

				return &proto.CreateAuthenticateChallengeRequest{
					Request: &proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID{RecoveryStartTokenID: startToken.GetName()},
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := srv.Auth().CreateAuthenticateChallenge(ctx, tc.getRequest())

			switch {
			case tc.wantErr:
				require.True(t, trace.IsAccessDenied(err))
			default:
				require.NoError(t, err)
				require.NotNil(t, res.GetTOTP())
				require.NotEmpty(t, res.GetWebauthnChallenge())
			}
		})
	}
}

func TestCreateAuthenticateChallenge_mfaVerification(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	ctx := context.Background()

	adminClient, err := testServer.NewClient(TestBuiltin(types.RoleAdmin))
	require.NoError(t, err, "NewClient(types.RoleAdmin)")

	// Register a couple of SSH nodes.
	registerNode := func(node, env string) error {
		_, err := adminClient.UpsertNode(ctx, &types.ServerV2{
			Kind:    types.KindNode,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: uuid.NewString(),
				Labels: map[string]string{
					"env": env,
				},
			},
			Spec: types.ServerSpecV2{
				Hostname: node,
			},
		})
		return err
	}
	const devNode = "node1"
	const prodNode = "node2"
	require.NoError(t, registerNode(devNode, "dev"), "registerNode(%q)", devNode)
	require.NoError(t, registerNode(prodNode, "prod"), "registerNode(%q)", prodNode)

	// Create an MFA required role for "prod" nodes.
	prodRole, err := types.NewRole("prod_access", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			Logins: []string{"{{internal.logins}}"},
			NodeLabels: types.Labels{
				"env": []string{"prod"},
			},
		},
	})
	require.NoError(t, err, "NewRole(prod)")
	prodRole, err = adminClient.UpsertRole(ctx, prodRole)
	require.NoError(t, err, "UpsertRole(%q)", prodRole.GetName())

	// Create a role that requires MFA when joining sessions
	joinMFARole, err := types.NewRole("mfa_session_join", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			Logins: []string{"{{internal.logins}}"},
			NodeLabels: types.Labels{
				"env": []string{"*"},
			},
			JoinSessions: []*types.SessionJoinPolicy{
				{
					Name:  "session_join",
					Roles: []string{"access"},
					Kinds: []string{string(types.SSHSessionKind)},
					Modes: []string{string(types.SessionPeerMode)},
				},
			},
		},
	})
	require.NoError(t, err, "NewRole(joinMFA)")
	joinMFARole, err = adminClient.UpsertRole(ctx, joinMFARole)
	require.NoError(t, err, "UpsertRole(%q)", joinMFARole.GetName())

	// Create a role that doesn't require MFA when joining sessions
	joinNoMFARole, err := types.NewRole("no_mfa_session_join", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"{{internal.logins}}"},
			NodeLabels: types.Labels{
				"env": []string{"*"},
			},
			JoinSessions: []*types.SessionJoinPolicy{
				{
					Name:  "session_join",
					Roles: []string{"access"},
					Kinds: []string{string(types.SSHSessionKind)},
					Modes: []string{string(types.SessionPeerMode)},
				},
			},
		},
	})
	require.NoError(t, err, "NewRole(joinNoMFA)")
	joinNoMFARole, err = adminClient.UpsertRole(ctx, joinNoMFARole)
	require.NoError(t, err, "UpsertRole(%q)", joinNoMFARole.GetName())

	const normalLogin = "llama"
	createUser := func(role types.Role) *authclient.Client {
		// Create a user with MFA devices...
		userCreds, err := createUserWithSecondFactors(testServer)
		require.NoError(t, err, "createUserWithSecondFactors")
		username := userCreds.username

		// ...and assign the user a sane unix login, plus the specified role.
		user, err := adminClient.GetUser(ctx, username, false /* withSecrets */)
		require.NoError(t, err, "GetUser(%q)", username)

		user.SetLogins(append(user.GetLogins(), normalLogin))
		user.AddRole(role.GetName())
		_, err = adminClient.UpdateUser(ctx, user.(*types.UserV2))
		require.NoError(t, err, "UpdateUser(%q)", username)

		userClient, err := testServer.NewClient(TestUser(username))
		require.NoError(t, err, "NewClient(%q)", username)

		return userClient
	}

	prodAccessClient := createUser(prodRole)
	joinMFAClient := createUser(joinMFARole)
	joinNoMFAClient := createUser(joinNoMFARole)

	createReqForNode := func(node, login string) *proto.CreateAuthenticateChallengeRequest {
		return &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
				ContextUser: &proto.ContextUser{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
			},
			MFARequiredCheck: &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Node{
					Node: &proto.NodeLogin{
						Node:  node,
						Login: login,
					},
				},
			},
		}
	}

	tests := []struct {
		name            string
		userClient      *authclient.Client
		req             *proto.CreateAuthenticateChallengeRequest
		wantMFARequired proto.MFARequired
		wantChallenges  bool
	}{
		{
			name:            "MFA not required to start session, no challenges issued",
			userClient:      prodAccessClient,
			req:             createReqForNode(devNode, normalLogin),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		},
		{
			name:            "MFA required to start session",
			userClient:      prodAccessClient,
			req:             createReqForNode(prodNode, normalLogin),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			wantChallenges:  true,
		},
		{
			name:            "MFA required to join session on prod node (prod role)",
			userClient:      prodAccessClient,
			req:             createReqForNode(prodNode, teleport.SSHSessionJoinPrincipal),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			wantChallenges:  true,
		},
		{
			name:            "MFA required to join session on dev node (prod role)",
			userClient:      prodAccessClient,
			req:             createReqForNode(devNode, teleport.SSHSessionJoinPrincipal),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			wantChallenges:  true,
		},
		{
			name:            "MFA required to join session on prod node (join MFA role)",
			userClient:      joinMFAClient,
			req:             createReqForNode(prodNode, teleport.SSHSessionJoinPrincipal),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			wantChallenges:  true,
		},
		{
			name:            "MFA required to join session dev node (join MFA role)",
			userClient:      joinMFAClient,
			req:             createReqForNode(prodNode, teleport.SSHSessionJoinPrincipal),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_YES,
			wantChallenges:  true,
		},
		{
			name:            "MFA not required to join session, no challenges issued on dev node (join no MFA role)",
			userClient:      joinNoMFAClient,
			req:             createReqForNode(devNode, teleport.SSHSessionJoinPrincipal),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		},
		{
			name:            "MFA not required to join session, no challenges issued on prod node (join no MFA role)",
			userClient:      joinNoMFAClient,
			req:             createReqForNode(prodNode, teleport.SSHSessionJoinPrincipal),
			wantMFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resp, err := test.userClient.CreateAuthenticateChallenge(ctx, test.req)
			require.NoError(t, err, "CreateAuthenticateChallenge")

			assert.Equal(t, test.wantMFARequired, resp.MFARequired, "resp.MFARequired mismatch")

			if test.wantChallenges {
				assert.NotNil(t, resp.GetTOTP(), "resp.TOTP")
				assert.NotNil(t, resp.GetWebauthnChallenge(), "resp.WebauthnChallenge")
			} else {
				assert.Nil(t, resp.GetTOTP(), "resp.TOTP")
				assert.Nil(t, resp.GetWebauthnChallenge(), "resp.WebauthnChallenge")
			}
		})
	}
}

// TestCreateAuthenticateChallenge_failedLoginAudit tests a password+webauthn
// login scenario where the user types the wrong password.
// This should issue a "Local Login Failed" audit event.
func TestCreateAuthenticateChallenge_failedLoginAudit(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	emitter := &eventstest.MockRecorderEmitter{}
	authServer := testServer.Auth()
	authServer.SetEmitter(emitter)

	ctx := context.Background()

	// Set the cluster to require 2nd factor, create the user, set a password and
	// register a webauthn device.
	// password+OTP logins go through another route.
	mfa := configureForMFA(t, testServer)

	// Proxy identity is used during login.
	proxyClient, err := testServer.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err, "NewClient(RoleProxy) failed")

	t.Run("emits audit event", func(t *testing.T) {
		emitter.Reset()
		_, err := proxyClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{
				UserCredentials: &proto.UserCredentials{
					Username: mfa.User,
					Password: []byte(mfa.Password + "BAD"),
				},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
		})
		assert.ErrorContains(t, err, "password", "CreateAuthenticateChallenge error mismatch")

		event := emitter.LastEvent()
		require.NotNil(t, event, "No audit event emitted")
		assert.Equal(t, events.UserLoginEvent, event.GetType(), "event.Type mismatch")
		assert.Equal(t, events.UserLocalLoginFailureCode, event.GetCode(), "event.Code mismatch")
	})
}

func TestCreateRegisterChallenge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	// Test invalid token type.
	wrongToken, err := srv.Auth().createRecoveryToken(ctx, u.username, authclient.UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
	require.NoError(t, err)
	_, err = srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:    wrongToken.GetName(),
		DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
	})
	require.True(t, trace.IsAccessDenied(err))

	// Create a valid token.
	validToken, err := srv.Auth().createRecoveryToken(ctx, u.username, authclient.UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
	require.NoError(t, err)

	// Test unspecified token returns error.
	_, err = srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID: validToken.GetName(),
	})
	require.True(t, trace.IsBadParameter(err))

	tests := []struct {
		name       string
		wantErr    bool
		deviceType proto.DeviceType
	}{
		{
			name:       "totp challenge",
			deviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
		},
		{
			name:       "webauthn challenge",
			deviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
				TokenID:    validToken.GetName(),
				DeviceType: tc.deviceType,
			})
			require.NoError(t, err)

			switch tc.deviceType {
			case proto.DeviceType_DEVICE_TYPE_TOTP:
				require.NotNil(t, res.GetTOTP().GetQRCode())
			case proto.DeviceType_DEVICE_TYPE_WEBAUTHN:
				require.NotNil(t, res.GetWebauthn())
			}
		})
	}

	t.Run("register using context user", func(t *testing.T) {
		authClient, err := srv.NewClient(TestUser(u.username))
		require.NoError(t, err, "NewClient(%q)", u.username)

		// Attempt without a token or a solved authn challenge should fail.
		_, err = authClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_MFA,
		})
		assert.ErrorContains(t, err, "second factor authentication required")

		// Acquire and solve an authn challenge.
		authnChal, err := authClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
				ContextUser: &proto.ContextUser{},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		require.NoError(t, err, "CreateAuthenticateChallenge")
		authnSolved, err := u.webDev.SolveAuthn(authnChal)
		require.NoError(t, err, "SolveAuthn")

		// Attempt with a solved authn challenge should work.
		registerChal, err := authClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			ExistingMFAResponse: authnSolved,
			DeviceType:          proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			DeviceUsage:         proto.DeviceUsage_DEVICE_USAGE_MFA,
		})
		require.NoError(t, err, "CreateRegisterChallenge")
		assert.NotNil(t, registerChal.GetWebauthn(), "CreateRegisterChallenge returned a nil Webauthn challenge")
	})
}

// TestCreateRegisterChallenge_unusableDevice tests that it is possible to
// register new devices even if the user has an "unusable" device (due to
// cluster setting changes).
func TestCreateRegisterChallenge_unusableDevice(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	clock := authServer.GetClock()
	ctx := context.Background()

	initialPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional, // most permissive setting
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err, "NewAuthPreference")

	setAuthPref := func(t *testing.T, authPref types.AuthPreference) {
		_, err = authServer.UpsertAuthPreference(ctx, authPref)
		require.NoError(t, err, "UpsertAuthPreference")
	}
	setAuthPref(t, initialPref)

	tests := []struct {
		name                  string
		existingType, newType proto.DeviceType
		newAuthSpec           types.AuthPreferenceSpecV2
	}{
		{
			name:         "unusable totp, new webauthn",
			existingType: proto.DeviceType_DEVICE_TYPE_TOTP,
			newType:      proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			newAuthSpec: types.AuthPreferenceSpecV2{
				Type:         initialPref.GetType(),
				SecondFactor: constants.SecondFactorWebauthn, // makes TOTP unusable
				Webauthn: func() *types.Webauthn {
					w, _ := initialPref.GetWebauthn()
					return w
				}(),
			},
		},
		{
			name:         "unusable webauthn, new totp",
			existingType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			newType:      proto.DeviceType_DEVICE_TYPE_TOTP,
			newAuthSpec: types.AuthPreferenceSpecV2{
				Type:         initialPref.GetType(),
				SecondFactor: constants.SecondFactorOTP, // makes Webauthn unusable
			},
		},
	}

	devOpts := []TestDeviceOpt{WithTestDeviceClock(clock)}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setAuthPref(t, initialPref) // restore permissive settings.

			// Create user.
			username := fmt.Sprintf("llama-%d", i)
			user, _, err := CreateUserAndRole(authServer, username, []string{username} /* logins */, nil /* allowRules */)
			require.NoError(t, err, "CreateUserAndRole")
			userClient, err := testServer.NewClient(TestUser(user.GetName()))
			require.NoError(t, err, "NewClient")

			// Register initial MFA device.
			_, err = RegisterTestDevice(
				ctx,
				userClient,
				"existing", test.existingType, nil /* authenticator */, devOpts...)
			require.NoError(t, err, "RegisterTestDevice")

			// Sanity check: register challenges for test.existingType require a
			// solved authn challenge.
			_, err = userClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
				ExistingMFAResponse: &proto.MFAAuthenticateResponse{},
				DeviceType:          test.existingType,
				DeviceUsage:         proto.DeviceUsage_DEVICE_USAGE_MFA, // not important for this test
			})
			assert.ErrorContains(t, err, "second factor")

			// Restore initial settings after test.
			defer func() {
				setAuthPref(t, initialPref)
			}()

			// Change cluster settings.
			// This should make the device registered above unusable.
			newAuthPref, err := types.NewAuthPreference(test.newAuthSpec)
			require.NoError(t, err, "NewAuthPreference")
			setAuthPref(t, newAuthPref)

			// Create a challenge for the "new" device without an ExistingMFAResponse.
			// Not allowed if the device above was usable.
			_, err = userClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
				ExistingMFAResponse: &proto.MFAAuthenticateResponse{},
				DeviceType:          test.newType,
				DeviceUsage:         proto.DeviceUsage_DEVICE_USAGE_MFA, // not important for this test
			})
			assert.NoError(t, err, "CreateRegisterChallenge")
		})
	}
}

// sshPubKey is a randomly-generated public key used for login tests.
//
// The corresponding private key is:
// -----BEGIN PRIVATE KEY-----
// MHcCAQEEIAKuZeB4WL4KAl5cnCrMYBy3kAX9qHt/g6OAbGGd7f3VoAoGCCqGSM49
// AwEHoUQDQgAEa/6A3YLbc/TyJ4lED2BT8iThuw6HcrDX3dRixwkPDjWYBOP4qrJ/
// jlGaPwXyuzeLuZgpFde7UiM1EHM2ClfGpw==
// -----END PRIVATE KEY-----
const sshPubKey = `ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBGv+gN2C23P08ieJRA9gU/Ik4bsOh3Kw193UYscJDw41mATj+Kqyf45Rmj8F8rs3i7mYKRXXu1IjNRBzNgpXxqc=`

// tlsPubKey is a randomly-generated public key used for login tests.
//
// The corresponding private key is:
// -----BEGIN EC PRIVATE KEY-----
// MHcCAQEEINmdcjzor3czsAVpSYFJCjs/623gDfMcFE2AIcGTYZARoAoGCCqGSM49
// AwEHoUQDQgAE/Jn3tYhc60M2IOen1yRht6r8xX3hv7nNLYBIfxaKxXf+dAFVllYz
// VUrSzAQxi1LSAplOJVgOtHv0J69dRSUSzA==
// -----END EC PRIVATE KEY-----
const tlsPubKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE/Jn3tYhc60M2IOen1yRht6r8xX3h
v7nNLYBIfxaKxXf+dAFVllYzVUrSzAQxi1LSAplOJVgOtHv0J69dRSUSzA==
-----END PUBLIC KEY-----`

func TestServer_AuthenticateUser_passwordOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()

	const username = "bowman"
	const password = "it's full of stars!"
	_, _, err := CreateUserAndRole(authServer, username, nil, nil)
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertPassword(username, []byte(password)))

	// makeRun is used to test both SSH and Web login by switching the
	// authenticate function.
	makeRun := func(authenticate func(*Server, authclient.AuthenticateUserRequest) error) func(t *testing.T) {
		return func(t *testing.T) {
			require.NoError(t, authenticate(authServer, authclient.AuthenticateUserRequest{
				Username: "bowman",
				Pass:     &authclient.PassCreds{Password: []byte("it's full of stars!")},
			}))
		}
	}
	t.Run("ssh", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
		req.SSHPublicKey = []byte(sshPubKey)
		req.TLSPublicKey = []byte(tlsPubKey)
		_, err := s.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
			AuthenticateUserRequest: req,
			TTL:                     24 * time.Hour,
		})
		return err
	}))
	t.Run("web", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
		_, err := s.AuthenticateWebUser(ctx, req)
		return err
	}))
}

func TestServer_AuthenticateUser_passwordOnly_failure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()

	const username = "capybara"

	setPassword := func(pwd string) func(*testing.T, *Server) {
		return func(t *testing.T, s *Server) {
			require.NoError(t, s.UpsertPassword(username, []byte(pwd)))
		}
	}

	tests := []struct {
		name         string
		setup        func(*testing.T, *Server)
		authUser     string
		authPassword string
	}{
		{
			name:         "wrong password",
			setup:        setPassword("secure password"),
			authUser:     username,
			authPassword: "wrong password",
		},
		{
			name:         "user not found",
			setup:        setPassword("secure password"),
			authUser:     "unknown",
			authPassword: "secure password",
		},
		{
			name:         "password not found",
			setup:        func(*testing.T, *Server) {},
			authUser:     username,
			authPassword: "secure password",
		},
	}

	for _, test := range tests {
		// makeRun is used to test both SSH and Web login by switching the
		// authenticate function.
		makeRun := func(authenticate func(*Server, authclient.AuthenticateUserRequest) error) func(t *testing.T) {
			return func(t *testing.T) {
				_, _, err := CreateUserAndRole(authServer, username, nil, nil)
				require.NoError(t, err)
				t.Cleanup(func() {
					assert.NoError(t, authServer.DeleteUser(ctx, username), "failed to delete user %s", username)
				})
				test.setup(t, authServer)

				err = authenticate(authServer, authclient.AuthenticateUserRequest{
					Username: test.authUser,
					Pass:     &authclient.PassCreds{Password: []byte(test.authPassword)},
				})
				assert.Error(t, err)
				assert.True(t, trace.IsAccessDenied(err), "got %T: %v, want AccessDenied", trace.Unwrap(err), err)
			}
		}
		t.Run(test.name+"/ssh", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
			req.SSHPublicKey = []byte(sshPubKey)
			req.TLSPublicKey = []byte(tlsPubKey)
			_, err := s.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
				AuthenticateUserRequest: req,
				TTL:                     24 * time.Hour,
			})
			return err
		}))
		t.Run(test.name+"/web", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
			_, err := s.AuthenticateWebUser(ctx, req)
			return err
		}))
	}
}

func TestServer_AuthenticateUser_setsPasswordState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()

	const username = "bowman"
	const password = "it's full of stars!"
	_, _, err := CreateUserAndRole(authServer, username, nil, nil)
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertPassword(username, []byte(password)))

	// makeRun is used to test both SSH and Web login by switching the
	// authenticate function.
	makeRun := func(authenticate func(*Server, authclient.AuthenticateUserRequest) error) func(t *testing.T) {
		return func(t *testing.T) {
			// Enforce unspecified password state.
			u, err := authServer.Identity.UpdateAndSwapUser(ctx, username, false, /* withSecrets */
				func(u types.User) (bool, error) {
					u.SetPasswordState(types.PasswordState_PASSWORD_STATE_UNSPECIFIED)
					return true, nil
				})
			require.NoError(t, err)
			assert.Equal(t, types.PasswordState_PASSWORD_STATE_UNSPECIFIED, u.GetPasswordState())

			// Finish login - either SSH or Web
			require.NoError(t, authenticate(authServer, authclient.AuthenticateUserRequest{
				Username: "bowman",
				Pass: &authclient.PassCreds{
					Password: []byte("it's full of stars!"),
				},
			}))

			// Verify that the password state has been changed.
			u, err = authServer.GetUser(ctx, username, false /* withSecrets */)
			require.NoError(t, err)
			assert.Equal(t, types.PasswordState_PASSWORD_STATE_SET, u.GetPasswordState())
		}
	}
	t.Run("ssh", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
		req.SSHPublicKey = []byte(sshPubKey)
		req.TLSPublicKey = []byte(tlsPubKey)
		_, err := s.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
			AuthenticateUserRequest: req,
			TTL:                     24 * time.Hour,
		})
		return err
	}))
	t.Run("web", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
		_, err := s.AuthenticateWebUser(ctx, req)
		return err
	}))
}

func TestServer_AuthenticateUser_mfaDevices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svr := newTestTLSServer(t)
	authServer := svr.Auth()
	mfa := configureForMFA(t, svr)
	username := mfa.User
	password := mfa.Password

	tests := []struct {
		name           string
		solveChallenge func(*proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)
	}{
		{name: "OK TOTP device", solveChallenge: mfa.TOTPDev.SolveAuthn},
		{name: "OK Webauthn device", solveChallenge: mfa.WebDev.SolveAuthn},
	}
	for _, test := range tests {
		// makeRun is used to test both SSH and Web login by switching the
		// authenticate function.
		makeRun := func(authenticate func(*Server, authclient.AuthenticateUserRequest) error) func(t *testing.T) {
			return func(t *testing.T) {
				// 1st step: acquire challenge
				challenge, err := authServer.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
					Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
						Username: username,
						Password: []byte(password),
					}},
				})
				require.NoError(t, err)

				// Solve challenge (client-side)
				resp, err := test.solveChallenge(challenge)
				authReq := authclient.AuthenticateUserRequest{
					Username:     username,
					SSHPublicKey: []byte(sshPubKey),
					TLSPublicKey: []byte(tlsPubKey),
				}
				require.NoError(t, err)

				switch {
				case resp.GetWebauthn() != nil:
					authReq.Webauthn = wantypes.CredentialAssertionResponseFromProto(resp.GetWebauthn())
				case resp.GetTOTP() != nil:
					authReq.OTP = &authclient.OTPCreds{
						Password: []byte(password),
						Token:    resp.GetTOTP().Code,
					}
				default:
					t.Fatalf("Unexpected solved challenge type: %T", resp.Response)
				}

				// 2nd step: finish login - either SSH or Web
				require.NoError(t, authenticate(authServer, authReq))
			}
		}
		t.Run(test.name+"/ssh", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
			_, err := s.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
				AuthenticateUserRequest: req,
				TTL:                     24 * time.Hour,
			})
			return err
		}))
		t.Run(test.name+"/web", makeRun(func(s *Server, req authclient.AuthenticateUserRequest) error {
			_, err := s.AuthenticateWebUser(ctx, req)
			return err
		}))
	}
}

func TestServer_Authenticate_passwordless(t *testing.T) {
	t.Parallel()
	svr := newTestTLSServer(t)
	authServer := svr.Auth()

	// Configure Auth separately, we want a passwordless-capable device
	// registered too.
	ctx := context.Background()
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = authServer.UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	// Create user and initial WebAuthn device (MFA).
	const user = "llama"
	const password = "p@ssw0rd1234"
	_, _, err = CreateUserAndRole(authServer, user, []string{"llama", "root"}, nil)
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertPassword(user, []byte(password)))
	userClient, err := svr.NewClient(TestUser(user))
	require.NoError(t, err)
	webDev, err := RegisterTestDevice(
		ctx, userClient, "web", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err)

	// Acquire a privilege token so we can register a passwordless device
	// synchronously.
	mfaChallenge, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{}, // already authenticated
		},
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	require.NoError(t, err)
	mfaResp, err := webDev.SolveAuthn(mfaChallenge)
	require.NoError(t, err)
	token, err := userClient.CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{
		ExistingMFAResponse: mfaResp,
	})
	require.NoError(t, err)

	// Register passwordless device.
	registerChallenge, err := userClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:     token.GetName(),
		DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	require.NoError(t, err, "Failed to create passwordless registration challenge")
	pwdKey, err := mocku2f.Create()
	require.NoError(t, err)
	pwdKey.SetPasswordless()
	const origin = "https://localhost"
	ccr, err := pwdKey.SignCredentialCreation(origin, wantypes.CredentialCreationFromProto(registerChallenge.GetWebauthn()))
	require.NoError(t, err)
	_, err = userClient.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
		TokenID:       token.GetName(),
		NewDeviceName: "pwdless1",
		NewMFAResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
			},
		},
	})
	require.NoError(t, err, "Failed to register passwordless device")

	// Use a proxy client for now on; the user's identity isn't established yet.
	proxyClient, err := svr.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	// used to keep track of calls to login hooks.
	var loginHookCounter atomic.Int32
	var loginHook LoginHook = func(_ context.Context, _ types.User) error {
		loginHookCounter.Add(1)
		return nil
	}

	tests := []struct {
		name         string
		loginHooks   []LoginHook
		authenticate func(t *testing.T, resp *wantypes.CredentialAssertionResponse)
	}{
		{
			name: "ssh",
			authenticate: func(t *testing.T, resp *wantypes.CredentialAssertionResponse) {
				loginResp, err := proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
					AuthenticateUserRequest: authclient.AuthenticateUserRequest{
						Webauthn:     resp,
						SSHPublicKey: []byte(sshPubKey),
						TLSPublicKey: []byte(tlsPubKey),
					},
					TTL: 24 * time.Hour,
				})
				require.NoError(t, err, "Failed to perform passwordless authentication")
				require.NotNil(t, loginResp, "SSH response nil")
				require.NotEmpty(t, loginResp.Cert, "SSH certificate empty")
				require.Equal(t, user, loginResp.Username, "Unexpected username")
			},
		},
		{
			name: "ssh with login hooks",
			loginHooks: []LoginHook{
				loginHook,
				loginHook,
			},
			authenticate: func(t *testing.T, resp *wantypes.CredentialAssertionResponse) {
				loginResp, err := proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
					AuthenticateUserRequest: authclient.AuthenticateUserRequest{
						Webauthn:     resp,
						SSHPublicKey: []byte(sshPubKey),
						TLSPublicKey: []byte(tlsPubKey),
					},
					TTL: 24 * time.Hour,
				})
				require.NoError(t, err, "Failed to perform passwordless authentication")
				require.NotNil(t, loginResp, "SSH response nil")
				require.NotEmpty(t, loginResp.Cert, "SSH certificate empty")
				require.Equal(t, user, loginResp.Username, "Unexpected username")
			},
		},
		{
			name: "web",
			authenticate: func(t *testing.T, resp *wantypes.CredentialAssertionResponse) {
				session, err := proxyClient.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
					Webauthn: resp,
				})
				require.NoError(t, err, "Failed to perform passwordless authentication")
				require.Equal(t, user, session.GetUser(), "Unexpected username")
			},
		},
		{
			name: "web with login hooks",
			loginHooks: []LoginHook{
				loginHook,
			},
			authenticate: func(t *testing.T, resp *wantypes.CredentialAssertionResponse) {
				session, err := proxyClient.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
					Webauthn: resp,
				})
				require.NoError(t, err, "Failed to perform passwordless authentication")
				require.Equal(t, user, session.GetUser(), "Unexpected username")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svr.Auth().ResetLoginHooks()
			loginHookCounter.Store(0)
			for _, hook := range test.loginHooks {
				svr.Auth().RegisterLoginHook(hook)
			}

			// Fail a login attempt so have a non-empty list of attempts.
			_, err := proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
				AuthenticateUserRequest: authclient.AuthenticateUserRequest{
					Username:     user,
					Webauthn:     &wantypes.CredentialAssertionResponse{}, // bad response
					SSHPublicKey: []byte(sshPubKey),
					TLSPublicKey: []byte(tlsPubKey),
				},
				TTL: 24 * time.Hour,
			})
			require.True(t, trace.IsAccessDenied(err), "got err = %v, want AccessDenied", err)
			attempts, err := authServer.GetUserLoginAttempts(user)
			require.NoError(t, err)
			require.NotEmpty(t, attempts, "Want at least one failed login attempt")

			// Create a passwordless challenge.
			mfaChallenge, err := proxyClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_Passwordless{
					Passwordless: &proto.Passwordless{},
				},
			})
			require.NoError(t, err, "Failed to create passwordless challenge")

			// Sign challenge (mocks user interaction).
			assertionResp, err := pwdKey.SignAssertion(origin, wantypes.CredentialAssertionFromProto(mfaChallenge.GetWebauthnChallenge()))
			require.NoError(t, err)

			// Complete login procedure (SSH or Web).
			test.authenticate(t, assertionResp)

			// Verify zeroed login attempts. This is a proxy for various other user
			// checks (locked, etc).
			attempts, err = authServer.GetUserLoginAttempts(user)
			require.NoError(t, err)
			require.Empty(t, attempts, "Login attempts not reset")

			require.Len(t, test.loginHooks, int(loginHookCounter.Load()))
		})
	}
}

// TestPasswordlessProhibitedForSSO tests a scenario where an SSO user bypasses
// SSO by using a passwordless login.
func TestPasswordlessProhibitedForSSO(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	clock := testServer.Clock()

	mfa := configureForMFA(t, testServer)
	ctx := context.Background()

	// Register a passwordless device.
	userClient, err := testServer.NewClient(TestUser(mfa.User))
	require.NoError(t, err, "NewClient failed")
	pwdlessDev, err := RegisterTestDevice(ctx, userClient, "pwdless", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, mfa.WebDev, WithPasswordless())
	require.NoError(t, err, "RegisterTestDevice failed")

	// Edit the configured user so it looks like an SSO user attempting local
	// logins. This isn't exactly like an SSO user, but it's close enough.
	_, err = authServer.UpdateAndSwapUser(ctx, mfa.User, true /* withSecrets */, func(user types.User) (changed bool, err error) {
		user.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type:     constants.Github,
				ID:       "github",
				Identity: mfa.User,
			},
			Time: clock.Now(),
			User: types.UserRef{
				Name: teleport.UserSystem,
			},
		})
		return true, nil
	})
	require.NoError(t, err, "UpdateAndSwapUser failed")

	// Authentication happens through the Proxy identity.
	proxyClient, err := testServer.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	tests := []struct {
		name         string
		authenticate func(*wantypes.CredentialAssertionResponse) error
	}{
		{
			name: "web not allowed",
			authenticate: func(car *wantypes.CredentialAssertionResponse) error {
				_, err := proxyClient.AuthenticateWebUser(ctx, authclient.AuthenticateUserRequest{
					Webauthn: car,
				})
				return err
			},
		},
		{
			name: "ssh not allowed",
			authenticate: func(car *wantypes.CredentialAssertionResponse) error {
				_, err := proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
					AuthenticateUserRequest: authclient.AuthenticateUserRequest{
						SSHPublicKey: []byte(sshPubKey),
						TLSPublicKey: []byte(tlsPubKey),
						Webauthn:     car,
					},
					TTL: 12 * time.Hour,
				})
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chal, err := proxyClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_Passwordless{
					Passwordless: &proto.Passwordless{},
				},
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_PASSWORDLESS_LOGIN,
				},
			})
			require.NoError(t, err, "CreateAuthenticateChallenge failed")

			chalResp, err := pwdlessDev.SolveAuthn(chal)
			require.NoError(t, err, "SolveAuthn failed")

			err = test.authenticate(wantypes.CredentialAssertionResponseFromProto(chalResp.GetWebauthn()))
			assert.ErrorIs(t, err, types.ErrPassswordlessLoginBySSOUser, "authentication error mismatch")
		})
	}
}

// TestSSOPasswordBypass verifies that SSO users can't login using passwords
// (aka local logins) or set passwords through various methods.
// Complements TestPasswordlessProhibitedForSSO.
func TestSSOPasswordBypass(t *testing.T) {
	t.Parallel()

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	clock := testServer.Clock()

	mfa := configureForMFA(t, testServer)
	ctx := context.Background()

	// Edit the configured user so it looks like an SSO user attempting local
	// logins. This isn't exactly like an SSO user, but it's close enough.
	_, err := authServer.UpdateAndSwapUser(ctx, mfa.User, true /* withSecrets */, func(user types.User) (changed bool, err error) {
		user.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type:     constants.Github,
				ID:       "github",
				Identity: mfa.User,
			},
			Time: clock.Now(),
			User: types.UserRef{
				Name: teleport.UserSystem,
			},
		})
		return true, nil
	})
	require.NoError(t, err, "UpdateAndSwapUser failed")

	// Because of configureForMFA the user has a password set. We'll assume for
	// this test that they *somehow* managed to get a password into their SSO user
	// and proceed from there.

	// Authentication happens through the Proxy identity.
	proxyClient, err := testServer.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	createAuthenticateChallenge := func(t *testing.T) *proto.MFAAuthenticateChallenge {
		chal, err := proxyClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{
				UserCredentials: &proto.UserCredentials{
					Username: mfa.User,
					Password: []byte(mfa.Password),
				},
			},
			ChallengeExtensions: &mfav1.ChallengeExtensions{},
		})
		require.NoError(t, err, "CreateAuthenticateChallenge failed")
		return chal
	}

	solveWebauthn := func(t *testing.T, req *authclient.AuthenticateSSHRequest) {
		chal := createAuthenticateChallenge(t)
		mfaResp, err := mfa.WebDev.SolveAuthn(chal)
		require.NoError(t, err, "SolveAuthn failed")

		req.Webauthn = wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn())
	}

	const wantError = "invalid credentials"

	// Verify local login methods.
	tests := []struct {
		name string
		// setSecondFactor sets the OTP, Webauthn or Pass request field.
		setSecondFactor func(t *testing.T, req *authclient.AuthenticateSSHRequest)
		// authenticateOverride may be used to change the authenticate function from
		// proxyClient.AuthenticateSSHUser to something else (eg,
		// proxyClient.AuthenticateWebUser).
		// Optional.
		authenticateOverride func(context.Context, authclient.AuthenticateSSHRequest) (*authclient.SSHLoginResponse, error)
	}{
		{
			name: "OTP",
			setSecondFactor: func(t *testing.T, req *authclient.AuthenticateSSHRequest) {
				chal := createAuthenticateChallenge(t)
				mfaResp, err := mfa.TOTPDev.SolveAuthn(chal)
				require.NoError(t, err, "SolveAuthn failed")

				req.OTP = &authclient.OTPCreds{
					Password: []byte(mfa.Password),
					Token:    mfaResp.GetTOTP().GetCode(),
				}
			},
		},
		{
			name:            "WebAuthn",
			setSecondFactor: solveWebauthn,
		},
		{
			name:            "AuthenticateWeb",
			setSecondFactor: solveWebauthn,
			authenticateOverride: func(ctx context.Context, req authclient.AuthenticateSSHRequest) (*authclient.SSHLoginResponse, error) {
				// We only care about the error here, it's OK to swallow the session.
				_, err := proxyClient.AuthenticateWebUser(ctx, req.AuthenticateUserRequest)
				return nil, err
			},
		},
		{
			name: "password only",
			setSecondFactor: func(t *testing.T, req *authclient.AuthenticateSSHRequest) {
				beforePref, err := authServer.GetAuthPreference(ctx)
				require.NoError(t, err, "GetAuthPreference")

				// Disable second factors.
				authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOff,
				})
				require.NoError(t, err, "NewAuthPreference failed")
				_, err = authServer.UpsertAuthPreference(ctx, authPref)
				require.NoError(t, err, "UpdateAuthPreference failed")

				// Reset after test.
				t.Cleanup(func() {
					_, err := authServer.UpsertAuthPreference(ctx, beforePref)
					assert.NoError(t, err, "UpsertAuthPreference failed, AuthPreference not restored")
				})

				// Password-only auth.
				req.Pass = &authclient.PassCreds{
					Password: []byte(mfa.Password),
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := authclient.AuthenticateSSHRequest{
				AuthenticateUserRequest: authclient.AuthenticateUserRequest{
					Username:     mfa.User,
					SSHPublicKey: []byte(sshPubKey),
					TLSPublicKey: []byte(tlsPubKey),
					// test.setSecondFactor sets either Pass, OTP or Webauthn.
				},
				TTL: 12 * time.Hour,
			}
			test.setSecondFactor(t, &req)

			authenticate := test.authenticateOverride
			if authenticate == nil {
				authenticate = proxyClient.AuthenticateSSHUser
			}

			_, err := authenticate(ctx, req)
			assert.True(t,
				trace.IsAccessDenied(err),
				"AuthenticateSSHUser returned err=%v (%T), want AccessDenied", err, trace.Unwrap(err))
			assert.ErrorContains(t, err, wantError, "AuthenticateSSHUser error mismatch")
		})
	}

	// Test that reset and password changes are not allowed.

	t.Run("ChangePassword", func(t *testing.T) {
		t.Parallel()

		chal := createAuthenticateChallenge(t)
		mfaResp, err := mfa.TOTPDev.SolveAuthn(chal)
		require.NoError(t, err, "SolveAuthn failed")

		userClient, err := testServer.NewClient(TestUser(mfa.User))
		require.NoError(t, err, "NewClient failed")

		err = userClient.ChangePassword(ctx, &proto.ChangePasswordRequest{
			User:              mfa.User,
			OldPassword:       []byte(mfa.Password),
			NewPassword:       []byte(mfa.Password + "NEW"),
			SecondFactorToken: mfaResp.GetTOTP().GetCode(),
		})
		assert.ErrorContains(t, err, wantError, "ChangePassword error mismatch")
	})

	t.Run("CreateResetPasswordToken", func(t *testing.T) {
		t.Parallel()

		adminClient, err := testServer.NewClient(TestBuiltin(types.RoleAdmin))
		require.NoError(t, err, "NewClient failed")

		_, err = adminClient.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
			Name: mfa.User,
			TTL:  1 * time.Hour,
			Type: authclient.UserTokenTypeResetPassword,
		})
		assert.ErrorContains(t, err, "only local", "CreateResetPasswordToken error mismatch")
	})
}

// TestSSOAccountRecoveryProhibited tests that SSO users cannot perform account
// recovery.
func TestSSOAccountRecoveryProhibited(t *testing.T) {
	// Can't t.Parallel because of modules.SetTestModules.

	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()
	clock := testServer.Clock()
	ctx := context.Background()

	// Enable RecoveryCodes feature.
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{
			RecoveryCodes: true,
		},
	})

	// Make second factor mandatory.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err, "NewAuthPreference failed")
	_, err = authServer.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err, "UpsertAuthPreference failed")

	// Create a user that looks like an SSO user. The name must be an e-mail for
	// account recovery to work.
	const user = "llama@example.com"
	_, _, err = CreateUserAndRole(authServer, user, []string{user}, nil /* allowRules */)
	require.NoError(t, err, "CreateUserAndRole failed")
	_, err = authServer.UpdateAndSwapUser(ctx, user, false /* withSecrets */, func(u types.User) (changed bool, err error) {
		u.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type:     constants.Github,
				ID:       "github",
				Identity: user,
			},
			Time: clock.Now(),
			User: types.UserRef{
				Name: teleport.UserSystem,
			},
		})
		return true, nil
	})
	require.NoError(t, err, "UpdateAndSwapUser failed")

	// Register an MFA device.
	userClient, err := testServer.NewClient(TestUser(user))
	require.NoError(t, err, "NewClient failed")
	totpDev, err := RegisterTestDevice(ctx, userClient, "totp", proto.DeviceType_DEVICE_TYPE_TOTP, nil /* authenticator */, WithTestDeviceClock(clock))
	require.NoError(t, err, "RegisterTestDevice failed")

	t.Run("CreateAccountRecoveryCodes", func(t *testing.T) {
		t.Parallel()

		// Issue a privilege token
		chal, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
			Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{},
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
			},
		})
		require.NoError(t, err, "CreateAuthenticateChallenge failed")
		mfaResp, err := totpDev.SolveAuthn(chal)
		require.NoError(t, err, "SolveAuthn failed")
		privilegeToken, err := userClient.CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{
			ExistingMFAResponse: mfaResp,
		})
		require.NoError(t, err, "CreatePrivilegeToken failed")

		_, err = userClient.CreateAccountRecoveryCodes(ctx, &proto.CreateAccountRecoveryCodesRequest{
			TokenID: privilegeToken.GetName(),
		})
		assert.ErrorContains(t, err, "only local", "CreateAccountRecoveryCodes error mismatch")
	})

	t.Run("StartAccountRecovery", func(t *testing.T) {
		t.Parallel()

		// Verify that we cannot start account recovery.
		_, err = userClient.StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{
			Username:     user,
			RecoveryCode: []byte("tele-aardvark-adviser-accrue-aggregate-adrift-almighty-afflict-amusement"), // fake
			RecoverType:  types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD,
		})
		assert.ErrorContains(t, err, "only local", "StartAccountRecovery error mismatch")
	})
}

func TestServer_Authenticate_nonPasswordlessRequiresUsername(t *testing.T) {
	t.Parallel()
	svr := newTestTLSServer(t)

	// We don't mind about the specifics of the configuration, as long as we have
	// a user and TOTP/WebAuthn devices.
	mfa := configureForMFA(t, svr)
	username := mfa.User
	password := mfa.Password

	userClient, err := svr.NewClient(TestUser(username))
	require.NoError(t, err)
	proxyClient, err := svr.NewClient(TestBuiltin(types.RoleProxy))
	require.NoError(t, err)

	ctx := context.Background()
	tests := []struct {
		name    string
		dev     *TestDevice
		wantErr string
	}{
		{
			name:    "OTP",
			dev:     mfa.TOTPDev,
			wantErr: "username", // Error contains "username"
		},
		{
			name:    "WebAuthn",
			dev:     mfa.WebDev,
			wantErr: "invalid credentials", // generic error as it _could_ be a passwordless attempt
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mfaChallenge, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
					ContextUser: &proto.ContextUser{},
				},
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
			})
			require.NoError(t, err)

			mfaResp, err := test.dev.SolveAuthn(mfaChallenge)
			require.NoError(t, err)

			req := authclient.AuthenticateUserRequest{
				SSHPublicKey: []byte(sshPubKey),
				TLSPublicKey: []byte(tlsPubKey),
			}
			switch {
			case mfaResp.GetWebauthn() != nil:
				req.Webauthn = wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn())
			case mfaResp.GetTOTP() != nil:
				req.OTP = &authclient.OTPCreds{
					Password: []byte(password),
					Token:    mfaResp.GetTOTP().Code,
				}
			}

			// SSH.
			_, err = proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
				AuthenticateUserRequest: req,
				TTL:                     24 * time.Hour,
			})
			require.Error(t, err, "SSH authentication expected fail (missing username)")
			require.Contains(t, err.Error(), test.wantErr)

			// Web.
			_, err = proxyClient.AuthenticateWebUser(ctx, req)
			require.Error(t, err, "Web authentication expected fail (missing username)")
			require.Contains(t, err.Error(), test.wantErr)

			// Get one right so we don't lock the user between tests.
			req.Username = username
			_, err = proxyClient.AuthenticateWebUser(ctx, req)
			require.NoError(t, err, "Web authentication expected to succeed")
		})
	}
}

func TestServer_Authenticate_headless(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	headlessID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))

	for _, tc := range []struct {
		name        string
		timeout     time.Duration
		update      func(*types.HeadlessAuthentication, *types.MFADevice)
		assertError require.ErrorAssertionFunc
	}{
		{
			name:    "OK approved",
			timeout: 10 * time.Second,
			update: func(ha *types.HeadlessAuthentication, mfa *types.MFADevice) {
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
				ha.MfaDevice = mfa
			},
			assertError: require.NoError,
		}, {
			name:    "NOK approved without MFA",
			timeout: 10 * time.Second,
			update: func(ha *types.HeadlessAuthentication, mfa *types.MFADevice) {
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		}, {
			name:    "NOK denied",
			timeout: 10 * time.Second,
			update: func(ha *types.HeadlessAuthentication, mfa *types.MFADevice) {
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED
			},
			assertError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		}, {
			name:    "NOK timeout",
			timeout: 100 * time.Millisecond,
			assertError: func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			srv := newTestTLSServer(t)
			proxyClient, err := srv.NewClient(TestBuiltin(types.RoleProxy))
			require.NoError(t, err)

			// We don't mind about the specifics of the configuration, as long as we have
			// a user and TOTP/WebAuthn devices.
			mfa := configureForMFA(t, srv)
			username := mfa.User

			ctx, cancel := context.WithTimeout(ctx, tc.timeout)
			defer cancel()

			// Start a goroutine to catch the headless authentication attempt and update with test case values.
			errC := make(chan error)
			go func() {
				defer close(errC)

				if tc.update == nil {
					return
				}

				err := srv.Auth().UpsertHeadlessAuthenticationStub(ctx, username)
				if err != nil {
					errC <- err
					return
				}

				headlessAuthn, err := srv.Auth().GetHeadlessAuthenticationFromWatcher(ctx, username, headlessID)
				if err != nil {
					errC <- err
					return
				}

				// create a shallow copy and update for the compare and swap below.
				replaceHeadlessAuthn := *headlessAuthn
				tc.update(&replaceHeadlessAuthn, mfa.WebDev.MFA)

				if _, err = srv.Auth().CompareAndSwapHeadlessAuthentication(ctx, headlessAuthn, &replaceHeadlessAuthn); err != nil {
					errC <- err
					return
				}
			}()

			_, err = proxyClient.AuthenticateSSHUser(ctx, authclient.AuthenticateSSHRequest{
				AuthenticateUserRequest: authclient.AuthenticateUserRequest{
					// HeadlessAuthenticationID should take precedence over WebAuthn and OTP fields.
					HeadlessAuthenticationID: headlessID,
					Webauthn:                 &wantypes.CredentialAssertionResponse{},
					OTP:                      &authclient.OTPCreds{},
					Username:                 username,
					SSHPublicKey:             []byte(sshPubKey),
					TLSPublicKey:             []byte(tlsPubKey),
					ClientMetadata: &authclient.ForwardedClientMetadata{
						RemoteAddr: "0.0.0.0",
					},
				},
				TTL: defaults.HeadlessLoginTimeout,
			})

			// Use assert so that we also output any test failures below.
			assert.NoError(t, <-errC, "Failed to get and update headless authentication in background")

			tc.assertError(t, err, trace.DebugReport(err))
		})
	}
}

type configureMFAResp struct {
	User, Password  string
	TOTPDev, WebDev *TestDevice
}

func configureForMFA(t *testing.T, srv *TestTLSServer) *configureMFAResp {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)

	authServer := srv.Auth()
	ctx := context.Background()
	_, err = authServer.UpsertAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	// Create user with a default password.
	const username = "llama@goteleport.com"
	const password = "supersecurepass"
	_, _, err = CreateUserAndRole(authServer, username, []string{"llama", "root"}, nil)
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertPassword(username, []byte(password)))

	clt, err := srv.NewClient(TestUser(username))
	require.NoError(t, err)

	totpDev, err := RegisterTestDevice(ctx, clt, "totp-1", proto.DeviceType_DEVICE_TYPE_TOTP, nil, WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	webDev, err := RegisterTestDevice(ctx, clt, "web-1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, totpDev)
	require.NoError(t, err)

	return &configureMFAResp{
		User:     username,
		Password: password,
		TOTPDev:  totpDev,
		WebDev:   webDev,
	}
}
