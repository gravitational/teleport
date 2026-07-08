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
	"slices"
	"strings"
	"sync"

	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// recentConnectionsStore keeps an in-memory, deduplicated list of the targets
// recently connected to through VNet. It holds one entry per target and orders
// them most-recently-connected first.
//
// The store is scoped to a single VNet run: Reset activates a fresh session when
// VNet starts and CloseSession deactivates it when VNet stops. Connections
// recorded while no session is active are dropped, because the connection
// callbacks run in detached goroutines that may fire after VNet has stopped.
type recentConnectionsStore struct {
	clock clockwork.Clock

	mu     sync.Mutex
	active bool
	// entries holds the deduplicated connections keyed by target identity. Stored
	// messages are never mutated in place; a repeat connection replaces the entry
	// with a new message, so snapshots handed to subscribers stay immutable.
	entries     map[connectionKey]*api.RecentConnection
	subscribers map[*connectionsSubscriber]struct{}
}

// connectionKey identifies a single target. Two connections with the same key
// are the same row in the list; the more recent one wins.
type connectionKey struct {
	kind        api.RecentConnectionKind
	cluster     string
	leafCluster string
	displayName string
}

// connectionsSubscriber receives the latest snapshot whenever the list changes.
// updates is a coalescing, latest-wins channel of capacity 1: a slow reader may
// miss intermediate snapshots but always converges to the current list, which is
// safe because every snapshot is the complete, authoritative list.
type connectionsSubscriber struct {
	updates chan []*api.RecentConnection
}

func newRecentConnectionsStore(clock clockwork.Clock) *recentConnectionsStore {
	return &recentConnectionsStore{
		clock:       clock,
		entries:     make(map[connectionKey]*api.RecentConnection),
		subscribers: make(map[*connectionsSubscriber]struct{}),
	}
}

// Reset clears the list and marks the store active for a new VNet run.
func (s *recentConnectionsStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = true
	s.entries = make(map[connectionKey]*api.RecentConnection)
	s.notifyLocked()
}

// CloseSession clears the list, marks the store inactive, and completes all
// current subscriptions so their streaming handlers return. It is idempotent.
func (s *recentConnectionsStore) CloseSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	s.entries = make(map[connectionKey]*api.RecentConnection)
	for sub := range s.subscribers {
		close(sub.updates)
	}
	s.subscribers = make(map[*connectionsSubscriber]struct{})
}

// RecordApp records a connection to a TCP app.
func (s *recentConnectionsStore) RecordApp(appKey *vnetv1.AppKey, publicAddr, clientProcessPath string) {
	s.record(api.RecentConnectionKind_RECENT_CONNECTION_KIND_APP,
		appKey.GetProfile(), appKey.GetLeafCluster(), displayName(publicAddr, appKey.GetName()), clientProcessPath)
}

// RecordDatabase records a connection to a database.
func (s *recentConnectionsStore) RecordDatabase(dbKey *vnetv1.DatabaseKey, fqdn, clientProcessPath string) {
	s.record(api.RecentConnectionKind_RECENT_CONNECTION_KIND_DATABASE,
		dbKey.GetProfile(), dbKey.GetLeafCluster(), displayName(fqdn, dbKey.GetName()), clientProcessPath)
}

// RecordSSH records a connection to an SSH host.
func (s *recentConnectionsStore) RecordSSH(profile, leafCluster, address, clientProcessPath string) {
	s.record(api.RecentConnectionKind_RECENT_CONNECTION_KIND_SSH, profile, leafCluster, address, clientProcessPath)
}

func (s *recentConnectionsStore) record(kind api.RecentConnectionKind, cluster, leafCluster, displayName, clientProcessPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return
	}
	key := connectionKey{kind: kind, cluster: cluster, leafCluster: leafCluster, displayName: displayName}
	s.entries[key] = api.RecentConnection_builder{
		Kind:                  kind,
		Cluster:               cluster,
		LeafCluster:           leafCluster,
		DisplayName:           displayName,
		LastConnected:         timestamppb.New(s.clock.Now()),
		LastClientProcessPath: clientProcessPath,
	}.Build()
	s.notifyLocked()
}

// Subscribe registers a subscriber and returns it along with the current
// snapshot. If the store is inactive (VNet is not running), the subscriber's
// channel is returned already closed.
func (s *recentConnectionsStore) Subscribe() (*connectionsSubscriber, []*api.RecentConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub := &connectionsSubscriber{updates: make(chan []*api.RecentConnection, 1)}
	if !s.active {
		close(sub.updates)
		return sub, nil
	}
	s.subscribers[sub] = struct{}{}
	return sub, s.snapshotLocked()
}

// Unsubscribe removes a subscriber. It is safe to call even if the subscriber
// was already removed by CloseSession.
func (s *recentConnectionsStore) Unsubscribe(sub *connectionsSubscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, sub)
}

// notifyLocked pushes the current snapshot to every subscriber without blocking.
func (s *recentConnectionsStore) notifyLocked() {
	snapshot := s.snapshotLocked()
	for sub := range s.subscribers {
		// Coalescing latest-wins send: drop any pending snapshot, then push the
		// newest, so a slow reader never blocks a connection being recorded.
		select {
		case <-sub.updates:
		default:
		}
		select {
		case sub.updates <- snapshot:
		default:
		}
	}
}

// snapshotLocked returns the deduplicated list ordered most-recently-connected
// first, breaking ties by display name so the order is stable.
func (s *recentConnectionsStore) snapshotLocked() []*api.RecentConnection {
	out := make([]*api.RecentConnection, 0, len(s.entries))
	for _, conn := range s.entries {
		out = append(out, conn)
	}
	slices.SortFunc(out, func(a, b *api.RecentConnection) int {
		if c := b.GetLastConnected().AsTime().Compare(a.GetLastConnected().AsTime()); c != 0 {
			return c
		}
		return strings.Compare(a.GetDisplayName(), b.GetDisplayName())
	})
	return out
}

// displayName returns addr if set, otherwise the fallback (the resource name).
func displayName(addr, fallback string) string {
	if addr != "" {
		return addr
	}
	return fallback
}
