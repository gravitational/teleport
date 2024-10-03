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

package gateway

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gatewaytest"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestKubeGateway(t *testing.T) {
	t.Parallel()

	const (
		teleportClusterName = "example.com"
		kubeClusterName     = "example-kube-cluster"
	)

	identity := tlsca.Identity{
		Username:          "alice",
		Groups:            []string{"test-group"},
		KubernetesCluster: kubeClusterName,
	}
	clock := clockwork.NewFakeClock()
	proxy := mustStartMockProxyWithKubeAPI(t, identity)
	gateway, err := New(
		Config{
			Clock:          clock,
			TargetName:     kubeClusterName,
			TargetURI:      uri.NewClusterURI(teleportClusterName).AppendKube(kubeClusterName),
			Cert:           proxy.clientCert,
			WebProxyAddr:   proxy.webProxyAddr,
			ClusterName:    teleportClusterName,
			Username:       identity.Username,
			KubeconfigsDir: t.TempDir(),
			RootClusterCACertPoolFunc: func(_ context.Context) (*x509.CertPool, error) {
				return proxy.certPool(), nil
			},
			OnExpiredCert: func(ctx context.Context, g Gateway) (tls.Certificate, error) {
				// We first "rotate" the cert with proxy.mustRotateClientCert which makes the cert used by
				// the gateway invalid. Then the gateway executes this function which makes it use the new
				// cert held by the proxy.
				return proxy.clientCert, nil
			},
		},
	)
	require.NoError(t, err)

	kubeGateway, err := AsKube(gateway)
	require.NoError(t, err)

	serveErr := make(chan error)
	go func() {
		err := kubeGateway.Serve()
		serveErr <- err
	}()

	// First request should succeed.
	kubeClient := kubeClientForLocalProxy(t, kubeGateway.KubeconfigPath(), teleportClusterName, kubeClusterName)
	sendRequestToKubeLocalProxyAndSucceed(t, kubeClient)

	// Let proxy "rotate" client cert. Request should fail as the gateway is
	// still using the old cert.
	proxy.mustRotateClientCert(t, identity)
	sendRequestToKubeLocalProxyAndFail(t, kubeClient)

	// Expire the cert so reissue flow is triggered:
	// kubeMiddleware -> kubeCertReissuer.reissueCert -> gateway.cfg.OnExpiredCert -> gateway.ReloadCert -> kubeCertReissuer.updateCert
	clock.Advance(time.Hour)
	sendRequestToKubeLocalProxyAndSucceed(t, kubeClient)
	require.True(t, utils.FileExists(kubeGateway.KubeconfigPath()))

	require.NoError(t, kubeGateway.Close())
	require.NoError(t, <-serveErr)
	require.False(t, utils.FileExists(kubeGateway.KubeconfigPath()))
}

func sendRequestToKubeLocalProxyAndSucceed(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()
	resp, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	require.Equal(t, "kube-pod-name", resp.Items[0].GetName())
}
func sendRequestToKubeLocalProxyAndFail(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()
	_, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.Error(t, err)
}
func kubeClientForLocalProxy(t *testing.T, kubeconfigPath, teleportCluster, kubeCluster string) *kubernetes.Clientset {
	t.Helper()

	config, err := kubeconfig.Load(kubeconfigPath)
	require.NoError(t, err)

	contextName := kubeconfig.ContextName(teleportCluster, kubeCluster)
	proxyURL, err := url.Parse(config.Clusters[contextName].ProxyURL)
	require.NoError(t, err)

	// Sanity check the CA and client cert both use ECDSA keys.
	kubeCAPEM := config.Clusters[contextName].CertificateAuthorityData
	kubeCA, err := tlsca.ParseCertificatePEM(kubeCAPEM)
	require.NoError(t, err)
	require.IsType(t, (*ecdsa.PublicKey)(nil), kubeCA.PublicKey)
	clientCertPEM := config.AuthInfos[contextName].ClientCertificateData
	clientCert, err := tlsca.ParseCertificatePEM(clientCertPEM)
	require.NoError(t, err)
	require.IsType(t, (*ecdsa.PublicKey)(nil), clientCert.PublicKey)

	tlsClientConfig := rest.TLSClientConfig{
		CAData:     kubeCAPEM,
		CertData:   clientCertPEM,
		KeyData:    config.AuthInfos[contextName].ClientKeyData,
		ServerName: common.KubeLocalProxySNI(teleportCluster, kubeCluster),
	}
	client, err := kubernetes.NewForConfig(&rest.Config{
		Host:            "https://" + teleportCluster,
		TLSClientConfig: tlsClientConfig,
		Proxy:           http.ProxyURL(proxyURL),
	})
	require.NoError(t, err)
	return client
}

