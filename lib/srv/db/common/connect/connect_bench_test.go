// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package connect

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tlsca"
)

func createBenchmarkDatabaseServers(b *testing.B, total int, targetName string) []types.DatabaseServer {
	b.Helper()

	servers := make([]types.DatabaseServer, 0, total)
	for i := range total {
		dbName := fmt.Sprintf("db-%d", i)
		if i == total-1 {
			dbName = targetName
		}

		server, err := types.NewDatabaseServerV3(types.Metadata{
			Name: fmt.Sprintf("db-server-%d", i),
		}, types.DatabaseServerSpecV3{
			Hostname: "localhost",
			HostID:   fmt.Sprintf("host-%d", i),
			Database: &types.DatabaseV3{
				Metadata: types.Metadata{
					Name: dbName,
				},
				Spec: types.DatabaseSpecV3{
					Protocol: types.DatabaseProtocolPostgreSQL,
					URI:      "localhost",
				},
			},
		})
		require.NoError(b, err)
		servers = append(servers, server)
	}

	return servers
}

// BenchmarkConnectGetDatabaseServers measures the memory usage of GetDatabaseServers
// with varying numbers of database servers in the cluster.
func BenchmarkConnectGetDatabaseServers(b *testing.B) {
	const (
		matchCount  = 1
		clusterName = "cluster-1"
		targetName  = "db-target"
	)

	totals := []int{
		1000,
		5000,
		10000,
	}

	for _, total := range totals {
		b.Run(fmt.Sprintf("total=%d", total), func(sb *testing.B) {
			sb.ReportAllocs()

			servers := createBenchmarkDatabaseServers(sb, total, targetName)

			authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
				ClusterName: clusterName,
				Dir:         sb.TempDir(),
			})
			require.NoError(sb, err)
			sb.Cleanup(func() { require.NoError(sb, authServer.Close()) })

			for _, server := range servers {
				_, err := authServer.AuthServer.UpsertDatabaseServer(b.Context(), server)
				require.NoError(sb, err)
			}

			cluster := reversetunnelclient.NewFakeCluster(clusterName, authServer.AuthServer)
			sb.Cleanup(func() { require.NoError(sb, cluster.Close()) })

			watcher, err := cluster.DatabaseServerWatcher()
			require.NoError(sb, err)
			require.NoError(sb, watcher.WaitInitialization())

			params := GetDatabaseServersParams{
				Logger:      slog.Default(),
				ClusterName: clusterName,
				Watcher:     watcher,
				Identity: tlsca.Identity{
					RouteToDatabase: tlsca.RouteToDatabase{ServiceName: targetName},
					RouteToCluster:  clusterName,
				},
			}

			for sb.Loop() {
				result, err := GetDatabaseServers(b.Context(), params)
				require.NoError(sb, err)
				require.Len(sb, result, matchCount)
			}
		})
	}
}
