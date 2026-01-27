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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
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

			fakeClock := clockwork.NewFakeClock()
			testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
				Clock:                   fakeClock,
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
			actualTTL := fakeClock.Until(bearerTokenExpiry)
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

	fakeClock := clockwork.NewFakeClock()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Clock: fakeClock,
		Dir:   t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { testAuthServer.Close() })

	authServer := testAuthServer.AuthServer

	tests := []struct {
		name             string
		trustMode        string
		botName          string
		deviceExtensions auth.DeviceExtensions
		wantErr          string
	}{
		{
			name:             "Access Denied - Trusted Device Required but Missing",
			trustMode:        constants.DeviceTrustModeRequired,
			deviceExtensions: auth.DeviceExtensions{},
			wantErr:          "requires a trusted device",
		},
		{
			name:             "Success - Trust Optional and Device Missing",
			trustMode:        constants.DeviceTrustModeOptional,
			deviceExtensions: auth.DeviceExtensions{},
		},
		{
			name:             "Success - Trust Mode Not Set",
			trustMode:        "",
			deviceExtensions: auth.DeviceExtensions{},
		},
		{
			name:      "Success - Trusted Device Required and Provided",
			trustMode: constants.DeviceTrustModeRequired,
			deviceExtensions: auth.DeviceExtensions{
				DeviceID:     "macbook-id-123",
				AssetTag:     "asset-tag-123",
				CredentialID: "cred-id-123",
			},
		},
		{
			name:             "Success - Bot Access with RequiredForHumans and Missing Device",
			trustMode:        constants.DeviceTrustModeRequiredForHumans,
			botName:          "example-bot",
			deviceExtensions: auth.DeviceExtensions{},
		},
		{
			name:             "Access Denied - Bot Access with Device Trust Required",
			trustMode:        constants.DeviceTrustModeRequired,
			botName:          "example-bot",
			deviceExtensions: auth.DeviceExtensions{},
			wantErr:          "requires a trusted device",
		},
		{
			name:             "Access Denied - Human Access with RequiredForHumans but Missing Device",
			trustMode:        constants.DeviceTrustModeRequiredForHumans,
			botName:          "",
			deviceExtensions: auth.DeviceExtensions{},
			wantErr:          "requires a trusted device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			suffix := uuid.NewString()
			username := "user-" + suffix
			roleName := "role-" + suffix
			publicAddr := "www-" + suffix + ".example.com"

			role, err := types.NewRole(roleName, types.RoleSpecV6{
				Options: types.RoleOptions{
					DeviceTrustMode: tt.trustMode,
				},
				Allow: types.RoleConditions{
					AppLabels: types.Labels{"*": []string{"*"}},
				},
			})
			require.NoError(t, err)
			_, err = authServer.CreateRole(ctx, role)
			require.NoError(t, err)

			traits := map[string][]string{
				"groups": {"admins", "devs"},
				"email":  {"alice@example.com"},
			}
			user, err := types.NewUser(username)
			require.NoError(t, err)
			user.SetTraits(traits)
			user.AddRole(roleName)
			_, err = authServer.CreateUser(ctx, user)
			require.NoError(t, err)

			identity := tlsca.Identity{
				Username:         username,
				BotName:          tt.botName,
				DeviceExtensions: tlsca.DeviceExtensions(tt.deviceExtensions),
			}

			cName, err := authServer.GetClusterName(ctx)
			require.NoError(t, err)
			clusterName := cName.GetClusterName()

			checker, err := services.NewAccessChecker(&services.AccessInfo{
				Username: username,
				Roles:    user.GetRoles(),
			}, clusterName, authServer)
			require.NoError(t, err)

			req := &proto.CreateAppSessionRequest{
				Username:    username,
				AppName:     "example-app",
				URI:         "http://example.com",
				ClusterName: clusterName,
				PublicAddr:  publicAddr,
			}

			startTime := fakeClock.Now()
			_, err = authServer.CreateAppSession(ctx, req, identity, checker)
			endTime := fakeClock.Now()

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.True(t, trace.IsAccessDenied(err), "Expected AccessDenied, got %v", err)
			} else {
				require.NoError(t, err)
			}

			logEntries, _, err := testAuthServer.AuditLog.SearchEvents(ctx, events.SearchEventsRequest{
				From:  startTime.Add(-time.Second),
				To:    endTime.Add(time.Second),
				Limit: 100, // arbitrary, enough to allow for events from concurrent tests
			})
			require.NoError(t, err)

			expectedKind := apievents.UserKind_USER_KIND_HUMAN
			if tt.botName != "" {
				expectedKind = apievents.UserKind_USER_KIND_BOT
			}

			expectedEvent := &apievents.AppSessionStart{
				Metadata: apievents.Metadata{
					Type:        events.AppSessionStartEvent,
					ClusterName: clusterName,
				},
				UserMetadata: apievents.UserMetadata{
					User:            username,
					BotName:         tt.botName,
					UserKind:        expectedKind,
					UserClusterName: clusterName,
					UserRoles:       []string{roleName},
					UserTraits:      traits,
				},
				AppMetadata: apievents.AppMetadata{
					AppName:       "example-app",
					AppURI:        "http://example.com",
					AppPublicAddr: publicAddr,
				},
			}

			if tt.wantErr == "" {
				expectedEvent.Metadata.Code = events.AppSessionStartCode
				expectedEvent.SessionMetadata.PrivateKeyPolicy = "none"
			} else {
				expectedEvent.Metadata.Code = events.AppSessionStartFailureCode
				expectedEvent.UserMessage = "requires a trusted device"
				expectedEvent.Error = "access to resource requires a trusted device"
			}

			if tt.deviceExtensions.DeviceID != "" {
				expectedEvent.UserMetadata.TrustedDevice = &apievents.DeviceMetadata{
					DeviceId:     tt.deviceExtensions.DeviceID,
					AssetTag:     tt.deviceExtensions.AssetTag,
					CredentialId: tt.deviceExtensions.CredentialID,
				}
			}

			var eventFound bool
			for _, event := range logEntries {
				appStart, ok := event.(*apievents.AppSessionStart)
				if !ok || appStart.UserMetadata.User != username {
					continue
				}

				eventFound = true

				diff := cmp.Diff(expectedEvent, appStart,
					cmpopts.IgnoreUnexported(
						apievents.AppSessionStart{},
						apievents.Metadata{},
						apievents.UserMetadata{},
						apievents.AppMetadata{},
						apievents.SessionMetadata{},
						apievents.ConnectionMetadata{},
						apievents.ServerMetadata{},
						apievents.DeviceMetadata{},
					),
					cmpopts.IgnoreFields(apievents.Metadata{}, "ID", "Time", "Index"),
					cmpopts.IgnoreFields(apievents.ServerMetadata{}, "ServerID", "ServerVersion", "ServerNamespace"),
					cmpopts.IgnoreFields(apievents.ConnectionMetadata{}, "RemoteAddr", "Protocol"),
					cmpopts.IgnoreFields(apievents.SessionMetadata{}, "SessionID"),
					cmpopts.IgnoreFields(apievents.AppSessionStart{}, "PublicAddr"), // deprecated field
				)

				require.Empty(t, diff, "Audit event mismatch for case: %s\n%s", tt.name, diff)
			}
			require.True(t, eventFound, "Expected AppSessionStart event was not found in audit log")
		})
	}
}
