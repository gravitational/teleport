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
	"sort"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/backend/memory"
)

var (
	embedding1 = ai.NewEmbedding(types.KindNode, "foo", ai.Vector64{0, 0}, sha256.Sum256([]byte("test1")))
	embedding2 = ai.NewEmbedding(types.KindNode, "bar", ai.Vector64{1, 1, 1}, sha256.Sum256([]byte("test2")))
	embedding3 = ai.NewEmbedding(types.KindDatabase, "bar", ai.Vector64{2}, sha256.Sum256([]byte("test3")))
)

func errorIsNotFound(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsNotFound(err), msgAndArgs...)
}

func TestGetEmbedding(t *testing.T) {
	t.Parallel()
	// Test setup: create the backend, the service, and load all fixtures
	ctx := context.Background()

	fixtures := []*ai.Embedding{embedding1, embedding2, embedding3}

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewEmbeddingsService(backend)

	for _, fixture := range fixtures {
		_, err := service.UpsertEmbedding(ctx, fixture)
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
			expected:  embedding1,
		},
		{
			name:      "Kind conflict",
			kind:      types.KindDatabase,
			id:        "bar",
			assertErr: require.NoError,
			expected:  embedding3,
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
			requireEmbeddingsEqual(t, tc.expected, embedding)
		})
	}
}

func TestGetEmbeddings(t *testing.T) {
	t.Parallel()
	// Test setup: create the backend, the service, and load all fixtures
	ctx := context.Background()

	fixtures := []*ai.Embedding{embedding1, embedding2, embedding3}

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewEmbeddingsService(backend)

	for _, fixture := range fixtures {
		_, err := service.UpsertEmbedding(ctx, fixture)
		require.NoError(t, err)
	}

	// Test execution
	tests := []struct {
		name      string
		kind      string
		assertErr require.ErrorAssertionFunc
		expected  sortableEmbeddings
	}{
		{
			name:      "Get multiple embeddings",
			kind:      types.KindNode,
			assertErr: require.NoError,
			expected:  sortableEmbeddings{embedding1, embedding2},
		},
		{
			name:      "Get single embedding",
			kind:      types.KindDatabase,
			assertErr: require.NoError,
			expected:  sortableEmbeddings{embedding3},
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
			var embeddings sortableEmbeddings
			var err error
			embeddings, err = stream.Collect(service.GetEmbeddings(ctx, tc.kind))
			tc.assertErr(t, err)
			sort.Sort(embeddings)
			sort.Sort(tc.expected)
			require.Equal(t, len(tc.expected), len(embeddings))
			for i, expected := range tc.expected {
				requireEmbeddingsEqual(t, expected, embeddings[i])
			}
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

	service := NewEmbeddingsService(backend)

	// Test: check there's nothing in the backend first
	_, err = service.GetEmbedding(ctx, types.KindNode, "foo")
	errorIsNotFound(t, err)

	// Test: add an element in the backend and check if we can retrieve it
	embedding := ai.NewEmbedding(types.KindNode, "foo", ai.Vector64{0, 0}, sha256.Sum256([]byte("test")))
	embedding, err = service.UpsertEmbedding(ctx, embedding)
	require.NoError(t, err)
	result, err := service.GetEmbedding(ctx, types.KindNode, "foo")
	require.NoError(t, err)
	requireEmbeddingsEqual(t, embedding, result)

	// Test: update the embedding and check we now retrieve the new version
	embedding = ai.NewEmbedding(types.KindNode, "foo", ai.Vector64{1, 1, 1, 1, 1}, sha256.Sum256([]byte("test2")))
	embedding, err = service.UpsertEmbedding(ctx, embedding)
	require.NoError(t, err)
	result, err = service.GetEmbedding(ctx, types.KindNode, "foo")
	require.NoError(t, err)
	requireEmbeddingsEqual(t, embedding, result)
}

// sortableEmbeddings is an ai.Embedding list that can be sorted. This is used
// in tests to compare two lists and their content.
type sortableEmbeddings []*ai.Embedding

func (s sortableEmbeddings) Len() int {
	return len(s)
}

func (s sortableEmbeddings) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

func (s sortableEmbeddings) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// requireEmbeddingsEqual checks if two embeddings are equal or fails the test otherwise.
// This is required because equivalent ai.Embedding might differ depending on
// how they have been created (marshaling/unmarshalling protobuf messages set
// some internal fields that a freshly created ai.Embedding doesn't have).
func requireEmbeddingsEqual(t require.TestingT, expected, actual *ai.Embedding) {
	if expected == nil {
		require.Nil(t, actual)
		return
	}
	require.NotNil(t, actual)
	require.Equal(t, expected.EmbeddedId, actual.EmbeddedId)
	require.Equal(t, expected.EmbeddedKind, actual.EmbeddedKind)
	require.Equal(t, expected.EmbeddedHash, actual.EmbeddedHash)
	require.Equal(t, expected.Vector, actual.Vector)
}
