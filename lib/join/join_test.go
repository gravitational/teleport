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
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/keygen"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/join/joinv1"
	"github.com/gravitational/teleport/lib/join/messages"
	"github.com/gravitational/teleport/lib/join/server"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
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
	authService.validTokens[token1.GetName()] = token1
	authService.validTokens[token2.GetName()] = token2

	authListener := bufconn.Listen(8192)
	t.Cleanup(func() { authListener.Close() })
	authService.runGRPCServer(t, authListener)

	proxy := newFakeProxy(authListener.DialContext, authService.unauthenticatedClientCreds())
	proxy.join(t)
	proxyListener := bufconn.Listen(8192)
	t.Cleanup(func() { proxyListener.Close() })
	proxy.runGRPCServer(t, proxyListener)

	node := newFakeNode(t)

	t.Run("invalid token", func(t *testing.T) {
		_, err := node.join(
			t.Context(),
			proxyListener.DialContext,
			insecure.NewCredentials(),
			"invalidtoken",
		)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		evt := <-authService.events
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
					RemoteAddr: "bufconn",
				},
				NodeName: "node",
				Role:     "Instance",
			},
			evt,
			protocmp.Transform(),
			cmpopts.IgnoreMapEntries(func(key string, val any) bool { return key == "Time" }),
		))
	})

	t.Run("join and rejoin", func(t *testing.T) {
		// Node joins by connecting to the proxy's gRPC service.
		joinResult, err := node.join(
			t.Context(),
			proxyListener.DialContext,
			insecure.NewCredentials(),
			token1.GetName(),
		)
		require.NoError(t, err)
		cert, err := x509.ParseCertificate(joinResult.TLSCert)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		require.NoError(t, err)
		require.ElementsMatch(t, token1.GetRoles().StringSlice(), identity.SystemRoles)

		// Node can rejoin with a different token by dialing the auth service
		// with its original credentials (for this test we omit the details of
		// the proxy's mTLS tunnel dialing and let the node dial auth
		// directly).
		//
		// It should get back its original host ID and the combined roles of
		// its original certificate and the new token.
		creds, err := clientCreds(node.hostKeys.tls, joinResult)
		require.NoError(t, err)
		rejoinResult, err := node.join(
			t.Context(),
			authListener.DialContext,
			creds,
			token2.GetName(),
		)
		require.NoError(t, err)
		cert, err = x509.ParseCertificate(rejoinResult.TLSCert)
		require.NoError(t, err)
		identity, err = tlsca.FromSubject(cert.Subject, cert.NotAfter)
		require.NoError(t, err)
		require.ElementsMatch(t,
			apiutils.Deduplicate(slices.Concat(
				token1.GetRoles().StringSlice(),
				token2.GetRoles().StringSlice(),
			)),
			identity.SystemRoles)

		// The node gets back its original host ID when rejoining with an
		// authenticated client.
		require.Equal(t, joinResult.HostID, rejoinResult.HostID)
	})

	t.Run("join and rejoin with bad token", func(t *testing.T) {
		// Node joins by connecting to the proxy's gRPC service.
		joinResult, err := node.join(
			t.Context(),
			proxyListener.DialContext,
			insecure.NewCredentials(),
			token1.GetName(),
		)
		require.NoError(t, err)

		// Node the tries to rejoin with valid certs but an invalid token.
		creds, err := clientCreds(node.hostKeys.tls, joinResult)
		require.NoError(t, err)
		_, err = node.join(
			t.Context(),
			authListener.DialContext,
			creds,
			"invalidtoken",
		)
		require.ErrorAs(t, err, new(*trace.AccessDeniedError))
		evt := <-authService.events
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
					RemoteAddr: "bufconn",
				},
				NodeName: "node",
				Role:     "Instance",
			},
			evt,
			protocmp.Transform(),
			cmpopts.IgnoreMapEntries(func(key string, val any) bool { return key == "Time" }),
		))
	})
}

type fakeAuthService struct {
	validTokens map[string]types.ProvisionToken
	events      chan apievents.AuditEvent
	ca          *tlsca.CertAuthority
}

func newFakeAuthService(t *testing.T) *fakeAuthService {
	keyPEM, certPEM, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "auth"}, nil, time.Minute)
	require.NoError(t, err)
	ca, err := tlsca.FromKeys(certPEM, keyPEM)
	require.NoError(t, err)
	return &fakeAuthService{
		validTokens: make(map[string]types.ProvisionToken),
		events:      make(chan apievents.AuditEvent, 5),
		ca:          ca,
	}
}

