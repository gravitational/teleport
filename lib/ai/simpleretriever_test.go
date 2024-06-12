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

package ai

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai/embedding"
)

// Function to calculate L2 norm
func L2norm(v []float64) float64 {
	sum := 0.0
	for _, value := range v {
		sum += value * value
	}
	return math.Sqrt(sum)
}

// Function to normalize vector using L2 norm
func normalize(v embedding.Vector64) embedding.Vector64 {
	norm := L2norm(v)
	result := make(embedding.Vector64, len(v))
	for i, value := range v {
		result[i] = value / norm
	}
	return result
}

func TestSimpleRetriever_GetRelevant(t *testing.T) {
	t.Parallel()

	// Generate random vector. The seed is fixed, so the results are deterministic.
	randGen := rand.New(rand.NewSource(42))

	generateVector := func() embedding.Vector64 {
		const testVectorDimension = 100
		// generate random vector
		// reduce the dimensionality to 100
		vec := make(embedding.Vector64, testVectorDimension)
		for i := 0; i < testVectorDimension; i++ {
			vec[i] = randGen.Float64()
		}
		// normalize vector, so the similarity between two vectors is the dot product
		// between [0, 1]
		return normalize(vec)
	}

	const testEmbeddingsSize = 100
	points := make([]*embedding.Embedding, testEmbeddingsSize)
	for i := 0; i < testEmbeddingsSize; i++ {
		points[i] = embedding.NewEmbedding(types.KindNode, strconv.Itoa(i), generateVector(), [32]byte{})
	}

	// Create a query.
	query := embedding.NewEmbedding(types.KindNode, "1", generateVector(), [32]byte{})

	retriever := NewSimpleRetriever()

	for _, point := range points {
		retriever.Insert(point.GetName(), point)
	}

	// Get the top 10 most similar documents.
	docs := retriever.GetRelevant(query, 10, func(id string, embedding *embedding.Embedding) bool {
		return true
	})
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
