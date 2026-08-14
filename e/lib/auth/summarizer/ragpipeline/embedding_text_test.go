package ragpipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// fakeEmbedder is a test double for EmbeddingGenerator.
type fakeEmbedder struct {
	embedding []float32
	err       error
}

func (f *fakeEmbedder) GenerateEmbeddings(_ context.Context, _ string) ([]float32, int, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.embedding, len(f.embedding), nil
}

// fakeProser is a test double for ProseGenerator.
type fakeProser struct {
	prose string
	err   error
}

func (f *fakeProser) CondenseForEmbedding(_ context.Context, _ *summarizerv1pb.Summary) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.prose, nil
}

func TestProcessSession(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	embedding := []float32{0.1, 0.2, 0.3}

	tests := []struct {
		name     string
		cfg      *ProcessorConfig // nil uses DefaultProcessorConfig
		summary  *summarizerv1pb.Summary
		proser   *fakeProser
		embedder *fakeEmbedder
		wantErr  string
		check    func(t *testing.T, docs []Document)
	}{
		{
			name:     "nil summary returns bad parameter error",
			summary:  nil,
			proser:   &fakeProser{},
			embedder: &fakeEmbedder{},
			wantErr:  "summary is required",
		},
		{
			name:     "proser error is propagated",
			summary:  summarizerv1pb.Summary_builder{SessionId: "sess-1"}.Build(),
			proser:   &fakeProser{err: errors.New("prose generation failed")},
			embedder: &fakeEmbedder{embedding: embedding},
			wantErr:  "prose generation failed",
		},
		{
			name:     "embedder error is propagated",
			summary:  summarizerv1pb.Summary_builder{SessionId: "sess-1"}.Build(),
			proser:   &fakeProser{prose: "some text"},
			embedder: &fakeEmbedder{err: errors.New("embedding failed")},
			wantErr:  "embedding failed",
		},
		{
			name:     "empty prose produces no documents",
			summary:  summarizerv1pb.Summary_builder{SessionId: "sess-1"}.Build(),
			proser:   &fakeProser{prose: ""},
			embedder: &fakeEmbedder{embedding: embedding},
			check: func(t *testing.T, docs []Document) {
				assert.Empty(t, docs)
			},
		},
		{
			name:     "prose within token limit produces one document",
			summary:  summarizerv1pb.Summary_builder{SessionId: "sess-1"}.Build(),
			proser:   &fakeProser{prose: "Short summary text."},
			embedder: &fakeEmbedder{embedding: embedding},
			check: func(t *testing.T, docs []Document) {
				require.Len(t, docs, 1)
				assert.Equal(t, 0, docs[0].ChunkIndex)
				assert.Equal(t, "Short summary text.", docs[0].Content)
				assert.Equal(t, embedding, docs[0].Embedding)
			},
		},
		{
			// teleportSentence is 35 tokens; max=10 with overlap=2 forces chunking,
			// producing one document per chunk each carrying the same embedding vector.
			name: "prose exceeding token limit produces one document per chunk",
			cfg: &ProcessorConfig{ChunkerConfig: ChunkerConfig{
				MaxTokensPerChunk: 10,
				OverlapTokens:     2,
			}},
			summary:  summarizerv1pb.Summary_builder{SessionId: "sess-1"}.Build(),
			proser:   &fakeProser{prose: teleportSentence},
			embedder: &fakeEmbedder{embedding: embedding},
			check: func(t *testing.T, docs []Document) {
				assert.Greater(t, len(docs), 1)
				for i, doc := range docs {
					assert.Equal(t, i, doc.ChunkIndex)
					assert.NotEmpty(t, doc.Content)
					assert.Equal(t, embedding, doc.Embedding)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultProcessorConfig()
			if tt.cfg != nil {
				cfg = *tt.cfg
			}
			p, err := NewProcessorWithConfig(tt.embedder, tt.proser, cfg)
			require.NoError(t, err)

			docs, err := p.ProcessSession(ctx, tt.summary)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, docs)
		})
	}
}
