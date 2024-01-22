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

package ai_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/ai/embedding"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func TestNodeEmbeddingGeneration(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClock()

	// Test setup: crate a backend, presence service, the node watcher and
	// the embeddings service.
	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	embedder := ai.MockEmbedder{
		TimesCalled: make(map[string]int),
	}
	events := local.NewEventsService(bk)
	accessLists, err := local.NewAccessListService(bk, clock)
	require.NoError(t, err)
	resources := &mockResourceGetter{
		Presence:    local.NewPresenceService(bk),
		AccessLists: accessLists,
	}

	cache, err := services.NewUnifiedResourceCache(ctx, services.UnifiedResourceCacheConfig{
		ResourceGetter: resources,
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "resource-watcher",
			Client:    events,
		},
	})
	require.NoError(t, err)

	embeddings := local.NewEmbeddingsService(bk)

	processor := ai.NewEmbeddingProcessor(&ai.EmbeddingProcessorConfig{
		AIClient:            &embedder,
		EmbeddingSrv:        embeddings,
		EmbeddingsRetriever: ai.NewSimpleRetriever(),
		NodeSrv:             cache,
		Log:                 utils.NewLoggerForTests(),
		Jitter:              retryutils.NewSeventhJitter(),
	})

	go func() {
		err := processor.Run(ctx, 100*time.Millisecond, time.Second)
		assert.ErrorIs(t, context.Canceled, err)
	}()

	// Add some node servers.
	const numInitialNodes = 5
	nodes := make([]types.Server, 0, numInitialNodes)
	for i := 0; i < numInitialNodes; i++ {
		node := makeNode(i + 1)
		_, err = resources.UpsertNode(ctx, node)
		require.NoError(t, err)
		nodes = append(nodes, node)
	}

	require.Eventually(t, func() bool {
		items, err := stream.Collect(embeddings.GetAllEmbeddings(ctx))
		assert.NoError(t, err)
		return len(items) == numInitialNodes
	}, 14*time.Second, 200*time.Millisecond)

	nodesAcquired, err := resources.GetNodes(ctx, defaults.Namespace)
	require.NoError(t, err)

	validateEmbeddings(t,
		nodesAcquired,
		embeddings.GetAllEmbeddings(ctx))

	for k, v := range embedder.TimesCalled {
		require.Equal(t, 1, v, "expected %v to be computed once, was %d", k, v)
	}

	// Run once more and verify that only changed or newly inserted nodes get their embeddings calculated
	node1 := nodes[0]
	node1.GetMetadata().Labels["foo"] = "bar"
	_, err = resources.UpsertNode(ctx, node1)
	require.NoError(t, err)
	node6 := makeNode(6)
	_, err = resources.UpsertNode(ctx, node6)
	require.NoError(t, err)

	// Since nodes are streamed in ascending order by names, when embeddings for node6 are calculated,
	// we can be sure that our recent changes have been fully processed
	require.Eventually(t, func() bool {
		items, err := stream.Collect(embeddings.GetAllEmbeddings(ctx))
		assert.NoError(t, err)
		return len(items) == numInitialNodes+1
	}, 7*time.Second, 200*time.Millisecond)

	for k, v := range embedder.TimesCalled {
		expected := 1
		if strings.Contains(k, "node1") {
			expected = 2
		}
		require.Equal(t, expected, v, "expected embedding for %q to be computed %d times, got computed %d times", k, expected, v)
	}

	nodesAcquired, err = resources.GetNodes(ctx, defaults.Namespace)
	require.NoError(t, err)

	validateEmbeddings(t,
		nodesAcquired,
		embeddings.GetAllEmbeddings(ctx))
}

func TestMarshallUnmarshallEmbedding(t *testing.T) {
	// We test that float precision is above six digits
	initial := embedding.NewEmbedding(types.KindNode, "foo", embedding.Vector64{0.1234567, 1, 1}, sha256.Sum256([]byte("test")))

	marshaled, err := ai.MarshalEmbedding(initial)
	require.NoError(t, err)

	final, err := ai.UnmarshalEmbedding(marshaled)
	require.NoError(t, err)

	require.Equal(t, initial.EmbeddedId, final.EmbeddedId)
	require.Equal(t, initial.EmbeddedKind, final.EmbeddedKind)
	require.Equal(t, initial.EmbeddedHash, final.EmbeddedHash)
	require.Equal(t, initial.Vector, final.Vector)
}

