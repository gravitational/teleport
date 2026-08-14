package bedrock

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func TestGenerateEmbeddings(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

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
				var apiErr smithy.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, "dummy", apiErr.ErrorCode())
				assert.Nil(t, embeddings)
				assert.Zero(t, tokenCount)
			},
		},
		{
			name: "invalid json response",
			text: "invalid json response",
			assert: func(t *testing.T, embeddings []float32, tokenCount int, err error) {
				require.Error(t, err)
				assert.Nil(t, embeddings)
				assert.Zero(t, tokenCount)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := NewEmbeddingProvider(ctx, EmbeddingProviderConfig{
				Spec: summarizerv1pb.BedrockProvider_builder{
					Region:         "us-east-1",
					BedrockModelId: "amazon.titan-embed-text-v2:0",
				}.Build(),
				ModelResourceName: "titan-embeddings",
				ClientFactory:     &FakeClientFactory{Clock: clockwork.NewFakeClock()},
				AWSConfigCache:    cache,
			})
			require.NoError(t, err)

			embeddings, tokenCount, err := provider.GenerateEmbeddings(ctx, tc.text)
			tc.assert(t, embeddings, tokenCount, err)
		})
	}
}

func TestNewEmbeddingProviderValidation(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

	cases := []struct {
		name    string
		cfg     EmbeddingProviderConfig
		wantErr string
	}{
		{
			name:    "missing spec",
			cfg:     EmbeddingProviderConfig{ModelResourceName: "m", AWSConfigCache: cache},
			wantErr: "provider spec is required",
		},
		{
			name: "missing model resource name",
			cfg: EmbeddingProviderConfig{
				Spec:           summarizerv1pb.BedrockProvider_builder{Region: "us-east-1"}.Build(),
				AWSConfigCache: cache,
			},
			wantErr: "model resource name is required",
		},
		{
			name: "missing region",
			cfg: EmbeddingProviderConfig{
				Spec:              &summarizerv1pb.BedrockProvider{},
				ModelResourceName: "m",
				AWSConfigCache:    cache,
			},
			wantErr: "region is required",
		},
		{
			name: "missing aws config cache",
			cfg: EmbeddingProviderConfig{
				Spec:              summarizerv1pb.BedrockProvider_builder{Region: "us-east-1"}.Build(),
				ModelResourceName: "m",
			},
			wantErr: "AWS config cache is required",
		},
		{
			name: "placeholder region with empty env var",
			cfg: EmbeddingProviderConfig{
				Spec:              summarizerv1pb.BedrockProvider_builder{Region: "{{env.bedrock_region}}"}.Build(),
				ModelResourceName: "m",
				AWSConfigCache:    cache,
			},
			wantErr: "TELEPORT_BEDROCK_REGION environment variable is not set",
		},
		{
			name: "placeholder region with surrounding spaces and empty env var",
			cfg: EmbeddingProviderConfig{
				Spec:              summarizerv1pb.BedrockProvider_builder{Region: "{{ env.bedrock_region }}"}.Build(),
				ModelResourceName: "m",
				AWSConfigCache:    cache,
			},
			wantErr: "TELEPORT_BEDROCK_REGION environment variable is not set",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEmbeddingProvider(ctx, tc.cfg)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNewEmbeddingProvider_RegionExpansion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	cache, err := createCache()
	require.NoError(t, err)

	clientFactory := &FakeClientFactory{
		Clock: clockwork.NewFakeClock(),
		configValidation: func(cfg aws.Config) {
			assert.Equal(t, "eu-west-1", cfg.Region)
		},
	}
	provider, err := NewEmbeddingProvider(ctx, EmbeddingProviderConfig{
		Spec: summarizerv1pb.BedrockProvider_builder{
			BedrockModelId: "amazon.titan-embed-text-v2:0",
			Region:         "{{env.bedrock_region}}",
		}.Build(),
		ModelResourceName: "m",
		AWSConfigCache:    cache,
		ClientFactory:     clientFactory,
		EnvBedrockRegion:  "eu-west-1",
	})
	require.NoError(t, err)
	assert.NotNil(t, provider)
}
