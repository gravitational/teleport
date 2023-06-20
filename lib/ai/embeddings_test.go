/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ai_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
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
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

// MockEmbedder returns embeddings based on the sha256 hash function. Those
// embeddings have no semantic meaning but ensure different embedded content
// provides different embeddings.
type MockEmbedder struct{}

func (m MockEmbedder) ComputeEmbeddings(_ context.Context, input []string) ([]ai.Vector64, error) {
	result := make([]ai.Vector64, len(input))
	for i, text := range input {
		hash := sha256.Sum256([]byte(text))
		vector := make(ai.Vector64, len(hash))
		for j, x := range hash {
			vector[j] = 1 / float64(int(x)+1)
		}
		result[i] = vector
	}
	return result, nil
}

func TestNodeEmbeddingGeneration(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClock()

	// Test setup: crate a backend, presence service, the node watcher and
	// the embeddings service
	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	embedder := MockEmbedder{}
	presence := local.NewPresenceService(bk)
	embeddings := local.NewEmbeddingsService(bk)

	processor := ai.NewEmbeddingProcessor(&ai.EmbeddingProcessorConfig{
		AiClient:     &embedder,
		EmbeddingSrv: embeddings,
		NodeSrv:      presence,
		Log:          utils.NewLoggerForTests(),
		Jitter:       retryutils.NewSeventhJitter(),
	})

	done := make(chan struct{})
	go func() {
		err := processor.Run(ctx, 100*time.Millisecond)
		assert.ErrorIs(t, context.Canceled, err)
		close(done)
	}()

	// Add some node servers.
	const numNodes = 5
	nodes := make([]types.Server, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		node, _ := types.NewServer(fmt.Sprintf("node%d", i), types.KindNode, types.ServerSpecV2{
			Addr:     "127.0.0.1:1234",
			Hostname: fmt.Sprintf("node%d", i),
			CmdLabels: map[string]types.CommandLabelV2{
				"version":  {Result: "v8"},
				"hostname": {Result: fmt.Sprintf("node%d.example.com", i)},
			},
		})
		_, err = presence.UpsertNode(ctx, node)
		require.NoError(t, err)
		nodes = append(nodes, node)
	}

	require.Eventually(t, func() bool {
		items, err := stream.Collect(embeddings.GetEmbeddings(ctx, types.KindNode))
		assert.NoError(t, err)
		return (len(items) == numNodes) && (len(nodes) == numNodes)
	}, 7*time.Second, 200*time.Millisecond)

	cancel()

	waitForDone(t, done, "timed out waiting for processor to stop")

	validateEmbeddings(t,
		presence.GetNodeStream(ctx, defaults.Namespace),
		embeddings.GetEmbeddings(ctx, types.KindNode))
}

func TestMarshallUnmarshallEmbedding(t *testing.T) {
	// We test that float precision is above six digits
	initial := ai.NewEmbedding(types.KindNode, "foo", ai.Vector64{0.1234567, 1, 1}, sha256.Sum256([]byte("test")))

	marshaled, err := ai.MarshalEmbedding(initial)
	require.NoError(t, err)

	final, err := ai.UnmarshalEmbedding(marshaled)
	require.NoError(t, err)

	require.Equal(t, initial.EmbeddedId, final.EmbeddedId)
	require.Equal(t, initial.EmbeddedKind, final.EmbeddedKind)
	require.Equal(t, initial.EmbeddedHash, final.EmbeddedHash)
	require.Equal(t, initial.Vector, final.Vector)
}

func waitForDone(t *testing.T, done chan struct{}, errMsg string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal(errMsg)
	}
}

func validateEmbeddings(t *testing.T, nodesStream stream.Stream[types.Server], embeddingsStream stream.Stream[*ai.Embedding]) {
	t.Helper()

	nodes, err := stream.Collect(nodesStream)
	require.NoError(t, err)

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
