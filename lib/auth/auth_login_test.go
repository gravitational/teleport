// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func TestServer_CreateAuthenticateChallenge_authPreference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name            string
		spec            *types.AuthPreferenceSpecV2
		assertChallenge func(*proto.MFAAuthenticateChallenge)
	}{
		{
			name: "OK second_factor:off",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOff,
			},
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
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:webauthn (standalone)",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				Webauthn: &types.Webauthn{
					RPID: "localhost",
				},
			},
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
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			svr := newTestTLSServer(t)
			authServer := svr.Auth()
			mfa := configureForMFA(t, svr)
			username := mfa.User
			password := mfa.Password

			authPreference, err := types.NewAuthPreference(*test.spec)
			require.NoError(t, err)
			require.NoError(t, authServer.SetAuthPreference(ctx, authPreference))

			challenge, err := authServer.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{
					UserCredentials: &proto.UserCredentials{
						Username: username,
						Password: []byte(password),
					},
				},
			})
			require.NoError(t, err)
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

	res, err := clt.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{})
	require.NoError(t, err)

	// MFA authentication works.
	// TODO(codingllama): Use a public endpoint to verify?
	mfaResp, err := u.webDev.SolveAuthn(res)
	require.NoError(t, err)
	_, _, err = srv.Auth().validateMFAAuthResponse(ctx, mfaResp, u.username, false /* passwordless */)
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
		tc := tc
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
			require.Equal(t, err.Error(), MaxFailedAttemptsErrMsg)
		} else {
			require.NotEqual(t, err.Error(), MaxFailedAttemptsErrMsg)
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
				wrongToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
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
				startToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
				require.NoError(t, err)

				return &proto.CreateAuthenticateChallengeRequest{
					Request: &proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID{RecoveryStartTokenID: startToken.GetName()},
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
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

func TestCreateRegisterChallenge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	u, err := createUserWithSecondFactors(srv)
	require.NoError(t, err)

	// Test invalid token type.
	wrongToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryStart, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
	require.NoError(t, err)
	_, err = srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:    wrongToken.GetName(),
		DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
	})
	require.True(t, trace.IsAccessDenied(err))

	// Create a valid token.
	validToken, err := srv.Auth().createRecoveryToken(ctx, u.username, UserTokenTypeRecoveryApproved, types.UserTokenUsage_USER_TOKEN_RECOVER_PASSWORD)
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
		tc := tc
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
		test := test
		// makeRun is used to test both SSH and Web login by switching the
		// authenticate function.
		makeRun := func(authenticate func(*Server, AuthenticateUserRequest) error) func(t *testing.T) {
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
				authReq := AuthenticateUserRequest{
					Username:  username,
					PublicKey: []byte(sshPubKey),
				}
				require.NoError(t, err)

				switch {
				case resp.GetWebauthn() != nil:
					authReq.Webauthn = wanlib.CredentialAssertionResponseFromProto(resp.GetWebauthn())
				case resp.GetTOTP() != nil:
					authReq.OTP = &OTPCreds{
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
		t.Run(test.name+"/ssh", makeRun(func(s *Server, req AuthenticateUserRequest) error {
			_, err := s.AuthenticateSSHUser(ctx, AuthenticateSSHRequest{
				AuthenticateUserRequest: req,
				TTL:                     24 * time.Hour,
			})
			return err
		}))
		t.Run(test.name+"/web", makeRun(func(s *Server, req AuthenticateUserRequest) error {
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
	require.NoError(t, authServer.SetAuthPreference(ctx, authPreference))

	// Create user and initial WebAuthn device (MFA).
	const user = "llama"
	const password = "p@ssw0rd"
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
	ccr, err := pwdKey.SignCredentialCreation(origin, wanlib.CredentialCreationFromProto(registerChallenge.GetWebauthn()))
	require.NoError(t, err)
	_, err = userClient.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
		TokenID:       token.GetName(),
		NewDeviceName: "pwdless1",
		NewMFAResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
			},
		},
	})
	require.NoError(t, err, "Failed to register passwordless device")

	// userWebID is what identifies the user for usernameless/passwordless.
	userWebID := registerChallenge.GetWebauthn().PublicKey.User.Id

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
		authenticate func(t *testing.T, resp *wanlib.CredentialAssertionResponse)
	}{
		{
			name: "ssh",
			authenticate: func(t *testing.T, resp *wanlib.CredentialAssertionResponse) {
				loginResp, err := proxyClient.AuthenticateSSHUser(ctx, AuthenticateSSHRequest{
					AuthenticateUserRequest: AuthenticateUserRequest{
						Webauthn:  resp,
						PublicKey: []byte(sshPubKey),
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
			authenticate: func(t *testing.T, resp *wanlib.CredentialAssertionResponse) {
				loginResp, err := proxyClient.AuthenticateSSHUser(ctx, AuthenticateSSHRequest{
					AuthenticateUserRequest: AuthenticateUserRequest{
						Webauthn:  resp,
						PublicKey: []byte(sshPubKey),
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
			authenticate: func(t *testing.T, resp *wanlib.CredentialAssertionResponse) {
				session, err := proxyClient.AuthenticateWebUser(ctx, AuthenticateUserRequest{
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
			authenticate: func(t *testing.T, resp *wanlib.CredentialAssertionResponse) {
				session, err := proxyClient.AuthenticateWebUser(ctx, AuthenticateUserRequest{
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
			_, err := proxyClient.AuthenticateSSHUser(ctx, AuthenticateSSHRequest{
				AuthenticateUserRequest: AuthenticateUserRequest{
					Username:  user,
					Webauthn:  &wanlib.CredentialAssertionResponse{}, // bad response
					PublicKey: []byte(sshPubKey),
				},
				TTL: 24 * time.Hour,
			})
			require.True(t, trace.IsAccessDenied(err), "got err = %v, want AccessDenied")
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
			assertionResp, err := pwdKey.SignAssertion(origin, wanlib.CredentialAssertionFromProto(mfaChallenge.GetWebauthnChallenge()))
			require.NoError(t, err)
			assertionResp.AssertionResponse.UserHandle = userWebID // identify user, a real device would set this

			// Complete login procedure (SSH or Web).
			test.authenticate(t, assertionResp)

			// Verify zeroed login attempts. This is a proxy for various other user
			// checks (locked, etc).
			attempts, err = authServer.GetUserLoginAttempts(user)
			require.NoError(t, err)
			require.Empty(t, attempts, "Login attempts not reset")

			require.Equal(t, len(test.loginHooks), int(loginHookCounter.Load()))
		})
	}
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
			wantErr: "invalid Webauthn response", // generic error as it _could_ be a passwordless attempt
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mfaChallenge, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
					ContextUser: &proto.ContextUser{},
				},
			})
			require.NoError(t, err)

			mfaResp, err := test.dev.SolveAuthn(mfaChallenge)
			require.NoError(t, err)

			req := AuthenticateUserRequest{
				PublicKey: []byte(sshPubKey),
			}
			switch {
			case mfaResp.GetWebauthn() != nil:
				req.Webauthn = wanlib.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn())
			case mfaResp.GetTOTP() != nil:
				req.OTP = &OTPCreds{
					Password: []byte(password),
					Token:    mfaResp.GetTOTP().Code,
				}
			}

			// SSH.
			_, err = proxyClient.AuthenticateSSHUser(ctx, AuthenticateSSHRequest{
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
		name      string
		update    func(*types.HeadlessAuthentication, *types.MFADevice)
		expectErr bool
	}{
		{
			name: "OK approved",
			update: func(ha *types.HeadlessAuthentication, mfa *types.MFADevice) {
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
				ha.MfaDevice = mfa
			},
		}, {
			name: "NOK approved without MFA",
			update: func(ha *types.HeadlessAuthentication, mfa *types.MFADevice) {
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
			},
			expectErr: true,
		}, {
			name: "NOK denied",
			update: func(ha *types.HeadlessAuthentication, mfa *types.MFADevice) {
				ha.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED
			},
			expectErr: true,
		}, {
			name:      "NOK timeout",
			update:    func(ha *types.HeadlessAuthentication, mfa *types.MFADevice) {},
			expectErr: true,
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

			// Fail a login attempt so we have a non-empty list of attempts.
			_, err = proxyClient.AuthenticateSSHUser(ctx, AuthenticateSSHRequest{
				AuthenticateUserRequest: AuthenticateUserRequest{
					Username:  username,
					Webauthn:  &wanlib.CredentialAssertionResponse{}, // bad response
					PublicKey: []byte(sshPubKey),
				},
				TTL: 24 * time.Hour,
			})
			require.True(t, trace.IsAccessDenied(err), "got err = %v, want AccessDenied", err)
			attempts, err := srv.Auth().GetUserLoginAttempts(username)
			require.NoError(t, err)
			require.NotEmpty(t, attempts, "Want at least one failed login attempt")

			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			// Start a goroutine to catch the headless authentication attempt and update with test case values.
			errC := make(chan error)
			go func() {
				defer close(errC)

				err := srv.Auth().CreateHeadlessAuthenticationStub(ctx, username)
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

			_, err = proxyClient.AuthenticateSSHUser(ctx, AuthenticateSSHRequest{
				AuthenticateUserRequest: AuthenticateUserRequest{
					// HeadlessAuthenticationID should take precedence over WebAuthn and OTP fields.
					HeadlessAuthenticationID: headlessID,
					Webauthn:                 &wanlib.CredentialAssertionResponse{},
					OTP:                      &OTPCreds{},
					Username:                 username,
					PublicKey:                []byte(sshPubKey),
					ClientMetadata: &ForwardedClientMetadata{
						RemoteAddr: "0.0.0.0",
					},
				},
				TTL: defaults.CallbackTimeout,
			})

			// Use assert so that we also output any test failures below.
			assert.NoError(t, <-errC, "Failed to get and update headless authentication in background")

			if tc.expectErr {
				require.Error(t, err)
				// Verify login attempts unchanged. This is a proxy for various other user
				// checks (locked, etc).
				updatedAttempts, err := srv.Auth().GetUserLoginAttempts(username)
				require.NoError(t, err)
				require.Equal(t, attempts, updatedAttempts, "Login attempts unexpectedly changed")
			} else {
				require.NoError(t, err)
				// Verify zeroed login attempts. This is a proxy for various other user
				// checks (locked, etc).
				updatedAttempts, err := srv.Auth().GetUserLoginAttempts(username)
				require.NoError(t, err)
				require.Empty(t, updatedAttempts, "Login attempts not reset")
			}
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
		// Use default Webauthn config.
	})
	require.NoError(t, err)

	authServer := srv.Auth()
	ctx := context.Background()
	require.NoError(t, authServer.SetAuthPreference(ctx, authPreference))

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
