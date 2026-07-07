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

package vnet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func connectionStats(displayName string, successful uint64) []*api.ConnectionStat {
	return []*api.ConnectionStat{
		api.ConnectionStat_builder{
			Kind:                  api.RecentConnectionKind_RECENT_CONNECTION_KIND_APP,
			Cluster:               "root",
			DisplayName:           displayName,
			SuccessfulConnections: successful,
		}.Build(),
	}
}

func TestConnectionStatsStore_snapshotReplaces(t *testing.T) {
	store := newConnectionStatsStore()
	store.Reset()

	store.SetStats(connectionStats("a.example.com", 1))
	_, snap := store.Subscribe()
	require.Len(t, snap, 1)
	assert.Equal(t, uint64(1), snap[0].GetSuccessfulConnections())

	// A fresh snapshot fully replaces the previous one.
	store.SetStats(connectionStats("a.example.com", 2))
	_, snap = store.Subscribe()
	require.Len(t, snap, 1)
	assert.Equal(t, uint64(2), snap[0].GetSuccessfulConnections())
}

func TestConnectionStatsStore_subscribersGetUpdates(t *testing.T) {
	store := newConnectionStatsStore()
	store.Reset()

	sub, snap := store.Subscribe()
	defer store.Unsubscribe(sub)
	require.Empty(t, snap)

	store.SetStats(connectionStats("a.example.com", 1))
	// A second snapshot before the subscriber reads coalesces, latest wins.
	store.SetStats(connectionStats("a.example.com", 2))

	select {
	case snap := <-sub.updates:
		require.Len(t, snap, 1)
		assert.Equal(t, uint64(2), snap[0].GetSuccessfulConnections())
	default:
		t.Fatal("expected a pending snapshot")
	}
}

func TestConnectionStatsStore_sessionLifecycle(t *testing.T) {
	store := newConnectionStatsStore()

	// Snapshots reported while no session is active are dropped.
	store.SetStats(connectionStats("a.example.com", 1))
	sub, snap := store.Subscribe()
	assert.Empty(t, snap)
	// The subscriber's channel is returned already closed.
	_, ok := <-sub.updates
	assert.False(t, ok)

	store.Reset()
	store.SetStats(connectionStats("a.example.com", 1))
	sub, snap = store.Subscribe()
	require.Len(t, snap, 1)

	// CloseSession clears the statistics and completes subscriptions.
	store.CloseSession()
	_, ok = <-sub.updates
	assert.False(t, ok)
	_, snap = store.Subscribe()
	assert.Empty(t, snap)
}

func TestConvertConnectionStats(t *testing.T) {
	stats := convertConnectionStats([]*vnetv1.ConnectionStat{
		vnetv1.ConnectionStat_builder{
			Kind:                  vnetv1.ConnectionKind_CONNECTION_KIND_DATABASE,
			Profile:               "root",
			LeafCluster:           "leaf",
			DisplayName:           "db.root.example.com",
			Port:                  1234,
			SuccessfulConnections: 3,
			FailedConnections:     1,
			BytesTx:               100,
			BytesRx:               200,
			BytesTxPerSec:         10,
			BytesRxPerSec:         20,
		}.Build(),
	})
	require.Len(t, stats, 1)
	stat := stats[0]
	assert.Equal(t, api.RecentConnectionKind_RECENT_CONNECTION_KIND_DATABASE, stat.GetKind())
	assert.Equal(t, "root", stat.GetCluster())
	assert.Equal(t, "leaf", stat.GetLeafCluster())
	assert.Equal(t, "db.root.example.com", stat.GetDisplayName())
	assert.Equal(t, uint32(1234), stat.GetPort())
	assert.Equal(t, uint64(3), stat.GetSuccessfulConnections())
	assert.Equal(t, uint64(1), stat.GetFailedConnections())
	assert.Equal(t, uint64(100), stat.GetBytesTx())
	assert.Equal(t, uint64(200), stat.GetBytesRx())
	assert.Equal(t, uint64(10), stat.GetBytesTxPerSec())
	assert.Equal(t, uint64(20), stat.GetBytesRxPerSec())
}
