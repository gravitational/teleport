/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package healthcheck

import (
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestOrderByHealthEmpty(t *testing.T) {
	t.Parallel()
	var servers []types.KubeServer
	var visited []string
	for server := range OrderByTargetHealthStatus(servers) {
		visited = append(visited, server.GetHostID())
	}
	require.Empty(t, visited)
}

func TestOrderByHealthOne(t *testing.T) {
	t.Parallel()
	servers := []types.KubeServer{
		newKubeServer(t, "one", types.TargetHealthStatusHealthy),
	}
	var visited []string
	for server := range OrderByTargetHealthStatus(servers) {
		visited = append(visited, server.GetHostID())
	}
	require.Equal(t, []string{"one"}, visited)
}

func TestOrderByHealthUnsorted(t *testing.T) {
	t.Parallel()
	servers := []types.KubeServer{
		newKubeServer(t, "unknown-2", types.TargetHealthStatusUnknown),
		newKubeServer(t, "unknown-1", types.TargetHealthStatusUnknown),
		newKubeServer(t, "unhealthy-1", types.TargetHealthStatusUnhealthy),
		newKubeServer(t, "healthy-2", types.TargetHealthStatusHealthy),
		newKubeServer(t, "unhealthy-2", types.TargetHealthStatusUnhealthy),
		newKubeServer(t, "healthy-1", types.TargetHealthStatusHealthy),
	}
	var visited []types.KubeServer
	for server := range OrderByTargetHealthStatus(servers) {
		visited = append(visited, server)
	}
	require.Len(t, visited, len(servers))
	require.True(t, slices.IsSortedFunc(visited, byHealthOrder))
}

func TestOrderByHealthEarlyExit(t *testing.T) {
	t.Parallel()
	servers := []types.KubeServer{
		newKubeServer(t, "unknown-1", types.TargetHealthStatusUnknown),
		newKubeServer(t, "unhealthy-1", types.TargetHealthStatusUnhealthy),
		newKubeServer(t, "healthy-2", types.TargetHealthStatusHealthy),
		newKubeServer(t, "healthy-1", types.TargetHealthStatusHealthy),
	}
	var visited []string
	for server := range OrderByTargetHealthStatus(servers) {
		visited = append(visited, server.GetHostID())
		if len(visited) >= 2 {
			break
		}
	}
	require.Len(t, visited, 2)
	require.NotContains(t, visited, "unknown-1")
	require.NotContains(t, visited, "unhealthy-1")
}

func newKubeServer(t *testing.T, hostID string, health types.TargetHealthStatus) types.KubeServer {
	t.Helper()
	cluster, err := types.NewKubernetesClusterV3(
		types.Metadata{Name: "test-cluster"},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)
	server, err := types.NewKubernetesServerV3FromCluster(cluster, "localhost:8080", hostID)
	require.NoError(t, err)
	server.Status = &types.KubernetesServerStatusV3{
		TargetHealth: &types.TargetHealth{
			Status: string(health),
		},
	}
	return server
}

func healthOrder(s types.KubeServer) int {
	switch s.GetTargetHealthStatus() {
	case types.TargetHealthStatusHealthy:
		return 0
	case types.TargetHealthStatusUnknown:
		return 1
	case types.TargetHealthStatusUnhealthy:
		return 2
	}
	return math.MaxInt
}

func byHealthOrder(a, b types.KubeServer) int {
	return healthOrder(a) - healthOrder(b)
}
