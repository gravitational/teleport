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

package common

import (
	"context"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/fixtures"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAuthSignKubeconfig(t *testing.T) {
	pingTestServer := httptest.NewTLSServer(&pingSrv{})
	t.Cleanup(func() { pingTestServer.Close() })

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	require.NoError(t, err)

	remoteCluster, err := types.NewRemoteCluster("leaf.example.com")
	require.NoError(t, err)

	cert := []byte(fixtures.TLSCACertPEM)
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{PublicKey: []byte("SSH CA cert")}},
			TLS: []*types.TLSKeyPair{{Cert: cert}},
		},
	})
	require.NoError(t, err)
	// newSeparatedCluster returns a mockClient that simulates a cluster with
	// a separate proxy.
	// We create a separate cluster per test because it's not safe to use it in
	// parallel.
	newSeparatedCluster := func() *mockClient {
		return &mockClient{
			clusterName:    clusterName,
			remoteClusters: []types.RemoteCluster{remoteCluster},
			userCerts: &proto.Certs{
				SSH: []byte("SSH cert"),
				TLS: cert,
				TLSCACerts: [][]byte{
					cert,
				},
			},
			cas: []types.CertAuthority{ca},
			proxies: []types.Server{
				&types.ServerV2{
					Kind:    types.KindNode,
					Version: types.V2,
					Metadata: types.Metadata{
						Name: "proxy",
					},
					Spec: types.ServerSpecV2{
						// This is the address that will be used by the client to call
						// the proxy ping endpoint. This is not the address that will be
						// used in the kubeconfig server address.
						PublicAddrs: []string{mustGetHost(t, pingTestServer.URL)},
					},
				},
			},
		}
	}
	// newMultiplexCluster returns a mockClient that simulates a cluster with
	// a multiplex proxy.
	// We create a separate cluster per test because it's not safe to use it in
	// parallel.
	newMultiplexCluster := func() *mockClient {
		return &mockClient{
			clusterName:    clusterName,
			remoteClusters: []types.RemoteCluster{remoteCluster},
			networkConfig: &types.ClusterNetworkingConfigV2{
				Spec: types.ClusterNetworkingConfigSpecV2{
					ProxyListenerMode: types.ProxyListenerMode_Multiplex,
				},
			},
			userCerts: &proto.Certs{
				SSH: []byte("SSH cert"),
				TLS: cert,
				TLSCACerts: [][]byte{
					cert,
				},
			},
			cas: []types.CertAuthority{ca},
			proxies: []types.Server{
				&types.ServerV2{
					Kind:    types.KindNode,
					Version: types.V2,
					Metadata: types.Metadata{
						Name: "proxy",
					},
					Spec: types.ServerSpecV2{
						PublicAddrs: []string{"proxy-from-api.example.com:3080"},
					},
				},
			},
		}
	}
	tests := []struct {
		desc        string
		ac          AuthCommand
		client      *mockClient
		wantAddr    string
		wantCluster string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			desc:   "valid --proxy URL with valid URL scheme",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				proxyAddr:     "https://proxy-from-flag.example.com",
			},
			wantAddr:  "https://proxy-from-flag.example.com",
			assertErr: require.NoError,
		},
		{
			desc:   "valid --proxy URL with invalid URL scheme",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				proxyAddr:     "file://proxy-from-flag.example.com",
			},
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.Equal(t, "expected --proxy URL with http or https scheme", err.Error())
			},
		},
		{
			desc:   "valid --proxy URL without URL scheme",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				proxyAddr:     "proxy-from-flag.example.com",
			},
			wantAddr:  "https://proxy-from-flag.example.com",
			assertErr: require.NoError,
		},
		{
			desc:   "invalid --proxy URL",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				proxyAddr:     "1https://proxy-from-flag.example.com",
			},
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "specified --proxy URL is invalid")
			},
		},
		{
			desc:   "k8s proxy running locally with public_addr",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &servicecfg.Config{Proxy: servicecfg.ProxyConfig{Kube: servicecfg.KubeProxyConfig{
					Enabled:     true,
					PublicAddrs: []utils.NetAddr{{Addr: "proxy-from-config.example.com:3026"}},
				}}},
			},
			wantAddr:  "https://proxy-from-config.example.com:3026",
			assertErr: require.NoError,
		},
		{
			desc:   "k8s proxy running locally without public_addr",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &servicecfg.Config{Proxy: servicecfg.ProxyConfig{
					Kube: servicecfg.KubeProxyConfig{
						Enabled: true,
					},
					PublicAddrs: []utils.NetAddr{{Addr: "proxy-from-config.example.com:3080"}},
				}},
			},
			wantAddr:  "https://proxy-from-config.example.com:3026",
			assertErr: require.NoError,
		},
		{
			desc:   "k8s proxy from cluster info",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &servicecfg.Config{Proxy: servicecfg.ProxyConfig{
					Kube: servicecfg.KubeProxyConfig{
						Enabled: false,
					},
				}},
				testInsecureSkipVerify: true,
			},
			wantAddr:  "https://proxy-from-api.example.com:3060",
			assertErr: require.NoError,
		},
		{
			desc:   "--kube-cluster specified with valid cluster",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				leafCluster:   remoteCluster.GetMetadata().Name,
				config: &servicecfg.Config{Proxy: servicecfg.ProxyConfig{
					Kube: servicecfg.KubeProxyConfig{
						Enabled: false,
					},
				}},
				testInsecureSkipVerify: true,
			},
			wantCluster: remoteCluster.GetMetadata().Name,
			assertErr:   require.NoError,
			wantAddr:    "https://proxy-from-api.example.com:3060",
		},
		{
			desc:   "--kube-cluster specified with invalid cluster",
			client: newSeparatedCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				leafCluster:   "doesnotexist.example.com",
				config: &servicecfg.Config{Proxy: servicecfg.ProxyConfig{
					Kube: servicecfg.KubeProxyConfig{
						Enabled: false,
					},
				}},
				testInsecureSkipVerify: true,
			},
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.Equal(t, `couldn't find leaf cluster named "doesnotexist.example.com"`, err.Error())
			},
		},
		{
			desc:   "k8s proxy running locally in multiplex mode without public_addr",
			client: newMultiplexCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &servicecfg.Config{
					Auth: servicecfg.AuthConfig{
						NetworkingConfig: &types.ClusterNetworkingConfigV2{
							Spec: types.ClusterNetworkingConfigSpecV2{
								ProxyListenerMode: types.ProxyListenerMode_Multiplex,
							},
						},
					},
					Proxy: servicecfg.ProxyConfig{Kube: servicecfg.KubeProxyConfig{
						Enabled: true,
					}, PublicAddrs: []utils.NetAddr{{Addr: "proxy-from-config.example.com:3080"}}},
				},
			},
			wantAddr:  "https://proxy-from-config.example.com:3080",
			assertErr: require.NoError,
		},
		{
			desc:   "k8s proxy from cluster info with multiplex mode",
			client: newMultiplexCluster(),
			ac: AuthCommand{
				output:        filepath.Join(t.TempDir(), "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &servicecfg.Config{Proxy: servicecfg.ProxyConfig{
					Kube: servicecfg.KubeProxyConfig{
						Enabled: false,
					},
				}},
				testInsecureSkipVerify: true,
			},
			wantAddr:  "https://proxy-from-api.example.com:3080",
			assertErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			// Generate kubeconfig.
			err := tt.ac.generateUserKeys(context.Background(), tt.client)
			tt.assertErr(t, err)
			// error is already asserted, so we can return early.
			if err != nil {
				return
			}
			// Validate kubeconfig contents.
			kc, err := kubeconfig.Load(tt.ac.output)
			require.NoError(t, err)
			currentCtx, ok := kc.Contexts[kc.CurrentContext]
			require.Truef(t, ok, "currentContext %q not present in kubeconfig", kc.CurrentContext)
			gotCert := kc.AuthInfos[currentCtx.AuthInfo].ClientCertificateData
			require.Equal(t, gotCert, tt.client.userCerts.TLS, "client certs not equal")
			gotCA := kc.Clusters[currentCtx.Cluster].CertificateAuthorityData
			wantCA := ca.GetActiveKeys().TLS[0].Cert
			require.Equal(t, wantCA, gotCA, "CA certs not equal")
			gotServerAddr := kc.Clusters[currentCtx.Cluster].Server
			require.Equal(t, tt.wantAddr, gotServerAddr, "server address not equal")
		})
	}
}

