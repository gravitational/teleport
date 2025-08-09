/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/web"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMFAAuthenticateChallenge_IsMFARequiredApp(t *testing.T) {
	ctx := context.Background()

	appAccessRole, err := types.NewRole("app-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels(map[string]utils.Strings{
				"name": []string{"root-app", "leaf-app"},
			}),
		},
	})
	require.NoError(t, err)

	appAccessMfaRole, err := types.NewRole("app-access-mfa", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels(map[string]utils.Strings{
				"name": []string{"root-app-mfa", "leaf-app-mfa"},
			}),
		},
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
	})
	require.NoError(t, err)

	user, err := types.NewUser("app-user")
	require.NoError(t, err)
	user.SetRoles([]string{"app-access", "app-access-mfa"})

	rootAppServer := testserver.StartDummyHTTPServer("root-app")
	t.Cleanup(rootAppServer.Close)

	rootAppMFAServer := testserver.StartDummyHTTPServer("root-app-mfa")
	t.Cleanup(rootAppMFAServer.Close)

	// Create root and leaf cluster.
	rootServer, err := testserver.NewTeleportProcess(t.TempDir(),
		testserver.WithBootstrap(appAccessRole, appAccessMfaRole, user),
		testserver.WithClusterName("root"),
		testserver.WithTestApp("root-app", rootAppServer.URL),
		testserver.WithTestApp("root-app-mfa", rootAppMFAServer.URL),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SSH.Enabled = false
			cfg.Auth.Preference = &types.AuthPreferenceV2{
				Metadata: types.Metadata{
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
				Spec: types.AuthPreferenceSpecV2{
					AllowPasswordless: types.NewBoolOption(true),
					SecondFactors:     []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN},
					Webauthn: &types.Webauthn{
						RPID: "127.0.0.1",
					},
				},
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, rootServer.Close())
		require.NoError(t, rootServer.Wait())
	})

	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	leafAppServer := testserver.StartDummyHTTPServer("leaf-app")
	t.Cleanup(leafAppServer.Close)

	leafAppMFAServer := testserver.StartDummyHTTPServer("leaf-app-mfa")
	t.Cleanup(leafAppMFAServer.Close)

	leafServer, err := testserver.NewTeleportProcess(t.TempDir(),
		testserver.WithBootstrap(appAccessRole, appAccessMfaRole),
		testserver.WithClusterName("leaf"),
		testserver.WithTestApp("leaf-app", leafAppServer.URL),
		testserver.WithTestApp("leaf-app-mfa", leafAppMFAServer.URL),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SSH.Enabled = false
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, leafServer.Close())
		require.NoError(t, leafServer.Wait())
	})
	leafAuth := leafServer.GetAuthServer()

	SetupTrustedCluster(ctx, t, rootServer, leafServer,
		types.RoleMapping{
			Remote: "app-access",
			Local:  []string{"app-access"},
		},
		types.RoleMapping{
			Remote: "app-access-mfa",
			Local:  []string{"app-access-mfa"},
		},
	)

	// Require Session MFA in the leaf only.
	leafAccess, err := leafAuth.GetRole(ctx, "access")
	require.NoError(t, err)
	o := leafAccess.GetOptions()
	o.RequireMFAType = types.RequireMFAType_SESSION
	leafAccess.SetOptions(o)
	_, err = leafAuth.UpsertRole(ctx, leafAccess)
	require.NoError(t, err)

	// Setup user for login, then login.
	device := RegisterPasswordlessDeviceForUser(t, rootServer, user.GetName())
	webPack := helpers.LoginMFAWebClient(t, rootProxyAddr.String(), device)

	endpoint, err := url.JoinPath("mfa", "authenticatechallenge")
	require.NoError(t, err)

	for _, tt := range []struct {
		name              string
		resolveAppParams  web.ResolveAppParams
		expectMFARequired bool
	}{
		{
			name: "root-app",
			resolveAppParams: web.ResolveAppParams{
				AppName:     "root-app",
				ClusterName: "root",
				PublicAddr:  "root-app.root",
				FQDNHint:    "root-app.root",
			},
			expectMFARequired: false,
		}, {
			name: "root-app-mfa",
			resolveAppParams: web.ResolveAppParams{
				AppName:     "root-app-mfa",
				ClusterName: "root",
				PublicAddr:  "root-app-mfa.root",
				FQDNHint:    "root-app-mfa.root",
			},
			expectMFARequired: true,
		}, {
			name: "leaf-app",
			resolveAppParams: web.ResolveAppParams{
				AppName:     "leaf-app",
				ClusterName: "leaf",
				PublicAddr:  "leaf-app.leaf",
				FQDNHint:    "leaf-app.root",
			},
			expectMFARequired: false,
		}, {
			name: "leaf-app-mfa",
			resolveAppParams: web.ResolveAppParams{
				AppName:     "leaf-app-mfa",
				ClusterName: "leaf",
				PublicAddr:  "leaf-app-mfa.leaf",
				FQDNHint:    "leaf-app-mfa.root",
			},
			expectMFARequired: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Check each different way to resolve the app.
			for variant, resolveParams := range map[string]web.ResolveAppParams{
				"name": {
					AppName:     tt.resolveAppParams.AppName,
					ClusterName: tt.resolveAppParams.ClusterName,
				},
				"publicAddr": {
					ClusterName: tt.resolveAppParams.ClusterName,
					PublicAddr:  tt.resolveAppParams.PublicAddr,
				},
				"fqdn": {
					FQDNHint: tt.resolveAppParams.FQDNHint,
				},
			} {
				t.Run(variant, func(t *testing.T) {
					req := web.CreateAuthenticateChallengeRequest{
						ChallengeScope: int(mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION),
						IsMFARequiredRequest: &web.IsMFARequiredRequest{
							App: &web.IsMFARequiredApp{
								ResolveAppParams: resolveParams,
							},
						},
					}

					respStatusCode, respBody := webPack.DoWebAPIRequest(t, http.MethodPost, endpoint, req)
					require.Equal(t, http.StatusOK, respStatusCode, string(respBody))

					var resp client.MFAAuthenticateChallenge
					require.NoError(t, json.Unmarshal(respBody, &resp))

					if tt.expectMFARequired {
						require.NotEmpty(t, resp.WebauthnChallenge)
					} else {
						require.Empty(t, resp.WebauthnChallenge)
					}
				})
			}
		})
	}
}

