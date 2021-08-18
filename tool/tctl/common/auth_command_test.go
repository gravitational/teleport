package common

import (
	"bytes"
	"context"
	"crypto/x509/pkix"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAuthSignKubeconfig(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "auth_command_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	remoteCluster, err := types.NewRemoteCluster("leaf.example.com")
	if err != nil {
		t.Fatal(err)
	}

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{PublicKey: []byte("SSH CA cert")}},
			TLS: []*types.TLSKeyPair{{Cert: []byte("TLS CA cert")}},
		},
		Roles:      nil,
		SigningAlg: types.CertAuthoritySpecV2_RSA_SHA2_512,
	})
	require.NoError(t, err)

	client := &mockClient{
		clusterName:    clusterName,
		remoteClusters: []types.RemoteCluster{remoteCluster},
		userCerts: &proto.Certs{
			SSH: []byte("SSH cert"),
			TLS: []byte("TLS cert"),
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
					PublicAddr: "proxy-from-api.example.com:3080",
				},
			},
		},
	}
	tests := []struct {
		desc        string
		ac          AuthCommand
		wantAddr    string
		wantCluster string
		wantError   string
	}{
		{
			desc: "--proxy specified",
			ac: AuthCommand{
				output:        filepath.Join(tmpDir, "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				proxyAddr:     "proxy-from-flag.example.com",
			},
			wantAddr: "proxy-from-flag.example.com",
		},
		{
			desc: "k8s proxy running locally with public_addr",
			ac: AuthCommand{
				output:        filepath.Join(tmpDir, "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &service.Config{Proxy: service.ProxyConfig{Kube: service.KubeProxyConfig{
					Enabled:     true,
					PublicAddrs: []utils.NetAddr{{Addr: "proxy-from-config.example.com:3026"}},
				}}},
			},
			wantAddr: "https://proxy-from-config.example.com:3026",
		},
		{
			desc: "k8s proxy running locally without public_addr",
			ac: AuthCommand{
				output:        filepath.Join(tmpDir, "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &service.Config{Proxy: service.ProxyConfig{
					Kube: service.KubeProxyConfig{
						Enabled: true,
					},
					PublicAddrs: []utils.NetAddr{{Addr: "proxy-from-config.example.com:3080"}},
				}},
			},
			wantAddr: "https://proxy-from-config.example.com:3026",
		},
		{
			desc: "k8s proxy from cluster info",
			ac: AuthCommand{
				output:        filepath.Join(tmpDir, "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				config: &service.Config{Proxy: service.ProxyConfig{
					Kube: service.KubeProxyConfig{
						Enabled: false,
					},
				}},
			},
			wantAddr: "https://proxy-from-api.example.com:3026",
		},
		{
			desc: "--kube-cluster specified with valid cluster",
			ac: AuthCommand{
				output:        filepath.Join(tmpDir, "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				leafCluster:   remoteCluster.GetMetadata().Name,
				config: &service.Config{Proxy: service.ProxyConfig{
					Kube: service.KubeProxyConfig{
						Enabled: false,
					},
				}},
			},
			wantCluster: remoteCluster.GetMetadata().Name,
		},
		{
			desc: "--kube-cluster specified with invalid cluster",
			ac: AuthCommand{
				output:        filepath.Join(tmpDir, "kubeconfig"),
				outputFormat:  identityfile.FormatKubernetes,
				signOverwrite: true,
				leafCluster:   "doesnotexist.example.com",
				config: &service.Config{Proxy: service.ProxyConfig{
					Kube: service.KubeProxyConfig{
						Enabled: false,
					},
				}},
			},
			wantError: "couldn't find leaf cluster named \"doesnotexist.example.com\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Generate kubeconfig.
			if err = tt.ac.generateUserKeys(client); err != nil && tt.wantError == "" {
				t.Fatalf("generating KubeProxyConfig: %v", err)
			}

			if tt.wantError != "" && (err == nil || err.Error() != tt.wantError) {
				t.Errorf("got error %v, want %v", err, tt.wantError)
			}

			// Validate kubeconfig contents.
			kc, err := kubeconfig.Load(tt.ac.output)
			if err != nil {
				t.Fatalf("loading generated kubeconfig: %v", err)
			}
			gotCert := kc.AuthInfos[kc.CurrentContext].ClientCertificateData
			if !bytes.Equal(gotCert, client.userCerts.TLS) {
				t.Errorf("got client cert: %q, want %q", gotCert, client.userCerts.TLS)
			}
			gotCA := kc.Clusters[kc.CurrentContext].CertificateAuthorityData
			wantCA := ca.GetActiveKeys().TLS[0].Cert
			if !bytes.Equal(gotCA, wantCA) {
				t.Errorf("got CA cert: %q, want %q", gotCA, wantCA)
			}
			gotServerAddr := kc.Clusters[kc.CurrentContext].Server
			if tt.wantAddr != "" && gotServerAddr != tt.wantAddr {
				t.Errorf("got server address: %q, want %q", gotServerAddr, tt.wantAddr)
			}
			if tt.wantCluster != "" && kc.CurrentContext != tt.wantCluster {
				t.Errorf("got cluster: %q, want %q", kc.CurrentContext, tt.wantCluster)
			}
		})
	}
}

