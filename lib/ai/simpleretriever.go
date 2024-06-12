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
	"sort"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/ai/embedding"
)

// Document is a embedding enriched with similarity score
type Document struct {
	*embedding.Embedding
	SimilarityScore float64
}

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

// SimpleRetriever is a simple implementation of embeddings retriever.
// It stores all the embeddings in memory and retrieves the k nearest neighbors
// by iterating over all the embeddings. Do not use for large datasets.
type SimpleRetriever struct {
	embeddings map[string]*embedding.Embedding
	maxSize    int
	mtx        sync.Mutex
}

func NewSimpleRetriever() *SimpleRetriever {
	return &SimpleRetriever{
		embeddings: make(map[string]*embedding.Embedding),
		maxSize:    1_000, // keep the number low to avoid OOM
	}
}

// Insert adds the embedding to the retriever. If the retriever is full, the
// embedding is not added and false is returned.
func (r *SimpleRetriever) Insert(id string, embedding *embedding.Embedding) bool {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if len(r.embeddings) >= r.maxSize {
		return false
	}
	r.embeddings[id] = embedding
	return true
}

// Remove removes the embedding from the retriever by ID.
func (r *SimpleRetriever) Remove(id string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	delete(r.embeddings, id)
}

// Swap replaces the embeddings in the retriever with the embeddings from the
// provided retriever.
// The mutex is acquired for the receiver, but not for the provided retriever.
func (r *SimpleRetriever) Swap(s *SimpleRetriever) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.embeddings = s.embeddings
	r.maxSize = s.maxSize
}

// FilterFn is a function that filters out embeddings.
// If the function returns false, the embedding is filtered out.
type FilterFn func(id string, embedding *embedding.Embedding) bool

// GetRelevant returns the k nearest neighbors to the query embedding.
// If a filter is provided, only the embeddings that pass the filter are considered.
func (r *SimpleRetriever) GetRelevant(query *embedding.Embedding, k int, filter FilterFn) []*Document {
	// Replace with priority queue if k is large.
	results := make([]*Document, 0, k)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	// Find the k nearest neighbors
	for id, embedding := range r.embeddings {
		// Skip if the document is filtered out
		if filter != nil && !filter(id, embedding) {
			continue
		}

		// Calculate the similarity score
		similarity, _ := calculateSimilarity(query.Vector, embedding.Vector)
		// If the results slice smaller than the k, add the element to the results
		if len(results) < k {
			results = append(results, &Document{
				Embedding:       embedding,
				SimilarityScore: similarity,
			})

			// Sort to preserve the invariant - the result slice is sorted by
			// similarity score
			sort.Slice(results, func(i, j int) bool {
				return results[i].SimilarityScore > results[j].SimilarityScore
			})
		} else if similarity > results[len(results)-1].SimilarityScore {
			// If the element is more relevant than the least similar element,
			// add it to the result slice
			results[len(results)-1] = &Document{
				Embedding:       embedding,
				SimilarityScore: similarity,
			}

			// Sort to preserve the invariant - the result slice is sorted by
			// similarity score
			sort.Slice(results, func(i, j int) bool {
				return results[i].SimilarityScore > results[j].SimilarityScore
			})
		}
	}

	// Return the results sorted by similarity score.
	return results
}
