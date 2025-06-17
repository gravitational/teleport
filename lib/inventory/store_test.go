/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package inventory

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
)

/*
goos: linux
goarch: amd64
pkg: github.com/gravitational/teleport/lib/inventory
cpu: Intel(R) Xeon(R) CPU @ 2.80GHz
BenchmarkStore-4               3         480249642 ns/op
*/
func BenchmarkStore(b *testing.B) {
	const insertions = 100_000
	const uniqueServers = 10_000
	const readMod = 100

	// bg goroutines propagate failures via a sync.Once
	// since require shouldn't be called except from the
	// main test/bench goroutine.
	var failOnce sync.Once
	var failErr error

	for b.Loop() {
		store := NewStore()
		var wg sync.WaitGroup

		for i := 0; i < insertions; i++ {
			wg.Add(1)
			go func(sn int) {
				defer wg.Done()
				serverID := fmt.Sprintf("server-%d", sn%uniqueServers)
				handle := &upstreamHandle{
					hello: &proto.UpstreamInventoryHello{
						ServerID: serverID,
					},
				}
				store.Insert(handle)
				_, ok := store.Get(serverID)
				if !ok {
					failOnce.Do(func() {
						failErr = fmt.Errorf("get failed for %s", serverID)
					})
					return
				}
				if sn%readMod == 0 {
					wg.Add(1)
					go func() {
						defer wg.Done()
						var foundServer bool
						store.UniqueHandles(func(h UpstreamHandle) {
							if h.Hello().ServerID == serverID {
								foundServer = true
							}
						})
						if !foundServer {
							failOnce.Do(func() {
								failErr = fmt.Errorf("iter failed to include %s", serverID)
							})
							return
						}
					}()
				}
			}(i)
		}
		wg.Wait()
		failOnce.Do(func() {})
		require.NoError(b, failErr)
		require.Equal(b, uniqueServers, store.Len())
	}
}

// TestStoreAccess verifies the two most important properties of the store:
// 1. Handles are loadable by ID.
// 2. When multiple handles have the same ID, loads are distributed across them.
func TestStoreAccess(t *testing.T) {
	store := NewStore()

	// we keep a record of all handles inserted into the store
	// so that we can ensure that we visit all of them during
	// iteration.
	handles := make(map[*upstreamHandle]int)

	// create 1_000 handles across 100 unique server IDs.
	for i := 0; i < 1_000; i++ {
		serverID := fmt.Sprintf("server-%d", i%100)
		handle := &upstreamHandle{
			hello: &proto.UpstreamInventoryHello{
				ServerID: serverID,
			},
		}
		store.Insert(handle)
		handles[handle] = 0
	}

	// ensure that all server IDs yield a handle
	for h := range handles {
		_, ok := store.Get(h.Hello().ServerID)
		require.True(t, ok)
	}

	// ensure that all handles are visited if we iterate many times
	for i := 0; i < 1_000; i++ {
		store.UniqueHandles(func(h UpstreamHandle) {
			ptr := h.(*upstreamHandle)
			n, ok := handles[ptr]
			require.True(t, ok)
			handles[ptr] = n + 1
		})
	}

	// verify that each handle was seen, then remove it
	for h, n := range handles {
		require.NotZero(t, n)
		store.Remove(h)
	}

	// verify that all handles were removed
	var count int
	store.UniqueHandles(func(h UpstreamHandle) {
		count++
	})
	require.Zero(t, count)
}

// TestAllHandles verifies that AllHandles allows us to visit
// every handle in the store, even when multiple handles are registered with
// the same server ID.
func TestAllHandles(t *testing.T) {
	store := NewStore()

	// we keep a record of all handles inserted into the store
	// so that we can ensure that we visit all of them during
	// iteration.
	handles := make(map[*upstreamHandle]int)

	// create 1_000 handles across 100 unique server IDs.
	for i := 0; i < 1_000; i++ {
		serverID := fmt.Sprintf("server-%d", i%100)
		handle := &upstreamHandle{
			hello: &proto.UpstreamInventoryHello{
				ServerID: serverID,
			},
		}
		store.Insert(handle)
		handles[handle] = 0
	}

	// ensure that all handles are visited
	store.AllHandles(func(h UpstreamHandle) {
		ptr := h.(*upstreamHandle)
		n, ok := handles[ptr]
		require.True(t, ok)
		handles[ptr] = n + 1
	})

	// verify that each handle was seen, then remove it
	for h, n := range handles {
		require.NotZero(t, n)
		store.Remove(h)
	}

	// verify that all handles were removed
	var count int
	store.UniqueHandles(func(h UpstreamHandle) {
		count++
	})
	require.Zero(t, count)
}