// mustGetHost returns the host from a full URL.
func mustGetHost(t *testing.T, fullURL string) string {
	u, err := url.Parse(fullURL)
	require.NoError(t, err)
	return u.Host
}

// pingSrv is a simple HTTP handler that returns a PingResponse with a
// kube proxy enabled.
type pingSrv struct{}

func (p *pingSrv) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	wr.WriteHeader(http.StatusOK)
	json.NewEncoder(wr).Encode(
		webclient.PingResponse{
			Proxy: webclient.ProxySettings{
				Kube: webclient.KubeProxySettings{
					Enabled:    true,
					PublicAddr: "proxy-from-api.example.com:3060",
				},
			},
		},
	)
}

type mockClient struct {
	*authclient.Client

	clusterName    types.ClusterName
	userCerts      *proto.Certs
	userCertsReq   *proto.UserCertsRequest
	dbCertsReq     *proto.DatabaseCertRequest
	dbCerts        *proto.DatabaseCertResponse
	cas            []types.CertAuthority
	caOverride     *subcav1.CertAuthorityOverride
	proxies        []types.Server
	remoteClusters []types.RemoteCluster
	kubeServers    []types.KubeServer
	appServices    []types.AppServer
	dbServices     []types.DatabaseServer
	appSession     types.WebSession
	networkConfig  types.ClusterNetworkingConfig
	crl            []byte
}

func (c *mockClient) GetClusterName(_ context.Context) (types.ClusterName, error) {
	return c.clusterName, nil
}

func (c *mockClient) Ping(ctx context.Context) (proto.PingResponse, error) {
	return proto.PingResponse{
		ServerVersion: api.Version,
	}, nil
}

func (c *mockClient) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	if c.networkConfig == nil {
		return &types.ClusterNetworkingConfigV2{}, nil
	}
	return c.networkConfig, nil
}

func (c *mockClient) GenerateUserCerts(ctx context.Context, userCertsReq proto.UserCertsRequest) (*proto.Certs, error) {
	c.userCertsReq = &userCertsReq
	return c.userCerts, nil
}

func (c *mockClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	for _, v := range c.cas {
		if v.GetType() == id.Type && v.GetClusterName() == id.DomainName {
			return v, nil
		}
	}
	return nil, trace.NotFound("%q CA not found", id)
}

func (c *mockClient) GetCertAuthorityOverride(ctx context.Context, id types.CertAuthorityOverrideID) (*subcav1.CertAuthorityOverride, error) {
	if c.caOverride == nil {
		return nil, trace.NotFound("ca override not found")
	}
	return c.caOverride, nil
}

func (c *mockClient) GetCertAuthorities(_ context.Context, caType types.CertAuthType, _ bool) ([]types.CertAuthority, error) {
	return c.cas, nil
}

func (c *mockClient) GetProxies() ([]types.Server, error) {
	return c.proxies, nil
}

func (c *mockClient) ListProxyServers(context.Context, int, string) ([]types.Server, string, error) {
	return c.proxies, "", nil
}

func (c *mockClient) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	return c.remoteClusters, nil
}

func (c *mockClient) GetKubernetesServers(context.Context) ([]types.KubeServer, error) {
	return c.kubeServers, nil
}

func (c *mockClient) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	if req.GetRequesterName() != proto.DatabaseCertRequest_TCTL {
		return nil, trace.BadParameter("need tctl requester name in tctl database cert request")
	}
	c.dbCertsReq = req
	return c.dbCerts, nil
}

func (c *mockClient) GetApplicationServers(context.Context, string) ([]types.AppServer, error) {
	return c.appServices, nil
}

func (c *mockClient) CreateAppSession(ctx context.Context, req *proto.CreateAppSessionRequest) (types.WebSession, error) {
	return c.appSession, nil
}

func (c *mockClient) GetDatabaseServers(context.Context, string, ...services.MarshalOption) ([]types.DatabaseServer, error) {
	return c.dbServices, nil
}

func (c *mockClient) GenerateCertAuthorityCRL(context.Context, types.CertAuthType) ([]byte, error) {
	return c.crl, nil
}

