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
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
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
