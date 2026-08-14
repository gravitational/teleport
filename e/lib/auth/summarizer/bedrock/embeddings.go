package bedrock

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// EmbeddingProvider is an Amazon Bedrock provider that generates text embeddings
// using Bedrock's embedding models.
type EmbeddingProvider struct {
	bedrockModelID    string
	client            Client
	logger            *slog.Logger
	modelResourceName string
}

// EmbeddingProviderConfig holds the configuration for the Amazon Bedrock embedding
// provider.
type EmbeddingProviderConfig struct {
	Spec *summarizerv1pb.BedrockProvider
	// ClientFactory is used to create Bedrock clients. Can be overridden for
	// testing. Defaults to a production implementation.
	ClientFactory ClientFactory
	// ModelResourceName is the name of the embedding model resource this
	// configuration is derived from.
	ModelResourceName string
	AWSConfigCache    *awsconfig.Cache
	// EnvBedrockRegion, if non-empty, replaces the region in Spec when Spec.Region
	// is set to the {{env.bedrock_region}} placeholder (with optional surrounding spaces).
	EnvBedrockRegion string
}

func NewEmbeddingProvider(ctx context.Context, cfg EmbeddingProviderConfig) (*EmbeddingProvider, error) {
	if cfg.Spec == nil {
		return nil, trace.BadParameter("provider spec is required")
	}
	if cfg.ModelResourceName == "" {
		return nil, trace.BadParameter("model resource name is required")
	}
	region := cfg.Spec.GetRegion()
	if strings.ReplaceAll(region, " ", "") == apisummarizer.BedrockRegionExpansionPlaceholder {
		if cfg.EnvBedrockRegion == "" {
			return nil, trace.BadParameter("region is set to the %s placeholder but the TELEPORT_BEDROCK_REGION environment variable is not set; either set TELEPORT_BEDROCK_REGION or specify a region directly in the model spec", apisummarizer.BedrockRegionExpansionPlaceholder)
		}
		region = cfg.EnvBedrockRegion
	}
	if region == "" {
		return nil, trace.BadParameter("region is required")
	}
	if cfg.AWSConfigCache == nil {
		return nil, trace.BadParameter("AWS config cache is required")
	}

	clientFactory := cfg.ClientFactory
	if clientFactory == nil {
		clientFactory = defaultClientFactory{}
	}

	awscfg, err := cfg.AWSConfigCache.GetConfig(
		ctx,
		region,
		awsconfig.WithCredentialsMaybeIntegration(
			awsconfig.IntegrationMetadata{
				Name: cfg.Spec.GetIntegration(),
			},
		),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clientFactory.NewFromConfig(awscfg)

	logger := slog.With(teleport.ComponentKey, "bedrock", "embedding_model", cfg.ModelResourceName)
	return &EmbeddingProvider{
		bedrockModelID:    cfg.Spec.GetBedrockModelId(),
		client:            client,
		logger:            logger,
		modelResourceName: cfg.ModelResourceName,
	}, nil
}

// embedRequest represents the request payload for generating embeddings from Bedrock.
type embedRequest struct {
	InputText string `json:"inputText"`
}

// embedResponse represents the response from Bedrock containing the generated embeddings.
type embedResponse struct {
	Embeddings          []float32 `json:"embedding"`
	InputTextTokenCount int       `json:"inputTextTokenCount"`
}

// GenerateEmbeddings generates vector embeddings for the given text using the
// configured Bedrock embedding model.
func (p *EmbeddingProvider) GenerateEmbeddings(ctx context.Context, text string) ([]float32, int, error) {
	if text == "" {
		return nil, 0, trace.BadParameter("input text is required")
	}

	payload, err := json.Marshal(embedRequest{
		InputText: text,
	})
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}

	p.logger.DebugContext(ctx, "Generating embeddings")

	out, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(p.bedrockModelID),
		ContentType: aws.String("application/json"),
		Body:        payload,
	})
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}

	var resp embedResponse
	if err := json.Unmarshal(out.Body, &resp); err != nil {
		return nil, 0, trace.Wrap(err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, 0, trace.BadParameter("no embeddings returned from Bedrock")
	}

	p.logger.DebugContext(ctx, "Embeddings generated", "dimension", len(resp.Embeddings), "input_tokens", resp.InputTextTokenCount)

	return resp.Embeddings, resp.InputTextTokenCount, nil
}
