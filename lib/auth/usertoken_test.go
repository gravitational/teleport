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
	"encoding/base32"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	wanpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
)

func TestCreateResetPasswordToken(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	mockEmitter := &eventstest.MockRecorderEmitter{}
	srv.Auth().emitter = mockEmitter

	// Configure cluster and user for MFA, registering various devices.
	mfa := configureForMFA(t, srv)
	username := mfa.User
	pass := mfa.Password

	ctx := context.Background()
	req := authclient.CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	token, err := srv.Auth().CreateResetPasswordToken(ctx, req)
	require.NoError(t, err)
	require.Equal(t, token.GetUser(), username)
	require.Equal(t, token.GetURL(), "https://<proxyhost>:3080/web/reset/"+token.GetName())

	event := mockEmitter.LastEvent()
	require.Equal(t, events.ResetPasswordTokenCreateEvent, event.GetType())
	require.Equal(t, username, event.(*apievents.UserTokenCreate).Name)
	require.Equal(t, teleport.UserSystem, event.(*apievents.UserTokenCreate).User)

	// verify that user has no MFA devices
	devs, err := srv.Auth().Services.GetMFADevices(ctx, username, false)
	require.NoError(t, err)
	require.Empty(t, devs)

	// verify that password was reset
	err = srv.Auth().checkPasswordWOToken(ctx, username, []byte(pass))
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
	ctx := context.Background()

	username := "joe@example.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username}, nil)
	require.NoError(t, err)

	type testCase struct {
		desc string
		req  authclient.CreateUserTokenRequest
	}

	testCases := []testCase{
		{
			desc: "Reset Password: TTL < 0",
			req: authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  -1,
			},
		},
		{
			desc: "Reset Password: TTL > max",
			req: authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  defaults.MaxChangePasswordTokenTTL + time.Hour,
			},
		},
		{
			desc: "Reset Password: empty user name",
			req: authclient.CreateUserTokenRequest{
				TTL: time.Hour,
			},
		},
		{
			desc: "Reset Password: user does not exist",
			req: authclient.CreateUserTokenRequest{
				Name: "doesnotexist@example.com",
				TTL:  time.Hour,
			},
		},
		{
			desc: "Invite: TTL > max",
			req: authclient.CreateUserTokenRequest{
				Name: username,
				TTL:  defaults.MaxSignupTokenTTL + time.Hour,
				Type: authclient.UserTokenTypeResetPasswordInvite,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := srv.Auth().CreateResetPasswordToken(ctx, tc.req)
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
							PublicAddrs: []string{"foo"},
							Version:     "bar",
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
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username}, nil)
	require.NoError(t, err)

	ctx := context.Background()

	req := authclient.CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	token, err := srv.Auth().CreateResetPasswordToken(ctx, req)
	require.NoError(t, err)

	_, err = srv.Auth().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:    token.GetName(),
		DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
	})
	require.NoError(t, err)

	secrets, err := srv.Auth().GetUserTokenSecrets(ctx, token.GetName())
	require.NoError(t, err)

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
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username}, nil)
	require.NoError(t, err)

	req := authclient.CreateUserTokenRequest{
		Name: username,
		TTL:  time.Hour,
		Type: authclient.UserTokenTypeResetPasswordInvite,
	}

	token, err := srv.Auth().newUserToken(req)
	require.NoError(t, err)
	require.Equal(t, req.Name, token.GetUser())
	require.Equal(t, req.Type, token.GetSubKind())
	require.Equal(t, token.GetURL(), "https://<proxyhost>:3080/web/invite/"+token.GetName())
	require.NotEmpty(t, token.GetCreated())
	require.NotEmpty(t, token.GetMetadata().Expires)
}

