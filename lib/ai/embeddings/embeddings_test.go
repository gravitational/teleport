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

package embeddings_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/utils/retryutils"
	aiembeddings "github.com/gravitational/teleport/lib/ai/embeddings"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
)

// MockEmbedder returns embeddings based on the sha256 hash function. Those
// embeddings have no semantic meaning but ensure different embedded content
// provides different embeddings.
type MockEmbedder struct {
}

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

	processor := aiembeddings.NewEmbeddingProcessor(&aiembeddings.EmbeddingProcessorConfig{
		AiClient:     &embedder,
		EmbeddingSrv: embeddings,
		NodeSrv:      presence,
		Log:          logrus.WithField(trace.Component, "test"),
		Jitter:       retryutils.NewSeventhJitter(),
	})

	done := make(chan struct{})
	go func() {
		err := processor.Run(ctx, 100*time.Millisecond)
		require.ErrorContains(t, err, "context canceled")
		close(done)
	}()

	// Add some node servers.
	nodes := make([]types.Server, 0, 5)
	for i := 0; i < 5; i++ {
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
		require.NoError(t, err)
		return (len(items) == 5) && (len(nodes) == 5)
	}, 7*time.Second, 200*time.Millisecond)

	cancel()

	waitForDone(t, done)

	validateEmbeddings(t,
		presence.GetNodeStream(ctx, defaults.Namespace),
		embeddings.GetEmbeddings(ctx, types.KindNode))
}

func TestMarshallUnmarshallEmbedding(t *testing.T) {
	// We test that float precision is above six digits
	initial := ai.NewEmbedding(types.KindNode, "foo", ai.Vector64{0.1234567, 1, 1}, sha256.Sum256([]byte("test")))

	marshaled, err := aiembeddings.MarshalEmbedding(initial)
	require.NoError(t, err)

	final, err := aiembeddings.UnmarshalEmbedding(marshaled)
	require.NoError(t, err)

	require.Equal(t, initial.EmbeddedId, final.EmbeddedId)
	require.Equal(t, initial.EmbeddedKind, final.EmbeddedKind)
	require.Equal(t, initial.EmbeddedHash, final.EmbeddedHash)
	require.Equal(t, initial.Vector, final.Vector)
}

func waitForDone(t *testing.T, done chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for processor to stop")
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
