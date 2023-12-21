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

package embedding

import (
	"context"
	"crypto/sha256"

	embeddingpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/embedding/v1"
	"github.com/gravitational/teleport/lib/backend"
)

// EmbeddingHash is the hash function that should be used to compute embedding
// hashes.
var EmbeddingHash = sha256.Sum256

// Sha256Hash is the hash of the embedded content. This hash allows to detect if
// the embedding is still up-to-date or if the content changed and the resource
// must be re-embedded.
type Sha256Hash = [sha256.Size]byte

// Vector32 is an array of float64 that contains the result of the
// embedding process. OpenAI client returns []float32, hence Vector32 is the
// main type for handling vector data.
type Vector32 = []float32

// Vector64 is an array of float64 that contains the result of the embedding
// process. While OpenAI returns us 32-bit floats, the vector index uses methods
// requiring 64-bit floats.
type Vector64 = []float64

// Embedder is implemented for batch text embedding. Embedding can happen in
// place (with an embedding model, for example) or be done by a remote embedding
// service like OpenAI.
type Embedder interface {
	// ComputeEmbeddings computes the embeddings of multiple strings.
	// The embedding list follows the input order (e.g., result[i] is the
	// embedding of input[i]).
	ComputeEmbeddings(ctx context.Context, input []string) ([]Vector64, error)
}

// Embedding contains a Teleport resource embedding. Embeddings are small semantic
// representations of larger and more complex data. Embeddings can be compared,
// the smaller the distance between two vectors, the closer the concepts are.
// Teleport Assist embeds resources to perform semantic search.
// The Embedding is named after the embedded resource id and kind. For example
// the SSH node "bastion-01" has the embedding "node/bastion-01".
type Embedding embeddingpb.Embedding

// GetEmbeddedKind returns the kind of the resource that was embedded.
func (e *Embedding) GetEmbeddedKind() string {
	return e.EmbeddedKind
}

// GetName returns the Embedding name, composed of the embedded resource kind
// and the embedded resource ID.
func (e *Embedding) GetName() string {
	return e.EmbeddedKind + string(backend.Separator) + e.EmbeddedId
}

// GetEmbeddedID returns the ID of the resource that was embedded.
func (e *Embedding) GetEmbeddedID() string {
	return e.EmbeddedId
}

// GetVector returns the embedding vector
func (e *Embedding) GetVector() Vector64 {
	return e.Vector
}

// Dimensions returns the number of dimensions of the embedding
// Implements kdtree.Point interface
func (e *Embedding) Dimensions() int {
	return len(e.Vector)
}

// Dimension returns the value of the i-th dimension
// Implements kdtree.Point interface
func (e *Embedding) Dimension(i int) float64 {
	return e.Vector[i]
}

// NewEmbedding is an Embedding constructor.
func NewEmbedding(kind, id string, vector Vector64, hash Sha256Hash) *Embedding {
	return &Embedding{
		EmbeddedKind: kind,
		EmbeddedId:   id,
		EmbeddedHash: hash[:],
		Vector:       vector,
	}
}

func Vector32to64(vector32 Vector32) Vector64 {
	vector64 := make(Vector64, len(vector32))
	for i, dimension := range vector32 {
		vector64[i] = float64(dimension)
	}
	return vector64
}
