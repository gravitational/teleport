package errors

import (
	"fmt"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/bedrock"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
)

// FormatInferenceError formats errors from OpenAI and AWS Bedrock providers into user-friendly messages.
func FormatInferenceError(err error, modelSpec *summarizerv1pb.InferenceModelSpec) string {
	if err == nil {
		return ""
	}

	switch modelSpec.WhichProvider() {
	case summarizerv1pb.InferenceModelSpec_Openai_case:
		return openai.FormatError(err, modelSpec.GetOpenai())
	case summarizerv1pb.InferenceModelSpec_Bedrock_case:
		return bedrock.FormatError(err, modelSpec.GetBedrock())
	default:
		return fmt.Sprintf("inference request failed: %v", err)
	}
}

// FormatRetrievalError formats errors from OpenAI and AWS Bedrock providers into user-friendly messages.
func FormatRetrievalError(err error, modelSpec *summarizerv1pb.RetrievalModelSpec) string {
	if err == nil {
		return ""
	}

	switch modelSpec.WhichEmbeddingsProvider() {
	case summarizerv1pb.RetrievalModelSpec_Openai_case:
		return openai.FormatError(err, modelSpec.GetOpenai())
	case summarizerv1pb.RetrievalModelSpec_Bedrock_case:
		return bedrock.FormatError(err, modelSpec.GetBedrock())
	default:
		return fmt.Sprintf("inference request failed: %v", err)
	}
}
