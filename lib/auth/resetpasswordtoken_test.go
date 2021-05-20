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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

func TestCreateResetPasswordToken(t *testing.T) {
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

	req := CreateResetPasswordTokenRequest{
		Name: username,
		TTL:  time.Hour,
	}

	token, err := srv.Auth().CreateResetPasswordToken(ctx, req)
	require.NoError(t, err)
	require.Equal(t, token.GetUser(), username)
	require.Equal(t, token.GetURL(), "https://<proxyhost>:3080/web/reset/"+token.GetName())

	event := mockEmitter.LastEvent()
	require.Equal(t, event.GetType(), events.ResetPasswordTokenCreateEvent)
	require.Equal(t, event.(*events.ResetPasswordTokenCreate).Name, "joe@example.com")
	require.Equal(t, event.(*events.ResetPasswordTokenCreate).User, teleport.UserSystem)

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
	tokens, err := srv.Auth().GetResetPasswordTokens(ctx)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, tokens[0].GetName(), token.GetName())
}

func TestCreateResetPasswordTokenErrors(t *testing.T) {
	srv := newTestTLSServer(t)

	username := "joe@example.com"
	_, _, err := CreateUserAndRole(srv.Auth(), username, []string{username})
	require.NoError(t, err)

	type testCase struct {
		desc string
		req  CreateResetPasswordTokenRequest
	}

	testCases := []testCase{
		{
			desc: "Reset Password: TTL < 0",
			req: CreateResetPasswordTokenRequest{
				Name: username,
				TTL:  -1,
			},
		},
		{
			desc: "Reset Password: TTL > max",
			req: CreateResetPasswordTokenRequest{
				Name: username,
				TTL:  defaults.MaxChangePasswordTokenTTL + time.Hour,
			},
		},
		{
			desc: "Reset Password: empty user name",
			req: CreateResetPasswordTokenRequest{
				TTL: time.Hour,
			},
		},
		{
			desc: "Reset Password: user does not exist",
			req: CreateResetPasswordTokenRequest{
				Name: "doesnotexist@example.com",
				TTL:  time.Hour,
			},
		},
		{
			desc: "Invite: TTL > max",
			req: CreateResetPasswordTokenRequest{
				Name: username,
				TTL:  defaults.MaxSignupTokenTTL + time.Hour,
				Type: ResetPasswordTokenTypeInvite,
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
