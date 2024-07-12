/*
Copyright 2017-2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"encoding/base32"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/keystore"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
)

type passwordSuite struct {
	bk          backend.Backend
	a           *Server
	mockEmitter *eventstest.MockRecorderEmitter
}

func setupPasswordSuite(t *testing.T) *passwordSuite {
	s := passwordSuite{}

	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Now())

	var err error

	s.bk, err = memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	// set cluster name
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		s.bk.Close()
	})

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		VersionStorage:         NewFakeTeleportVersion(),
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
		KeyStoreConfig: keystore.Config{
			Software: keystore.SoftwareConfig{
				RSAKeyPairSource: authority.New().GenerateKeyPair,
			},
		},
	}
	s.a, err = NewServer(authConfig)
	require.NoError(t, err)

	err = s.a.SetClusterName(clusterName)
	require.NoError(t, err)

	// set lock watcher
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentAuth,
			Client:    s.a,
		},
	})
	require.NoError(t, err, "NewLockWatcher")
	s.a.SetLockWatcher(lockWatcher)

	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	require.NoError(t, err)
	err = s.a.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	s.mockEmitter = &eventstest.MockRecorderEmitter{}
	s.a.emitter = s.mockEmitter
	return &s
}

func TestUserNotFound(t *testing.T) {
	t.Parallel()

	s := setupPasswordSuite(t)
	username := "unknown-user"
	password := "barbaz"

	err := s.a.checkPasswordWOToken(username, []byte(password))
	require.Error(t, err)
	// Make sure the error is not a NotFound. That would be a username oracle.
	require.True(t, trace.IsBadParameter(err))
}

func TestChangePassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := setupPasswordSuite(t)
	req, err := s.prepareForPasswordChange("user1", []byte("abc123"), constants.SecondFactorOff)
	require.NoError(t, err)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)
	req.NewPassword = []byte("abce456")

	err = s.a.ChangePassword(ctx, req)
	require.NoError(t, err)
	require.Equal(t, events.UserPasswordChangeEvent, s.mockEmitter.LastEvent().GetType())
	require.Equal(t, "user1", s.mockEmitter.LastEvent().(*apievents.UserPasswordChange).User)
	s.shouldLockAfterFailedAttempts(t, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	err = s.a.ChangePassword(ctx, req)
	require.NoError(t, err)
}

func TestChangePasswordWithOTP(t *testing.T) {
	t.Parallel()

	s := setupPasswordSuite(t)
	req, err := s.prepareForPasswordChange("user2", []byte("abc123"), constants.SecondFactorOTP)
	require.NoError(t, err)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)

	otpSecret := base32.StdEncoding.EncodeToString([]byte("def456"))
	dev, err := services.NewTOTPDevice("otp", otpSecret, fakeClock.Now())
	require.NoError(t, err)
	ctx := context.Background()
	err = s.a.UpsertMFADevice(ctx, req.User, dev)
	require.NoError(t, err)

	validToken, err := totp.GenerateCode(otpSecret, s.a.GetClock().Now())
	require.NoError(t, err)

	// change password
	req.NewPassword = []byte("abce456")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(ctx, req)
	require.NoError(t, err)

	s.shouldLockAfterFailedAttempts(t, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	validToken, _ = totp.GenerateCode(otpSecret, s.a.GetClock().Now())
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(ctx, req)
	require.NoError(t, err)
}

func TestServer_ChangePassword(t *testing.T) {
	t.Parallel()

	srv := newTestTLSServer(t)

	mfa := configureForMFA(t, srv)
	username := mfa.User
	password := mfa.Password

	tests := []struct {
		name    string
		newPass string
		device  *TestDevice
	}{
		{
			name:    "OK TOTP-based change",
			newPass: "llamasarecool11",
			device:  mfa.TOTPDev,
		},
		{
			name:    "OK Webauthn-based change",
			newPass: "llamasarecool13",
			device:  mfa.WebDev,
		},
	}

	authServer := srv.Auth()
	ctx := context.Background()

	oldPass := []byte(password)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newPass := []byte(test.newPass)

			// Acquire and solve an MFA challenge.
			mfaChallenge, err := authServer.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_UserCredentials{
					UserCredentials: &proto.UserCredentials{
						Username: username,
						Password: oldPass,
					},
				},
			})
			require.NoError(t, err, "creating challenge")
			mfaResp, err := test.device.SolveAuthn(mfaChallenge)
			require.NoError(t, err, "solving challenge with device")

			// Change password.
			req := &proto.ChangePasswordRequest{
				User:        username,
				OldPassword: oldPass,
				NewPassword: newPass,
			}
			switch {
			case mfaResp.GetTOTP() != nil:
				req.SecondFactorToken = mfaResp.GetTOTP().Code
			case mfaResp.GetWebauthn() != nil:
				req.Webauthn = mfaResp.GetWebauthn()
			}
			require.NoError(t, authServer.ChangePassword(ctx, req), "changing password")

			// Did the password change take effect?
			require.NoError(t, authServer.checkPasswordWOToken(username, newPass), "password change didn't take effect")

			oldPass = newPass // Set for next iteration.
		})
	}
}

// This test asserts that an attacker is unable to change password without
// providing the old one if they take over a user's web session and use a
// different type of WebAuthn challenge that would be normally requested by the
// Web UI. This is a regression test for
// https://github.com/gravitational/teleport-private/issues/1369.
func TestServer_ChangePassword_FailsWithoutOldPassword(t *testing.T) {
	t.Parallel()

	server := newTestTLSServer(t)
	mfa := configureForMFA(t, server)
	authServer := server.Auth()
	ctx := context.Background()

	username := mfa.User
	newPass := []byte("capybarasarecool123")

	userClient, err := server.NewClient(TestUser(username))
	require.NoError(t, err)
	defer userClient.Close()

	// Acquire and solve an MFA challenge.
	mfaChallenge, err := userClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
	})
	require.NoError(t, err, "creating challenge")
	mfaResp, err := mfa.WebDev.SolveAuthn(mfaChallenge)
	require.NoError(t, err, "solving challenge with device")

	// Change password.
	req := &proto.ChangePasswordRequest{
		User:        username,
		NewPassword: newPass,
		Webauthn:    mfaResp.GetWebauthn(),
	}
	err = authServer.ChangePassword(ctx, req)
	assert.True(t,
		trace.IsAccessDenied(err),
		"ChangePassword error mismatch, want=AccessDenied, got=%v (%T)",
		err, trace.Unwrap(err))

	// Did the password change take effect?
	assert.Error(t, authServer.checkPasswordWOToken(username, newPass), "password was changed")
}

func TestChangeUserAuthentication(t *testing.T) {
	t.Parallel()

	srv := newTestTLSServer(t)
	ctx := context.Background()

	tests := []struct {
		name              string
		setAuthPreference func()
		getReq            func(string) *proto.ChangeUserAuthenticationRequest
		getInvalidReq     func(string) *proto.ChangeUserAuthenticationRequest
	}{
		{
			name: "with second factor off and password only",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOff,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password1"),
				}
			},
		},
		{
			name: "with second factor otp",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOTP,
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				res, err := srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
					TokenID:    resetTokenID,
					DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
				})
				require.NoError(t, err)

				otpToken, err := totp.GenerateCode(res.GetTOTP().GetSecret(), srv.Clock().Now())
				require.NoError(t, err)

				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password2"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
						TOTP: &proto.TOTPRegisterResponse{Code: otpToken},
					}},
				}
			},
			// Invalid MFA fields when auth settings set to only otp.
			getInvalidReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password2"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_Webauthn{
						Webauthn: &wanpb.CredentialCreationResponse{},
					}},
				}
			},
		},
		{
			name: "with second factor webauthn",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorWebauthn,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				_, webauthnRegRes, err := getMockedWebauthnAndRegisterRes(srv.Auth(), resetTokenID, proto.DeviceUsage_DEVICE_USAGE_MFA)
				require.NoError(t, err)

				return &proto.ChangeUserAuthenticationRequest{
					TokenID:                resetTokenID,
					NewPassword:            []byte("password3"),
					NewMFARegisterResponse: webauthnRegRes,
				}
			},
			// Invalid totp fields when auth settings set to only webauthn.
			getInvalidReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:                resetTokenID,
					NewPassword:            []byte("password3"),
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{}},
				}
			},
		},
		{
			name: "with passwordless",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorWebauthn,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				_, webauthnRes, err := getMockedWebauthnAndRegisterRes(srv.Auth(), resetTokenID, proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS)
				require.NoError(t, err)

				return &proto.ChangeUserAuthenticationRequest{
					TokenID:                resetTokenID,
					NewMFARegisterResponse: webauthnRes,
				}
			},
			// Missing webauthn for passwordless.
			getInvalidReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:                resetTokenID,
					NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{}},
				}
			},
		},
		{
			name: "with second factor on",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOn,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				_, mfaResp, err := getMockedWebauthnAndRegisterRes(srv.Auth(), resetTokenID, proto.DeviceUsage_DEVICE_USAGE_MFA)
				require.NoError(t, err)

				return &proto.ChangeUserAuthenticationRequest{
					TokenID:                resetTokenID,
					NewPassword:            []byte("password4"),
					NewMFARegisterResponse: mfaResp,
					NewDeviceName:          "new-device",
				}
			},
			// Empty register response, when auth settings requires second factors.
			getInvalidReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password4"),
				}
			},
		},
		{
			name: "with second factor optional and no second factor",
			setAuthPreference: func() {
				authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				})
				require.NoError(t, err)
				err = srv.Auth().SetAuthPreference(ctx, authPreference)
				require.NoError(t, err)
			},
			getReq: func(resetTokenID string) *proto.ChangeUserAuthenticationRequest {
				return &proto.ChangeUserAuthenticationRequest{
					TokenID:     resetTokenID,
					NewPassword: []byte("password5"),
				}
			},
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			username := fmt.Sprintf("llama%v@goteleport.com", rand.Int())
			_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username}, nil)
			require.NoError(t, err)

			c.setAuthPreference()

			token, err := srv.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
				Name: username,
			})
			require.NoError(t, err)

			if c.getInvalidReq != nil {
				invalidReq := c.getInvalidReq(token.GetName())
				_, err := srv.Auth().changeUserAuthentication(ctx, invalidReq)
				require.True(t, trace.IsBadParameter(err))
			}

			validReq := c.getReq(token.GetName())
			_, err = srv.Auth().changeUserAuthentication(ctx, validReq)
			require.NoError(t, err)

			// Test password is updated.
			if len(validReq.NewPassword) != 0 {
				err := srv.Auth().checkPasswordWOToken(username, validReq.NewPassword)
				require.NoError(t, err)
			}

			// Test device was registered.
			if validReq.NewMFARegisterResponse != nil {
				devs, err := srv.Auth().Services.GetMFADevices(ctx, username, false /* without secrets*/)
				require.NoError(t, err)
				require.Len(t, devs, 1)

				// Test device name setting.
				dev := devs[0]
				var wantName string
				switch {
				case validReq.NewDeviceName != "":
					wantName = validReq.NewDeviceName
				case dev.GetTotp() != nil:
					wantName = "otp"
				case dev.GetWebauthn() != nil:
					wantName = "webauthn"
				}
				require.Equal(t, wantName, dev.GetName(), "device name mismatch")
			}
		})
	}
}

