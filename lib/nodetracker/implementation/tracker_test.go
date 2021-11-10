/*
Copyright 2021 Gravitational, Inc.

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

package implementation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/nodetracker/api"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

// TestTracker verifies that the tracker Add/Get/Remove methods
// work as expected
func TestTracker(t *testing.T) {
	input := []struct {
		nodeID      string
		proxyID     string
		clusterName string
		addr        string
	}{
		{
			nodeID:      "node-1",
			proxyID:     "proxy-1",
			clusterName: "cluster",
			addr:        "proxy-1:3080",
		},
		{
			nodeID:      "node-2",
			proxyID:     "proxy-2",
			clusterName: "cluster",
			addr:        "proxy-2:3080",
		},
		{
			nodeID:      "node-2",
			proxyID:     "proxy-3",
			clusterName: "cluster",
			addr:        "proxy-3:3080",
		},
	}

	ctx := context.Background()

	config := &Config{OfflineThreshold: 10 * time.Minute}
	tracker := NewTracker(config)

	for _, node := range input {
		tracker.AddNode(ctx, node.nodeID, node.proxyID, node.clusterName, node.addr)
	}

	// simple node get
	proxies := tracker.GetProxies(ctx, "node-1")
	require.Equal(t, len(proxies), 1)
	require.Equal(t, proxies[0].ID, "proxy-1")
	require.Equal(t, proxies[0].ClusterName, "cluster")
	require.Equal(t, proxies[0].Addr, "proxy-1:3080")

	// node remove
	tracker.RemoveNode(ctx, "node-1")
	proxies = tracker.GetProxies(ctx, "node-1")
	require.Equal(t, len(proxies), 0)

	// get multiple proxies
	// the proxies slice is sorted by the insert/update date
	// from the newest to the oldest
	proxies = tracker.GetProxies(ctx, "node-2")
	require.Equal(t, len(proxies), 2)
	require.Equal(t, proxies[0].ID, "proxy-3")
	require.Equal(t, proxies[0].ClusterName, "cluster")
	require.Equal(t, proxies[0].Addr, "proxy-3:3080")
	require.Equal(t, proxies[1].ID, "proxy-2")
	require.Equal(t, proxies[1].ClusterName, "cluster")
	require.Equal(t, proxies[1].Addr, "proxy-2:3080")

	// simulate node update
	// the proxy slice order should change
	tracker.AddNode(ctx, input[1].nodeID, input[1].proxyID, input[1].clusterName, input[1].addr)
	proxies = tracker.GetProxies(ctx, "node-2")
	require.Equal(t, len(proxies), 2)
	require.Equal(t, proxies[0].ID, "proxy-2")
	require.Equal(t, proxies[1].ID, "proxy-3")

	tracker.Stop()
}

// TestTracker verifies that the tracker cleanup functionality
// removes node/proxy relationships that haven't been updated
// in a certain timeframe
func TestTrackerCleanup(t *testing.T) {
	input := []struct {
		nodeID      string
		proxyID     string
		clusterName string
		addr        string
	}{
		{
			nodeID:      "node-1",
			proxyID:     "proxy-1",
			clusterName: "cluster",
			addr:        "proxy-1:3080",
		},
		{
			nodeID:      "node-1",
			proxyID:     "proxy-2",
			clusterName: "cluster",
			addr:        "proxy-2:3080",
		},
		{
			nodeID:      "node-2",
			proxyID:     "proxy-2",
			clusterName: "cluster",
			addr:        "proxy-2:3080",
		},
		{
			nodeID:      "node-2",
			proxyID:     "proxy-3",
			clusterName: "cluster",
			addr:        "proxy-3:3080",
		},
	}

	totalStates := 3
	states := []struct {
		nodeID               string
		proxiesAfterEachTick []int
	}{
		{
			nodeID:               "node-1",
			proxiesAfterEachTick: []int{2, 0, 0},
		},
		{
			nodeID:               "node-2",
			proxiesAfterEachTick: []int{2, 1, 1},
		},
	}

	var tracker api.Tracker
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	// the callback is used to check the results after each tick of the
	// proxy relationship cleaner
	var wg sync.WaitGroup
	wg.Add(totalStates)
	stateCounter := 0
	callback := func() {
		for _, state := range states {
			proxies := tracker.GetProxies(ctx, state.nodeID)
			require.Equal(t, len(proxies), state.proxiesAfterEachTick[stateCounter])
		}

		// issue and update for node-2/proxy-3 to keep the relationship alive
		tracker.AddNode(ctx, input[3].nodeID, input[3].proxyID, input[3].clusterName, input[3].addr)

		wg.Done()
		stateCounter++
		if stateCounter < totalStates {
			clock.Advance(1 * time.Minute)
		}
	}

	config := &Config{
		OfflineThreshold:     1 * time.Minute,
		Clock:                clock,
		ProxyControlCallback: callback,
	}
	tracker = NewTracker(config)

	for _, node := range input {
		tracker.AddNode(ctx, node.nodeID, node.proxyID, node.clusterName, node.addr)
	}

	clock.Advance(1 * time.Minute)

	wg.Wait()
	tracker.Stop()
}

// BenchmarkTracker gathers metrics about the tracker
// running 1,000,000 nodes
//
// run the benchmarks using:
// go test -bench=. -test.cpuprofile=cpu.out -test.memprofile=mem.out
//
// inspect the results using
// go tool pprof implementation.test mem.out
// go tool pprof implementation.test cpu.out
//
// generate a graph
// go tool pprof --pdf implementation.test mem.out > mem.pdf
// go tool pprof --pdf implementation.test cpu.out > cpu.pdf
func BenchmarkTracker(b *testing.B) {
	b.ReportAllocs()

	ctx := context.Background()
	config := &Config{OfflineThreshold: 10 * time.Minute}
	tracker := NewTracker(config)

	input := make(map[string]string, 1000000)
	for i := 0; i < 1000000; i++ {
		input[uuid.New()] = uuid.New()
	}

	// start the benchmark timer.
	b.ResetTimer()

	// insert 1000000 nodes and proxies relationships
	// update them 5 times
	for i := 0; i < 6; i++ {
		for node, proxy := range input {
			tracker.AddNode(ctx, node, proxy, "cluster", "proxy:3080")
		}
		time.Sleep(10 * time.Second)
	}
}