type mockProxyWithKubeAPI struct {
	webProxyAddr string
	key          *keys.PrivateKey
	ca           *tlsca.CertAuthority
	// clientCert is used to verify the cert of the incoming connection. The cert sent by the gateway
	// must be equal to clientCert for the verification to pass.
	clientCert tls.Certificate
}

func (m *mockProxyWithKubeAPI) mustRotateClientCert(t *testing.T, identity tlsca.Identity) tls.Certificate {
	t.Helper()
	cert := gatewaytest.MustGenCertSignedWithCA(t, m.ca, identity)
	m.clientCert = cert
	return cert
}

func (m *mockProxyWithKubeAPI) verifyConnection(state tls.ConnectionState) error {
	if len(state.PeerCertificates) != 1 {
		return trace.BadParameter("expecting one client cert")
	}
	wantCert, err := utils.TLSCertLeaf(m.clientCert)
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Equal(state.PeerCertificates[0].Raw, wantCert.Raw) {
		return trace.AccessDenied("client cert is invalid")
	}
	return nil
}

func (m *mockProxyWithKubeAPI) certPool() *x509.CertPool {
	certPool := x509.NewCertPool()
	certPool.AddCert(m.ca.Cert)
	return certPool
}

func mustStartMockProxyWithKubeAPI(t *testing.T, identity tlsca.Identity) *mockProxyWithKubeAPI {
	t.Helper()

	netListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		netListener.Close()
	})

	key, err := keys.ParsePrivateKey(fixtures.LocalhostKey)
	require.NoError(t, err)
	serverTLSCert, serverCA := mustGenCAForProxyKubeAddr(t, key, netListener.Addr().String())

	m := &mockProxyWithKubeAPI{
		webProxyAddr: netListener.Addr().String(),
		key:          key,
		ca:           serverCA,
	}
	m.mustRotateClientCert(t, identity)

	tlsListener := tls.NewListener(netListener, &tls.Config{
		Certificates:     []tls.Certificate{serverTLSCert},
		VerifyConnection: m.verifyConnection,
		ClientAuth:       tls.RequireAndVerifyClientCert,
		ClientCAs:        m.certPool(),
	})
	go func() {
		err := http.Serve(tlsListener, mockKubeAPIHandler(t))
		if err != nil && !errors.Is(err, net.ErrClosed) {
			assert.NoError(t, err)
		}
	}()
	return m
}

func mustGenCAForProxyKubeAddr(t *testing.T, key *keys.PrivateKey, hostAddr string) (tls.Certificate, *tlsca.CertAuthority) {
	t.Helper()

	addr, err := utils.ParseAddr(hostAddr)
	require.NoError(t, err)

	certPem, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Entity: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Teleport"},
		},
		Signer: key,
		// Use special kube SNI. Make sure only host (no port) is used.
		DNSNames: []string{client.GetKubeTLSServerName(addr.Host())},
		TTL:      defaults.CATTL,
	})
	require.NoError(t, err)
	tlsCert, err := keys.X509KeyPair(certPem, key.PrivateKeyPEM())
	require.NoError(t, err)
	ca, err := tlsca.FromTLSCertificate(tlsCert)
	require.NoError(t, err)
	return tlsCert, ca
}

func mockKubeAPIHandler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(&v1.PodList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PodList",
				APIVersion: "v1",
			},
			Items: []v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-pod-name",
						Namespace: "default",
					},
				},
			},
		})
		assert.NoError(t, err)
	})
	return mux
}
