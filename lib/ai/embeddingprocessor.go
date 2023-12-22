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
	"context"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"

	embeddingpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/embedding/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	embeddinglib "github.com/gravitational/teleport/lib/ai/embedding"
	"github.com/gravitational/teleport/lib/services"
	streamutils "github.com/gravitational/teleport/lib/utils/stream"
)

// maxEmbeddingAPISize is the maximum number of entities that can be embedded in a single API call.
const maxEmbeddingAPISize = 1000

// Embeddings implements the minimal interface used by the Embedding processor.
type Embeddings interface {
	// GetAllEmbeddings returns all embeddings.
	GetAllEmbeddings(ctx context.Context) stream.Stream[*embeddinglib.Embedding]

	// UpsertEmbedding creates or update a single ai.Embedding in the backend.
	UpsertEmbedding(ctx context.Context, embedding *embeddinglib.Embedding) (*embeddinglib.Embedding, error)
}

// MarshalEmbedding marshals the ai.Embedding resource to binary ProtoBuf.
func MarshalEmbedding(embedding *embeddinglib.Embedding) ([]byte, error) {
	data, err := proto.Marshal((*embeddingpb.Embedding)(embedding))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalEmbedding unmarshals binary ProtoBuf into an ai.Embedding resource.
func UnmarshalEmbedding(bytes []byte) (*embeddinglib.Embedding, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing embedding data")
	}
	var embedding embeddingpb.Embedding
	err := proto.Unmarshal(bytes, &embedding)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return (*embeddinglib.Embedding)(&embedding), nil
}

// EmbeddingHashMatches returns true if the hash of the embedding matches the
// given hash.
func EmbeddingHashMatches(embedding *embeddinglib.Embedding, hash embeddinglib.Sha256Hash) bool {
	if len(embedding.EmbeddedHash) != 32 {
		return false
	}

	return *(*embeddinglib.Sha256Hash)(embedding.EmbeddedHash) == hash
}

// BatchReducer is a helper that processes data in batches.
type BatchReducer[T, V any] struct {
	data      []T
	batchSize int
	processFn func(ctx context.Context, data []T) (V, error)
}

// NewBatchReducer is a BatchReducer constructor.
func NewBatchReducer[T, V any](processFn func(ctx context.Context, data []T) (V, error), batchSize int) *BatchReducer[T, V] {
	return &BatchReducer[T, V]{
		data:      make([]T, 0),
		batchSize: batchSize,
		processFn: processFn,
	}
}

// Add adds a new item to the batch. If the batch is full, it will be processed
// and the result will be returned. Otherwise, a zero value will be returned.
// Finalize must be called to process the remaining data in the batch.
func (b *BatchReducer[T, V]) Add(ctx context.Context, data T) (V, error) {
	b.data = append(b.data, data)
	if len(b.data) >= b.batchSize {
		val, err := b.processFn(ctx, b.data)
		b.data = b.data[:0]
		return val, trace.Wrap(err)
	}

	var def V
	return def, nil
}

// Finalize processes the remaining data in the batch and returns the result.
func (b *BatchReducer[T, V]) Finalize(ctx context.Context) (V, error) {
	if len(b.data) > 0 {
		val, err := b.processFn(ctx, b.data)
		b.data = b.data[:0]
		return val, trace.Wrap(err)
	}

	var def V
	return def, nil
}

// EmbeddingProcessorConfig is the configuration for EmbeddingProcessor.
type EmbeddingProcessorConfig struct {
	AIClient            embeddinglib.Embedder
	EmbeddingSrv        Embeddings
	EmbeddingsRetriever *SimpleRetriever
	NodeSrv             *services.UnifiedResourceCache
	Log                 logrus.FieldLogger
	Jitter              retryutils.Jitter
}

// EmbeddingProcessor is responsible for processing nodes, generating embeddings
// and storing their embeddings in the backend.
type EmbeddingProcessor struct {
	aiClient            embeddinglib.Embedder
	embeddingSrv        Embeddings
	embeddingsRetriever *SimpleRetriever
	nodeSrv             *services.UnifiedResourceCache
	log                 logrus.FieldLogger
	jitter              retryutils.Jitter
}

// NewEmbeddingProcessor returns a new EmbeddingProcessor.
func NewEmbeddingProcessor(cfg *EmbeddingProcessorConfig) *EmbeddingProcessor {
	return &EmbeddingProcessor{
		aiClient:            cfg.AIClient,
		embeddingSrv:        cfg.EmbeddingSrv,
		embeddingsRetriever: cfg.EmbeddingsRetriever,
		nodeSrv:             cfg.NodeSrv,
		log:                 cfg.Log,
		jitter:              cfg.Jitter,
	}
}

// resourceStringPair is a helper struct that pairs a resource with a data string.
type resourceStringPair struct {
	resource types.Resource
	data     string
}

