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

package local

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/backend/memory"
)

var (
	embedding1 = ai.NewEmbedding(types.KindNode, "foo", ai.Vector32{0, 0}, sha256.Sum256([]byte("test1")))
	embedding2 = ai.NewEmbedding(types.KindNode, "bar", ai.Vector32{1, 1, 1}, sha256.Sum256([]byte("test2")))
	embedding3 = ai.NewEmbedding(types.KindDatabase, "bar", ai.Vector32{2}, sha256.Sum256([]byte("test3")))
)

func errorIsNotFound(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsNotFound(err), msgAndArgs...)
}

func TestGetEmbedding(t *testing.T) {
	t.Parallel()
	// Test setup: create the backend, the service, and load all fixtures
	ctx := context.Background()

	fixtures := []ai.Embedding{embedding1, embedding2, embedding3}

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewEmbeddingsService(backend, nil)

	for _, fixture := range fixtures {
		_, err := service.UpsertEmbedding(ctx, &fixture)
		require.NoError(t, err)
	}

	// Test execution
	tests := []struct {
		name      string
		kind      string
		id        string
		assertErr require.ErrorAssertionFunc
		expected  *ai.Embedding
	}{
		{
			name:      "Simple get",
			kind:      types.KindNode,
			id:        "foo",
			assertErr: require.NoError,
			expected:  &embedding1,
		},
		{
			name:      "Kind conflict",
			kind:      types.KindDatabase,
			id:        "bar",
			assertErr: require.NoError,
			expected:  &embedding3,
		},
		{
			name:      "Non-existing",
			kind:      types.KindDatabase,
			id:        "foo",
			assertErr: errorIsNotFound,
			expected:  nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			embedding, err := service.GetEmbedding(ctx, tc.kind, tc.id)
			tc.assertErr(t, err)
			require.Equal(t, tc.expected, embedding)
		})
	}
}

func TestGetEmbeddings(t *testing.T) {
	t.Parallel()
	// Test setup: create the backend, the service, and load all fixtures
	ctx := context.Background()

	fixtures := []ai.Embedding{embedding1, embedding2, embedding3}

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewEmbeddingsService(backend, nil)

	for _, fixture := range fixtures {
		_, err := service.UpsertEmbedding(ctx, &fixture)
		require.NoError(t, err)
	}

	// Test execution
	tests := []struct {
		name      string
		kind      string
		assertErr require.ErrorAssertionFunc
		expected  []ai.Embedding
	}{
		{
			name:      "Get multiple embeddings",
			kind:      types.KindNode,
			assertErr: require.NoError,
			expected:  []ai.Embedding{embedding1, embedding2},
		},
		{
			name:      "Get single embedding",
			kind:      types.KindDatabase,
			assertErr: require.NoError,
			expected:  []ai.Embedding{embedding3},
		},
		{
			name:      "Get no embeddings",
			kind:      types.KindApp,
			assertErr: require.NoError,
			expected:  nil,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			embeddings, err := service.GetEmbeddings(ctx, tc.kind)
			tc.assertErr(t, err)
			require.ElementsMatch(t, tc.expected, embeddings)
		})
	}
}

func TestUpsertEmbedding(t *testing.T) {
	t.Parallel()
	// Test setup: create the backend, the service
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewEmbeddingsService(backend, nil)

	// Test: check there's nothing in the backend first
	_, err = service.GetEmbedding(ctx, types.KindNode, "foo")
	errorIsNotFound(t, err)

	// Test: add an element in the backend and check if we can retrieve it
	embedding := ai.NewEmbedding(types.KindNode, "foo", ai.Vector32{0, 0}, sha256.Sum256([]byte("test")))
	_, err = service.UpsertEmbedding(ctx, &embedding)
	require.NoError(t, err)
	result, err := service.GetEmbedding(ctx, types.KindNode, "foo")
	require.NoError(t, err)
	require.Equal(t, &embedding, result)

	// Test: update the embedding and check we now retrieve the new version
	embedding = ai.NewEmbedding(types.KindNode, "foo", ai.Vector32{1, 1, 1, 1, 1}, sha256.Sum256([]byte("test2")))
	_, err = service.UpsertEmbedding(ctx, &embedding)
	require.NoError(t, err)
	result, err = service.GetEmbedding(ctx, types.KindNode, "foo")
	require.NoError(t, err)
	require.Equal(t, &embedding, result)
}
