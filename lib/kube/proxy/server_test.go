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

package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestServeConfigureError(t *testing.T) {
	srv := &TLSServer{Server: &http.Server{TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, CipherSuites: []uint16{}}}, closeContext: context.Background()}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	err = srv.Serve(listener)
	require.Error(t, err) // expected due to incompatible ciphers

	require.True(t, srv.mu.TryLock()) // verify that lock was released despite error
}

func TestMTLSClientCAs(t *testing.T) {
	ap := &mockAccessPoint{
		cas: make(map[string]types.CertAuthority),
	}
	// Reuse the same CA private key for performance.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	addCA := func(t *testing.T, name string) (key, cert []byte) {
		cert, err := tlsca.GenerateSelfSignedCAWithSigner(caKey, pkix.Name{CommonName: name}, nil, time.Minute)
		require.NoError(t, err)
		key, err = keys.MarshalPrivateKey(caKey)
		require.NoError(t, err)
		ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        types.HostCA,
			ClusterName: name,
			ActiveKeys: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: cert,
					Key:  key,
				}},
			},
		})
		require.NoError(t, err)
		ap.cas[name] = ca
		return key, cert
	}

	const mainClusterName = "cluster-main"
	key, cert := addCA(t, mainClusterName)
	ca, err := tlsca.FromKeys(cert, key)
	require.NoError(t, err)

	// Generate user and host credentials, using the same private key.
	userHostKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	genCert := func(t *testing.T, cn string, sans ...string) tls.Certificate {
		certRaw, err := ca.GenerateCertificate(tlsca.CertificateRequest{
			PublicKey: userHostKey.Public(),
			Subject:   pkix.Name{CommonName: cn},
			NotAfter:  time.Now().Add(time.Minute),
			DNSNames:  sans,
		})
		require.NoError(t, err)
		keyPEM, err := keys.MarshalPrivateKey(userHostKey)
		require.NoError(t, err)
		cert, err := tls.X509KeyPair(certRaw, keyPEM)
		require.NoError(t, err)
		return cert
	}
	hostCert := genCert(t, "localhost", "localhost", "127.0.0.1", "::1")
	userCert := genCert(t, "user")
	srv := &TLSServer{
		TLSServerConfig: TLSServerConfig{
			Log: utils.NewSlogLoggerForTests(),
			ForwarderConfig: ForwarderConfig{
				ClusterName: mainClusterName,
			},
			AccessPoint: ap,
			TLS: &tls.Config{
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{hostCert},
			},
			GetRotation:          func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
			ConnectedProxyGetter: reversetunnel.NewConnectedProxyGetter(),
		},
		log: utils.NewSlogLoggerForTests(),
	}

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()
	lis = tls.NewListener(lis, &tls.Config{GetConfigForClient: srv.GetConfigForClient})

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		for {
			con, err := lis.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					errCh <- err
				}
				return
			}
			if err := con.(*tls.Conn).Handshake(); err != nil {
				errCh <- err
				return
			}
			if err := con.Close(); err != nil {
				errCh <- err
				return
			}
		}
	}()
	t.Cleanup(func() {
		require.NoError(t, <-errCh)
	})

	// CA pool for the client to validate the server against.
	userCAPool := x509.NewCertPool()
	userCAPool.AddCert(ca.Cert)

	testDial := func(t *testing.T, wantCAs int) {
		con, err := tls.Dial("tcp", lis.Addr().String(), &tls.Config{
			RootCAs: userCAPool,
			GetClientCertificate: func(req *tls.CertificateRequestInfo) (*tls.Certificate, error) {
				require.Len(t, req.AcceptableCAs, wantCAs)
				return &userCert, nil
			},
		})
		require.NoError(t, err)
		require.NoError(t, con.Handshake())
		require.NoError(t, con.Close())
	}

	// Only the original main CA registered.
	t.Run("1 CA", func(t *testing.T) {
		testDial(t, 1)
	})
	// 100 additional CAs registered, all CAs should be sent to the client in
	// the handshake.
	t.Run("100 CAs", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			addCA(t, fmt.Sprintf("cluster-%d", i))
		}
		testDial(t, 101)
	})
	// 1000 total CAs registered, all CAs no longer fit in the handshake.
	// Server truncates the CA list to just the main CA.
	t.Run("1000 CAs", func(t *testing.T) {
		for i := 100; i < 1000; i++ {
			addCA(t, fmt.Sprintf("cluster-%d", i))
		}
		testDial(t, 1)
	})
}

