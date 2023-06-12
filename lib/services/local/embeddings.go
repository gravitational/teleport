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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// EmbeddingsService implements the services.Embeddings interface.
type EmbeddingsService struct {
	log    *logrus.Entry
	jitter retryutils.Jitter
	backend.Backend
	client ai.Embedder
	clock  clockwork.Clock
}

const (
	embeddingsPrefix = "embeddings"
	embeddingExpiry  = 30 * 24 * time.Hour // 30 days
)

// GetEmbedding looks up a single embedding by its name in the backend.
func (e EmbeddingsService) GetEmbedding(ctx context.Context, kind, resourceID string) (*ai.Embedding, error) {
	result, err := e.Get(ctx, backend.Key(embeddingsPrefix, kind, resourceID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalEmbedding(result.Value)
}

// GetEmbeddings returns all embeddings for a given kind.
func (e EmbeddingsService) GetEmbeddings(ctx context.Context, kind string) ([]*ai.Embedding, error) {
	startKey := backend.Key(embeddingsPrefix, kind)
	result, err := e.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	embeddings := make([]*ai.Embedding, len(result.Items))
	var embedding *ai.Embedding
	if len(result.Items) == 0 {
		return nil, nil
	}
	for i, item := range result.Items {
		embedding, err = services.UnmarshalEmbedding(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		embeddings[i] = embedding
	}
	return embeddings, nil
}

// UpsertEmbedding creates or update a single ai.Embedding in the backend.
func (e EmbeddingsService) UpsertEmbedding(ctx context.Context, embedding *ai.Embedding) (*ai.Embedding, error) {
	value, err := services.MarshalEmbedding(embedding)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = e.Put(ctx, backend.Item{
		Key:     embeddingItemKey(embedding),
		Value:   value,
		Expires: e.clock.Now().Add(embeddingExpiry),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return embedding, nil
}

// Embed takes a resource textual representation, checks if the resource
// already has an up-to-date embedding stored in the backend, and computes
// a new embedding otherwise. The newly computed embedding is stored in
// the backend.
func (e EmbeddingsService) Embed(ctx context.Context, kind string, resources map[string][]byte) ([]*ai.Embedding, error) {

	// Lookup if there are embeddings in the backend for this node
	// and the hash matches
	embeddingsFromCache := make([]*ai.Embedding, 0)
	toEmbed := make(map[string][]byte)
	for name, data := range resources {
		existingEmbedding, err := e.GetEmbedding(ctx, kind, name)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if err == nil {
			if embeddingHashMatches(existingEmbedding, e.hash(data)) {
				embeddingsFromCache = append(embeddingsFromCache, existingEmbedding)
				continue
			}
		}
		toEmbed[name] = data
	}

	// Convert to a list but keep track of the order so that we know which
	// input maps to which resource.
	keys := make([]string, 0, len(toEmbed))
	input := make([]string, len(toEmbed))

	for key := range toEmbed {
		keys = append(keys, key)
	}

	for i, key := range keys {
		input[i] = string(toEmbed[key])
	}

	response, err := e.client.ComputeEmbeddings(ctx, input)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newEmbeddings := make([]*ai.Embedding, 0, len(response))
	for i, vector := range response {
		newEmbeddings = append(newEmbeddings, ai.NewEmbedding(kind, keys[i], vector, e.hash(resources[keys[i]])))
	}

	// Store the new embeddings into the backend
	for _, embedding := range newEmbeddings {
		_, err := e.UpsertEmbedding(ctx, embedding)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return append(embeddingsFromCache, newEmbeddings...), nil
}

// NewEmbeddingsService is a constructor for the EmbeddingsService.
func NewEmbeddingsService(b backend.Backend, embedder ai.Embedder) *EmbeddingsService {
	return &EmbeddingsService{
		log:     logrus.WithFields(logrus.Fields{trace.Component: "Embeddings"}),
		jitter:  retryutils.NewFullJitter(),
		Backend: b,
		client:  embedder,
		clock:   clockwork.NewRealClock(),
	}
}

// embeddingItemKey builds the backend item key for a given ai.Embedding.
func embeddingItemKey(embedding *ai.Embedding) []byte {
	return backend.Key(embeddingsPrefix, embedding.GetName())
}

func embeddingHashMatches(embedding *ai.Embedding, hash ai.Sha256Hash) bool {
	if len(embedding.EmbeddedHash) != 32 {
		return false
	}

	return *(*ai.Sha256Hash)(embedding.EmbeddedHash) == hash
}

func (e EmbeddingsService) hash(data []byte) ai.Sha256Hash {
	return sha256.Sum256(data)
}
