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
	"github.com/gravitational/trace"
	"github.com/kyroy/kdtree"

	"github.com/gravitational/teleport/lib/ai/embedding"
)

// Document is a embedding enriched with similarity score
type Document struct {
	*embedding.Embedding
	SimilarityScore float64
}

// KNNRetriever is a retriever that uses KNN to find relevant documents.
type KNNRetriever struct {
	tree    *kdtree.KDTree
	mapping map[string]*embedding.Embedding
	// vectorsDimension is the dimension of the vectors.
	// All vectors must have the same dimension.
	vectorsDimension int
}

// NewKNNRetriever returns a new KNNRetriever. It expects that all points
// have the same dimension and all vectors are normalized.
func NewKNNRetriever(points []*embedding.Embedding) (*KNNRetriever, error) {
	if len(points) == 0 {
		return nil, trace.BadParameter("no points provided")
	}
	expectedDimension := points[0].Dimensions()
	kpoints := make([]kdtree.Point, len(points))
	mapping := make(map[string]*embedding.Embedding, len(points))
	for i, point := range points {
		// Make sure that all points have the same dimension
		if point.Dimensions() != expectedDimension {
			return nil, trace.BadParameter("all points must have the same dimension")
		}
		kpoints[i] = point
		mapping[point.GetName()] = point
	}

	return &KNNRetriever{
		tree:             kdtree.New(kpoints),
		mapping:          mapping,
		vectorsDimension: expectedDimension,
	}, nil
}

// GetRelevant returns the k most relevant documents to the query
func (r *KNNRetriever) GetRelevant(query *embedding.Embedding, k int) []*Document {
	result := r.tree.KNN(query, k)
	relevant := make([]*Document, len(result))
	for i, item := range result {
		embedding := item.(*embedding.Embedding)
		// Ignore error. We've already checked that all points have the same dimension
		similarity, _ := calculateSimilarity(query.Vector, embedding.Vector)

		relevant[i] = &Document{
			Embedding:       embedding,
			SimilarityScore: similarity,
		}
	}
	return relevant
}

// Insert inserts a new point into the retriever
func (r *KNNRetriever) Insert(point *embedding.Embedding) error {
	if point.Dimensions() != r.vectorsDimension {
		return trace.BadParameter("point has wrong dimension")
	}
	r.tree.Insert(point)
	r.mapping[point.GetName()] = point

	return nil
}

// Remove removes an element from the retriever
func (r *KNNRetriever) Remove(name string) error {
	point, ok := r.mapping[name]
	if !ok {
		return trace.BadParameter("point %q not found", name)
	}

	delete(r.mapping, name)
	r.tree.Remove(kdtree.Point(point))

	return nil
}

// calculateSimilarity calculates the dot product/similarity between two normalized vectors.
func calculateSimilarity(v1, v2 []float64) (float64, error) {
	if len(v1) != len(v2) {
		return 0, trace.BadParameter("vectors must be the same length")
	}

	var result float64
	for i, val := range v1 {
		result += val * v2[i]
	}

	return result, nil
}
