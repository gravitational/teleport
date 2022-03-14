// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"testing"
	"time"

	clientapi "github.com/gravitational/teleport/api/client/proto"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
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

type mockProxyService struct{}

func (s *mockProxyService) DialNode(stream clientapi.ProxyService_DialNodeServer) error {
	errChan := make(chan error)
	defer close(errChan)

	go func() {
		for {
			if _, err := stream.Recv(); err != nil {
				errChan <- err
				return
			}
		}
	}()

	select {
	case <-stream.Context().Done():
		<-errChan
		return stream.Context().Err()
	case err := <-errChan:
		return err
	}
	return nil
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

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
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
func setupClient(t *testing.T, clientCA, serverCA *tlsca.CertAuthority, role types.SystemRole) (*Client, *tls.Config) {
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
		ID:                 "client-proxy",
		AuthClient:         mockAuthClient{},
		AccessPoint:        &mockProxyAccessPoint{},
		TLSConfig:          tlsConf,
		Clock:              clockwork.NewFakeClock(),
		getConfigForServer: getConfigForServer,
		sync:               func() {},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		client.Close()
	})

	return client, tlsConf
}

// setupServer return a Server object.
func setupServer(t *testing.T, name string, serverCA, clientCA *tlsca.CertAuthority, role types.SystemRole) (*Server, *tls.Config, types.Server) {
	tlsConf := certFromIdentity(t, serverCA, tlsca.Identity{
		Groups: []string{string(role)},
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

	server, err := NewServer(ServerConfig{
		AccessCache:        &mockAccessCache{},
		Listener:           listener,
		TLSConfig:          tlsConf,
		ClusterDialer:      &mockClusterDialer{},
		getConfigForClient: getConfigForClient,
		getService:         func() clientapi.ProxyServiceServer { return &mockProxyService{} },
	})
	require.NoError(t, err)

	ts, err := types.NewServer(
		name, types.KindProxy,
		types.ServerSpecV2{PeerAddr: listener.Addr().String()},
	)
	require.NoError(t, err)

	go server.Serve()
	t.Cleanup(func() {
		server.Close()
	})

	return server, tlsConf, ts
}

func sendMsg(stream clientapi.ProxyService_DialNodeClient) error {
	return stream.Send(&clientapi.Frame{
		Message: &clientapi.Frame_Data{
			&clientapi.Data{Bytes: []byte("test")},
		},
	})
}