func (s *fakeAuthService) runGRPCServer(t *testing.T, l net.Listener) {
	authGRPCServer := grpc.NewServer(
		grpc.Creds(s.serverCreds(t)),
		grpc.ChainStreamInterceptor(
			interceptors.GRPCServerStreamErrorInterceptor,
			authInterceptor,
		),
	)
	joinv1.RegisterJoinServiceServer(authGRPCServer, server.NewServer(&server.Config{
		AuthService: s,
		Authorizer:  &fakeAuthorizer{},
	}))
	testutils.RunTestBackgroundTask(t.Context(), t, &testutils.TestBackgroundTask{
		Name: "auth gRPC server",
		Task: func(ctx context.Context) error {
			return trace.Wrap(authGRPCServer.Serve(l))
		},
		Terminate: func() error {
			authGRPCServer.Stop()
			return nil
		},
	})
}

func (s *fakeAuthService) serverCreds(t *testing.T) credentials.TransportCredentials {
	authTLSKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	authIdentity := tlsca.Identity{
		Username:        utils.HostFQDN("auth", "testcluster"),
		Groups:          []string{types.RoleAuth.String()},
		TeleportCluster: "testcluster",
	}
	subject, err := authIdentity.Subject()
	require.NoError(t, err)
	authTLSCertPEM, err := s.ca.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: authTLSKey.Public(),
		Subject:   subject,
		NotAfter:  time.Now().Add(time.Minute),
		DNSNames:  []string{"auth"},
	})
	require.NoError(t, err)
	authTLSCertPEMBlock, _ := pem.Decode(authTLSCertPEM)
	authTLSCert := authTLSCertPEMBlock.Bytes

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(s.ca.Cert)
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{authTLSCert},
			PrivateKey:  authTLSKey,
		}},
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  clientCAs,
	})
}

func (s *fakeAuthService) unauthenticatedClientCreds() credentials.TransportCredentials {
	caPool := x509.NewCertPool()
	caPool.AddCert(s.ca.Cert)
	return credentials.NewTLS(&tls.Config{
		RootCAs: caPool,
	})
}

func (s *fakeAuthService) ValidateToken(ctx context.Context, tokenName string) (types.ProvisionToken, error) {
	token, ok := s.validTokens[tokenName]
	if !ok {
		return nil, trace.AccessDenied("token expired or not found")
	}
	return token, nil
}

func (s *fakeAuthService) GenerateCertsForJoin(ctx context.Context, provisionToken types.ProvisionToken, req *server.GenerateCertsForJoinRequest) (*proto.Certs, error) {
	identity := tlsca.Identity{
		Username:        utils.HostFQDN(req.HostID, "testcluster"),
		Groups:          []string{req.Role.String()},
		TeleportCluster: "testcluster",
	}
	if req.Role == types.RoleInstance {
		identity.SystemRoles = apiutils.Deduplicate(slices.Concat(
			provisionToken.GetRoles().StringSlice(),
			req.AuthenticatedSystemRoles.StringSlice(),
		))
	}
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPub, err := keys.ParsePublicKey(req.PublicTLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := s.ca.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: tlsPub,
		Subject:   subject,
		NotAfter:  time.Now().Add(time.Minute),
		DNSNames:  req.AdditionalPrincipals,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshCASigner, err := ssh.NewSignerFromSigner(s.ca.Signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshKeyGen := keygen.New(ctx)
	sshCert, err := sshKeyGen.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      sshCASigner,
		PublicHostKey: req.PublicSSHKey,
		HostID:        req.HostID,
		NodeName:      req.NodeName,
		Identity: sshca.Identity{
			ClusterName: "testcluster",
			SystemRole:  req.Role,
			Principals:  req.AdditionalPrincipals,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.Certs{
		TLS: tlsCert,
		TLSCACerts: [][]byte{
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.ca.Cert.Raw}),
		},
		SSH: sshCert,
		SSHCACerts: [][]byte{
			ssh.MarshalAuthorizedKey(sshCASigner.PublicKey()),
		},
	}, nil
}

func (s *fakeAuthService) EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	select {
	case s.events <- e:
		return nil
	default:
		return trace.Errorf("fakeAuthService events channel full")
	}
}

type fakeProxy struct {
	authDialer               func(context.Context) (net.Conn, error)
	unauthenticatedAuthCreds credentials.TransportCredentials
	authenticatedAuthCreds   credentials.TransportCredentials
}

func newFakeProxy(
	authDialer func(context.Context) (net.Conn, error),
	unauthenticatedAuthCreds credentials.TransportCredentials,
) *fakeProxy {
	return &fakeProxy{
		authDialer:               authDialer,
		unauthenticatedAuthCreds: unauthenticatedAuthCreds,
	}
}

func (p *fakeProxy) join(t *testing.T) {
	authConn, err := grpc.NewClient("passthrough:auth",
		grpc.WithContextDialer(func(ctx context.Context, name string) (net.Conn, error) {
			return p.authDialer(ctx)
		}),
		grpc.WithTransportCredentials(p.unauthenticatedAuthCreds),
	)
	require.NoError(t, err)
	defer authConn.Close()
	joinClient := joinv1.NewClient(authConn)

	stream, err := joinClient.Join(t.Context())
	require.NoError(t, err)

	hostKeys := genHostKeys(t)
	require.NoError(t, stream.Send(&messages.ClientInit{
		TokenName:    "token1",
		NodeName:     "proxy",
		Role:         types.RoleProxy.String(),
		PublicTLSKey: hostKeys.tlsPubKey,
		PublicSSHKey: hostKeys.sshPubKey,
	}))
	resp, err := stream.Recv()
	require.NoError(t, err)

	require.IsType(t, (*messages.Result)(nil), resp)
	result := resp.(*messages.Result)

	p.authenticatedAuthCreds, err = clientCreds(hostKeys.tls, result)
	require.NoError(t, err)
}

