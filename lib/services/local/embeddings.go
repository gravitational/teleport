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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/ai/embedding"
	"github.com/gravitational/teleport/lib/backend"
)

// EmbeddingsService implements the services.Embeddings interface.
type EmbeddingsService struct {
	log    *logrus.Entry
	jitter retryutils.Jitter
	backend.Backend
	clock clockwork.Clock
}

const (
	embeddingsPrefix = "embeddings"
	embeddingExpiry  = 30 * 24 * time.Hour // 30 days
)

// GetEmbedding looks up a single embedding by its name in the backend.
func (e EmbeddingsService) GetEmbedding(ctx context.Context, kind, resourceID string) (*embedding.Embedding, error) {
	result, err := e.Get(ctx, backend.NewKey(embeddingsPrefix, kind, resourceID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ai.UnmarshalEmbedding(result.Value)
}

// GetEmbeddings returns a stream of all embeddings
func (e EmbeddingsService) GetAllEmbeddings(ctx context.Context) stream.Stream[*embedding.Embedding] {
	startKey := backend.ExactKey(embeddingsPrefix)
	items := backend.StreamRange(ctx, e, startKey, backend.RangeEnd(startKey), 50)
	return stream.FilterMap(items, func(item backend.Item) (*embedding.Embedding, bool) {
		embedding, err := ai.UnmarshalEmbedding(item.Value)
		if err != nil {
			e.log.Warnf("Skipping embedding at %s, failed to unmarshal: %v", item.Key, err)
			return nil, false
		}
		return embedding, true
	})
}

// GetEmbeddings returns a stream of embeddings for a given kind.
func (e EmbeddingsService) GetEmbeddings(ctx context.Context, kind string) stream.Stream[*embedding.Embedding] {
	startKey := backend.ExactKey(embeddingsPrefix, kind)
	items := backend.StreamRange(ctx, e, startKey, backend.RangeEnd(startKey), 50)
	return stream.FilterMap(items, func(item backend.Item) (*embedding.Embedding, bool) {
		embedding, err := ai.UnmarshalEmbedding(item.Value)
		if err != nil {
			e.log.Warnf("Skipping embedding at %s, failed to unmarshal: %v", item.Key, err)
			return nil, false
		}
		return embedding, true
	})
}

// UpsertEmbedding creates or update a single ai.Embedding in the backend.
func (e EmbeddingsService) UpsertEmbedding(ctx context.Context, embedding *embedding.Embedding) (*embedding.Embedding, error) {
	value, err := ai.MarshalEmbedding(embedding)
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

// NewEmbeddingsService is a constructor for the EmbeddingsService.
func NewEmbeddingsService(b backend.Backend) *EmbeddingsService {
	return &EmbeddingsService{
		log:     logrus.WithFields(logrus.Fields{trace.Component: "Embeddings"}),
		jitter:  retryutils.NewFullJitter(),
		Backend: b,
		clock:   clockwork.NewRealClock(),
	}
}

// embeddingItemKey builds the backend item key for a given ai.Embedding.
func embeddingItemKey(embedding *embedding.Embedding) []byte {
	return backend.NewKey(embeddingsPrefix, embedding.GetName())
}
