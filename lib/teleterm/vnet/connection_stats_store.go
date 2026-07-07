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
	"sync"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// connectionStatsStore keeps the latest snapshot of the aggregated per-target
// connection statistics reported by the VNet admin process. All counters in a
// snapshot are absolute values accumulated since VNet started, so each
// snapshot fully replaces the previous one.
//
// The store is scoped to a single VNet run: Reset activates a fresh session
// when VNet starts and CloseSession deactivates it when VNet stops. Snapshots
// reported while no session is active are dropped, because the reports come
// from the admin process and may still be in flight after VNet has stopped.
type connectionStatsStore struct {
	mu     sync.Mutex
	active bool
	// stats holds the latest snapshot. Stored messages are never mutated in
	// place; every report replaces the whole snapshot, so snapshots handed to
	// subscribers stay immutable.
	stats       []*api.ConnectionStat
	subscribers map[*statsSubscriber]struct{}
}

// statsSubscriber receives the latest snapshot whenever it changes. updates is
// a coalescing, latest-wins channel of capacity 1: a slow reader may miss
// intermediate snapshots but always converges to the current statistics, which
// is safe because every snapshot carries absolute values.
type statsSubscriber struct {
	updates chan []*api.ConnectionStat
}

func newConnectionStatsStore() *connectionStatsStore {
	return &connectionStatsStore{
		subscribers: make(map[*statsSubscriber]struct{}),
	}
}

// Reset clears the statistics and marks the store active for a new VNet run.
func (s *connectionStatsStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = true
	s.stats = nil
	s.notifyLocked()
}

// CloseSession clears the statistics, marks the store inactive, and completes
// all current subscriptions so their streaming handlers return. It is
// idempotent.
func (s *connectionStatsStore) CloseSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	s.stats = nil
	for sub := range s.subscribers {
		close(sub.updates)
	}
	s.subscribers = make(map[*statsSubscriber]struct{})
}

// SetStats replaces the current snapshot with a fresh one reported by the
// admin process.
func (s *connectionStatsStore) SetStats(stats []*api.ConnectionStat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return
	}
	s.stats = stats
	s.notifyLocked()
}

// Subscribe registers a subscriber and returns it along with the current
// snapshot. If the store is inactive (VNet is not running), the subscriber's
// channel is returned already closed.
func (s *connectionStatsStore) Subscribe() (*statsSubscriber, []*api.ConnectionStat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub := &statsSubscriber{updates: make(chan []*api.ConnectionStat, 1)}
	if !s.active {
		close(sub.updates)
		return sub, nil
	}
	s.subscribers[sub] = struct{}{}
	return sub, s.stats
}

// Unsubscribe removes a subscriber. It is safe to call even if the subscriber
// was already removed by CloseSession.
func (s *connectionStatsStore) Unsubscribe(sub *statsSubscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, sub)
}

// notifyLocked pushes the current snapshot to every subscriber without blocking.
func (s *connectionStatsStore) notifyLocked() {
	for sub := range s.subscribers {
		// Coalescing latest-wins send: drop any pending snapshot, then push the
		// newest, so a slow reader never blocks a report being recorded.
		select {
		case <-sub.updates:
		default:
		}
		select {
		case sub.updates <- s.stats:
		default:
		}
	}
}

// convertConnectionStats converts a snapshot reported by the VNet admin
// process to the teleterm API representation streamed to the Electron app.
func convertConnectionStats(stats []*vnetv1.ConnectionStat) []*api.ConnectionStat {
	out := make([]*api.ConnectionStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, api.ConnectionStat_builder{
			Kind:                  convertConnectionKind(stat.GetKind()),
			Cluster:               stat.GetProfile(),
			LeafCluster:           stat.GetLeafCluster(),
			DisplayName:           stat.GetDisplayName(),
			Port:                  stat.GetPort(),
			SuccessfulConnections: stat.GetSuccessfulConnections(),
			FailedConnections:     stat.GetFailedConnections(),
			BytesTx:               stat.GetBytesTx(),
			BytesRx:               stat.GetBytesRx(),
			BytesTxPerSec:         stat.GetBytesTxPerSec(),
			BytesRxPerSec:         stat.GetBytesRxPerSec(),
		}.Build())
	}
	return out
}

func convertConnectionKind(kind vnetv1.ConnectionKind) api.RecentConnectionKind {
	switch kind {
	case vnetv1.ConnectionKind_CONNECTION_KIND_APP:
		return api.RecentConnectionKind_RECENT_CONNECTION_KIND_APP
	case vnetv1.ConnectionKind_CONNECTION_KIND_SSH:
		return api.RecentConnectionKind_RECENT_CONNECTION_KIND_SSH
	case vnetv1.ConnectionKind_CONNECTION_KIND_DATABASE:
		return api.RecentConnectionKind_RECENT_CONNECTION_KIND_DATABASE
	default:
		return api.RecentConnectionKind_RECENT_CONNECTION_KIND_UNSPECIFIED
	}
}
