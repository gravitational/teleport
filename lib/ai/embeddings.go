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
	"context"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/defaults"
	embeddingpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/embedding/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	streamutils "github.com/gravitational/teleport/lib/utils/stream"
)

// maxEmbeddingAPISize is the maximum number of entities that can be embedded in a single API call.
const maxEmbeddingAPISize = 1000

// Embeddings implements the minimal interface used by the Embedding processor.
type Embeddings interface {
	// GetEmbeddings returns all embeddings for a given kind.
	GetEmbeddings(ctx context.Context, kind string) stream.Stream[*Embedding]
	// UpsertEmbedding creates or update a single ai.Embedding in the backend.
	UpsertEmbedding(ctx context.Context, embedding *Embedding) (*Embedding, error)
}

// NodesStreamGetter is a service that gets nodes.
type NodesStreamGetter interface {
	// GetNodeStream returns a list of registered servers.
	GetNodeStream(ctx context.Context, namespace string) stream.Stream[types.Server]
}

// MarshalEmbedding marshals the ai.Embedding resource to binary ProtoBuf.
func MarshalEmbedding(embedding *Embedding) ([]byte, error) {
	data, err := proto.Marshal((*embeddingpb.Embedding)(embedding))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// UnmarshalEmbedding unmarshals binary ProtoBuf into an ai.Embedding resource.
func UnmarshalEmbedding(bytes []byte) (*Embedding, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing embedding data")
	}
	var embedding embeddingpb.Embedding
	err := proto.Unmarshal(bytes, &embedding)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return (*Embedding)(&embedding), nil
}

// EmbeddingHashMatches returns true if the hash of the embedding matches the
// given hash.
func EmbeddingHashMatches(embedding *Embedding, hash Sha256Hash) bool {
	if len(embedding.EmbeddedHash) != 32 {
		return false
	}

	return *(*Sha256Hash)(embedding.EmbeddedHash) == hash
}

// SerializeNode converts a type.Server into text ready to be fed to an
// embedding model. The YAML serialization function was chosen over JSON and
// CSV as it provided better results.
func SerializeNode(node types.Server) ([]byte, error) {
	a := struct {
		Name    string            `yaml:"name"`
		Kind    string            `yaml:"kind"`
		SubKind string            `yaml:"subkind"`
		Labels  map[string]string `yaml:"labels"`
	}{
		// Create artificial Name file for the node "name". Using node.GetName() as Name seems to confuse the model.
		Name:    node.GetHostname(),
		Kind:    types.KindNode,
		SubKind: node.GetSubKind(),
		Labels:  node.GetAllLabels(),
	}
	text, err := yaml.Marshal(&a)
	return text, trace.Wrap(err)
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
	AIClient            Embedder
	EmbeddingSrv        Embeddings
	EmbeddingsRetriever *SimpleRetriever
	NodeSrv             NodesStreamGetter
	Log                 logrus.FieldLogger
	Jitter              retryutils.Jitter
}

// EmbeddingProcessor is responsible for processing nodes, generating embeddings
// and storing their embeddings in the backend.
type EmbeddingProcessor struct {
	aiClient            Embedder
	embeddingSrv        Embeddings
	embeddingsRetriever *SimpleRetriever
	nodeSrv             NodesStreamGetter
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

// nodeStringPair is a helper struct that pairs a node with a data string.
type nodeStringPair struct {
	node types.Server
	data string
}

// mapProcessFn is a helper function that maps a slice of nodeStringPair,
// compute embeddings and return them as a slice of ai.Embedding.
func (e *EmbeddingProcessor) mapProcessFn(ctx context.Context, data []*nodeStringPair) ([]*Embedding, error) {
	dataBatch := make([]string, 0, len(data))
	for _, pair := range data {
		dataBatch = append(dataBatch, pair.data)
	}

	embeddings, err := e.aiClient.ComputeEmbeddings(ctx, dataBatch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := make([]*Embedding, 0, len(embeddings))
	for i, embedding := range embeddings {
		emb := NewEmbedding(types.KindNode,
			data[i].node.GetName(), embedding,
			EmbeddingHash([]byte(data[i].data)),
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

func (e *EmbeddingProcessor) process(ctx context.Context) {
	batch := NewBatchReducer(e.mapProcessFn,
		maxEmbeddingAPISize, // Max batch size allowed by OpenAI API,
	)

	e.log.Debugf("embedding processor started")
	defer e.log.Debugf("embedding processor finished")

	embeddingsStream := e.embeddingSrv.GetEmbeddings(ctx, types.KindNode)
	nodesStream := e.nodeSrv.GetNodeStream(ctx, defaults.Namespace)

	s := streamutils.NewZipStreams(
		nodesStream,
		embeddingsStream,
		// On new node callback. Add the node to the batch.
		func(node types.Server) error {
			nodeData, err := SerializeNode(node)
			if err != nil {
				return trace.Wrap(err)
			}
			vectors, err := batch.Add(ctx, &nodeStringPair{node, string(nodeData)})
			if err != nil {
				return trace.Wrap(err)
			}
			if err := e.upsertEmbeddings(ctx, vectors); err != nil {
				return trace.Wrap(err)
			}

			return nil
		},
		// On equal node callback. Check if the node's embedding hash matches
		// the one in the backend. If not, add the node to the batch.
		func(node types.Server, embedding *Embedding) error {
			nodeData, err := SerializeNode(node)
			if err != nil {
				return trace.Wrap(err)
			}
			nodeHash := EmbeddingHash(nodeData)

			if !EmbeddingHashMatches(embedding, nodeHash) {
				vectors, err := batch.Add(ctx, &nodeStringPair{node, string(nodeData)})
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
		func(node types.Server, embeddings *Embedding) int {
			if node.GetName() == embeddings.GetName() {
				return 0
			}

			return strings.Compare(node.GetName(), embeddings.GetName())
		},
	)

	if err := s.Process(); err != nil {
		e.log.Warnf("Failed to generate nodes embedding: %v", err)
	}

	// Process the remaining nodes in the batch
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
	embeddingsStream := e.embeddingSrv.GetEmbeddings(ctx, types.KindNode)

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
func (e *EmbeddingProcessor) upsertEmbeddings(ctx context.Context, rawEmbeddings []*Embedding) error {
	// Store the new embeddings into the backend
	for _, embedding := range rawEmbeddings {
		_, err := e.embeddingSrv.UpsertEmbedding(ctx, embedding)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
