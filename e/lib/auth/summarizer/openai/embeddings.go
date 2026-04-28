package openai

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// EmbeddingProvider is an OpenAI provider that generates text embeddings.
type EmbeddingProvider struct {
	client            Client
	logger            *slog.Logger
	modelResourceName string
	openAIModel       string
}

// EmbeddingProviderConfig holds the configuration for the OpenAI embedding provider.
type EmbeddingProviderConfig struct {
	// EmbeddingsSpec is the OpenAI provider specification for embeddings.
	EmbeddingsSpec *summarizerv1pb.OpenAIProvider
	// SecretSpec is the inference secret specification.
	SecretSpec *summarizerv1pb.InferenceSecretSpec
	// ClientFactory is used to create OpenAI clients. Can be overridden for
	// testing. Defaults to a production implementation.
	ClientFactory ClientFactory
	// ModelResourceName is the name of an embedding model this configuration is
	// derived from.
	ModelResourceName string
}

// NewEmbeddingProvider creates a new OpenAI embedding provider.
func NewEmbeddingProvider(ctx context.Context, cfg EmbeddingProviderConfig) (*EmbeddingProvider, error) {
	if cfg.EmbeddingsSpec == nil {
		return nil, trace.BadParameter("embeddings spec is required")
	}
	if cfg.SecretSpec == nil {
		return nil, trace.BadParameter("secret spec is required")
	}
	if cfg.ModelResourceName == "" {
		return nil, trace.BadParameter("model resource name is required")
	}

	if cfg.EmbeddingsSpec.GetOpenaiModelId() == "" {
		return nil, trace.BadParameter("openai model id is required")
	}

	clientFactory := cfg.ClientFactory
	if clientFactory == nil {
		clientFactory = defaultClientFactory{}
	}

	clientOptions := []option.RequestOption{option.WithAPIKey(cfg.SecretSpec.GetValue())}
	baseURL := cfg.EmbeddingsSpec.GetBaseUrl()
	if baseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(baseURL))
	}
	client := clientFactory.NewClient(clientOptions...)

	logger := slog.With(teleport.ComponentKey, "openai", "embedding_model", cfg.ModelResourceName)
	return &EmbeddingProvider{
		client:            client,
		logger:            logger,
		openAIModel:       cfg.EmbeddingsSpec.GetOpenaiModelId(),
		modelResourceName: cfg.ModelResourceName,
	}, nil
}

// embeddingDimensions is the output dimension requested from MRL-capable models.
// Requesting fewer dimensions at the API level is more accurate than post-hoc
// truncation: the model packs the most useful information into the first N dims.
const embeddingDimensions = 1024

// mrlCapableModels is the set of OpenAI model IDs that support the Dimensions
// parameter via Matryoshka Representation Learning. Models not in this set
// (legacy models, custom fine-tunes, or third-party compatible endpoints) do
// not accept the parameter and must use the default request shape.
var mrlCapableModels = map[string]bool{
	"text-embedding-3-small": true,
	"text-embedding-3-large": true,
}

// GenerateEmbeddings generates vector embeddings for the given text using the
// configured OpenAI embedding model.
func (p *EmbeddingProvider) GenerateEmbeddings(ctx context.Context, text string) ([]float32, int, error) {
	if text == "" {
		return nil, 0, trace.BadParameter("input text is required")
	}
	p.logger.DebugContext(ctx, "Generating embeddings")

	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{OfString: openai.String(text)},
		Model: p.openAIModel,
	}
	if mrlCapableModels[p.openAIModel] {
		params.Dimensions = openai.Int(embeddingDimensions)
	}

	rsp, err := p.client.GenerateEmbeddings(ctx, params)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}

	if len(rsp.Data) == 0 || len(rsp.Data[0].Embedding) == 0 {
		return nil, 0, trace.BadParameter("no embedding returned in response")
	}

	p.logger.DebugContext(ctx, "Embeddings generated", "dimension", len(rsp.Data[0].Embedding), "input_tokens", rsp.Usage.TotalTokens)
	return convertToFloat32Slice(rsp.Data[0].Embedding), int(rsp.Usage.TotalTokens), nil
}

func convertToFloat32Slice(input []float64) []float32 {
	output := make([]float32, len(input))
	for i, v := range input {
		output[i] = float32(v)
	}
	return output
}
