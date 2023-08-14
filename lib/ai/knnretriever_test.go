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

package ai

import (
	"crypto/sha256"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestKNNRetriever_GetRelevant(t *testing.T) {
	t.Parallel()

	// Generate random vector. The seed is fixed, so the results are deterministic.
	randGen := rand.New(rand.NewSource(42))

	generateVector := func() Vector64 {
		const testVectorDimension = 100
		// generate random vector
		// reduce the dimensionality to 100
		vec := make(Vector64, testVectorDimension)
		for i := 0; i < testVectorDimension; i++ {
			vec[i] = randGen.Float64()
		}
		// normalize vector, so the simiarity between two vectors is the dot product
		// between [0, 1]
		return normalize(vec)
	}

	const testEmbeddingsSize = 100
	points := make([]*Embedding, testEmbeddingsSize)
	for i := 0; i < testEmbeddingsSize; i++ {
		points[i] = NewEmbedding(types.KindNode, strconv.Itoa(i), generateVector(), sha256.Sum256([]byte{byte(i)}))
	}

	// Create a query.
	query := NewEmbedding(types.KindNode, "1", generateVector(), sha256.Sum256([]byte("1")))

	retriever, err := NewKNNRetriever(points)
	require.NoError(t, err)

	// Get the top 10 most similar documents.
	docs := retriever.GetRelevant(query, 10)
	require.Len(t, docs, 10)

	expectedResults := []int{57, 92, 95, 49, 33, 56, 30, 99, 90, 47}
	expectedSimilarities := []float64{0.80405, 0.79051, 0.78161, 0.78159,
		0.77655, 0.77374, 0.77306, 0.76688, 0.76634, 0.76458}

	for i, result := range docs {
		require.Equal(t,
			fmt.Sprintf("%s/%s", types.KindNode, strconv.Itoa(expectedResults[i])),
			result.GetName(), "expected order is wrong")
		require.InDelta(t, expectedSimilarities[i], result.SimilarityScore, 10e-6, "similarity score is wrong")
	}
}

func TestKNNRetriever_Insert(t *testing.T) {
	t.Parallel()

	points := []*Embedding{
		NewEmbedding(types.KindNode, "1", Vector64{1, 2, 3}, sha256.Sum256([]byte("1"))),
		NewEmbedding(types.KindNode, "2", Vector64{4, 5, 6}, sha256.Sum256([]byte("2"))),
	}

	retriever, err := NewKNNRetriever(points)
	require.NoError(t, err)

	newEmbedding := NewEmbedding(types.KindNode, "3", Vector64{7, 8, 9}, sha256.Sum256([]byte("3")))
	docs1 := retriever.GetRelevant(newEmbedding, 10)
	require.Len(t, docs1, 2)

	err = retriever.Insert(newEmbedding)
	require.NoError(t, err)

	docs2 := retriever.GetRelevant(newEmbedding, 10)
	require.Len(t, docs2, 3)
}

func TestKNNRetriever_Remove(t *testing.T) {
	t.Parallel()

	points := []*Embedding{
		NewEmbedding(types.KindNode, "1", Vector64{1, 2, 3}, sha256.Sum256([]byte("1"))),
		NewEmbedding(types.KindNode, "2", Vector64{4, 5, 6}, sha256.Sum256([]byte("2"))),
		NewEmbedding(types.KindNode, "3", Vector64{7, 8, 9}, sha256.Sum256([]byte("3"))),
	}

	retriever, err := NewKNNRetriever(points)
	require.NoError(t, err)

	query := NewEmbedding(types.KindNode, "3", Vector64{7, 8, 9}, sha256.Sum256([]byte("3")))
	docs1 := retriever.GetRelevant(query, 10)

	require.Len(t, docs1, 3)

	err = retriever.Remove("node/2")
	require.NoError(t, err)

	docs2 := retriever.GetRelevant(query, 10)
	require.Len(t, docs2, 2)
}

// Function to calculate L2 norm
func L2norm(v []float64) float64 {
	sum := 0.0
	for _, value := range v {
		sum += value * value
	}
	return math.Sqrt(sum)
}

// Function to normalize vector using L2 norm
func normalize(v Vector64) Vector64 {
	norm := L2norm(v)
	result := make(Vector64, len(v))
	for i, value := range v {
		result[i] = value / norm
	}
	return result
}