func (p *fakeProxy) runGRPCServer(t *testing.T, l net.Listener) {
	authConn, err := grpc.NewClient("passthrough:auth",
		grpc.WithTransportCredentials(p.authenticatedAuthCreds),
		grpc.WithContextDialer(func(ctx context.Context, name string) (net.Conn, error) {
			return p.authDialer(ctx)
		}),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authConn.Close())
	})
	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)
	joinv1.RegisterProxyForwardingJoinServiceServer(grpcServer, authConn)

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

type fakeNode struct {
	hostKeys *hostKeys
}

func newFakeNode(t *testing.T) *fakeNode {
	return &fakeNode{
		hostKeys: genHostKeys(t),
	}
}

func (n *fakeNode) join(
	ctx context.Context,
	dialer func(context.Context) (net.Conn, error),
	creds credentials.TransportCredentials,
	token string,
) (*messages.Result, error) {
	conn, err := grpc.NewClient("passthrough:auth",
		grpc.WithTransportCredentials(creds),
		grpc.WithContextDialer(func(ctx context.Context, name string) (net.Conn, error) {
			return dialer(ctx)
		}),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()
	joinClient := joinv1.NewClient(conn)

	stream, err := joinClient.Join(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = stream.Send(&messages.ClientInit{
		TokenName:    token,
		NodeName:     "node",
		Role:         types.RoleInstance.String(),
		PublicTLSKey: n.hostKeys.tlsPubKey,
		PublicSSHKey: n.hostKeys.sshPubKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, ok := resp.(*messages.Result)
	if !ok {
		return nil, trace.Errorf("expected *messages.Result, got %T", resp)
	}
	return result, nil
}

func clientCreds(tlsKey crypto.PrivateKey, result *messages.Result) (credentials.TransportCredentials, error) {
	caPool := x509.NewCertPool()
	for _, caCertDER := range result.TLSCACerts {
		caCert, err := x509.ParseCertificate(caCertDER)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		caPool.AddCert(caCert)
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{result.TLSCert},
			PrivateKey:  tlsKey,
		}},
		RootCAs: caPool,
	}), nil
}

type hostKeys struct {
	tls       crypto.Signer
	tlsPubKey []byte
	ssh       ssh.Signer
	sshPubKey []byte
}

func genHostKeys(t *testing.T) *hostKeys {
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	tlsPubKey, err := x509.MarshalPKIXPublicKey(signer.Public())
	require.NoError(t, err)
	sshKey, err := ssh.NewSignerFromSigner(signer)
	require.NoError(t, err)
	sshPubKey := sshKey.PublicKey().Marshal()
	return &hostKeys{
		tls:       signer,
		tlsPubKey: tlsPubKey,
		ssh:       sshKey,
		sshPubKey: sshPubKey,
	}
}

func authInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	peerInfo, ok := peer.FromContext(ss.Context())
	if !ok {
		return trace.BadParameter("could not get gRPC peer info")
	}
	if tlsInfo, ok := peerInfo.AuthInfo.(credentials.TLSInfo); ok {
		if len(tlsInfo.State.PeerCertificates) > 0 {
			cert := tlsInfo.State.PeerCertificates[0]
			id, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}
			additionalSystemRoles, err := types.NewTeleportRoles(id.SystemRoles)
			if err != nil {
				return trace.Wrap(err)
			}
			role := authz.BuiltinRole{
				Role:                  types.SystemRole(id.Groups[0]),
				AdditionalSystemRoles: additionalSystemRoles,
				Username:              id.Username,
				ClusterName:           "testcluster",
				Identity:              *id,
			}
			ctx := authz.ContextWithUser(ss.Context(), role)
			return handler(srv, newWrappedStream(ctx, ss))
		}
	}
	return handler(srv, ss)
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func newWrappedStream(ctx context.Context, ss grpc.ServerStream) *wrappedStream {
	return &wrappedStream{
		ServerStream: ss,
		ctx:          ctx,
	}
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

type fakeAuthorizer struct{}

func (a *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	identityGetter, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.AccessDenied("access denied")
	}
	identity := identityGetter.GetIdentity()
	return &authz.Context{
		Identity: identityGetter,
		Checker: &fakeChecker{
			roles: slices.Concat(identity.Groups, identity.SystemRoles),
		},
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
	roles []string
}

func (c *fakeChecker) HasRole(role string) bool {
	return slices.Contains(c.roles, role)
}
