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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/vnet/v1"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func appKey(name string) *vnetv1.AppKey {
	return vnetv1.AppKey_builder{Profile: "root", Name: name}.Build()
}

func TestRecentConnectionsStore_dedupeAndOrder(t *testing.T) {
	clock := clockwork.NewFakeClock()
	store := newRecentConnectionsStore(clock)
	store.Reset()

	store.RecordApp(appKey("a"), "a.example.com")
	clock.Advance(time.Second)
	store.RecordApp(appKey("b"), "b.example.com")

	_, snap := store.Subscribe()
	require.Len(t, snap, 2)
	// Most-recently-connected first.
	assert.Equal(t, "b.example.com", snap[0].GetDisplayName())
	assert.Equal(t, "a.example.com", snap[1].GetDisplayName())

	// Reconnecting to an existing target updates its timestamp and reorders it,
	// rather than adding a duplicate row.
	clock.Advance(time.Second)
	store.RecordApp(appKey("a"), "a.example.com")

	_, snap = store.Subscribe()
	require.Len(t, snap, 2)
	assert.Equal(t, "a.example.com", snap[0].GetDisplayName())
}

func TestRecentConnectionsStore_kindsAreDistinct(t *testing.T) {
	store := newRecentConnectionsStore(clockwork.NewFakeClock())
	store.Reset()

	// Same display name but different kinds are different rows.
	store.RecordApp(appKey("shared"), "shared.example.com")
	store.RecordDatabase(vnetv1.DatabaseKey_builder{Profile: "root", Name: "shared"}.Build(), "shared.example.com")
	store.RecordSSH("root", "", "host.example.com")

	_, snap := store.Subscribe()
	require.Len(t, snap, 3)

	kinds := map[api.RecentConnectionKind]string{}
	for _, c := range snap {
		kinds[c.GetKind()] = c.GetDisplayName()
	}
	assert.Equal(t, "shared.example.com", kinds[api.RecentConnectionKind_RECENT_CONNECTION_KIND_APP])
	assert.Equal(t, "shared.example.com", kinds[api.RecentConnectionKind_RECENT_CONNECTION_KIND_DATABASE])
	assert.Equal(t, "host.example.com", kinds[api.RecentConnectionKind_RECENT_CONNECTION_KIND_SSH])
}

func TestRecentConnectionsStore_fallsBackToName(t *testing.T) {
	store := newRecentConnectionsStore(clockwork.NewFakeClock())
	store.Reset()

	// With no public address, the resource name is used as the display name.
	store.RecordApp(appKey("no-addr"), "")

	_, snap := store.Subscribe()
	require.Len(t, snap, 1)
	assert.Equal(t, "no-addr", snap[0].GetDisplayName())
}

func TestRecentConnectionsStore_inactiveDropsRecords(t *testing.T) {
	store := newRecentConnectionsStore(clockwork.NewFakeClock())

	// The store starts inactive: records before Reset are dropped. This mirrors
	// connection callbacks firing from detached goroutines after VNet stopped.
	store.RecordApp(appKey("a"), "a.example.com")

	sub, snap := store.Subscribe()
	assert.Empty(t, snap)
	// Subscribing to an inactive store yields an already-closed channel.
	_, ok := <-sub.updates
	assert.False(t, ok)
}

func TestRecentConnectionsStore_resetClears(t *testing.T) {
	store := newRecentConnectionsStore(clockwork.NewFakeClock())
	store.Reset()
	store.RecordApp(appKey("a"), "a.example.com")

	store.Reset()

	_, snap := store.Subscribe()
	assert.Empty(t, snap)
}

func TestRecentConnectionsStore_subscriberReceivesUpdates(t *testing.T) {
	store := newRecentConnectionsStore(clockwork.NewFakeClock())
	store.Reset()

	sub, snap := store.Subscribe()
	assert.Empty(t, snap)

	store.RecordApp(appKey("a"), "a.example.com")
	got := <-sub.updates
	require.Len(t, got, 1)
	assert.Equal(t, "a.example.com", got[0].GetDisplayName())
}

func TestRecentConnectionsStore_closeSessionCompletesSubscribers(t *testing.T) {
	store := newRecentConnectionsStore(clockwork.NewFakeClock())
	store.Reset()
	sub, _ := store.Subscribe()

	store.CloseSession()

	// The subscriber's channel is closed so its streaming handler returns.
	_, ok := <-sub.updates
	assert.False(t, ok)

	// CloseSession is idempotent.
	assert.NotPanics(t, store.CloseSession)
}