// RegisterPasswordlessDeviceForUser registers a mocked passwordless device for the user
// to use during login or pass other routine MFA checks. The cluster auth preference should
// be set up to allow webauthn and passwordless before calling this helper.
func RegisterPasswordlessDeviceForUser(t *testing.T, server *service.TeleportProcess, username string) *mocku2f.Key {
	proxyAddr, err := server.ProxyWebAddr()
	require.NoError(t, err)
	origin := fmt.Sprintf("https://%v", proxyAddr.Host())

	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()

	ctx := context.Background()
	token, err := server.GetAuthServer().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: username,
	})
	require.NoError(t, err)

	tokenID := token.GetName()
	res, err := server.GetAuthServer().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:     tokenID,
		DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	require.NoError(t, err)
	cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

	ccr, err := device.SignCredentialCreation(origin, cc)
	require.NoError(t, err)
	_, err = server.GetAuthServer().ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID: tokenID,
		NewMFARegisterResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
			},
		},
	})
	require.NoError(t, err)

	return device
}

func SetupTrustedCluster(ctx context.Context, t *testing.T, rootServer, leafServer *service.TeleportProcess, additionalRoleMappings ...types.RoleMapping) {
	// Use insecure mode so that the trusted cluster can establish trust over reverse tunnel.
	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(isInsecure) })

	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)
	rootProxyTunnelAddr, err := rootServer.ProxyTunnelAddr()
	require.NoError(t, err)

	tc, err := types.NewTrustedCluster(rootServer.Config.Auth.ClusterName.GetClusterName(), types.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                testserver.StaticToken,
		ProxyAddress:         rootProxyAddr.String(),
		ReverseTunnelAddress: rootProxyTunnelAddr.String(),
		RoleMap: append(additionalRoleMappings,
			types.RoleMapping{
				Remote: "access",
				Local:  []string{"access"},
			},
		),
	})
	require.NoError(t, err)

	_, err = leafServer.GetAuthServer().UpsertTrustedClusterV2(ctx, tc)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rt, err := rootServer.GetAuthServer().GetTunnelConnections(leafServer.Config.Auth.ClusterName.GetClusterName())
		assert.NoError(t, err)
		assert.Len(t, rt, 1)
	}, time.Second*10, time.Second)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rts, err := rootServer.GetAuthServer().GetRemoteClusters(ctx)
		assert.NoError(t, err)
		assert.Len(t, rts, 1)
	}, time.Second*10, time.Second)
}
