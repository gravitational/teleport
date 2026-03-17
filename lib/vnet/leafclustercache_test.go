// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
)

// paginatingAuthClient is a mock implementation of authclient.ClientI that
// returns clusters in pages of a fixed size to simulate a large cluster.
type paginatingAuthClient struct {
	authclient.ClientI
	clusters []string
	pageSize int
}

func (m *paginatingAuthClient) ListRemoteClusters(ctx context.Context, pageSize int, pageToken string) ([]types.RemoteCluster, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if pageSize <= 0 {
		pageSize = m.pageSize
	}

	// Find the start index from the page token.
	start := 0
	if pageToken != "" {
		for i, name := range m.clusters {
			if name == pageToken {
				start = i
				break
			}
		}
	}

	end := start + pageSize
	if end >= len(m.clusters) {
		return makeRemoteClusters(m.clusters[start:]), "", nil
	}
	return makeRemoteClusters(m.clusters[start:end]), m.clusters[end], nil
}

func makeRemoteClusters(names []string) []types.RemoteCluster {
	result := make([]types.RemoteCluster, 0, len(names))
	for _, name := range names {
		rc, _ := types.NewRemoteCluster(name)
		result = append(result, rc)
	}
	return result
}

type testLeafClusterClient struct {
	authClient *paginatingAuthClient
}

func (c *testLeafClusterClient) CurrentCluster() authclient.ClientI { return c.authClient }
func (c *testLeafClusterClient) ClusterName() string                { return "root" }
func (c *testLeafClusterClient) RootClusterName() string            { return "root" }
func (c *testLeafClusterClient) SessionSSHKeyRing(ctx context.Context, user string, target client.NodeDetails) (*client.KeyRing, bool, error) {
	panic("not implemented")
}

func makeLeafClusterNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("leaf-%04d", i)
	}
	return names
}

func TestGetLeafClustersUncached(t *testing.T) {
	// Operations are in-memory and should complete in milliseconds. 5 seconds is generous enough to
	// catch an infinite loop without making the test suite slow.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name         string
		clusterCount int
		wantLen      int
	}{
		{
			name:         "empty cluster list",
			clusterCount: 0,
			wantLen:      0,
		},
		{
			name:         "under 1000 clusters fits in a single page",
			clusterCount: 500,
			wantLen:      500,
		},
		{
			name:         "exactly at page size boundary",
			clusterCount: 1000,
			wantLen:      1000,
		},
		{
			name:         "over 1000 clusters requires multiple pages",
			clusterCount: 1500,
			wantLen:      1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusters := makeLeafClusterNames(tt.clusterCount)
			rootClient := &testLeafClusterClient{
				authClient: &paginatingAuthClient{clusters: clusters, pageSize: 1000},
			}
			cache, err := newLeafClusterCache(clockwork.NewRealClock())
			require.NoError(t, err)

			got, err := cache.getLeafClustersUncached(ctx, rootClient)
			require.NoError(t, err)
			require.Len(t, got, tt.wantLen)
		})
	}
}