func makeNode(num int) types.Server {
	node, _ := types.NewServer(fmt.Sprintf("node%d", num), types.KindNode, types.ServerSpecV2{
		Addr:     "127.0.0.1:1234",
		Hostname: fmt.Sprintf("node%d", num),
		CmdLabels: map[string]types.CommandLabelV2{
			"version":  {Result: "v8"},
			"hostname": {Result: fmt.Sprintf("node%d.example.com", num)},
		},
	})
	return node
}

func validateEmbeddings(t *testing.T, nodes []types.Server, embeddingsStream stream.Stream[*embedding.Embedding]) {
	t.Helper()

	embeddings, err := stream.Collect(embeddingsStream)
	require.NoError(t, err)

	require.Equal(t, len(nodes), len(embeddings), "Number of nodes and embeddings should be equal")

	for i, node := range nodes {
		emb := embeddings[i]

		require.Equal(t, node.GetName(), emb.GetEmbeddedID(), "Node ID and embedding ID should be equal")
		require.Equal(t, types.KindNode, emb.GetEmbeddedKind(), "Node kind and embedding kind should be equal")
	}
}

func Test_batchReducer_Add(t *testing.T) {
	t.Parallel()

	// Sum process function - used for simplicity
	sumFn := func(ctx context.Context, data []int) (int, error) {
		sum := 0
		for _, d := range data {
			sum += d
		}
		return sum, nil
	}

	type testCase struct {
		// Test case name
		name string
		// Process batch size
		batchSize int
		// Input data
		data []int
		// Function to process batch
		processFn func(ctx context.Context, data []int) (int, error)
		// Expected result on Add
		want []int
		// Expected result on Finalize
		finalizeResult int
		// Expected error
		wantErr assert.ErrorAssertionFunc
	}

	tests := []testCase{
		{
			name:           "empty",
			batchSize:      100,
			data:           []int{},
			want:           []int{},
			finalizeResult: 0,
			processFn:      sumFn,
			wantErr:        assert.NoError,
		},
		{
			name:           "one element",
			batchSize:      100,
			data:           []int{1},
			want:           []int{0},
			finalizeResult: 1,
			processFn:      sumFn,
			wantErr:        assert.NoError,
		},
		{
			name:           "many elements",
			batchSize:      3,
			data:           []int{1, 1, 1, 1},
			want:           []int{0, 0, 3, 0},
			finalizeResult: 1,
			processFn:      sumFn,
			wantErr:        assert.NoError,
		},
		{
			name:      "propagate error",
			batchSize: 2,
			data:      []int{0},
			want:      []int{0},
			processFn: func(ctx context.Context, data []int) (int, error) {
				return 0, errors.New("error")
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			br := ai.NewBatchReducer[int, int](tt.processFn, tt.batchSize)

			for i, d := range tt.data {
				got, err := br.Add(ctx, d)
				require.NoError(t, err)
				assert.Equalf(t, tt.want[i], got, "Add(%v)", tt.data)
			}

			got, err := br.Finalize(ctx)
			if !tt.wantErr(t, err, fmt.Sprintf("Finalize(%v)", tt.data)) {
				return
			}
			assert.Equalf(t, tt.finalizeResult, got, "Finalize(%v)", tt.data)
		})
	}
}

type mockResourceGetter struct {
	services.Presence
	services.AccessLists
}

func (m *mockResourceGetter) GetDatabaseServers(_ context.Context, _ string, _ ...services.MarshalOption) ([]types.DatabaseServer, error) {
	return nil, nil
}

func (m *mockResourceGetter) GetKubernetesServers(_ context.Context) ([]types.KubeServer, error) {
	return nil, nil
}

func (m *mockResourceGetter) GetApplicationServers(_ context.Context, _ string) ([]types.AppServer, error) {
	return nil, nil
}

func (m *mockResourceGetter) GetWindowsDesktops(_ context.Context, _ types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	return nil, nil
}

func (m *mockResourceGetter) ListSAMLIdPServiceProviders(_ context.Context, _ int, _ string) ([]types.SAMLIdPServiceProvider, string, error) {
	return nil, "", nil
}
