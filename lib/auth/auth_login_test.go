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
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"github.com/tstranex/u2f"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

func TestServer_CreateAuthenticateChallenge_authPreference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svr := newTestTLSServer(t)
	authServer := svr.Auth()
	mfa := configureForMFA(t, svr)
	username := mfa.User
	password := mfa.Password

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
				require.Empty(t, challenge.GetU2F())
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
				require.Empty(t, challenge.GetU2F())
				require.Empty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:u2f",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorU2F,
				U2F: &types.U2F{
					AppID:  "https://localhost",
					Facets: []string{"https://localhost"},
				},
			},
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetU2F())
				require.Empty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:webauthn (derived from U2F)",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				U2F: &types.U2F{
					AppID:  "https://localhost",
					Facets: []string{"https://localhost"},
				},
			},
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.Empty(t, challenge.GetU2F())
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
				require.Empty(t, challenge.GetU2F())
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
					Facets: []string{
						"https://myoldappid.com",
						"https://localhost",
					},
				},
				Webauthn: &types.Webauthn{
					RPID: "myexplicitid",
				},
			},
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.Empty(t, challenge.GetU2F())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
				require.Equal(t, "myexplicitid", challenge.GetWebauthnChallenge().GetPublicKey().GetRpId())
			},
		},
		{
			name: "OK second_factor:webauthn (derived from U2F)",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorWebauthn,
				U2F: &types.U2F{
					AppID:  "https://localhost",
					Facets: []string{"https://localhost"},
				},
			},
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.Empty(t, challenge.GetTOTP())
				require.Empty(t, challenge.GetU2F())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:optional",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOptional,
				U2F: &types.U2F{
					AppID:  "https://localhost",
					Facets: []string{"https://localhost"},
				},
			},
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetU2F())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:optional with global Webauthn disable",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOptional,
				U2F: &types.U2F{
					AppID:  "https://localhost",
					Facets: []string{"https://localhost"},
				},
				Webauthn: &types.Webauthn{
					Disabled: true,
				},
			},
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetU2F())
				require.Empty(t, challenge.GetWebauthnChallenge())
			},
		},
		{
			name: "OK second_factor:on",
			spec: &types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOn,
				U2F: &types.U2F{
					AppID:  "https://localhost",
					Facets: []string{"https://localhost"},
				},
			},
			assertChallenge: func(challenge *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, challenge.GetTOTP())
				require.NotEmpty(t, challenge.GetU2F())
				require.NotEmpty(t, challenge.GetWebauthnChallenge())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authPreference, err := types.NewAuthPreference(*test.spec)
			require.NoError(t, err)
			require.NoError(t, authServer.SetAuthPreference(ctx, authPreference))

			challenge, err := authServer.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
					Username: username,
					Password: []byte(password),
				}},
			})
			require.NoError(t, err)
			test.assertChallenge(challenge)
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
		{name: "OK U2F device", solveChallenge: mfa.U2FDev.SolveAuthn},
		{name: "OK Webauthn device", solveChallenge: mfa.WebDev.SolveAuthn},
	}
	for _, test := range tests {
		test := test
		// makeRun is used to test both SSH and Web login by switching the
		// authenticate function.
		makeRun := func(authenticate func(*Server, AuthenticateUserRequest) error) func(t *testing.T) {
			return func(t *testing.T) {
				// 1st step: acquire challenge
				challenge, err := authServer.CreateAuthenticateChallenge(context.Background(), &proto.CreateAuthenticateChallengeRequest{
					Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{UserCredentials: &proto.UserCredentials{
						Username: username,
						Password: []byte(password),
					}},
				})
				require.NoError(t, err)

				// Solve challenge (client-side)
				resp, err := test.solveChallenge(challenge)
				authReq := AuthenticateUserRequest{
					Username: username,
				}
				require.NoError(t, err)

				switch {
				case resp.GetWebauthn() != nil:
					authReq.Webauthn = wanlib.CredentialAssertionResponseFromProto(resp.GetWebauthn())
				case resp.GetU2F() != nil:
					authReq.U2F = &U2FSignResponseCreds{
						SignResponse: u2f.SignResponse{
							KeyHandle:     resp.GetU2F().KeyHandle,
							SignatureData: resp.GetU2F().Signature,
							ClientData:    resp.GetU2F().ClientData,
						},
					}
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
			_, err := s.AuthenticateSSHUser(AuthenticateSSHRequest{
				AuthenticateUserRequest: req,
				PublicKey:               []byte(sshPubKey),
				TTL:                     24 * time.Hour,
			})
			return err
		}))
		t.Run(test.name+"/web", makeRun(func(s *Server, req AuthenticateUserRequest) error {
			_, err := s.AuthenticateWebUser(req)
			return err
		}))
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
	_, err = srv.Auth().validateMFAAuthResponse(ctx, u.username, mfaResp, nil /* u2fStorage */)
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
				require.True(t, trace.IsAccessDenied(err))
			default:
				require.NoError(t, err)
				require.NotNil(t, res.GetTOTP())
				require.NotEmpty(t, res.GetU2F())
				require.NotEmpty(t, res.GetWebauthnChallenge())
			}
		})
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
				require.NotEmpty(t, res.GetU2F())
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
			name:       "u2f challenge",
			deviceType: proto.DeviceType_DEVICE_TYPE_U2F,
		},
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

			case proto.DeviceType_DEVICE_TYPE_U2F:
				require.NotNil(t, res.GetU2F())

			case proto.DeviceType_DEVICE_TYPE_WEBAUTHN:
				require.NotNil(t, res.GetWebauthn())
			}
		})
	}
}

type configureMFAResp struct {
	User, Password          string
	TOTPDev, U2FDev, WebDev *TestDevice
}

func configureForMFA(t *testing.T, srv *TestTLSServer) *configureMFAResp {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		U2F: &types.U2F{
			AppID:  "https://localhost",
			Facets: []string{"https://localhost"},
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
	_, _, err = CreateUserAndRole(authServer, username, []string{"llama", "root"})
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertPassword(username, []byte(password)))

	clt, err := srv.NewClient(TestUser(username))
	require.NoError(t, err)

	totpDev, err := RegisterTestDevice(ctx, clt, "totp-1", proto.DeviceType_DEVICE_TYPE_TOTP, nil, WithTestDeviceClock(srv.Clock()))
	require.NoError(t, err)

	u2fDev, err := RegisterTestDevice(ctx, clt, "u2f-1", proto.DeviceType_DEVICE_TYPE_U2F, totpDev)
	require.NoError(t, err)

	webDev, err := RegisterTestDevice(ctx, clt, "web-1", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, totpDev)
	require.NoError(t, err)

	return &configureMFAResp{
		User:     username,
		Password: password,
		TOTPDev:  totpDev,
		U2FDev:   u2fDev,
		WebDev:   webDev,
	}
}
