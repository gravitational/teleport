package openai

import (
	"log/slog"
	"testing"

	"github.com/gravitational/trace"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	summarizererrorstypes "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
)

func TestSummarizeCommand(t *testing.T) {
	ctx := t.Context()

	provider := &InferenceProvider{
		openAIModelName:   openai.ChatModelGPT5,
		temperature:       1.0,
		client:            &fakeClient{},
		modelResourceName: "test-model",
	}

	provider.logger = slog.With(teleport.ComponentKey, "openai", "inference_model", provider.modelResourceName)

	testCases := []struct {
		name    string
		content string
		assert  func(t *testing.T, resp *schema.CommandAnalysis, err error)
	}{
		{
			name:    "typical case",
			content: "json response for command analysis",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				require.NoError(t, err)
				assert.Equal(t, "ls -al", resp.Command)
				assert.Equal(t, "file_operation", resp.Category)
			},
		},
		{
			name:    "empty response",
			content: "no choices",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned no choices",
				})
			},
		},
		{
			name:    "length limit exceeded",
			content: "make the output too long",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, &trace.LimitExceededError{
					Message: "model response length limit exceeded",
				})
			},
		},
		{
			name:    "bad response",
			content: "cause an error",
			assert: func(t *testing.T, resp *schema.CommandAnalysis, err error) {
				assert.ErrorIs(t, err, summarizererrorstypes.BadResponseError{
					Message: "model returned unexpected finish reason: \"content_filter\"",
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := provider.SummarizeCommand(
				ctx, "2bce7245-6508-43e0-ab31-b1a2dcec714e", "testuser", "ubuntu", tc.content,
			)
			tc.assert(t, resp, err)
		})
	}
}