// TestGenerateDatabaseKeys verifies cert/key pair generation for databases.
func TestGenerateDatabaseKeys(t *testing.T) {
	clusterName, err := services.NewClusterNameWithRandomID(
		types.ClusterNameSpecV2{
			ClusterName: "example.com",
		})
	require.NoError(t, err)

	certBytes := []byte("TLS cert")
	dbClientCABytes := []byte("DB Client CA cert")
	dbServerCABytes := []byte("DB Server CA cert")
	dbCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{Cert: dbServerCABytes}},
		},
	})
	require.NoError(t, err)

	authClient := &mockClient{
		clusterName: clusterName,
		dbCerts: &proto.DatabaseCertResponse{
			Cert:    certBytes,
			CACerts: [][]byte{dbClientCABytes},
		},
		cas: []types.CertAuthority{dbCA},
	}

	keyRing, err := generateKeyRing(context.Background(), authClient, cryptosuites.DatabaseClient)
	require.NoError(t, err)

	tests := []struct {
		name           string
		inFormat       identityfile.Format
		inHost         string
		inOutDir       string
		inOutFile      string
		outSubject     pkix.Name
		outServerNames []string
		// maps filename -> file contents
		wantFiles    map[string][]byte
		genKeyErrMsg string
	}{
		{
			name:           "database certificate",
			inFormat:       identityfile.FormatDatabase,
			inHost:         "postgres.example.com",
			inOutDir:       t.TempDir(),
			inOutFile:      "db",
			outSubject:     pkix.Name{CommonName: "postgres.example.com"},
			outServerNames: []string{"postgres.example.com"},
			wantFiles: map[string][]byte{
				"db.key": keyRing.TLSPrivateKey.PrivateKeyPEM(),
				"db.crt": certBytes,
				"db.cas": dbClientCABytes,
			},
		},
		{
			name:           "database certificate multiple SANs",
			inFormat:       identityfile.FormatDatabase,
			inHost:         "mysql.external.net,mysql.internal.net,192.168.1.1",
			inOutDir:       t.TempDir(),
			inOutFile:      "db",
			outSubject:     pkix.Name{CommonName: "mysql.external.net"},
			outServerNames: []string{"mysql.external.net", "mysql.internal.net", "192.168.1.1"},
			wantFiles: map[string][]byte{
				"db.key": keyRing.TLSPrivateKey.PrivateKeyPEM(),
				"db.crt": certBytes,
				"db.cas": dbClientCABytes,
			},
		},
		{
			name:           "mongodb certificate",
			inFormat:       identityfile.FormatMongo,
			inHost:         "mongo.example.com",
			inOutDir:       t.TempDir(),
			inOutFile:      "mongo",
			outSubject:     pkix.Name{CommonName: "mongo.example.com", Organization: []string{"example.com"}},
			outServerNames: []string{"mongo.example.com"},
			wantFiles: map[string][]byte{
				"mongo.crt": append(certBytes, keyRing.TLSPrivateKey.PrivateKeyPEM()...),
				"mongo.cas": dbClientCABytes,
			},
		},
		{
			name:           "cockroachdb certificate",
			inFormat:       identityfile.FormatCockroach,
			inHost:         "localhost,roach1",
			inOutDir:       t.TempDir(),
			outSubject:     pkix.Name{CommonName: "node"},
			outServerNames: []string{"node", "localhost", "roach1"}, // "node" principal should always be added
			wantFiles: map[string][]byte{
				"node.key":      keyRing.TLSPrivateKey.PrivateKeyPEM(),
				"node.crt":      certBytes,
				"ca.crt":        dbServerCABytes,
				"ca-client.crt": dbClientCABytes,
			},
		},
		{
			name:           "redis certificate",
			inFormat:       identityfile.FormatRedis,
			inHost:         "localhost,redis1,172.0.0.1",
			inOutDir:       t.TempDir(),
			inOutFile:      "db",
			outSubject:     pkix.Name{CommonName: "localhost"},
			outServerNames: []string{"localhost", "redis1", "172.0.0.1"},
			wantFiles: map[string][]byte{
				"db.key": keyRing.TLSPrivateKey.PrivateKeyPEM(),
				"db.crt": certBytes,
				"db.cas": dbClientCABytes,
			},
		},
		{
			name:         "missing host",
			inFormat:     identityfile.FormatRedis,
			inOutDir:     t.TempDir(),
			inHost:       "", // missing host
			inOutFile:    "db",
			genKeyErrMsg: "at least one hostname must be specified",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ac := AuthCommand{
				output:        filepath.Join(test.inOutDir, test.inOutFile),
				outputFormat:  test.inFormat,
				signOverwrite: true,
				genHost:       test.inHost,
				genTTL:        time.Hour,
			}

			err = ac.generateDatabaseKeysForKeyRing(context.Background(), authClient, keyRing)
			if test.genKeyErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.genKeyErrMsg)
				return
			}

			require.NotNil(t, authClient.dbCertsReq)
			csr, err := tlsca.ParseCertificateRequestPEM(authClient.dbCertsReq.CSR)
			require.NoError(t, err)
			require.Equal(t, test.outSubject.String(), csr.Subject.String())
			require.Equal(t, test.outServerNames, authClient.dbCertsReq.ServerNames)
			require.Equal(t, test.outServerNames[0], authClient.dbCertsReq.ServerName)

			for wantFilename, wantContents := range test.wantFiles {
				contents, err := os.ReadFile(filepath.Join(test.inOutDir, wantFilename))
				require.NoError(t, err)
				require.Equal(t, wantContents, contents, "contents of %s match", wantFilename)
			}
		})
	}
}

// TestGenerateAppCertificates verifies cert/key pair generation for applications.
func TestGenerateAppCertificates(t *testing.T) {
	const appName = "app-1"
	const clusterNameStr = "example.com"
	const publicAddr = "https://app-1.example.com"
	const sessionID = "foobar"

	clusterName, err := services.NewClusterNameWithRandomID(
		types.ClusterNameSpecV2{
			ClusterName: clusterNameStr,
		})
	require.NoError(t, err)

	authClient := &mockClient{
		clusterName: clusterName,
		userCerts: &proto.Certs{
			SSH: []byte("SSH cert"),
			TLS: []byte("TLS cert"),
		},
		appServices: []types.AppServer{
			&types.AppServerV3{
				Metadata: types.Metadata{
					Name: appName,
				},
				Spec: types.AppServerSpecV3{
					App: &types.AppV3{
						Spec: types.AppSpecV3{
							PublicAddr: publicAddr,
						},
					},
				},
			},
		},
		appSession: &types.WebSessionV2{
			Metadata: types.Metadata{
				Name: sessionID,
			},
		},
	}

	tests := []struct {
		name        string
		outDir      string
		outFileBase string
		appName     string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			name:        "app happy path",
			outDir:      t.TempDir(),
			outFileBase: "app-1",
			appName:     "app-1",
			assertErr:   require.NoError,
		},
		{
			name:        "app non-existent",
			outDir:      t.TempDir(),
			outFileBase: "app-2",
			appName:     "app-2",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(tc.outDir, tc.outFileBase)
			ac := AuthCommand{
				output:        output,
				outputFormat:  identityfile.FormatTLS,
				signOverwrite: true,
				genTTL:        time.Hour,
				appName:       tc.appName,
			}
			err = ac.generateUserKeys(context.Background(), authClient)
			tc.assertErr(t, err)
			if err != nil {
				return
			}

			expectedRouteToApp := proto.RouteToApp{
				Name:        tc.appName,
				PublicAddr:  publicAddr,
				ClusterName: clusterNameStr,
			}
			require.Equal(t, proto.UserCertsRequest_App, authClient.userCertsReq.Usage)
			require.Equal(t, expectedRouteToApp, authClient.userCertsReq.RouteToApp)

			certBytes, err := os.ReadFile(filepath.Join(tc.outDir, tc.outFileBase+".crt"))
			require.NoError(t, err)
			require.Equal(t, authClient.userCerts.TLS, certBytes, "certificates match")
		})
	}
}

