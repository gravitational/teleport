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

	switch providerCfg := modelSpec.Provider.(type) {
	case *summarizerv1pb.InferenceModelSpec_Openai:
		return openai.FormatError(err, providerCfg.Openai)
	case *summarizerv1pb.InferenceModelSpec_Bedrock:
		return bedrock.FormatError(err, providerCfg.Bedrock)
	default:
		return fmt.Sprintf("inference request failed: %v", err)
	}
}