func TestGetServerInfo(t *testing.T) {
	ap := &mockAccessPoint{
		cas: make(map[string]types.CertAuthority),
	}

	listener, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)

	srv := &TLSServer{
		TLSServerConfig: TLSServerConfig{
			Log: utils.NewSlogLoggerForTests(),
			ForwarderConfig: ForwarderConfig{
				Clock:       clockwork.NewFakeClock(),
				ClusterName: "kube-cluster",
				HostID:      "server_uuid",
			},
			AccessPoint:          ap,
			TLS:                  &tls.Config{},
			ConnectedProxyGetter: reversetunnel.NewConnectedProxyGetter(),
			GetRotation:          func(role types.SystemRole) (*types.Rotation, error) { return &types.Rotation{}, nil },
		},
		fwd: &Forwarder{
			cfg: ForwarderConfig{},
			clusterDetails: map[string]*kubeDetails{
				"kube-cluster": {
					kubeCluster: mustCreateKubernetesClusterV3(t, "kube-cluster"),
				},
			},
		},
		listener: listener,
	}

	t.Run("GetServerInfo gets listener addr with PublicAddr unset", func(t *testing.T) {
		kubeServer, err := srv.GetServerInfo("kube-cluster")
		require.NoError(t, err)
		require.Equal(t, listener.Addr().String(), kubeServer.GetHostname())
	})

	t.Run("GetServerInfo gets correct public addr with PublicAddr set", func(t *testing.T) {
		srv.TLSServerConfig.ForwarderConfig.PublicAddr = "k8s.example.com"

		kubeServer, err := srv.GetServerInfo("kube-cluster")
		require.NoError(t, err)
		require.Equal(t, "k8s.example.com", kubeServer.GetHostname())
	})
}

func TestHeartbeat(t *testing.T) {
	kubeCluster1 := "kubeCluster1"
	kubeCluster2 := "kubeCluster2"

	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster1, APIEndpoint: kubeMock.URL}, {Name: kubeCluster2, APIEndpoint: kubeMock.URL}},
		},
	)

	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	type args struct {
		kubeClusterGetter func(authclient.ClientI) []string
	}
	tests := []struct {
		name      string
		args      args
		wantEmpty bool
	}{
		{
			name: "List KubeServers",
			args: args{
				kubeClusterGetter: func(authClient authclient.ClientI) []string {
					rsp, err := authClient.ListResources(testCtx.Context, proto.ListResourcesRequest{
						ResourceType: types.KindKubeServer,
						Limit:        10,
					})
					require.NoError(t, err)
					clusters := []string{}
					for _, resource := range rsp.Resources {
						srv, ok := resource.(types.KubeServer)
						require.Truef(t, ok, "type is %T; expected types.KubeServer", srv)
						clusters = append(clusters, srv.GetName())
					}
					sort.Strings(clusters)
					return clusters
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeClusters := tt.args.kubeClusterGetter(testCtx.AuthClient)
			if tt.wantEmpty {
				require.Empty(t, kubeClusters)
			} else {
				require.Equal(t, []string{kubeCluster1, kubeCluster2}, kubeClusters)
			}
		})
	}
}

func TestTLSServerConfig_validateLabelsKey(t *testing.T) {
	type fields struct {
		staticLabels map[string]string
	}
	tests := []struct {
		name           string
		fields         fields
		errorAssertion require.ErrorAssertionFunc
	}{
		{
			name: "valid labels",
			fields: fields{
				staticLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			errorAssertion: require.NoError,
		},
		{
			name: "invalid labels",
			fields: fields{
				staticLabels: map[string]string{
					"key 1": "value1",
					"key2":  "value2",
				},
			},
			errorAssertion: require.Error,
		},
		{
			name: "invalid labels",
			fields: fields{
				staticLabels: map[string]string{
					"key\\1": "value1",
					"key2":   "value2",
				},
			},
			errorAssertion: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &TLSServerConfig{
				StaticLabels: tt.fields.staticLabels,
			}
			err := c.validateLabelKeys()
			tt.errorAssertion(t, err)
		})
	}
}
