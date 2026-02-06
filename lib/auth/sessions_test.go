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
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
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

// TestCreateAppSession_DeviceTrust validates the core Device Trust enforcement logic.
// It tests every permutation of Device Trust modes and against both human users and bots.
func TestCreateAppSession_DeviceTrust(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClock()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Clock: fakeClock,
		Dir:   t.TempDir(),
	})
	require.NoError(t, err, "NewAuthServer failed")
	t.Cleanup(func() {
		assert.NoError(t, testAuthServer.Close(), "testAuthServer.Close() errored")
	})

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
			appName := "app-" + suffix
			publicAddr := "www-" + suffix + ".example.com"

			app, err := types.NewAppV3(types.Metadata{
				Name: appName,
			}, types.AppSpecV3{
				URI: "http://example.com",
			})
			require.NoError(t, err)
			require.NoError(t, authServer.CreateApp(ctx, app))

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
				AppName:     appName,
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
					AppName:       appName,
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

func TestCreateAppSession_UntrustedDevice(t *testing.T) {
	t.Parallel()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { testAuthServer.Close() })

	// The first role allows access to all apps and doesn't require a trusted device.
	allowAllWithoutDeviceTrust, err := types.NewRole("all-apps-any-device", types.RoleSpecV6{
		Options: types.RoleOptions{DeviceTrustMode: constants.DeviceTrustModeOptional},
		Allow:   types.RoleConditions{AppLabels: types.Labels{"*": []string{"*"}}},
	})
	require.NoError(t, err)
	_, err = testAuthServer.AuthServer.CreateRole(t.Context(), allowAllWithoutDeviceTrust)
	require.NoError(t, err)

	// The second role requires a trusted device, but only applies to apps in prod.
	requireDeviceTrustForProd, err := types.NewRole("prod-trusted-device", types.RoleSpecV6{
		Options: types.RoleOptions{DeviceTrustMode: constants.DeviceTrustModeRequired},
		Allow:   types.RoleConditions{AppLabels: types.Labels{"env": []string{"prod"}}},
	})
	require.NoError(t, err)
	_, err = testAuthServer.AuthServer.CreateRole(t.Context(), requireDeviceTrustForProd)
	require.NoError(t, err)

	// Create a user with both roles.
	user, err := types.NewUser("bob")
	require.NoError(t, err)
	user.AddRole(allowAllWithoutDeviceTrust.GetName())
	user.AddRole(requireDeviceTrustForProd.GetName())
	_, err = testAuthServer.AuthServer.CreateUser(t.Context(), user)
	require.NoError(t, err)

	devApp, err := types.NewAppV3(types.Metadata{
		Name:   "dev-app",
		Labels: map[string]string{"env": "dev"},
	}, types.AppSpecV3{
		URI: "http://dev-app.example.com",
	})
	require.NoError(t, err)
	require.NoError(t, testAuthServer.AuthServer.CreateApp(t.Context(), devApp))

	prodApp, err := types.NewAppV3(types.Metadata{
		Name:   "prod-app",
		Labels: map[string]string{"env": "prod"},
	}, types.AppSpecV3{
		URI: "http://prod-app.example.com",
	})
	require.NoError(t, err)
	require.NoError(t, testAuthServer.AuthServer.CreateApp(t.Context(), prodApp))

	cName, err := testAuthServer.AuthServer.GetClusterName(t.Context())
	require.NoError(t, err)
	clusterName := cName.GetClusterName()

	checker, err := services.NewAccessChecker(&services.AccessInfo{
		Username: user.GetName(),
		Roles:    user.GetRoles(),
	}, clusterName, testAuthServer.AuthServer)
	require.NoError(t, err)

	for _, test := range []struct {
		app     types.Application
		assert  require.ErrorAssertionFunc
		wantErr string
	}{
		{app: prodApp, assert: require.Error, wantErr: "requires a trusted device"},
		{app: devApp, assert: require.NoError},
	} {
		t.Run(test.app.GetName(), func(t *testing.T) {
			req := &proto.CreateAppSessionRequest{
				Username:    user.GetName(),
				AppName:     test.app.GetName(),
				URI:         test.app.GetURI(),
				PublicAddr:  test.app.GetPublicAddr(),
				ClusterName: clusterName,
			}
			// simulate a user identity WITHOUT a trusted device
			identity := tlsca.Identity{Username: user.GetName()}

			_, err = testAuthServer.AuthServer.CreateAppSession(t.Context(), req, identity, checker)
			if test.wantErr == "" {
				assert.NoError(t, err, "CreateAppSession errored unexpectedly")
				return
			}
			assert.ErrorContains(t, err, test.wantErr, "CreateAppSession error mismatch")
		})
	}
}

