/*
Copyright 2022 Gravitational, Inc.

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

package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/utils/golden"
)

// TestTemplateKubernetesRender renders a Kubernetes template and compares it
// to the saved golden result.
func TestTemplateKubernetesRender(t *testing.T) {
	cfg, err := newTestConfig("example.com")
	require.NoError(t, err)
	k8sCluster := "example"
	mockBot := newMockProvider(cfg)

	tests := []struct {
		name            string
		useRelativePath bool
	}{
		{
			name: "absolute path",
		},
		{
			name:            "relative path",
			useRelativePath: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			tmpl := templateKubernetes{
				clusterName:          k8sCluster,
				executablePathGetter: fakeGetExecutablePath,
			}
			dest := &DestinationDirectory{
				Path:     dir,
				Symlinks: botfs.SymlinksInsecure,
				ACLs:     botfs.ACLOff,
			}
			if tt.useRelativePath {
				wd, err := os.Getwd()
				require.NoError(t, err)
				relativePath, err := filepath.Rel(wd, dir)
				require.NoError(t, err)
				dest.Path = relativePath
			}

			ident := getTestIdent(t, "bot-test", kubernetesRequest(k8sCluster))

			err = tmpl.render(context.Background(), mockBot, ident, dest)
			require.NoError(t, err)

			kubeconfigBytes, err := os.ReadFile(filepath.Join(dir, defaultKubeconfigPath))
			require.NoError(t, err)
			kubeconfigBytes = bytes.ReplaceAll(kubeconfigBytes, []byte(dir), []byte("/test/dir"))

			if golden.ShouldSet() {
				golden.SetNamed(t, "kubeconfig.yaml", kubeconfigBytes)
			}
			require.Equal(
				t, string(golden.GetNamed(t, "kubeconfig.yaml")), string(kubeconfigBytes),
			)
		})
	}
}

func Test_selectKubeConnectionMethod(t *testing.T) {
	tests := []struct {
		name string

		proxyPing *webclient.PingResponse
		wantAddr  string
		wantSNI   string
	}{
		{
			// Copied from my real Teleport Cloud webapi/ping
			name: "TLS Routing",
			proxyPing: &webclient.PingResponse{
				Proxy: webclient.ProxySettings{
					Kube: webclient.KubeProxySettings{
						Enabled:    true,
						ListenAddr: "0.0.0.0:3080",
					},
					SSH: webclient.SSHProxySettings{
						ListenAddr:       "0.0.0.0:3080",
						TunnelListenAddr: "0.0.0.0:3080",
						WebListenAddr:    "0.0.0.0:3080",
						PublicAddr:       "noah.teleport.sh:443",
					},
					TLSRoutingEnabled: true,
				},
				ClusterName: "noah.teleport.sh",
			},
			wantAddr: "https://noah.teleport.sh:443",
			wantSNI:  "kube-teleport-proxy-alpn.noah.teleport.sh",
		},
		{
			name: "KubePublicAddr specified",
			proxyPing: &webclient.PingResponse{
				Proxy: webclient.ProxySettings{
					Kube: webclient.KubeProxySettings{
						Enabled:    true,
						ListenAddr: "0.0.0.0:1337",
						PublicAddr: "kube.example.com:1337",
					},
					SSH: webclient.SSHProxySettings{
						ListenAddr:       "0.0.0.0:3023",
						TunnelListenAddr: "0.0.0.0:3024",
						WebListenAddr:    "0.0.0.0:3080",
						PublicAddr:       "cluster.example.com:443",
						SSHPublicAddr:    "cluster.example.com:3023",
						TunnelPublicAddr: "cluster.example.com:3024",
					},
					TLSRoutingEnabled: false,
				},
				ClusterName: "cluster.example.com",
			},
			wantAddr: "https://kube.example.com:1337",
		},
		{
			// https://github.com/gravitational/teleport/issues/19811
			name: "Falls back to Kube ListenAddr Port with PublicAddr",
			proxyPing: &webclient.PingResponse{
				Proxy: webclient.ProxySettings{
					Kube: webclient.KubeProxySettings{
						Enabled:    true,
						ListenAddr: "0.0.0.0:3026",
					},
					SSH: webclient.SSHProxySettings{
						ListenAddr:       "[::]:3023",
						TunnelListenAddr: "0.0.0.0:3024",
						WebListenAddr:    "0.0.0.0:3080",
						PublicAddr:       "cluster.example.com:5443",
						SSHPublicAddr:    "cluster.example.com:3023",
						TunnelPublicAddr: "cluster.example.com:3024",
					},
					TLSRoutingEnabled: false,
				},
				ClusterName: "cluster.example.com",
			},
			wantAddr: "https://cluster.example.com:3026",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, sni, err := selectKubeConnectionMethod(tt.proxyPing)
			require.NoError(t, err)
			require.Equal(t, tt.wantAddr, addr)
			require.Equal(t, tt.wantSNI, sni)
		})
	}
}
