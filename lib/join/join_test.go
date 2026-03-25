// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package join_test

import (
	"context"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joinv1proto "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authtest"
	authjoin "github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/join/joinv1"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

// TestJoinToken tests the full cycle of proxy and node joining via the join service.
//
// It first sets up a fake auth service running the gRPC join service.
//
// A fake proxy then joins to auth. It then uses its new credentials from the
// join process to connect to auth and serve it's own join service forwarding
// to the real service at auth.
//
// Finally, it tests various scenarios where a node attempts to join by
// connecting to the proxy's gRPC join service.
func TestJoinToken(t *testing.T) {
	t.Parallel()

	token1, err := types.NewProvisionTokenFromSpec("token1", time.Now().Add(time.Minute), types.ProvisionTokenSpecV2{
		Roles: []types.SystemRole{
			types.RoleInstance,
			types.RoleProxy,
			types.RoleNode,
			types.RoleApp,
		},
	})
	require.NoError(t, err)
	token2, err := types.NewProvisionTokenFromSpec("token2", time.Now().Add(time.Minute), types.ProvisionTokenSpecV2{
		Roles: []types.SystemRole{
			types.RoleInstance,
			types.RoleProxy,
			types.RoleNode,
			types.RoleDatabase,
		},
	})
	require.NoError(t, err)

	authService := newFakeAuthService(t)
	require.NoError(t, authService.Auth().UpsertToken(t.Context(), token1))
	require.NoError(t, authService.Auth().UpsertToken(t.Context(), token2))

	// generate scoped tokens
	scopedToken1 := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Scope:   "/aa",
		Metadata: &headerv1.Metadata{
			Name: "scoped1",
		},
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/aa/bb",
			Roles:         []string{types.RoleNode.String()},
			JoinMethod:    string(types.JoinMethodToken),
			UsageMode:     string(joining.TokenUsageModeUnlimited),
		},
		Status: &joiningv1.ScopedTokenStatus{
			Secret: "secret",
		},
	}
	scopedToken2 := proto.CloneOf(scopedToken1)
	scopedToken2.Spec.AssignedScope = "/aa/cc"
	scopedToken2.Metadata.Name = "scoped2"

	scopedToken3 := proto.CloneOf(scopedToken1)
	scopedToken3.Metadata.Name = "scoped3"

	singleUseToken := proto.CloneOf(scopedToken1)
	singleUseToken.Spec.UsageMode = string(joining.TokenUsageModeSingle)
	singleUseToken.Metadata.Name = "scoped-single-use-1"

	for _, tok := range []*joiningv1.ScopedToken{scopedToken1, scopedToken2, scopedToken3, singleUseToken} {
		_, err = authService.Auth().CreateScopedToken(t.Context(), &joiningv1.CreateScopedTokenRequest{
			Token: tok,
		})
		require.NoError(t, err)
	}

	proxy := newFakeProxy(authService)
	proxy.join(t)
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { proxyListener.Close() })
	proxy.runGRPCServer(t, proxyListener)

	t.Run("invalid token", func(t *testing.T) {
		_, err := joinViaProxy(
			t.Context(),
			"invalidtoken",
			proxyListener.Addr(),
		)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		ctx := t.Context()
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			evt, err := authService.lastEvent(ctx, "instance.join")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(
				&apievents.InstanceJoin{
					Metadata: apievents.Metadata{
						Type: "instance.join",
						Code: events.InstanceJoinFailureCode,
					},
					Status: apievents.Status{
						Success: false,
						Error:   "token expired or not found",
					},
					ConnectionMetadata: apievents.ConnectionMetadata{
						RemoteAddr: "127.0.0.1",
					},
					Role: "Instance",
				},
				evt,
				protocmp.Transform(),
				cmpopts.IgnoreMapEntries(func(key string, val any) bool {
					return key == "Time" || key == "ID"
				}),
			))
		}, 5*time.Second, 5*time.Millisecond, "expected instance.join failed event not found")
	})

	t.Run("join and rejoin", func(t *testing.T) {
		// Node initially joins by connecting to the proxy's gRPC service.
		identity, err := joinViaProxy(
			t.Context(),
			token1.GetName(),
			proxyListener.Addr(),
		)
		require.NoError(t, err)
		// Make sure the result contains a host ID and expected certificate roles.
		require.NotEmpty(t, identity.ID.HostUUID)
		require.Equal(t, types.RoleInstance, identity.ID.Role)
		expectedSystemRoles := slices.DeleteFunc(
			token1.GetRoles().StringSlice(),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, identity.SystemRoles)

		// Build an auth client with the new identity.
		tlsConfig, err := identity.TLSConfig(nil /*cipherSuites*/)
		require.NoError(t, err)
		authClient, err := authService.TLS.NewClientWithCert(tlsConfig.Certificates[0])
		require.NoError(t, err)

		// Node can rejoin with a different token by dialing the auth service
		// with an auth client authenticed with its original credentials.
		//
		// It should get back its original host ID and the combined roles of
		// its original certificate and the new token.
		newIdentity, err := rejoinViaAuthClient(
			t.Context(),
			token2.GetName(),
			authClient,
		)
		require.NoError(t, err)
		require.Equal(t, identity.ID, newIdentity.ID)
		expectedSystemRoles = slices.DeleteFunc(
			apiutils.Deduplicate(slices.Concat(
				token1.GetRoles().StringSlice(),
				token2.GetRoles().StringSlice(),
			)),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, newIdentity.SystemRoles)
	})

	t.Run("join and rejoin with scoped token", func(t *testing.T) {
		// Node initially joins by connecting to the proxy's gRPC service.
		identity, err := joinViaProxyWithSecret(
			t.Context(),
			scopedToken1.GetMetadata().GetName(),
			scopedToken1.GetStatus().GetSecret(),
			proxyListener.Addr(),
		)
		require.NoError(t, err)
		// Make sure the result contains a host ID and expected certificate roles.
		require.NotEmpty(t, identity.ID.HostUUID)
		require.Equal(t, types.RoleInstance, identity.ID.Role)
		expectedSystemRoles := slices.DeleteFunc(
			scopedToken1.GetSpec().GetRoles(),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, identity.SystemRoles)

		require.Equal(t, scopedToken1.GetSpec().GetAssignedScope(), identity.AgentScope)
		// Build an auth client with the new identity.
		tlsConfig, err := identity.TLSConfig(nil /*cipherSuites*/)
		require.NoError(t, err)
		authClient, err := authService.TLS.NewClientWithCert(tlsConfig.Certificates[0])
		require.NoError(t, err)

		// Node can rejoin with a different token assigning the same scope
		// by dialing the auth service with an auth client authenticated with
		// its original credentials.
		//
		// It should get back its original host ID and the combined roles of
		// its original certificate and the new token.
		newIdentity, err := rejoinViaAuthClientWithSecret(
			t.Context(),
			scopedToken3.GetMetadata().GetName(),
			scopedToken3.GetStatus().GetSecret(),
			authClient,
		)
		require.NoError(t, err)
		require.Equal(t, identity.AgentScope, newIdentity.AgentScope)
		require.Equal(t, identity.ID.HostUUID, newIdentity.ID.HostUUID)
		require.Equal(t, identity.ID.NodeName, newIdentity.ID.NodeName)
		require.Equal(t, identity.ID.Role, newIdentity.ID.Role)
		expectedSystemRoles = slices.DeleteFunc(
			apiutils.Deduplicate(slices.Concat(
				scopedToken1.GetSpec().GetRoles(),
				scopedToken3.GetSpec().GetRoles(),
			)),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, newIdentity.SystemRoles)
	})

	t.Run("join and rejoin with mismatched scoped tokens", func(t *testing.T) {
		// Node initially joins by connecting to the proxy's gRPC service.
		identity, err := joinViaProxyWithSecret(
			t.Context(),
			scopedToken1.GetMetadata().GetName(),
			scopedToken1.GetStatus().GetSecret(),
			proxyListener.Addr(),
		)
		require.NoError(t, err)
		// Make sure the result contains a host ID and expected certificate roles.
		require.NotEmpty(t, identity.ID.HostUUID)
		require.Equal(t, types.RoleInstance, identity.ID.Role)
		expectedSystemRoles := slices.DeleteFunc(
			scopedToken1.GetSpec().GetRoles(),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, identity.SystemRoles)

		require.Equal(t, scopedToken1.GetSpec().GetAssignedScope(), identity.AgentScope)
		// Build an auth client with the new identity.
		tlsConfig, err := identity.TLSConfig(nil /*cipherSuites*/)
		require.NoError(t, err)
		authClient, err := authService.TLS.NewClientWithCert(tlsConfig.Certificates[0])
		require.NoError(t, err)

		// Node cannot rejoin with a different token assigning a different scope.
		_, err = rejoinViaAuthClient(
			t.Context(),
			scopedToken2.GetMetadata().GetName(),
			authClient,
		)
		require.Error(t, err)
	})

	t.Run("join and rejoin with bad token", func(t *testing.T) {
		// Node joins by connecting to the proxy's gRPC service.
		identity, err := joinViaProxy(
			t.Context(),
			token1.GetName(),
			proxyListener.Addr(),
		)
		require.NoError(t, err)

		// Build an auth client with the new identity.
		tlsConfig, err := identity.TLSConfig(nil /*cipherSuites*/)
		require.NoError(t, err)
		authClient, err := authService.TLS.NewClientWithCert(tlsConfig.Certificates[0])
		require.NoError(t, err)

		// Node the tries to rejoin with valid certs but an invalid token.
		_, err = rejoinViaAuthClient(
			t.Context(),
			"invalidtoken",
			authClient,
		)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		ctx := t.Context()
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			evt, err := authService.lastEvent(ctx, "instance.join")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(
				&apievents.InstanceJoin{
					Metadata: apievents.Metadata{
						Type: "instance.join",
						Code: events.InstanceJoinFailureCode,
					},
					Status: apievents.Status{
						Success: false,
						Error:   "token expired or not found",
					},
					ConnectionMetadata: apievents.ConnectionMetadata{
						RemoteAddr: "127.0.0.1",
					},
					Role: "Instance",
				},
				evt,
				protocmp.Transform(),
				cmpopts.IgnoreMapEntries(func(key string, val any) bool {
					return key == "Time" || key == "ID"
				}),
			))
		}, 5*time.Second, 5*time.Millisecond, "expected instance.join failed event not found")
	})

	t.Run("join with single use scoped token", func(t *testing.T) {
		identity, err := joinViaProxyWithSecret(
			t.Context(),
			singleUseToken.GetMetadata().GetName(),
			singleUseToken.GetStatus().GetSecret(),
			proxyListener.Addr(),
		)
		require.NoError(t, err)
		// Make sure the result contains a host ID and expected certificate roles.
		require.NotEmpty(t, identity.ID.HostUUID)
		require.Equal(t, types.RoleInstance, identity.ID.Role)
		expectedSystemRoles := slices.DeleteFunc(
			singleUseToken.GetSpec().GetRoles(),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, identity.SystemRoles)
		require.Equal(t, singleUseToken.GetSpec().GetAssignedScope(), identity.AgentScope)

		// ensure subsequent join attempts fail
		_, err = joinViaProxyWithSecret(
			t.Context(),
			singleUseToken.GetMetadata().GetName(),
			singleUseToken.GetStatus().GetSecret(),
			proxyListener.Addr(),
		)
		require.ErrorContains(t, err, joining.ErrTokenExhausted.Error())
	})

	for i, tc := range []struct {
		name string
		// updateTokenFunc modifies the token after the initial join.
		updateTokenFunc func(token *joiningv1.ScopedToken)
		// assertRejoinExpectation is used at the end to assert whether rejoining has failed or not
		assertRejoinExpectation func(t *testing.T, identity *state.Identity, err error)
	}{
		{
			name: "join after upsert modifies assigned scope",
			updateTokenFunc: func(token *joiningv1.ScopedToken) {
				token.Spec.AssignedScope = "/aa/cc"
			},
			assertRejoinExpectation: func(t *testing.T, identity *state.Identity, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "join after upsert preserves assigned scope",
			updateTokenFunc: func(token *joiningv1.ScopedToken) {
				token.Metadata.Labels = map[string]string{"env": "updated"}
			},
			assertRejoinExpectation: func(t *testing.T, identity *state.Identity, err error) {
				require.NoError(t, err)
				require.Equal(t, "/aa/bb", identity.AgentScope)
				require.Equal(t, identity.ID.HostUUID, identity.ID.HostUUID)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Version: types.V1,
				Scope:   "/aa",
				Metadata: &headerv1.Metadata{
					Name: fmt.Sprintf("upsertcheck%d", i),
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
					UsageMode:     string(joining.TokenUsageModeUnlimited),
				},
				Status: &joiningv1.ScopedTokenStatus{
					Secret: "somesecret",
				},
			}
			_, err := authService.Auth().CreateScopedToken(t.Context(), &joiningv1.CreateScopedTokenRequest{
				Token: token,
			})
			require.NoError(t, err)

			// Join with the original assigned scope.
			identity, err := joinViaProxyWithSecret(
				t.Context(),
				token.GetMetadata().GetName(),
				token.GetStatus().GetSecret(),
				proxyListener.Addr(),
			)
			require.NoError(t, err)
			require.Equal(t, "/aa/bb", identity.AgentScope)

			// Change and upsert token
			fetchedRes, err := authService.Auth().GetScopedToken(t.Context(), &joiningv1.GetScopedTokenRequest{
				Name:       token.GetMetadata().GetName(),
				WithSecret: true,
			})
			require.NoError(t, err)
			updatedToken := proto.CloneOf(fetchedRes.GetToken())
			tc.updateTokenFunc(updatedToken)

			_, err = authService.Auth().UpsertScopedToken(t.Context(), &joiningv1.UpsertScopedTokenRequest{
				Token: updatedToken,
			})
			require.NoError(t, err)

			// Attempt to rejoin using the identity from the first join.
			tlsConfig, err := identity.TLSConfig(nil)
			require.NoError(t, err)
			authClient, err := authService.TLS.NewClientWithCert(tlsConfig.Certificates[0])
			require.NoError(t, err)

			newIdentity, err := rejoinViaAuthClientWithSecret(
				t.Context(),
				token.GetMetadata().GetName(),
				token.GetStatus().GetSecret(),
				authClient,
			)
			tc.assertRejoinExpectation(t, newIdentity, err)
		})
	}
}

