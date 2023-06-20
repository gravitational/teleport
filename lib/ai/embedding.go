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

package ai

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"

	embeddingpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/embedding/v1"
	"github.com/gravitational/teleport/lib/backend"
)

const (
	maxOpenAIEmbeddingsPerRequest = 1000
	// EmbeddingPeriod is the time between two embedding routines.
	// A seventh jitter is applied on the period.
	EmbeddingPeriod = time.Hour
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

// NewEmbedding is an Embedding constructor.
func NewEmbedding(kind, id string, vector Vector64, hash Sha256Hash) *Embedding {
	return &Embedding{
		EmbeddedKind: kind,
		EmbeddedId:   id,
		EmbeddedHash: hash[:],
		Vector:       vector,
	}
}

// Embedder is implemented for batch text embedding. Embedding can happen in
// place (with an embedding model for example) or be done by a remote embedding
// service like OpenAI.
type Embedder interface {
	// ComputeEmbeddings computes the embeddings of multiple strings.
	// The embedding list follows the input order (e.g. result[i] is the
	// embedding of input[i]).
	ComputeEmbeddings(ctx context.Context, input []string) ([]Vector64, error)
}

// ComputeEmbeddings taxes a map of nodes and calls openAI to generate
// embeddings for those nodes. ComputeEmbeddings is responsible for
// implementing a retry mechanism if the embedding computation is flaky.
func (client *Client) ComputeEmbeddings(ctx context.Context, input []string) ([]Vector64, error) {
	var results []Vector64
	for i := 0; maxOpenAIEmbeddingsPerRequest*i < len(input); i++ {
		result, err := client.computeEmbeddings(ctx, paginateInput(input, i, maxOpenAIEmbeddingsPerRequest))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, vector := range result {
			results = append(results, vector32to64(vector))
		}
	}
	return results, nil
}

func paginateInput(input []string, page, pageSize int) []string {
	begin := page * pageSize
	var end int
	if len(input) < (page+1)*pageSize {
		end = len(input)
	} else {
		end = (page + 1) * pageSize
	}
	return input[begin:end]
}

func vector32to64(vector32 Vector32) Vector64 {
	vector64 := make(Vector64, len(vector32))
	for i, dimension := range vector32 {
		vector64[i] = float64(dimension)
	}
	return vector64
}

// computeEmbeddings calls the openAI embedding model with the provided input.
// This function should not be called directly, use ComputeEmbeddings instead
// to ensure input is properly batched.
func (client *Client) computeEmbeddings(ctx context.Context, input []string) ([]Vector32, error) {
	req := openai.EmbeddingRequest{
		Input: input,
		Model: openai.AdaEmbeddingV2,
	}

	// Execute the query
	resp, err := client.svc.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result := make([]Vector32, len(input))
	for i, item := range resp.Data {
		result[i] = item.Embedding
	}
	return result, nil
}
