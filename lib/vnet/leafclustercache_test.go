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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/client"
	localservice "github.com/gravitational/teleport/lib/services/local"
)

// trustAuthClient wraps the real local CA trust service to satisfy authclient.ClientI,
// delegating only ListRemoteClusters to the real implementation.
type trustAuthClient struct {
	authclient.ClientI
	trust *localservice.CA
}

func (c *trustAuthClient) ListRemoteClusters(ctx context.Context, pageSize int, pageToken string) ([]types.RemoteCluster, string, error) {
	return c.trust.ListRemoteClusters(ctx, pageSize, pageToken)
}

type testLeafClusterClient struct {
	authClient authclient.ClientI
}

func (c *testLeafClusterClient) CurrentCluster() authclient.ClientI { return c.authClient }
func (c *testLeafClusterClient) ClusterName() string                { return "root" }
func (c *testLeafClusterClient) RootClusterName() string            { return "root" }
func (c *testLeafClusterClient) SessionSSHKeyRing(ctx context.Context, user string, target client.NodeDetails) (*client.KeyRing, bool, error) {
	panic("not implemented")
}

func TestGetLeafClustersUncached(t *testing.T) {
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
			ctx := t.Context()
			mem, err := memory.New(memory.Config{
				Context: ctx,
			})
			require.NoError(t, err)
			defer mem.Close()

			trustSvc := localservice.NewCAService(mem)
			for i := range tt.clusterCount {
				rc, err := types.NewRemoteCluster(fmt.Sprintf("leaf-%04d", i))
				require.NoError(t, err)
				_, err = trustSvc.CreateRemoteCluster(ctx, rc)
				require.NoError(t, err)
			}

			rootClient := &testLeafClusterClient{
				authClient: &trustAuthClient{trust: trustSvc},
			}
			cache, err := newLeafClusterCache(clockwork.NewRealClock())
			require.NoError(t, err)

			got, err := cache.getLeafClustersUncached(ctx, rootClient)
			require.NoError(t, err)
			require.Len(t, got, tt.wantLen)

			seen := make(map[string]struct{}, len(got))
			for _, name := range got {
				seen[name] = struct{}{}
			}
			require.Len(t, seen, tt.wantLen, "expected all cluster names to be distinct")
		})
	}
}
