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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/fixtures"
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
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
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
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
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
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
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
		tt := tt
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
	proxies        []types.Server
	remoteClusters []types.RemoteCluster
	kubeServers    []types.KubeServer
	appServices    []types.AppServer
	dbServices     []types.DatabaseServer
	appSession     types.WebSession
	networkConfig  types.ClusterNetworkingConfig
	crl            []byte
}

func (c *mockClient) GetClusterName(...services.MarshalOption) (types.ClusterName, error) {
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

func (c *mockClient) GetCertAuthorities(_ context.Context, caType types.CertAuthType, _ bool) ([]types.CertAuthority, error) {
	return c.cas, nil
}

func (c *mockClient) GetProxies() ([]types.Server, error) {
	return c.proxies, nil
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
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
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
		expectedErr        error
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
			expectedErr: trace.NotFound(""),
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
				require.IsType(t, test.expectedErr, err)
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

func TestGenerateCRLForCA(t *testing.T) {
	ctx := context.Background()

	for _, caType := range allowedCRLCertificateTypes {
		t.Run(caType, func(t *testing.T) {
			ac := AuthCommand{caType: caType}
			authClient := &mockClient{crl: []byte{}}
			require.NoError(t, ac.GenerateCRLForCA(ctx, authClient))
		})
	}

	t.Run("InvalidCAType", func(t *testing.T) {
		ac := AuthCommand{caType: "wrong-ca"}
		authClient := &mockClient{crl: []byte{}}
		require.Error(t, ac.GenerateCRLForCA(ctx, authClient))
	})
}
