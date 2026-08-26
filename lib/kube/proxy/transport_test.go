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
	"fmt"
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
		hostname          = "localhost:8080"
		hostId            = "hostId"
		proxyIds          = []string{"proxyId"}
		clusterName       = "cluster"
		health            = types.TargetHealthStatusHealthy
		scopedClusterName = "scoped-cluster"
		testScope         = "/test"
		otherHostname     = "localhost:8081"
		otherHostID       = "other-hostId"
		otherScope        = "/other"
	)
	f := &Forwarder{
		cfg: ForwarderConfig{
			tracer:      otel.Tracer("test"),
			ClusterName: clusterName,
		},
		getKubernetesServersForKubeCluster: func(_ context.Context, kubeClusterName string) ([]types.KubeServer, error) {
			switch kubeClusterName {
			case clusterName:
				return []types.KubeServer{
					newKubeServer(t, hostname, hostId, proxyIds, health),
				}, nil
			case scopedClusterName:
				return []types.KubeServer{
					newScopedKubeServer(t, scopedClusterName, hostname, hostId, testScope, proxyIds, health),
					newScopedKubeServer(t, scopedClusterName, otherHostname, otherHostID, otherScope, proxyIds, health),
				}, nil
			}
			return nil, trace.NotFound("cluster %s is not found", kubeClusterName)
		},
	}
	tests := []struct {
		name       string
		dialerFunc dialContextFunc
		want       reversetunnelclient.DialParams
		assertErr  require.ErrorAssertionFunc
	}{
		{
			name:       "local site",
			dialerFunc: f.localClusterDialer(clusterName, ""),
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
			name:       "remote site",
			dialerFunc: f.remoteClusterDialer(clusterName),
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
		{
			name: "local site - test scope",
			// It's important that the test scope and other scope cases resolve to the same cluster name.
			// This ensures that the dialer checks for both name and scope match before selecting a kube server.
			dialerFunc: f.localClusterDialer(scopedClusterName, testScope),
			want: reversetunnelclient.DialParams{
				From: &utils.NetAddr{
					Addr:        "0.0.0.0:0",
					AddrNetwork: "tcp",
				},
				To: &utils.NetAddr{
					Addr:        hostname,
					AddrNetwork: "tcp",
				},
				ServerID:    hostId + "." + clusterName,
				ConnType:    types.KubeTunnel,
				ProxyIDs:    proxyIds,
				TargetScope: testScope,
			},
		},
		{
			name: "local site - other scope",
			// It's important that the test scope and other scope cases resolve to the same cluster name.
			// This ensures that the dialer checks for both name and scope match before selecting a kube server.
			dialerFunc: f.localClusterDialer(scopedClusterName, otherScope),
			want: reversetunnelclient.DialParams{
				From: &utils.NetAddr{
					Addr:        "0.0.0.0:0",
					AddrNetwork: "tcp",
				},
				To: &utils.NetAddr{
					Addr:        otherHostname,
					AddrNetwork: "tcp",
				},
				ServerID:    otherHostID + "." + clusterName,
				ConnType:    types.KubeTunnel,
				ProxyIDs:    proxyIds,
				TargetScope: otherScope,
			},
		},
		{
			name:       "local site - unknown scope",
			dialerFunc: f.localClusterDialer(scopedClusterName, "/unknown"),
			assertErr: func(t require.TestingT, err error, msgAndArgs ...any) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err), "expected not found error")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.cfg.ReverseTunnelSrv = &fakeReverseTunnel{
				t:    t,
				want: tt.want,
			}
			_, err := tt.dialerFunc(t.Context(), "tcp", "")
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}
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
	return newScopedKubeServer(t, "cluster", hostname, hostID, "", proxyIds, health)
}

func newScopedKubeServer(t *testing.T, clusterName, hostname, hostID, scope string, proxyIds []string, health types.TargetHealthStatus) types.KubeServer {
	k, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: clusterName,
	}, types.KubernetesClusterSpecV3{}, types.KubeClusterWithScope(scope))
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

// TestsTransportCache tests whether or not we generate transport cache keys
// properly when the cluster names are identical.
func TestTransportCache(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	transportClients, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   transportCacheTTL,
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	var (
		proxyIds    = []string{"proxyId"}
		clusterName = "cluster"
		health      = types.TargetHealthStatusHealthy
		testScope   = "/test"
		otherScope  = "/other"
	)
	// It's critical that all three servers share the same cluster name so
	// we properly exercise cache key generation.
	unscopedServer := newKubeServer(t, "localhost:8080", "hostId", proxyIds, health)
	testServer := newScopedKubeServer(t, clusterName, "localhost:8081", "test-hostid", testScope, proxyIds, health)
	otherServer := newScopedKubeServer(t, clusterName, "localhost:8082", "other-hostid", otherScope, proxyIds, health)
	forwarder := &Forwarder{
		cfg: ForwarderConfig{
			ReverseTunnelSrv: healthReverseTunnel{},
		},
		ctx:             ctx,
		cachedTransport: transportClients,
		getKubernetesServersForKubeCluster: func(_ context.Context, _ string) ([]types.KubeServer, error) {
			return []types.KubeServer{
				unscopedServer,
				testServer,
				otherServer,
			}, nil
		},
	}

	clusterSess := &clusterSession{
		authContext: authContext{
			kubeClusterName: clusterName,
			kubeCluster:     unscopedServer.GetCluster(),
			teleportCluster: teleportClusterClient{
				name: "a",
			},
		},
	}

	// generate transport for the unscoped server
	_, _, err = forwarder.transportForRequestWithImpersonation(clusterSess)
	require.NoError(t, err)

	// replace the cluster on the clusterSession and ensure we generate a new
	// transport rather than reusing the cached, unscoped transport
	clusterSess.kubeCluster = testServer.GetCluster()
	_, _, err = forwarder.transportForRequestWithImpersonation(clusterSess)
	require.NoError(t, err)

	// replace again with a different scope and confirm we get another new
	// transport
	clusterSess.kubeCluster = otherServer.GetCluster()
	_, _, err = forwarder.transportForRequestWithImpersonation(clusterSess)
	require.NoError(t, err)

	// we should have cached transports for all three clusters
	unscopedTransport, ok := forwarder.cachedTransport.GetIfExists(fmt.Sprintf("%x/%x", "a", clusterName))
	require.True(t, ok, "expected transport to be cached")

	testTransport, ok := forwarder.cachedTransport.GetIfExists(fmt.Sprintf("%x/%x/%x", "a", testScope, clusterName))
	require.True(t, ok, "expected transport to be cached")

	otherTransport, ok := forwarder.cachedTransport.GetIfExists(fmt.Sprintf("%x/%x/%x", "a", otherScope, clusterName))
	require.True(t, ok, "expected transport to be cached")

	// all of the cached transports should be unique because none of these clusters should collide
	require.NotEqual(t, unscopedTransport, testTransport)
	require.NotEqual(t, unscopedTransport, otherTransport)
	require.NotEqual(t, testTransport, otherTransport)
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
			_, err := f.localClusterDialer(clusterName, "")(ctx, "tcp", "")
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
