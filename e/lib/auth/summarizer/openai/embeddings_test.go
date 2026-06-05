package openai

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func newTestEmbeddingProvider() *EmbeddingProvider {
	p := &EmbeddingProvider{
		client:            &fakeClient{},
		modelResourceName: "test-model",
	}
	p.logger = slog.With(teleport.ComponentKey, "openai", "embedding_model", p.modelResourceName)
	return p
}

func TestGenerateEmbeddings(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	provider := newTestEmbeddingProvider()

	cases := []struct {
		name   string
		text   string
		assert func(t *testing.T, embeddings []float32, tokenCount int, err error)
	}{
		{
			name: "typical case",
			text: "hello world",
			assert: func(t *testing.T, embeddings []float32, tokenCount int, err error) {
				require.NoError(t, err)
				assert.Equal(t, []float32{0.1, 0.2, 0.3}, embeddings)
				assert.Equal(t, 2, tokenCount) // "hello world" = 2 words
			},
		},
		{
			name: "empty text",
			text: "",
			assert: func(t *testing.T, embeddings []float32, tokenCount int, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "client error",
			text: "cause an error",
			assert: func(t *testing.T, embeddings []float32, tokenCount int, err error) {
				require.Error(t, err)
				assert.Nil(t, embeddings)
				assert.Zero(t, tokenCount)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			embeddings, tokenCount, err := provider.GenerateEmbeddings(ctx, tc.text)
			tc.assert(t, embeddings, tokenCount, err)
		})
	}
}

func TestNewEmbeddingProviderValidation(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	secret := summarizerv1pb.InferenceSecretSpec_builder{Value: "test-key"}.Build()
	spec := &summarizerv1pb.OpenAIProvider{}

	cases := []struct {
		name    string
		cfg     EmbeddingProviderConfig
		wantErr string
	}{
		{
			name:    "missing embeddings spec",
			cfg:     EmbeddingProviderConfig{SecretSpec: secret, ModelResourceName: "m"},
			wantErr: "embeddings spec is required",
		},
		{
			name:    "missing secret spec",
			cfg:     EmbeddingProviderConfig{EmbeddingsSpec: spec, ModelResourceName: "m"},
			wantErr: "secret spec is required",
		},
		{
			name:    "missing model resource name",
			cfg:     EmbeddingProviderConfig{EmbeddingsSpec: spec, SecretSpec: secret},
			wantErr: "model resource name is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEmbeddingProvider(ctx, tc.cfg)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
