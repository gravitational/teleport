/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
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
			Clock:              clock,
			TargetName:         kubeClusterName,
			TargetURI:          uri.NewClusterURI(teleportClusterName).AppendKube(kubeClusterName).String(),
			TargetUser:         identity.Username,
			CertPath:           proxy.clientCertPath(),
			KeyPath:            proxy.clientKeyPath(),
			WebProxyAddr:       proxy.webProxyAddr,
			ClusterName:        teleportClusterName,
			CLICommandProvider: mockCLICommandProvider{},
			RootClusterCACertPoolFunc: func(_ context.Context) (*x509.CertPool, error) {
				return proxy.certPool(), nil
			},
			OnExpiredCert: func(_ context.Context, gateway *Gateway) error {
				return trace.Wrap(gateway.ReloadCert())
			},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		gateway.Close()
	})
	go gateway.Serve()

	// First request should succeed.
	kubeClient := kubeClientForLocalProxy(t, gateway.KubeconfigPath(), teleportClusterName, kubeClusterName)
	sendRequestToKubeLocalProxyAndSucceed(t, kubeClient)

	// Let proxy "rotate" client cert. Request should fail as the gateway is
	// still using the old cert.
	proxy.mustIssueClientCert(t, identity)
	sendRequestToKubeLocalProxyAndFail(t, kubeClient)

	// Expire the cert so reissue flow is triggered:
	// kubeMiddleware -> kubeCertReissuer.reissueCert -> gateway.cfg.OnExpiredCert -> gateway.ReloadCert -> kubeCertReissuer.updateCert
	clock.Advance(time.Hour)
	sendRequestToKubeLocalProxyAndSucceed(t, kubeClient)
}

func sendRequestToKubeLocalProxyAndSucceed(t *testing.T, client *kubernetes.Clientset) {
	t.Helper()
	resp, err := client.CoreV1().Pods("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, len(resp.Items), 1)
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

	tlsClientConfig := rest.TLSClientConfig{
		CAData:     config.Clusters[contextName].CertificateAuthorityData,
		CertData:   config.AuthInfos[contextName].ClientCertificateData,
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
	dir          string
}

func (m *mockProxyWithKubeAPI) clientCertPath() string {
	return path.Join(m.dir, "cert.pem")
}
func (m *mockProxyWithKubeAPI) clientKeyPath() string {
	return path.Join(m.dir, "key.pem")
}

func (m *mockProxyWithKubeAPI) mustIssueClientCert(t *testing.T, identity tlsca.Identity) {
	t.Helper()
	gatewaytest.MustGenCertSignedWithCAAndSaveToPaths(t, m.ca, identity, m.clientCertPath(), m.clientKeyPath())
}

func (m *mockProxyWithKubeAPI) verifyConnection(state tls.ConnectionState) error {
	if len(state.PeerCertificates) != 1 {
		return trace.BadParameter("expecting one client cert")
	}
	wantCert, err := utils.ReadCertificatesFromPath(m.clientCertPath())
	if err != nil {
		return trace.Wrap(err)
	}
	if !bytes.Equal(state.PeerCertificates[0].Raw, wantCert[0].Raw) {
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
		dir:          t.TempDir(),
	}
	m.mustIssueClientCert(t, identity)

	tlsListener := tls.NewListener(netListener, &tls.Config{
		Certificates:     []tls.Certificate{serverTLSCert},
		VerifyConnection: m.verifyConnection,
		ClientAuth:       tls.RequireAndVerifyClientCert,
		ClientCAs:        m.certPool(),
	})
	go http.Serve(tlsListener, mockKubeAPIHandler())
	return m
}

func mustGenCAForProxyKubeAddr(t *testing.T, key *keys.PrivateKey, host string) (tls.Certificate, *tlsca.CertAuthority) {
	t.Helper()

	certPem, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Entity: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Teleport"},
		},
		Signer:   key,
		DNSNames: []string{client.GetKubeTLSServerName(host)}, // Use special kube SNI.
		TTL:      defaults.CATTL,
	})
	require.NoError(t, err)
	tlsCert, err := keys.X509KeyPair(certPem, key.PrivateKeyPEM())
	require.NoError(t, err)
	ca, err := tlsca.FromTLSCertificate(tlsCert)
	require.NoError(t, err)
	return tlsCert, ca
}

func mockKubeAPIHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/namespaces/default/pods", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(&v1.PodList{
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
	})
	return mux
}