// mapProcessFn is a helper function that maps a slice of resourceStringPair,
// compute embeddings and return them as a slice.
func (e *EmbeddingProcessor) mapProcessFn(ctx context.Context, data []*resourceStringPair) ([]*embeddinglib.Embedding, error) {
	dataBatch := make([]string, 0, len(data))
	for _, pair := range data {
		dataBatch = append(dataBatch, pair.data)
	}

	embeddings, err := e.aiClient.ComputeEmbeddings(ctx, dataBatch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := make([]*embeddinglib.Embedding, 0, len(embeddings))
	for i, embedding := range embeddings {
		emb := embeddinglib.NewEmbedding(data[i].resource.GetKind(),
			data[i].resource.GetName(), embedding,
			embeddinglib.EmbeddingHash([]byte(data[i].data)),
		)
		results = append(results, emb)
	}

	return results, nil
}

// Run runs the EmbeddingProcessor.
func (e *EmbeddingProcessor) Run(ctx context.Context, initialDelay, period time.Duration) error {
	initTimer := time.NewTimer(initialDelay)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-initTimer.C:
			// Stop the timer after the initial delay.
			initTimer.Stop()
			e.process(ctx)
		case <-time.After(e.jitter(period)):
			e.process(ctx)
		}
	}
}

// process updates embeddings for all resources once.
func (e *EmbeddingProcessor) process(ctx context.Context) {
	batch := NewBatchReducer(e.mapProcessFn,
		maxEmbeddingAPISize, // Max batch size allowed by OpenAI API,
	)

	e.log.Debugf("embedding processor started")
	defer e.log.Debugf("embedding processor finished")

	embeddingsStream := e.embeddingSrv.GetAllEmbeddings(ctx)
	unifiedResources, err := e.nodeSrv.GetUnifiedResources(ctx)
	if err != nil {
		e.log.Debugf("embedding processor failed with error: %v", err)
		return
	}

	resources := make([]types.Resource, len(unifiedResources))
	for i, unifiedResource := range unifiedResources {
		resources[i] = unifiedResource
		unifiedResources[i] = nil
	}

	resourceStream := stream.Slice(resources)

	s := streamutils.NewZipStreams(
		resourceStream,
		embeddingsStream,
		// On new resource callback. Add the resource to the batch.
		func(resource types.Resource) error {
			resourceData, err := embeddinglib.SerializeResource(resource)
			if err != nil {
				return trace.Wrap(err)
			}
			vectors, err := batch.Add(ctx, &resourceStringPair{resource, string(resourceData)})
			if err != nil {
				return trace.Wrap(err)
			}
			if err := e.upsertEmbeddings(ctx, vectors); err != nil {
				return trace.Wrap(err)
			}

			return nil
		},
		// On equal resource callback. Check if the resource's embedding hash matches
		// the one in the backend. If not, add the resource to the batch.
		func(resource types.Resource, embedding *embeddinglib.Embedding) error {
			resourceData, err := embeddinglib.SerializeResource(resource)
			if err != nil {
				return trace.Wrap(err)
			}
			resourceHash := embeddinglib.EmbeddingHash(resourceData)

			if !EmbeddingHashMatches(embedding, resourceHash) {
				vectors, err := batch.Add(ctx, &resourceStringPair{resource, string(resourceData)})
				if err != nil {
					return trace.Wrap(err)
				}
				if err := e.upsertEmbeddings(ctx, vectors); err != nil {
					return trace.Wrap(err)
				}
			}
			return nil
		},
		// On compare keys callback. Compare the keys for iteration.
		func(resource types.Resource, embeddings *embeddinglib.Embedding) int {
			return strings.Compare(resource.GetName(), embeddings.GetEmbeddedID())
		},
	)

	if err := s.Process(); err != nil {
		e.log.Warnf("Failed to generate nodes embedding: %v", err)
	}

	// Process the remaining resources in the batch
	vectors, err := batch.Finalize(ctx)
	if err != nil {
		e.log.Warnf("Failed to add node to batch: %v", err)
		return
	}

	if err := e.upsertEmbeddings(ctx, vectors); err != nil {
		e.log.Warnf("Failed to upsert embeddings: %v", err)

	}

	if err := e.updateMemIndex(ctx); err != nil {
		e.log.Warnf("Failed to update memory index: %v", err)
	}
}

// updateMemIndex is a helper function that updates the in-memory index with the
// latest embeddings. The new index is created and then swapped with the old one.
func (e *EmbeddingProcessor) updateMemIndex(ctx context.Context) error {
	embeddingsIndex := NewSimpleRetriever()
	embeddingsStream := e.embeddingSrv.GetAllEmbeddings(ctx)

	for embeddingsStream.Next() {
		embedding := embeddingsStream.Item()
		if !embeddingsIndex.Insert(embedding.GetEmbeddedID(), embedding) {
			e.log.Warnf("Embeddings index is full, some resources can be missing")
			break
		}
	}

	if err := embeddingsStream.Done(); err != nil {
		return trace.Wrap(err)
	}

	e.embeddingsRetriever.Swap(embeddingsIndex)

	return nil
}

// upsertEmbeddings is a helper function that upserts the embeddings into the backend.
func (e *EmbeddingProcessor) upsertEmbeddings(ctx context.Context, rawEmbeddings []*embeddinglib.Embedding) error {
	// Store the new embeddings into the backend
	for _, embedding := range rawEmbeddings {
		_, err := e.embeddingSrv.UpsertEmbedding(ctx, embedding)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
