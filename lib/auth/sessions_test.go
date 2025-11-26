// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

func TestCreateWebSession(t *testing.T) {
	t.Parallel()

	const userLlama = "llama"
	tenHours := time.Hour * 10
	eightHours := time.Hour * 8

	testCases := []struct {
		name                   string
		webIdleTimeout         *time.Duration
		sessionTTL             time.Duration
		expectedBearerTokenTTL time.Duration
	}{
		{
			name:                   "bearerTokenExpiry equal webidletimeout",
			webIdleTimeout:         &tenHours,
			expectedBearerTokenTTL: tenHours,
			sessionTTL:             time.Hour * 12,
		},
		{
			name:                   "bearerTokenExpiry is sessionTTL when shorter than webidletimeout",
			webIdleTimeout:         &tenHours,
			sessionTTL:             eightHours,
			expectedBearerTokenTTL: eightHours,
		},
		{
			name:                   "bearerTokenExpiry defaults to 10 minutes when webidletimeout not configured",
			webIdleTimeout:         nil,
			sessionTTL:             time.Hour * 12,
			expectedBearerTokenTTL: defaults.BearerTokenTTL,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clusterNetworkConfig := types.DefaultClusterNetworkingConfig()
			if tc.webIdleTimeout != nil {
				clusterNetworkConfig.SetWebIdleTimeout(*tc.webIdleTimeout)
			}

			fakeclock := clockwork.NewFakeClock()
			testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
				Clock:                   fakeclock,
				Dir:                     t.TempDir(),
				ClusterNetworkingConfig: clusterNetworkConfig,
			})
			require.NoError(t, err, "NewAuthServer failed")
			t.Cleanup(func() {
				assert.NoError(t, testAuthServer.Close(), "testAuthServer.Close() errored")
			})

			authServer := testAuthServer.AuthServer
			ctx := context.Background()

			_, _, err = authtest.CreateUserAndRole(authServer, userLlama, []string{userLlama} /* logins */, nil /* allowRules */)
			require.NoError(t, err, "CreateUserAndRole failed")

			session, err := authServer.CreateWebSessionFromReq(ctx, auth.NewWebSessionRequest{
				User:       userLlama,
				SessionTTL: tc.sessionTTL,
			})
			require.NoError(t, err, "CreateWebSessionFromReq failed")

			bearerTokenExpiry := session.GetBearerTokenExpiryTime()
			actualTTL := fakeclock.Until(bearerTokenExpiry)
			require.Equal(t, tc.expectedBearerTokenTTL, actualTTL)
		})
	}
}

func TestServer_CreateWebSessionFromReq_deviceWebToken(t *testing.T) {
	t.Parallel()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err, "NewAuthServer failed")
	t.Cleanup(func() {
		assert.NoError(t, testAuthServer.Close(), "testAuthServer.Close() errored")
	})

	authServer := testAuthServer.AuthServer

	var storedWebTokens utils.SyncMap[string, *devicepb.DeviceWebToken]
	authServer.SetCreateDeviceWebTokenFunc(func(ctx context.Context, dwt *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
		if dwt.BrowserMaxTouchPoints > 1 {
			// Simulate CreateDeviceWebToken not creating tokens for iPads.
			return nil, nil
		}

		dwt.Id = uuid.NewString()
		dwt.Token = uuid.NewString()

		storedWebTokens.Store(dwt.Id, dwt)

		return &devicepb.DeviceWebToken{
			Id:    dwt.Id,
			Token: dwt.Token,
		}, nil
	})

	const userLlama = "llama"
	user, _, err := authtest.CreateUserAndRole(authServer, userLlama, []string{userLlama} /* logins */, nil /* allowRules */)
	require.NoError(t, err, "CreateUserAndRole failed")

	// Arbitrary, real-looking values.
	const loginIP = "40.89.244.232"
	const loginUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

	tests := []struct {
		name                string
		loginMaxTouchPoints int
		wantWebToken        bool
	}{
		{
			name:                "macOS",
			loginMaxTouchPoints: 0,
			wantWebToken:        true,
		},
		{
			name:                "iPadOS",
			loginMaxTouchPoints: 5,
			wantWebToken:        false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session, err := authServer.CreateWebSessionFromReq(t.Context(), auth.NewWebSessionRequest{
				User:                 userLlama,
				LoginIP:              loginIP,
				LoginUserAgent:       loginUserAgent,
				LoginMaxTouchPoints:  test.loginMaxTouchPoints,
				Roles:                user.GetRoles(),
				Traits:               user.GetTraits(),
				SessionTTL:           1 * time.Minute,
				LoginTime:            time.Now(),
				CreateDeviceWebToken: true,
			})
			require.NoError(t, err, "CreateWebSessionFromReq failed")

			gotToken := session.GetDeviceWebToken()
			if !test.wantWebToken {
				require.Nil(t, gotToken, "device web token was created for this session")
				return
			}

			require.NotNil(t, gotToken, "device web token was not created for this session")
			storedWebToken, ok := storedWebTokens.Load(gotToken.Id)
			require.True(t, ok, "created web token was not found")

			require.Equal(t, storedWebToken.Token, gotToken.Token)
			require.Equal(t, loginIP, storedWebToken.BrowserIp)
			require.Equal(t, loginUserAgent, storedWebToken.BrowserUserAgent)
			require.Equal(t, test.loginMaxTouchPoints, int(storedWebToken.BrowserMaxTouchPoints))
		})
	}
}
