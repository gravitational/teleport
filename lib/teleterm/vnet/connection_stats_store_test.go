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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func testConnectionsReport(displayName string, successful uint64) connectionsReport {
	return connectionsReport{
		stats: []*api.ConnectionStat{
			api.ConnectionStat_builder{
				Kind:                  api.RecentConnectionKind_RECENT_CONNECTION_KIND_APP,
				Cluster:               "root",
				DisplayName:           displayName,
				SuccessfulConnections: successful,
			}.Build(),
		},
		connections: []*api.ConnectionRecord{
			api.ConnectionRecord_builder{
				Id:          successful,
				Kind:        api.RecentConnectionKind_RECENT_CONNECTION_KIND_APP,
				Cluster:     "root",
				DisplayName: displayName,
				State:       api.ConnectionRecordState_CONNECTION_RECORD_STATE_DONE,
			}.Build(),
		},
	}
}

func TestConnectionStatsStore_snapshotReplaces(t *testing.T) {
	store := newConnectionStatsStore()
	store.Reset()

	store.SetReport(testConnectionsReport("a.example.com", 1))
	_, snap := store.Subscribe()
	require.Len(t, snap.stats, 1)
	require.Len(t, snap.connections, 1)
	assert.Equal(t, uint64(1), snap.stats[0].GetSuccessfulConnections())

	// A fresh snapshot fully replaces the previous one.
	store.SetReport(testConnectionsReport("a.example.com", 2))
	_, snap = store.Subscribe()
	require.Len(t, snap.stats, 1)
	assert.Equal(t, uint64(2), snap.stats[0].GetSuccessfulConnections())
	require.Len(t, snap.connections, 1)
	assert.Equal(t, uint64(2), snap.connections[0].GetId())
}

func TestConnectionStatsStore_subscribersGetUpdates(t *testing.T) {
	store := newConnectionStatsStore()
	store.Reset()

	sub, snap := store.Subscribe()
	defer store.Unsubscribe(sub)
	require.Empty(t, snap.stats)
	require.Empty(t, snap.connections)

	store.SetReport(testConnectionsReport("a.example.com", 1))
	// A second snapshot before the subscriber reads coalesces, latest wins.
	store.SetReport(testConnectionsReport("a.example.com", 2))

	select {
	case snap := <-sub.updates:
		require.Len(t, snap.stats, 1)
		assert.Equal(t, uint64(2), snap.stats[0].GetSuccessfulConnections())
		require.Len(t, snap.connections, 1)
		assert.Equal(t, uint64(2), snap.connections[0].GetId())
	default:
		t.Fatal("expected a pending snapshot")
	}
}

func TestConnectionStatsStore_sessionLifecycle(t *testing.T) {
	store := newConnectionStatsStore()

	// Snapshots reported while no session is active are dropped.
	store.SetReport(testConnectionsReport("a.example.com", 1))
	sub, snap := store.Subscribe()
	assert.Empty(t, snap.stats)
	// The subscriber's channel is returned already closed.
	_, ok := <-sub.updates
	assert.False(t, ok)

	store.Reset()
	store.SetReport(testConnectionsReport("a.example.com", 1))
	sub, snap = store.Subscribe()
	require.Len(t, snap.stats, 1)
	require.Len(t, snap.connections, 1)

	// CloseSession clears the snapshot and completes subscriptions.
	store.CloseSession()
	_, ok = <-sub.updates
	assert.False(t, ok)
	_, snap = store.Subscribe()
	assert.Empty(t, snap.stats)
	assert.Empty(t, snap.connections)
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

func TestConvertConnectionRecords(t *testing.T) {
	startedAt := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(time.Minute)

	records := convertConnectionRecords([]*vnetv1.ConnectionRecord{
		vnetv1.ConnectionRecord_builder{
			Id:                42,
			Kind:              vnetv1.ConnectionKind_CONNECTION_KIND_APP,
			Profile:           "root",
			LeafCluster:       "leaf",
			DisplayName:       "app.root.example.com",
			Port:              8443,
			LocalPort:         8443,
			ClientProcessPath: "/usr/bin/curl",
			StartedAt:         timestamppb.New(startedAt),
			EndedAt:           timestamppb.New(endedAt),
			BytesTx:           100,
			BytesRx:           200,
			State:             vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_DONE,
			ErrorMessage:      "conn reset",
		}.Build(),
		// An active connection has no end time yet.
		vnetv1.ConnectionRecord_builder{
			Id:        43,
			Kind:      vnetv1.ConnectionKind_CONNECTION_KIND_SSH,
			StartedAt: timestamppb.New(startedAt),
			State:     vnetv1.ConnectionRecordState_CONNECTION_RECORD_STATE_ACTIVE,
		}.Build(),
	})
	require.Len(t, records, 2)

	rec := records[0]
	assert.Equal(t, uint64(42), rec.GetId())
	assert.Equal(t, api.RecentConnectionKind_RECENT_CONNECTION_KIND_APP, rec.GetKind())
	assert.Equal(t, "root", rec.GetCluster())
	assert.Equal(t, "leaf", rec.GetLeafCluster())
	assert.Equal(t, "app.root.example.com", rec.GetDisplayName())
	assert.Equal(t, uint32(8443), rec.GetPort())
	assert.Equal(t, uint32(8443), rec.GetLocalPort())
	assert.Equal(t, "/usr/bin/curl", rec.GetClientProcessPath())
	assert.Equal(t, startedAt, rec.GetStartedAt().AsTime())
	assert.Equal(t, endedAt, rec.GetEndedAt().AsTime())
	assert.Equal(t, uint64(100), rec.GetBytesTx())
	assert.Equal(t, uint64(200), rec.GetBytesRx())
	assert.Equal(t, api.ConnectionRecordState_CONNECTION_RECORD_STATE_DONE, rec.GetState())
	assert.Equal(t, "conn reset", rec.GetErrorMessage())

	active := records[1]
	assert.Equal(t, api.ConnectionRecordState_CONNECTION_RECORD_STATE_ACTIVE, active.GetState())
	assert.Nil(t, active.GetEndedAt())
	assert.Empty(t, active.GetErrorMessage())
}
