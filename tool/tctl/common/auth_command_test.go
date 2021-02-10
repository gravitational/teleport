package common

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

func TestAuthSignKubeconfig(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "auth_command_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	remoteCluster, err := services.NewRemoteCluster("leaf.example.com")
	if err != nil {
		t.Fatal(err)
	}

	ca := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:         services.HostCA,
		ClusterName:  "example.com",
		SigningKeys:  nil,
		CheckingKeys: [][]byte{[]byte("SSH CA cert")},
		Roles:        nil,
		SigningAlg:   services.CertAuthoritySpecV2_RSA_SHA2_512,
	})
	ca.SetTLSKeyPairs([]services.TLSKeyPair{{Cert: []byte("TLS CA cert")}})

	client := mockClient{
		clusterName:    clusterName,
		remoteClusters: []services.RemoteCluster{remoteCluster},
		userCerts: &proto.Certs{
			SSH: []byte("SSH cert"),
			TLS: []byte("TLS cert"),
		},
		cas: []services.CertAuthority{ca},
		proxies: []services.Server{
			&services.ServerV2{
				Kind:    services.KindNode,
				Version: services.V2,
				Metadata: services.Metadata{
					Name: "proxy",
				},
				Spec: services.ServerSpecV2{
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
			wantCA := ca.GetTLSKeyPairs()[0].Cert
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
	client.ClientI

	clusterName    services.ClusterName
	userCerts      *proto.Certs
	dbCerts        *proto.DatabaseCertResponse
	cas            []services.CertAuthority
	proxies        []services.Server
	remoteClusters []services.RemoteCluster
	kubeServices   []services.Server
}

func (c mockClient) GetClusterName(...auth.MarshalOption) (services.ClusterName, error) {
	return c.clusterName, nil
}
func (c mockClient) GenerateUserCerts(context.Context, proto.UserCertsRequest) (*proto.Certs, error) {
	return c.userCerts, nil
}
func (c mockClient) GetCertAuthorities(services.CertAuthType, bool, ...auth.MarshalOption) ([]services.CertAuthority, error) {
	return c.cas, nil
}
func (c mockClient) GetProxies() ([]services.Server, error) {
	return c.proxies, nil
}
func (c mockClient) GetRemoteClusters(opts ...auth.MarshalOption) ([]services.RemoteCluster, error) {
	return c.remoteClusters, nil
}
func (c mockClient) GetKubeServices(context.Context) ([]services.Server, error) {
	return c.kubeServices, nil
}
func (c mockClient) GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	return c.dbCerts, nil
}

func TestCheckKubeCluster(t *testing.T) {
	const teleportCluster = "local-teleport"
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: teleportCluster,
	})
	require.NoError(t, err)
	client := mockClient{
		clusterName: clusterName,
	}
	tests := []struct {
		desc               string
		kubeCluster        string
		leafCluster        string
		outputFormat       identityfile.Format
		registeredClusters []*services.KubernetesCluster
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
			registeredClusters: []*services.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "foo",
			assertErr:          require.NoError,
		},
		{
			desc:               "local cluster, empty kube cluster",
			kubeCluster:        "",
			leafCluster:        teleportCluster,
			registeredClusters: []*services.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "foo",
			assertErr:          require.NoError,
		},
		{
			desc:               "local cluster, empty kube cluster, no registered kube clusters",
			kubeCluster:        "",
			leafCluster:        teleportCluster,
			registeredClusters: []*services.KubernetesCluster{},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "",
			assertErr:          require.NoError,
		},
		{
			desc:               "local cluster, invalid kube cluster",
			kubeCluster:        "bar",
			leafCluster:        teleportCluster,
			registeredClusters: []*services.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			assertErr:          require.Error,
		},
		{
			desc:               "remote cluster, empty kube cluster",
			kubeCluster:        "",
			leafCluster:        "remote-teleport",
			registeredClusters: []*services.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "",
			assertErr:          require.NoError,
		},
		{
			desc:               "remote cluster, non-empty kube cluster",
			kubeCluster:        "bar",
			leafCluster:        "remote-teleport",
			registeredClusters: []*services.KubernetesCluster{{Name: "foo"}},
			outputFormat:       identityfile.FormatKubernetes,
			want:               "bar",
			assertErr:          require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			client.kubeServices = []services.Server{&services.ServerV2{
				Spec: services.ServerSpecV2{
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

func TestGenerateDatabaseKeys(t *testing.T) {
	tmpDir := t.TempDir()

	client := mockClient{
		dbCerts: &proto.DatabaseCertResponse{
			Cert: []byte("TLS cert"),
			CACerts: [][]byte{
				[]byte("CA cert"),
			},
		},
	}

	ac := AuthCommand{
		output:        filepath.Join(tmpDir, "db"),
		outputFormat:  identityfile.FormatDatabase,
		signOverwrite: true,
		genHost:       "example.com",
		genTTL:        time.Hour,
	}

	err := ac.GenerateAndSignKeys(client)
	require.NoError(t, err)

	certBytes, err := ioutil.ReadFile(ac.output + ".crt")
	require.NoError(t, err)
	require.Equal(t, client.dbCerts.Cert, certBytes, "certificates match")

	caBytes, err := ioutil.ReadFile(ac.output + ".cas")
	require.NoError(t, err)
	require.Equal(t, client.dbCerts.CACerts[0], caBytes, "CA certificates match")
}
