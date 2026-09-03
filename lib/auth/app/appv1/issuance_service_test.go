// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package appv1_test

import (
	"cmp"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport/api/client/proto"
	appv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/app/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestIssuanceService_IssueAppOIDCToken(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now()),
	})
	require.NoError(t, err)
	tlsServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { tlsServer.Close() })
	clock := tlsServer.Clock()

	// OIDC IdP CA signing key for token verification.
	oidcCA, err := tlsServer.Auth().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: tlsServer.ClusterName(),
	}, true)
	require.NoError(t, err)
	oidcSigner, err := tlsServer.Auth().GetKeyStore().GetJWTSigner(ctx, oidcCA)
	require.NoError(t, err)
	oidcKey, err := services.GetJWTSigner(oidcSigner, oidcCA.GetClusterName(), clock)
	require.NoError(t, err)

	// User and an app session for that user against each app.
	user, role, err := authtest.CreateUserAndRole(
		tlsServer.Auth(),
		"foo@example.com",
		[]string{"bar"},
		nil,
		authtest.WithUserMutator(func(u types.User) {
			u.SetTraits(map[string][]string{
				"team": {"engineering"},
			})
		}),
		authtest.WithRoleMutator(func(r types.Role) {
			r.SetOptions(types.RoleOptions{
				MaxSessionTTL: types.NewDuration(20 * time.Minute),
			})
		}),
	)
	require.NoError(t, err)
	userClient, err := tlsServer.NewClient(authtest.TestUser(user.GetName()))
	require.NoError(t, err)

	// The OIDC issuer URL is derived from a registered proxy.
	proxyServer, err := types.NewServer("proxy-hostname", types.KindProxy, types.ServerSpecV2{
		PublicAddrs: []string{"https://teleport.example.com"},
	})
	require.NoError(t, err)
	_, err = tlsServer.Auth().UpsertProxyServer(ctx, proxyServer)
	require.NoError(t, err)

	supportedApp, err := types.NewAppV3(types.Metadata{
		Name: "allowed-app",
	}, types.AppSpecV3{
		URI:        "mcp+https://localhost:8080",
		PublicAddr: "testapp.example.com",
	})
	require.NoError(t, err)
	unsupportedApp, err := types.NewAppV3(types.Metadata{
		Name: "unsupported-app",
	}, types.AppSpecV3{
		URI:        "https://localhost:9090",
		PublicAddr: "unsupported-app.example.com",
	})
	require.NoError(t, err)
	noClaimsApp, err := types.NewAppV3(types.Metadata{
		Name: "testapp-no-claims",
	}, types.AppSpecV3{
		URI:        "mcp+https://localhost:8081",
		PublicAddr: "testapp-no-claims.example.com",
		// Strips roles and traits from minted tokens.
		Rewrite: &types.Rewrite{
			JWTClaims: types.JWTClaimsRewriteNone,
		},
	})
	require.NoError(t, err)

	const appHostID = "app-host-id"

	// Setup sessions and app servers.
	sessions := make(map[string]types.WebSession)
	for _, app := range []*types.AppV3{supportedApp, unsupportedApp, noClaimsApp} {
		appServer, err := types.NewAppServerV3FromApp(app, "host", appHostID)
		require.NoError(t, err)
		_, err = tlsServer.Auth().UpsertApplicationServer(ctx, appServer)
		require.NoError(t, err)
		session, err := userClient.CreateAppSession(ctx, &proto.CreateAppSessionRequest{
			Username:    user.GetName(),
			PublicAddr:  app.GetPublicAddr(),
			ClusterName: tlsServer.ClusterName(),
			AppName:     app.GetName(),
			URI:         app.GetURI(),
		})
		require.NoError(t, err)
		sessions[app.GetName()] = session
	}

	// Extract raw DER cert from a session for the UserCertificate field.
	sessionCertDER := func(appName string) []byte {
		cert, err := tlsca.ParseCertificatePEM(sessions[appName].GetTLSCert())
		require.NoError(t, err)
		return cert.Raw
	}

	tests := []struct {
		name       string
		role       types.SystemRole
		hostID     string
		req        *appv1pb.IssueAppOIDCTokenRequest
		assertErr  require.ErrorAssertionFunc
		assertResp func(t *testing.T, token string)
	}{
		{
			name: "non-app role rejected",
			role: types.RoleProxy,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[supportedApp.GetName()].GetName(),
				Ttl:          durationpb.New(time.Minute),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
			},
		},
		{
			name: "missing user certificate and session id",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				Ttl: durationpb.New(time.Minute),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
			},
		},
		{
			name: "missing ttl",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[supportedApp.GetName()].GetName(),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
			},
		},
		{
			name: "non-existent session",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: "does-not-exist",
				Ttl:          durationpb.New(time.Minute),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
			},
		},
		{
			name: "ttl exceeds maximum",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[supportedApp.GetName()].GetName(),
				Ttl:          durationpb.New(time.Hour * 2),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
			},
		},
		{
			name: "unsupported app rejected",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[unsupportedApp.GetName()].GetName(),
				Ttl:          durationpb.New(time.Minute),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
			},
		},
		{
			name:   "host id mismatch",
			role:   types.RoleApp,
			hostID: "wrong-host-id",
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[supportedApp.GetName()].GetName(),
				Ttl:          durationpb.New(time.Minute),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
			},
		},
		{
			name: "invalid user certificate",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId:    sessions[supportedApp.GetName()].GetName(),
				Ttl:             durationpb.New(time.Minute),
				UserCertificate: []byte("invalid-cert"),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
			},
		},
		{
			name: "session ID mismatch",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId:    "wrong-session-id",
				UserCertificate: sessionCertDER(supportedApp.GetName()),
				Ttl:             durationpb.New(time.Minute),
			}.Build(),
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
			},
		},
		{
			name: "success with session id",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[supportedApp.GetName()].GetName(),
				Ttl:          durationpb.New(time.Minute),
			}.Build(),
			assertErr: require.NoError,
			assertResp: func(t *testing.T, token string) {
				claims, err := oidcKey.Verify(jwt.VerifyParams{
					Username: user.GetName(),
					RawToken: token,
					Audience: supportedApp.GetURI(),
					Issuer:   "https://teleport.example.com",
				})
				require.NoError(t, err)
				require.Equal(t, user.GetName(), claims.Username)
				require.Equal(t, []string{role.GetName()}, claims.Roles)
				require.Equal(t, []string{"engineering"}, claims.Traits["team"])
			},
		},
		{
			name: "success with user certificate",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId:    sessions[supportedApp.GetName()].GetName(),
				UserCertificate: sessionCertDER(supportedApp.GetName()),
				Ttl:             durationpb.New(time.Minute),
			}.Build(),
			assertErr: require.NoError,
			assertResp: func(t *testing.T, token string) {
				claims, err := oidcKey.Verify(jwt.VerifyParams{
					Username: user.GetName(),
					RawToken: token,
					Audience: supportedApp.GetURI(),
					Issuer:   "https://teleport.example.com",
				})
				require.NoError(t, err)
				require.Equal(t, user.GetName(), claims.Username)
				require.Equal(t, []string{role.GetName()}, claims.Roles)
				require.Equal(t, []string{"engineering"}, claims.Traits["team"])
			},
		},
		{
			name: "success with claims stripped by app rewrite",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[noClaimsApp.GetName()].GetName(),
				Ttl:          durationpb.New(time.Minute),
			}.Build(),
			assertErr: require.NoError,
			assertResp: func(t *testing.T, token string) {
				claims, err := oidcKey.Verify(jwt.VerifyParams{
					Username: user.GetName(),
					RawToken: token,
					Audience: noClaimsApp.GetURI(),
					Issuer:   "https://teleport.example.com",
				})
				require.NoError(t, err)
				require.Equal(t, user.GetName(), claims.Username)
				require.Empty(t, claims.Roles)
				require.Empty(t, claims.Traits)
			},
		},
		{
			name: "ttl clamped to identity expiry",
			role: types.RoleApp,
			req: appv1pb.IssueAppOIDCTokenRequest_builder{
				AppSessionId: sessions[supportedApp.GetName()].GetName(),
				Ttl:          durationpb.New(30 * time.Minute),
			}.Build(),
			assertErr: require.NoError,
			assertResp: func(t *testing.T, token string) {
				claims, err := oidcKey.Verify(jwt.VerifyParams{
					Username: user.GetName(),
					RawToken: token,
					Audience: supportedApp.GetURI(),
					Issuer:   "https://teleport.example.com",
				})
				require.NoError(t, err)
				sessExpiry := sessions[supportedApp.GetName()].GetExpiryTime()
				require.False(t, claims.Expiry.Time().After(sessExpiry),
					"token expiry %v should not exceed session expiry %v", claims.Expiry.Time(), sessExpiry)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hostID := cmp.Or(tc.hostID, appHostID)
			client, err := tlsServer.NewClient(authtest.TestServerID(tc.role, hostID))
			require.NoError(t, err)

			resp, err := client.AppIssuanceClient().IssueAppOIDCToken(ctx, tc.req)
			tc.assertErr(t, err)
			if tc.assertResp != nil {
				require.NotNil(t, resp)
				tc.assertResp(t, resp.GetToken())
			}
		})
	}
}
