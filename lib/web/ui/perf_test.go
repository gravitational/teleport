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

package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

const clusterName = "bench.example.com"

func BenchmarkGetClusterDetails(b *testing.B) {
	ctx := context.Background()

	const authCount = 6
	const proxyCount = 6

	type testCase struct {
		memory bool
		nodes  int
	}

	var tts []testCase

	for _, memory := range []bool{true, false} {
		for _, nodes := range []int{100, 1000, 10000} {
			tts = append(tts, testCase{
				memory: memory,
				nodes:  nodes,
			})
		}
	}

	for _, tt := range tts {
		// create a descriptive name for the sub-benchmark.
		name := fmt.Sprintf("tt(memory=%v,nodes=%d)", tt.memory, tt.nodes)

		// run the sub benchmark
		b.Run(name, func(sb *testing.B) {
			// configure the backend instance
			var bk backend.Backend
			var err error
			if tt.memory {
				bk, err = memory.New(memory.Config{})
				require.NoError(b, err)
			} else {
				bk, err = memory.New(memory.Config{
					Context: ctx,
				})
				require.NoError(b, err)
			}
			defer bk.Close()

			svc := local.NewPresenceService(bk)

			// seed the test nodes
			insertServers(ctx, b, svc, types.KindNode, tt.nodes)
			insertServers(ctx, b, svc, types.KindProxy, proxyCount)
			insertServers(ctx, b, svc, types.KindAuthServer, authCount)

			site := &mockRemoteSite{
				accessPoint: &mockAccessPoint{
					presence: svc,
				},
			}
			benchmarkGetClusterDetails(ctx, sb, site, tt.nodes)
		})
	}
}

// insertServers inserts a collection of servers into a backend.
func insertServers(ctx context.Context, b *testing.B, svc services.Presence, kind string, count int) {
	const labelCount = 10
	labels := make(map[string]string, labelCount)
	for i := range labelCount {
		labels[fmt.Sprintf("label-key-%d", i)] = fmt.Sprintf("label-val-%d", i)
	}
	for range count {
		name := uuid.New().String()
		addr := fmt.Sprintf("%s.%s", name, clusterName)
		server := &types.ServerV2{
			Kind:    kind,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
				Labels:    labels,
			},
			Spec: types.ServerSpecV2{
				Addr:    addr,
				Version: teleport.Version,
			},
		}
		var err error
		switch kind {
		case types.KindNode:
			_, err = svc.UpsertNode(ctx, server)
		case types.KindProxy:
			err = svc.UpsertProxy(ctx, server)
		case types.KindAuthServer:
			err = svc.UpsertAuthServer(ctx, server)
		default:
			b.Errorf("Unexpected server kind: %s", kind)
		}
		require.NoError(b, err)
	}
}

func benchmarkGetClusterDetails(ctx context.Context, b *testing.B, site reversetunnelclient.RemoteSite, nodes int, opts ...services.MarshalOption) {
	var cluster *Cluster
	var err error
	for b.Loop() {
		cluster, err = GetClusterDetails(ctx, site, opts...)
		require.NoError(b, err)
	}
	require.NotNil(b, cluster)
}

type mockRemoteSite struct {
	reversetunnelclient.RemoteSite
	accessPoint authclient.ProxyAccessPoint
}

func (m *mockRemoteSite) CachingAccessPoint() (authclient.RemoteProxyAccessPoint, error) {
	return m.accessPoint, nil
}

func (m *mockRemoteSite) GetName() string {
	return clusterName
}

func (m *mockRemoteSite) GetLastConnected() time.Time {
	return time.Now()
}

func (m *mockRemoteSite) GetStatus() string {
	return teleport.RemoteClusterStatusOnline
}

type mockAccessPoint struct {
	authclient.ProxyAccessPoint
	presence *local.PresenceService
}

func (m *mockAccessPoint) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	return m.presence.GetNodes(ctx, namespace)
}

func (m *mockAccessPoint) GetProxies() ([]types.Server, error) {
	return m.presence.GetProxies()
}

func (m *mockAccessPoint) GetAuthServers() ([]types.Server, error) {
	return m.presence.GetAuthServers()
}