func TestGenerateDatabaseUserCertificates(t *testing.T) {
	ctx := context.Background()
	tests := map[string]struct {
		clusterName        string
		dbService          string
		dbName             string
		dbUser             string
		expectedDbProtocol string
		dbServices         []types.DatabaseServer
		expectedErr        any
	}{
		"DatabaseExists": {
			clusterName:        "example.com",
			dbService:          "db-1",
			expectedDbProtocol: defaults.ProtocolPostgres,
			dbServices: []types.DatabaseServer{
				&types.DatabaseServerV3{
					Metadata: types.Metadata{
						Name: "db-1",
					},
					Spec: types.DatabaseServerSpecV3{
						Hostname: "example.com",
						Database: &types.DatabaseV3{
							Spec: types.DatabaseSpecV3{
								Protocol: defaults.ProtocolPostgres,
							},
						},
					},
				},
			},
		},
		"DatabaseWithUserExists": {
			clusterName:        "example.com",
			dbService:          "db-user-1",
			dbUser:             "mongo-user",
			expectedDbProtocol: defaults.ProtocolMongoDB,
			dbServices: []types.DatabaseServer{
				&types.DatabaseServerV3{
					Metadata: types.Metadata{
						Name: "db-user-1",
					},
					Spec: types.DatabaseServerSpecV3{
						Hostname: "example.com",
						Database: &types.DatabaseV3{
							Spec: types.DatabaseSpecV3{
								Protocol: defaults.ProtocolMongoDB,
							},
						},
					},
				},
			},
		},
		"DatabaseWithDatabaseNameExists": {
			clusterName:        "example.com",
			dbService:          "db-user-1",
			dbName:             "root-database",
			expectedDbProtocol: defaults.ProtocolMongoDB,
			dbServices: []types.DatabaseServer{
				&types.DatabaseServerV3{
					Metadata: types.Metadata{
						Name: "db-user-1",
					},
					Spec: types.DatabaseServerSpecV3{
						Hostname: "example.com",
						Database: &types.DatabaseV3{
							Spec: types.DatabaseSpecV3{
								Protocol: defaults.ProtocolMongoDB,
							},
						},
					},
				},
			},
		},
		"DatabaseNotFound": {
			clusterName: "example.com",
			dbService:   "db-2",
			dbServices:  []types.DatabaseServer{},
			expectedErr: &trace.NotFoundError{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			clusterName, err := services.NewClusterNameWithRandomID(
				types.ClusterNameSpecV2{
					ClusterName: test.clusterName,
				})
			require.NoError(t, err)

			authClient := &mockClient{
				clusterName: clusterName,
				userCerts: &proto.Certs{
					SSH: []byte("SSH cert"),
					TLS: []byte("TLS cert"),
				},
				dbServices: test.dbServices,
			}

			certsDir := t.TempDir()
			output := filepath.Join(certsDir, test.dbService)
			ac := AuthCommand{
				output:        output,
				outputFormat:  identityfile.FormatTLS,
				signOverwrite: true,
				genTTL:        time.Hour,
				dbService:     test.dbService,
				dbName:        test.dbName,
				dbUser:        test.dbUser,
			}

			err = ac.generateUserKeys(ctx, authClient)
			if test.expectedErr != nil {
				require.Error(t, err)
				require.ErrorAs(t, err, &test.expectedErr)
				return
			}

			require.NoError(t, err)

			expectedRouteToDatabase := proto.RouteToDatabase{
				ServiceName: test.dbService,
				Protocol:    test.expectedDbProtocol,
				Database:    test.dbName,
				Username:    test.dbUser,
			}
			require.Equal(t, proto.UserCertsRequest_Database, authClient.userCertsReq.Usage)
			require.Equal(t, expectedRouteToDatabase, authClient.userCertsReq.RouteToDatabase)

			certBytes, err := os.ReadFile(filepath.Join(certsDir, test.dbService+".crt"))
			require.NoError(t, err)
			require.Equal(t, authClient.userCerts.TLS, certBytes, "certificates match")
		})
	}
}

func TestGenerateAndSignKeys(t *testing.T) {
	clusterName, err := services.NewClusterNameWithRandomID(
		types.ClusterNameSpecV2{
			ClusterName: "example.com",
		})
	require.NoError(t, err)

	_, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "example.com"}, nil, time.Minute)
	require.NoError(t, err)
	dbCARoot, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{Cert: cert}},
		},
	})
	require.NoError(t, err)

	dbCALeaf, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseCA,
		ClusterName: "leaf.example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{Cert: cert}},
		},
	})
	require.NoError(t, err)

	dbClientCARoot, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseClientCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{Cert: cert}},
		},
	})
	require.NoError(t, err)

	dbClientCALeaf, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.DatabaseClientCA,
		ClusterName: "leaf.example.com",
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{Cert: cert}},
		},
	})
	require.NoError(t, err)

	allCAs := []types.CertAuthority{dbCARoot, dbCALeaf, dbClientCARoot, dbClientCALeaf}

	certBytes := []byte("TLS cert")
	caBytes := []byte("CA cert")

	tests := []struct {
		name       string
		inFormat   identityfile.Format
		inHost     string
		inOutDir   string
		inOutFile  string
		authClient *mockClient
	}{
		{
			name:      "snowflake format",
			inFormat:  identityfile.FormatSnowflake,
			inOutDir:  t.TempDir(),
			inOutFile: "ca",
			authClient: &mockClient{
				clusterName: clusterName,
				dbCerts: &proto.DatabaseCertResponse{
					Cert:    certBytes,
					CACerts: [][]byte{caBytes},
				},
				cas: allCAs,
			},
		},
		{
			name:      "db format",
			inFormat:  identityfile.FormatDatabase,
			inOutDir:  t.TempDir(),
			inOutFile: "server",
			inHost:    "localhost",
			authClient: &mockClient{
				clusterName: clusterName,
				dbCerts: &proto.DatabaseCertResponse{
					Cert:    certBytes,
					CACerts: [][]byte{caBytes},
				},
				cas: allCAs,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ac := AuthCommand{
				output:        filepath.Join(test.inOutDir, test.inOutFile),
				outputFormat:  test.inFormat,
				signOverwrite: true,
				genHost:       test.inHost,
				genTTL:        time.Hour,
			}

			err = ac.GenerateAndSignKeys(context.Background(), test.authClient)
			require.NoError(t, err)
		})
	}
}

