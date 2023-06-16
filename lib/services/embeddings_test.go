// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/services"
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

func TestNodeEmbeddingWatcherCreate(t *testing.T) {
	t.Parallel()
	/*

		ctx := context.Background()
		clock := clockwork.NewFakeClock()

		// Test setup: crate a backend, presence service, the node watcher and
		// the embeddings service
		bk, err := memory.New(memory.Config{
			Context: ctx,
			Clock:   clock,
		})
		require.NoError(t, err)

		type client struct {
			services.Presence
			services.Embeddings
			types.Events
		}

		embedder := MockEmbedder{}
		presence := local.NewPresenceService(bk)
		embeddings := local.NewEmbeddingsService(bk)

		cfg := services.NodeEmbeddingWatcherConfig{
			NodeWatcherConfig: services.NodeWatcherConfig{
				ResourceWatcherConfig: services.ResourceWatcherConfig{
					Component: "test",
					Client: &client{
						Presence:   presence,
						Embeddings: embeddings,
						Events:     local.NewEventsService(bk),
					},
					MaxStaleness: time.Minute,
				},
			},
			Embeddings: embeddings,
			Embedder:   embedder,
		}
		watcher, err := services.NewNodeEmbeddingWatcher(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(watcher.Close)

		// Test start
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

		// Validate the nodes are eventually tracked by the embedding collector
		require.Eventually(t, func() bool {
			return watcher.NodeCount(true) == len(nodes)
		}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive currentNodes.")
		require.Zero(t, watcher.NodeCount(false))

		// Trigger the embedding routine
		err = watcher.RunIndexation(ctx)
		require.NoError(t, err)

		// Validate that all nodes were embedded and snapshot the backend content
		require.Equal(t, watcher.NodeCount(false), len(nodes))
		require.Zero(t, watcher.NodeCount(true))
		items, err := stream.Collect(embeddings.GetEmbeddings(ctx, types.KindNode))
		require.NoError(t, err)
		require.Equal(t, len(items), len(nodes))
	*/
}

func TestNodeEmbeddingWatcherIdempotency(t *testing.T) {
	t.Parallel()
	/*
		ctx := context.Background()
		clock := clockwork.NewFakeClock()

		// Test setup: crate a backend, presence service, the node watcher and
		// the embeddings service
		bk, err := memory.New(memory.Config{
			Context: ctx,
			Clock:   clock,
		})
		require.NoError(t, err)

		type client struct {
			services.Presence
			services.Embeddings
			types.Events
		}

		embedder := MockEmbedder{}
		presence := local.NewPresenceService(bk)
		embeddings := local.NewEmbeddingsService(bk)

		cfg := services.NodeEmbeddingWatcherConfig{
			NodeWatcherConfig: services.NodeWatcherConfig{
				ResourceWatcherConfig: services.ResourceWatcherConfig{
					Component: "test",
					Client: &client{
						Presence:   presence,
						Embeddings: embeddings,
						Events:     local.NewEventsService(bk),
					},
					MaxStaleness: time.Minute,
				},
			},
			Embeddings: embeddings,
			Embedder:   embedder,
		}
		watcher, err := services.NewNodeEmbeddingWatcher(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(watcher.Close)

		// Test start
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

		// Validate the nodes are eventually tracked by the embedding collector
		require.Eventually(t, func() bool {
			return watcher.NodeCount(true) == len(nodes)
		}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive currentNodes.")
		require.Zero(t, watcher.NodeCount(false))

		// Trigger the embedding routine
		err = watcher.RunIndexation(ctx)
		require.NoError(t, err)

		// Validate that all nodes were embedded and snapshot the backend content
		require.Equal(t, watcher.NodeCount(false), len(nodes))
		require.Zero(t, watcher.NodeCount(true))
		items, err := stream.Collect(embeddings.GetEmbeddings(ctx, types.KindNode))
		require.NoError(t, err)
		require.Equal(t, len(items), len(nodes))

		// Trigger the embedding routine again
		err = watcher.RunIndexation(ctx)
		require.NoError(t, err)

		// Validate no nodes are needing embedding and that the items in the backend
		// have been updated
		require.Zero(t, watcher.NodeCount(true))
		newItems, err := stream.Collect(embeddings.GetEmbeddings(ctx, types.KindNode))
		require.NoError(t, err)
		require.Equal(t, len(items), len(newItems))

		for _, oldEmbedding := range items {
			newEmbedding, err := embeddings.GetEmbedding(ctx, types.KindNode, oldEmbedding.GetEmbeddedID())
			require.NoError(t, err)
			require.Equal(t, oldEmbedding.GetVector(), newEmbedding.GetVector())
		}
	*/
}