func TestCreatePrivilegeToken(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	fakeClock := srv.Clock().(*clockwork.FakeClock)
	mockEmitter := &eventstest.MockRecorderEmitter{}
	srv.Auth().emitter = mockEmitter
	ctx := context.Background()

	// Create a user and client with identity.
	username := "joe@example.com"
	_, _, err := CreateUserAndRoleWithoutRoles(srv.Auth(), username, []string{username})
	require.NoError(t, err)
	clt, err := srv.NewClient(TestUser(username))
	require.NoError(t, err)

	// Test a failure when second factor isn't enabled.
	_, err = clt.CreatePrivilegeToken(ctx, &proto.CreatePrivilegeTokenRequest{})
	require.True(t, trace.IsAccessDenied(err))

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOTP,
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	tests := []struct {
		name      string
		tokenType string
		getReq    func() *proto.CreatePrivilegeTokenRequest
	}{
		{
			name:      "privilege exception token",
			tokenType: authclient.UserTokenTypePrivilegeException,
			getReq: func() *proto.CreatePrivilegeTokenRequest {
				return &proto.CreatePrivilegeTokenRequest{}
			},
		},
		{
			name:      "privilege token",
			tokenType: authclient.UserTokenTypePrivilege,
			getReq: func() *proto.CreatePrivilegeTokenRequest {
				// Upsert a TOTP device to authn with.
				otpSecret := base32.StdEncoding.EncodeToString([]byte("def456"))
				dev, err := services.NewTOTPDevice("otp", otpSecret, fakeClock.Now())
				require.NoError(t, err)

				err = srv.Auth().UpsertMFADevice(ctx, username, dev)
				require.NoError(t, err)

				totpCode, err := totp.GenerateCode(otpSecret, srv.Clock().Now())
				require.NoError(t, err)

				return &proto.CreatePrivilegeTokenRequest{
					ExistingMFAResponse: &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
						TOTP: &proto.TOTPResponse{Code: totpCode},
					}},
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := clt.CreatePrivilegeToken(ctx, tc.getReq())
			require.NoError(t, err)
			require.Equal(t, tc.tokenType, token.GetSubKind())
			require.Equal(t, username, token.GetUser())

			// Test events emitted.
			event := mockEmitter.LastEvent()
			require.Equal(t, events.PrivilegeTokenCreateEvent, event.GetType())
			require.Equal(t, events.PrivilegeTokenCreateCode, event.GetCode())
			require.Equal(t, username, event.(*apievents.UserTokenCreate).Name)
			require.Equal(t, username, event.(*apievents.UserTokenCreate).User)

			// Test token expires after designated time.
			fakeClock.Advance(defaults.PrivilegeTokenTTL)
			_, err = srv.Auth().GetUserToken(context.Background(), token.GetName())
			require.True(t, trace.IsNotFound(err))
		})
	}
}

func TestCreatePrivilegeToken_WithLock(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)
	ctx := context.Background()

	// Enable second factor.
	ap, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: "teleport",
		},
	})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertAuthPreference(ctx, ap)
	require.NoError(t, err)

	tests := []struct {
		name   string
		getReq func() *proto.CreatePrivilegeTokenRequest
	}{
		{
			name: "locked from totp attempts",
			getReq: func() *proto.CreatePrivilegeTokenRequest {
				return &proto.CreatePrivilegeTokenRequest{
					ExistingMFAResponse: &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
						TOTP: &proto.TOTPResponse{Code: "wrong-otp-token-value"},
					}},
				}
			},
		},
		{
			name: "locked from webauthn attempts",
			getReq: func() *proto.CreatePrivilegeTokenRequest {
				return &proto.CreatePrivilegeTokenRequest{
					ExistingMFAResponse: &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
						Webauthn: &wanpb.CredentialAssertionResponse{},
					}},
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create a user and client with identity.
			username := fmt.Sprintf("llama%v@goteleport.com", rand.Int())
			_, _, err := CreateUserAndRoleWithoutRoles(srv.Auth(), username, []string{username})
			require.NoError(t, err)
			clt, err := srv.NewClient(TestUser(username))
			require.NoError(t, err)

			// Test lock from max failed auth attempts.
			for i := 1; i <= defaults.MaxLoginAttempts; i++ {
				_, err := clt.CreatePrivilegeToken(ctx, tc.getReq())
				require.True(t, trace.IsAccessDenied(err))

				// Test last attempt returns locked error.
				if i == defaults.MaxLoginAttempts {
					require.Equal(t, MaxFailedAttemptsErrMsg, err.Error())
				} else {
					require.NotEqual(t, MaxFailedAttemptsErrMsg, err.Error())
				}
			}

			// Test user is locked.
			user, err := srv.Auth().GetUser(ctx, username, false)
			require.NoError(t, err)
			require.True(t, user.GetStatus().IsLocked)
			require.False(t, user.GetStatus().LockExpires.IsZero())
		})
	}
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
