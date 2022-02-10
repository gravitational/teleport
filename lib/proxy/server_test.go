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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// newSelfSignedCA creates a new CA for testing.
func newSelfSignedCA() (*tlsca.CertAuthority, error) {
	rsaKey, err := ssh.ParseRawPrivateKey(fixtures.PEMBytes["rsa"])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tlsca.GenerateSelfSignedCAWithSigner(
		rsaKey.(*rsa.PrivateKey), pkix.Name{}, nil, defaults.CATTL,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := tlsca.FromCertAndSigner(cert, rsaKey.(*rsa.PrivateKey))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, nil
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

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}

	return config
}

type mockAccessCache struct {
	auth.AccessCache
}

// TestServerTLS ensures that only trusted certificates with the proxy role
// are accepted by the server.
func TestServerTLS(t *testing.T) {
	ca1, err := newSelfSignedCA()
	require.NoError(t, err)

	ca2, err := newSelfSignedCA()
	require.NoError(t, err)

	tests := []struct {
		desc      string
		server    *tls.Config
		client    *tls.Config
		expectErr bool
	}{
		{
			desc: "trusted certificates with proxy roles",
			server: certFromIdentity(t, ca1, tlsca.Identity{
				Groups: []string{string(types.RoleProxy)},
			}),
			client: certFromIdentity(t, ca1, tlsca.Identity{
				Groups: []string{string(types.RoleProxy)},
			}),
			expectErr: false,
		},
		{
			desc: "trusted certificates with incorrect server role",
			server: certFromIdentity(t, ca1, tlsca.Identity{
				Groups: []string{string(types.RoleAdmin)},
			}),
			client: certFromIdentity(t, ca1, tlsca.Identity{
				Groups: []string{string(types.RoleProxy)},
			}),
			expectErr: true,
		},
		{
			desc: "certificates with correct role from different CAs",
			server: certFromIdentity(t, ca1, tlsca.Identity{
				Groups: []string{string(types.RoleProxy)},
			}),
			client: certFromIdentity(t, ca2, tlsca.Identity{
				Groups: []string{string(types.RoleProxy)},
			}),
			expectErr: true,
		},
	}

	for _, tc := range tests {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)

		server, err := NewServer(ServerConfig{
			AccessCache:   &mockAccessCache{},
			Listener:      listener,
			TLSConfig:     tc.server,
			ClusterDialer: &mockClusterDialer{},
			getConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
				config := tc.server.Clone()
				config.ClientAuth = tls.RequireAndVerifyClientCert
				config.ClientCAs = config.RootCAs
				return config, nil
			},
		})
		require.NoError(t, err)
		go server.Serve()
		t.Cleanup(func() { server.Close() })

		creds := newProxyCredentials(credentials.NewTLS(tc.client))
		conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(creds))
		require.NoError(t, err)

		func() {
			defer conn.Close()

			client := proto.NewProxyServiceClient(conn)
			_, err = client.DialNode(context.TODO())
			if tc.expectErr {
				require.Error(t, err, tc.desc)
			} else {
				require.NoError(t, err, tc.desc)
			}
		}()
	}
}
