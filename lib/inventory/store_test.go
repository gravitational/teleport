/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inventory

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/stretchr/testify/require"
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

	for n := 0; n < b.N; n++ {
		store := NewStore()
		var wg sync.WaitGroup

		for i := 0; i < insertions; i++ {
			wg.Add(1)
			go func(sn int) {
				defer wg.Done()
				serverID := fmt.Sprintf("server-%d", sn%uniqueServers)
				handle := &upstreamHandle{
					hello: proto.UpstreamInventoryHello{
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
						store.Iter(func(h UpstreamHandle) {
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
		require.True(b, store.Len() == uniqueServers)
	}
}

// TestStoreAccess verifies the two most important properties of the store:
// 1. Handles are loadable by ID.
// 2. When multiple handles have the same ID, loads are distributed across them.
//
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
			hello: proto.UpstreamInventoryHello{
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
		store.Iter(func(h UpstreamHandle) {
			ptr := h.(*upstreamHandle)
			n, ok := handles[ptr]
			require.True(t, ok)
			handles[ptr] = n + 1
		})
	}

	for _, n := range handles {
		require.NotZero(t, n)
	}
}
