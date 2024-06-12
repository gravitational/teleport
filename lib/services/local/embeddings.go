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

package local

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
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
	result, err := e.Get(ctx, backend.Key(embeddingsPrefix, kind, resourceID))
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
		log:     logrus.WithFields(logrus.Fields{teleport.ComponentKey: "Embeddings"}),
		jitter:  retryutils.NewFullJitter(),
		Backend: b,
		clock:   clockwork.NewRealClock(),
	}
}

// embeddingItemKey builds the backend item key for a given ai.Embedding.
func embeddingItemKey(embedding *embedding.Embedding) []byte {
	return backend.Key(embeddingsPrefix, embedding.GetName())
}