func TestExportCRL(t *testing.T) {
	cn, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "zarquon2",
		ClusterID:   "399282f3-4eb1-4fe9-88a1-bb98c199ced9",
	})
	require.NoError(t, err)

	cas := make([]types.CertAuthority, len(allowedCRLCertificateTypes))
	for i, certificateType := range allowedCRLCertificateTypes {
		cas[i], err = types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        types.CertAuthType(certificateType),
			ClusterName: cn.GetClusterName(),
			ActiveKeys: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					CRL: []byte{}, // Exercise fallback.
					// Cert is correct, but doesn't necessarily matches the CA types.
					// Good enough for this test.
					Cert: []byte(`-----BEGIN CERTIFICATE-----
MIIDfTCCAmWgAwIBAgIRAIuoscyl0dc9/t/KwsJQlT0wDQYJKoZIhvcNAQELBQAw
WDERMA8GA1UEChMIemFycXVvbjIxETAPBgNVBAMTCHphcnF1b24yMTAwLgYDVQQF
EycxODU2Mzg2MDM0ODY3MDA4NjUyMDAwNDQ0MzI1NTkwOTkxMjMwMDUwHhcNMjYw
NTI1MTkwNzE1WhcNMzYwNTIyMTkwNzE1WjBYMREwDwYDVQQKEwh6YXJxdW9uMjER
MA8GA1UEAxMIemFycXVvbjIxMDAuBgNVBAUTJzE4NTYzODYwMzQ4NjcwMDg2NTIw
MDA0NDQzMjU1OTA5OTEyMzAwNTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAJtsB4WxMx3XT5YPESccngAK1SrvozCxlklY0lR+T59t9NXEntvCguYmkbmH
83+rHRH7RJcle31HXmfotO0E1mLjE9wfMeSetp/4N0eEUMWGOIrzWiEpwKTuN+XD
2A7wyKQM2d6gSkXb1wwLHNP244ht/aiItOFdvVkc9SI0vwT1skT+oaHj+oG7ZeGd
mBOMtTMBr5d2tC+MBBb1q4Wa4ZaSrtyM68mhP5agGhOh+M0RjS+6j1AMcZ6+J9WH
Qzf9ARmOzmDr3mvQT7Hhx8yAF6P79+U9aZ+2lYbKOKZIG+SY+9eS2MO7lpV1d8v3
9EeGNgT+90qpEYUc7zIEC/tQ0QUCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgGmMA8G
A1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFNJZ4MklyJbcQEMnTYWtRbfhxwTQMA0G
CSqGSIb3DQEBCwUAA4IBAQBKyATEQ5vM5ADJc1MgiL6JezoLgziIypePunhCNrT4
u+1giXOCVN2hvL5Txi297gdI6mkzvXCF2A/zWu9GC52/6dVZJdzV3B0DZA1ndO9w
59aVs1M42TL4/UFMC13uVlBGDNWqVRuwbj/32f+Fk2XS5wWifCALt2FZFoC8EJtz
J5sVCOjW4770V/Rv6dKrgQ4Aetjh+uFfkwGIrS+ML9P+kGYn/DH7YjAd+G1zDO0J
GpjHiUgMO+DiVCbXvGVkCDIsTY9if7mKqK3/1NApw7dNPYTsbEd/MvBZjavTp4g3
g91LhyoHvL2hDz1Dxg3yMf5qQXgmn6b7+Cb4XGkfLii0
-----END CERTIFICATE-----`),
				}}},
		})
		require.NoError(t, err)
	}

	const caCRLPEM = `insert fallback CRL here`
	authClient := &mockClient{
		clusterName: cn,
		cas:         cas,
		crl:         []byte(caCRLPEM),
	}

	for _, caType := range allowedCRLCertificateTypes {
		t.Run(caType, func(t *testing.T) {
			ac := AuthCommand{caType: caType}
			require.NoError(t, ac.ExportCRL(t.Context(), authClient))
		})
	}

	t.Run("InvalidCAType", func(t *testing.T) {
		ac := AuthCommand{caType: "wrong-ca"}
		require.Error(t, ac.ExportCRL(t.Context(), authClient))
	})

	t.Run("export CA override CRLs", func(t *testing.T) {
		const caType = types.DatabaseClientCA

		// This test uses realistic data extracted from a test cluster.

		// CRLs for the CA keys.
		const crl0B64 = `MIIB0jCBuwIBATANBgkqhkiG9w0BAQsFADBYMREwDwYDVQQKEwh6YXJxdW9uMjERMA8GA1UEAxMIemFycXVvbjIxMDAuBgNVBAUTJzE2MTI4NzczNDA3MDQwMTExNTQ4MTE1MTQxNjc4NTg4MDc5MDMyNBcNMjYwNzAzMjE0MTU3WhcNMzYwNjMwMjE0MjU3WqAvMC0wHwYDVR0jBBgwFoAUAnBL+mnAnFYlInnsCxa16c840JcwCgYDVR0UBAMCAQEwDQYJKoZIhvcNAQELBQADggEBAHPVNlteYTPsC38KB+ClTbEKvuj4ete8eIwWeE1mPENRA7u8LgnTHMy53FwfNhwu93fUuebQi5umO+0nh4z8bYnYLnADv2V7gND3cBHGdqdUVQ+4fH9kTbH/JJ4udRfTfNcrhRzDlt2+xIh4UuYJVu99xsXeTRmpemvRevA7hwaDuAQoMtaGMI5eVjHFzCBKNuRo7GEQyRNKk/XNq6b/5mOnrvOa5YHaE2uo6Z5mw+CrvF7L3yEAqjhG+RGgsfS/wbn7GTRcNkOk+dq8YOrjzo3bH5O88fCJhTyZYHXBopOM1jzE8KI9BLiULsvTMta5tvAAimtlPA31w/7Wc4DTmuo=`
		const crl1B64 = `MIIB0TCBugIBATANBgkqhkiG9w0BAQsFADBXMREwDwYDVQQKEwh6YXJxdW9uMjERMA8GA1UEAxMIemFycXVvbjIxLzAtBgNVBAUTJjk4OTc4MjA4ODYwNjY0NDI4MTk2MDc4NDM4NDk4OTYyMjQyNzQyFw0yNjA3MDMyMTQyMTFaFw0zNjA2MzAyMTQzMTFaoC8wLTAfBgNVHSMEGDAWgBRwPftLdWlY7fcq2R52heLZ4uOn4TAKBgNVHRQEAwIBATANBgkqhkiG9w0BAQsFAAOCAQEAmY2p9h1QZ39TrZnhnVycgwnGQIGwM/BtFqM7xcX0kgbx46eAbZCG8jZRFzDOy25xIzPAe0JtfhOo/DCkIqoUL9OFVCrQm3xFoNGcGugFhelwOiONYOLllsCYPvYnIDKCj4UgI46BsqKYBqo0ajFx7ZkuYCMx1NkFBNMhRKen9hM3VxITlt6OqI5Ti38SGaVwde+mmTOMBYw+EBq37nga4Aty759rQ735qdoWNG+CBWad8CX8AhkEV8IA2ArDKmqv7D5NX8FVE2S6zDGLWEr0rzavJ58h1U/mhNPxTcICTUdXYBCTXYsjaDSdYxh3R22hMTK88AtxscEK1olf4i++uQ==`
		crl0DER, err := base64.StdEncoding.DecodeString(crl0B64)
		require.NoError(t, err)
		crl1DER, err := base64.StdEncoding.DecodeString(crl1B64)
		require.NoError(t, err)

		dbCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        caType,
			ClusterName: cn.GetClusterName(),
			ActiveKeys: types.CAKeySet{
				TLS: []*types.TLSKeyPair{
					{
						Cert: []byte(`-----BEGIN CERTIFICATE-----
MIIDfDCCAmSgAwIBAgIQeVbjPmWsOfHncpf+XTi1NDANBgkqhkiG9w0BAQsFADBY
MREwDwYDVQQKEwh6YXJxdW9uMjERMA8GA1UEAxMIemFycXVvbjIxMDAuBgNVBAUT
JzE2MTI4NzczNDA3MDQwMTExNTQ4MTE1MTQxNjc4NTg4MDc5MDMyNDAeFw0yNjA3
MDMyMTQyNTdaFw0zNjA2MzAyMTQyNTdaMFgxETAPBgNVBAoTCHphcnF1b24yMREw
DwYDVQQDEwh6YXJxdW9uMjEwMC4GA1UEBRMnMTYxMjg3NzM0MDcwNDAxMTE1NDgx
MTUxNDE2Nzg1ODgwNzkwMzI0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKC
AQEAvM447Iks2+YBGKnXutWtyI95qip8j+XwhfWvZU8/UibUdK82Tt5XgEi29bVy
ZlylpgFng8zYJXtaJ+YOLvrmRj0+4yMxP3auMRj4zZagmuWYquB9krCc9/NvLtTA
Ij76/dEeFUQ+Tp6InlDpHF7XH8iIe2/axDsp1osYNMj6bf0ceKCNJnpb53Xdab0V
a5roeb8xNjDmshb7UVyLtonKcZp0yGia/ewvuwxnu+eVx2iI++oqY6w6ZFdy7kML
VzMnQzIKjoziso0vzgCo7CsHC9uhOssMcTZEIeQvawINDstkGPxn5tam+q7XcrzP
xp2dKY8cB6owLjvRyE9z+LovOwIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAaYwDwYD
VR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUAnBL+mnAnFYlInnsCxa16c840JcwDQYJ
KoZIhvcNAQELBQADggEBABfTDRHfs6qrcxYYrXWax8M6UzsTflveNgU+uaFNiK+U
IKZ6D3ffrbMlgeVdBZE7x49LQ0rlM0nQrCPnQ+zCM3gCQvb7z2+kM7zjve5AE+Yh
PSp04g+KBziEi7Gph/acungRSgvShpPnhjr6j+CKgx12OVsQR0RKJCHNar1OCEMl
8u03f2e2UHV2rSoWPH6m40unNVUFmRAaKA4GYNxyzKSt9I53XXYmLWKxSgo1Cs5j
BPVpD3G/VE471YE4VXyP57nGCF4f/I3q0giHu5J3HciI+hstqGv6/yRzCe1v3l+o
qaN2yzRzuW5Kxr0Lq39hGVjbrrG6RuSq8Z4PoSOeJkM=
-----END CERTIFICATE-----`),
						CRL: crl0DER,
					},
					{
						Cert: []byte(`-----BEGIN CERTIFICATE-----
MIIDejCCAmKgAwIBAgIQSnaCdwYN6cRDxWdfh2v8tjANBgkqhkiG9w0BAQsFADBX
MREwDwYDVQQKEwh6YXJxdW9uMjERMA8GA1UEAxMIemFycXVvbjIxLzAtBgNVBAUT
Jjk4OTc4MjA4ODYwNjY0NDI4MTk2MDc4NDM4NDk4OTYyMjQyNzQyMB4XDTI2MDcw
MzIxNDMxMVoXDTM2MDYzMDIxNDMxMVowVzERMA8GA1UEChMIemFycXVvbjIxETAP
BgNVBAMTCHphcnF1b24yMS8wLQYDVQQFEyY5ODk3ODIwODg2MDY2NDQyODE5NjA3
ODQzODQ5ODk2MjI0Mjc0MjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AODhC8EStri0FiF5jkEVOPJCnYSE+qwiEwTk7LecjRO3ek1JVCCG0UpbhmzeNpa1
CqTa/LJ3nwCxp5F4MfP+OSKHQ0r0OSqQhx8N7mF4ug/E4onivVkzokX02T8C/lww
ncES+yBW6lJLlsEiZfvqpJZRhJZ2ij1XjBsWe7GhUXth0sfdxoOHL8DEkaOxMojG
fx+JbfsMEsqIEwECHG2mjgzHSEgPSuqFYXn1XKu9vYQXGHaUa0tH3RrmUXczvP1w
yE/nu8AWryUtPNsBqqrdKJomST714oEZvwzkT4fTCdpvCITY2aAouBhdPxa8772A
IWOYQxyZXahVGkKrpHlAQ4MCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgGmMA8GA1Ud
EwEB/wQFMAMBAf8wHQYDVR0OBBYEFHA9+0t1aVjt9yrZHnaF4tni46fhMA0GCSqG
SIb3DQEBCwUAA4IBAQAF4jFGIoZhBuB4n4AnfC8bUyf2tHke3PkM2C5orVqQwMTl
XOaA2BKTXKuIJlcGNzgI3Zr5i7Gh9y4f7MwQonruffGLN/rR5NnpzA6Kkf/p2ksm
wSvDPiCkGyxCDg3Yigx/MD7LHReC2CDU3CsnBjTENVX4yj71LwYQUslUYi542yzl
TsSpWpepmZtL6l0PvsmfvVErK9I4qOi5pKlpyI9gvDy1q+8FqqVcf0NWF0NPQwCq
UVA40Q9XMe+1LYkPf1DQSR2jqQvle8zBDPIRLupK0DFlRfjMEK3xFw8c26itsJB+
4fdpEnavGFEZi5jrrj4Nf8oxw41QVXb18E92YS42
-----END CERTIFICATE-----`),
						CRL: crl1DER,
					},
				},
			},
		})
		require.NoError(t, err)

		dbOverride := subcav1.CertAuthorityOverride_builder{
			Kind:    types.KindCertAuthorityOverride,
			SubKind: string(caType),
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: cn.GetClusterName(),
			}.Build(),
			Spec: subcav1.CertAuthorityOverrideSpec_builder{
				CertificateOverrides: []*subcav1.CertificateOverride{
					subcav1.CertificateOverride_builder{
						PublicKey: "ea16c3a8c1f31943019ecc9bfb2899b60e8ec156874bdf4606a899c95392cef3",
						Certificate: `-----BEGIN CERTIFICATE-----
MIICmTCCAkCgAwIBAgIUZnPGsB96Sun0DmoXogO492VLDFUwCgYIKoZIzj0EAwIw
RDETMBEGA1UEChMKTGxhbWEgQ29ycDERMA8GA1UECxMITGxhbWEgQ0ExGjAYBgNV
BAMTEUxsYW1hIERhdGFiYXNlIENBMB4XDTI2MDcwNjE3MTUwMFoXDTMxMDcwNTE3
MTUwMFowJjERMA8GA1UEChMIemFycXVvbjIxETAPBgNVBAMTCHphcnF1b24yMIIB
IjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvM447Iks2+YBGKnXutWtyI95
qip8j+XwhfWvZU8/UibUdK82Tt5XgEi29bVyZlylpgFng8zYJXtaJ+YOLvrmRj0+
4yMxP3auMRj4zZagmuWYquB9krCc9/NvLtTAIj76/dEeFUQ+Tp6InlDpHF7XH8iI
e2/axDsp1osYNMj6bf0ceKCNJnpb53Xdab0Va5roeb8xNjDmshb7UVyLtonKcZp0
yGia/ewvuwxnu+eVx2iI++oqY6w6ZFdy7kMLVzMnQzIKjoziso0vzgCo7CsHC9uh
OssMcTZEIeQvawINDstkGPxn5tam+q7XcrzPxp2dKY8cB6owLjvRyE9z+LovOwID
AQABo2MwYTAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4E
FgQURQRbVb2EI4egIQZ2zt/oXcb2NC0wHwYDVR0jBBgwFoAUDC1GIwweTul8uBzi
Q4FKcCFBuhowCgYIKoZIzj0EAwIDRwAwRAIgQiPzqSjKq8ucTqmBNVeZklTuyw5M
kTSEqy+nGEix+p0CIHRzwR+NglPmmAyxH6bSFZ95a96MmhziET2vaoTicSEW
-----END CERTIFICATE-----`,
						Disabled: true, // Doesn't matter, should still print.
					}.Build(),
					subcav1.CertificateOverride_builder{
						PublicKey: "1cd6a96e049f643d1f8c1cdd0390c08c2a7587df204ba254ee46009c08e80456",
						Certificate: `-----BEGIN CERTIFICATE-----
MIIDejCCAmKgAwIBAgIQSnaCdwYN6cRDxWdfh2v8tjANBgkqhkiG9w0BAQsFADBX
MREwDwYDVQQKEwh6YXJxdW9uMjERMA8GA1UEAxMIemFycXVvbjIxLzAtBgNVBAUT
Jjk4OTc4MjA4ODYwNjY0NDI4MTk2MDc4NDM4NDk4OTYyMjQyNzQyMB4XDTI2MDcw
MzIxNDMxMVoXDTM2MDYzMDIxNDMxMVowVzERMA8GA1UEChMIemFycXVvbjIxETAP
BgNVBAMTCHphcnF1b24yMS8wLQYDVQQFEyY5ODk3ODIwODg2MDY2NDQyODE5NjA3
ODQzODQ5ODk2MjI0Mjc0MjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AODhC8EStri0FiF5jkEVOPJCnYSE+qwiEwTk7LecjRO3ek1JVCCG0UpbhmzeNpa1
CqTa/LJ3nwCxp5F4MfP+OSKHQ0r0OSqQhx8N7mF4ug/E4onivVkzokX02T8C/lww
ncES+yBW6lJLlsEiZfvqpJZRhJZ2ij1XjBsWe7GhUXth0sfdxoOHL8DEkaOxMojG
fx+JbfsMEsqIEwECHG2mjgzHSEgPSuqFYXn1XKu9vYQXGHaUa0tH3RrmUXczvP1w
yE/nu8AWryUtPNsBqqrdKJomST714oEZvwzkT4fTCdpvCITY2aAouBhdPxa8772A
IWOYQxyZXahVGkKrpHlAQ4MCAwEAAaNCMEAwDgYDVR0PAQH/BAQDAgGmMA8GA1Ud
EwEB/wQFMAMBAf8wHQYDVR0OBBYEFHA9+0t1aVjt9yrZHnaF4tni46fhMA0GCSqG
SIb3DQEBCwUAA4IBAQAF4jFGIoZhBuB4n4AnfC8bUyf2tHke3PkM2C5orVqQwMTl
XOaA2BKTXKuIJlcGNzgI3Zr5i7Gh9y4f7MwQonruffGLN/rR5NnpzA6Kkf/p2ksm
wSvDPiCkGyxCDg3Yigx/MD7LHReC2CDU3CsnBjTENVX4yj71LwYQUslUYi542yzl
TsSpWpepmZtL6l0PvsmfvVErK9I4qOi5pKlpyI9gvDy1q+8FqqVcf0NWF0NPQwCq
UVA40Q9XMe+1LYkPf1DQSR2jqQvle8zBDPIRLupK0DFlRfjMEK3xFw8c26itsJB+
4fdpEnavGFEZi5jrrj4Nf8oxw41QVXb18E92YS42
-----END CERTIFICATE-----`,
						Chain:    []string{},
						Disabled: false,
					}.Build(),
				},
			}.Build(),
			Status: subcav1.CertAuthorityOverrideStatus_builder{
				PublicKeyHashToCrl: map[string]*subcav1.CertificateRevocationList{
					"ea16c3a8c1f31943019ecc9bfb2899b60e8ec156874bdf4606a899c95392cef3": subcav1.CertificateRevocationList_builder{
						Pem: `-----BEGIN X509 CRL-----
MIIBoDCBiQIBATANBgkqhkiG9w0BAQsFADAmMREwDwYDVQQKEwh6YXJxdW9uMjER
MA8GA1UEAxMIemFycXVvbjIXDTI2MDgxNDE3NDc0MloXDTM2MDYzMDIxNDI1N1qg
LzAtMB8GA1UdIwQYMBaAFEUEW1W9hCOHoCEGds7f6F3G9jQtMAoGA1UdFAQDAgEB
MA0GCSqGSIb3DQEBCwUAA4IBAQCRGbSir414VBt3g5ZUI4gv9E3nvFu09rZBWOj6
i30gHEsZZog0CBfP82gZU55SknJFeI/MpcpzUYedyuQw9caZNHYtHZEvosY5VbL5
pNIvX0kPlvLb5noX5T+X3Nu5WC1OI0Ms7Wkf8/+BcBt92OFfOdiA4q2ayKQINg9S
5ppN//kyVTwL5SSDXneDzwFzWELKDn2FkAtAdw4AdmOzcyNy+UrroOiKq+3j9GIa
PDtU51hI1fJYfKQsw1d2MOfbUgPjB8uytX7nDtE804XvZc2pi+Ad3ETAO6JnJsN8
T5uCDcQc3NmievDZ4l0Z5mW/EZY2G2p4GTbMlHQ/jn9trdAw
-----END X509 CRL-----`,
					}.Build(),
					"1cd6a96e049f643d1f8c1cdd0390c08c2a7587df204ba254ee46009c08e80456": subcav1.CertificateRevocationList_builder{
						Pem: `-----BEGIN X509 CRL-----
MIIBoDCBiQIBATANBgkqhkiG9w0BAQsFADAmMREwDwYDVQQKEwh6YXJxdW9uMjER
MA8GA1UEAxMIemFycXVvbjIXDTI2MDgxNDE3NDc0NVoXDTM2MDYzMDIxNDMxMVqg
LzAtMB8GA1UdIwQYMBaAFEy7tFBeWHGV2/R88jUvUqJ9rSNaMAoGA1UdFAQDAgEB
MA0GCSqGSIb3DQEBCwUAA4IBAQBzRwg3M4ISfibS4G4vYhkrUC5sjT6xY7Y5mdOw
6TMDwqEMLHY03wRlsrSRriAZWiIn4EryWFqLUBa0NoOeGWnlzRiECWqrheqQx8nK
GRZiwVVgd2WccI+/8keyXo3INhQ8v74GOkJSHVKky1mVcI7Y6kbuAfBEExbK4ZFN
es8dmGd0jSRo34VydUMdQ4JSZhKdSLg8MS0fQBB9Wu6ThGVz6VSopczNRZvuNR7R
E756JiXjWgGMayVLpU2eODddiIIZgwUOyeqcYUu5bcqtQcm/mLUUKIIzsy+1/Qgv
CUtq0a2VtxmCG/KK3WLQKMk5X2VeOSX+kW8NH+UR8dOKwH6w
-----END X509 CRL-----`,
					}.Build(),
				},
			}.Build(),
		}.Build()

		// Parse override CRLs.
		override0PK := dbOverride.GetSpec().GetCertificateOverrides()[0].GetPublicKey()
		override1PK := dbOverride.GetSpec().GetCertificateOverrides()[1].GetPublicKey()
		override0CRLBlock, _ := pem.Decode([]byte(dbOverride.GetStatus().GetPublicKeyHashToCrl()[override0PK].GetPem()))
		require.NotNil(t, override0CRLBlock, "Failed to decoded CRL PEM")
		override1CRLBlock, _ := pem.Decode([]byte(dbOverride.GetStatus().GetPublicKeyHashToCrl()[override1PK].GetPem()))
		require.NotNil(t, override1CRLBlock, "Failed to decoded CRL PEM")

		authClient := &mockClient{
			clusterName: cn,
			cas:         []types.CertAuthority{dbCA},
			caOverride:  dbOverride,
		}

		stderr := &strings.Builder{}
		ac := AuthCommand{
			caType:         string(caType),
			stderrOverride: stderr,
		}
		// This should fail, we only allow 1 PEM to stdout.
		require.ErrorContains(t, ac.ExportCRL(t.Context(), authClient), "CA has multiple exportable CRLs")
		stderr.Reset()

		tempDir := t.TempDir()
		ac.output = filepath.Join(tempDir, "c")
		require.NoError(t, ac.ExportCRL(t.Context(), authClient))

		// Filenames are deterministic.
		wantFiles := map[string][]byte{
			tempDir + "/c-db_client-ea16c3a8.crl":          crl0DER,
			tempDir + "/c-db_client-1cd6a96e.crl":          crl1DER,
			tempDir + "/c-db_client-override-ea16c3a8.crl": override0CRLBlock.Bytes,
			tempDir + "/c-db_client-override-1cd6a96e.crl": override1CRLBlock.Bytes,
		}
		for path, want := range wantFiles {
			got, err := os.ReadFile(path)
			if assert.NoError(t, err, "read output file") {
				assert.Equal(t, want, got, "output file %q contents mismatch", path)
			}
		}

		// Assert instructions.
		wantInstructions := []string{
			`certutil -dspublish ` + tempDir + `/c-db_client-ea16c3a8.crl TeleportDB "09O4NUJ9O2E5C992F7M0M5LLT77JHK4N_zarquon2"`,
			`certutil -dspublish ` + tempDir + `/c-db_client-1cd6a96e.crl TeleportDB "E0UVMIRLD5CERTPAR4F7D1F2R7HE79V1_zarquon2"`,
			`certutil -dspublish ` + tempDir + `/c-db_client-override-ea16c3a8.crl TeleportDB "8K25MLDTGGHOF8110PRCTNV8BN3FCD1D_zarquon2"`,
			`certutil -dspublish ` + tempDir + `/c-db_client-override-1cd6a96e.crl TeleportDB "E0UVMIRLD5CERTPAR4F7D1F2R7HE79V1_zarquon2"`,
		}
		t.Logf("Command stderr:\n[%s]\n\n", stderr)
		lines := strings.Split(stderr.String(), "\n")
		for _, want := range wantInstructions {
			found := slices.ContainsFunc(lines, func(line string) bool {
				return strings.Contains(line, want)
			})
			assert.True(t, found, "Missing instruction in stderr: %q", want)
		}
	})
}