func BenchmarkCreateAppSession(b *testing.B) {
	// Enable Enterprise features
	modulestest.SetTestModules(b, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
				entitlements.App:         {Enabled: true},
			},
		},
	})

	b.StopTimer()
	ctx := context.Background()
	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: b.TempDir(),
	})
	if err != nil {
		b.Fatalf("NewAuthServer failed: %v", err)
	}
	defer testAuthServer.Close()
	authServer := testAuthServer.AuthServer

	// Set up multiple roles for AccessChecker to evaluate
	roleOptional, _ := types.NewRole("optional-role", types.RoleSpecV6{
		Options: types.RoleOptions{DeviceTrustMode: constants.DeviceTrustModeOptional},
		Allow:   types.RoleConditions{AppLabels: types.Labels{"*": []string{"*"}}},
	})
	authServer.CreateRole(ctx, roleOptional)

	roleRequired, _ := types.NewRole("required-role", types.RoleSpecV6{
		Options: types.RoleOptions{DeviceTrustMode: constants.DeviceTrustModeRequired},
		Allow:   types.RoleConditions{AppLabels: types.Labels{"env": []string{"prod"}}},
	})
	authServer.CreateRole(ctx, roleRequired)

	username := "bench-user"
	user, _ := types.NewUser(username)
	user.AddRole(roleOptional.GetName())
	user.AddRole(roleRequired.GetName())
	authServer.CreateUser(ctx, user)

	appName := "prod-app"
	app, _ := types.NewAppV3(types.Metadata{
		Name:   appName,
		Labels: map[string]string{"env": "prod"},
	}, types.AppSpecV3{URI: "http://prod.example.com"})
	authServer.CreateApp(ctx, app)

	cName, _ := authServer.GetClusterName(ctx)
	clusterName := cName.GetClusterName()

	benchmarks := []struct {
		name         string
		appName      string
		hasDevice    bool
		expectDenied bool
	}{
		{
			name:         "Success_App_With_Device",
			appName:      "prod-app",
			hasDevice:    true,
			expectDenied: false,
		},
		{
			name:         "Fail_App_No_Device",
			appName:      "prod-app",
			hasDevice:    false,
			expectDenied: true,
		},
		{
			name:         "Pass_Static_App",
			appName:      "static-app", // should pass early because app isn't found
			hasDevice:    false,
			expectDenied: false,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			identity := tlsca.Identity{
				Username: username,
				Groups:   []string{"required-role", "optional-role"},
			}
			if bm.hasDevice {
				identity.DeviceExtensions = tlsca.DeviceExtensions{
					DeviceID:     "macbook-id-123",
					AssetTag:     "asset-tag-123",
					CredentialID: "cred-id-123",
				}
			}

			checker, err := services.NewAccessChecker(&services.AccessInfo{
				Username: username,
				Roles:    user.GetRoles(),
			}, clusterName, authServer)
			if err != nil {
				b.Fatalf("NewAccessChecker failed: %v", err)
			}

			req := &proto.CreateAppSessionRequest{
				Username:    username,
				AppName:     bm.appName,
				ClusterName: clusterName,
			}

			_, err = authServer.CreateAppSession(ctx, req, identity, checker)
			if bm.expectDenied {
				if err == nil || !trace.IsAccessDenied(err) {
					b.Fatalf("First call failed: expected access denied, got %v", err)
				}
			} else {
				if err != nil {
					b.Fatalf("First call failed: setup is incorrect for %s: %v", bm.name, err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := authServer.CreateAppSession(ctx, req, identity, checker)
				if bm.expectDenied {
					if err == nil || !trace.IsAccessDenied(err) {
						b.Fatal("expected access denied")
					}
				} else if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
