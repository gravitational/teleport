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
	"net"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/constants"
	joinv1proto "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
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
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

// TestJoin tests the full cycle of proxy and node joining via the join service.
//
// It first sets up a fake auth service running the gRPC join service.
//
// A fake proxy then joins to auth. It then uses its new credentials from the
// join process to connect to auth and serve it's own join service forwarding
// to the real service at auth.
//
// Finally, it tests various scenarios where a node attempts to join by
// connecting to the proxy's gRPC join service.
func TestJoin(t *testing.T) {
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
	events, _, err := s.Auth().SearchEvents(ctx, events.SearchEventsRequest{
		From:       s.Auth().GetClock().Now().Add(-time.Hour),
		To:         s.Auth().GetClock().Now().Add(time.Hour),
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

func joinViaProxy(
	ctx context.Context,
	token string,
	addr net.Addr,
) (*state.Identity, error) {
	joinResult, err := joinclient.Join(ctx, joinclient.JoinParams{
		Token: token,
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

func rejoinViaAuthClient(
	ctx context.Context,
	token string,
	authClient authjoin.AuthJoinClient,
) (*state.Identity, error) {
	joinResult, err := joinclient.Join(ctx, joinclient.JoinParams{
		Token: token,
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
