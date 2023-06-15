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
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestSimpleRetriever_GetRelevant(t *testing.T) {
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
		points[i] = NewEmbedding(types.KindNode, strconv.Itoa(i), generateVector())
	}

	// Create a query.
	query := NewEmbedding(types.KindNode, "1", generateVector())

	retriever := NewSimpleRetriever()

	for _, point := range points {
		retriever.Insert(point.GetName(), point)
	}

	// Get the top 10 most similar documents.
	docs := retriever.GetRelevant(query, 10, func(id string, embedding *Embedding) bool {
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