// TestJoinError asserts that attempts to join with an invalid token return an
// AccessDenied error and do not fall back to joining via the legacy join
// service.
func TestJoinError(t *testing.T) {
	t.Parallel()

	token, err := types.NewProvisionTokenFromSpec("token1", time.Now().Add(time.Minute), types.ProvisionTokenSpecV2{
		Roles: []types.SystemRole{
			types.RoleNode,
			types.RoleProxy,
		},
	})
	require.NoError(t, err)

	authService := newFakeAuthService(t)
	require.NoError(t, authService.Auth().UpsertToken(t.Context(), token))

	proxy := newFakeProxy(authService)
	proxy.join(t)
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { proxyListener.Close() })
	proxy.runGRPCServer(t, proxyListener)

	// List on a free port just to guarantee an address that will reject/close
	// all connection attempts.
	badListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	testutils.RunTestBackgroundTask(t.Context(), t, &testutils.TestBackgroundTask{
		Name: "bad listener",
		Task: func(ctx context.Context) error {
			for {
				conn, err := badListener.Accept()
				if err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					return err
				}
				conn.Close()
			}
		},
		Terminate: badListener.Close,
	})

	// Assert that the real AccessDenied error is returned with various
	// configurations joining via an auth or proxy address.
	for _, tc := range []struct {
		desc       string
		joinParams joinclient.JoinParams
		assertErr  assert.ErrorAssertionFunc
	}{
		{
			desc: "auth direct",
			joinParams: joinclient.JoinParams{
				AuthServers: []utils.NetAddr{utils.FromAddr(authService.TLS.Listener.Addr())},
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...any) bool {
				// Should get AccessDenied and should not fall back to joining
				// via the legacy service.
				return assert.ErrorAs(t, err, new(*trace.AccessDeniedError)) &&
					assert.NotErrorAs(t, err, new(*joinclient.LegacyJoinError))
			},
		},
		{
			// With teleport config v2 or certain bot configurations a proxy
			// address is passed in AuthServers, which supports both auth and
			// proxy addresses.
			desc: "proxy as auth",
			joinParams: joinclient.JoinParams{
				AuthServers: []utils.NetAddr{utils.FromAddr(proxyListener.Addr())},
				Insecure:    true,
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...any) bool {
				// Should get AccessDenied and should not fall back to joining
				// via the legacy service.
				return assert.ErrorAs(t, err, new(*trace.AccessDeniedError)) &&
					assert.NotErrorAs(t, err, new(*joinclient.LegacyJoinError))
			},
		},
		{
			desc: "proxy direct",
			joinParams: joinclient.JoinParams{
				ProxyServer: utils.FromAddr(proxyListener.Addr()),
				Insecure:    true,
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...any) bool {
				// Should get AccessDenied and should not fall back to joining
				// via the legacy service.
				return assert.ErrorAs(t, err, new(*trace.AccessDeniedError)) &&
					assert.NotErrorAs(t, err, new(*joinclient.LegacyJoinError))
			},
		},
		{
			desc: "bad auth address",
			joinParams: joinclient.JoinParams{
				AuthServers: []utils.NetAddr{utils.FromAddr(badListener.Addr())},
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...any) bool {
				// Should fall back to a legacy join attempt before failing.
				return assert.ErrorAs(t, err, new(*joinclient.LegacyJoinError))
			},
		},
		{
			desc: "bad proxy address",
			joinParams: joinclient.JoinParams{
				ProxyServer: utils.FromAddr(badListener.Addr()),
			},
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...any) bool {
				// Should fall back to a legacy join attempt before failing.
				return assert.ErrorAs(t, err, new(*joinclient.LegacyJoinError))
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			joinParams := tc.joinParams
			joinParams.ID = state.IdentityID{
				Role:     types.RoleInstance,
				NodeName: "test",
			}

			joinParams.Token = "invalid"
			_, err = joinclient.Join(t.Context(), joinParams)
			tc.assertErr(t, err)
		})
	}
}

