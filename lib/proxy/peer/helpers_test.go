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
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	clientapi "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

type mockAuthClient struct {
	auth.ClientI
}

func (c mockAuthClient) GetProxies() ([]types.Server, error) {
	return []types.Server{}, nil
}

type mockAccessCache struct {
	auth.AccessCache
}

type mockProxyAccessPoint struct {
	auth.ProxyAccessPoint
}

type mockProxyService struct {
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

// certFromIdentity creates a tls config for a given CA and identity.
func certFromIdentity(t *testing.T, ca *tlsca.CertAuthority, ident tlsca.Identity) *tls.Config {
	if ident.Username == "" {
		ident.Username = "test-user"
	}

	subj, err := ident.Subject()
	require.NoError(t, err)

	privateKey, err := native.GenerateRSAPrivateKey()
	require.NoError(t, err)

	clock := clockwork.NewRealClock()

	request := tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  clock.Now().UTC().Add(time.Minute),
		DNSNames:  []string{"127.0.0.1"},
	}
	certBytes, err := ca.GenerateCertificate(request)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	cert, err := tls.X509KeyPair(certBytes, keyPEM)
	require.NoError(t, err)

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return config
}

// setupClients return a Client object.
func setupClient(t *testing.T, clientCA, serverCA *tlsca.CertAuthority, role types.SystemRole) *Client {
	tlsConf := certFromIdentity(t, clientCA, tlsca.Identity{
		Groups: []string{string(role)},
	})

	getConfigForServer := func() (*tls.Config, error) {
		config := tlsConf.Clone()
		rootCAs := x509.NewCertPool()
		rootCAs.AddCert(serverCA.Cert)
		config.RootCAs = rootCAs
		return config, nil
	}

	client, err := NewClient(ClientConfig{
		ID:                      "client-proxy",
		AuthClient:              mockAuthClient{},
		AccessPoint:             &mockProxyAccessPoint{},
		TLSConfig:               tlsConf,
		Clock:                   clockwork.NewFakeClock(),
		GracefulShutdownTimeout: time.Second,
		getConfigForServer:      getConfigForServer,
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
	tlsConf := certFromIdentity(t, serverCA, tlsca.Identity{
		Username: name + ".test",
		Groups:   []string{string(role)},
	})

	getConfigForClient := func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
		config := tlsConf.Clone()
		config.ClientAuth = tls.RequireAndVerifyClientCert
		clientCAs := x509.NewCertPool()
		clientCAs.AddCert(clientCA.Cert)
		config.ClientCAs = clientCAs
		return config, nil
	}

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	config := ServerConfig{
		AccessCache:        &mockAccessCache{},
		Listener:           listener,
		TLSConfig:          tlsConf,
		ClusterDialer:      &mockClusterDialer{},
		getConfigForClient: getConfigForClient,
		service:            &mockProxyService{},
		ClusterName:        "test",
	}
	for _, option := range options {
		option(&config)
	}

	server, err := NewServer(config)
	require.NoError(t, err)

	ts, err := types.NewServer(
		name, types.KindProxy,
		types.ServerSpecV2{PeerAddr: listener.Addr().String()},
	)
	require.NoError(t, err)

	go server.Serve()
	t.Cleanup(func() {
		require.NoError(t, server.Close())
	})

	return server, ts
}

func sendMsg(t *testing.T, stream frameStream) {
	err := stream.Send([]byte("ping"))
	require.NoError(t, err)
}
