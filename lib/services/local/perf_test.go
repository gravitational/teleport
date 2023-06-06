/*
Copyright 2020 Gravitational, Inc.

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

package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

// BenchmarkGetNodes verifies the performance of the GetNodes operation
// on local (sqlite) databases (as used by the cache system).
func BenchmarkGetNodes(b *testing.B) {
	ctx := context.Background()

	type testCase struct {
		memory bool
		nodes  int
	}

	var tts []testCase

	for _, memory := range []bool{true, false} {
		for _, nodes := range []int{100, 1000, 10000} {
			tts = append(tts, testCase{
				memory: memory,
				nodes:  nodes,
			})
		}
	}

	for _, tt := range tts {
		// create a descriptive name for the sub-benchmark.
		name := fmt.Sprintf("tt(memory=%v,nodes=%d)", tt.memory, tt.nodes)

		// run the sub benchmark
		b.Run(name, func(sb *testing.B) {

			sb.StopTimer() // stop timer while running setup

			// configure the backend instance
			var bk backend.Backend
			var err error
			if tt.memory {
				bk, err = memory.New(memory.Config{})
				require.NoError(b, err)
			} else {
				dir := b.TempDir()

				bk, err = lite.NewWithConfig(context.TODO(), lite.Config{
					Path: dir,
				})
				require.NoError(b, err)
			}
			defer bk.Close()

			svc := NewPresenceService(bk)
			// seed the test nodes
			insertNodes(ctx, b, svc, tt.nodes)

			sb.StartTimer() // restart timer for benchmark operations

			benchmarkGetNodes(ctx, sb, svc, tt.nodes)

			sb.StopTimer() // stop timer to exclude deferred cleanup
		})
	}
}

// insertNodes inserts a collection of test nodes into a backend.
func insertNodes(ctx context.Context, b *testing.B, svc services.Presence, nodeCount int) {
	const labelCount = 10
	labels := make(map[string]string, labelCount)
	for i := 0; i < labelCount; i++ {
		labels[fmt.Sprintf("label-key-%d", i)] = fmt.Sprintf("label-val-%d", i)
	}
	for i := 0; i < nodeCount; i++ {
		name, addr := fmt.Sprintf("node-%d", i), fmt.Sprintf("node%d.example.com", i)
		node := &types.ServerV2{
			Kind:    types.KindNode,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
				Labels:    labels,
			},
			Spec: types.ServerSpecV2{
				Addr: addr,
			},
		}
		_, err := svc.UpsertNode(ctx, node)
		require.NoError(b, err)
	}
}

// benchmarkGetNodes runs GetNodes b.N times.
func benchmarkGetNodes(ctx context.Context, b *testing.B, svc services.Presence, nodeCount int) {
	var nodes []types.Server
	var err error
	for i := 0; i < b.N; i++ {
		nodes, err = svc.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(b, err)
	}
	// do *something* with the loop result.  probably unnecessary since the loop
	// contains I/O, but I don't know enough about the optimizer to be 100% certain
	// about that.
	require.Equal(b, nodeCount, len(nodes))
}