type mockClient struct {
	auth.ClientI

	clusterName    types.ClusterName
	userCerts      *proto.Certs
	dbCertsReq     *proto.DatabaseCertRequest
	dbCerts        *proto.DatabaseCertResponse
	cas            []types.CertAuthority
	proxies        []types.Server
	remoteClusters []types.RemoteCluster
	kubeServices   []types.Server
}

func (c *mockClient) GetClusterName(...services.MarshalOption) (types.ClusterName, error) {
	return c.clusterName, nil
}
func (c *mockClient) GenerateUserCerts(context.Context, proto.UserCertsRequest) (*proto.Certs, error) {
	return c.userCerts, nil
}
func (c *mockClient) GetCertAuthorities(types.CertAuthType, bool, ...services.MarshalOption) ([]types.CertAuthority, error) {
	return c.cas, nil
}
func (c *mockClient) GetProxies() ([]types.Server, error) {
	return c.proxies, nil
}
func (c *mockClient) GetRemoteClusters(opts ...services.MarshalOption) ([]types.RemoteCluster, error) {
	return c.remoteClusters, nil
}
func (c *mockClient) GetKubeServices(context.Context) ([]types.Server, error) {
	return c.kubeServices, nil
}
func (c *mockClient) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	c.dbCertsReq = req
	return c.dbCerts, nil
}

func TestCheckKubeCluster(t *testing.T) {
	const teleportCluster = "local-teleport"
	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: teleportCluster,
	})
	require.NoError(t, err)
	client := &mockClient{
		clusterName: clusterName,
	}
	tests := []struct {
		desc               string
		kubeCluster        string
		leafCluster        string
		outputFormat       identityfile.Format
		registeredClusters []*types.KubernetesCluster
		want               string
		assertErr          require.ErrorAssertionFunc
	}{
		{
			desc:         "non-k8s output format",
			outputFormat: identityfile.FormatFile,
			assertErr:    require.NoError,
		},
		{
			desc:               "local cluster, valid kube cluster",
			kubeCluster:        "foo",
			leafCluster:        teleportCluster,
			registeredClusters: []*types.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "foo",
			assertErr:          require.NoError,
		},
		{
			desc:               "local cluster, empty kube cluster",
			kubeCluster:        "",
			leafCluster:        teleportCluster,
			registeredClusters: []*types.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "foo",
			assertErr:          require.NoError,
		},
		{
			desc:               "local cluster, empty kube cluster, no registered kube clusters",
			kubeCluster:        "",
			leafCluster:        teleportCluster,
			registeredClusters: []*types.KubernetesCluster{},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "",
			assertErr:          require.NoError,
		},
		{
			desc:               "local cluster, invalid kube cluster",
			kubeCluster:        "bar",
			leafCluster:        teleportCluster,
			registeredClusters: []*types.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			assertErr:          require.Error,
		},
		{
			desc:               "remote cluster, empty kube cluster",
			kubeCluster:        "",
			leafCluster:        "remote-teleport",
			registeredClusters: []*types.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "",
			assertErr:          require.NoError,
		},
		{
			desc:               "remote cluster, non-empty kube cluster",
			kubeCluster:        "bar",
			leafCluster:        "remote-teleport",
			registeredClusters: []*types.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "bar",
			assertErr:          require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			client.kubeServices = []types.Server{&types.ServerV2{
				Spec: types.ServerSpecV2{
					KubernetesClusters: tt.registeredClusters,
				},
			}}
			a := &AuthCommand{
				kubeCluster:  tt.kubeCluster,
				leafCluster:  tt.leafCluster,
				outputFormat: tt.outputFormat,
			}
			err := a.checkKubeCluster(client)
			tt.assertErr(t, err)
			require.Equal(t, tt.want, a.kubeCluster)
		})
	}
}

