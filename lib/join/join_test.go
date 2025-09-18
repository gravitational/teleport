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
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/testing/protocmp"

	joinv1proto "github.com/gravitational/teleport/api/gen/proto/go/teleport/join/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/join/joinv1"
	"github.com/gravitational/teleport/lib/tlsca"
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
		_, _, err := join(
			t.Context(),
			proxyListener.Addr(),
			insecure.NewCredentials(),
			"invalidtoken",
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
		// Node joins by connecting to the proxy's gRPC service.
		joinResult, signer, err := join(
			t.Context(),
			proxyListener.Addr(),
			insecure.NewCredentials(),
			token1.GetName(),
		)
		// Make sure the result contains a host ID and expected certificate roles.
		require.NoError(t, err)
		require.NotNil(t, joinResult.HostID)
		require.NotEmpty(t, joinResult.HostID)
		cert, err := x509.ParseCertificate(joinResult.Certificates.TLSCert)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		require.NoError(t, err)
		require.Len(t, identity.Groups, 1)
		require.Equal(t, identity.Groups[0], types.RoleInstance.String())
		expectedSystemRoles := slices.DeleteFunc(
			token1.GetRoles().StringSlice(),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, identity.SystemRoles)

		// Node can rejoin with a different token by dialing the auth service
		// with its original credentials (for this test we omit the details of
		// the proxy's mTLS tunnel dialing and let the node dial auth
		// directly).
		//
		// It should get back its original host ID and the combined roles of
		// its original certificate and the new token.
		creds, err := clientCreds(signer, joinResult.Certificates)
		require.NoError(t, err)
		rejoinResult, _, err := join(
			t.Context(),
			authService.TLS.Listener.Addr(),
			creds,
			token2.GetName(),
		)
		require.NoError(t, err)
		cert, err = x509.ParseCertificate(rejoinResult.Certificates.TLSCert)
		require.NoError(t, err)
		identity, err = tlsca.FromSubject(cert.Subject, cert.NotAfter)
		require.NoError(t, err)
		require.Len(t, identity.Groups, 1)
		require.Equal(t, identity.Groups[0], types.RoleInstance.String())
		expectedSystemRoles = slices.DeleteFunc(
			apiutils.Deduplicate(slices.Concat(
				token1.GetRoles().StringSlice(),
				token2.GetRoles().StringSlice(),
			)),
			func(s string) bool { return s == types.RoleInstance.String() },
		)
		require.ElementsMatch(t, expectedSystemRoles, identity.SystemRoles)

		// The node gets back its original host ID when rejoining with an
		// authenticated client.
		require.Equal(t, joinResult.HostID, rejoinResult.HostID)
	})

	t.Run("join and rejoin with bad token", func(t *testing.T) {
		// Node joins by connecting to the proxy's gRPC service.
		joinResult, signer, err := join(
			t.Context(),
			proxyListener.Addr(),
			insecure.NewCredentials(),
			token1.GetName(),
		)
		require.NoError(t, err)

		// Node the tries to rejoin with valid certs but an invalid token.
		creds, err := clientCreds(signer, joinResult.Certificates)
		require.NoError(t, err)
		_, _, err = join(
			t.Context(),
			authService.TLS.Listener.Addr(),
			creds,
			"invalidtoken",
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
	auth                   *fakeAuthService
	authenticatedAuthCreds credentials.TransportCredentials
}

func newFakeProxy(auth *fakeAuthService) *fakeProxy {
	return &fakeProxy{
		auth: auth,
	}
}

func (p *fakeProxy) join(t *testing.T) {
	unauthenticatedAuthClt, err := p.auth.NewClient(authtest.TestNop())
	require.NoError(t, err)
	joinClient := joinv1.NewClient(unauthenticatedAuthClt.JoinV1Client())

	// Initiate the join request and get a client stream.
	stream, err := joinClient.Join(t.Context())
	require.NoError(t, err)

	// Send the ClientInit messaage.
	require.NoError(t, stream.Send(&messages.ClientInit{
		TokenName:  "token1",
		SystemRole: types.RoleInstance.String(),
	}))

	// Wait for the ServerInit response.
	serverInit, err := messages.RecvResponse[*messages.ServerInit](stream)
	require.NoError(t, err)

	require.Equal(t, string(types.JoinMethodToken), serverInit.JoinMethod)

	// Generate host keys with the suite from the ServerInit message.
	hostKeys, err := genHostKeys(t.Context(), serverInit.SignatureAlgorithmSuite)
	require.NoError(t, err)

	// Send the TokenInit message.
	require.NoError(t, stream.Send(&messages.TokenInit{
		ClientParams: messages.ClientParams{
			HostParams: &messages.HostParams{
				PublicKeys: messages.PublicKeys{
					PublicTLSKey: hostKeys.tlsPubKey,
					PublicSSHKey: hostKeys.sshPubKey,
				},
				HostName:             "proxy",
				AdditionalPrincipals: []string{"proxy"},
			},
		},
	}))

	// Wait for the result from the server.
	result, err := messages.RecvResponse[*messages.HostResult](stream)
	require.NoError(t, err)

	// Save the host credentials we got from the successful join.
	p.authenticatedAuthCreds, err = clientCreds(hostKeys.tls, result.Certificates)
	require.NoError(t, err)
}

func (p *fakeProxy) runGRPCServer(t *testing.T, l net.Listener) {
	authenticatedAuthClientConn, err := grpc.NewClient(p.auth.TLS.Listener.Addr().String(),
		grpc.WithTransportCredentials(p.authenticatedAuthCreds),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authenticatedAuthClientConn.Close())
	})

	grpcServer := grpc.NewServer(
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

func join(
	ctx context.Context,
	addr net.Addr,
	creds credentials.TransportCredentials,
	token string,
) (*messages.HostResult, crypto.Signer, error) {
	conn, err := grpc.NewClient(addr.String(),
		grpc.WithTransportCredentials(creds),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer conn.Close()
	joinClient := joinv1.NewClient(joinv1proto.NewJoinServiceClient(conn))

	// Initiate the join request.
	stream, err := joinClient.Join(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Send the ClientInit message.
	err = stream.Send(&messages.ClientInit{
		TokenName:  token,
		SystemRole: types.RoleInstance.String(),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Wait for the ServerInit response.
	serverInit, err := messages.RecvResponse[*messages.ServerInit](stream)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Generate host keys with the suite from the ServerInit message.
	hostKeys, err := genHostKeys(ctx, serverInit.SignatureAlgorithmSuite)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Send the TokenInit message with the host keys.
	if err := stream.Send(&messages.TokenInit{
		ClientParams: messages.ClientParams{
			HostParams: &messages.HostParams{
				PublicKeys: messages.PublicKeys{
					PublicTLSKey: hostKeys.tlsPubKey,
					PublicSSHKey: hostKeys.sshPubKey,
				},
				HostName: "node",
			},
		},
	}); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Wait for the result.
	result, err := messages.RecvResponse[*messages.HostResult](stream)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return result, hostKeys.tls, nil
}

func clientCreds(tlsKey crypto.PrivateKey, certs messages.Certificates) (credentials.TransportCredentials, error) {
	caPool := x509.NewCertPool()
	for _, caCertDER := range certs.TLSCACerts {
		caCert, err := x509.ParseCertificate(caCertDER)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		caPool.AddCert(caCert)
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{certs.TLSCert},
			PrivateKey:  tlsKey,
		}},
		RootCAs:    caPool,
		ServerName: "teleport.cluster.local",
	}), nil
}

type hostKeys struct {
	tls       crypto.Signer
	tlsPubKey []byte
	ssh       ssh.Signer
	sshPubKey []byte
}

func genHostKeys(ctx context.Context, suite types.SignatureAlgorithmSuite) (*hostKeys, error) {
	signer, err := cryptosuites.GenerateKey(ctx, cryptosuites.StaticAlgorithmSuite(suite), cryptosuites.HostIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPubKey, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshKey, err := ssh.NewSignerFromSigner(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPubKey := sshKey.PublicKey().Marshal()
	return &hostKeys{
		tls:       signer,
		tlsPubKey: tlsPubKey,
		ssh:       sshKey,
		sshPubKey: sshPubKey,
	}, nil
}
