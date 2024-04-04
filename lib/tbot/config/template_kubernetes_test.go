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
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils/golden"
)

// Fairly ugly hardcoded certs to use in the generation so that the tests are
// deterministic.
var (
	tlsCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDfDCCAmSgAwIBAgIQP/jI85sqjDWDTd9qF0V5qjANBgkqhkiG9w0BAQsFADBA
MRUwEwYDVQQKEwxUZWxlcG9ydCBPU1MxJzAlBgNVBAMTHnRlbGVwb3J0LmxvY2Fs
aG9zdC5sb2NhbGRvbWFpbjAeFw0yNDA0MDIxNDA5MjZaFw0yNDA0MDIxNTEwMjZa
MHMxGzAZBgNVBAkTEnRlbGUuYmxhY2ttZXNhLmdvdjENMAsGA1UEERMEbnVsbDER
MA8GA1UEAxMIYm90LXRlc3QxDjAMBgUrzg8BARMDZm9vMQ4wDAYFK84PAQITA2Jh
cjESMBAGBSvODwEDEwdleGFtcGxlMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIB
CgKCAQEAr8WfEDOq1TN0bT0SGEtEuDrRaf+VudmbypHokewy46md9XB3gQWbin9N
/5tyNdbFsWsDgDIyXP3Ube0ubcPYlcsCNtgCvK4qd3RyRvxY5lOfS1pZESPEtvO/
sxEu6E3O0ofcwq4uKenHuf1EUQuVD6WxABUOaOs2/3aahmYy4SnKNUsM2/l1XrcI
0ekvB0h10nXUC4VJS4sKGzGzThD308ia/bgDSXc0fiUwZPB5TLn7lScuisi+8JSs
qWccknXonGEEtism7FNi+mseV1ahzjEbRM/kfFwZ0H+ekz3CdnsmkND0FmxB9WTf
5PwG8oXM42QJkwuEIu+8Q/VVSSFe4wIDAQABoz8wPTAOBgNVHQ8BAf8EBAMCBaAw
HQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwDQYJ
KoZIhvcNAQELBQADggEBAEZYzIS0tx+Yn+cEfS83hpL1jELq9C8V3PTC6y44kjAK
i85Mx+ridYq0ddMdV/91JZ5t1Rqde4LSYUVUP6q/Ukih0mx/I3z2Siwp/YjpOu6x
SZXakjGnt5pCCa/t1NcVNSrqQpLQ8bJ0mruRiNawrKo3/ge57rgidnBFcOv3zV5t
BIkYjQJK2YVYcJjJ68olXzpCEw5hTZ9fw42fs2TORHHrPNUAzPf3Qdrzi03hrvhd
p6nYNA18b6ggygkcRU2MRte5E+/2ABYiwRklwJWNIMQJJHyZAW8Dyws0E2e+B3w+
7/SYPyeKOlt7foD7z0XM2Kw9ndurJq97AWfkcnrZwbQ=
-----END CERTIFICATE-----`)

	keyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAr8WfEDOq1TN0bT0SGEtEuDrRaf+VudmbypHokewy46md9XB3
gQWbin9N/5tyNdbFsWsDgDIyXP3Ube0ubcPYlcsCNtgCvK4qd3RyRvxY5lOfS1pZ
ESPEtvO/sxEu6E3O0ofcwq4uKenHuf1EUQuVD6WxABUOaOs2/3aahmYy4SnKNUsM
2/l1XrcI0ekvB0h10nXUC4VJS4sKGzGzThD308ia/bgDSXc0fiUwZPB5TLn7lScu
isi+8JSsqWccknXonGEEtism7FNi+mseV1ahzjEbRM/kfFwZ0H+ekz3CdnsmkND0
FmxB9WTf5PwG8oXM42QJkwuEIu+8Q/VVSSFe4wIDAQABAoIBACvb8vnW+pyibz3G
zFoVhfs2agS6CsFKJE6io9atinE2ZLzWqGsgXBRt+ad7QT9f7Qp9Om1lmR2NFNGt
KjWndcbC1jWbJuuvxdbyzoUZ+JDYctoZnDnjo/VG0yG6eurqZ14vGo3Vap14wSaO
pNpYOoSiAo2Ts3nIn3uVO6+nlrCKFBWiIMMvJ4L89vSCXyI/5kpwCURxpYeYJrQb
Jzi4TsN0ViYqf6XNfhRCSD+Fk9km2e6zsPIuWyfPrtGx1cA2UeAjnjKK9IS9qkAy
632T63X6M5kWtirIHM/r/IbdSj+lxGCMqnsSgNYILQv5sNkjgjwlkKl182w5l3CZ
TkjyLPkCgYEA0guhs3KBqM4ASDzQI3+h5GTGA87ITb0B/TcitElPQ+u3PLypwz9u
KzS9BHMpaWPWIOJzYRAjb8BDldyCcAhmyr5lv7O/ezRmgGD9NPV66IE5AX9nKVNS
PhJTNiYPSH7g4zO3K4sd5397YunRZzAxgsVWzu7E2gUrJSCsf0nL77UCgYEA1jpg
mJbSGYVrYEyQmt6YjHnwcLiNTJDPbcn27g0LmcRbq9SfIva/IfZaf0ru8b9c8QNM
WMag57WGQghS2B7698GvlwF+nKXXZDjCZ4z6+Efi/T5uHL5VDcpHE4yqdZG42hTW
m+K4wl4Z6B50xGC/mJlxzB6je4qBM9Zsn8wSwzcCgYEAk5O0kv4a9111eUuw+aAN
QQlEzxwUQ/pOUXjRm1X+qTwOTFBJ/nKslxLA00WOjQumQQiaBFJwc23kjoCV7N0a
S8ymdKB4IrpYYk7C2Ni4+G8CfHjlJHX0TMRXTq5DAq6Sl0+YnLFr22EIciDSDewg
fT7llRLRoFUNUVK5n91buhkCgYEAwqUEA2B1wQ6Cg1rNwIkjne9lUWW9rKWecqig
naZoteu9RyDG/qOnAhquGx5ggHJY5fsTMU44AI/kTrb1Xry3Vsk62z9WZMoiLEOO
Dzv/A/t8+I/yyFb/PKpfbhnO/0fJ5wwr+jNDoAaUD10sxwkIzIQO62GjNKqhvhHD
XGW1Xn0CgYEAzULAI9ij/q5S+GMyYj1xLCU4qxBpU/04nE+PfSSmfv8Ma34uh4QM
nRDcZHBqZYNRDt5zNvRTjgwJi4iHGwSB+D4SIYGb0ioTI2MOS2F7zBDyl8FXp7dT
3TSKronwoWYoLSisqnn/s8iN1M9RJA9pyIy7FTVwq59XL3NoetISuKc=
-----END RSA PRIVATE KEY-----`)
)

// TestTemplateKubernetesRender renders a Kubernetes template and compares it
// to the saved golden result.
func TestTemplateKubernetesRender(t *testing.T) {
	cfg, err := newTestConfig("example.com")
	require.NoError(t, err)
	k8sCluster := "example"
	mockBot := newMockProvider(cfg)

	// We need a fixed cert/key pair here for the golden files testing
	// to behave properly.
	id := &identity.Identity{
		PrivateKeyBytes: keyPEM,
		TLSCertBytes:    tlsCert,
		ClusterName:     mockClusterName,
	}

	tests := []struct {
		name              string
		useRelativePath   bool
		disableExecPlugin bool
	}{
		{
			name: "absolute path",
		},
		{
			name:            "relative path",
			useRelativePath: true,
		},
		{
			name:              "exec plugin disabled",
			disableExecPlugin: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			tmpl := templateKubernetes{
				clusterName:          k8sCluster,
				executablePathGetter: fakeGetExecutablePath,
				disableExecPlugin:    tt.disableExecPlugin,
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

			err = tmpl.render(context.Background(), mockBot, id, dest)
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
