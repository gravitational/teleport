/*
Copyright 2020 Gravitational, Inc.

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
	"encoding/hex"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"github.com/tstranex/u2f"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestCreateResetPasswordToken(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	username := "joe@example.com"
	pass := "pass123"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	ctx := context.Background()

	// Add several MFA devices.
	mfaDev, err := services.NewTOTPDevice("otp1", "secret", srv.Clock().Now())
	require.NoError(t, err)
	err = srv.Auth().UpsertMFADevice(ctx, username, mfaDev)
	require.NoError(t, err)
	mfaDev, err = services.NewTOTPDevice("otp2", "secret", srv.Clock().Now())
	require.NoError(t, err)
	err = srv.Auth().UpsertMFADevice(ctx, username, mfaDev)
	require.NoError(t, err)

	req := CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	token, err := srv.Auth().CreateResetPasswordToken(ctx, req)
	require.NoError(t, err)
	require.Equal(t, token.GetUser(), username)
	require.Equal(t, token.GetURL(), "https://<proxyhost>:3080/web/reset/"+token.GetName())

	event := mockEmitter.LastEvent()
	require.Equal(t, event.GetType(), events.ResetPasswordTokenCreateEvent)
	require.Equal(t, event.(*apievents.UserTokenCreate).Name, "joe@example.com")
	require.Equal(t, event.(*apievents.UserTokenCreate).User, teleport.UserSystem)

	// verify that user has no MFA devices
	devs, err := srv.Auth().GetMFADevices(ctx, username)
	require.NoError(t, err)
	require.Empty(t, devs)

	// verify that password was reset
	err = srv.Auth().checkPasswordWOToken(username, []byte(pass))
	require.Error(t, err)

	// create another reset token for the same user
	token, err = srv.Auth().CreateResetPasswordToken(ctx, req)
	require.NoError(t, err)

	// previous token must be deleted
	tokens, err := srv.Auth().GetUserTokens(ctx)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, tokens[0].GetName(), token.GetName())
}

func TestCreateResetPasswordTokenErrors(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	username := "joe@example.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	type testCase struct {
		desc string
		req  CreateUserTokenRequest
	}

	testCases := []testCase{
		{
			desc: "Reset Password: TTL < 0",
			req: CreateUserTokenRequest{
				Name: username,
				TTL:  -1,
			},
		},
		{
			desc: "Reset Password: TTL > max",
			req: CreateUserTokenRequest{
				Name: username,
				TTL:  defaults.MaxChangePasswordTokenTTL + time.Hour,
			},
		},
		{
			desc: "Reset Password: empty user name",
			req: CreateUserTokenRequest{
				TTL: time.Hour,
			},
		},
		{
			desc: "Reset Password: user does not exist",
			req: CreateUserTokenRequest{
				Name: "doesnotexist@example.com",
				TTL:  time.Hour,
			},
		},
		{
			desc: "Invite: TTL > max",
			req: CreateUserTokenRequest{
				Name: username,
				TTL:  defaults.MaxSignupTokenTTL + time.Hour,
				Type: UserTokenTypeResetPasswordInvite,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := srv.Auth().CreateResetPasswordToken(context.TODO(), tc.req)
			require.Error(t, err)
		})
	}
}

// TestFormatAccountName makes sure that the OTP account name fallback values
// are correct. description
func TestFormatAccountName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description    string
		inDebugAuth    *debugAuth
		outAccountName string
		outError       require.ErrorAssertionFunc
	}{
		{
			description: "failed to fetch proxies",
			inDebugAuth: &debugAuth{
				proxiesError: true,
			},
			outAccountName: "",
			outError:       require.Error,
		},
		{
			description: "proxies with public address",
			inDebugAuth: &debugAuth{
				proxies: []types.Server{
					&types.ServerV2{
						Spec: types.ServerSpecV2{
							PublicAddr: "foo",
							Version:    "bar",
						},
					},
				},
			},
			outAccountName: "foo@foo",
			outError:       require.NoError,
		},
		{
			description: "proxies with no public address",
			inDebugAuth: &debugAuth{
				proxies: []types.Server{
					&types.ServerV2{
						Spec: types.ServerSpecV2{
							Hostname: "baz",
							Version:  "quxx",
						},
					},
				},
			},
			outAccountName: "foo@baz:3080",
			outError:       require.NoError,
		},
		{
			description: "no proxies, with domain name",
			inDebugAuth: &debugAuth{
				clusterName: "example.com",
			},
			outAccountName: "foo@example.com",
			outError:       require.NoError,
		},
		{
			description:    "no proxies, no domain name",
			inDebugAuth:    &debugAuth{},
			outAccountName: "foo@00000000-0000-0000-0000-000000000000",
			outError:       require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			accountName, err := formatAccountName(tt.inDebugAuth, "foo", "00000000-0000-0000-0000-000000000000")
			tt.outError(t, err)
			require.Equal(t, accountName, tt.outAccountName)
		})
	}
}

func TestUserTokenSecretsCreationSettings(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	username := "joe@example.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	ctx := context.Background()

	req := CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	token, err := srv.Auth().CreateResetPasswordToken(ctx, req)
	require.NoError(t, err)

	secrets, err := srv.Auth().RotateUserTokenSecrets(context.TODO(), token.GetName())
	require.NoError(t, err)
	require.Equal(t, secrets.GetName(), token.GetName())
	require.Equal(t, token.GetMetadata().Expires, secrets.GetMetadata().Expires)
	require.NotEmpty(t, secrets.GetOTPKey())
	require.NotEmpty(t, secrets.GetQRCode())
}

func TestUserTokenCreationSettings(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	username := "joe@example.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	req := CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
		Type: UserTokenTypeResetPasswordInvite,
	}

	token, err := srv.Auth().newUserToken(req)
	require.NoError(t, err)
	require.Equal(t, req.Name, token.GetUser())
	require.Equal(t, req.Type, token.GetSubKind())
	require.Equal(t, token.GetURL(), "https://<proxyhost>:3080/web/invite/"+token.GetName())
	require.NotEmpty(t, token.GetCreated())
	require.NotEmpty(t, token.GetMetadata().Expires)

}

// DELETE IN 9.0: remove legacy prefix and fallbacks.
func TestBackwardsCompForUserTokenWithLegacyPrefix(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	username := "joe@example.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	ctx := context.Background()

	req := CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	// Create a reset password user token.
	legacyToken, err := srv.Auth().newUserToken(req)
	require.NoError(t, err)

	marshalledToken, err := services.MarshalUserToken(legacyToken)
	require.NoError(t, err)

	// Insert the token in backend using legacy prefix.
	_, err = srv.AuthServer.Backend.Create(ctx, backend.Item{
		Key:   backend.Key(local.LegacyPasswordTokensPrefix, legacyToken.GetName(), "params"),
		Value: marshalledToken,
	})
	require.NoError(t, err)

	// Test fallback get token.
	retrievedToken, err := srv.Auth().GetUserToken(ctx, legacyToken.GetName())
	require.NoError(t, err)
	require.Equal(t, legacyToken.GetName(), retrievedToken.GetName())

	// Create a user token secrets.
	legacySecrets, err := types.NewUserTokenSecrets(legacyToken.GetName())
	legacySecrets.SetOTPKey("test")
	require.NoError(t, err)

	marshalledSecrets, err := services.MarshalUserTokenSecrets(legacySecrets)
	require.NoError(t, err)

	// Insert the secret in backend using legacy prefix.
	_, err = srv.AuthServer.Backend.Create(ctx, backend.Item{
		Key:   backend.Key(local.LegacyPasswordTokensPrefix, legacySecrets.GetName(), "secrets"),
		Value: marshalledSecrets,
	})
	require.NoError(t, err)

	// Test fallback get secrets.
	retrievedSecrets, err := srv.Auth().GetUserTokenSecrets(ctx, legacySecrets.GetName())
	require.NoError(t, err)
	require.Equal(t, legacyToken.GetName(), retrievedSecrets.GetName())
	require.Equal(t, legacySecrets.GetOTPKey(), retrievedSecrets.GetOTPKey())

	// Test deletion of token stored with legacy prefix.
	// Helper method deleteUserTokens hits both GetUserTokens and DeleteUserToken path.
	err = srv.Auth().deleteUserTokens(ctx, req.Name)
	require.NoError(t, err)

	// Test for deletion of token and secrets.
	_, err = srv.Auth().GetUserToken(ctx, legacyToken.GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = srv.Auth().GetUserTokenSecrets(ctx, legacySecrets.GetName())
	require.True(t, trace.IsNotFound(err))
}

func TestCreateRecoveryToken(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	username := "joe@example.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	ctx := context.Background()

	startToken, err := srv.Auth().createRecoveryToken(ctx, username, UserTokenTypeRecoveryStart, true)
	require.NoError(t, err)
	require.Equal(t, startToken.GetURL(), "https://<proxyhost>:3080/web/recovery/"+startToken.GetName())

	// Test token uses correct byte length.
	bytes, err := hex.DecodeString(startToken.GetName())
	require.NoError(t, err)
	require.Len(t, bytes, RecoveryTokenLenBytes)

	// Test usage setting.
	require.Equal(t, types.UserTokenUsage_RECOVER_PWD, startToken.GetUsage())

	approvedToken, err := srv.Auth().createRecoveryToken(ctx, username, UserTokenTypeRecoveryApproved, false)
	require.NoError(t, err)
	require.Equal(t, approvedToken.GetURL(), "https://<proxyhost>:3080/web/recovery/"+approvedToken.GetName())

	bytes, err = hex.DecodeString(approvedToken.GetName())
	require.NoError(t, err)
	require.Len(t, bytes, RecoveryTokenLenBytes)

	// Test usage setting.
	require.Equal(t, types.UserTokenUsage_RECOVER_2FA, approvedToken.GetUsage())
}

func TestCreatePrivilegeTokenHelper(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	fakeClock := srv.Clock().(clockwork.FakeClock)
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter

	username := "joe@example.com"
	_, _, err := CreateUserAndRoleWithoutRoles(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	token, err := srv.Auth().createPrivilegeToken(context.Background(), username)
	require.NoError(t, err)
	require.Equal(t, token.GetSubKind(), UserTokenTypePrivilege)
	require.Equal(t, token.GetUser(), username)
	require.Equal(t, token.GetURL(), "https://<proxyhost>:3080")

	event := mockEmitter.LastEvent()
	require.Equal(t, event.GetType(), events.PrivilegeTokenCreateEvent)
	require.Equal(t, event.GetCode(), events.PrivilegeTokenCreateCode)
	require.Equal(t, event.(*apievents.UserTokenCreate).Name, username)
	require.Equal(t, event.(*apievents.UserTokenCreate).User, username)

	// Test token expires after designated time.
	fakeClock.Advance(defaults.MaxPrivilegeTokenTTL)
	_, err = srv.Auth().getPrivilegeToken(context.Background(), token.GetName())
	require.True(t, trace.IsNotFound(err))
}

// TestCreatePrivilegeTokenWithTOTPAuthAndLock tests a user can authenticate
// with their totp device and when attempting too many wrong attempts
// locks the user.
func TestCreatePrivilegeTokenWithTOTPAuthAndLock(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter
	ctx := context.Background()

	username := "joe@example.com"
	_, _, err := CreateUserAndRoleWithoutRoles(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	// Test failure when second factor isn't enabled.
	_, err = srv.Auth().CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{})
	require.True(t, trace.IsAccessDenied(err))

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, ap)
	require.NoError(t, err)

	token1, err := srv.Auth().createPrivilegeToken(context.Background(), username)
	require.NoError(t, err)

	// Add a mfa device.
	otp, err := getOTPCode(srv, token1.GetName())
	require.NoError(t, err)
	err = srv.Auth().AddMFADeviceWithToken(ctx, &proto.AddMFADeviceWithTokenRequest{
		TokenID:          token1.GetName(),
		SecondFactorCred: &proto.AddMFADeviceWithTokenRequest_SecondFactorToken{SecondFactorToken: otp},
	})
	require.NoError(t, err)

	// Get new otp code.
	mfas, err := srv.Auth().GetMFADevices(ctx, username)
	require.NoError(t, err)

	newOTP, err := totp.GenerateCode(mfas[0].GetTotp().Key, srv.Clock().Now().Add(30*time.Second))
	require.NoError(t, err)

	// Create token with otp auth.
	token2, err := srv.Auth().CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{
		Username:         username,
		SecondFactorCred: &proto.CreatePrivilegeTokenRequest_SecondFactorToken{SecondFactorToken: newOTP},
	})
	require.NoError(t, err)
	require.Equal(t, token2.GetSubKind(), UserTokenTypePrivilege)

	// Test first privilege token is deleted.
	_, err = srv.Auth().getPrivilegeToken(ctx, token1.GetName())
	require.True(t, trace.IsNotFound(err))

	// Test lock from max failed auth attempts.
	for i := 0; i < defaults.MaxLoginAttempts; i++ {
		_, err := srv.Auth().CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{
			Username:         username,
			SecondFactorCred: &proto.CreatePrivilegeTokenRequest_SecondFactorToken{SecondFactorToken: "wrong-value"},
		})
		require.Error(t, err)
	}

	// Test user is locked.
	user, err := srv.Auth().GetUser(username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
	require.False(t, user.GetStatus().LockExpires.IsZero())

	// Test providing correct value fails after lock.
	mfas, err = srv.Auth().GetMFADevices(ctx, username)
	require.NoError(t, err)

	newOTP, err = totp.GenerateCode(mfas[0].GetTotp().Key, srv.Clock().Now().Add(30*time.Second))
	require.NoError(t, err)

	_, err = srv.Auth().CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{
		Username:         username,
		SecondFactorCred: &proto.CreatePrivilegeTokenRequest_SecondFactorToken{SecondFactorToken: newOTP},
	})
	require.True(t, trace.IsAccessDenied(err))
}

// TestCreatePrivilegeTokenWithU2FAuthAndLock tests a user can authenticate
// with their u2f device and when attempting too many wrong attempts
// locks the user.
func TestCreatePrivilegeTokenWithU2FAuthAndLock(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	mockEmitter := &events.MockEmitter{}
	srv.Auth().emitter = mockEmitter
	ctx := context.Background()

	username := "joe@example.com"
	_, _, err := CreateUserAndRoleWithoutRoles(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, ap)
	require.NoError(t, err)

	token1, err := srv.Auth().createPrivilegeToken(context.Background(), username)
	require.NoError(t, err)

	// Add a u2f device.
	u2fRegResp, u2fKey, err := getMockedU2FAndRegisterRes(srv, token1.GetName())
	require.NoError(t, err)
	err = srv.Auth().AddMFADeviceWithToken(ctx, &proto.AddMFADeviceWithTokenRequest{
		TokenID:          token1.GetName(),
		DeviceName:       "new-u2f",
		SecondFactorCred: &proto.AddMFADeviceWithTokenRequest_U2FRegisterResponse{U2FRegisterResponse: u2fRegResp},
	})
	require.NoError(t, err)

	// Get u2f challenge and sign.
	chal, err := srv.Auth().GetMFAAuthenticateChallengeWithAuth(ctx, &proto.GetMFAAuthenticateChallengeWithAuthRequest{
		Username: username,
	})
	require.NoError(t, err)

	u2f, err := u2fKey.SignResponse(&u2f.SignRequest{
		Version:   chal.GetU2F()[0].Version,
		Challenge: chal.GetU2F()[0].Challenge,
		KeyHandle: chal.GetU2F()[0].KeyHandle,
		AppID:     chal.GetU2F()[0].AppID,
	})
	require.NoError(t, err)

	// Successfully create token with u2f auth.
	token2, err := srv.Auth().CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{
		Username: username,
		SecondFactorCred: &proto.CreatePrivilegeTokenRequest_U2FSignResponse{U2FSignResponse: &proto.U2FResponse{
			KeyHandle:  u2f.KeyHandle,
			ClientData: u2f.ClientData,
			Signature:  u2f.SignatureData,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, token2.GetSubKind(), UserTokenTypePrivilege)

	// Test lock from max failed auth attempts.
	for i := 0; i < defaults.MaxLoginAttempts; i++ {
		_, err := srv.Auth().CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{
			Username:         username,
			SecondFactorCred: &proto.CreatePrivilegeTokenRequest_U2FSignResponse{U2FSignResponse: &proto.U2FResponse{}},
		})
		require.Error(t, err)
	}

	// Test user is locked.
	user, err := srv.Auth().GetUser(username, false)
	require.NoError(t, err)
	require.True(t, user.GetStatus().IsLocked)
}

type debugAuth struct {
	proxies      []types.Server
	proxiesError bool
	clusterName  string
}

func (s *debugAuth) GetProxies() ([]types.Server, error) {
	if s.proxiesError {
		return nil, trace.BadParameter("failed to fetch proxies")
	}
	return s.proxies, nil
}

func (s *debugAuth) GetDomainName() (string, error) {
	if s.clusterName == "" {
		return "", trace.NotFound("no cluster name set")
	}
	return s.clusterName, nil
}