func TestChangeUserAuthenticationWithErrors(t *testing.T) {
	t.Parallel()

	s := setupPasswordSuite(t)
	ctx := context.Background()
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)

	username := "joe@example.com"
	_, _, err = CreateUserAndRole(s.a, username, []string{username}, nil)
	require.NoError(t, err)

	token, err := s.a.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: username,
	})
	require.NoError(t, err)

	validPassword := []byte("qweQWE1")
	validTokenID := token.GetName()

	type testCase struct {
		desc         string
		secondFactor constants.SecondFactorType
		req          *proto.ChangeUserAuthenticationRequest
	}

	testCases := []testCase{
		{
			secondFactor: constants.SecondFactorOff,
			desc:         "invalid tokenID value",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     "what_token",
				NewPassword: validPassword,
			},
		},
		{
			secondFactor: constants.SecondFactorOff,
			desc:         "invalid password",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     validTokenID,
				NewPassword: []byte("short"),
			},
		},
		{
			secondFactor: constants.SecondFactorOTP,
			desc:         "missing second factor",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     validTokenID,
				NewPassword: validPassword,
			},
		},
		{
			secondFactor: constants.SecondFactorOTP,
			desc:         "invalid OTP value",
			req: &proto.ChangeUserAuthenticationRequest{
				TokenID:     validTokenID,
				NewPassword: validPassword,
				NewMFARegisterResponse: &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
					TOTP: &proto.TOTPRegisterResponse{Code: "invalid"},
				}},
			},
		},
	}

	for _, tc := range testCases {
		// set new auth preference settings
		authPreference.SetSecondFactor(tc.secondFactor)
		err = s.a.SetAuthPreference(ctx, authPreference)
		require.NoError(t, err)

		_, err = s.a.changeUserAuthentication(ctx, tc.req)
		require.Error(t, err, "test case %q", tc.desc)
	}

	authPreference.SetSecondFactor(constants.SecondFactorOff)
	err = s.a.SetAuthPreference(ctx, authPreference)
	require.NoError(t, err)

	_, err = s.a.changeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     validTokenID,
		NewPassword: validPassword,
	})
	require.NoError(t, err)

	// invite token cannot be reused
	_, err = s.a.changeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     validTokenID,
		NewPassword: validPassword,
	})
	require.Error(t, err)
}

