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
	"crypto/tls"
	"fmt"
	"net"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
)

func TestForwarderClusterDialer(t *testing.T) {
	t.Parallel()
	var (
		hostname    = "localhost:8080"
		hostId      = "hostId"
		proxyIds    = []string{"proxyId"}
		clusterName = "cluster"
	)
	f := &Forwarder{
		cfg: ForwarderConfig{
			tracer:      otel.Tracer("test"),
			ClusterName: clusterName,
		},
		getKubernetesServersForKubeCluster: func(_ context.Context, kubeClusterName string) ([]types.KubeServer, error) {
			return []types.KubeServer{
				newKubeServerWithProxyIDs(t, hostname, hostId, proxyIds),
			}, nil
		},
	}
	tests := []struct {
		name          string
		dialerCreator func(kubeClusterName string) dialContextFunc
		want          reversetunnelclient.DialParams
	}{
		{
			name: "local site",
			dialerCreator: func(kubeClusterName string) dialContextFunc {
				return f.localClusterDialer(kubeClusterName)
			},
			want: reversetunnelclient.DialParams{
				From: &utils.NetAddr{
					Addr:        "0.0.0.0:0",
					AddrNetwork: "tcp",
				},
				To: &utils.NetAddr{
					Addr:        hostname,
					AddrNetwork: "tcp",
				},
				ServerID: fmt.Sprintf("%s.%s", hostId, clusterName),
				ConnType: types.KubeTunnel,
				ProxyIDs: proxyIds,
			},
		},
		{
			name:          "remote site",
			dialerCreator: f.remoteClusterDialer,
			want: reversetunnelclient.DialParams{
				From: &utils.NetAddr{
					Addr:        "0.0.0.0:0",
					AddrNetwork: "tcp",
				},
				To: &utils.NetAddr{
					Addr:        reversetunnelclient.LocalKubernetes,
					AddrNetwork: "tcp",
				},
				ConnType: types.KubeTunnel,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.cfg.ReverseTunnelSrv = &fakeReverseTunnel{
				t:    t,
				want: tt.want,
			}
			_, _ = tt.dialerCreator("")(context.Background(), "tcp", "")
		})
	}
}

type fakeReverseTunnel struct {
	reversetunnelclient.Server
	want reversetunnelclient.DialParams
	t    *testing.T
}

func (f *fakeReverseTunnel) GetSite(_ string) (reversetunnelclient.RemoteSite, error) {
	return &fakeRemoteSiteTunnel{
		want: f.want,
		t:    f.t,
	}, nil
}

type fakeRemoteSiteTunnel struct {
	reversetunnelclient.RemoteSite
	want reversetunnelclient.DialParams
	t    *testing.T
}

func (f *fakeRemoteSiteTunnel) DialTCP(p reversetunnelclient.DialParams) (net.Conn, error) {
	require.Equal(f.t, f.want, p)
	return nil, nil
}

func newKubeServerWithProxyIDs(t *testing.T, hostname, hostID string, proxyIds []string) types.KubeServer {
	k, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "cluster",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)

	ks, err := types.NewKubernetesServerV3FromCluster(k, hostname, hostID)
	require.NoError(t, err)
	ks.Spec.ProxyIDs = proxyIds
	return ks
}

func TestDirectTransportNotCached(t *testing.T) {
	t.Parallel()

	transportClients, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   transportCacheTTL,
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	forwarder := &Forwarder{
		ctx:             context.Background(),
		cachedTransport: transportClients,
	}

	kubeAPICreds := &dynamicKubeCreds{
		staticCreds: &staticKubeCreds{
			tlsConfig: &tls.Config{
				ServerName: "localhost",
			},
		},
	}

	clusterSess := &clusterSession{
		kubeAPICreds: kubeAPICreds,
		authContext: authContext{
			kubeClusterName: "b",
			teleportCluster: teleportClusterClient{
				name: "a",
			},
		},
	}

	_, tlsConfig, err := forwarder.transportForRequestWithImpersonation(clusterSess)
	require.NoError(t, err)
	require.Equal(t, "localhost", tlsConfig.ServerName)

	kubeAPICreds.staticCreds.tlsConfig.ServerName = "example.com"
	_, tlsConfig, err = forwarder.transportForRequestWithImpersonation(clusterSess)
	require.NoError(t, err)
	require.Equal(t, "example.com", tlsConfig.ServerName)
}
