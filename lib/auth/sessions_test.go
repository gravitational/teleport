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

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
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
	ctx := context.Background()

	// Wire a fake CreateDeviceWebTokenFunc to authServer.
	fakeWebToken := &devicepb.DeviceWebToken{
		Id:    "423f10ed-c3c1-4de7-99dc-3bc5b9ab7fd5",
		Token: "409d21e4-9563-497f-9393-1209f9e4289c",
	}
	wantToken := &types.DeviceWebToken{
		Id:    fakeWebToken.Id,
		Token: fakeWebToken.Token,
	}
	authServer.SetCreateDeviceWebTokenFunc(func(ctx context.Context, dwt *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
		return fakeWebToken, nil
	})

	const userLlama = "llama"
	user, _, err := authtest.CreateUserAndRole(authServer, userLlama, []string{userLlama} /* logins */, nil /* allowRules */)
	require.NoError(t, err, "CreateUserAndRole failed")

	// Arbitrary, real-looking values.
	const loginIP = "40.89.244.232"
	const loginUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

	t.Run("ok", func(t *testing.T) {
		session, err := authServer.CreateWebSessionFromReq(ctx, auth.NewWebSessionRequest{
			User:                 userLlama,
			LoginIP:              loginIP,
			LoginUserAgent:       loginUserAgent,
			Roles:                user.GetRoles(),
			Traits:               user.GetTraits(),
			SessionTTL:           1 * time.Minute,
			LoginTime:            time.Now(),
			CreateDeviceWebToken: true,
		})
		require.NoError(t, err, "CreateWebSessionFromReq failed")

		gotToken := session.GetDeviceWebToken()
		if diff := cmp.Diff(wantToken, gotToken); diff != "" {
			t.Errorf("CreateWebSessionFromReq DeviceWebToken mismatch (-want +got)\n%s", diff)
		}
	})
}

func TestCreateAppSession_DeviceTrust(t *testing.T) {
	t.Parallel()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { testAuthServer.Close() })

	authServer := testAuthServer.AuthServer
	ctx := context.Background()

	const username = "example-user"
	roleName := "example-role"

	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Options: types.RoleOptions{
			DeviceTrustMode: constants.DeviceTrustModeRequired,
		},
		Allow: types.RoleConditions{
			AppLabels: types.Labels{"*": []string{"*"}},
		},
	})
	require.NoError(t, err)
	_, err = authServer.CreateRole(ctx, role)
	require.NoError(t, err)

	user, err := types.NewUser(username)
	require.NoError(t, err)
	user.AddRole(roleName)

	_, err = authServer.CreateUser(ctx, user)
	require.NoError(t, err)

	_, err = authServer.CreateAppSessionFromReq(ctx, auth.NewAppSessionRequest{
		NewWebSessionRequest: auth.NewWebSessionRequest{
			User:       username,
			SessionTTL: 1 * time.Hour,
			Roles:      user.GetRoles(),
			Traits:     user.GetTraits(),
		},
		AppName:    "example-app",
		AppURI:     "http://example.com",
		PublicAddr: "www.example.com",
	})

	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err), "Expected AccessDenied error, got %v", err)
	require.Contains(t, err.Error(), "requires a trusted device")

	logEntries, _, err := testAuthServer.AuditLog.SearchEvents(ctx, events.SearchEventsRequest{
		From:  time.Now().Add(-1 * time.Minute),
		To:    time.Now().Add(1 * time.Minute),
		Limit: 10,
	})
	foundSuccess := false
	foundFailure := false

	for _, event := range logEntries {
		if event.GetType() == events.AppSessionStartEvent {
			foundSuccess = true
		}

		if event.GetType() == events.AuthAttemptEvent && event.GetCode() == events.AuthAttemptFailureCode {
			if authEvent, ok := event.(*apievents.AuthAttempt); ok {
				if authEvent.AppMetadata.AppPublicAddr == "www.example.com" {
					foundFailure = true
				}
			}
		}
	}

	require.False(t, foundSuccess, "BUG: 'app.session.start' was emitted.")
	require.True(t, foundFailure, "Missing 'auth.fail' (AuthAttempt) event for device trust rejection.")
}