func (s *passwordSuite) shouldLockAfterFailedAttempts(t *testing.T, req *proto.ChangePasswordRequest) {
	ctx := context.Background()
	loginAttempts, _ := s.a.GetUserLoginAttempts(req.User)
	require.Empty(t, loginAttempts)
	for i := 0; i < defaults.MaxLoginAttempts; i++ {
		err := s.a.ChangePassword(ctx, req)
		require.Error(t, err)
		loginAttempts, _ = s.a.GetUserLoginAttempts(req.User)
		require.Len(t, loginAttempts, i+1)
	}

	err := s.a.ChangePassword(ctx, req)
	require.True(t, trace.IsAccessDenied(err))
}

func (s *passwordSuite) prepareForPasswordChange(user string, pass []byte, secondFactorType constants.SecondFactorType) (*proto.ChangePasswordRequest, error) {
	ctx := context.Background()
	req := &proto.ChangePasswordRequest{
		User:        user,
		OldPassword: pass,
	}

	err := s.a.UpsertCertAuthority(ctx, suite.NewTestCA(types.UserCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	err = s.a.UpsertCertAuthority(ctx, suite.NewTestCA(types.HostCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: secondFactorType,
	})
	if err != nil {
		return req, err
	}

	err = s.a.SetAuthPreference(ctx, ap)
	if err != nil {
		return req, err
	}

	_, _, err = CreateUserAndRole(s.a, user, []string{user}, nil)
	if err != nil {
		return req, err
	}
	err = s.a.UpsertPassword(user, pass)
	if err != nil {
		return req, err
	}

	return req, nil
}
