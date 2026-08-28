/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package peer

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	clientapi "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

type mockAuthClient struct {
	authclient.ClientI
}

func (c mockAuthClient) GetProxies() ([]types.Server, error) {
	return []types.Server{}, nil
}

type mockProxyAccessPoint struct {
	AccessPoint
}

type mockProxyService struct {
	clientapi.UnimplementedProxyServiceServer
	mockDialNode func(stream clientapi.ProxyService_DialNodeServer) error
}

func (s *mockProxyService) DialNode(stream clientapi.ProxyService_DialNodeServer) error {
	if s.mockDialNode != nil {
		return s.mockDialNode(stream)
	}

	return s.defaultDialNode(stream)
}

func (s *mockProxyService) defaultDialNode(stream clientapi.ProxyService_DialNodeServer) error {
	sendErr := make(chan error)
	recvErr := make(chan error)

	frame, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	if frame.GetDialRequest() == nil {
		return trace.BadParameter("invalid dial request")
	}

	err = stream.Send(&clientapi.Frame{
		Message: &clientapi.Frame_ConnectionEstablished{
			ConnectionEstablished: &clientapi.ConnectionEstablished{},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		for {
			if _, err := stream.Recv(); err != nil {
				recvErr <- err
				close(recvErr)
				return
			}
		}
	}()

	go func() {
		for {
			err := stream.Send(&clientapi.Frame{
				Message: &clientapi.Frame_Data{
					Data: &clientapi.Data{Bytes: []byte("pong")},
				},
			})
			if err != nil {
				sendErr <- err
				close(sendErr)
				return
			}
		}
	}()

	select {
	case <-stream.Context().Done():
		return stream.Context().Err()
	case err := <-recvErr:
		return err
	case err := <-sendErr:
		return err
	}
}

// newSelfSignedCA creates a new CA for testing.
func newSelfSignedCA(t *testing.T) *tlsca.CertAuthority {
	rsaKey, err := ssh.ParseRawPrivateKey(fixtures.PEMBytes["rsa"])
	require.NoError(t, err)

	cert, err := tlsca.GenerateSelfSignedCAWithSigner(
		rsaKey.(*rsa.PrivateKey), pkix.Name{}, nil, defaults.CATTL,
	)
	require.NoError(t, err)

	ca, err := tlsca.FromCertAndSigner(cert, rsaKey.(*rsa.PrivateKey))
	require.NoError(t, err)

	return ca
}

func newAtomicCA(ca *tlsca.CertAuthority) *atomic.Pointer[tlsca.CertAuthority] {
	a := new(atomic.Pointer[tlsca.CertAuthority])
	a.Store(ca)
	return a
}

// certFromIdentity creates a tls config for a given CA and identity.
func certFromIdentity(t *testing.T, ca *tlsca.CertAuthority, ident tlsca.Identity) tls.Certificate {
	if ident.Username == "" {
		ident.Username = "test-user"
	}

	subj, err := ident.Subject()
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	clock := clockwork.NewRealClock()

	request := tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  clock.Now().UTC().Add(time.Minute),
		DNSNames:  []string{"127.0.0.1", apiutils.EncodeClusterName("test")},
	}
	certBytes, err := ca.GenerateCertificate(request)
	require.NoError(t, err)

	keyPEM, err := keys.MarshalPrivateKey(privateKey)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(certBytes, keyPEM)
	require.NoError(t, err)

	return cert
}

// setupClients return a Client object.
func setupClient(t *testing.T, clientCA *tlsca.CertAuthority, serverCA *atomic.Pointer[tlsca.CertAuthority], role types.SystemRole) *Client {
	tlsCert := certFromIdentity(t, clientCA, tlsca.Identity{
		Groups: []string{string(role)},
	})

	client, err := NewClient(ClientConfig{
		ID:          "client-proxy",
		AuthClient:  mockAuthClient{},
		AccessPoint: &mockProxyAccessPoint{},

		GetTLSCertificate: func() (*tls.Certificate, error) {
			return &tlsCert, nil
		},
		GetTLSRoots: func() (*x509.CertPool, error) {
			pool := x509.NewCertPool()
			ca := serverCA.Load()
			pool.AddCert(ca.Cert)
			return pool, nil
		},
		Clock:                   clockwork.NewFakeClock(),
		GracefulShutdownTimeout: time.Second,
		sync:                    func() {},
		connShuffler:            noopConnShuffler(),
		ClusterName:             "test",
	})
	require.NoError(t, err)

	t.Cleanup(func() { client.Stop() })

	return client
}

type serverTestOption func(*ServerConfig)

// setupServer return a Server object.
func setupServer(t *testing.T, name string, serverCA, clientCA *tlsca.CertAuthority, role types.SystemRole, options ...serverTestOption) (*Server, types.Server) {
	tlsCert := certFromIdentity(t, serverCA, tlsca.Identity{
		Username: name + ".test",
		Groups:   []string{string(role)},
	})
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(clientCA.Cert)

	config := ServerConfig{
		Dialer: &mockClusterDialer{},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &tlsCert, nil
		},
		GetClientCAs: func(*tls.ClientHelloInfo) (*x509.CertPool, error) {
			return clientCAs, nil
		},

		service: &mockProxyService{},
	}

	for _, option := range options {
		option(&config)
	}

	server, err := NewServer(config)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	go server.Serve(listener)
	t.Cleanup(func() {
		require.NoError(t, server.Close())
	})

	ts, err := types.NewServer(
		name, types.KindProxy,
		types.ServerSpecV2{PeerAddr: listener.Addr().String()},
	)
	require.NoError(t, err)

	return server, ts
}

func sendMsg(t *testing.T, stream net.Conn) {
	_, err := stream.Write([]byte("ping"))
	require.NoError(t, err)
}