type fakeAuthService struct {
	*authtest.Server
}

func newFakeAuthService(t *testing.T) *fakeAuthService {
	testServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:         t.TempDir(),
			ClusterName: "testcluster",
		},
	})
	require.NoError(t, err)
	return &fakeAuthService{
		Server: testServer,
	}
}

func (s *fakeAuthService) lastEvent(ctx context.Context, eventType string) (apievents.AuditEvent, error) {
	return lastEvent(ctx, s.Auth(), s.Auth().GetClock(), eventType)
}

func lastEvent(ctx context.Context, auditLog events.AuditLogger, clock clockwork.Clock, eventType string) (apievents.AuditEvent, error) {
	events, _, err := auditLog.SearchEvents(ctx, events.SearchEventsRequest{
		From:       clock.Now().Add(-time.Hour),
		To:         clock.Now().Add(time.Hour),
		EventTypes: []string{eventType},
		Limit:      1,
		Order:      types.EventOrderDescending,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(events) == 0 {
		return nil, trace.NotFound("no matching events")
	}
	return events[0], nil
}

type fakeProxy struct {
	auth     *fakeAuthService
	identity *state.Identity
}

func newFakeProxy(auth *fakeAuthService) *fakeProxy {
	return &fakeProxy{
		auth: auth,
	}
}

func (p *fakeProxy) join(t *testing.T) {
	unauthenticatedAuthClt, err := p.auth.NewClient(authtest.TestNop())
	require.NoError(t, err)

	joinResult, err := joinclient.Join(t.Context(), joinclient.JoinParams{
		Token: "token1",
		ID: state.IdentityID{
			Role:     types.RoleInstance,
			NodeName: "proxy",
		},
		AuthClient:           unauthenticatedAuthClt,
		DNSNames:             []string{"proxy"},
		AdditionalPrincipals: []string{"127.0.0.1"},
	})
	require.NoError(t, err)

	privateKeyPEM, err := keys.MarshalPrivateKey(joinResult.PrivateKey)
	require.NoError(t, err)
	p.identity, err = state.ReadIdentityFromKeyPair(privateKeyPEM, joinResult.Certs)
	require.NoError(t, err)
}

func (p *fakeProxy) runGRPCServer(t *testing.T, l net.Listener) {
	tlsConfig, err := p.identity.TLSConfig(nil /*cipherSuites*/)
	require.NoError(t, err)
	// Set NextProtos such that the ALPN conn upgrade test passes.
	tlsConfig.NextProtos = []string{string(constants.ALPNSNIProtocolReverseTunnel), string(common.ProtocolProxyGRPCInsecure), http2.NextProtoTLS}

	grpcCreds := credentials.NewTLS(tlsConfig)

	authenticatedAuthClientConn, err := grpc.NewClient(p.auth.TLS.Listener.Addr().String(),
		grpc.WithTransportCredentials(grpcCreds),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authenticatedAuthClientConn.Close())
	})

	grpcServer := grpc.NewServer(
		grpc.Creds(grpcCreds),
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)
	joinv1.RegisterProxyForwardingJoinServiceServer(grpcServer, joinv1proto.NewJoinServiceClient(authenticatedAuthClientConn))

	testutils.RunTestBackgroundTask(t.Context(), t, &testutils.TestBackgroundTask{
		Name: "proxy gRPC server",
		Task: func(ctx context.Context) error {
			return trace.Wrap(grpcServer.Serve(l))
		},
		Terminate: func() error {
			grpcServer.Stop()
			return nil
		},
	})
}

