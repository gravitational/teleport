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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
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
		total       = 1000
	)

	b.Run(fmt.Sprintf("total=%d", total), func(sb *testing.B) {
		sb.ReportAllocs()

		servers := createBenchmarkDatabaseServers(sb, total, targetName)

		backend, err := memory.New(memory.Config{Context: sb.Context()})
		require.NoError(sb, err)

		presenceService := local.NewPresenceService(backend)
		for _, server := range servers {
			_, err = presenceService.UpsertDatabaseServer(sb.Context(), server)
			require.NoError(sb, err)
		}

		watcher, err := services.NewDatabaseServerWatcher(sb.Context(), services.DatabaseServerWatcherConfig{
			DatabaseServersGetter: presenceService,
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component:      "bench",
				MaxRetryPeriod: 200 * time.Millisecond,
				Client:         local.NewEventsService(backend),
			},
		})
		require.NoError(sb, err)
		sb.Cleanup(watcher.Close)

		require.NoError(sb, watcher.WaitInitialization())

		params := GetDatabaseServersParams{
			Logger:      slog.New(slog.DiscardHandler),
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
