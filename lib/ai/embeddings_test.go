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
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKNNRetriever_GetRelevant(t *testing.T) {
	t.Parallel()

	// Generate random vector. The seed is fixed, so the results are deterministic.
	randGen := rand.New(rand.NewSource(42))

	generateVector := func() []float64 {
		const testVectorDimension = 100
		// generate random vector
		// reduce the dimensionality to 100
		vec := make([]float64, testVectorDimension)
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
		points[i] = &Embedding{
			Vector:  generateVector(),
			Name:    strconv.Itoa(i),
			Content: strconv.Itoa(i),
		}
	}

	// Create a query.
	query := &Embedding{
		Vector: generateVector(),
	}

	retriever, err := NewKNNRetriever(points)
	require.NoError(t, err)

	// Get the top 10 most similar documents.
	docs := retriever.GetRelevant(query, 10)
	require.Len(t, docs, 10)

	expectedResults := []int{57, 92, 95, 49, 33, 56, 30, 99, 90, 47}
	expectedSimilarities := []float64{0.80405, 0.79051, 0.78161, 0.78159,
		0.77655, 0.77374, 0.77306, 0.76688, 0.76634, 0.76458}

	for i, result := range docs {
		require.Equal(t, strconv.Itoa(expectedResults[i]), result.Name, "expected order is wrong")
		require.InDelta(t, expectedSimilarities[i], result.SimilarityScore, 10e-6, "similarity score is wrong")
	}
}

func TestKNNRetriever_Insert(t *testing.T) {
	t.Parallel()

	points := []*Embedding{
		{
			Vector:  []float64{1, 2, 3},
			Name:    "1",
			Content: "1",
		},
		{
			Vector:  []float64{4, 5, 6},
			Name:    "2",
			Content: "2",
		},
	}

	retriever, err := NewKNNRetriever(points)
	require.NoError(t, err)

	docs1 := retriever.GetRelevant(&Embedding{
		Vector: []float64{7, 8, 9},
	}, 10)
	require.Len(t, docs1, 2)

	err = retriever.Insert(&Embedding{
		Vector:  []float64{7, 8, 9},
		Name:    "3",
		Content: "3",
	})
	require.NoError(t, err)

	docs2 := retriever.GetRelevant(&Embedding{
		Vector: []float64{7, 8, 9},
	}, 10)
	require.Len(t, docs2, 3)
}

func TestKNNRetriever_Remove(t *testing.T) {
	t.Parallel()

	points := []*Embedding{
		{
			Vector:  []float64{1, 2, 3},
			Name:    "1",
			Content: "1",
		},
		{
			Vector:  []float64{4, 5, 6},
			Name:    "2",
			Content: "2",
		},
		{
			Vector:  []float64{7, 8, 9},
			Name:    "3",
			Content: "3",
		},
	}

	retriever, err := NewKNNRetriever(points)
	require.NoError(t, err)

	docs1 := retriever.GetRelevant(&Embedding{
		Vector: []float64{7, 8, 9},
	}, 10)

	require.Len(t, docs1, 3)

	err = retriever.Remove("2")
	require.NoError(t, err)

	docs2 := retriever.GetRelevant(&Embedding{
		Vector: []float64{7, 8, 9},
	}, 10)
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
func normalize(v []float64) []float64 {
	norm := L2norm(v)
	result := make([]float64, len(v))
	for i, value := range v {
		result[i] = value / norm
	}
	return result
}
