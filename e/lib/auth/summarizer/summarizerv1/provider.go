package summarizerv1

import (
	"context"
	"io"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	apisummarizer "github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	"github.com/gravitational/teleport/lib/session"
)

// newTestProvider creates an inference provider from the test request specs.
func (s *Service) newTestProvider(ctx context.Context, req *pb.TestInferenceModelRequest) (testProvider, error) {
	modelSpec := req.GetModel()
	if modelSpec == nil {
		return nil, trace.BadParameter("model spec is required")
	}

	switch modelSpec.WhichProvider() {
	case pb.InferenceModelSpec_Openai_case:
		if req.GetSecret() == nil {
			if modelSpec.GetOpenai().GetApiKeySecretRef() == "" {
				return nil, trace.BadParameter("api_key_secret_ref is required for OpenAI models when no secret is provided in the request")
			}
			// Fetch the secret from the backend
			secret, err := s.backend.GetInferenceSecret(ctx, modelSpec.GetOpenai().GetApiKeySecretRef())
			if trace.IsNotFound(err) {
				return nil, trace.BadParameter("secret %q not found in backend; please provide it in the request or ensure it exists", modelSpec.GetOpenai().GetApiKeySecretRef())
			} else if err != nil {
				return nil, trace.Wrap(err)
			}
			req.SetSecret(secret.GetSpec())

		}

	case pb.InferenceModelSpec_Bedrock_case:
		if !s.enableBedrockWithoutRestrictions &&
			modelSpec.GetBedrock().GetIntegration() == "" {
			return nil, trace.AccessDenied(
				"access to Amazon Bedrock models provided by Teleport Cloud is restricted; " +
					"please refer to the documentation for more information on enabling Bedrock integrations",
			)
		}

	default:
		return nil, trace.BadParameter("unsupported provider type: %v", modelSpec.WhichProvider())
	}

	return s.newProvider(ctx, apisummarizer.NewInferenceModel("test-model", modelSpec), req.GetSecret())
}

func (s *Service) newProvider(ctx context.Context, model *pb.InferenceModel, secret *pb.InferenceSecretSpec) (inferenceProvider, error) {
	switch model.GetSpec().WhichProvider() {
	case pb.InferenceModelSpec_Openai_case:
		if secret == nil {
			return nil, trace.BadParameter("secret is required for OpenAI models")
		}

		p, err := openai.NewProvider(ctx, openai.ProviderConfig{
			ModelProvider:         model.GetSpec().GetOpenai(),
			SecretSpec:            secret,
			MaxSessionLength:      model.GetSpec().GetMaxSessionLengthBytes(),
			ClientFactory:         s.openAIClientFactory,
			ModelResourceName:     model.GetMetadata().GetName(),
			StructuredOutputCache: s.structuredOutputCache,
		})
		return p, trace.Wrap(err)

	case pb.InferenceModelSpec_Bedrock_case:
		p, err := bedrock.NewProvider(ctx, bedrock.ProviderConfig{
			Spec:                  model.GetSpec().GetBedrock(),
			MaxSessionLength:      model.GetSpec().GetMaxSessionLengthBytes(),
			ClientFactory:         s.bedrockClientFactory,
			ModelResourceName:     model.GetMetadata().GetName(),
			AWSConfigCache:        s.awsConfigCache,
			StructuredOutputCache: s.structuredOutputCache,
		})
		return p, trace.Wrap(err)

	default:
		return nil, trace.BadParameter("unsupported provider type: %v", model.GetSpec().WhichProvider())
	}
}

// testProvider is a minimal interface for testing inference models.
type testProvider interface {
	Summarize(ctx context.Context, sessionID session.ID, systemPrompt string, reader io.ReadCloser) (string, error)
}

// newTestEmbeddingsProvider creates an embeddings provider from the test
// request spec, using the inline secret when provided and falling back to
// the backend when only an api_key_secret_ref is set (OpenAI only).
func (s *Service) newTestEmbeddingsProvider(ctx context.Context, req *pb.TestRetrievalModelRequest) (embeddingProvider, error) {
	modelSpec := req.GetModel()
	if modelSpec == nil {
		return nil, trace.BadParameter("model spec is required")
	}

	switch modelSpec.WhichEmbeddingsProvider() {
	case pb.RetrievalModelSpec_Openai_case:
		secret := req.GetSecret()
		if secret == nil {
			if modelSpec.GetOpenai().GetApiKeySecretRef() == "" {
				return nil, trace.BadParameter("api_key_secret_ref is required for OpenAI models when no secret is provided in the request")
			}
			stored, err := s.backend.GetInferenceSecret(ctx, modelSpec.GetOpenai().GetApiKeySecretRef())
			if trace.IsNotFound(err) {
				return nil, trace.BadParameter("secret %q not found in backend; please provide it in the request or ensure it exists", modelSpec.GetOpenai().GetApiKeySecretRef())
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}
			secret = stored.GetSpec()
		}
		p, err := openai.NewEmbeddingProvider(ctx, openai.EmbeddingProviderConfig{
			EmbeddingsSpec:    modelSpec.GetOpenai(),
			SecretSpec:        secret,
			ClientFactory:     s.openAIClientFactory,
			ModelResourceName: "test-retrieval-model",
		})
		return p, trace.Wrap(err)

	case pb.RetrievalModelSpec_Bedrock_case:
		if !s.enableBedrockWithoutRestrictions && modelSpec.GetBedrock().GetIntegration() == "" {
			return nil, trace.AccessDenied(
				"access to Amazon Bedrock models provided by Teleport Cloud is restricted; " +
					"please refer to the documentation for more information on enabling Bedrock integrations",
			)
		}
		p, err := bedrock.NewEmbeddingProvider(ctx, bedrock.EmbeddingProviderConfig{
			Spec:              modelSpec.GetBedrock(),
			ClientFactory:     s.bedrockClientFactory,
			ModelResourceName: "test-retrieval-model",
			AWSConfigCache:    s.awsConfigCache,
		})
		return p, trace.Wrap(err)

	default:
		return nil, trace.BadParameter("unsupported provider type: %v", modelSpec.WhichEmbeddingsProvider())
	}
}

type inferenceProvider interface {
	Summarize(ctx context.Context, sessionID session.ID, systemPrompt string, reader io.ReadCloser) (string, error)
}

type embeddingProvider interface {
	// GenerateEmbeddings generates vector embeddings for the given text.
	GenerateEmbeddings(ctx context.Context, text string) ([]float32, int, error)
}