// TestGenerateDatabaseKeys verifies cert/key pair generation for databases.
func TestGenerateDatabaseKeys(t *testing.T) {
	clusterName, err := services.NewClusterNameWithRandomID(
		types.ClusterNameSpecV2{
			ClusterName: "example.com",
		})
	require.NoError(t, err)

	certBytes := []byte("TLS cert")
	caBytes := []byte("CA cert")

	authClient := &mockClient{
		clusterName: clusterName,
		dbCerts: &proto.DatabaseCertResponse{
			Cert:    certBytes,
			CACerts: [][]byte{caBytes},
		},
	}

	key, err := client.NewKey()
	require.NoError(t, err)

	tests := []struct {
		name       string
		inFormat   identityfile.Format
		inHost     string
		outSubject pkix.Name
		outKey     []byte
		outCert    []byte
		outCA      []byte
	}{
		{
			name:       "database certificate",
			inFormat:   identityfile.FormatDatabase,
			inHost:     "postgres.example.com",
			outSubject: pkix.Name{CommonName: "postgres.example.com"},
			outKey:     key.Priv,
			outCert:    certBytes,
			outCA:      caBytes,
		},
		{
			name:       "mongodb certificate",
			inFormat:   identityfile.FormatMongo,
			inHost:     "mongo.example.com",
			outSubject: pkix.Name{CommonName: "mongo.example.com", Organization: []string{"example.com"}},
			outCert:    append(certBytes, key.Priv...),
			outCA:      caBytes,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ac := AuthCommand{
				output:        filepath.Join(t.TempDir(), "db"),
				outputFormat:  test.inFormat,
				signOverwrite: true,
				genHost:       test.inHost,
				genTTL:        time.Hour,
			}

			err = ac.generateDatabaseKeysForKey(authClient, key)
			require.NoError(t, err)

			require.NotNil(t, authClient.dbCertsReq)
			csr, err := tlsca.ParseCertificateRequestPEM(authClient.dbCertsReq.CSR)
			require.NoError(t, err)
			require.Equal(t, test.outSubject.String(), csr.Subject.String())

			if len(test.outKey) > 0 {
				keyBytes, err := ioutil.ReadFile(ac.output + ".key")
				require.NoError(t, err)
				require.Equal(t, test.outKey, keyBytes, "keys match")
			}

			if len(test.outCert) > 0 {
				certBytes, err := ioutil.ReadFile(ac.output + ".crt")
				require.NoError(t, err)
				require.Equal(t, test.outCert, certBytes, "certificates match")
			}

			if len(test.outCA) > 0 {
				caBytes, err := ioutil.ReadFile(ac.output + ".cas")
				require.NoError(t, err)
				require.Equal(t, test.outCA, caBytes, "CA certificates match")
			}
		})
	}
}
