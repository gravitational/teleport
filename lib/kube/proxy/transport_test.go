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
	"errors"
	"math"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/gravitational/trace"
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
		health      = types.TargetHealthStatusHealthy
	)
	f := &Forwarder{
		cfg: ForwarderConfig{
			tracer:      otel.Tracer("test"),
			ClusterName: clusterName,
		},
		getKubernetesServersForKubeCluster: func(_ context.Context, kubeClusterName string) ([]types.KubeServer, error) {
			return []types.KubeServer{
				newKubeServer(t, hostname, hostId, proxyIds, health),
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
				ServerID: hostId + "." + clusterName,
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

func (f *fakeReverseTunnel) Cluster(context.Context, string) (reversetunnelclient.Cluster, error) {
	return &fakeRemoteSiteTunnel{
		want: f.want,
		t:    f.t,
	}, nil
}

type fakeRemoteSiteTunnel struct {
	reversetunnelclient.Cluster
	want reversetunnelclient.DialParams
	t    *testing.T
}

func (f *fakeRemoteSiteTunnel) DialTCP(p reversetunnelclient.DialParams) (net.Conn, error) {
	require.Equal(f.t, f.want, p)
	return nil, nil
}

func newKubeServer(t *testing.T, hostname, hostID string, proxyIds []string, health types.TargetHealthStatus) types.KubeServer {
	k, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "cluster",
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)

	ks, err := types.NewKubernetesServerV3FromCluster(k, hostname, hostID)
	require.NoError(t, err)
	ks.Spec.ProxyIDs = proxyIds
	ks.Status = &types.KubernetesServerStatusV3{
		TargetHealth: &types.TargetHealth{
			Status: string(health),
		},
	}
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

func TestLocalClusterDialsByHealth(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	const (
		hostname    = "localhost:8080"
		clusterName = "cluster"
	)
	proxyIds := []string{"proxyId"}
	tests := []struct {
		name    string
		servers []types.KubeServer
	}{
		{
			name: "one",
			servers: []types.KubeServer{
				newKubeServer(t, hostname, "healthy-1", proxyIds, types.TargetHealthStatusHealthy),
			},
		},
		{
			name: "healthy",
			servers: []types.KubeServer{
				newKubeServer(t, hostname, "healthy-1", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "healthy-2", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "healthy-3", proxyIds, types.TargetHealthStatusHealthy),
			},
		},
		{
			name: "unknown",
			servers: []types.KubeServer{
				newKubeServer(t, hostname, "unknown-1", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unknown-2", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unknown-3", proxyIds, types.TargetHealthStatusUnknown),
			},
		},
		{
			name: "unhealthy",
			servers: []types.KubeServer{
				newKubeServer(t, hostname, "unhealthy-1", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unhealthy-2", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unhealthy-3", proxyIds, types.TargetHealthStatusUnhealthy),
			},
		},
		{
			name: "random",
			servers: []types.KubeServer{
				newKubeServer(t, hostname, "unhealthy-1", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "healthy-3", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "unknown-2", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "healthy-2", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "unknown-1", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unhealthy-3", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unknown-3", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unhealthy-2", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "healthy-1", proxyIds, types.TargetHealthStatusHealthy),
			},
		},
		{
			name: "reversed",
			servers: []types.KubeServer{
				newKubeServer(t, hostname, "unhealthy-3", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unhealthy-2", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unhealthy-1", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unknown-3", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unknown-2", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unknown-1", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "healthy-3", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "healthy-2", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "healthy-1", proxyIds, types.TargetHealthStatusHealthy),
			},
		},
		{
			name: "sorted",
			servers: []types.KubeServer{
				newKubeServer(t, hostname, "healthy-1", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "healthy-2", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "healthy-3", proxyIds, types.TargetHealthStatusHealthy),
				newKubeServer(t, hostname, "unknown-1", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unknown-2", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unknown-3", proxyIds, types.TargetHealthStatusUnknown),
				newKubeServer(t, hostname, "unhealthy-1", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unhealthy-2", proxyIds, types.TargetHealthStatusUnhealthy),
				newKubeServer(t, hostname, "unhealthy-3", proxyIds, types.TargetHealthStatusUnhealthy),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Forwarder{
				cfg: ForwarderConfig{
					tracer:           otel.Tracer("test"),
					ClusterName:      clusterName,
					ReverseTunnelSrv: healthReverseTunnel{},
				},
				getKubernetesServersForKubeCluster: func(context.Context, string) ([]types.KubeServer, error) {
					return tt.servers, nil
				},
			}
			_, err := f.localClusterDialer(clusterName)(ctx, "tcp", "")
			require.Error(t, err)

			var aggErr trace.Aggregate
			require.ErrorAs(t, err, &aggErr, "expected an aggregate error")
			var healthErrs []healthError
			for _, e := range aggErr.Errors() {
				var he healthError
				if errors.As(e, &he) {
					healthErrs = append(healthErrs, he)
				}
			}

			require.Len(t, healthErrs, len(tt.servers))
			require.True(t, slices.IsSortedFunc(healthErrs, byHealthOrder),
				"expected dialed errors to be order by healthy, unknown, and unhealthy")
		})
	}
}

type healthReverseTunnel struct {
	reversetunnelclient.Server
}

func (f healthReverseTunnel) Cluster(context.Context, string) (reversetunnelclient.Cluster, error) {
	return &healthRemoteSiteTunnel{}, nil
}

type healthRemoteSiteTunnel struct {
	reversetunnelclient.Cluster
}

func (f healthRemoteSiteTunnel) DialTCP(p reversetunnelclient.DialParams) (net.Conn, error) {
	// Extract health from ServerID.
	// ServerID = <health>-<n>.<clusterName>
	idx := strings.Index(p.ServerID, "-")
	if idx < 0 {
		return nil, trace.BadParameter("invalid server ID: %q", p.ServerID)
	}
	return nil, healthError{health: types.TargetHealthStatus(p.ServerID[:idx])}
}

type healthError struct {
	health types.TargetHealthStatus
}

func (e healthError) Error() string {
	return string(e.health)
}

func healthOrder(e healthError) int {
	switch e.health {
	case types.TargetHealthStatusHealthy:
		return 0
	case types.TargetHealthStatusUnknown:
		return 1
	case types.TargetHealthStatusUnhealthy:
		return 2
	}
	return math.MaxInt
}

func byHealthOrder(a, b healthError) int {
	return healthOrder(a) - healthOrder(b)
}
