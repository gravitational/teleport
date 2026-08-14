package ragpipeline

import (
	"context"

	"github.com/gravitational/trace"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// EmbeddingGenerator generates vector embeddings for text.
type EmbeddingGenerator interface {
	// GenerateEmbeddings generates a vector embedding for the provided text.
	// Returns float32 slice representing the embedding vector.
	// Also returns the token count for the input text, which can be used for
	// monitoring and debugging purposes.
	GenerateEmbeddings(ctx context.Context, text string) ([]float32, int, error)
}

// ProseGenerator defines an interface for generating text suitable for embedding
// based on a session summary.
type ProseGenerator interface {
	// CondenseForEmbedding generates text suitable for embedding based on the session summary.
	CondenseForEmbedding(ctx context.Context, summary *summarizerv1pb.Summary) (string, error)
}

// Document represents content prepared for embedding and retrieval.
type Document struct {
	// ChunkIndex is the zero-based position of this chunk within the session's document set.
	ChunkIndex int
	// Content is the actual text that was embedded
	Content string
	// Embedding is the vector representation of the content
	Embedding []float32
}

// ProcessorConfig holds configuration for the embedding processor.
type ProcessorConfig struct {
	// ChunkerConfig controls how text is chunked before embedding
	ChunkerConfig ChunkerConfig
}

// DefaultProcessorConfig returns a ProcessorConfig with sensible defaults for RAG.
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		ChunkerConfig: defaultChunkerConfig(),
	}
}

// Processor orchestrates chunking, text building, and embedding generation for
// session summaries.
type Processor struct {
	chunker        *chunker
	embedder       EmbeddingGenerator
	proseGenerator ProseGenerator
}

// NewProcessorWithConfig creates a new embedding Processor with custom configuration.
func NewProcessorWithConfig(embedder EmbeddingGenerator, proseGenerator ProseGenerator, cfg ProcessorConfig) (*Processor, error) {
	if embedder == nil {
		return nil, trace.BadParameter("embedder is required")
	}
	if proseGenerator == nil {
		return nil, trace.BadParameter("proseGenerator is required")
	}
	c, err := newChunkerWithConfig(cfg.ChunkerConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Processor{
		chunker:        c,
		embedder:       embedder,
		proseGenerator: proseGenerator,
	}, nil
}

// ProcessSession generates embeddings for a session summary, producing one or
// more documents depending on the summary length.
func (p *Processor) ProcessSession(ctx context.Context, summary *summarizerv1pb.Summary) ([]Document, error) {
	if summary == nil {
		return nil, trace.BadParameter("summary is required")
	}

	sessionDocs, err := p.processSessionSummary(ctx, summary)
	if err != nil {
		return nil, trace.Wrap(err, "failed to process session summary")
	}

	return sessionDocs, nil
}

func (p *Processor) embedChunks(ctx context.Context, chunks []string) ([]Document, error) {
	docs := make([]Document, 0, len(chunks))

	for chunkIdx, content := range chunks {
		embedding, _, err := p.embedder.GenerateEmbeddings(ctx, content)
		if err != nil {
			return nil, trace.Wrap(err, "failed to generate embedding for chunk %d", chunkIdx)
		}

		docs = append(docs, Document{
			ChunkIndex: chunkIdx,
			Content:    content,
			Embedding:  embedding,
		})
	}

	return docs, nil
}

// processSessionSummary creates embeddings for the session-level summary.
// May create multiple documents if the summary is large.
func (p *Processor) processSessionSummary(ctx context.Context, summary *summarizerv1pb.Summary) ([]Document, error) {
	prose, err := p.proseGenerator.CondenseForEmbedding(ctx, summary)
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate prose for session summary")
	}

	textChunks, err := p.chunker.chunkText(prose)
	if err != nil {
		return nil, trace.Wrap(err, "failed to chunk session summary")
	}

	return p.embedChunks(ctx, textChunks)
}