func TestNodeEmbeddingWatcherUpdate(t *testing.T) {
	t.Parallel()
	/*
		ctx := context.Background()
		clock := clockwork.NewFakeClock()

		// Test setup: crate a backend, presence service, the node watcher and
		// the embeddings service
		bk, err := memory.New(memory.Config{
			Context: ctx,
			Clock:   clock,
		})
		require.NoError(t, err)

		type client struct {
			services.Presence
			services.Embeddings
			types.Events
		}

		embedder := MockEmbedder{}
		presence := local.NewPresenceService(bk)
		embeddings := local.NewEmbeddingsService(bk)

		cfg := services.NodeEmbeddingWatcherConfig{
			NodeWatcherConfig: services.NodeWatcherConfig{
				ResourceWatcherConfig: services.ResourceWatcherConfig{
					Component: "test",
					Client: &client{
						Presence:   presence,
						Embeddings: embeddings,
						Events:     local.NewEventsService(bk),
					},
					MaxStaleness: time.Minute,
				},
			},
			Embeddings: embeddings,
			Embedder:   embedder,
		}
		watcher, err := services.NewNodeEmbeddingWatcher(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(watcher.Close)

		// Test setup: Add some node servers.
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

		// Validate the nodes are eventually tracked by the embedding collector
		require.Eventually(t, func() bool {
			return watcher.NodeCount(true) == len(nodes)
		}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive currentNodes.")
		require.Zero(t, watcher.NodeCount(false))

		// Trigger the embedding routine
		err = watcher.RunIndexation(ctx)
		require.NoError(t, err)

		// Validate that all nodes were embedded and snapshot the backend content
		require.Equal(t, watcher.NodeCount(false), len(nodes))
		require.Zero(t, watcher.NodeCount(true))
		items, err := stream.Collect(embeddings.GetEmbeddings(ctx, types.KindNode))
		require.NoError(t, err)
		require.Equal(t, len(items), len(nodes))

		// Test start
		// Edit the node server labels
		for i := 0; i < 5; i++ {
			nodes[i].SetCmdLabels(
				map[string]types.CommandLabel{
					"version":  &types.CommandLabelV2{Result: "v9"},
					"hostname": &types.CommandLabelV2{Result: fmt.Sprintf("node%d.example.com", i)},
				})
			_, err = presence.UpsertNode(ctx, nodes[i])
			require.NoError(t, err)
		}

		// Validate the node updates have been tracked by the watcher and that the
		// nodes are embedding candidates
		require.Eventually(t, func() bool {
			return watcher.NodeCount(true) == len(nodes)
		}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive currentNodes.")
		require.Zero(t, watcher.NodeCount(false))

		// Trigger the embedding routine again
		err = watcher.RunIndexation(ctx)
		require.NoError(t, err)

		// Validate no nodes are needing embedding and that the items in the backend
		// have been updated
		require.Zero(t, watcher.NodeCount(true))
		newItems, err := stream.Collect(embeddings.GetEmbeddings(ctx, types.KindNode))
		require.NoError(t, err)
		require.Equal(t, len(items), len(newItems))

		for _, oldEmbedding := range items {
			newEmbedding, err := embeddings.GetEmbedding(ctx, types.KindNode, oldEmbedding.GetEmbeddedID())
			require.NoError(t, err)
			require.NotEqual(t, oldEmbedding.GetVector(), newEmbedding.GetVector())
		}
	*/
}

func TestMarshallUnmarshallEmbedding(t *testing.T) {
	// We test that float precision is above six digits
	initial := ai.NewEmbedding(types.KindNode, "foo", ai.Vector64{0.1234567, 1, 1}, sha256.Sum256([]byte("test")))

	marshaled, err := services.MarshalEmbedding(initial)
	require.NoError(t, err)

	final, err := services.UnmarshalEmbedding(marshaled)
	require.NoError(t, err)

	require.Equal(t, initial.EmbeddedId, final.EmbeddedId)
	require.Equal(t, initial.EmbeddedKind, final.EmbeddedKind)
	require.Equal(t, initial.EmbeddedHash, final.EmbeddedHash)
	require.Equal(t, initial.Vector, final.Vector)
}