func joinViaProxy(ctx context.Context, token string, addr net.Addr) (*state.Identity, error) {
	return joinViaProxyWithSecret(ctx, token, "", addr)
}

func joinViaProxyWithSecret(
	ctx context.Context,
	token string,
	tokenSecret string,
	addr net.Addr,
) (*state.Identity, error) {
	joinResult, err := joinclient.Join(ctx, joinclient.JoinParams{
		Token:       token,
		TokenSecret: tokenSecret,
		ID: state.IdentityID{
			Role:     types.RoleInstance,
			NodeName: "node",
		},
		ProxyServer: utils.NetAddr{
			AddrNetwork: addr.Network(),
			Addr:        addr.String(),
		},
		AdditionalPrincipals: []string{"node"},
		// The proxy's TLS cert for the test is not trusted.
		Insecure: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(joinResult.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return state.ReadIdentityFromKeyPair(privateKeyPEM, joinResult.Certs)
}

func rejoinViaAuthClient(ctx context.Context, token string, authClient authjoin.AuthJoinClient) (*state.Identity, error) {
	return rejoinViaAuthClientWithSecret(ctx, token, "", authClient)
}

func rejoinViaAuthClientWithSecret(
	ctx context.Context,
	token string,
	tokenSecret string,
	authClient authjoin.AuthJoinClient,
) (*state.Identity, error) {
	joinResult, err := joinclient.Join(ctx, joinclient.JoinParams{
		Token:       token,
		TokenSecret: tokenSecret,
		ID: state.IdentityID{
			Role:     types.RoleInstance,
			NodeName: "node",
		},
		AdditionalPrincipals: []string{"node"},
		AuthClient:           authClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(joinResult.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return state.ReadIdentityFromKeyPair(privateKeyPEM, joinResult.Certs)
}
